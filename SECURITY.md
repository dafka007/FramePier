# FramePier security policy

## Supported version

Security fixes are currently made on the latest supported FramePier development line. The next public binary release remains subject to the project's dependency, security, and redistribution release gate.

## Reporting a vulnerability

Do not open a public issue for a vulnerability that could expose user data or execute code. Use GitHub's private vulnerability reporting feature when it is available for this repository. If private reporting is unavailable, contact the maintainer privately before publishing exploit details.

Please include:

- affected FramePier version;
- browser and Windows version;
- reproducible steps;
- expected and actual behavior;
- logs with personal paths and video identifiers redacted.

Do not include cookies, authentication tokens, private video URLs, credentials, or downloaded media.

## Security boundaries

FramePier intentionally:

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
- requests only narrowly scoped browser permissions and explicit supported-site host permissions;
- does not read browser cookies, OAuth tokens, credentials, profiles, or unrelated browsing history.

The helper is not a DRM or access-control bypass. Changes that add credential theft, cookie extraction, protected-stream circumvention, or generic system control are outside the project security model.

## Legacy compatibility identifiers

Some internal identifiers intentionally retain the former VidDock name so existing installations can be upgraded safely. These include the Native Messaging host name `com.viddock.helper`, legacy AppData paths under `%LOCALAPPDATA%\VidDock`, and migration/package markers. They are compatibility identifiers, not the current product name, and must not be renamed independently without a tested migration plan.

## Dependency and update trust

Builds retrieve Go from `go.dev`, `yt-dlp.exe` from the official `yt-dlp/yt-dlp` GitHub release, and a pinned FFmpeg Windows build from a provider linked by the FFmpeg project. Downloaded release artifacts are checksum-verified before packaging.

Before every public binary release, the release gate must verify the exact dependency versions, current security advisories, the exact artifacts and hashes being bundled, license and redistribution obligations, and required source/source-availability status. A failed or incomplete gate blocks the public release.

The in-app yt-dlp updater invokes only `yt-dlp.exe -U`; callers cannot choose a repository or arbitrary update arguments.

Release maintainers should retain checksums and third-party notices with build records, scan the complete source tree and relevant Git history for secrets, run the automated test/security checks, and manually inspect the final installer before distribution.

## Known limits

- Browser acceptance tests require interactive Chrome/Brave sessions and cannot be fully represented by unit tests.
- A user-selected writable directory can be changed by other local processes after validation. FramePier rechecks that the final reported file remains beneath the configured folder and is a non-empty regular file.
- Native Messaging host authorization is enforced by Chromium and checked again from the origin argument received by the helper. Local malware running as the same Windows user is outside the browser-extension trust boundary.
