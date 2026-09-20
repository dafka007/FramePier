import { spawn } from "node:child_process";
import { once } from "node:events";
import path from "node:path";
import { readFileSync } from "node:fs";

const expectedVersion = JSON.parse(readFileSync(new URL("../extension/manifest.json", import.meta.url), "utf8")).version;

const helper = process.argv[2];
const action = process.argv[3] || "info";
const requestedMedia = process.argv[4] || "aqz-KE-bpKQ";
if (!helper) {
  console.error("Usage: node native-smoke.mjs <FramePierHelper.exe> [info|download|download_mkv|m4a|mp3|cancel] [videoId-or-supported-URL] [quality]");
  process.exit(2);
}

const child = spawn(path.resolve(helper), ["chrome-extension://kclnooibijmfenaldmpkffdbednfipkk/"], {
  stdio: ["pipe", "pipe", "inherit"],
  windowsHide: true
});
const messages = [];
let buffer = Buffer.alloc(0);
child.stdout.on("data", (chunk) => {
  buffer = Buffer.concat([buffer, chunk]);
  while (buffer.length >= 4) {
    const size = buffer.readUInt32LE(0);
    if (buffer.length < size + 4) break;
    const message = JSON.parse(buffer.subarray(4, size + 4).toString("utf8"));
    buffer = buffer.subarray(size + 4);
    messages.push(message);
    process.stdout.write(`${JSON.stringify(message)}\n`);
  }
});

function send(message) {
  const data = Buffer.from(JSON.stringify(message), "utf8");
  const prefix = Buffer.alloc(4);
  prefix.writeUInt32LE(data.length);
  child.stdin.write(Buffer.concat([prefix, data]));
}

function waitFor(predicate, timeout = 120000) {
  const existing = messages.find(predicate);
  if (existing) return Promise.resolve(existing);
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      clearInterval(interval);
      reject(new Error("Native smoke test timed out"));
    }, timeout);
    const interval = setInterval(() => {
      const match = messages.find(predicate);
      if (!match && child.exitCode === null && child.signalCode === null) return;
      clearTimeout(timer);
      clearInterval(interval);
      if (!match) {
        reject(new Error("Native helper exited before the expected response"));
        return;
      }
      resolve(match);
    }, 25);
  });
}

try {
  send({ id: "ping", command: "ping" });
  const ping = await waitFor((item) => item.id === "ping");
  if (!ping.ok || ping.version !== expectedVersion) throw new Error("Helper ping failed");

  send({ id: "versions", command: "get_versions" });
  const versions = await waitFor((item) => item.id === "versions");
  if (!versions.ok || !versions.versions.ytDlp.installed || !versions.versions.ffmpeg.installed) throw new Error("Dependencies unavailable");

  const { url, videoId } = parseRequestedMedia(requestedMedia);
  send({ id: "info", command: "get_video_info", url, videoId });
  const info = await waitFor((item) => item.id === "info", 180000);
  if (!info.ok || info.video.id !== videoId || !Array.isArray(info.video.qualities)) throw new Error(info.error || "Metadata test failed");

  if (["download", "download_mkv", "m4a", "mp3", "cancel"].includes(action)) {
    const jobId = `job_${Date.now()}`;
    const audioOnly = action === "mp3" || action === "m4a";
    const requestedQuality = process.argv[5] || (["download", "download_mkv"].includes(action) ? "144" : "best");
    send({ id: jobId, command: "start_download", url, videoId, mode: audioOnly ? "audio" : "video", quality: requestedQuality, container: ["cancel", "download_mkv"].includes(action) ? "mkv" : "mp4", audioFormat: audioOnly ? action : "best" });
    const started = await waitFor((item) => item.id === jobId && item.command === "start_download");
    if (!started.ok) throw new Error(started.error || "Download did not start");
    if (action === "cancel") {
      await waitFor((item) => item.event === "progress" && item.job?.id === jobId && item.job.state === "downloading", 180000);
      await new Promise((resolve) => setTimeout(resolve, 3000));
      send({ id: jobId, command: "cancel_download" });
      const cancelled = await waitFor((item) => item.event === "progress" && item.job?.id === jobId && item.job.state === "cancelled", 60000);
      if (!cancelled) throw new Error("Cancellation was not confirmed");
    } else {
      const finished = await waitFor((item) => item.event === "progress" && item.job?.id === jobId && ["finished", "failed"].includes(item.job.state), 600000);
      if (finished.job.state !== "finished") throw new Error(finished.job.error || "Download failed");
      if (process.argv.includes("--open-folder")) {
        send({ id: "open_folder", command: "open_download_folder" });
        const opened = await waitFor(item => item.id === "open_folder");
        if (!opened.ok) throw new Error(opened.error || "Folder opening failed");
      }
    }
  }
} finally {
  child.stdin.end();
  const exited = await Promise.race([
    once(child, "exit").then(() => true),
    new Promise((resolve) => setTimeout(() => resolve(false), 2000))
  ]);
  if (!exited) child.kill();
}

function parseRequestedMedia(value) {
  if (!value.startsWith("https://")) return { url: `https://www.youtube.com/watch?v=${value}`, videoId: value };
  const parsed = new URL(value);
  const host = parsed.hostname.toLowerCase();
  const parts = parsed.pathname.split("/").filter(Boolean);
  if (host === "clips.twitch.tv" && parts.length === 1) return { url: value, videoId: parts[0] };
  if (["twitch.tv", "www.twitch.tv", "m.twitch.tv"].includes(host) && parts.length === 2 && parts[0].toLowerCase() === "videos") return { url: value, videoId: parts[1] };
  if (["twitch.tv", "www.twitch.tv", "m.twitch.tv"].includes(host) && parts.length === 3 && parts[1].toLowerCase() === "clip") return { url: value, videoId: parts[2] };
  const youtubeId = host === "youtu.be" ? parts[0] : parsed.searchParams.get("v");
  return { url: value, videoId: youtubeId };
}
