# Third-party notices

FramePier release packages include the following executable tools, unmodified except where stated below. They are not covered by FramePier's MIT license. `licenses/SOURCE-AVAILABILITY.txt` says where the source for each is published.

## yt-dlp

Project: <https://github.com/yt-dlp/yt-dlp>

Bundled file: `tools\yt-dlp.exe`, FramePier's own build of yt-dlp 2026.08.19 (source tag commit `3a08beaf031ab68f966401ead017ac81fe8486cf`), SHA-256 `8b976e7ff71a6f8e8dd48feb2920b3c41a44641ffe192dba08cde0175890ee44`, 22,349,496 bytes. It is built with yt-dlp's own PyInstaller recipe, hash-locked requirements and build scripts, but on Python 3.13.16 instead of the Python 3.10 used for upstream's published asset, so that it embeds OpenSSL 3.5.9 (Apache-2.0) instead of OpenSSL 1.1.1t. It is not upstream's published binary.

yt-dlp itself is licensed under The Unlicense. The executable is a PyInstaller bundle of Python 3.13.16, OpenSSL 3.5.9, SQLite 3.50.4, libffi 3.4.4, curl_cffi 0.16.0 with libcurl-impersonate, websockets 17.0.1 and the other Python packages pinned in yt-dlp's `bundle/requirements/`. It contains GPL-2.0-or-later code (mutagen 1.48.1) and MPL-2.0 code (certifi 2026.07.22); the executable as a whole is therefore conveyed under the GNU General Public License, version 3 (`licenses/GPL-3.0.txt`). The complete license list shipped inside the executable (identical to `licenses/YT-DLP-2026.08.19-NOTICES.txt`) names every bundled component and license; `licenses/YT-DLP-PYTHON-RUNTIME-NOTICES.txt` adds the license texts of the CPython runtime and its zlib, bzip2, xz, expat and mpdecimal components. Components that yt-dlp's own list marks as "Only included in Linux builds" are not in this Windows executable. The Microsoft C/C++ runtime files inside the executable are Microsoft redistributable components.

Source: `FramePier-0.2.3-YtDlp-Source.zip` (SHA-256 `821539ef5b1bf853d1c9cc4a7d2860b7fa9350cbf5ebcb76e2d39629a889db6e`), published on the same release page as the installer. It contains the yt-dlp source at the tag, the build workflow that produced the executable together with its output evidence, the requirement pins, and the sources of CPython 3.13.16 and the libraries it links, mutagen, certifi, PyInstaller and the other Python packages, each at the exact version found in or pinned for the executable. FramePier has no in-app yt-dlp updater, so this file is not replaced at run time.

## Deno

Project: <https://github.com/denoland/deno>

Deno is bundled only as the JavaScript runtime that yt-dlp uses to solve YouTube's extraction challenges. FramePier selects it explicitly by absolute path and never uses a runtime found on PATH.

Selected build: Deno 2.9.7, `deno-x86_64-pc-windows-msvc.zip` from the official release tag `v2.9.7`
Archive SHA-256: `a0c3101b4158d1dfb7d6a78a7bf0f3de80c96bb423c152beec8beb22786f2238`, verified against the `.sha256sum` file published with that release (the build script checks both)
Extracted `deno.exe` SHA-256: `e020f3e232bd16e33768dee528e5983349c962952051ced0a5d58ad42f5d9b33`

Deno is licensed under the MIT License (`licenses/DENO-2.9.7-MIT.txt`, Deno's `LICENSE.md` at tag `v2.9.7`). `deno.exe` statically includes the V8 JavaScript engine and Rust libraries. The license texts that accompany the V8 engine used by this Deno version are in `licenses/DENO-V8-LICENSES.txt`. Deno does not publish a consolidated notice for its other statically linked libraries, so FramePier compiles one: `licenses/DENO-2.9.7-CRATE-LICENSES.txt` reproduces the license and notice files of the 1,046 crates.io packages in Deno's `Cargo.lock` at their locked versions (identical texts printed once with the crates they cover), `licenses/DENO-2.9.7-CRATE-AUTHORS.txt` records the declared license, authors and repository of the 116 crates that ship no license file, and `licenses/DENO-V8-LICENSES.txt` carries V8 and the Chromium, LLVM, ICU and other libraries linked into V8. Seven crates are MPL-2.0 (cooked-waker, cssparser, cssparser-macros, dtoa-short, smartstring, webpki-roots, webpki-root-certs); they are used unmodified and their source is published on crates.io at the versions in `Cargo.lock`. No Deno crate is GPL or LGPL only.

## FFmpeg (shared build) and LAME

Projects: <https://ffmpeg.org/>, <https://lame.sourceforge.io/>

FramePier 0.2.3 bundles a minimal shared FFmpeg build instead of the earlier static BtbN build. It is installed as `ffmpeg.exe`, `ffprobe.exe` and nine DLLs in the tools folder:

- FFmpeg libraries `avcodec-62`, `avdevice-62`, `avfilter-11`, `avformat-62`, `avutil-60`, `swresample-6`, `swscale-9`: FFmpeg commit `1a748fe2cd43e3ead22fafb1b5b7d77f153898a8` (RELEASE 8.1.2), built unmodified. The shipped build reports LGPL v2.1 or later; configuration has no `--enable-gpl`, `--enable-version3` or `--enable-nonfree`.
- `libmp3lame-0.dll`: LAME 3.100 (LGPL-2.0-or-later), `lame-3.100.tar.gz` SHA-256 `ddfe36cab873794038ae2c1210557ad34857a4b6bdc515785d1da9e175b1da1e`, with one build-time edit (the obsolete `lame_init_old` export line removed, upstream LAME bug #503).
- `libwinpthread-1.dll`: mingw-w64 winpthreads (permissive license), MSYS2 package `mingw-w64-ucrt-x86_64-libwinpthread 14.0.0.r426.g4564ee4b5-1`.

Configuration, MSYS2 package versions, build workflow and file SHA-256 values are recorded in `licenses/FFMPEG-BUILD-PROVENANCE.txt` (installed) and `third-party/ffmpeg-shared/` (repository). The build ran on GitHub Actions (MSYS2 UCRT64). Byte-identical rebuilds have not been demonstrated. The FFmpeg source archive (`ffmpeg-1a748fe2cd43e3ead22fafb1b5b7d77f153898a8.tar.xz`, a `git archive` of that commit, SHA-256 `84005145844bbeed777c150859edae189c797f92bbcd81f5f55fa50f2312eb27`) and the LAME tarball are packaged into `FramePier-0.2.3-FFmpeg-LAME-Source.zip` by `third-party/build-source-bundles.py`.

The previous static BtbN build (`ffmpeg-n8.1.2-50-g1a748fe2cd-win64-lgpl-8.1.zip`, 58 direct libraries) is no longer used. The v0.2.2 build used a Gyan GPL build.

## Go

Project: <https://go.dev/>

Go's toolchain and standard library are BSD-style licensed. The helper binary is compiled with the Go standard library. The Go toolchain itself is not placed in FramePier release packages.

## Included license texts

In `licenses/`: `GO-LICENSE.txt`; `FFMPEG-LGPL-2.1.txt` (`COPYING.LGPLv2.1` from the pinned FFmpeg commit); `LAME-LGPL-2.0.txt` (identical to `COPYING` in the official `lame-3.100.tar.gz`); `WINPTHREADS-MINGW-W64.txt` and `MINGW-W64-RUNTIME.txt` (from mingw-w64 revision `4564ee4b5063097bf747af3a3f8270a28adff820`); `GPL-3.0.txt`; `DENO-2.9.7-MIT.txt`; `DENO-V8-LICENSES.txt`; `YT-DLP-2026.08.19-NOTICES.txt`; `FFMPEG-BUILD-PROVENANCE.txt`; `SOURCE-AVAILABILITY.txt`.

The installer is built with Inno Setup (<https://jrsoftware.org/isinfo.php>), Copyright (C) 1997-2026 Jordan Russell and Martijn Laan. Its license permits use for any purpose, including commercial applications; the Inno Setup copyright notices in the setup program are not removed.

FramePier's MIT license applies to FramePier's own code, not to bundled third-party executables.
