# Tailclip Windows Tray UI Plan

## Summary

Add a Windows-only tray-based shell around Tailclip so the app runs as a background utility with a compact native settings window. Keep the existing JSON config file as the source of truth, but stop requiring users to edit it manually. The single Windows process will own the tray icon, settings popup, config persistence, startup toggle, and the clipboard sync loop.

## Key Changes

### App shape and runtime

- Replace the current CLI-first Windows entrypoint with a Windows GUI app that starts a tray icon immediately and does not show a console window in normal release builds.
- Use a single-process design: tray/UI and clipboard sync run in one executable, with the sync loop managed in a restartable goroutine controlled by the UI layer.
- Introduce a small runtime controller with three states: `needs_config`, `running`, `paused`.
- On startup:
  - create tray icon and menu
  - load config from the existing default path or `-config`
  - if config is valid and `enabled=true`, start the sync loop
  - if config is missing or invalid, keep the tray alive and show status as setup/error instead of exiting the process
- Clicking the tray icon opens or focuses a compact native settings window anchored near the tray area when practical; if exact anchoring is unreliable, open it as a small centered utility window.
- Closing the settings window hides it; only `Quit` exits the app.

### UI and Windows integration

- Implement the Windows UI with a native Go Windows toolkit and tray support in the same stack. Target `walk` so text inputs, checkboxes, tray icon, and window lifecycle are all native Windows controls.
- Tray menu/actions:
  - `Open Settings`
  - `Pause Syncing` / `Resume Syncing`
  - `Quit`
- Settings window contents:
  - read-only config path display
  - editable `Android URL`
  - editable `Auth token`
  - editable `Device ID`
  - checkbox `Enabled`
  - checkbox `Start on login`
  - `Save`
  - `Open Config Folder`
  - status text for current runtime state / last validation error
- `Start on login` is backed by the current-user Windows Run key (`HKCU\Software\Microsoft\Windows\CurrentVersion\Run`), not by JSON config.
- `Enabled` is persisted in config and controls whether the sync loop should run. `Quit` only exits the app; it does not change `enabled`.

### Config and behavior changes

- Extend the JSON schema with optional `enabled`.
- Default `enabled` to `true` when missing, for backward compatibility.
- Keep existing fields and defaults unchanged otherwise.
- Add config save support in the config package:
  - serialize the current settings back to the existing config path
  - create parent directory if missing
  - write atomically via temp file + rename
- Saving from the UI:
  - validate fields with the existing config validation rules before commit
  - on success, persist JSON, update startup registry state, and restart the sync loop in-place
  - on validation failure, keep the window open and show the error inline
- The sync engine remains the existing app loop, but it must be controllable:
  - start with context
  - stop on pause, disable, save/reconfigure, or quit
  - restart cleanly after config edits
- Add file logging under `%APPDATA%\tailclip\logs\tailclip.log` for GUI builds so diagnostics are not lost when no console is present.

## Public Interfaces / Types

- Config file schema adds:
  - `enabled: boolean` optional, defaults to `true`
- Internal config API additions:
  - save/write function for persisting config
  - helper to expose the resolved config path for UI display
- Internal app/runtime additions:
  - controller interface or struct for `Start`, `Stop`, `Reload`, `Status`
- No network payload or Android-side contract changes.

## Test Plan

- Config tests:
  - loading old config without `enabled` defaults to enabled
  - saving creates the config directory and writes valid JSON
  - invalid field values are rejected before save
- Runtime/controller tests:
  - valid enabled config starts the loop
  - disabled config does not start the loop
  - saving new settings restarts the loop with updated config
  - pause/resume toggles runtime state without losing saved config
- Windows integration scenarios:
  - missing config still shows tray and settings window
  - invalid config does not crash the app
  - `Start on login` adds/removes the HKCU Run entry correctly
  - `Quit` exits cleanly and stops the watcher
- Manual acceptance:
  - user can launch app, click tray, fill in settings, save, and sync works
  - user can disable syncing without quitting
  - user can quit from tray
  - user can relaunch and retained settings are loaded

## Assumptions and Defaults

- The existing config path remains the canonical settings location.
- v1 is Windows-only for the tray/UI layer; non-Windows builds continue to compile without this UI.
- The compact settings surface is a real small native window, not editable tray-menu rows.
- No onboarding wizard, rich status history, or connection test button in v1.
- Logging is file-based for GUI mode; no separate log viewer is included in this iteration.
