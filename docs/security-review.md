# v0.2.1 security review

The 0.2.1 folder action uses `ShellExecuteExW` with the fixed `open` verb, `SW_SHOWNORMAL`, a validated existing local directory, and no parameter string. A COM apartment and `SEE_MASK_NOASYNC` keep the shell request alive until dispatch completes; `SEE_MASK_FLAG_NO_UI` leaves error reporting to FramePier. Background tool execution retains `CREATE_NO_WINDOW` and `HideWindow`. No new protocol command or caller-supplied folder path is accepted. Path checks are repeated at launch, including reparse-point rejection, and technical failures are logged with the user-profile prefix redacted.

The 0.2.0 source expansion keeps extraction and downloads behind the same fixed local protocol. Twitch input is accepted only from official HTTPS hosts and only as a validated public clip slug or numeric completed-VOD ID. Live channel pages, collections, directories, custom ports, credentials, encoded path tricks, and non-media Twitch pages are rejected before a child process starts. The helper never passes cookie, browser-profile, OAuth, or authentication arguments to yt-dlp.

Twitch metadata remains untrusted. Titles, creator names, thumbnails, and errors are rendered with `textContent`; thumbnails are limited to Twitch's `static-cdn.jtvnw.net` over HTTPS. Exact format IDs are selected only from metadata returned by the helper's own yt-dlp query, not from extension input. VOD size arithmetic and progress counters use 64-bit integers. Preflight space checks query the destination and temporary-processing volumes and reject clearly insufficient space without exposing a generic filesystem command.

The 0.1.3 integration repair writes a fixed manifest schema and fixed host name; no manifest fields, registry keys, executable paths, or extension origins come from extension messages. Repair validates the installed helper path, requires the single stable allowed origin, registers only detected supported browsers in HKCU, queries both registry views, and performs a framed ping against the fixed installed helper. Native Messaging diagnostics are capped and stored only in extension-local storage.

The 0.1.2 lifecycle change exposes no new Native Messaging command. Installer lifecycle operations are command-line-only, accept only an absolute local path whose basename is exactly `FramePierHelper.exe`, reject root and reparse-point parents, and verify every PID using `QueryFullProcessImageNameW` before forced termination. yt-dlp and FFmpeg are never terminated by a system-wide name match: each download tree belongs to its launching helper through a Windows kill-on-close Job Object. Native Messaging registration is removed only during the replacement window and restored on setup rollback.

The 0.1.4 resolution change accepts only a validated numeric `widthxheight` selection within fixed bounds. The helper matches those dimensions against metadata it obtained directly from yt-dlp; it still selects executables and constructs every yt-dlp/FFmpeg argument itself. Labels are derived from numeric dimensions and cannot introduce command arguments or HTML.

The 0.1.5 folder picker is a single fixed Native Messaging command with no caller-supplied path or generic filesystem operation. It opens Windows' folder-only Common Item Dialog at the helper's current setting, returns one local filesystem path, and does not save on selection or cancellation. Saving repeats the existing path restrictions, creates the directory if needed, and verifies write access with a removed private probe file.

Automatic extension activation accepts only strict three-part numeric versions read by the fixed helper from the installer's `{app}\package-version.json` marker. The marker has a closed JSON schema with fixed product and extension ID values and sits outside Chromium's cached extension package. A webpage cannot call the Native Messaging host, and the extension exposes no `externally_connectable` entry. Reload attempts use Chromium's own `runtime.reload()` API, are limited to one attempt for each running/target version pair, and execute no installer-, webpage-, or message-supplied code. Diagnostic events accept only fixed event/browser names and validated versions.

Completed 2026-09-05 on the Windows release build.

## Results

- Secret scan: no API keys, passwords, access tokens, private signing keys, personal browser profiles, or machine-specific user paths found in publishable source.
- Extension permissions: `activeTab`, `nativeMessaging`, and `storage`; no `<all_urls>`, history, cookies, downloads, or scripting permission.
- Host permissions: explicit official YouTube, `youtu.be`, and Twitch web/clip hosts only; no `<all_urls>`.
- CSP: local scripts only, no eval/inline/remote script, objects/base/frame ancestors disabled, thumbnail images limited to ytimg and Twitch's static CDN over HTTPS.
- XSS: untrusted title/channel/URL/error fields use `textContent`; no `innerHTML` assignment.
- Native Messaging: fixed origin, helper-side origin recheck, 1 MiB frame limit, strict field types, unknown fields and command-inappropriate fields rejected.
- Command execution: every process uses an executable selected by the helper and a discrete argument array. No shell, `cmd /c`, PowerShell, arbitrary executable, raw yt-dlp option, or environment input is accepted from a browser message.
- URL handling: HTTPS-only allowlist with canonical YouTube IDs, Twitch clip slugs, or numeric Twitch VOD IDs; `javascript:`, `data:`, `file:`, localhost, credentials, alternate ports, lookalike hosts, playlists, malformed values, Twitch live/channel pages, and mismatched IDs rejected.
- Filesystem: absolute local output directory required; roots, UNC/device paths, traversal segments, controls, overlong paths, and existing reparse points rejected. Fixed output templates use yt-dlp Windows sanitization and length limits. Final output must resolve beneath the configured folder and be a non-empty regular file.
- Process/resource control: maximum two active jobs. Every child command is created directly without a shell and uses Windows `CREATE_NO_WINDOW` plus `HideWindow`; redirected stdout/stderr and exit status remain available. Cancellation targets only the PID tree recorded on the FramePier job before cancelling its context. Long downloads have no arbitrary wall-clock timeout.
- Large-file handling: byte counters and estimates use 64-bit integers; VOD progress is monotonic; destination and processing-volume free space are checked before a known-size VOD download starts.
- Overwrite behavior: `--no-overwrites`; video filenames include video ID and actual height, preventing a different requested quality from silently reusing another quality.
- Logging: UTC timestamps, helper/job/version/action results, source type, validated media ID, and redacted diagnostics; no cookies, credentials, OAuth tokens, signed media URLs, or full requested page URLs.
- Updates: build downloads only official Go/yt-dlp endpoints and FFmpeg from the provider linked by ffmpeg.org. Go and yt-dlp use publisher checksums; FFmpeg 8.1.1 is pinned to the SHA-256 published in the Windows Package Manager manifest. Runtime updating is the fixed `yt-dlp.exe -U` operation.
- Installer: per-user Inno Setup registration is limited to FramePier's detected/selected Native Messaging keys. Repair detects newly installed supported browsers and only rewrites those same restricted keys. No extension force-install policy, browser preference, or unrelated registry setting is created.

## Tools run

- `go fmt`
- `go vet ./...`
- `go test ./...`
- `govulncheck -mode=binary dist/helper/FramePierHelper.exe` — no vulnerabilities found
- Node `--check` for every extension script
- Manifest/CSP/permission assertions
- static unsafe-DOM/dynamic-code scan
- source secret-pattern and machine-path scans
- compiled Native Messaging framing/ping smoke test
- real yt-dlp/FFmpeg integration and ffprobe validation

The helper uses only the Go standard library, and the extension has no npm dependencies, so there is no third-party Go module or npm dependency tree to audit. Source-mode govulncheck misdetected the module on this Windows workspace path, so the supported compiled-binary mode was used for the release executable.

## Remaining manual boundary

Chrome Stable and Brave require the user-visible **Load unpacked** action for this non-store Windows distribution. Google removed automated `--load-extension` support from Chrome-branded builds; Chrome for Testing remains appropriate for automation. FramePier does not misuse enterprise policy or persistent launch flags to bypass this restriction.
