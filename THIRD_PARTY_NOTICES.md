# Third-party notices

FramePier release packages can include the following unmodified executable tools. They are not covered by FramePier's MIT license.

## yt-dlp

Project: <https://github.com/yt-dlp/yt-dlp>

The official Windows executable is a PyInstaller-bundled release and includes components under GPLv3+ and other licenses. Its release includes `THIRD_PARTY_LICENSES.txt`. Redistributors must review and comply with the exact release's applicable license terms and preserve required notices and source availability.

## FFmpeg

Project: <https://ffmpeg.org/>

The v0.2.2 build uses a Windows FFmpeg build from Gyan, a provider linked by ffmpeg.org. The selected v0.2.2 essentials build is GPL-licensed. Redistributors must comply with the corresponding GPL terms and make complete corresponding source available as required for the exact packaged build.

Future releases may use a different verified provider or license variant. The exact provider, version, configuration, artifact hash, license, and source-availability evidence must be recorded by the release gate.

## Go

Project: <https://go.dev/>

Go's toolchain and standard library are BSD-style licensed. The helper binary is compiled with the Go standard library. The Go toolchain itself is not placed in FramePier release packages.

## Included license texts for v0.2.2

The v0.2.2 installer includes `licenses/GO-LICENSE.txt` from the build toolchain, `licenses/FFMPEG-GPL-3.0.txt` from the verified FFmpeg 8.1.1 essentials archive, and `licenses/YT-DLP-2026.08.19-NOTICES.txt` from the official Windows archive.

The yt-dlp archive SHA-256 recorded for that build is:

`30b4c14aafab6082becff7881e41b76df46dc43ea7633479410a91e29da492bf`

It was verified against that release's official checksum list.

## Redistribution and release gate

License texts and checksum records are not substitutes for complete corresponding source where a redistributed license requires it.

For every future public binary release, FramePier must assemble and verify the required corresponding source, dependency sources, build scripts, notices, exact artifact hashes, and source-availability evidence for the redistributed tools. A link to a moving upstream branch is not treated as sufficient evidence.

The public v0.2.2 assets were published before this gate was fully completed and do not include a dedicated corresponding-source bundle for the redistributed GPL tools. Do not use v0.2.2 as the compliance baseline for a future release. The next public binary release remains blocked until the current dependency and redistribution gate is completed.

Changing bundled dependencies requires refreshing these notices and repeating the release gate. FramePier's MIT license applies to FramePier's own code, not to bundled third-party executables.
