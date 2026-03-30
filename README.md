# Tailclip

Tailclip is a small Go tray app that syncs clipboard text between a Windows PC and an Android phone over your local network, including Tailscale or plain LAN.

It is designed for a local-first flow:

- Windows -> Android automatic clipboard sync
- Android -> Windows background clipboard send via Tasker
- No cloud relay, no broker service

Current platform support is intentionally narrow: Windows as the always-on agent and Android/Tasker as the phone-side integration. Other platforms are not supported today unless someone adds the missing implementation and opens a PR.

## How It Works

1. Tailclip watches Windows clipboard change notifications.
2. When new non-empty text appears, it creates a clipboard event payload.
3. It sends the payload as `POST` JSON to your Android endpoint over HTTP on your local network.
4. Duplicate content is skipped using a content hash.
5. Tailclip also exposes a small authenticated HTTP endpoint on Windows so Tasker can send Android clipboard text back to the PC.

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
- Network connectivity between both devices, either over Tailscale or the same LAN
- Tasker on Android with an HTTP endpoint reachable from the Windows PC

## Configuration

By default, Tailclip reads config from:

`%APPDATA%\tailclip\config.json`

You can override this path with `-config`.

Example config (`docs/config.example.json`):

```json
{
  "android_url": "http://100.101.102.103:8080/clipboard",
  "max_outbound_chars": 0,
  "windows_listen_addr": "",
  "auth_token": "replace-me",
  "device_id": "windows-laptop",
  "enabled": true,
  "http_timeout_ms": 3000,
  "log_level": "info"
}
```

Fields:

- `android_url` (required): full target URL, e.g. `http://100.x.y.z:8080/clipboard`
- `max_outbound_chars` (optional): maximum Unicode characters Tailclip will send from Windows to Android; `0` disables the limit (default `0`)
- `windows_listen_addr` (optional): local bind address for inbound Tasker shares; set it to `:8080` (or another address) to enable the listener, leave it empty to disable it
- `auth_token` (required): bearer token sent as `Authorization: Bearer <token>`
- `device_id` (optional): sender identifier; defaults to hostname if omitted
- `enabled` (optional): whether syncing is active (default `true`)
- `http_timeout_ms` (optional): HTTP timeout in milliseconds (default `3000`)
- `log_level` (optional): `debug`, `info`, `warn`, `error` (default `info`)

## Run

Build the binaries first:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
```

Then run the Windows agent:

```powershell
.\bin\tailclip-agent.exe
```

The build embeds the Windows manifest into the executable, so no sidecar `.manifest` file is required at runtime.

Run directly with Go:

```powershell
go run ./cmd/tailclip-agent
```

On Windows this launches the tray app. Click the tray icon to open the settings window, edit config values, enable or disable syncing, and toggle start on login.

Run with explicit config path:

```powershell
go run ./cmd/tailclip-agent -config "C:\path\to\config.json"
```

## Android / Tasker

Tasker setup instructions are in:

- [docs/TASKER_SETUP.md](docs/TASKER_SETUP.md) for both Tasker profiles and the overall Android setup requirements
- [docs/TASKER_SHARE_TO_PC.md](docs/TASKER_SHARE_TO_PC.md) for the Android `->` Windows sender profile, including the required `READ_LOGS` and background clipboard ADB grants

Importable Tasker assets live in:

- `integrations/tasker/tailclip_receiver_server.prf.xml`
- `integrations/tasker/tailclip_sender.prf.xml`
- `integrations/tasker/test-tailclip-endpoint.ps1`
- `integrations/tasker/test-tailclip-windows-endpoint.ps1`

## Build

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\build.ps1
```

Equivalent manual commands:

```powershell
go run ./cmd/genwinres -manifest .\cmd\tailclip-agent\app.manifest -out .\cmd\tailclip-agent\rsrc_windows_amd64.syso -arch amd64
go build -ldflags="-H windowsgui" -o bin/tailclip-agent.exe ./cmd/tailclip-agent
```

## Test

```powershell
go test ./...
```

## Release

GitHub release automation is documented in [docs/RELEASING.md](docs/RELEASING.md).

The intended V1 flow is:

- CI on push and pull request
- trigger the `Release` workflow from terminal with a version such as `v1.0.0`
- automatic tag creation, GitHub release publishing, asset upload, and generated notes

Terminal shortcut:

```powershell
.\scripts\release.ps1 -Version v1.0.0
```

## Project Layout

- `cmd/tailclip-agent`: platform entrypoint (`tray` on Windows, `CLI` elsewhere)
- `internal/app`: main runtime loop
- `internal/clipboard`: clipboard watcher implementation (Windows + non-Windows stub)
- `internal/event`: clipboard event model and hashing
- `internal/transport`: HTTP client for sending events
- `internal/config`: config loading/validation
- `scripts`: local developer helpers, including release trigger
- `docs/ARCHITECTURE.md`: architecture and design notes
- `docs/TASKER_SETUP.md`: Tasker guide for both Android Tasker profiles and setup requirements
- `docs/TASKER_SHARE_TO_PC.md`: Tasker guide for automatic Android `->` Windows clipboard sending
- `docs/RELEASING.md`: GitHub release workflow and versioning
- `integrations/tasker`: importable Tasker assets and endpoint test helpers

## Notes

- Current implementation is automatic in both directions, with Windows clipboard watching on the PC side and a Tasker logcat-plus-clipboard flow on Android.
- Tasker on Android is required for both the phone receiver flow and the Android clipboard sender flow.
- Tailclip works over either Tailscale or plain LAN. Tailscale is the recommended default when you want private cross-network access without opening ports.
- The Android sender profile depends on extra privileges that are not part of normal app setup: Tasker needs logcat access and background clipboard access granted over ADB.
- Delivery is best effort: failed sends are logged and the agent continues.
- Windows logs are written to `%APPDATA%\tailclip\logs\tailclip.log`.
- Non-Windows builds compile, but clipboard watching is only implemented on Windows.
- Non-Android receivers are not implemented. Cross-platform support would require new receiver/client implementations and a PR.
