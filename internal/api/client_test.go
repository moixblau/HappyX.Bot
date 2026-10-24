package api

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// Contract test against a running HappyX.Api. Set HAPPYX_API_URL and HAPPYX_API_KEY to run it.
// It writes to the guild 100000000000000077, so don't point it at production.
func TestClientAgainstAPI(t *testing.T) {
	url, key := os.Getenv("HAPPYX_API_URL"), os.Getenv("HAPPYX_API_KEY")
	if url == "" || key == "" {
		t.Skip("HAPPYX_API_URL / HAPPYX_API_KEY not set")
	}
	ctx := context.Background()
	c := New(url, key)
	const g, admin, u = "100000000000000077", "300000000000000077", "200000000000000077"
	today := time.Now().In(mustLoc(t, "Europe/Madrid")).Format("2006-01-02")

	if _, err := c.RegisterGuild(ctx, g, "Contract test", admin); err != nil {
		t.Fatal(err)
	}
	if err := c.UpsertMember(ctx, g, u, "tester", nil); err != nil {
		t.Fatal(err)
	}
	if err := c.RecordMood(ctx, g, u, today, 4, "hola"); err != nil {
		t.Fatal(err)
	}
	if err := c.RecordMood(ctx, g, "200000000000000078", today, 4, ""); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-member answer: want ErrForbidden, got %v", err)
	}
	r, err := c.Report(ctx, g, today, today)
	if err != nil || r.Responses < 1 {
		t.Fatalf("report: %+v %v", r, err)
	}

	period := "2001-01-01" // fixed so a rerun sees it already claimed
	first, err := c.ClaimDispatch(ctx, g, "survey", period)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.ClaimDispatch(ctx, g, "survey", period)
	if err != nil || second {
		t.Fatalf("second claim should be false without error: %v %v (first %v)", second, err, first)
	}

	const u2 = "200000000000000079"
	if err := c.UpsertMember(ctx, g, u2, "tester2", nil); err != nil {
		t.Fatal(err)
	}
	k, err := c.GiveKudos(ctx, g, u, u2, "gracias")
	if err != nil && !errors.Is(err, ErrTooMany) { // reruns on the same day hit the limit
		t.Fatal(err)
	}
	if err == nil && (k.FromUsername != "tester" || k.ToUsername != "tester2") {
		t.Fatalf("kudos should come back with usernames: %+v", k)
	}
	var apiErr *Error
	if _, err := c.GiveKudos(ctx, g, u, u, "yo"); !errors.As(err, &apiErr) || apiErr.Status != 400 {
		t.Fatalf("self kudos: want 400, got %v", err)
	}
	if r, err := c.Report(ctx, g, today, today); err != nil || r.Kudos.Count < 1 || len(r.Kudos.Highlights) == 0 {
		t.Fatalf("report kudos: %+v %v", r.Kudos, err)
	}

	if err := c.RemoveMember(ctx, g, u); err != nil {
		t.Fatal(err)
	}
	if err := c.UpsertMember(ctx, g, u, "tester", nil); err != nil {
		t.Fatal(err)
	}
	ms, err := c.Members(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range ms {
		if m.DiscordID == u {
			t.Fatal("sync re-subscribed a member who opted out")
		}
	}

	ids, err := c.Removals(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if id == g {
			t.Fatal("a registered guild shouldn't be waiting for removal")
		}
	}
	if err := c.DoneRemoving(ctx, g); err != nil {
		t.Fatalf("confirming a removal that isn't queued should be harmless: %v", err)
	}
}

func mustLoc(t *testing.T, name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}
