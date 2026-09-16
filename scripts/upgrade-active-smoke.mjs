import { spawn } from "node:child_process";
import { once } from "node:events";
import path from "node:path";

const [helperArg, installerArg, videoId = "Ttl8Gg-P-Ao"] = process.argv.slice(2);
const mode = process.argv[5] || "upgrade";
if (!helperArg || !installerArg) {
  console.error("Usage: node upgrade-active-smoke.mjs <installed-helper> <installer> [videoId]");
  process.exit(2);
}
const helper = spawn(path.resolve(helperArg), ["chrome-extension://kclnooibijmfenaldmpkffdbednfipkk/"], { stdio: ["pipe", "pipe", "inherit"], windowsHide: true });
let buffer = Buffer.alloc(0);
const messages = [];
helper.stdout.on("data", chunk => {
  buffer = Buffer.concat([buffer, chunk]);
  while (buffer.length >= 4) {
    const size = buffer.readUInt32LE(0);
    if (buffer.length < size + 4) break;
    messages.push(JSON.parse(buffer.subarray(4, size + 4).toString("utf8")));
    buffer = buffer.subarray(size + 4);
  }
});
const send = message => {
  const data = Buffer.from(JSON.stringify(message));
  const prefix = Buffer.alloc(4); prefix.writeUInt32LE(data.length);
  helper.stdin.write(Buffer.concat([prefix, data]));
};
const waitFor = (predicate, timeout = 180000) => new Promise((resolve, reject) => {
  const timer = setTimeout(() => { clearInterval(poll); reject(new Error("timed out")); }, timeout);
  const poll = setInterval(() => {
    const value = messages.find(predicate);
    if (value) { clearTimeout(timer); clearInterval(poll); resolve(value); }
  }, 25);
});

send({ id: "active_upgrade", command: "start_download", url: `https://www.youtube.com/watch?v=${videoId}`, videoId, mode: "video", quality: "best", container: "mkv", audioFormat: "best" });
const accepted = await waitFor(item => item.id === "active_upgrade");
if (!accepted.ok) throw new Error(accepted.error);
await waitFor(item => item.event === "progress" && item.job?.id === "active_upgrade" && item.job.state === "downloading");
const status = spawn(path.resolve(helperArg), ["--lifecycle-status", path.resolve(helperArg)], { stdio: "ignore", windowsHide: true });
const [statusCode] = await once(status, "exit");
if (statusCode !== 11) throw new Error(`expected active lifecycle status 11, got ${statusCode}`);
const helperExitPromise = once(helper, "exit").then(([code]) => code);
if (mode === "crash") {
  helper.kill();
  const helperExit = await helperExitPromise;
  console.log(JSON.stringify({ statusCode, forcedCrash: true, helperExit }));
  process.exit(0);
}
const installer = spawn(path.resolve(installerArg), ["/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART"], { stdio: "ignore", windowsHide: true });
const [installerCode] = await once(installer, "exit");
const helperExit = await Promise.race([helperExitPromise, new Promise(resolve => setTimeout(() => resolve("timeout"), 15000))]);
if (helperExit === "timeout") helper.kill();
console.log(JSON.stringify({ statusCode, installerCode, helperExit }));
if (installerCode !== 0 || helperExit === "timeout") process.exit(1);
