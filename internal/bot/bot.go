// Package bot connects Discord to HappyX.Api: it keeps guilds and members in sync, sends the
// daily survey by DM, records answers and sends the admin their report.
package bot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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
	// webURL is the dashboard's address, e.g. https://happyx.example.com, for links to the
	// game. Empty when the dashboard isn't deployed.
	webURL string

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
	dg.AddHandler(b.onMemberUpdate)
	dg.AddHandler(b.onMemberRemove)
	dg.AddHandler(func(s *discordgo.Session, e *discordgo.GuildRoleCreate) { b.syncRoles(context.Background(), e.GuildID) })
	dg.AddHandler(func(s *discordgo.Session, e *discordgo.GuildRoleUpdate) { b.syncRoles(context.Background(), e.GuildID) })
	dg.AddHandler(func(s *discordgo.Session, e *discordgo.GuildRoleDelete) { b.syncRoles(context.Background(), e.GuildID) })
	dg.AddHandler(b.onInteraction)
	return b, nil
}

// SetWebURL tells the bot where the dashboard is, so it can link members to the game.
func (b *Bot) SetWebURL(u string) { b.webURL = strings.TrimRight(u, "/") }

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
	if _, err := b.api.RegisterGuild(ctx, e.ID, e.Name, e.OwnerID); errors.Is(err, api.ErrGone) {
		b.leave(ctx, e.ID) // deleted from HappyX while the bot was offline
		return
	} else if err != nil {
		b.log.Error("register guild", "guild", e.ID, "err", err)
		return
	}
	b.sendRoles(ctx, e.ID, e.Roles)
	n, err := b.syncMembers(ctx, e.ID)
	if err != nil {
		b.log.Error("sync members", "guild", e.ID, "err", err)
		return
	}
	b.log.Info("guild ready", "guild", e.ID, "name", e.Name, "members", n)
}

// syncRoles sends the server's roles, as the session's state holds them after a role event.
func (b *Bot) syncRoles(ctx context.Context, guildID string) {
	g, err := b.dg.State.Guild(guildID)
	if err != nil {
		b.log.Error("roles from state", "guild", guildID, "err", err)
		return
	}
	b.sendRoles(ctx, guildID, g.Roles)
}

// sendRoles gives the API the roles the dashboard can grant access to. @everyone (the guild's
// own ID) and the roles of bots and integrations are left out.
func (b *Bot) sendRoles(ctx context.Context, guildID string, roles []*discordgo.Role) {
	out := make([]api.Role, 0, len(roles))
	for _, r := range roles {
		if r.ID == guildID || r.Managed {
			continue
		}
		out = append(out, api.Role{ID: r.ID, Name: r.Name, Color: r.Color, Position: r.Position})
	}
	if err := b.api.SyncRoles(ctx, guildID, out); err != nil {
		b.log.Error("sync roles", "guild", guildID, "err", err)
	}
}

// syncMembers registers every human member with their roles, a page of Discord's member list
// per API request. It doesn't change anyone's opt-in state.
func (b *Bot) syncMembers(ctx context.Context, guildID string) (int, error) {
	n, after := 0, ""
	for {
		page, err := b.dg.GuildMembers(guildID, after, 1000, discordgo.WithContext(ctx))
		if err != nil {
			return n, err
		}
		batch := make([]api.MemberSync, 0, len(page))
		for _, m := range page {
			if m.User == nil || m.User.Bot {
				continue
			}
			batch = append(batch, api.MemberSync{ID: m.User.ID, Username: m.User.Username, RoleIDs: m.Roles})
			// Keep them in the session state so the survey can greet them by their server name,
			// even if the API sync below fails.
			m.GuildID = guildID
			_ = b.dg.State.MemberAdd(m)
		}
		if err := b.sendMembers(ctx, guildID, batch); err != nil {
			return n, err
		}
		n += len(batch)
		if len(page) < 1000 {
			return n, nil
		}
		after = page[len(page)-1].User.ID
	}
}

// sendMembers syncs a page of members in one request, or one by one when the API is older
// than the bulk endpoint.
func (b *Bot) sendMembers(ctx context.Context, guildID string, ms []api.MemberSync) error {
	if len(ms) == 0 {
		return nil
	}
	err := b.api.SyncMembers(ctx, guildID, ms)
	var apiErr *api.Error
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusMethodNotAllowed {
		return err
	}
	for _, m := range ms {
		if err := b.api.SyncMember(ctx, guildID, m.ID, m.Username, m.RoleIDs); err != nil {
			return err
		}
	}
	return nil
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

// leaveRemoved leaves the servers whose admin deleted them from HappyX on the dashboard.
func (b *Bot) leaveRemoved(ctx context.Context) {
	ids, err := b.api.Removals(ctx)
	if err != nil {
		b.log.Error("list removals", "err", err)
		return
	}
	for _, id := range ids {
		b.leave(ctx, id)
	}
}

// leave makes the bot leave a server deleted from HappyX and tells the API once it's out.
func (b *Bot) leave(ctx context.Context, guildID string) {
	err := b.dg.GuildLeave(guildID, discordgo.WithContext(ctx))
	var re *discordgo.RESTError
	if err != nil && !(errors.As(err, &re) && re.Response != nil && re.Response.StatusCode == http.StatusNotFound) {
		b.log.Error("leave deleted guild", "guild", guildID, "err", err)
		return // retried on the next tick
	}
	if err := b.api.DoneRemoving(ctx, guildID); err != nil {
		b.log.Error("confirm removal", "guild", guildID, "err", err)
		return
	}
	b.log.Info("left guild deleted from HappyX", "guild", guildID)
}

func (b *Bot) onMemberAdd(s *discordgo.Session, e *discordgo.GuildMemberAdd) {
	if e.User == nil || e.User.Bot {
		return
	}
	ctx := context.Background()
	yes := true
	if err := b.api.UpsertMember(ctx, e.GuildID, e.User.ID, e.User.Username, &yes); err != nil {
		b.log.Error("member join", "guild", e.GuildID, "err", err)
		return
	}
	// Someone coming back starts without the roles they had before.
	if err := b.api.SyncMember(ctx, e.GuildID, e.User.ID, e.User.Username, e.Roles); err != nil {
		b.log.Error("member roles", "guild", e.GuildID, "err", err)
	}
}

// onMemberUpdate keeps a member's roles and username current, so access given to a role on the
// dashboard follows who has it.
func (b *Bot) onMemberUpdate(s *discordgo.Session, e *discordgo.GuildMemberUpdate) {
	if e.Member == nil || e.User == nil || e.User.Bot {
		return
	}
	if err := b.api.SyncMember(context.Background(), e.GuildID, e.User.ID, e.User.Username, e.Roles); err != nil && !errors.Is(err, api.ErrConflict) {
		b.log.Error("member update", "guild", e.GuildID, "err", err)
	}
}

func (b *Bot) onMemberRemove(s *discordgo.Session, e *discordgo.GuildMemberRemove) {
	if e.User == nil || e.User.Bot {
		return
	}
	if err := b.api.MemberLeft(context.Background(), e.GuildID, e.User.ID); err != nil && !errors.Is(err, api.ErrNotFound) {
		b.log.Error("member leave", "guild", e.GuildID, "err", err)
	}
}

// --- Scheduler ---

func (b *Bot) tick(ctx context.Context) {
	b.leaveRemoved(ctx)
	guilds, err := b.api.ActiveGuilds(ctx)
	if err != nil {
		b.log.Error("list guilds", "err", err)
		return
	}
	now := b.now()
	for _, g := range guilds {
		st := schedule.Settings{Timezone: g.Timezone, SurveyHour: g.SurveyHour, ReportFrequency: g.ReportFrequency, WorkingDays: g.WorkingDays}
		if day, due := schedule.SurveyDue(st, now); due {
			b.dispatch(ctx, g, "survey", day, func() error { return b.sendSurvey(ctx, g, day) })
		}
		if from, to, due := schedule.ReportDue(st, now); due {
			b.dispatch(ctx, g, "report", from, func() error { return b.sendReport(ctx, g, from, to) })
		}
		if !g.GamesEnabled {
			continue
		}
		if week, due := schedule.GameQuestionDue(st, now); due {
			b.dispatch(ctx, g, "game", week, func() error { return b.sendGameQuestion(ctx, g, week) })
		}
		if week, due := schedule.GameGuessDue(st, now); due {
			b.dispatch(ctx, g, "game_guess", week, func() error { return b.sendGameGuess(ctx, g, week) })
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
	list := b.api.Members
	if strings.Contains(g.SurveyMessage, "user.streak") {
		list = b.api.MembersWithStreak
	}
	members, err := list(ctx, g.DiscordID)
	if err != nil {
		return err
	}
	l := i18n.Get(g.Language)
	return b.dmEach(g, members, func(m api.Member) *discordgo.MessageSend {
		return surveyMessage(l, g, day, surveyRecipient{ID: m.DiscordID, Name: b.displayName(g.DiscordID, m), Streak: m.Streak})
	}, "survey")
}

// displayName is the member's name as the server shows it: their server nickname, else their
// Discord display name, else the username HappyX stored.
func (b *Bot) displayName(guildID string, m api.Member) string {
	if dm, err := b.dg.State.Member(guildID, m.DiscordID); err == nil && dm.User != nil {
		return dm.DisplayName()
	}
	return m.Username
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
