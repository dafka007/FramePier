import { spawn } from "node:child_process";
import { once } from "node:events";
import { resolve } from "node:path";

const [helperArg, installerArg] = process.argv.slice(2);
if (!helperArg || !installerArg) throw new Error("Usage: upgrade-idle-smoke.mjs <installed-helper> <installer>");
const helperPath = resolve(helperArg);
const helper = spawn(helperPath, ["chrome-extension://kclnooibijmfenaldmpkffdbednfipkk/"], { stdio: ["pipe", "pipe", "inherit"], windowsHide: true });
const helperExit = once(helper, "exit");
let buffer = Buffer.alloc(0);
let timer;
try {
  const ping = new Promise((res, rej) => {
    timer = setTimeout(() => rej(new Error("Helper ping timed out")), 10000);
    helper.stdout.on("data", chunk => {
      buffer = Buffer.concat([buffer, chunk]);
      if (buffer.length < 4 || buffer.length < 4 + buffer.readUInt32LE(0)) return;
      clearTimeout(timer);
      res(JSON.parse(buffer.subarray(4, 4 + buffer.readUInt32LE(0)).toString()));
    });
  });
  const message = Buffer.from(JSON.stringify({ id: "upgrade_ping", command: "ping" }));
  const frame = Buffer.alloc(4 + message.length);
  frame.writeUInt32LE(message.length); message.copy(frame, 4); helper.stdin.write(frame);
  const before = await ping;
  if (!before.ok) throw new Error("Old helper ping failed");
  const status = spawn(helperPath, ["--lifecycle-status", helperPath], { stdio: "ignore", windowsHide: true });
  const [statusCode] = await once(status, "exit");
  if (statusCode !== 10) throw new Error(`Upgrade test requires idle helpers; status ${statusCode}`);
  const installer = spawn(resolve(installerArg), ["/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART"], { stdio: "ignore", windowsHide: true });
  const [installerCode] = await once(installer, "exit");
  if (installerCode !== 0) throw new Error(`Installer exited ${installerCode}`);
  const [helperCode] = await helperExit;
  console.log(JSON.stringify({ oldVersion: before.version, statusCode, installerCode, helperCode }));
} finally {
  clearTimeout(timer);
  helper.stdin.end();
}
