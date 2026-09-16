# Third-party notices

VidDock release packages can include the following unmodified executable tools. They are not covered by VidDock's MIT license.

## yt-dlp

Project: <https://github.com/yt-dlp/yt-dlp>

The official Windows executable is a PyInstaller-bundled release and includes components under GPLv3+ and other licenses. Its release includes `THIRD_PARTY_LICENSES.txt`. Redistributors must review and comply with the release's applicable license terms and preserve required notices/source availability.

## FFmpeg

Project: <https://ffmpeg.org/>

Windows build provider: <https://www.gyan.dev/ffmpeg/builds/>, as linked by ffmpeg.org.

The selected essentials build is GPL-licensed. Redistributors must comply with the corresponding GPL terms and make the complete corresponding source available as required. See the build provider's license/source information for the exact packaged build.

## Go

Project: <https://go.dev/>

Go's toolchain and standard library are BSD-style licensed. The helper binary is compiled with the Go standard library. The portable build toolchain is not placed in VidDock release packages.

## Included license texts

The installer includes `licenses/GO-LICENSE.txt` from the build toolchain,
`licenses/FFMPEG-GPL-3.0.txt` from the verified FFmpeg 8.1.1 essentials archive,
and `licenses/YT-DLP-2026.08.19-NOTICES.txt` from the official Windows archive.
The latter archive's SHA-256 is
`30b4c14aafab6082becff7881e41b76df46dc43ea7633479410a91e29da492bf`,
verified against that release's official checksum list.

## Redistribution gate

These texts are not a substitute for complete corresponding source. Before
publishing a binary release, assemble and verify the corresponding source,
dependency sources, and build scripts for the exact redistributed GPL tools.
Do not assume a link to a moving upstream branch satisfies these obligations.
This source bundle has not yet been assembled for the local 0.2.2 audit build;
that build must not be described as cleared for public redistribution.

Changing the bundled dependencies requires refreshing the notices and reviewing
source availability again. VidDock's MIT license applies to its own code, not
to the bundled third-party executables.
