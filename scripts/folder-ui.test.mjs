import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';

function popupHarness(sendMessage) {
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
    chrome:{runtime:{onMessage:{addListener:()=>{}},sendMessage}},
    console:{error:()=>{}}, URL
  });
  vm.runInContext(readFileSync(new URL('../extension/popup.js', import.meta.url), 'utf8'), context);
  return {nodes,context};
}

for (const source of ['YouTube','Twitch clip','Twitch VOD','MP3','M4A']) {
  test(`completed ${source} uses the fixed folder command without a stale job/file path`, async () => {
    const sent = [];
    const {nodes,context} = popupHarness(async message => { sent.push(message); return {ok:true}; });
    vm.runInContext(`jobId='old_job'; renderProgress({state:'finished',percent:100,outputPath:'C:/old location/title.mp4'})`, context);
    assert.equal(nodes.openFolderButton.classList.contains('hidden'), false);
    await nodes.openFolderButton.handlers.click();
    assert.equal(sent.length, 1);
    assert.equal(sent[0].command, 'open_download_folder');
    assert.equal(sent[0].id, null);
    assert.equal(Object.keys(sent[0].payload).length, 0);
    assert.equal(nodes.folderStatus.textContent, '');
    assert.equal(nodes.statusText.textContent, 'Finished');
    assert.equal(nodes.openFolderButton.disabled, false);
  });
}

test('folder failure is visible, preserves completion, and the button can retry', async () => {
  let fail = true;
  const {nodes,context} = popupHarness(async () => fail ? {ok:false,error:"VidDock couldn't open the download folder."} : {ok:true});
  vm.runInContext(`renderProgress({state:'finished',percent:100,outputPath:'C:/downloads/clip.mp4'})`, context);
  await nodes.openFolderButton.handlers.click();
  assert.match(nodes.folderStatus.textContent, /couldn't open/);
  assert.equal(nodes.statusText.textContent, 'Finished');
  assert.match(nodes.outputText.textContent, /clip.mp4/);
  assert.equal(nodes.openFolderButton.disabled, false);
  fail = false;
  await nodes.openFolderButton.handlers.click();
  assert.equal(nodes.folderStatus.textContent, '');
});

test('folder button is hidden on cancellation; download-another stays available', () => {
  const {nodes,context} = popupHarness(async () => ({ok:true}));
  vm.runInContext(`renderProgress({state:'cancelled',percent:20})`, context);
  assert.equal(nodes.openFolderButton.classList.contains('hidden'), true);
  assert.equal(nodes.newDownloadButton.classList.contains('hidden'), false);
  assert.equal(typeof nodes.newDownloadButton.handlers.click, 'function');
});
