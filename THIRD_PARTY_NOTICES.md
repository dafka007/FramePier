# Third-party notices

FramePier release packages can include the following unmodified executable tools. They are not covered by FramePier's MIT license.

## yt-dlp

Project: <https://github.com/yt-dlp/yt-dlp>

yt-dlp itself is licensed under The Unlicense (public domain). The official Windows executable is a PyInstaller-bundled release that includes third-party components under additional licenses (including GPLv3+). Its release includes `THIRD_PARTY_LICENSES.txt`. Redistributors must review and comply with the exact release's applicable license terms and preserve required notices and source availability.

## Deno

Project: <https://github.com/denoland/deno>

Deno is bundled only as the JavaScript runtime that yt-dlp uses to solve YouTube's extraction challenges. FramePier selects it explicitly by absolute path and never uses a runtime found on PATH.

Selected build: Deno 2.9.7, `deno-x86_64-pc-windows-msvc.zip` from the official release tag `v2.9.7`
Archive SHA-256: `a0c3101b4158d1dfb7d6a78a7bf0f3de80c96bb423c152beec8beb22786f2238`, verified against the `.sha256sum` file published with that release (the build script checks both)
Extracted `deno.exe` SHA-256: `e020f3e232bd16e33768dee528e5983349c962952051ced0a5d58ad42f5d9b33`

Deno is licensed under the MIT License (`licenses/DENO-2.9.7-MIT.txt`, Deno's `LICENSE.md` at tag `v2.9.7`). The release archive contains only `deno.exe` and no consolidated third-party notice file; the binary statically includes third-party components (for example the V8 JavaScript engine and Rust libraries) under their own licenses. Redistributors should review the upstream project for those notices.

## FFmpeg

Project: <https://ffmpeg.org/>

Windows build provider: <https://github.com/BtbN/FFmpeg-Builds>

Selected build: `ffmpeg-n8.1.2-50-g1a748fe2cd-win64-lgpl-8.1.zip`
Release tag: `autobuild-2026-08-31-13-27`
BtbN build-repository commit: `8267213e26c1031621e6e1210fe3aa4867214f6a`
FFmpeg source commit: `1a748fe2cd43e3ead22fafb1b5b7d77f153898a8`
Archive SHA-256: `f6274bbd9c247f9e90c1bbed066b03ed4a3907cece2fb91be6dd352393936365`

The selected build is LGPL-3.0-or-later. BtbN's LGPL variant uses `--enable-version3`. Redistributors must comply with the applicable LGPL terms.

The corresponding source for the FFmpeg binary is available at the pinned commit; dependency source commits are pinned in the build scripts. Byte-identical reproducibility has not been independently demonstrated (the build embeds a date string), but functional reproduction is achievable from the published scripts. A partial corresponding-source bundle has been assembled (it is not complete; see the redistribution gate below): `FramePier-0.2.3-FFmpeg-Source/` contains the FFmpeg source at commit `1a748fe2cd43e3ead22fafb1b5b7d77f153898a8` plus the BtbN build scripts at commit `8267213e26c1031621e6e1210fe3aa4867214f6a`. Archive: `FramePier-0.2.3-FFmpeg-Source.zip` (SHA-256: `185c4f5a431c1bfe15c34184277b3a02d4d6b034b91921cd0ddd3648dd833b95`).

The v0.2.2 build used a Windows FFmpeg build from Gyan, a provider linked by ffmpeg.org (the GPL-licensed essentials build). FramePier 0.2.3 replaces it with the BtbN LGPL build described above. The exact provider, version, configuration, artifact hash, license, and source-availability evidence for any future release must be recorded by the release gate.

## Go

Project: <https://go.dev/>

Go's toolchain and standard library are BSD-style licensed. The helper binary is compiled with the Go standard library. The Go toolchain itself is not placed in FramePier release packages.

## Included license texts

The installer includes `licenses/GO-LICENSE.txt` from the build toolchain,
`licenses/FFMPEG-LGPL-3.0.txt` for the current BtbN LGPL-3.0 build,
`licenses/DENO-2.9.7-MIT.txt` for the bundled Deno runtime,
and `licenses/YT-DLP-2026.08.19-NOTICES.txt` from the official Windows archive.
The latter archive's SHA-256 is
`30b4c14aafab6082becff7881e41b76df46dc43ea7633479410a91e29da492bf`,
verified against that release's official checksum list.

The v0.2.2 installer included `licenses/GO-LICENSE.txt` from the build toolchain, `licenses/FFMPEG-GPL-3.0.txt` from the verified FFmpeg 8.1.1 essentials archive, and `licenses/YT-DLP-2026.08.19-NOTICES.txt` from the official Windows archive. The GPL text is retained in the v0.2.2 tag and Git history and is not part of the 0.2.3 license set.

## Redistribution and release gate

License texts and checksum records are not substitutes for complete corresponding source where a redistributed license requires it.

For every future public binary release, FramePier must assemble and verify the required corresponding source, dependency sources, build scripts, notices, exact artifact hashes, and source-availability evidence for the redistributed tools. A link to a moving upstream branch is not treated as sufficient evidence.

**Redistribution gate for FramePier 0.2.3: PENDING / NOT CLEARED.** The FFmpeg corresponding source bundle (`FramePier-0.2.3-FFmpeg-Source/`, archive SHA-256 `185c4f5a431c1bfe15c34184277b3a02d4d6b034b91921cd0ddd3648dd833b95`) contains the FFmpeg source at commit `1a748fe2cd43e3ead22fafb1b5b7d77f153898a8` and the BtbN build scripts at commit `8267213e26c1031621e6e1210fe3aa4867214f6a`, but a compliance review found the following items unresolved. Do not publish a binary release until they are closed:

- **Third-party source and license/notice coverage:** the bundle does not include the source trees of the external libraries compiled into the bundled FFmpeg executable (including LGPL libraries such as libmp3lame), the compliance manifest does not yet list exactly the libraries enabled in this build with their exact revisions and licenses, and no per-library license/copyright notices ship with the installer.
- **Exact source distribution:** the exact FFmpeg configure line is not delivered as a separate file, and the source that corresponds to the shipped static executable (and how it can be rebuilt or relinked) has not been established. Publishing source trees alone does not automatically resolve every LGPL obligation for a statically linked executable; this needs further review.
- **Release publication:** the source archive is not yet published alongside the binary release it corresponds to.

This status will remain PENDING until these items are addressed and re-reviewed.

The public v0.2.2 assets were published before this gate was fully completed and do not include a dedicated corresponding-source bundle for the redistributed GPL tools. Do not use v0.2.2 as the compliance baseline for a future release. The next public binary release remains blocked until the current dependency and redistribution gate is completed.

Changing bundled dependencies requires refreshing these notices and repeating the release gate. FramePier's MIT license applies to FramePier's own code, not to bundled third-party executables.
