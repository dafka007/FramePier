import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

function node(id) {
  const classes = new Set(id === 'thumbnailPlaceholder' ? ['hidden'] : []);
  const attrs = {};
  return {
    id, textContent: '', disabled: false, style: {}, dataset: {}, handlers: {}, options: [], selectedOptions: [], value: '',
    addEventListener(event, fn) { this.handlers[event] = fn; },
    replaceChildren() {}, append() {},
    removeAttribute(name) { delete attrs[name]; if (name === 'src') this._src = undefined; },
    set src(value) { this._src = value; }, get src() { return this._src; },
    classList: { add: c => classes.add(c), remove: c => classes.delete(c), contains: c => classes.has(c), toggle: (c, on) => on ? classes.add(c) : classes.delete(c) }
  };
}

function popup() {
  const html = readFileSync(new URL('../extension/popup.html', import.meta.url), 'utf8');
  const nodes = Object.fromEntries([...html.matchAll(/id="([^"]+)"/g)].map(([, id]) => [id, node(id)]));
  const context = vm.createContext({
    document: { querySelectorAll: () => Object.values(nodes), addEventListener: () => {}, createElement: () => ({ dataset: {} }) },
    chrome: { runtime: { onMessage: { addListener: () => {} }, sendMessage: async () => ({ ok: true }) } },
    console: { error: () => {} }, URL
  });
  vm.runInContext(readFileSync(new URL('../extension/popup.js', import.meta.url), 'utf8'), context);
  return { nodes, context };
}

const video = (thumbnail) => ({ id: 'xmo8jh', title: 'T', channel: 'C', duration: 126, url: 'https://www.dailymotion.com/video/xmo8jh', sourceLabel: 'Dailymotion', thumbnail, qualities: [] });
const shown = (n) => !n.classList.contains('hidden');

test('an empty thumbnail URL shows the placeholder and hides the image (no broken-image icon)', () => {
  const { nodes, context } = popup();
  context.renderVideo(video(''));
  assert.equal(shown(nodes.thumbnail), false);
  assert.equal(shown(nodes.thumbnailPlaceholder), true);
  assert.equal(nodes.thumbnail.src, undefined);
});

test('an unsafe thumbnail host is never loaded and shows the placeholder', () => {
  const { nodes, context } = popup();
  context.renderVideo(video('https://s1.dmcdn.net/v/abc/x240'));
  assert.equal(nodes.thumbnail.src, undefined);
  assert.equal(shown(nodes.thumbnail), false);
  assert.equal(shown(nodes.thumbnailPlaceholder), true);
});

test('a safe thumbnail is shown and the placeholder is hidden', () => {
  const { nodes, context } = popup();
  context.renderVideo(video('https://i.ytimg.com/vi/abc/hq.jpg'));
  assert.equal(nodes.thumbnail.src, 'https://i.ytimg.com/vi/abc/hq.jpg');
  assert.equal(shown(nodes.thumbnail), true);
  assert.equal(shown(nodes.thumbnailPlaceholder), false);
});

test('a safe thumbnail that fails to load falls back to the placeholder', () => {
  const { nodes, context } = popup();
  context.renderVideo(video('https://static-cdn.jtvnw.net/x.jpg'));
  nodes.thumbnail.handlers.error();
  assert.equal(shown(nodes.thumbnail), false);
  assert.equal(shown(nodes.thumbnailPlaceholder), true);
  // A later video with a good thumbnail restores the image.
  context.renderVideo(video('https://i.ytimg.com/vi/abc/hq.jpg'));
  assert.equal(shown(nodes.thumbnail), true);
  assert.equal(shown(nodes.thumbnailPlaceholder), false);
});

test('a finished download shows the real file size, not the progress estimate', () => {
  const { nodes, context } = popup();
  context.renderProgress({ state: 'finished', percent: 100, downloadedBytes: 76861645, totalBytes: 76861645, outputBytes: 22822230, outputPath: 'C:/x/video.mp4' });
  assert.match(nodes.etaText.textContent, /^Final file size: 21\.8 MiB \(22,822,230 bytes\)$/);
  assert.ok(!nodes.etaText.textContent.includes('73.3'));
});

test('the final file size uses binary unit names for each magnitude', () => {
  const { nodes, context } = popup();
  for (const [bytes, text] of [[512, '512 B'], [2048, '2 KiB'], [5 * 1024 * 1024, '5.0 MiB'], [3 * 1024 ** 3, '3.0 GiB']]) {
    context.renderProgress({ state: 'finished', percent: 100, outputBytes: bytes });
    assert.ok(nodes.etaText.textContent.startsWith(`Final file size: ${text} (`), nodes.etaText.textContent);
  }
});

test('running progress still shows downloaded / total, and finished without a size falls back', () => {
  const { nodes, context } = popup();
  context.renderProgress({ state: 'downloading', percent: 50, downloadedBytes: 1048576, totalBytes: 2097152, eta: '00:05' });
  assert.equal(nodes.etaText.textContent, '1.0 MB / 2.0 MB · ETA 00:05');
  context.renderProgress({ state: 'downloading', percent: 99, downloadedBytes: 1048576, totalBytes: 2097152, outputBytes: 5 });
  assert.equal(nodes.etaText.textContent, '1.0 MB / 2.0 MB');
  context.renderProgress({ state: 'finished', percent: 100, downloadedBytes: 1048576, totalBytes: 2097152, outputPath: 'C:/x/a.mp4' });
  assert.equal(nodes.etaText.textContent, '1.0 MB / 2.0 MB');
});

test('a finished download hides speed and ETA but keeps percent, bar, size, name and buttons', () => {
  const { nodes, context } = popup();
  context.renderProgress({ state: 'finished', percent: 100, speed: '3.2MiB/s', eta: '00:00', downloadedBytes: 100, totalBytes: 100, outputBytes: 22822230, outputPath: 'C:/x/video.mp4' });
  assert.equal(nodes.speedText.textContent, '');
  assert.ok(!nodes.etaText.textContent.includes('ETA'));
  assert.equal(nodes.etaText.textContent, 'Final file size: 21.8 MiB (22,822,230 bytes)');
  assert.equal(nodes.statusText.textContent, 'Finished');
  assert.equal(nodes.percentText.textContent, '100%');
  assert.equal(nodes.progressBar.style.width, '100%');
  assert.equal(nodes.outputText.textContent, 'Saved as video.mp4');
  assert.equal(nodes.openFolderButton.classList.contains('hidden'), false);
  assert.equal(nodes.newDownloadButton.classList.contains('hidden'), false);
  assert.equal(nodes.cancelButton.classList.contains('hidden'), true);
});

test('a finished download without a known size shows no speed or ETA either', () => {
  const { nodes, context } = popup();
  context.renderProgress({ state: 'finished', percent: 100, speed: '1MiB/s', eta: '00:00', outputPath: 'C:/x/a.mp4' });
  assert.equal(nodes.speedText.textContent, '');
  assert.equal(nodes.etaText.textContent, '');
});

test('active downloads keep live speed, ETA and byte progress', () => {
  const { nodes, context } = popup();
  for (const state of ['downloading', 'merging', 'converting_audio', 'finalizing']) {
    context.renderProgress({ state, percent: 42.5, speed: '3.2MiB/s', eta: '00:07', downloadedBytes: 1048576, totalBytes: 2097152 });
    assert.equal(nodes.speedText.textContent, '3.2MiB/s', state);
    assert.equal(nodes.etaText.textContent, '1.0 MB / 2.0 MB · ETA 00:07', state);
    assert.equal(nodes.percentText.textContent, '42.5%');
    assert.equal(nodes.cancelButton.classList.contains('hidden'), false);
  }
});

test('failed and cancelled states are unchanged and show their messages', () => {
  const { nodes, context } = popup();
  context.renderProgress({ state: 'failed', percent: 30, speed: '1MiB/s', eta: '00:09', error: 'Download failed.' });
  assert.equal(nodes.statusText.textContent, 'Failed');
  assert.equal(nodes.outputText.textContent, 'Download failed.');
  assert.equal(nodes.speedText.textContent, '1MiB/s');
  assert.equal(nodes.etaText.textContent, 'ETA 00:09');
  assert.equal(nodes.openFolderButton.classList.contains('hidden'), true);
  assert.equal(nodes.newDownloadButton.classList.contains('hidden'), false);
  context.renderProgress({ state: 'cancelled', percent: 12 });
  assert.equal(nodes.statusText.textContent, 'Cancelled');
  assert.equal(nodes.newDownloadButton.classList.contains('hidden'), false);
});

test('the thumbnail markup keeps a fixed-size frame and the manifest image policy is unchanged', () => {
  const html = readFileSync(new URL('../extension/popup.html', import.meta.url), 'utf8');
  assert.match(html, /class="thumbnail-frame"[\s\S]*id="thumbnail"[\s\S]*id="thumbnailPlaceholder"/);
  const manifest = JSON.parse(readFileSync(new URL('../extension/manifest.json', import.meta.url), 'utf8'));
  assert.ok(manifest.content_security_policy.extension_pages.includes("img-src 'self' https://i.ytimg.com https://*.ytimg.com https://static-cdn.jtvnw.net data:"));
});
