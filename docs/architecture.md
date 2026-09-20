# Architecture and native protocol

FramePier's extension is a UI/controller. It queries the active tab, recognizes supported YouTube videos, public Twitch clips, and completed public Twitch VODs, and sends a canonical request to a local Windows helper. It has no content script and does not attempt to extract streams.

The Manifest V3 service worker owns one `connectNative` port so a download can continue when the popup closes. Popup and Settings requests are correlated with random IDs. Progress events are cached in the service worker and rebroadcast to open extension pages.

The helper reads Chromium's four-byte little-endian length prefix followed by UTF-8 JSON. Messages above 1 MiB are rejected. Requests permit only these fields: `id`, `command`, `url`, `videoId`, `mode`, `quality`, `container`, `audioFormat`, `downloadPath`, and `autoOpen`. Fields not used by a command are ignored only when they are still part of this fixed schema; callers cannot pass raw process arguments.

Supported commands:

| Command | Purpose |
| --- | --- |
| `ping` | Native host liveness and helper version |
| `get_video_info` / `get_formats` | Validated yt-dlp metadata request |
| `start_download` | Begin one validated video/audio job |
| `cancel_download` | Cancel the matching job and its child process tree |
| `get_status` | Return the cached state for one job |
| `check_dependencies` / `get_versions` | Helper, yt-dlp, and FFmpeg status |
| `get_settings` / `set_settings` | Read or validate/save local helper settings |
| `choose_download_folder` | Show the fixed Windows folder-only picker and return one selected local path |
| `open_download_folder` / `open_logs_folder` | Open one helper-owned configured folder |
| `update_ytdlp` | Run the fixed official executable's `-U` action |
| `record_extension_update_event` | Record a closed set of validated local update diagnostics |

The host manifest and helper origin check both restrict access to `chrome-extension://kclnooibijmfenaldmpkffdbednfipkk/`. The ID is derived from the public key embedded in `manifest.json`; no private signing key is stored in the repository.

For downloads, the helper chooses format selectors from validated enums and yt-dlp metadata it obtained itself. It passes a fixed output template, `--no-playlist`, Windows filename rules, a fixed FFmpeg location, and a canonical allowlisted media URL. It never runs a shell. Completion is reported only after the final path is beneath the configured folder and points to a non-empty regular file.

YouTube retains adaptive video-plus-audio selection. Twitch clips select an exact combined source format because the clip extractor can omit codec fields even though the stream contains both tracks. Twitch VOD choices preserve `Source` and frame-rate identity and use combined HLS formats when supplied. VOD format discovery runs before download so FramePier can estimate size and check free space. Only short metadata lookups have a timeout; a running download is bounded by explicit cancellation and the helper-owned Windows Job Object instead of an arbitrary duration limit.

The URL allowlist accepts only official HTTPS YouTube and Twitch hosts, rejects credentials and custom ports, and canonicalizes a validated ID or clip slug. Twitch channel/live, directory, collection, and other non-media pages are rejected before yt-dlp is launched. FramePier never supplies cookies, OAuth credentials, browser profiles, or authentication flags.
