# Acceptance test matrix

Automated results should be recorded with the release build. Interactive items must be checked on the target Windows machine rather than inferred from source tests.

## 0.2.1 folder action regression

- Before changing product code, invoked the installed 0.2.0 `open_download_folder` Native Messaging command. It returned `ok: true` and added an Explorer window for the configured folder, but `IsWindowVisible` returned false. Seven hidden VidDock folder windows were present after the reproduction.
- The source button listener sends the fixed folder command. Its original handler did not catch rejected requests. Folder dispatch uses the current configured directory and has no dependency on the media source, title, output filename, or previous job ID.
- The corrected 0.2.1 helper created a visible Explorer window at that same configured folder. Verified path and visibility using read-only Windows Shell window enumeration.
- Fresh Twitch clip (720p60 MP4), YouTube video (144p MP4), and MP3 downloads completed through Native Messaging in an isolated custom directory. Calling the same folder command immediately after Finished succeeded. Logs show the requested custom directory and successful normal-window dispatch.
- Visible Explorer windows were verified for spaces, parentheses, Hebrew/Japanese characters, and a long directory name within the existing path length limit. User settings were not changed by these isolated tests.
- Automated popup listener tests cover completed YouTube/clip/VOD/audio states, absence of job/path arguments, error display without losing Finished/file details, retry, and cancellation visibility. Windows helper tests cover directory validation, file and traversal rejection, reparse points where supported, OS-error propagation, and x64 ShellExecuteEx structure layout.
- The already downloaded 72-minute Twitch VOD remains available in the normal folder. This patch did not repeat the full VOD download; the source-independent folder command and VOD Finished-state listener were tested.
- These are native integration and popup-source tests. The Windows Computer Use tool stopped the attempted Brave check because it could not confidently determine the current browser URL. A fresh physical click in the user's Brave/Chrome popup was therefore not observed.
- Installed 0.2.1 over the current per-user installation: setup exited 0, the settings hash was unchanged, installed helper/popup hashes matched the release files, no Code 5 appeared in the log, and no Windows reboot was required. Go vet/tests, extension checks, folder UI regressions, static security checks and the final binary vulnerability scan passed.

| Area | Chrome 152 / Chrome for Testing 152.0.7977.75 | Brave 152 (default Shields) | Edge 152 |
| --- | --- | --- | --- |
| Extension loads with expected stable ID | Pass (automated CFT); Stable manual load remains | Pass | Not run |
| Watch URL detected | Pass | Pass | Not run |
| Shorts / youtu.be parser | Pass (unit/helper) | Pass (unit/helper) | Not run |
| Native helper connects | Pass | Pass | Registration verified |
| Metadata and dynamic formats | Pass; exact-dimension selection and true 3840x2160 labels | Pass; 3840x1920 popup label verified as `3840×1920` | Not run |
| Standard video download | Pass; 720p MP4 | Shared helper pass | Not run |
| 1080p separate-stream merge | Pass; 1920x1080 H.264 + AAC verified | Shared helper pass | Not run |
| 2160p separate-stream merge | Pass; 3840x2160 AV1 + AAC MP4 verified | Pass; popup selection plus shared helper | Not run |
| 2160p MKV | Pass; 3840x2160 VP9 + AAC verified | Shared helper pass | Not run |
| Best audio / M4A / MP3 | Pass (helper integration) | Shared helper pass | Not run |
| Progress and cancellation | Pass; no orphan/temp files | Pass; popup progress observed | Not run |
| Hidden child windows | yt-dlp/FFmpeg handles observed as zero | Shared helper pass | Shared helper/installer path |
| Logs and folder actions | Logs pass; UI folder open not invoked | Logs pass; UI folder open not invoked | Not run |
| Public Twitch clip metadata/download | Pass (shared helper + extension parser) | Pass; 1080p60 H.264/AAC MP4 | Not run |
| Public Twitch VOD metadata/download | Pass (shared helper + extension parser) | Pass; complete 72-minute 160p H.264/AAC MP4 | Not run |
| Twitch live/private/deleted handling | Unit/helper pass; deleted clip real fixture | Shared helper pass | Not run |
| Large VOD progress/cancellation/disk preflight | Shared helper pass | Pass | Not run |

Browser installation scenarios:

- Chrome only: only Chrome's selected/detected registry key is created.
- Brave only: only Brave's selected/detected registry key is created.
- Both: both point to the same restricted per-user host manifest.
- Browser installed later: Start Menu **Repair Browser Integration** detects the new browser and recreates its VidDock key.

### Chrome for Testing restoration

Chrome for Testing 152.0.7977.75 (win64) is required for browser integration tests. If `.tools/browser-tests/chrome-for-testing/` is deleted or missing, restore it with:

```powershell
powershell -ExecutionPolicy Bypass -File scripts/ensure-chrome-for-testing.ps1
```

The script:
- Fetches the download URL from the official Chrome for Testing per-version JSON API (`https://googlechromelabs.github.io/chrome-for-testing/152.0.7977.75.json`).
- Downloads `chrome-win64.zip` from `storage.googleapis.com/chrome-for-testing-public/`.
- Extracts and installs to `.tools/browser-tests/chrome-for-testing/chrome-win64/chrome.exe`.
- Verifies the installed version by reading the embedded `${Version}.manifest` file (version metadata contained in the downloaded package, no process needed), with `chrome.exe --version` as a fallback.
- If an existing `chrome.exe` is present, the script verifies its version before returning. If the version cannot be confirmed, it fails clearly and does not overwrite the existing installation.
- Cleans up temporary files on failure via `try/finally`.

No SHA-256 checksum is published by Google for Chrome for Testing artifacts; the script relies on the official JSON API as the authoritative source and verifies the version from the embedded manifest file after extraction. The manifest is version metadata contained in the same downloaded package — it is not independent cryptographic integrity verification.

Version 0.1.2 lifecycle scenarios:

- Reproduced the 0.1.1 locked-helper failure first: the old installer retried `DeleteFile` four times and exited with Code 5 while the Native Messaging pipe remained open.
- Upgraded that same running 0.1.1 instance with 0.1.2: old PID exited, installer returned 0, settings hash was preserved, and the installer log contained no access-denied or file-in-use retry.
- Two simultaneous Native Messaging helper instances exited gracefully during a same-version upgrade; no exact-path helper remained.
- Active real-video download reported lifecycle status 11, was cancelled by silent upgrade, and left zero VidDock-owned tool processes and zero private temporary entries.
- Explicit cancellation left zero VidDock-owned yt-dlp/FFmpeg processes. Force-terminating an active helper also left zero descendants because the downloader tree was assigned to a kill-on-close Windows Job Object.
- Uninstall with a helper connection open returned 0 and removed the locked executable; reinstall preserved settings. Repair returned 0 while another helper connection stayed active.
- Two installers launched together: the first returned 0 and the second was rejected by the setup mutex with exit code 1.
- Chrome and Brave remained open during installer runs. Native-host relaunch was separately exercised against the replacement gate and was blocked until lifecycle resume.

Version 0.1.3 Native Messaging repair scenarios:

- Confirmed the 0.1.2 upgrade sequence removed Brave's HKCU Native Messaging key during replacement even though the installed manifest and restored key were later valid. An open Brave session had already observed the host-not-found interval.
- Upgraded 0.1.2 to 0.1.3 with Brave open: the Brave registry value remained continuously unchanged, post-install repair passed, and the installer log contained no access-denied error.
- Opened the existing VidDock extension settings page in Brave without removing or re-adding it. Brave launched helper 0.1.3 through `chrome.nativeMessaging` pipes with the stable extension origin.
- Repair restored a missing Brave key, missing manifest, invalid JSON, wrong helper path, wrong allowed origin, and missing Chrome key. Both 32-bit and 64-bit registry views were queried successfully after each repair.
- Captured Brave's raw `chrome.runtime.lastError` with both relevant keys absent: `Specified native messaging host not found.` The extension recorded the raw diagnostic and returned the specific user-facing host-not-found category.
- Existing-profile Brave connection after upgrade and fresh install passed without removing or re-adding VidDock. An isolated Brave 152 extension run passed Native Messaging ping, real YouTube metadata, and a completed 144p download.
- Chrome-for-Testing 152 passed the equivalent extension-level ping, metadata, and completed download. Chrome Stable's HKCU registration was repaired and validated; branded Chrome cannot use command-line unpacked loading for an isolated automation profile.

Version 0.1.4 resolution-label scenarios:

- Reproduced the incorrect label with `fo-uubnajWM`: yt-dlp reports both 3840x1920 AV1 and VP9 streams with the generic `format_note` value `2160p`; VidDock 0.1.3 reduced that note to integer tier 2160 and the popup rendered `2160p (4K)`.
- Brave 152 rendered the real popup options as `3840×1920`, `2560×1280`, `1920×960`, and their actual lower dimensions. The option values carry exact `widthxheight` identities.
- Chrome for Testing 152 passed the same extension-level Native Messaging ping, real metadata, exact-resolution option handling, and download path.
- Explicit 3840x1920 MP4 download of `fo-uubnajWM` produced AV1 video at 3840x1920 with AAC audio. ffprobe identified the MP4-family container.
- True 4K fixture `2PuFyjAs7JA` rendered `2160p (4K)` and its explicit 3840x2160 MP4 download produced AV1 video at 3840x2160 with AAC audio.
- Unit coverage includes standard 16:9, ultrawide/non-standard landscape, vertical, square, width-and-height deduplication, deterministic pixel-count sorting, and exact-resolution stream selection.

Version 0.1.5 settings and activation scenarios:

- Reproduced the missing Browse button before editing: source, installer payload, and installed 0.1.4 Settings files had identical hashes, and none contained a Browse element or handler. The feature was absent in source rather than lost during packaging.
- Reproduced stale unpacked-extension behavior in isolated Brave: replacing the manifest on disk from 0.1.4 to 0.1.5 left the active context reporting 0.1.4 until an extension reload.
- The actual installed 0.1.5 Settings files match source hashes. Isolated Brave 152 and Chrome for Testing 152 rendered a visible, aligned `Browse...` button and reported Native Messaging `Connected`, helper/package 0.1.5, and FFmpeg `Installed · 8.1.1`.
- A temporary Unicode-capable writable download directory was saved through Native Messaging, survived the 0.1.5 installer upgrade, survived helper restart, and received a completed real 256x144 MP4 download. The user's original download location was restored afterward.
- Helper tests cover fixed-command field restrictions, invalid paths, Unicode directory creation, write-probe cleanup, folder-dialog cancellation HRESULT classification, strict package-version syntax, and stable extension-ID derivation from the manifest key.
- Chrome's official runtime documentation confirms `chrome.runtime.reload()` reloads an extension and treats an unpacked reload as an update. The isolated command-line-loaded test extension is not persistent across `runtime.reload()`, so automatic activation cannot be honestly certified from `--load-extension`; persistent user-profile confirmation remains manual.
- The already-active 0.1.4 worker cannot gain the 0.1.5 detector from files copied underneath it. Because the user's Brave and Chrome sessions were active, they were not forcibly closed; one restart or manual Reload is required for this transition. Once 0.1.5 is active, native disconnect alarms and the next trusted helper handshake can trigger one guarded reload for a newer package.

Version 0.1.6 foreground-picker and activation scenarios:

- Reproduced the picker ownership defect in the installed implementation: `IFileDialog.Show` received owner HWND 0, so Windows had no modal owner relationship tying the picker to the browser.
- The helper now captures the foreground top-level window when the fixed Browse command arrives, accepts it only when it is a visible Chrome-class window from Brave, Chrome, or Edge in the same Windows session, and passes it to `IFileDialog.Show`.
- Isolated Brave 152 and Chrome for Testing 152 both kept the loaded 0.1.5 manifest after their on-disk extension files were replaced by 0.1.6. After a complete exit and reopen of the same profiles, both service workers reported 0.1.6.
- The 0.1.6 detector read the installer-owned external package marker, reached the version-mismatch branch, stored the running/target guard, and invoked `chrome.runtime.reload()` exactly once in an instrumented service-worker test.
- A real `chrome.runtime.reload()` call unloaded the command-line-loaded Chrome for Testing extension, confirming that this harness cannot certify persistent Developer-mode reload behavior. The product therefore treats full browser restart as the supported reliable fallback and the circular Reload button only as a final fallback.
- The reload guard no longer expires and retry-loops for the same running/target pair. If activation did not occur, the extension persists and displays a complete-browser-restart instruction.

Google removed `--load-extension` from Chrome-branded builds in Chrome 137. The installed Chrome Stable 152.0.7977.82 test exposed zero VidDock extension targets when launched with that flag, while the matching official Chrome for Testing 152.0.7977.75 build passed. Brave 152.1.94.119 permits the isolated command-line test and was exercised with a clean profile and default Shields. Command-line loading is used only for testing; it is not installed as a persistent user shortcut.

Real metadata fixtures included Big Buck Bunny (`aqz-KE-bpKQ`), a maximum-1080p CC BY upload (`rOHD0n7-BtQ`), `fo-uubnajWM` at 3840x1920, and an ultrawide clip (`Ttl8Gg-P-Ao`) whose raw heights are 1644/1096/822 despite yt-dlp's generic 2160p/1440p/1080p notes. Short download fixture `2PuFyjAs7JA` produced 720p H.264/AAC MP4, 2160p AV1/AAC MP4, and 2160p VP9/AAC MKV outputs. M4A and MP3 outputs were confirmed. Cancellation left zero new VidDock temp entries and zero yt-dlp/FFmpeg processes.

Version 0.2.0 Twitch scenarios:

- Public clip `AmusedMildTitanOSkomodo-hl2PV3WlBurF_8uo` worked anonymously through both `clips.twitch.tv/<slug>` and `twitch.tv/twitch/clip/<slug>`. Metadata identified `twitch:clips`; duplicate codecs were collapsed by dimensions/FPS while landscape and portrait encodes remained distinct.
- Explicit 1080p60 clip selection completed as a 42,917,500-byte MP4. ffprobe reported H.264 1920x1080 at 60 fps plus AAC audio, duration 44.583 seconds.
- Explicit 720p60 clip selection also completed through the MKV path. ffprobe reported H.264 1280x720 at approximately 60 fps, AAC audio, Matroska container, duration 44.586 seconds, and size 14,579,229 bytes.
- Public VOD `2865128806` worked anonymously and exposed `Source (1080p60)`, 720p60, 480p, 360p, 160p, and audio. Duration rendered with hours and M4A availability was reported from the AAC audio format.
- A complete 160p selection of that 72-minute VOD downloaded with live percentage, speed, ETA, and 64-bit byte totals. ffprobe reported H.264 284x160 at 30 fps, AAC audio, MP4-family container, duration 4,395.115 seconds, and size 137,484,940 bytes.
- Cancellation during that VOD returned the job to `cancelled`; no VidDock-owned yt-dlp/FFmpeg process remained. A deleted real clip produced the specific unavailable message. Live channel URLs were rejected locally without invoking yt-dlp.
- Unit coverage includes both clip URL forms, VOD URLs, lookalikes and unsupported Twitch pages, extractor identity, incomplete clip codec metadata, Source/FPS labels, exact format selection, auth/deleted/rate-limit/network error mapping, more-than-4-GiB counters, and insufficient-space decisions.
- No cookie, OAuth, browser-profile, or login integration was used. Subscriber-only/private handling was validated by deterministic error mapping rather than attempting access to protected media.
- The installed 0.2.0 payload matched source hashes, retained the stable extension ID, preserved the settings-file SHA-256 across upgrade, and kept Chrome/Brave HKCU registration pointed at the valid restricted host manifest. A local framed installed-helper ping returned helper/package/extension 0.2.0.
- An isolated Brave 152.1.94.119 profile loaded the installed extension and rendered `Twitch VOD`, duration `1:12:13`, `Best available`, `Source (1080p60)`, `720p60`, the lower source resolutions, and an approximately 3.9 GB estimate. Native Messaging and real anonymous VOD metadata passed.
- Chrome for Testing 152.0.7977.75 passed the equivalent installed-extension Native Messaging and VOD metadata flow. Chrome Stable 152.0.7977.82 registration was validated, but branded Chrome's command-line unpacked-extension restriction still prevents isolated automated UI loading.
- A same-version repair install replaced the final helper without Code 5, preserved settings, and required no reboot. The exact-path lifecycle check reported only an idle browser-connected helper and no yt-dlp, FFmpeg, or ffprobe process.
- A fresh 0.2.0 YouTube regression download of `2PuFyjAs7JA` at 144p completed in the isolated configured folder. ffprobe reported H.264 256x144 at 30000/1001 fps, AAC audio, MP4-family container, and duration 11.053 seconds; the same metadata request still exposed the true 2160p/1440p/1080p ladder.

Do not mark an item passed without observing it. Access-limited/private cases must be tested without importing cookies or bypassing restrictions.
