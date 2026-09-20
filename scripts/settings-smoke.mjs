import { spawn } from "node:child_process";
import { once } from "node:events";
import path from "node:path";

const helper = process.argv[2];
const replacement = process.argv[3] || "";
if (!helper) {
  console.error("Usage: node settings-smoke.mjs <FramePierHelper.exe> [replacement-folder]");
  process.exit(2);
}

const child = spawn(path.resolve(helper), ["chrome-extension://kclnooibijmfenaldmpkffdbednfipkk/"], {
  stdio: ["pipe", "pipe", "inherit"],
  windowsHide: true
});
let buffer = Buffer.alloc(0);
const pending = new Map();
child.stdout.on("data", (chunk) => {
  buffer = Buffer.concat([buffer, chunk]);
  while (buffer.length >= 4) {
    const size = buffer.readUInt32LE(0);
    if (buffer.length < size + 4) break;
    const message = JSON.parse(buffer.subarray(4, size + 4).toString("utf8"));
    buffer = buffer.subarray(size + 4);
    pending.get(message.id)?.(message);
    pending.delete(message.id);
  }
});

function request(command, payload = {}) {
  const id = `settings_${Date.now()}_${Math.random().toString(16).slice(2)}`;
  const data = Buffer.from(JSON.stringify({ id, command, ...payload }), "utf8");
  const frame = Buffer.alloc(data.length + 4);
  frame.writeUInt32LE(data.length, 0);
  data.copy(frame, 4);
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error(`${command} timed out`)), 30000);
    pending.set(id, (message) => {
      clearTimeout(timer);
      if (!message.ok) reject(new Error(message.error || `${command} failed`));
      else resolve(message);
    });
    child.stdin.write(frame);
  });
}

try {
  const ping = await request("ping");
  const before = (await request("get_settings")).settings;
  let after = before;
  if (replacement) {
    after = (await request("set_settings", { downloadPath: replacement, autoOpen: before.autoOpen })).settings;
    const restartedRead = (await request("get_settings")).settings;
    if (restartedRead.downloadPath !== after.downloadPath) throw new Error("Settings did not persist in the helper");
  }
  console.log(JSON.stringify({ ping, before, after }));
} finally {
  child.stdin.end();
  await Promise.race([once(child, "exit"), new Promise((resolve) => setTimeout(resolve, 2000))]);
  if (child.exitCode === null) child.kill();
}
