# Tailclip Tasker Share To PC Setup

This document covers the manual `Android -> Windows` path.

The intended flow is:

1. Select text on Android.
2. Tap `Share`.
3. Choose Tasker's `Send to PC` share target.
4. Tasker posts that text to the Tailclip tray app on Windows over Tailscale.
5. Tailclip writes the text to the Windows clipboard.

This path is manual by design. It does not rely on Android background clipboard access.

## Prerequisites

- Tailclip running on Windows with `windows_listen_addr` configured
- Tasker installed on Android
- Tailscale connected on both devices
- A shared token for `Authorization: Bearer <token>`

Recommended Tasker globals:

- `%TAILCLIP_PC_URL`: full Windows endpoint URL, for example `http://100.67.245.84:8080/share`
- `%TAILCLIP_TOKEN`: the same bearer token used in Tailclip config

## Windows Config

Use a config like:

```json
{
  "android_url": "http://100.101.102.103:8080/clipboard",
  "windows_listen_addr": ":8080",
  "auth_token": "replace-me",
  "device_id": "windows-laptop",
  "enabled": true,
  "http_timeout_ms": 3000
}
```

`windows_listen_addr` is the local address Tailclip binds on the PC. Leave it empty to disable inbound sharing entirely.

Tasker should point to the PC's tailnet URL, for example:

```text
http://100.67.245.84:8080/share
```

## Tasker Profile

Create a profile:

1. `Profiles` -> `+` -> `Event`
2. Choose `Received Share`
3. Set:
   - `Share Trigger`: `Send to PC`
   - `Mime Type`: `text/plain`
4. Attach a new task named `Tailclip Send To PC`

If you want the trigger to appear directly in the Android share sheet, enable Tasker's `Direct Share Targets` preference and fully exit/reopen Tasker once.

This repo also includes an exported Tasker asset for this flow:

- [`integrations/tasker/tailclip_share_sender.xml`](../integrations/tasker/tailclip_share_sender.xml)

## Tasker Task

Use these actions in `Tailclip Send To PC`:

1. `If`
   - `%rs_text ~ ^$`
2. `Flash`
   - `No shared text received`
3. `Stop`
4. `End If`
5. `HTTP Request`
   - `Method`: `POST`
   - `URL`: `%TAILCLIP_PC_URL`
   - `Headers`:
     - `Authorization: Bearer %TAILCLIP_TOKEN`
     - `Content-Type: application/json`
   - `Body`:

```json
{
  "content": "%rs_text",
  "source_device_id": "android-tasker"
}
```

6. `If`
   - `%http_response_code ~ 2*`
7. `Flash`
   - `Sent to PC`
8. `Else`
9. `Notify`
   - `Tailclip share failed (%http_response_code)`
10. `End If`

Tasker does not need to compute the content hash. Tailclip fills in missing event metadata server-side.

## Windows Endpoint Contract

Tailclip listens for:

- `POST /share`
- `Content-Type: application/json`
- `Authorization: Bearer <token>`

Minimal body:

```json
{
  "content": "hello from android",
  "source_device_id": "android-tasker"
}
```

Tailclip rejects:

- wrong token with `401`
- wrong method with `405`
- missing or empty `content` with `400`

## Import Recommendation

If the bundled export works on your Tasker version, prefer importing it instead of rebuilding the share flow manually:

1. Import [`integrations/tasker/tailclip_share_sender.xml`](../integrations/tasker/tailclip_share_sender.xml).
2. Set `%TAILCLIP_PC_URL` to your Windows tailnet endpoint, for example `http://100.67.245.84:8080/share`.
3. Set `%TAILCLIP_TOKEN` to the same value as Tailclip's `auth_token`.
4. Test from Android with `Share -> Send to PC`.

## Test From Windows

You can test the inbound Windows endpoint from PowerShell with:

```powershell
powershell -ExecutionPolicy Bypass -File .\integrations\tasker\test-tailclip-windows-endpoint.ps1 `
  -Url "http://100.67.245.84:8080/share" `
  -Token "replace-me" `
  -Content "hello from tasker share"
```

Expected result:

- the script prints an HTTP `2xx` response
- the Windows clipboard is updated

## Notes

- This path is intentionally manual. It works with Gboard because it uses Android's share flow, not background clipboard reads.
- Tailclip dedups repeated inbound text and suppresses the obvious immediate bounce-back case against the last outbound Windows clipboard send.
