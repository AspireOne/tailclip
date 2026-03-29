# Tailclip

Tailclip is a small Go agent that syncs text copied on a Windows PC to an Android phone over your Tailscale network.

It is designed for a private, local-first flow:

- Windows clipboard text only (current scope)
- Android receives updates via a Tasker HTTP endpoint
- No cloud relay, no broker service

Current platform support is intentionally narrow: Windows as the sender, Android as the receiver. Other platforms are not supported today unless someone adds the missing implementation and opens a PR.

## How It Works

1. Tailclip watches Windows clipboard change notifications.
2. When new non-empty text appears, it creates a clipboard event payload.
3. It sends the payload as `POST` JSON to your Android endpoint over Tailscale.
4. Duplicate content is skipped using a content hash.

Request payload shape:

```json
{
  "id": "evt_abc123",
  "content": "hello from windows",
  "content_hash": "sha256:...",
  "source_device_id": "windows-laptop",
  "created_at": "2026-03-29T22:10:00Z"
}
```

## Requirements

- Windows 11 (primary runtime target)
- Go 1.26.1+
- Tailscale connected on both devices
- Tasker on Android with an HTTP endpoint reachable from the tailnet

## Configuration

By default, Tailclip reads config from:

`%APPDATA%\tailclip\config.json`

You can override this path with `-config`.

Example config (`docs/config.example.json`):

```json
{
  "android_url": "http://100.101.102.103:8080/clipboard",
  "auth_token": "replace-me",
  "device_id": "windows-laptop",
  "http_timeout_ms": 3000,
  "poll_interval_ms": 300,
  "log_level": "info"
}
```

Fields:

- `android_url` (required): full target URL, e.g. `http://100.x.y.z:8080/clipboard`
- `auth_token` (required): bearer token sent as `Authorization: Bearer <token>`
- `device_id` (optional): sender identifier; defaults to hostname if omitted
- `http_timeout_ms` (optional): HTTP timeout in milliseconds (default `3000`)
- `poll_interval_ms` (optional): watcher fallback interval in milliseconds (default `300`)
- `log_level` (optional): `debug`, `info`, `warn`, `error` (default `info`)

## Run

Run directly with Go:

```powershell
go run ./cmd/tailclip-agent
```

Run with explicit config path:

```powershell
go run ./cmd/tailclip-agent -config "C:\path\to\config.json"
```

## Android / Tasker

Tasker setup instructions are in [docs/TASKER_SETUP.md](docs/TASKER_SETUP.md).

Importable Tasker assets live in:

- `integrations/tasker/Tailclip.prf.xml`
- `integrations/tasker/test-tailclip-endpoint.ps1`

## Build

```powershell
go build -o bin/tailclip-agent.exe ./cmd/tailclip-agent
```

## Test

```powershell
go test ./...
```

## Project Layout

- `cmd/tailclip-agent`: CLI entrypoint
- `internal/app`: main runtime loop
- `internal/clipboard`: clipboard watcher implementation (Windows + non-Windows stub)
- `internal/event`: clipboard event model and hashing
- `internal/transport`: HTTP client for sending events
- `internal/config`: config loading/validation
- `docs/ARCHITECTURE.md`: architecture and design notes
- `docs/TASKER_SETUP.md`: Android/Tasker setup guide
- `integrations/tasker`: importable Tasker assets and endpoint test helper

## Notes

- Current implementation is one-way: Windows -> Android.
- Tasker on Android is required for the receiver side in the current design.
- Delivery is best effort: failed sends are logged and the agent continues.
- Non-Windows builds compile, but clipboard watching is only implemented on Windows.
- Non-Android receivers are not implemented. Cross-platform support would require new receiver/client implementations and a PR.
