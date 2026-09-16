# VidDock security policy

## Supported version

Security fixes are currently made on the latest `0.2.x` development release only.

## Reporting a vulnerability

Do not open a public issue for a vulnerability that could expose user data or execute code. Until a private reporting address is established for a future public repository, contact the maintainer privately and include:

- affected VidDock version;
- browser and Windows version;
- reproducible steps;
- expected and actual behavior;
- logs with personal paths and video identifiers redacted.

Do not include cookies, authentication tokens, private video URLs, credentials, or downloaded media.

## Security boundaries

VidDock intentionally:

- authorizes only extension origin `chrome-extension://kclnooibijmfenaldmpkffdbednfipkk/` in the Native Messaging host manifest;
- uses HTTPS and an allowlist of official YouTube and Twitch hostnames;
- canonicalizes accepted URLs using a validated YouTube ID, Twitch clip slug, or numeric Twitch VOD ID;
- accepts a small fixed JSON command set with a 1 MiB message limit;
- rejects unknown message fields and invalid field types;
- builds all `yt-dlp`, FFmpeg, Explorer, and registry process calls from explicit argument arrays without a command shell;
- never accepts executable names, command-line fragments, environment variables, or arbitrary folder-open paths from the extension;
- limits active download jobs and kills only the requested job's process tree;
- confines and verifies final output inside the configured local download directory;
- uses a restrictive extension CSP and no remotely hosted JavaScript;
- requests only `activeTab`, `nativeMessaging`, `storage`, and explicit supported-site host permissions;
- does not read browser cookies, OAuth tokens, credentials, profiles, or unrelated browsing history.

The helper is not a DRM or access-control bypass. Do not propose changes that add credential theft, cookie extraction, protected-stream circumvention, or generic system control.

## Dependency and update trust

Builds retrieve Go from `go.dev`, `yt-dlp.exe` from the official `yt-dlp/yt-dlp` GitHub release, and FFmpeg from gyan.dev (linked by ffmpeg.org). The build verifies SHA-256 values from the corresponding publisher before packaging. The in-app yt-dlp updater invokes only `yt-dlp.exe -U`; callers cannot choose a repository or arbitrary update arguments.

Release maintainers should retain checksums and third-party notices with build records, scan the complete source tree for secrets, run the local test/security script, and manually inspect the final installer before distribution.

## Known limits

- Browser acceptance tests require interactive Chrome/Brave sessions and cannot be fully represented by unit tests.
- A user-selected writable directory can be changed by other local processes after validation. VidDock rechecks that the final reported file remains beneath the configured folder and is a non-empty regular file.
- Native Messaging host authorization is enforced by Chromium and checked again from the origin argument received by the helper. Local malware running as the same Windows user is outside the browser-extension trust boundary.
