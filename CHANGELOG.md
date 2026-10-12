# Changelog

All notable changes to FramePier are documented here. Versions follow Semantic Versioning.

## [Unreleased]

### Added

- Bundled the Deno 2.9.7 JavaScript runtime (hash-pinned, verified against the checksum published with the official release) so yt-dlp can solve YouTube's extraction challenges without relying on anything installed on the PC.
- Settings now shows the JavaScript runtime status. If the runtime is missing or not working, YouTube requests fail with a clear message to reinstall FramePier; Twitch and Dailymotion are unaffected.

- Public Dailymotion video support (`dailymotion.com/video/<id>`): combined HLS qualities, MP4/MKV by remux, and FFmpeg audio extraction, using yt-dlp's dedicated Dailymotion extractor. It needs no JavaScript runtime, shows no thumbnail, adds no extension permissions, and rejects live streams, playlists, embeds, short links and other Dailymotion hosts.

### Fixed

- A finished download now shows the real size of the saved file ("Final file size") instead of the downloader's running total, which for HLS downloads is an estimate and could be several times the actual size.
- When a video has no safe thumbnail (for example Dailymotion), the popup shows a neutral placeholder instead of a broken-image icon.

### Changed

- Replaced upstream's published `yt-dlp.exe` (Python 3.10, OpenSSL 1.1.1t, end-of-life) with a build of the same tagged source (2026.08.19) made with yt-dlp's own recipe on Python 3.13.16 (OpenSSL 3.5.9, SQLite 3.50.4). curl_cffi impersonation and the Deno integration are unchanged. SHA-256 `8b976e7ff71a6f8e8dd48feb2920b3c41a44641ffe192dba08cde0175890ee44`; recipe, evidence and corresponding source are in `FramePier-0.2.3-YtDlp-Source.zip`.
- Removed the in-app yt-dlp self-updater: the Settings button and the helper's `update_ytdlp` command are gone (the helper now rejects it), so the pinned, verified yt-dlp cannot be replaced by an unverified upstream executable. yt-dlp changes only with a new FramePier release.
- Added the compiled license texts for Deno's Rust crates and V8 build libraries, and the CPython runtime component notices, to `licenses/`.
- Replaced the static BtbN FFmpeg with a minimal shared LGPL build (FFmpeg `1a748fe`, LAME 3.100; `ffmpeg.exe`, `ffprobe.exe` and 9 DLLs). All eleven files are SHA-256 pinned in `scripts/build.ps1` and installed together. FFmpeg is no longer downloaded during the build.
- Settings now shows the abbreviated commit hash when FFmpeg reports `ffmpeg version <hash>` (previously "version unknown").
- Replaced the FFmpeg LGPL-3.0 license text with LGPL-2.1, added LAME and winpthreads license texts, the MinGW-w64 runtime text, the FFmpeg build provenance and a source-availability index to `licenses/`.
- Every yt-dlp process is now created by one helper that always passes `--ignore-config`, `--no-plugin-dirs`, `--no-remote-components` and `--no-js-runtimes`, then enables only the bundled Deno by absolute path for YouTube. User-level yt-dlp configuration, plugins, remotely fetched components and runtimes found on PATH are no longer used.

## [0.2.3] - 2026-09-20

### Changed

- Upgraded FFmpeg from Gyan 8.1.1 (GPL) to BtbN 8.1.2 LGPL-3.0 build (`autobuild-2026-08-31-13-27`), fixing CVE-2026-8461 (CVSS 8.8) and CVE-2026-30999.
- Updated build script to pin the immutable BtbN release-tag URL with SHA-256 verification on both the archive and extracted binaries.
- Updated `THIRD_PARTY_NOTICES.md` to document the BtbN provider, build-repo commit, FFmpeg source commit, and LGPL-3.0 licensing.

## [0.2.2] - 2026-09-05

### Fixed

- Removed dependency version checks from helper startup; System Status still checks the installed tools on request.
- Prevented duplicate download IDs from replacing jobs and separated temporary directories across helper connections.
- Tightened cancellation and upgrade process ownership, and made settings replacement atomic.
- Stopped unrelated downloader diagnostics from resetting progress and bounded completed-job history.
- Serialized update checks so simultaneous responses cannot schedule duplicate reload attempts.
- Removed the duplicate installer manifest writer; browser launch resumes after registration validation. Failed uninstall attempts now release the launch gate.

### Maintenance

- Added worker and helper regression tests and Staticcheck configuration, removed an unused resolution parser, and fixed smoke-test timeout cleanup.

## [0.2.1] - 2026-09-05

### Fixed

- Fixed “Open download folder” creating a hidden Explorer window after completed downloads, including Twitch clips.
- Folder actions now use the Windows shell with a normal visible window. Background downloader and FFmpeg processes remain hidden.
- Popup and Settings now display folder-launch errors and allow retrying; helper logs record the folder request and Windows result.
- Applied the same fix to automatic folder opening, the logs folder, and the installer's extension-folder shortcut action.

## [0.2.0] - 2026-09-05

### Added

- Added anonymous downloads for public Twitch clips and completed public Twitch VODs in both MP4 and MKV.
- Added Twitch source labels, source-aware quality choices including `Source` and 60 fps variants, and approximate size reporting when metadata permits.
- Added preflight free-space checks and 64-bit byte progress for long, multi-gigabyte VOD downloads.

### Changed

- Twitch clip and VOD URLs are validated and canonicalized separately from YouTube while retaining the same restricted Native Messaging architecture.
- Long VOD downloads no longer inherit metadata timeouts and retain yt-dlp's normal partial-file/resume behavior where supported.
- Public Twitch media works without cookies, OAuth, login prompts, or browser-profile access; live, private, subscriber-only, deleted, and expired media remain unsupported.

### Fixed

- Twitch clip formats with incomplete codec metadata are now recognized without admitting storyboard or image pseudo-formats.
- Twitch VOD progress and estimated totals remain monotonic when HLS size estimates fluctuate.
- Private job directories are cleaned if VOD metadata or disk-space preflight fails before downloading.

## [0.1.6] - 2026-09-04

### Fixed

- Fixed the download-folder picker opening behind Brave or Chrome by using the validated foreground browser window as the native dialog owner.
- Improved update activation diagnostics and the browser-restart fallback when Chromium does not reload changed unpacked extension files automatically.

### Changed

- Moved the trusted installed package version marker outside the extension directory so an older active worker can observe the new installer version independently of Chromium's cached manifest.
- Added guarded helper log events for version mismatch, reload request, successful activation, and duplicate reload suppression.
- A failed reload is now guarded permanently for that running/target version pair, and the popup and Settings page show an explicit browser-restart instruction instead of retrying in a loop.

## [0.1.5] - 2026-09-04

### Added

- Added a native Browse button for selecting the FramePier download folder through the Windows folder picker.

### Changed

- Download locations are validated for creation and write access and continue to persist through browser restarts, helper restarts, and FramePier upgrades.
- Added a guarded installed-package version handshake so FramePier 0.1.5 and later can activate future unpacked-extension updates with Chromium's supported reload API.
- Added clear restart guidance for the one-time upgrade from older builds that do not yet contain the reload detector.
- Shortened the FFmpeg status display while retaining the full build string in helper logs.

## [0.1.4] - 2026-09-04

### Fixed

- Fixed quality labels for ultrawide, vertical, square, and other non-standard video resolutions.
- FramePier now displays actual source dimensions when a video does not match a standard landscape 16:9 resolution.
- Resolution choices are deduplicated by width and height, and explicit selections resolve to that exact source size.

## [0.1.3] - 2026-09-04

### Fixed

- Fixed Brave losing Native Messaging connectivity after an upgrade temporarily removed its host registration.
- Upgrades now preserve browser registration while stopping and replacing the helper.
- Post-install validation now rebuilds and verifies the host manifest, both Windows registry views, helper path, allowed extension origin, and a real helper ping.
- Repair Browser Integration now recreates missing or corrupt manifests and corrects registry, helper-path, and allowed-origin damage.
- Native Messaging errors now retain technical diagnostics while showing users a specific, readable failure category.

## [0.1.2] - 2026-09-04

### Fixed

- Fixed upgrades failing when `FramePierHelper.exe` was still running.
- The installer now suspends Native Messaging launches and safely stops FramePier background processes before updating or uninstalling.
- Active FramePier downloads and merges are detected before interactive upgrades; continuing cancels them and removes only FramePier's private temporary fragments.
- Added Windows Job Object ownership for downloader process trees so helper shutdowns and crashes do not leave FramePier-owned yt-dlp or ffmpeg processes behind.
- Reviewed upgrade, uninstall, repair, cancellation, rollback, and concurrent-installer paths for file-lock and process-lifecycle issues.

## [0.1.1] - 2026-09-04

### Fixed

- Launch all helper child processes with Windows `CREATE_NO_WINDOW` and hidden-window attributes, including metadata checks, downloads, updates, version checks, FFmpeg work, registry repair, cancellation, and folder actions.
- Discover quality tiers from real yt-dlp format metadata instead of matching raw pixel heights against a fixed landscape-only list. This restores 1440p/2160p choices for adaptive, portrait, and ultrawide sources.
- Treat video-only DASH streams, WebM, VP9, and AV1 as valid quality sources while excluding storyboards and image pseudo-formats.
- Resolve explicitly selected qualities to an exact current video format and compatible audio format, with no silent quality downgrade or video transcode.
- Report when MKV is needed for a codec combination instead of silently hiding the resolution.
- Recover the completed output path safely when yt-dlp finishes successfully without emitting its expected post-processing marker.

### Changed

- The installer now detects Chrome, Brave, and Edge, lets the user choose detected browsers, reports per-browser integration status, and can open the correct extension pages plus the installed extension folder.
- Added a Start Menu **Repair Browser Integration** action that detects browsers installed later and restores FramePier's restricted Native Messaging registrations.
- Documented the Windows browser restriction that prevents supported permanent local CRX installation in ordinary Chrome/Brave profiles without store publication or managed enterprise policy.

## [0.1.0] - 2026-09-04

### Added

- Manifest V3 extension for Chrome, Brave, and Edge with a compact dark popup.
- Stable unpacked extension identity and restricted Native Messaging origin.
- Dynamic YouTube metadata, thumbnails, duration, qualities, and audio choices through yt-dlp.
- Video downloads to MP4/MKV and audio downloads to best, M4A, or MP3.
- Progress, speed, ETA, cancellation, verified completion, and folder actions.
- Static Go Windows helper with a validated fixed JSON protocol and local logging.
- Per-user Inno Setup installer with Chrome, Brave, and Edge registration and normal uninstall support.
- Checksum-verified dependency build flow, security tests, documentation, and original icon set.
