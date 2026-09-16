import test from "node:test";
import assert from "node:assert/strict";
import vm from "node:vm";
import { readFileSync } from "node:fs";
import { webcrypto } from "node:crypto";

function worker() {
  const storage = {};
  let receive;
  let reloads = 0;
  const context = vm.createContext({
    console, crypto: webcrypto, setTimeout, clearTimeout,
    navigator: { userAgent: "Chrome/test", brave: {} },
    chrome: {
      runtime: {
        getManifest: () => ({ version: "0.2.1" }),
        connectNative: () => ({
          onMessage: { addListener: fn => { receive = fn; } },
          onDisconnect: { addListener() {} },
          postMessage: message => queueMicrotask(() => receive({ id: message.id, ok: true }))
        }),
        sendMessage: async () => {}, reload: () => { reloads++; },
        onInstalled: { addListener() {} }, onMessage: { addListener() {} }
      },
      storage: { local: {
        get: async defaults => ({ ...defaults, ...storage }),
        set: async values => { Object.assign(storage, values); },
        remove: async keys => { for (const key of [keys].flat()) delete storage[key]; }
      } },
      alarms: { create() {}, onAlarm: { addListener() {} } }
    }
  });
  vm.runInContext(readFileSync(new URL("../extension/background.js", import.meta.url), "utf8"), context);
  return { run: code => vm.runInContext(code, context), storage, reloads: () => reloads };
}

test("simultaneous package checks request exactly one reload", async () => {
  const w = worker();
  await w.run('Promise.all([considerInstalledVersion("0.2.2"), considerInstalledVersion("0.2.2")])');
  await new Promise(resolve => setTimeout(resolve, 300));
  assert.equal(w.reloads(), 1);
  await w.run('considerInstalledVersion("0.2.2")');
  assert.equal(w.storage.extensionReloadGuard.target, "0.2.2");
  assert.equal(w.reloads(), 1);
});

test("invalid and older versions do not reload", async () => {
  const w = worker();
  for (const version of ["0.2.0", "0.2.1", "x", "01.2.3", "1.2.3.4"]) {
    await w.run(`considerInstalledVersion(${JSON.stringify(version)})`);
  }
  assert.equal(w.reloads(), 0);
});

test("job history retains active jobs and bounds terminal entries", () => {
  const w = worker();
  w.run(`
    handleNativeMessage({event: "progress", job: {id: "active", state: "downloading"}});
    for (let n = 0; n < 150; n++) handleNativeMessage({event: "progress", job: {id: String(n), state: "finished"}});
  `);
  assert.equal(w.run('jobs.size'), 101);
  assert.equal(w.run('jobs.has("active")'), true);
  assert.equal(w.run('jobs.has("0")'), false);
});
