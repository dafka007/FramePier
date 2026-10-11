# FramePier — Browser Video Downloader for Windows

FramePier is an open-source Windows video downloader that works through a browser extension and local helper. It currently supports YouTube videos, public Twitch clips/VODs, and public Dailymotion videos, with Chrome, Brave, and Edge supported today. Downloads are handled locally by `yt-dlp`, with FFmpeg used for merging or conversion.

Use FramePier only for media you own, have permission to download, or are otherwise legally allowed to save.

Version: **0.2.3**

## Support

- Windows 10 and Windows 11 (x64)
- Google Chrome 137 or newer
- Brave Browser 1.60 or newer, including normal/default Shields
- Microsoft Edge 137 or newer (secondary support)

## Requirements

The Windows installer includes the FramePier helper, a checksum-verified official `yt-dlp` Windows release, and a shared FFmpeg build (see below). It also includes a pinned, checksum-verified Deno JavaScript runtime, which yt-dlp needs to extract YouTube videos. No Python or other runtime is required after installation.

## Install the helper

1. Run FramePier-Setup-0.2.3.exe.
2. Accept the default install location.
3. Select the installed browsers shown on the **Browser Integration** page. FramePier registers Native Messaging manifests for those browsers.
4. After installation, load the FramePier extension manually using the instructions below.

Administrator access is not normally required. If a browser is installed after FramePier, run **Repair Browser Integration** from the FramePier Start Menu folder. It detects newly installed browsers and repairs FramePier's Native Messaging registrations.

### Windows browser installation restriction

Chrome and Brave on Windows do not provide a supported consumer-profile API that lets an ordinary third-party installer permanently install a local, self-hosted extension with only an approval click. Chrome's documented external-extension registry mechanism requires a Chrome Web Store update URL on Windows; self-hosting is limited to managed enterprise environments. Chrome 137 and later also ignore `--load-extension` in branded Chrome builds. FramePier deliberately does not set enterprise force-install policy, alter browser security, or add persistent command-line launch flags.

Consequently, the installer automates browser detection and Native Messaging registration, but **Developer mode → Load unpacked** remains a manual step for Chrome, Brave, and Edge until FramePier is published through an approved store. Normal updates keep that installed extension and its stable ID `kclnooibijmfenaldmpkffdbednfipkk`; Remove/Load unpacked is not repeated.

## Load the extension (Chrome, Brave, or Edge)

This is a sideloaded, unpacked extension. It is not installed from the Chrome Web Store. Do not move or delete the extension folder after loading it.

1. Install FramePier first (run the setup installer).
2. Open your browser's extensions page:
   - Chrome: `chrome://extensions`
   - Brave: `brave://extensions`
   - Edge: `edge://extensions`
3. Turn on **Developer mode** (toggle in the top-right corner).
4. Click **Load unpacked**.
5. Select `%LOCALAPPDATA%\Programs\FramePier\Extension` — the folder that contains `manifest.json`.
6. Confirm the extension appears as **FramePier** in the extension list.
7. Open the FramePier **Settings** page (gear icon in the popup).
8. Confirm **Native Messaging: Connected** is shown.

The GitHub release also includes `FramePier-extension-0.2.3.zip`. You can extract that ZIP to a permanent folder and use **Load unpacked** on the extracted folder containing `manifest.json` instead of the installed path.

The checked-in manifest public key keeps the unpacked extension ID stable: `kclnooibijmfenaldmpkffdbednfipkk`. The Native Messaging manifest accepts only that extension origin.

Brave Shields can remain enabled. FramePier uses the active tab URL and the local helper; it does not scrape the page or inject a content script.

## First download

1. Open a supported YouTube watch, short, live, or `youtu.be` URL; a public Twitch clip; a completed public Twitch VOD; or a public Dailymotion video (`dailymotion.com/video/<id>`).
2. Click the FramePier toolbar icon.
3. Wait for metadata and the available quality list.
4. Choose **Video** or **Audio**, then a quality/container or audio format.
5. Click **Download**.
6. Keep the browser running while the popup may be closed; the extension service worker keeps its native connection for the active job.
7. Reopen FramePier to view progress, or open the default output folder at `%USERPROFILE%\Downloads\FramePier`.

The ordinary download action always uses `--no-playlist`, so a watch URL cannot accidentally download an entire playlist. Video filenames include the actual output height so requesting another quality cannot silently reuse an older file.

### Twitch support

FramePier accepts public Twitch clip URLs in either `clips.twitch.tv/<slug>` or `twitch.tv/<channel>/clip/<slug>` form and completed public VOD URLs in `twitch.tv/videos/<id>` form. It works anonymously: it does not import browser cookies, request OAuth tokens, prompt for a Twitch login, or bypass subscriber/private restrictions. Live channel streams, chat, collections, categories, channel pages, subscriber-only VODs, and deleted/expired media are intentionally unsupported.

Twitch VODs can be several hours and multiple gigabytes. Before a VOD starts, FramePier retrieves its current formats, estimates the selected stream size when bitrate metadata permits, and checks free space for the destination and processing directory. Estimates are approximate. Downloads have no arbitrary duration timeout, use 64-bit byte counters, remain cancellable, and retain yt-dlp's normal partial-file/resume behavior when the source protocol permits it.

### Dailymotion support

FramePier accepts public Dailymotion video pages in the form `https://www.dailymotion.com/video/<id>` (also without `www`, and with the older `/video/<id>_<slug>` suffix). The URL is rebuilt from the validated ID; query strings, slugs and fragments are never passed to the downloader. Other Dailymotion hosts and page types (embeds, short links, playlists, user pages, search, live streams) are intentionally unsupported.

Dailymotion serves each quality as one combined HLS stream, so qualities are the stream sizes it reports, MP4 and MKV are produced by remuxing that stream without re-encoding, and Audio is extracted from the video stream with FFmpeg (MP3 or M4A; the default "best" audio is saved as M4A). No Dailymotion thumbnail is shown. It works anonymously (no cookies, login, or DRM handling), does not need the JavaScript runtime, and uses yt-dlp's dedicated Dailymotion extractor.

## Architecture

```text
Chrome / Brave / Edge
        │
Manifest V3 extension (UI and controller)
        │  Chrome Native Messaging
FramePierHelper.exe (Go, fixed JSON protocol)
        │
yt-dlp.exe ── FFmpeg/ffprobe when required; bundled Deno (YouTube only)
        │
%USERPROFILE%\Downloads\FramePier
```

The helper accepts only named operations such as `ping`, `get_video_info`, `start_download`, `cancel_download`, `get_versions`, and settings/folder actions. It does not accept arbitrary executables, shell text, environment variables, or command-line arguments. See [docs/architecture.md](docs/architecture.md) for protocol details.

## Formats

- Video: best available or every unique source quality tier reported by yt-dlp, including video-only DASH, VP9, AV1, WebM, portrait, and ultrawide sources; MP4 or MKV output.
- Audio: best audio, M4A, or MP3.
- Separate audio and video streams are merged by FFmpeg. FramePier asks for remuxing/stream copy and does not intentionally re-encode video.

For an explicitly selected quality, FramePier resolves an exact current video format plus a compatible audio format. MP4 prefers H.264, AV1, or HEVC sources and AAC/M4A audio that FFmpeg can mux without video transcoding. If a quality is available only as a source combination that is unsuitable for a compatible MP4, FramePier reports that MKV is required instead of hiding or downgrading the quality. MKV accepts H.264, VP9, AV1, AAC, and Opus combinations supported by FFmpeg. MP3 necessarily converts audio.

## yt-dlp and FFmpeg

The build packages FramePier's own `yt-dlp.exe` build of yt-dlp 2026.08.19 (Python 3.13 / OpenSSL 3.5.9, built from the tagged source with yt-dlp's recipe; see `third-party/ytdlp-openssl3/`), supplied as a local file (`-YtDlpPath`, default `.tools\ytdlp-openssl3\yt-dlp.exe`) and checked against a pinned SHA-256. Public binary releases remain subject to the dependency, security and redistribution release gate. The in-app yt-dlp updater was removed in 0.2.3 so the pinned, verified executable cannot be replaced by an unverified one.

FFmpeg is a shared (DLL) LGPL build made from FFmpeg commit `1a748fe2cd43e3ead22fafb1b5b7d77f153898a8` and LAME 3.100 with MSYS2 UCRT64 (configuration and provenance in `third-party/ffmpeg-shared/` and `licenses/FFMPEG-BUILD-PROVENANCE.txt`). `ffmpeg.exe`, `ffprobe.exe` and nine DLLs are installed together in the tools folder. The build script copies them from a local directory and checks the SHA-256 of all eleven files; it never downloads FFmpeg. FramePier never downloads tools from page-provided URLs.

Deno 2.9.7 is bundled as the JavaScript runtime yt-dlp requires for YouTube. The build downloads it from the official release, checks the archive against the checksum published with that release and against a hash recorded in `scripts/build.ps1`, and checks the extracted `deno.exe`. FramePier runs every yt-dlp process with `--ignore-config`, `--no-plugin-dirs`, `--no-remote-components` and `--no-js-runtimes`, then enables only the bundled Deno by absolute path. It never uses a runtime found on PATH, and Twitch downloads do not need it. Third-party licenses and where the source is published are listed in `THIRD_PARTY_NOTICES.md` and `licenses\SOURCE-AVAILABILITY.txt`.

## Settings and logs

Open the gear button in the popup to configure defaults, select the download folder with the native Windows **Browse...** dialog, and choose whether the folder opens after a successful download. The helper accepts only an absolute writable local Windows path; UNC paths, relative paths, root directories, traversal, control characters, reparse-point paths, and overlong values are rejected. Settings are stored outside the program installation directory and survive browser/helper restarts and upgrades.

Logs are stored at `%LOCALAPPDATA%\VidDock\logs`. Open them from Settings. Logs avoid cookies, credentials, browser history, and full requested URLs; video IDs and output base names can appear for diagnostics.

**Open download folder** opens the currently configured directory for every media source and audio format. It does not use the media title, source URL, or previous job's output path. Folder actions open a normal visible Explorer window; failures appear in the popup or Settings and are recorded in the helper log.

## Updating

- Extension/helper: build and install a newer FramePier release. FramePier compares the active extension with the installer's trusted `%LOCALAPPDATA%\Programs\FramePier\package-version.json` marker through the helper and requests Chromium's supported `chrome.runtime.reload()` once when a newer package is detected. Reload attempts and successful activation are recorded in the local helper log. If Chromium does not activate changed unpacked files, completely exit and reopen the browser; manual **Reload** remains the final fallback and Remove/Load unpacked is not required.
- yt-dlp: bundled and pinned (2026.08.19, FramePier's verified build); it is updated only by installing a newer FramePier release. There is no in-app yt-dlp updater.
- Browser installed later: run **Repair Browser Integration** from the Start Menu.

## Troubleshooting

- **Helper not installed / host missing:** install FramePier, then run Repair Browser Integration and restart the browser.
- **Extension ID not authorized:** remove duplicate unpacked copies and load the exact installed or repository `extension` folder. Confirm the ID shown by Chromium is `kclnooibijmfenaldmpkffdbednfipkk`.
- **yt-dlp or FFmpeg missing:** reinstall FramePier; Settings reports the discovered versions.
- **Unavailable/private/access-limited video:** FramePier does not take cookies or bypass access controls. Use a public or otherwise directly accessible video you are entitled to save.
- **Twitch live channel:** live streams are not supported yet; use a public clip or a completed public VOD URL.
- **Large Twitch VOD:** keep the browser and helper available, leave enough free space for the selected stream and processing, and use Cancel if needed. A retained `.part` file may allow yt-dlp to continue later when Twitch's delivery protocol supports it.
- **JavaScript runtime missing or not working:** YouTube requests fail with a message saying so, and Settings shows the runtime as Missing or Not working. Reinstall FramePier; Twitch and Dailymotion downloads are not affected.
- **Dailymotion quality missing:** Dailymotion's extractor can return a different set of renditions on successive requests for the same video (for example 288p and 480p on one request and 288p, 480p and 720p on the next). This was observed with the bundled yt-dlp and is not caused by FramePier's filtering; the upstream cause is unverified. Close and reopen FramePier to request the list again. A download at "Best available" uses whatever the downloader sees at that moment.
- **Format unavailable:** refresh the popup and choose one of the qualities currently reported by the helper.
- **Permission/disk error:** choose a writable local folder with sufficient free space.
- **Brave:** leave Shields enabled. If Native Messaging fails, repair registration and restart Brave; Shields do not control Native Messaging.

Detailed errors are written to the local FramePier log; normal UI messages intentionally avoid raw stack traces.

## Uninstall

Use **Installed apps → FramePier → Uninstall** or the FramePier Start Menu shortcut. Uninstall removes the helper, bundled tools, extension copy, Native Messaging manifest, and FramePier-created registry keys. Downloaded media is never removed. User settings and logs under `%LOCALAPPDATA%\VidDock` are left in place for diagnostics and can be deleted manually.

## Build from source

From PowerShell on Windows:

```powershell
.\scripts\build-icons.ps1 -Source .\assets\framepier-mark.png
.\scripts\build.ps1
```

The build script obtains the current stable Go Windows archive from `go.dev`, validates its official checksum, runs Go tests, builds a static helper, downloads and verifies the two media tools, packages the extension, and compiles the Inno Setup installer. Inno Setup 6 must be installed (`winget install --id JRSoftware.InnoSetup -e`). Build downloads and artifacts live under ignored `.tools` and `dist` directories.

Run all local checks with:

```powershell
.\scripts\test.ps1
```

## Development

The extension is plain HTML, CSS, and JavaScript with no remote code and no npm dependency tree. The helper uses only the Go standard library. Load `extension` unpacked, run the helper build, and register a development host manifest that points to `dist\helper\FramePierHelper.exe`.

Useful project directories:

- `extension/` — Manifest V3 source
- `helper/src/` — Go Native Messaging helper and tests
- `helper/native-host/` — restricted host manifest template
- `installer/` — Inno Setup source
- `scripts/` — reproducible build, registration, icon, and test scripts
- `docs/` — architecture and acceptance-test notes

## Security and privacy

FramePier has no analytics, telemetry, ads, accounts, or cloud backend. Extension metadata is treated as untrusted and rendered with safe DOM text APIs. The helper validates message shapes, URLs, enum values, IDs, numeric sizes, and filesystem paths. Process arguments are constructed by the helper and launched without a shell.

Please read [SECURITY.md](SECURITY.md) before reporting a vulnerability. The browser and helper deliberately do not import cookies or browser credentials.

## Legal use

Downloading media can be restricted by copyright law, contracts, or a service's terms. You are responsible for confirming that you have the necessary rights. FramePier does not bypass DRM, paywalls, membership restrictions, authentication, or other access controls.

## Contributing

Small, reviewable changes with tests are welcome. Do not add telemetry, broad host permissions, remote JavaScript, cookie extraction, credential access, DRM bypasses, or generic command execution. Run `scripts/test.ps1` and document browser acceptance results with the change.

## License

FramePier's original source is licensed under the MIT License; see [LICENSE](LICENSE). Bundled release tools have their own licenses, including LGPL-2.1-or-later terms for FFmpeg, LGPL terms for LAME, and GPLv3+ and other terms for components inside the yt-dlp executable. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). A distributor must preserve those notices and comply with the applicable third-party licenses.
