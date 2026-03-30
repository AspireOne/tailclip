# Tailclip Architecture

## Goal

Build a bidirectional clipboard sync between a Windows 11 PC and an Android phone over a local network, whether that is Tailscale or plain LAN.

The current scope is intentionally narrow:

- Text clipboard only
- Automatic and near real-time in both directions
- No cloud services
- No user interaction during normal use

## End-State Design

Tailclip has three moving parts:

- A small Windows background agent written in Go
- A Tasker receiver flow on Android that accepts `Windows -> Android` clipboard events over HTTP
- A Tasker sender flow on Android that detects clipboard changes and sends them to the Windows `/share` endpoint

The Windows agent watches the Windows clipboard and sends updates to Android. The Android sender profile watches for clipboard changes with a Tasker `Logcat Entry` trigger, reads the current clipboard text, and posts it back to the Windows agent.

No custom server, broker, or cloud relay is involved.

## Data Flow

### Windows -> Android

1. User copies text on Windows.
2. The Windows agent receives an event-driven clipboard change notification.
3. The agent reads the current clipboard text.
4. If the text is empty, non-text, or unchanged from the last successfully sent value, it does nothing.
5. If the text is new, the agent sends an HTTP `POST` to the Android Tasker receiver endpoint over the local network.
6. Tasker validates the shared token, extracts the text, and writes it to the Android clipboard.
7. Tasker returns a success response.
8. The Windows agent records that content as the last successful send.

### Android -> Windows

1. User copies text on Android.
2. Tasker sees the clipboard-related logcat entry.
3. The sender profile runs `Get Clipboard`.
4. If the clipboard is empty or matches `%PREV_CLIP`, Tasker stops without sending.
5. Otherwise Tasker sends `POST /share` with the clipboard text as `text/plain` to the Windows agent over the local network.
6. The Windows agent validates the token and writes the received text to the Windows clipboard.
7. On success, Tasker stores the sent value in `%PREV_CLIP`.

## Components

### Windows agent

Responsibilities:

- Run as a lightweight background executable
- Listen for clipboard changes using Windows clipboard notifications
- Read plain text clipboard content only
- Ignore duplicate clipboard events for unchanged text
- Send clipboard payloads to Android over HTTP
- Expose an authenticated HTTP endpoint for Android `->` Windows sends
- Apply received Android clipboard text to the Windows clipboard
- Log failures without blocking future clipboard updates

Non-goals for the Windows agent:

- Syncing images, files, or rich clipboard formats
- Persistent offline retry queues
- Device discovery

### Android Tasker receiver flow

Responsibilities:

- Expose an HTTP endpoint on a fixed port reachable from the Windows PC
- Validate a shared auth token
- Parse the incoming JSON payload
- Write `content` to the Android clipboard
- Return a simple `2xx` response on success

### Android Tasker sender flow

Responsibilities:

- Listen for clipboard-related logcat events
- Read the current clipboard content with Tasker's `Get Clipboard`
- Skip empty clipboard values
- Skip values matching `%PREV_CLIP`
- Send the current text to the Windows `/share` endpoint with bearer auth

Constraints:

- Requires Tasker logcat access
- Requires background clipboard access granted over ADB on modern Android builds
- Depends on Tasker surviving background execution limits

## Networking

Transport is plain HTTP over the local network.

- No Taildrop
- No relay or broker service
- No separate backend service
- Static target endpoint configured in the Windows agent

Typical Android receiver endpoint:

```text
http://100.x.y.z:8080/clipboard
```

Typical Windows sender target:

```text
http://100.x.y.z:8080/share
```

Tailscale IPs or MagicDNS names are fine, and plain LAN IPs or local DNS names work too. Endpoint discovery is not part of the design.

## Request Format

### Windows -> Android

The Windows agent sends a JSON payload like this:

```json
{
  "id": "evt_abc123",
  "content": "hello from windows",
  "content_hash": "sha256:...",
  "source_device_id": "windows-laptop",
  "created_at": "2026-03-29T22:10:00Z"
}
```

Fields:

- `id`: unique per clipboard event
- `content`: clipboard text to apply on Android
- `content_hash`: dedup helper derived from the exact text content
- `source_device_id`: sender identity for future loop prevention
- `created_at`: UTC timestamp for logging and debugging

HTTP request requirements:

- Method: `POST`
- Path: `/clipboard`
- Header: `Content-Type: application/json`
- Header: auth token, preferably `Authorization: Bearer <token>`
- Success: any `2xx` response

### Android -> Windows

The Tasker sender sends:

- Method: `POST`
- Path: `/share`
- Header: `Content-Type: text/plain`
- Header: `Authorization: Bearer <token>`
- Body: raw clipboard text
- Success: any `2xx` response

## Dedup Rules

The design stays intentionally simple.

- Windows sender:
  - only sends text clipboard values
  - ignores empty clipboard text
  - does not send if the new text matches the last successfully sent text or content hash
  - treats repeated Windows clipboard notifications for the same copied value as duplicates

- Android sender:
  - only sends plain text clipboard values Tasker can read
  - ignores empty clipboard text
  - uses `%PREV_CLIP` to suppress immediate duplicates after a successful send

## Reliability Model

Delivery is best effort.

- Send immediately on new clipboard content in either direction
- Use a short HTTP timeout
- If either side is unreachable or rejects the request, log or notify and move on
- Do not queue unsent clipboard items on disk
- Do not replay old clipboard history

That keeps the agent simple and predictable.

## Configuration

The Windows agent should use a small local config file with:

- `android_url`
- `windows_listen_addr`
- `auth_token`
- `device_id`
- `http_timeout_ms`

Optional settings:

- `log_level`

If `device_id` is missing, the agent may derive a stable default locally.

## Deployment

### Windows

- Single executable
- Runs in background
- Starts on login once the core path is stable

### Android

- Tailscale connected
- Or both devices reachable over the same LAN
- Tasker installed and configured
- Battery optimization disabled if needed
- Clipboard write permissions working on the target Android version
- Sender profile granted `READ_LOGS`
- Sender profile granted background clipboard access over ADB

## Success Criteria

The implementation is done when:

- Copying text on Windows causes that text to appear on Android automatically
- Copying text on Android causes that text to appear on Windows automatically
- End-to-end sync is usually under 1 second
- Re-copying the same text does not spam either device
- Temporary network failures do not crash the Windows agent
- The setup works without ongoing manual intervention
