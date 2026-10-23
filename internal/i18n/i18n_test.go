package i18n

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

var placeholder = regexp.MustCompile(`\{[a-z_]+\}`)

// Discord command and option names: lowercase, no spaces, 1-32 characters.
var commandName = regexp.MustCompile(`^[-_\p{Ll}\p{N}]{1,32}$`)

// discordLocales are the locales Discord clients send.
var discordLocales = map[string]bool{
	"id": true, "da": true, "de": true, "en-GB": true, "en-US": true, "es-ES": true, "es-419": true,
	"fr": true, "hr": true, "it": true, "lt": true, "hu": true, "nl": true, "no": true, "pl": true,
	"pt-BR": true, "ro": true, "fi": true, "sv-SE": true, "vi": true, "tr": true, "cs": true, "el": true,
	"bg": true, "ru": true, "uk": true, "hi": true, "th": true, "zh-CN": true, "ja": true, "zh-TW": true, "ko": true,
}

// Every translation must have exactly English's keys, with the same placeholders.
func TestLocalesMatchEnglish(t *testing.T) {
	en := Get(Default)
	for _, code := range Codes() {
		l := Get(code)
		if l.Name == "" {
			t.Errorf("%s: missing _name", code)
		}
		for _, d := range l.Discord {
			if !discordLocales[d] {
				t.Errorf("%s: %q is not a Discord locale", code, d)
			}
		}
		for key, want := range en.messages {
			got, ok := l.messages[key]
			if !ok {
				t.Errorf("%s: missing %q", code, key)
				continue
			}
			if a, b := placeholders(want), placeholders(got); a != b {
				t.Errorf("%s: %q has placeholders %s, English has %s", code, key, b, a)
			}
			if strings.HasSuffix(key, ".name") && !commandName.MatchString(got) {
				t.Errorf("%s: %q = %q is not a valid Discord command name", code, key, got)
			}
		}
		for key := range l.messages {
			if _, ok := en.messages[key]; !ok {
				t.Errorf("%s: %q is not in en.json", code, key)
			}
		}
		if n := len(l.List("date.months")); n != 12 {
			t.Errorf("%s: date.months has %d names", code, n)
		}
		if n := len(l.List("date.weekdays")); n != 7 {
			t.Errorf("%s: date.weekdays has %d names", code, n)
		}
	}
}

func placeholders(s string) string {
	p := placeholder.FindAllString(s, -1)
	sort.Strings(p)
	return strings.Join(p, ",")
}

func TestLookup(t *testing.T) {
	if Get("es").T("survey.question", "guild", "Test") != "👋 ¿Cómo te encuentras hoy? (**Test**)" {
		t.Fatal("placeholder not filled")
	}
	if Get("xx").Code != Default || Get("pt-PT").Code != Default {
		t.Fatal("unknown languages fall back to English")
	}
	if l, ok := FromDiscord("es-419"); !ok || l.Code != "es" {
		t.Fatal("Discord es-419 should map to es")
	}
	if _, ok := FromDiscord("ja"); ok {
		t.Fatal("no Japanese catalog yet")
	}
}

// The language option lists every catalog as a choice, and Discord allows at most 25.
func TestFitsDiscordChoices(t *testing.T) {
	if n := len(Codes()); n > 25 {
		t.Fatalf("%d languages: switch /happyx config language to autocomplete", n)
	}
}
