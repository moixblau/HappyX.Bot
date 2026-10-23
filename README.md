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
- **Report**: on Mondays (weekly) or the 1st (monthly), from the survey hour, the admin gets a
  summary of the last period by DM.
- The API records each survey and report it has sent, so restarts never send twice.
- Surveys are **anonymous by default**: the admin sees aggregated results and comments, not who wrote them.

## Commands

| Command | Who | What |
| --- | --- | --- |
| `/happyx estado` | everyone | settings and whether you receive the survey |
| `/happyx pausar` / `/happyx activar` | everyone | stop / resume receiving the survey in this server |
| `/happyx config` | admin | `anonimo`, `hora`, `zona`, `frecuencia`, `admin` |
| `/happyx report` | admin | see the last period's report now |

## Setting up the Discord application

1. Create an application at https://discord.com/developers/applications and add a **Bot**.
2. Under **Bot > Privileged Gateway Intents**, enable **Server Members Intent** (needed to list members).
3. Copy the bot token into `DISCORD_TOKEN`.
4. Invite it with **OAuth2 > URL Generator**, scopes `bot` and `applications.commands`. No extra
   permissions are needed; it only sends DMs.

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
