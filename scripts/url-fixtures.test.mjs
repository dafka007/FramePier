import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

// The same fixture file drives helper/src/url_fixtures_test.go, so the
// extension's parseMediaUrl and the Go validator cannot drift apart silently.
const cases = JSON.parse(readFileSync(new URL('./fixtures/url-cases.json', import.meta.url), 'utf8'));

function loadPopup() {
  const html = readFileSync(new URL('../extension/popup.html', import.meta.url), 'utf8');
  const nodes = Object.fromEntries([...html.matchAll(/id="([^"]+)"/g)].map(([, id]) => {
    const classes = new Set();
    return [id, {id, textContent:'', disabled:false, style:{}, handlers:{},
      addEventListener(event, fn) { this.handlers[event] = fn; },
      classList:{add:c => classes.add(c),remove:c => classes.delete(c),contains:c => classes.has(c),toggle:(c,on) => on ? classes.add(c) : classes.delete(c)}
    }];
  }));
  const context = vm.createContext({
    document:{querySelectorAll:()=>Object.values(nodes),addEventListener:()=>{}},
    chrome:{runtime:{onMessage:{addListener:()=>{}},sendMessage:async()=>({ok:true})}},
    console:{error:()=>{}}, URL
  });
  vm.runInContext(readFileSync(new URL('../extension/popup.js', import.meta.url), 'utf8'), context);
  return context;
}

const popup = loadPopup();
const parse = (url) => popup.parseMediaUrl(url);

for (const entry of cases) {
  if (entry.accept) {
    test(`accepts ${entry.url}`, () => {
      const parsed = parse(entry.url);
      assert.equal(parsed.source, entry.source);
      assert.equal(parsed.videoId, entry.id);
      assert.equal(parsed.url, entry.canonical);
    });
  } else {
    test(`rejects ${entry.url} (${entry.why})`, () => {
      // The error comes from another realm (vm context), so Error is not comparable.
      assert.throws(() => parse(entry.url), (error) => typeof error?.message === 'string' && error.message.length > 0);
    });
  }
}

test('Dailymotion canonical URLs never carry untrusted components', () => {
  for (const entry of cases.filter((item) => item.accept && item.source === 'dailymotion')) {
    const parsed = parse(entry.url);
    assert.match(parsed.url, /^https:\/\/www\.dailymotion\.com\/video\/x[0-9a-z]{4,10}$/);
  }
});

test('Dailymotion credentials and ports are rejected with a Dailymotion message', () => {
  assert.throws(() => parse('https://user:pass@www.dailymotion.com/video/xmo8jh'), /Dailymotion/);
  assert.throws(() => parse('https://www.dailymotion.com:8443/video/xmo8jh'), /Dailymotion/);
});

test('unsupported pages mention every supported source', () => {
  assert.throws(() => parse('https://example.com/'), /YouTube.*Twitch.*Dailymotion/);
});

test('Dailymotion thumbnails are never displayed', () => {
  for (const url of ['https://s1.dmcdn.net/v/abc/x240', 'https://s2.dmcdn.net/v/abc/x240', 'https://www.dailymotion.com/thumbnail/video/xmo8jh', 'https://evil.test/i.ytimg.com']) {
    assert.equal(popup.isSafeThumbnail(url), false, url);
  }
  assert.equal(popup.isSafeThumbnail('https://i.ytimg.com/vi/abc/hq.jpg'), true);
  assert.equal(popup.isSafeThumbnail('https://static-cdn.jtvnw.net/x.jpg'), true);
  assert.equal(popup.isSafeThumbnail(''), false);
});

test('manifest permissions and image policy were not widened for Dailymotion', () => {
  const manifest = JSON.parse(readFileSync(new URL('../extension/manifest.json', import.meta.url), 'utf8'));
  assert.deepEqual(manifest.host_permissions, [
    'https://youtube.com/*', 'https://www.youtube.com/*', 'https://m.youtube.com/*', 'https://music.youtube.com/*', 'https://youtu.be/*',
    'https://twitch.tv/*', 'https://www.twitch.tv/*', 'https://m.twitch.tv/*', 'https://clips.twitch.tv/*'
  ]);
  assert.deepEqual(manifest.permissions, ['activeTab', 'alarms', 'nativeMessaging', 'storage']);
  const csp = manifest.content_security_policy.extension_pages;
  assert.ok(!/dailymotion|dmcdn/i.test(csp), csp);
  assert.ok(csp.includes("img-src 'self' https://i.ytimg.com https://*.ytimg.com https://static-cdn.jtvnw.net data:"));
});
