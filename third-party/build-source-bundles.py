"""Builds the two source-bundle zips (FFmpeg/LAME and yt-dlp) published next to the FramePier 0.2.3 installer.

Usage: python3 -I build-source-bundles.py <repo> <inputs-dir> <out-dir>
<inputs-dir> holds the FFmpeg/LAME archives named below, the yt-dlp component archives in its ytdlp/ subfolder (git archives made with
`git archive --format=tar --prefix=<name>/ <commit> | xz -9e`, plus the LAME tarball).
Entries use forward slashes and a fixed timestamp so the result does not depend on the builder.
"""
import hashlib, sys, zipfile
from pathlib import Path

repo, inputs, out = (Path(a) for a in sys.argv[1:4])
out.mkdir(parents=True, exist_ok=True)
FIXED = (2026, 10, 11, 0, 0, 0)

def sha(p): return hashlib.sha256(Path(p).read_bytes()).hexdigest()

def build(zip_name, root, files):
    target = out / zip_name
    with zipfile.ZipFile(target, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as z:
        for arc, data in sorted(files.items()):
            info = zipfile.ZipInfo(f'{root}/{arc}', FIXED)
            info.compress_type = zipfile.ZIP_DEFLATED
            info.external_attr = 0o644 << 16
            z.writestr(info, data if isinstance(data, bytes) else Path(data).read_bytes())
    print(zip_name, sha(target))

def expect(path, digest):
    if sha(path) != digest: raise SystemExit(f'hash mismatch: {path}')
    return path

ffmpeg = expect(inputs / 'ffmpeg-1a748fe2cd43e3ead22fafb1b5b7d77f153898a8.tar.xz', '84005145844bbeed777c150859edae189c797f92bbcd81f5f55fa50f2312eb27')
lame = expect(inputs / 'lame-3.100.tar.gz', 'ddfe36cab873794038ae2c1210557ad34857a4b6bdc515785d1da9e175b1da1e')
files = {ffmpeg.name: ffmpeg, lame.name: lame,
         'README.txt': (inputs / 'FFMPEG-README.txt'), 'BUILD-INSTRUCTIONS.txt': (inputs / 'BUILD-INSTRUCTIONS.txt')}
for p in (repo / 'third-party' / 'ffmpeg-shared').iterdir(): files[f'third-party/ffmpeg-shared/{p.name}'] = p
for n in ('FFMPEG-LGPL-2.1.txt', 'LAME-LGPL-2.0.txt', 'WINPTHREADS-MINGW-W64.txt', 'MINGW-W64-RUNTIME.txt', 'FFMPEG-BUILD-PROVENANCE.txt'):
    files[f'licenses/{n}'] = repo / 'licenses' / n
build('FramePier-0.2.3-FFmpeg-LAME-Source.zip', 'FramePier-0.2.3-FFmpeg-LAME-Source', files)

files = {f'sources/{p.name}': p for p in sorted((inputs / 'ytdlp').iterdir())}
files['README.txt'] = inputs / 'YTDLP-README.txt'
for n in ('YT-DLP-2026.08.19-NOTICES.txt', 'YT-DLP-PYTHON-RUNTIME-NOTICES.txt', 'GPL-3.0.txt'):
    files[f'licenses/{n}'] = repo / 'licenses' / n
files['build/ytdlp-openssl3-experiment.workflow.yml.txt'] = repo / 'third-party' / 'ytdlp-openssl3' / 'ytdlp-openssl3-experiment.workflow.yml.txt'
for p in sorted((repo / 'third-party' / 'ytdlp-openssl3' / 'evidence').iterdir()): files[f'build/evidence/{p.name}'] = p
build('FramePier-0.2.3-YtDlp-Source.zip', 'FramePier-0.2.3-YtDlp-Source', files)
