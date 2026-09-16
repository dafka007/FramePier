# VidDock — Implementation Plan

Chrome/Brave MV3 extension → Native Messaging → local Windows helper → yt-dlp (→ ffmpeg when needed). Downloads only content the user is permitted to download. No DRM/paywall/auth bypass, no shell interface.

## 1. Architecture

```
[Extension popup UI]
   |  chrome.runtime.connectNative("viddock.helper")
   v
[Native Messaging host manifest (.reg / install script)]
   |  stdio: 4-byte LE length prefix + UTF-8 JSON (NM protocol)
   v
[VidDock Helper (single-file Windows exe, Python/PyInstaller)]
   |  subprocess.run([...], shell=False), allowlisted args only
   v
[yt-dlp.exe] -- ffmpeg.exe on PATH for merging when needed
```

- Extension is UI + transport only. All file I/O and process execution happens in the helper.
- Helper never accepts free-form commands; it maps a fixed set of typed JSON commands to fixed yt-dlp argument vectors.
- No cookies, no auth headers, no `--username`/`--password`, no extractor args that bypass restrictions.

## 2. Project Folders / Files

```
VidDock/
├─ PLAN.md
├─ helper/
│  ├─ pyproject.toml            # deps: none beyond stdlib (yt-dlp.exe bundled separately)
│  ├─ src/viddock_helper/
│  │  ├─ __main__.py            # entry: NM stdio loop
│  │  ├─ protocol.py            # frame read/write, message dataclasses, strict validation
│  │  ├─ commands.py            # command allowlist + dispatch (ping, probe, download)
│  │  ├─ validators.py          # URL / filename / path validation, traversal guards
│  │  ├─ runner.py              # yt-dlp subprocess management (no shell), progress parsing
│  │  └─ config.py              # fixed base dir: %LOCALAPPDATA%\VidDock\downloads
│  └─ tests/                    # pytest: validators, protocol, dispatch negatives
├─ extension/
│  ├─ manifest.json             # MV3, permissions: ["nativeMessaging"] only
│  ├─ popup.html                # minimal static markup (no inline JS)
│  ├─ popup.css
│  └─ js/
│     ├─ popup.js               # UI wiring, DOM via textContent/createElement only
│     ├─ native.js              # port connect, send/receive, timeout handling
│     └─ validate.js            # strict inbound message validation (schema + types)
├─ tools/
│  ├─ install.ps1               # writes NM host manifest to HKCU for Chrome + Brave
│  ├─ uninstall.ps1
│  └─ viddock.helper.json       # host manifest template (allowed_origins, path)
└─ docs/
   └─ PROTOCOL.md               # message schema reference
```

## 3. Native Messaging Protocol

- Transport: NM stdio framing — `uint32 LE length` + UTF-8 JSON payload. Max frame size 1 MB; reject larger.
- All messages are objects with a literal `type` field. Unknown `type`/`command` → error response, connection stays open (no crash).

Request:
```json
{"id": "uuid", "command": "ping"}
{"id": "uuid", "command": "probe",    "params": {"url": "https://www.youtube.com/watch?v=..."}}
{"id": "uuid", "command": "download", "params": {"url": "...", "format_id": "...", "filename": "safe-name"}}
```

Response:
```json
{"id": "uuid", "ok": true,  "data": {...}}
{"id": "uuid", "ok": false, "error": {"code": "INVALID_URL", "message": "..."}}
```

Progress (helper-initiated, same `id`):
```json
{"id": "uuid", "event": "progress", "percent": 42.5, "speed": "...", "status": "downloading|merging|done|error"}
```

Commands: `ping`, `probe` (yt-dlp `-J` metadata + format list), `download`. Nothing else exists; dispatch is a closed dict lookup.

## 4. Helper Responsibilities

- Read/write NM frames on stdio; enforce frame size limit and JSON parse errors → structured error, never traceback to browser.
- Validate every request: command in allowlist, param types/shapes exact (no extra keys), URL scheme `https`, host in allowlist (`youtube.com`, `youtu.be`), filename matches `^[A-Za-z0-9._-]{1,80}$`.
- Resolve output path as `base_dir / filename.ext`; verify with `os.path.realpath` + `commonpath` that result stays inside base dir. Base dir is fixed by the helper (user cannot set arbitrary dirs in v1).
- Run yt-dlp via `subprocess.run/Popen` with an explicit argument list built from validated values only (`shell=False`, no env passthrough of secrets, `--no-playlist` unless explicitly requested and valid).
- Parse progress from `--newline --progress-template` output; forward as `event: progress`.
- ffmpeg is invoked by yt-dlp itself when merging; helper only ensures `ffmpeg.exe` exists on PATH at probe time and reports a clear error if missing.
- Log to `%LOCALAPPDATA%\VidDock\helper.log` (no URLs of private content, no secrets).

## 5. Extension Responsibilities

- Popup UI: URL input → Probe button → format list → Download button → progress bar + status line.
- `native.js`: open port on popup open, close on unload; send framed JSON via `port.postMessage`; enforce per-request timeout (e.g., 10 s for ping/probe, none for download except heartbeat).
- `validate.js`: every inbound message checked before use — exact shape, string/number types, bounded lengths. Anything unexpected is dropped and surfaced as a generic error state.
- DOM safety: build UI with `document.createElement` + `textContent`; no `innerHTML`, no `eval`, no remote code; CSP left at MV3 default.
- Permissions: `"nativeMessaging"` only. No host permissions, no storage beyond optional `storage.local` for last-used settings (no URLs of private content persisted).

## 6. Validation / Security Rules

| Rule | Where enforced |
|---|---|
| Closed command set; unknown command → `UNKNOWN_COMMAND` error | helper `commands.py`, extension `validate.js` |
| Strict JSON: exact keys, types, max lengths; no extra fields | both sides |
| URL: `https:` only, host allowlist, no credentials in URL (`user:pass@`) | helper + extension pre-check |
| Filename: regex whitelist, no separators/dots tricks, ≤ 80 chars | helper (authoritative) |
| Path traversal: realpath containment check inside fixed base dir; reject `..`, absolute paths, symlinks escaping base | helper |
| No shell: `shell=False` always; args are a fixed list, values validated before interpolation into argv | helper `runner.py` |
| No eval/exec/`Function()` anywhere in either codebase (lint rule) | both |
| No cookie/auth support: no `--cookies`, no credential flags, no session extraction — by construction, not by flag | helper |
| Minimal permissions: `nativeMessaging` only; no remote JS; MV3 default CSP | extension manifest |
| Frame size cap + JSON parse guard on stdio loop | helper |
| No unsafe DOM: `textContent`/createElement only | extension |

## 7. Test Strategy

- **Unit (pytest, helper):** validators (URL/filename/path traversal matrix incl. `..`, absolute, UNC, symlink cases), protocol framing (truncated/huge/malformed frames), dispatch negatives (unknown command, extra keys, wrong types).
- **Protocol integration:** fake NM host script speaking the same framing; extension-side validator tested against recorded fixtures of valid + hostile messages.
- **Runner tests:** with a stub `yt-dlp.exe` (batch/ps1) that emits canned progress lines — verify arg vectors are exactly as expected and progress events flow.
- **Security negative suite:** every rule in §6 has at least one failing-input test asserting the structured error, not a crash.
- **Manual E2E checklist:** install script → Chrome + Brave → ping/probe/download of a user-permitted (e.g., Creative Commons) video; verify file lands in base dir; verify cancel/close behavior.

## 8. Packaging / Setup Strategy

- Helper: Python 3.11, PyInstaller `--onefile` → `viddock-helper.exe`; bundle `yt-dlp.exe` next to it (pinned version); ffmpeg expected on PATH (documented).
- NM host manifest (`viddock.helper.json`) points at the exe; `install.ps1` writes it under `HKCU\Software\Google\Chrome\NativeMessagingHosts\VidDock.Helper` (same key works for Brave) and copies files to `%LOCALAPPDATA%\VidDock`. `uninstall.ps1` reverses both.
- Extension: load unpacked from `extension/` during dev; zip the folder for store submission later. No build step required in v1.
- Versioning: single version string mirrored in manifest + helper `--version`; protocol changes bump a `proto` field checked on `ping`.

## 9. Ordered Implementation Stages

1. **Protocol spec** — finalize message schemas in `docs/PROTOCOL.md`; agree error codes.
2. **Helper core** — NM stdio loop, framing, strict validation, closed dispatch with only `ping` working; unit tests green.
3. **Validators + path safety** — URL/filename/path rules with full negative test matrix.
4. **yt-dlp integration** — `probe` (metadata/formats) and `download` (progress events), stub-based runner tests, then real yt-dlp smoke test.
5. **Extension UI** — manifest, popup, port layer, inbound validation; manual ping/probe round-trip in Chrome + Brave.
6. **Security hardening pass** — run §6 checklist against code, add any missing negative tests, lint for eval/innerHTML/shell patterns.
7. **Packaging** — PyInstaller build, install/uninstall scripts, host manifest registration verified on both browsers.
8. **E2E + docs** — full manual checklist, README with setup steps and the permitted-use statement.
