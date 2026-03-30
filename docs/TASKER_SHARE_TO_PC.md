# Tailclip Tasker Sender Setup

This document covers the Android `->` Windows sender profile shipped in this repo:

- [`integrations/tasker/tailclip_sender.prf.xml`](../integrations/tasker/tailclip_sender.prf.xml)

This profile is a background clipboard sender. It is not the old manual share-sheet flow.

The actual behavior of the exported profile is:

1. Tasker listens for a `Logcat Entry` that indicates the Android clipboard changed.
2. When that trigger fires, Tasker runs `Get Clipboard`.
3. Tasker ignores empty results.
4. Tasker ignores the clipboard value if it matches `%PREV_CLIP`.
5. Tasker sends the clipboard text to Tailclip on Windows using `POST %TAILCLIP_PC_URL`.
6. If the send succeeds, Tasker stores the sent value in `%PREV_CLIP`.

Because of that design, this profile requires extra Android privileges beyond normal Tasker setup.

## What This Profile Requires

Required:

- Tailclip running on Windows with `windows_listen_addr` configured
- Tasker installed on Android
- Network connectivity between both devices, either over Tailscale or the same LAN
- `%TAILCLIP_PC_URL` and `%TAILCLIP_TOKEN` configured in Tasker
- Tasker allowed to read logcat
- Tasker allowed to read the clipboard while in the background
- Battery optimization disabled for Tasker

Important:

- The logcat permission is required because the profile trigger is `Logcat Entry`.
- The background clipboard permission is required because the task uses `Get Clipboard` after the trigger fires.
- Without both grants, the sender profile will either never trigger or trigger and then fail to read the clipboard.

## ADB Setup

Modern Android versions do not reliably allow this flow with in-app permissions alone. Plan on using ADB once during setup.

### 1. Enable USB debugging

- Enable Developer Options on the phone
- Enable USB debugging
- Connect the phone to a computer with `adb`
- Approve the RSA fingerprint prompt on the phone

### 2. Grant Tasker logcat access

Run:

```powershell
adb shell pm grant net.dinglisch.android.taskerm android.permission.READ_LOGS
```

This is what allows Tasker's `Logcat Entry` profile to observe clipboard-related logcat messages.

### 3. Grant Tasker background clipboard access

Run:

```powershell
adb shell cmd appops set net.dinglisch.android.taskerm READ_CLIPBOARD allow
```

If your device does not support `cmd appops`, try:

```powershell
adb shell appops set net.dinglisch.android.taskerm READ_CLIPBOARD allow
```

This is what allows Tasker's `Get Clipboard` action to work even when Tasker is not the foreground app.

### 4. Keep Tasker alive

Also check:

- battery optimization is disabled for Tasker
- Tasker is set to unrestricted background battery usage if your ROM exposes that option
- vendor-specific app sleeping / app freezer features are disabled for Tasker

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

`windows_listen_addr` is the local address Tailclip binds on the PC. Leave it empty to disable inbound Android sends entirely.

Tasker should point to the PC's reachable URL, for example:

```text
http://100.67.245.84:8080/share
```

## Tasker Globals

Set these after import:

- `%TAILCLIP_PC_URL`: full Windows endpoint URL, for example `http://100.67.245.84:8080/share`
- `%TAILCLIP_TOKEN`: the same bearer token used in Tailclip config

## What The Imported Profile Actually Does

The profile name in the export is:

- `Logcat Detect Clip Change -> Tailclip Send`

The imported trigger is:

- Tasker `Logcat Entry`
- Component/tag: `ClipboardService`
- Text match: `op=30 result=true`

The task that runs after the trigger:

- reads the current clipboard with `Get Clipboard`
- shows a toast if clipboard read fails
- runs a small JavaScript block to:
  - copy `%cl_text` into `%tailclip_text`
  - drop the event if the clipboard is empty
  - drop the event if `%tailclip_text == %PREV_CLIP`
- sends `%tailclip_text` to `%TAILCLIP_PC_URL`
- includes:
  - `Content-Type: text/plain`
  - `Authorization: Bearer %TAILCLIP_TOKEN`
- stores `%PREV_CLIP = %tailclip_text` only after an HTTP `2xx`
- shows a failure notification if the request is non-`2xx`

## Windows Endpoint Contract

Tailclip listens for:

- `POST /share`
- `Authorization: Bearer <token>`

Supported request body for this Tasker sender:

- `text/plain`
  - request body is the raw text to place on the Windows clipboard

Tailclip rejects:

- wrong token with `401`
- wrong method with `405`
- missing or empty content with `400`

## Import Recommendation

If the bundled export works on your Tasker version, use it directly:

1. Import [`integrations/tasker/tailclip_sender.prf.xml`](../integrations/tasker/tailclip_sender.prf.xml).
2. Set `%TAILCLIP_PC_URL`.
3. Set `%TAILCLIP_TOKEN`.
4. Apply the ADB grants for `READ_LOGS` and `READ_CLIPBOARD`.
5. Disable battery optimization for Tasker.
6. Copy text on Android and confirm the Windows clipboard updates.

## Test From Windows

You can test the inbound Windows endpoint from PowerShell with:

```powershell
powershell -ExecutionPolicy Bypass -File .\integrations\tasker\test-tailclip-windows-endpoint.ps1 `
  -Url "http://100.67.245.84:8080/share" `
  -Token "replace-me" `
  -Content "hello from tasker sender"
```

Expected result:

- the script prints an HTTP `2xx` response
- the Windows clipboard is updated

This only validates the Windows endpoint. It does not validate Android logcat or clipboard permissions.

## Troubleshooting

### Clipboard changes on Android do nothing

Check:

- the sender profile is enabled
- `adb shell pm grant net.dinglisch.android.taskerm android.permission.READ_LOGS` succeeded
- Tasker is still allowed to run in the background
- your Android build still emits `ClipboardService` logcat entries for clipboard changes

### The profile triggers but nothing is sent

Check:

- `adb shell cmd appops set net.dinglisch.android.taskerm READ_CLIPBOARD allow` succeeded
- the clipboard currently contains plain text
- `%TAILCLIP_PC_URL` points to the Windows `/share` endpoint
- `%TAILCLIP_TOKEN` matches Tailclip's `auth_token`

### Tasker shows `Get clipboard failed`

That means the logcat trigger worked, but Tasker could not read the clipboard value. The usual cause is missing background clipboard access or the ROM resetting the clipboard app-op.

### You only want a manual Android send action

This exported profile is not that. If you want a pure manual share-sheet flow, build a separate `Received Share` Tasker profile and post the shared text to the same Windows `/share` endpoint.
