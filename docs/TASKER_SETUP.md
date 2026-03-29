# Tailclip Tasker Setup

This document covers the Android side of Tailclip.

Phase 1 is intentionally simple:

- Windows sends clipboard text to Android over Tailscale.
- Tasker exposes an HTTP endpoint on the phone.
- Tasker validates the auth token.
- Tasker reads `content` from the JSON body.
- Tasker writes that text to the Android clipboard.

The expected wire contract comes from [ARCHITECTURE.md](./ARCHITECTURE.md) and the sender implementation in `internal/transport/http.go`.

## Prerequisites

- Android phone with `Tasker` installed
- `Tailscale` installed on the phone and signed into the same tailnet as the Windows PC
- Tasker allowed to run in the background
- Battery optimization disabled for Tasker if your device is aggressive about killing background apps
- Clipboard writes working on your Android version

Recommended:

- Give the phone a stable Tailscale identity with either:
  - its tailnet IP, for example `100.x.y.z`
  - or a MagicDNS hostname

## What Windows Sends

The Windows agent sends:

- `POST /clipboard`
- `Content-Type: application/json`
- `Authorization: Bearer <token>`

Example body:

```json
{
  "id": "evt_abc123",
  "content": "hello from windows",
  "content_hash": "sha256:...",
  "source_device_id": "windows-laptop",
  "created_at": "2026-03-29T22:10:00Z"
}
```

Tasker only needs `content` for v1.

## Tasker Profile

Create a profile:

1. Open `Tasker`.
2. Go to `Profiles`.
3. Add a new `Event`.
4. Choose `Net > HTTP Request`.
5. Set:
   - `Port`: `8080`
   - `Method`: `POST`
   - `Path`: `/clipboard`
   - `Quick Response`: leave empty
6. Attach a new task named `Tailclip Receive Clipboard`.

This makes Tasker listen on:

```text
http://PHONE_TAILSCALE_IP:8080/clipboard
```

Important:

- Keep the path exactly `/clipboard`.
- Keep the method as `POST`.
- Do not create a second Tasker `HTTP Request` event on the same port and path.

## Task Logic

The task should do this:

1. Check the `Authorization` header.
2. If it is not `Bearer <your-token>`, return `401` or `403`.
3. Parse the request body as JSON.
4. Read `content`.
5. If `content` is missing or empty, return `400`.
6. Otherwise set the Android clipboard to `content`.
7. Return `200`.

### Suggested Tasker Actions

These are the actions to add to the `Tailclip Receive Clipboard` task.

#### 1. Copy the headers/body to local variables

Add:

- `Variable Set`
  - `Name`: `%tailclip_headers`
  - `To`: `%http_request_headers()`
- `Variable Set`
  - `Name`: `%tailclip_body`
  - `To`: `%http_request_body`
  - `Structure Output`: `On`

If your Tasker build does not parse `%tailclip_body.content` directly, add:

- `Set Variable Structure Type`
  - `Name`: `%tailclip_body`
  - `Type`: `JSON`

#### 2. Validate the auth header

Tasker exposes incoming headers in `%http_request_headers()` as `key:value` strings.

You need to confirm one of these matches:

- `Authorization:Bearer YOUR_TOKEN`
- or `Authorization: Bearer YOUR_TOKEN`

Practical options:

- `Array Process` on `%http_request_headers()` to search for the header string
- or `Simple Match/Regex`
- or `Variable Search Replace` in regex mode

Recommended logic:

- Search `%http_request_headers()` for `(?i)^Authorization:\s*Bearer YOUR_TOKEN$`
- If there is no match:
  - `HTTP Response`
    - `Request ID`: `%http_request_id`
    - `Status Code`: `401`
    - `Type`: `Text`
    - `Body`: `unauthorized`
  - `Stop`

#### 3. Validate the JSON payload

Read:

- `%tailclip_body.content`

If `%tailclip_body.content` is not set or is empty:

- `HTTP Response`
  - `Request ID`: `%http_request_id`
  - `Status Code`: `400`
  - `Type`: `Text`
  - `Body`: `missing content`
- `Stop`

#### 4. Set the phone clipboard

Add:

- `Set Clipboard`
  - `Text`: `%tailclip_body.content`

#### 5. Return success

Add:

- `HTTP Response`
  - `Request ID`: `%http_request_id`
  - `Status Code`: `200`
  - `Type`: `Text`
  - `Body`: `ok`

## Minimal Flow Summary

If you prefer the shortest possible version, the task can be summarized as:

1. `If` auth header is wrong -> `HTTP Response 401` -> `Stop`
2. `If` JSON `content` is empty -> `HTTP Response 400` -> `Stop`
3. `Set Clipboard` to the parsed `content`
4. `HTTP Response 200`

## Windows Config

Point the Windows agent at the phone endpoint, for example:

```json
{
  "android_url": "http://100.101.102.103:8080/clipboard",
  "auth_token": "replace-me",
  "device_id": "windows-laptop",
  "enabled": true,
  "http_timeout_ms": 3000
}
```

Use the same token in:

- the Windows config `auth_token`
- the Tasker auth check

## Test The Phone Before Running The Agent

Use the PowerShell helper in [`integrations/tasker/test-tailclip-endpoint.ps1`](../integrations/tasker/test-tailclip-endpoint.ps1).

Example:

```powershell
powershell -ExecutionPolicy Bypass -File .\integrations\tasker\test-tailclip-endpoint.ps1 `
  -Url "http://100.101.102.103:8080/clipboard" `
  -Token "replace-me" `
  -Content "hello from manual test"
```

Expected result:

- the script prints an HTTP `2xx` response
- the phone clipboard is updated

## Import / Export Options

Tasker supports import/export through:

- `.tsk.xml` for a task
- `.prf.xml` for a profile
- `.prj.xml` for a whole project
- TaskerNet links

So yes, the Android side can be defined as a file instead of being recreated forever in the UI.

This repo now includes an importable profile export:

- [`integrations/tasker/Tailclip.prf.xml`](../integrations/tasker/Tailclip.prf.xml)

After import, set a Tasker global variable named `%TAILCLIP_TOKEN` to the same value as the Windows `auth_token`.

## Practical Recommendation

Tasker XML is importable, but it is not a pleasant authoring format and is version-sensitive enough that hand-writing it from scratch is brittle.

The pragmatic workflow is:

1. Import [`integrations/tasker/Tailclip.prf.xml`](../integrations/tasker/Tailclip.prf.xml).
2. Set `%TAILCLIP_TOKEN` in Tasker to your shared secret.
3. Test with [`integrations/tasker/test-tailclip-endpoint.ps1`](../integrations/tasker/test-tailclip-endpoint.ps1).
4. If you customize the Tasker flow on-device, export the updated XML back into the repo.

For this repo, a standalone exported `.prj.xml` is the best end-state artifact once one clean on-device export exists.

## Troubleshooting

### Windows reports non-2xx

Check:

- phone is online in Tailscale
- port `8080` is reachable
- path is exactly `/clipboard`
- token matches exactly
- Tasker actually returns an `HTTP Response`

### Request reaches Tasker but clipboard is not updated

Check:

- `%tailclip_body.content` is populated
- Android clipboard restrictions on your device
- Tasker permissions

### Tasker says it cannot find `%http_request_id`

Keep the `HTTP Response` action inside the request-handling task and avoid doing unnecessary delayed/background work before responding.

### Duplicate profile collisions

Tasker only considers one `HTTP Request` event for a given port/path pair. If another profile uses `8080` and `/clipboard`, remove the conflict.
