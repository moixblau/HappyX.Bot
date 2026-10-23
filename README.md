# HappyX.Bot

Discord bot for HappyX. Every day it sends each member of a server a private message asking
how they feel (1-5, with an optional comment), and sends the server's admin a weekly or monthly
report. One bot serves any number of servers; their data is kept apart by
[HappyX.Api](https://github.com/moixblau/HappyX.Api).

## How it works

- **Joining a server** registers it in the API with the server owner as admin, and registers every
  human member (bots are ignored). New members are added as they join; members who leave are removed.
- **Survey**: from the configured hour (default 13:00 Europe/Madrid) the bot DMs every active member
  five buttons. After answering, they can add a comment. Answering again the same day replaces the answer.
  A **Don't ask me again** button under the survey stops it for that server (same as `/happyx pause`).
- **Report**: on Mondays (weekly) or the 1st (monthly), from the survey hour, the admin gets the
  last period by DM as an image (average, daily trend, distribution) plus the comments as text.
- **Kudos**: `/happyx kudos user:@someone message:...` thanks a teammate in public. The card is posted
  in the kudos channel (`/happyx config kudos_channel`) or, without one, where the command was used,
  and the receiver gets a DM linking to it. No kudos to bots or to yourself, and at most 10 a day each.
  Reports count them and quote the latest five; kudos keep their names even in anonymous servers,
  since they are public anyway.
- The API records each survey and report it has sent, so restarts never send twice.
- Surveys are **anonymous by default**: the admin sees aggregated results and comments, not who wrote them.
- **Languages**: each server has a language (English by default, set with `/happyx config language`).
  Surveys, answers and reports use it. Slash command names and replies follow each user's Discord
  language when the bot has it, and the server's language otherwise.

## Commands

| Command | Who | What |
| --- | --- | --- |
| `/happyx status` | everyone | settings and whether you receive the survey |
| `/happyx pause` / `/happyx resume` | everyone | stop / resume receiving the survey in this server |
| `/happyx kudos` | everyone | `user`, `message` (≤300): publicly thank a teammate |
| `/happyx config` | admin | `anonymous`, `hour`, `timezone`, `frequency`, `language`, `admin`, `kudos_channel` |
| `/happyx report` | admin | see the last completed period's report now, or the one in progress with `period:current` |

Discord shows these names translated for users whose client is in a supported language.

## Adding a language

Every language is one file in [`internal/i18n/locales`](internal/i18n/locales), named by its code
(`en.json`, `pt-BR.json`…).

1. Copy `en.json` to `<code>.json` and translate the values. Keep `{placeholders}` as they are.
2. Set `_name` to the language's own name (shown in `/happyx config language`) and `_discord` to the
   [Discord locales](https://discord.com/developers/docs/reference#locales) that should use it
   (`[]` if Discord has none, as with Catalan).
3. Run `go test ./...`. It fails if a key or placeholder is missing or extra, or a command name isn't
   valid for Discord (lowercase, no spaces, up to 32 characters).

No code changes are needed; the new language appears in `/happyx config` on the next deploy.
Discord allows at most 25 choices, so past 25 languages the option will need autocomplete instead.

## Previewing messages

`go run ./cmd/preview -lang es -out preview.html` writes a page that shows every message the bot
sends (survey, kudos, replies, report with its image) styled like Discord, built with the bot's own
code and sample data. No Discord server or API needed.

## Setting up the Discord application

1. Create an application at https://discord.com/developers/applications and add a **Bot**.
2. Under **Bot > Privileged Gateway Intents**, enable **Server Members Intent** (needed to list members).
3. Copy the bot token into `DISCORD_TOKEN`.
4. Invite it with **OAuth2 > URL Generator**, scopes `bot` and `applications.commands`. No extra
   permissions are needed for the survey and reports, which go by DM. To post kudos in a
   `kudos_channel`, the bot needs **View Channel**, **Send Messages** and **Embed Links** there;
   otherwise kudos are posted as the reply to the command, which needs no permissions.

Members need "Allow direct messages from server members" enabled (Discord's default) to receive the survey.

## Configuration

All from environment variables (see `.env.example`):

| Variable | Purpose |
| --- | --- |
| `DISCORD_TOKEN` | Bot token |
| `API_URL` | HappyX.Api base URL, e.g. `http://api:8080` inside docker compose |
| `API_KEY` | API key; put its SHA-256 hash in the API's `API_KEY_HASHES` |

## Development

```sh
go test ./...
# contract test against a running API (writes test data; not production):
HAPPYX_API_URL=http://localhost:8083 HAPPYX_API_KEY=... go test ./internal/api/
```

To run everything, use the `docker-compose.yml` in [HappyX](https://github.com/moixblau/HappyX).
