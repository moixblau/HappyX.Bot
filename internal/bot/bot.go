// Package bot connects Discord to HappyX.Api: it keeps guilds and members in sync, sends the
// daily survey by DM, records answers and sends the admin their report.
package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/moixblau/HappyX.Bot/internal/api"
	"github.com/moixblau/HappyX.Bot/internal/i18n"
	"github.com/moixblau/HappyX.Bot/internal/schedule"
)

type Bot struct {
	dg  *discordgo.Session
	api *api.Client
	log *slog.Logger
	now func() time.Time

	commandID atomic.Pointer[string] // /happyx's ID once registered, for clickable mentions
}

func New(token string, client *api.Client, log *slog.Logger) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}
	// GuildMembers is a privileged intent: enable "Server Members Intent" in the
	// Discord developer portal.
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsDirectMessages
	b := &Bot{dg: dg, api: client, log: log, now: time.Now}
	dg.AddHandler(b.onReady)
	dg.AddHandler(b.onGuildCreate)
	dg.AddHandler(b.onGuildDelete)
	dg.AddHandler(b.onMemberAdd)
	dg.AddHandler(b.onMemberRemove)
	dg.AddHandler(b.onInteraction)
	return b, nil
}

// Run connects to Discord and runs the scheduler until ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	if err := b.dg.Open(); err != nil {
		return fmt.Errorf("discord: %w", err)
	}
	defer b.dg.Close()

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			b.tick(ctx)
		}
	}
}

func (b *Bot) onReady(s *discordgo.Session, r *discordgo.Ready) {
	b.log.Info("connected", "user", r.User.Username, "guilds", len(r.Guilds))
	// One status for every server, so it stays in English.
	if err := s.UpdateCustomStatus("💙 How are you today? · /happyx"); err != nil {
		b.log.Warn("set status", "err", err)
	}
	cmds, err := s.ApplicationCommandBulkOverwrite(r.User.ID, "", commands())
	if err != nil {
		b.log.Error("register commands", "err", err)
		return
	}
	for _, c := range cmds {
		if c.Name == "happyx" {
			id := c.ID
			b.commandID.Store(&id)
		}
	}
}

// --- Guild and member sync ---

// onGuildCreate fires when the bot joins a server and, on every (re)connect, for each server it
// is in. Registering is idempotent and keeps the guild's settings.
func (b *Bot) onGuildCreate(s *discordgo.Session, e *discordgo.GuildCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := b.api.RegisterGuild(ctx, e.ID, e.Name, e.OwnerID); err != nil {
		b.log.Error("register guild", "guild", e.ID, "err", err)
		return
	}
	n, err := b.syncMembers(ctx, e.ID)
	if err != nil {
		b.log.Error("sync members", "guild", e.ID, "err", err)
		return
	}
	b.log.Info("guild ready", "guild", e.ID, "name", e.Name, "members", n)
}

// syncMembers registers every human member. It doesn't change anyone's opt-in state.
func (b *Bot) syncMembers(ctx context.Context, guildID string) (int, error) {
	n, after := 0, ""
	for {
		page, err := b.dg.GuildMembers(guildID, after, 1000, discordgo.WithContext(ctx))
		if err != nil {
			return n, err
		}
		for _, m := range page {
			if m.User == nil || m.User.Bot {
				continue
			}
			if err := b.api.UpsertMember(ctx, guildID, m.User.ID, m.User.Username, nil); err != nil {
				return n, err
			}
			n++
		}
		if len(page) < 1000 {
			return n, nil
		}
		after = page[len(page)-1].User.ID
	}
}

func (b *Bot) onGuildDelete(s *discordgo.Session, e *discordgo.GuildDelete) {
	if e.Unavailable {
		return // Discord outage, not a removal
	}
	if err := b.api.RemoveGuild(context.Background(), e.ID); err != nil && !errors.Is(err, api.ErrNotFound) {
		b.log.Error("remove guild", "guild", e.ID, "err", err)
		return
	}
	b.log.Info("removed from guild", "guild", e.ID)
}

func (b *Bot) onMemberAdd(s *discordgo.Session, e *discordgo.GuildMemberAdd) {
	if e.User == nil || e.User.Bot {
		return
	}
	yes := true
	if err := b.api.UpsertMember(context.Background(), e.GuildID, e.User.ID, e.User.Username, &yes); err != nil {
		b.log.Error("member join", "guild", e.GuildID, "err", err)
	}
}

func (b *Bot) onMemberRemove(s *discordgo.Session, e *discordgo.GuildMemberRemove) {
	if e.User == nil || e.User.Bot {
		return
	}
	if err := b.api.RemoveMember(context.Background(), e.GuildID, e.User.ID); err != nil && !errors.Is(err, api.ErrNotFound) {
		b.log.Error("member leave", "guild", e.GuildID, "err", err)
	}
}

// --- Scheduler ---

func (b *Bot) tick(ctx context.Context) {
	guilds, err := b.api.ActiveGuilds(ctx)
	if err != nil {
		b.log.Error("list guilds", "err", err)
		return
	}
	now := b.now()
	for _, g := range guilds {
		st := schedule.Settings{Timezone: g.Timezone, SurveyHour: g.SurveyHour, ReportFrequency: g.ReportFrequency}
		if day, due := schedule.SurveyDue(st, now); due {
			b.dispatch(ctx, g, "survey", day, func() error { return b.sendSurvey(ctx, g, day) })
		}
		if from, to, due := schedule.ReportDue(st, now); due {
			b.dispatch(ctx, g, "report", from, func() error { return b.sendReport(ctx, g, from, to) })
		}
	}
}

// dispatch runs send only if the API confirms nobody sent it yet for this period.
func (b *Bot) dispatch(ctx context.Context, g api.Guild, kind, period string, send func() error) {
	claimed, err := b.api.ClaimDispatch(ctx, g.DiscordID, kind, period)
	if err != nil {
		b.log.Error("claim dispatch", "guild", g.DiscordID, "kind", kind, "err", err)
		return
	}
	if !claimed {
		return
	}
	if err := send(); err != nil {
		b.log.Error("dispatch", "guild", g.DiscordID, "kind", kind, "period", period, "err", err)
		return
	}
	b.log.Info("dispatched", "guild", g.DiscordID, "kind", kind, "period", period)
}

func (b *Bot) sendSurvey(ctx context.Context, g api.Guild, day string) error {
	members, err := b.api.Members(ctx, g.DiscordID)
	if err != nil {
		return err
	}
	msg := surveyMessage(i18n.Get(g.Language), g, day)
	failed := 0
	for _, m := range members {
		if err := b.dm(m.DiscordID, msg); err != nil {
			// Usually the user has DMs from server members turned off.
			b.log.Warn("survey DM failed", "guild", g.DiscordID, "user", m.DiscordID, "err", err)
			failed++
		}
	}
	if failed > 0 && failed == len(members) {
		return fmt.Errorf("could not DM any of %d members", failed)
	}
	return nil
}

func (b *Bot) sendReport(ctx context.Context, g api.Guild, from, to string) error {
	r, err := b.api.Report(ctx, g.DiscordID, from, to)
	if err != nil {
		return err
	}
	return b.dm(g.AdminDiscordID, reportMessage(i18n.Get(g.Language), g, r))
}

func (b *Bot) dm(userID string, msg *discordgo.MessageSend) error {
	ch, err := b.dg.UserChannelCreate(userID)
	if err != nil {
		return err
	}
	_, err = b.dg.ChannelMessageSendComplex(ch.ID, msg)
	return err
}
