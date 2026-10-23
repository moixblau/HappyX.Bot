// Package api is a small client for HappyX.Api.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrConflict  = errors.New("conflict")
	ErrForbidden = errors.New("forbidden")
	ErrTooMany   = errors.New("too many requests")
)

// Error is a non-2xx answer from the API.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("api: %d %s", e.Status, e.Message) }

func (e *Error) Is(target error) bool {
	switch target {
	case ErrNotFound:
		return e.Status == http.StatusNotFound
	case ErrConflict:
		return e.Status == http.StatusConflict
	case ErrForbidden:
		return e.Status == http.StatusForbidden
	case ErrTooMany:
		return e.Status == http.StatusTooManyRequests
	}
	return false
}

type Client struct {
	base string
	key  string
	http *http.Client
}

func New(baseURL, key string) *Client {
	return &Client{base: baseURL, key: key, http: &http.Client{Timeout: 10 * time.Second}}
}

type Guild struct {
	DiscordID       string `json:"discord_guild_id"`
	Name            string `json:"name"`
	AdminDiscordID  string `json:"admin_discord_user_id"`
	Anonymous       bool   `json:"anonymous"`
	Timezone        string `json:"timezone"`
	SurveyHour      int    `json:"survey_hour"`
	ReportFrequency string `json:"report_frequency"`
	Language        string `json:"language"`
	KudosChannelID  string `json:"kudos_channel_id"` // empty: where the command is used
	WorkingDays     []int  `json:"working_days"`     // ISO days the survey goes out, 1 is Monday
	Active          bool   `json:"active"`
}

type Member struct {
	DiscordID string `json:"discord_user_id"`
	Username  string `json:"username"`
	Active    bool   `json:"active"`
}

type GuildPatch struct {
	Anonymous       *bool   `json:"anonymous,omitempty"`
	Timezone        *string `json:"timezone,omitempty"`
	SurveyHour      *int    `json:"survey_hour,omitempty"`
	ReportFrequency *string `json:"report_frequency,omitempty"`
	AdminDiscordID  *string `json:"admin_discord_user_id,omitempty"`
	Language        *string `json:"language,omitempty"`
	KudosChannelID  *string `json:"kudos_channel_id,omitempty"`
	WorkingDays     []int   `json:"working_days,omitempty"`
}

type Report struct {
	DiscordGuildID string         `json:"discord_guild_id"`
	From           string         `json:"from"`
	To             string         `json:"to"`
	Anonymous      bool           `json:"anonymous"`
	Responses      int            `json:"responses"`
	Respondents    int            `json:"respondents"`
	Average        float64        `json:"average"`
	Distribution   map[string]int `json:"distribution"`
	Days           []ReportDay    `json:"days"`
	Messages       []ReportMsg    `json:"messages"`
	Kudos          KudosSummary   `json:"kudos"`
}

// KudosSummary counts the period's kudos; Highlights are the latest few, newest first.
type KudosSummary struct {
	Count      int     `json:"count"`
	Givers     int     `json:"givers"`
	Receivers  int     `json:"receivers"`
	Highlights []Kudos `json:"highlights"`
}

type Kudos struct {
	Day          string `json:"day"`
	FromID       string `json:"from_discord_user_id"`
	FromUsername string `json:"from_username"`
	ToID         string `json:"to_discord_user_id"`
	ToUsername   string `json:"to_username"`
	Message      string `json:"message"`
}

type ReportDay struct {
	Day       string  `json:"day"`
	Responses int     `json:"responses"`
	Average   float64 `json:"average"`
}

type ReportMsg struct {
	Day       string `json:"day"`
	Mood      int    `json:"mood"`
	Message   string `json:"message"`
	DiscordID string `json:"discord_user_id,omitempty"`
	Username  string `json:"username,omitempty"`
}

func (c *Client) RegisterGuild(ctx context.Context, id, name, adminID string) (Guild, error) {
	var g Guild
	err := c.do(ctx, http.MethodPost, "/v1/guilds", map[string]string{
		"discord_guild_id": id, "name": name, "admin_discord_user_id": adminID,
	}, &g)
	return g, err
}

func (c *Client) Guild(ctx context.Context, id string) (Guild, error) {
	var g Guild
	err := c.do(ctx, http.MethodGet, "/v1/guilds/"+url.PathEscape(id), nil, &g)
	return g, err
}

func (c *Client) ActiveGuilds(ctx context.Context) ([]Guild, error) {
	var gs []Guild
	err := c.do(ctx, http.MethodGet, "/v1/guilds", nil, &gs)
	return gs, err
}

func (c *Client) UpdateGuild(ctx context.Context, id string, p GuildPatch) (Guild, error) {
	var g Guild
	err := c.do(ctx, http.MethodPatch, "/v1/guilds/"+url.PathEscape(id), p, &g)
	return g, err
}

func (c *Client) RemoveGuild(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/v1/guilds/"+url.PathEscape(id), nil, nil)
}

// UpsertMember registers or renames a member. active nil keeps an existing member's
// opt-in state; true opts them in.
func (c *Client) UpsertMember(ctx context.Context, guildID, userID, username string, active *bool) error {
	body := map[string]any{"username": username}
	if active != nil {
		body["active"] = *active
	}
	return c.do(ctx, http.MethodPut, memberPath(guildID, userID), body, nil)
}

func (c *Client) RemoveMember(ctx context.Context, guildID, userID string) error {
	return c.do(ctx, http.MethodDelete, memberPath(guildID, userID), nil, nil)
}

func (c *Client) Members(ctx context.Context, guildID string) ([]Member, error) {
	var ms []Member
	err := c.do(ctx, http.MethodGet, "/v1/guilds/"+url.PathEscape(guildID)+"/members", nil, &ms)
	return ms, err
}

func (c *Client) RecordMood(ctx context.Context, guildID, userID, day string, mood int, message string) error {
	return c.do(ctx, http.MethodPut, memberPath(guildID, userID)+"/mood", map[string]any{
		"mood": mood, "message": message, "day": day,
	}, nil)
}

// GiveKudos records kudos from one member to another. It fails with ErrForbidden when either
// isn't a known member and ErrTooMany when the giver reached today's limit.
func (c *Client) GiveKudos(ctx context.Context, guildID, fromID, toID, message string) (Kudos, error) {
	var k Kudos
	err := c.do(ctx, http.MethodPost, "/v1/guilds/"+url.PathEscape(guildID)+"/kudos", map[string]string{
		"from_discord_user_id": fromID, "to_discord_user_id": toID, "message": message,
	}, &k)
	return k, err
}

// ClaimDispatch returns true when this caller should send the survey or report for period,
// false when it was already sent.
func (c *Client) ClaimDispatch(ctx context.Context, guildID, kind, period string) (bool, error) {
	err := c.do(ctx, http.MethodPost, "/v1/guilds/"+url.PathEscape(guildID)+"/dispatches",
		map[string]string{"kind": kind, "period": period}, nil)
	if errors.Is(err, ErrConflict) {
		var apiErr *Error
		if errors.As(err, &apiErr) && apiErr.Message == "already dispatched" {
			return false, nil
		}
	}
	return err == nil, err
}

// Report returns the report for [from, to]; empty strings mean the last complete period.
func (c *Client) Report(ctx context.Context, guildID, from, to string) (Report, error) {
	path := "/v1/guilds/" + url.PathEscape(guildID) + "/report"
	if from != "" || to != "" {
		path += "?" + url.Values{"from": {from}, "to": {to}}.Encode()
	}
	var r Report
	err := c.do(ctx, http.MethodGet, path, nil, &r)
	return r, err
}

func memberPath(guildID, userID string) string {
	return "/v1/guilds/" + url.PathEscape(guildID) + "/members/" + url.PathEscape(userID)
}

func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&e)
		return &Error{Status: res.StatusCode, Message: e.Error}
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	return nil
}
