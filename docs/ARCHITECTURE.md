# Tailclip Architecture

## Goal

Build a private clipboard sync from a Windows 11 PC to an Android phone over Tailscale.

Phase 1 is intentionally narrow:

- Windows -> Android only
- Text clipboard only
- Automatic and near real-time
- No cloud services
- No user interaction during normal use

Future bidirectional sync is out of scope for now, but the wire format should leave room for it.

## End-State Design

Tailclip has only two moving parts:

- A small Windows background agent written in Go
- A Tasker flow on Android that receives clipboard events and writes them to the phone clipboard

The Windows agent detects clipboard changes, sends them over HTTP through the tailnet, and Tasker applies the received text locally on Android.

No custom server, broker, or cloud relay is involved.

## Data Flow

1. User copies text on Windows.
2. The Windows agent receives an event-driven clipboard change notification.
3. The agent reads the current clipboard text.
4. If the text is empty, non-text, or unchanged from the last successfully sent value, it does nothing.
5. If the text is new, the agent sends an HTTP `POST` to the Android Tasker endpoint over Tailscale.
6. Tasker validates the shared token, extracts the text, and writes it to the Android clipboard.
7. Tasker returns a success response.
8. The Windows agent records that content as the last successful send.

## Components

### Windows agent

Responsibilities:

- Run as a lightweight background executable
- Listen for clipboard changes using Windows clipboard notifications
- Read plain text clipboard content only
- Ignore duplicate clipboard events for unchanged text
- Send clipboard payloads to Android over HTTP
- Log failures without blocking future clipboard updates

Non-goals for the Windows agent in Phase 1:

- Receiving clipboard updates from Android
- Syncing images, files, or rich clipboard formats
- Persistent offline retry queues
- Device discovery

### Android Tasker flow

Responsibilities:

- Expose an HTTP endpoint on a fixed port reachable over Tailscale
- Validate a shared auth token
- Parse the incoming JSON payload
- Write `content` to the Android clipboard
- Return a simple `2xx` response on success

The Android side is intentionally dumb in v1. The Windows agent owns the primary dedup behavior.

## Networking

Transport is plain HTTP over the tailnet.

- No Taildrop
- No public internet exposure
- No separate backend service
- Static target endpoint configured in the Windows agent

Typical endpoint shape:

```text
http://100.x.y.z:8080/clipboard
```

MagicDNS is fine too, but endpoint discovery is not part of the design.

## Request Format

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

## Dedup Rules

The design stays intentionally simple.

- Only send text clipboard values
- Ignore empty clipboard text
- Do not send if the new text matches the last successfully sent text or content hash
- Treat repeated Windows clipboard notifications for the same copied value as duplicates

This is enough for Phase 1. More advanced loop prevention can come later when Android -> Windows exists.

## Reliability Model

Delivery is best effort.

- Send immediately on new clipboard content
- Use a short HTTP timeout
- If the phone is unreachable or Tasker rejects the request, log the failure and move on
- Do not queue unsent clipboard items on disk
- Do not replay old clipboard history

That keeps the agent simple and predictable.

## Configuration

The Windows agent should use a small local config file with:

- `android_url`
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
- Tasker installed and configured
- Battery optimization disabled if needed
- Clipboard write permissions working on the target Android version

## Success Criteria

The implementation is done when:

- Copying text on Windows causes that text to appear on Android automatically
- End-to-end sync is usually under 1 second
- Re-copying the same text does not spam the Android device
- Temporary network failures do not crash the Windows agent
- The setup works without ongoing manual intervention
