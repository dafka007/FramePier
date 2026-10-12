# FramePier 0.2.3 redistribution gate

**Status: documentation complete; owner sign-off pending (see Owner decisions).** This file is a repository document; it is not installed. A working build or a source archive is not a compliance finding.

## Component status

| Component | Status | Evidence / remaining |
|---|---|---|
| FFmpeg + LAME + 9 DLLs | Material ready; publication and owner review pending | `FramePier-0.2.3-FFmpeg-LAME-Source.zip` (SHA-256 `61b8539d5866b899ca173a7edb71502ff78d0b090500990f7442f36b54b1327a`, reproduced byte-identically by `third-party/build-source-bundles.py`) holds the FFmpeg commit, the LAME 3.100 tarball (hash and `COPYING` checked), `BUILD-INSTRUCTIONS.txt`, configuration, package versions, workflow and licenses. Must be attached to the same release as the installer. One build run; not shown to be byte-reproducible. |
| yt-dlp 2026.08.19 (FramePier build, Python 3.13.16 / OpenSSL 3.5.9) | Source and notices complete; OpenSSL 1.1.1t issue removed | See below. |
| Deno 2.9.7 | Notices compiled; two input steps recorded below | See below. |
| winpthreads / MinGW-w64 runtime | Done | License texts match mingw-w64 revision `4564ee4b5063097bf747af3a3f8270a28adff820` byte for byte. |
| Inno Setup | Closed | `jrsoftware/issrc` `license.txt` permits any purpose "including commercial applications"; the copyright notices in the setup program must stay. The FAQ position was supplied by the owner; the installed version's `license.txt` was not read. |
| Go standard library | Unchanged | `licenses/GO-LICENSE.txt`. |

## yt-dlp

Evidence (VERIFIED): `yt-dlp.exe` SHA-256 `8b976e7ff71a6f8e8dd48feb2920b3c41a44641ffe192dba08cde0175890ee44` (22,349,496 bytes) was produced by the workflow in `third-party/ytdlp-openssl3/ytdlp-openssl3-experiment.workflow.yml.txt` (branch commit `08a4a4f8f4965b6a6fc2376722b68b2cfc14202b`; yt-dlp commit `3a08beaf031ab68f966401ead017ac81fe8486cf`, tag 2026.08.19, verified in the run), Python 3.13.16, hash-locked upstream requirements. The PyInstaller archive was opened: it contains python313.dll, libcrypto-3.dll (string "OpenSSL 3.5.9 29 Sep 2026"), libssl-3.dll, sqlite3.dll 3.50.4, libffi-8.dll, curl_cffi 0.16.0 with libcurl-impersonate, websockets 17.0.1, mutagen 1.48.1 (GPL-2.0-or-later), certifi (MPL-2.0); no OpenSSL 1.1 DLL. Its embedded license list is byte-identical to the previous one and to `licenses/YT-DLP-2026.08.19-NOTICES.txt`. The CI run printed the curl_cffi impersonation targets (Chrome, Safari, Firefox, Edge...) and `--version` 2026.08.19.

OpenSSL: OpenSSL 3.x is Apache-2.0, which is compatible with GPLv3; the earlier OpenSSL/SSLeay-license problem of OpenSSL 1.1.1t no longer applies to this executable.

Mechanism: the executable is conveyed under GPLv3 (mutagen is "or later"). GPLv3 section 6(d) lets a distributor who offers the installer from a network server provide the Corresponding Source from the same place with clear directions next to the download, and requires it to stay available. `FramePier-0.2.3-YtDlp-Source.zip` (SHA-256 `821539ef5b1bf853d1c9cc4a7d2860b7fa9350cbf5ebcb76e2d39629a889db6e`) and `licenses/SOURCE-AVAILABILITY.txt` implement that. This is a standing duty while the installer is distributed, not a time-limited written offer; the unapproved offer text stays excluded (`third-party/DRAFT-SOURCE-OFFER-NOT-APPROVED.txt`). yt-dlp's own statement ("the maintainers will provide it") binds yt-dlp, not FramePier, and is not relied on.

Remaining (stated limits):
1. The Python installation, wheels and libcurl-impersonate binary GitHub's runner used are binaries; every component is included as source at the version found in the executable or pinned by yt-dlp. Microsoft runtime DLLs inside the executable are system components. Byte-identical rebuilds were not attempted.
2. The in-app yt-dlp updater is removed in 0.2.3 (`update_ytdlp` rejected by the helper, Settings button deleted), so the pinned executable is not replaced at run time.
3. The release must carry the two source zips and the directions in its description, and they must stay available.

## Deno

Evidence (VERIFIED): the release archive contains only `deno.exe`; the repository at `v2.9.7` has `LICENSE.md` and no consolidated third-party notice. `Cargo.lock` (`third-party/deno/Cargo.lock`) lists 1,128 packages: 82 Deno workspace crates (MIT) and 1,046 crates.io crates. The license table (`third-party/deno/deno-v2.9.7-licenses.tsv`) shows no GPL- or LGPL-only crate (`r-efi` offers MIT OR Apache-2.0 OR LGPL-2.1-or-later; MIT used). Seven crates are MPL-2.0 (cooked-waker, cssparser, cssparser-macros, dtoa-short, smartstring, webpki-roots, webpki-root-certs); they are unmodified, so the license text is kept and their source is on crates.io at the locked versions. `libuv-sys-lite`'s "non-standard" license is MIT plus the libuv license (read from the crate).

Notices compiled: `licenses/DENO-2.9.7-CRATE-LICENSES.txt` (all license/notice files of 930 crates that ship them, 574 distinct texts, each listed with the crates it covers; all 1,046 crates accounted for), `licenses/DENO-V8-LICENSES.txt` (V8, rusty_v8 and the build libraries FP16, Highway, fast_float, dragonbox, simdutf, abseil-cpp, ICU, LLVM libc++/libc++abi/libunwind/llvm-libc, Chromium, Rust std; taken from the projects' GitHub repositories, dragonbox at the pinned commit, the others at the default branch because Chromium's git host was not reachable), and `licenses/DENO-2.9.7-CRATE-AUTHORS.txt` (generated by `third-party/deno/collect-missing-crate-authors.ps1` for the 116 crates that ship no license file). Limit: the V8 library texts are not the exact pinned commits except dragonbox; terms are unchanged between them, copyright years may differ.

## Owner decisions

- Review the published release page text (both source zips and directions next to the installer).
- v0.2.2 remains public with a GPL FFmpeg and no source bundle: leave, replace or withdraw.
