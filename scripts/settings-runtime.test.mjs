import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

function settingsHarness(jsRuntime, { omitRuntime = false, ffmpegVersion = 'ffmpeg version 8.1.2-50' } = {}) {
  const html = readFileSync(new URL('../extension/settings.html', import.meta.url), 'utf8');
  const nodes = Object.fromEntries([...html.matchAll(/id="([^"]+)"/g)].map(([, id]) => [id, {
    id, textContent: '', value: '', checked: false, disabled: false, style: {}, addEventListener() {}
  }]));
  const versions = {
    helper: { version: '0.2.3' },
    ytDlp: { installed: true, version: '2026.08.19' },
    ffmpeg: { installed: true, version: ffmpegVersion }
  };
  if (!omitRuntime) versions.jsRuntime = jsRuntime;
  const context = vm.createContext({
    document: { getElementById: id => nodes[id], addEventListener() {} },
    navigator: { userAgent: 'Chrome/test' }, console,
    chrome: {
      runtime: {
        onMessage: { addListener() {} },
        getManifest: () => ({ version: '0.2.3' }),
        sendMessage: async message => {
          if (message.command === 'get_versions') return { ok: true, result: { versions } };
          if (message.command === 'get_settings') return { ok: true, result: { settings: { downloadPath: 'C:/x', autoOpen: false } } };
          return { ok: true, result: {} };
        }
      },
      storage: { sync: { get: async d => d, set: async () => {} }, local: { get: async d => d } }
    }
  });
  vm.runInContext(readFileSync(new URL('../extension/settings.js', import.meta.url), 'utf8'), context);
  return { nodes, context };
}

test('Settings shows the usable bundled Deno version', async () => {
  const { nodes, context } = settingsHarness({ installed: true, usable: true, version: '2.9.7', name: 'Deno' });
  await vm.runInContext('refresh()', context);
  assert.equal(nodes.jsRuntimeStatus.textContent, 'Installed · Deno 2.9.7');
  assert.equal(nodes.ffmpegVersion.textContent, 'Installed · 8.1.2');
});

test('Settings reports an installed but unusable runtime with a remedy', async () => {
  const { nodes, context } = settingsHarness({ installed: true, usable: false, version: '' });
  await vm.runInContext('refresh()', context);
  assert.match(nodes.jsRuntimeStatus.textContent, /Not working/);
  assert.match(nodes.jsRuntimeStatus.textContent, /reinstall FramePier/);
});

test('Settings reports a missing runtime and that YouTube is affected', async () => {
  const { nodes, context } = settingsHarness({ installed: false, usable: false, version: '' });
  await vm.runInContext('refresh()', context);
  assert.match(nodes.jsRuntimeStatus.textContent, /Missing/);
  assert.match(nodes.jsRuntimeStatus.textContent, /YouTube unavailable/);
});

test('Settings tolerates an older helper that reports no runtime, without breaking other fields', async () => {
  const { nodes, context } = settingsHarness(undefined, { omitRuntime: true });
  await vm.runInContext('refresh()', context);
  assert.equal(nodes.jsRuntimeStatus.textContent, 'Unknown');
  assert.equal(nodes.nativeStatus.textContent, 'Connected');
  assert.equal(nodes.ytdlpVersion.textContent, 'Installed · 2026.08.19');
});

test('Settings shows the abbreviated commit hash of an untagged shared FFmpeg build', async () => {
  const { nodes, context } = settingsHarness({ installed: true, usable: true, version: '2.9.7' }, { ffmpegVersion: 'ffmpeg version 1a748fe Copyright (c) 2000-2026 the FFmpeg developers' });
  await vm.runInContext('refresh()', context);
  assert.equal(nodes.ffmpegVersion.textContent, 'Installed · 1a748fe');
});

test('Settings still reports an unrecognised FFmpeg version string as unknown', async () => {
  for (const version of ['ffmpeg version N-abc', 'ffmpeg version git-2026-08-31-1a748fe', '']) {
    const { nodes, context } = settingsHarness({ installed: true, usable: true, version: '2.9.7' }, { ffmpegVersion: version });
    await vm.runInContext('refresh()', context);
    assert.equal(nodes.ffmpegVersion.textContent, 'Installed · version unknown', version);
  }
});
