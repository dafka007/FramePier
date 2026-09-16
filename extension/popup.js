const elements = Object.fromEntries([...document.querySelectorAll("[id]")].map((element) => [element.id, element]));
let currentVideo = null;
let mode = "video";
let jobId = null;

document.addEventListener("DOMContentLoaded", initialize);
elements.retryButton.addEventListener("click", initialize);
elements.videoMode.addEventListener("click", () => setMode("video"));
elements.audioMode.addEventListener("click", () => setMode("audio"));
elements.downloadButton.addEventListener("click", startDownload);
elements.cancelButton.addEventListener("click", cancelDownload);
elements.quality.addEventListener("change", renderEstimate);
elements.openFolderButton.addEventListener("click", openDownloadFolder);
elements.newDownloadButton.addEventListener("click", async () => {
  jobId = null;
  await chrome.storage.session.remove("activeJobId");
  initialize();
});
elements.settingsButton.addEventListener("click", () => chrome.runtime.sendMessage({ type: "open_settings" }));
chrome.runtime.onMessage.addListener((message) => {
  if (message.type === "progress" && message.job?.id === jobId) renderProgress(message.job);
  if (message.type === "update_status") {
    elements.loadingText.textContent = message.message || "VidDock was updated. Reloading…";
    show("loadingPanel");
  }
});

async function initialize() {
  show("loadingPanel");
  elements.loadingText.textContent = "Connecting to helper…";
  try {
    await native("ping");
    const { extensionUpdateMessage = "" } = await chrome.storage.local.get({ extensionUpdateMessage: "" });
    if (extensionUpdateMessage) {
      elements.loadingText.textContent = extensionUpdateMessage;
      return;
    }
    const savedJob = await chrome.storage.session.get({ activeJobId: "" });
    if (savedJob.activeJobId) {
      const cached = await chrome.runtime.sendMessage({ type: "job_status", id: savedJob.activeJobId });
      if (cached?.job) {
        jobId = savedJob.activeJobId;
        renderProgress(cached.job);
        show("progressPanel");
        return;
      }
    }
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    const parsed = parseMediaUrl(tab?.url || "");
    elements.loadingText.textContent = "Fetching video information…";
    const info = await native("get_video_info", { url: parsed.url, videoId: parsed.videoId });
    currentVideo = info.result.video;
    await loadDefaults();
    renderVideo(currentVideo);
    show("videoPanel");
  } catch (error) {
    showError(error.message);
  }
}

async function loadDefaults() {
  const defaults = await chrome.storage.sync.get({ defaultQuality: "best", defaultContainer: "mp4", defaultAudioFormat: "best" });
  elements.container.value = defaults.defaultContainer;
  elements.audioFormat.value = defaults.defaultAudioFormat;
  elements.quality.dataset.preferred = defaults.defaultQuality;
}

function renderVideo(video) {
	currentVideo = video;
  elements.sourceLabel.textContent = video.sourceLabel || "Supported video";
  elements.videoTitle.textContent = video.title || "Untitled video";
  elements.channel.textContent = video.channel || "Unknown channel";
  elements.duration.textContent = formatDuration(video.duration);
  elements.currentUrl.textContent = video.url;
  if (isSafeThumbnail(video.thumbnail)) {
    elements.thumbnail.src = video.thumbnail;
  } else {
    elements.thumbnail.removeAttribute("src");
  }
  elements.quality.replaceChildren(option("best", "Best available"));
  for (const resolution of video.qualities || []) {
    if (typeof resolution === "number") {
      const label = resolution === 2160 ? "2160p (4K)" : `${resolution}p`;
      elements.quality.append(option(String(resolution), label));
      continue;
    }
    if (!Number.isInteger(resolution?.width) || !Number.isInteger(resolution?.height)) continue;
    const fps = Number.isInteger(resolution.fps) && resolution.fps > 0 ? `@${resolution.fps}` : "";
    const item = option(`${resolution.width}x${resolution.height}${fps}`, resolution.label || `${resolution.width}×${resolution.height}`);
    item.dataset.approxBytes = String(resolution.approxBytes || 0);
    elements.quality.append(item);
  }
  const preferred = elements.quality.dataset.preferred;
  if ([...elements.quality.options].some((item) => item.value === preferred)) elements.quality.value = preferred;
  renderEstimate();
}

function renderEstimate() {
  if (!currentVideo || mode !== "video") {
    elements.estimateText.textContent = "";
    return;
  }
  const selected = elements.quality.selectedOptions[0];
  const bytes = selected?.value === "best" ? Number(currentVideo.approxBytes) : Number(selected?.dataset.approxBytes);
  elements.estimateText.textContent = Number.isFinite(bytes) && bytes > 0 ? `Estimated size: approximately ${formatBytes(bytes)}` : "";
}

function setMode(nextMode) {
  mode = nextMode;
  elements.videoMode.classList.toggle("active", mode === "video");
  elements.audioMode.classList.toggle("active", mode === "audio");
  elements.videoOptions.classList.toggle("hidden", mode !== "video");
  elements.audioOptions.classList.toggle("hidden", mode !== "audio");
  renderEstimate();
}

async function startDownload() {
  if (!currentVideo) return;
  jobId = crypto.randomUUID().replaceAll("-", "_");
  await chrome.storage.session.set({ activeJobId: jobId });
  show("progressPanel");
  renderProgress({ id: jobId, state: "preparing", percent: 0 });
  try {
    await native("start_download", {
      url: currentVideo.url,
      videoId: currentVideo.id,
      mode,
      quality: elements.quality.value || "best",
      container: elements.container.value,
      audioFormat: elements.audioFormat.value
    }, jobId);
  } catch (error) {
    renderProgress({ id: jobId, state: "failed", percent: 0, error: error.message });
  }
}

async function cancelDownload() {
  if (!jobId) return;
  elements.cancelButton.disabled = true;
  elements.statusText.textContent = "Cancelling…";
  try {
    await native("cancel_download", {}, jobId);
  } catch (error) {
    elements.outputText.textContent = error.message;
    elements.cancelButton.disabled = false;
  }
}

async function openDownloadFolder() {
  elements.openFolderButton.disabled = true;
  elements.folderStatus.textContent = "Opening download folder…";
  try {
    await native("open_download_folder");
    elements.folderStatus.textContent = "";
  } catch (error) {
    console.error("VidDock open_download_folder failed", error.message);
    elements.folderStatus.textContent = error.message || "VidDock couldn't open the download folder.";
  } finally {
    elements.openFolderButton.disabled = false;
  }
}

function renderProgress(job) {
  elements.folderStatus.textContent = "";
  const labels = {
    preparing: "Preparing…", fetching_formats: "Fetching formats…", downloading: "Downloading…", merging: "Merging…",
    converting_audio: "Converting audio…", finalizing: "Finalizing…", finished: "Finished", cancelled: "Cancelled", failed: "Failed"
  };
  const percent = Number.isFinite(job.percent) ? Math.max(0, Math.min(100, job.percent)) : 0;
  elements.statusText.textContent = labels[job.state] || "Working…";
  elements.percentText.textContent = `${percent.toFixed(percent % 1 ? 1 : 0)}%`;
  elements.progressBar.style.width = `${percent}%`;
  elements.speedText.textContent = job.speed || "";
  const byteProgress = job.downloadedBytes > 0 && job.totalBytes > 0 ? `${formatBytes(job.downloadedBytes)} / ${formatBytes(job.totalBytes)}` : "";
  elements.etaText.textContent = [byteProgress, job.eta ? `ETA ${job.eta}` : ""].filter(Boolean).join(" · ");
  elements.outputText.textContent = job.error || (job.outputPath ? `Saved as ${fileName(job.outputPath)}` : "");
  const done = ["finished", "cancelled", "failed"].includes(job.state);
  elements.cancelButton.classList.toggle("hidden", done);
  elements.cancelButton.disabled = false;
  elements.openFolderButton.classList.toggle("hidden", job.state !== "finished");
  elements.newDownloadButton.classList.toggle("hidden", !done);
}

function parseMediaUrl(value) {
  try {
    const url = new URL(value);
    if (url.protocol !== "https:") throw new Error("Open a supported YouTube video or public Twitch clip/VOD, then click VidDock again.");
    const host = url.hostname.toLowerCase();
    let videoId = "";
    if (host === "youtu.be") videoId = url.pathname.split("/").filter(Boolean)[0] || "";
    if (["youtube.com", "www.youtube.com", "m.youtube.com", "music.youtube.com"].includes(host)) {
      if (url.pathname === "/watch") videoId = url.searchParams.get("v") || "";
      else if (/^\/(shorts|live|embed)\//.test(url.pathname)) videoId = url.pathname.split("/")[2] || "";
    }
    if (/^[A-Za-z0-9_-]{6,20}$/.test(videoId)) return { videoId, url: `https://www.youtube.com/watch?v=${videoId}`, source: "youtube" };

    if (["twitch.tv", "www.twitch.tv", "m.twitch.tv", "clips.twitch.tv"].includes(host)) {
      const parts = url.pathname.split("/").filter(Boolean).map(decodeURIComponent);
      const clipPattern = /^[A-Za-z0-9_-]{6,100}$/;
      if (host === "clips.twitch.tv" && parts.length === 1 && clipPattern.test(parts[0])) {
        return { videoId: parts[0], url: `https://clips.twitch.tv/${parts[0]}`, source: "twitch_clip" };
      }
      if (parts.length === 2 && parts[0].toLowerCase() === "videos" && /^[0-9]{1,20}$/.test(parts[1])) {
        return { videoId: parts[1], url: `https://www.twitch.tv/videos/${parts[1]}`, source: "twitch_vod" };
      }
      if (parts.length === 3 && /^[A-Za-z0-9_]{1,25}$/.test(parts[0]) && parts[1].toLowerCase() === "clip" && clipPattern.test(parts[2])) {
        return { videoId: parts[2], url: `https://clips.twitch.tv/${parts[2]}`, source: "twitch_clip" };
      }
      if (parts.length === 1 && /^[A-Za-z0-9_]{1,25}$/.test(parts[0])) {
		const reserved = new Set(["directory", "settings", "subscriptions", "search", "downloads", "inventory", "wallet", "jobs", "p", "turbo", "videos"]);
		if (reserved.has(parts[0].toLowerCase())) throw new Error("This Twitch page is not a supported clip or VOD.");
        throw new Error("Live Twitch streams are not supported yet. Public Twitch clips and VODs are supported.");
      }
      throw new Error("This Twitch page is not a supported clip or VOD.");
    }
    throw new Error("Open a supported YouTube video or public Twitch clip/VOD, then click VidDock again.");
  } catch (error) {
    if (error instanceof Error && /Twitch|supported YouTube/.test(error.message)) throw error;
    throw new Error("Open a supported YouTube video or public Twitch clip/VOD, then click VidDock again.");
  }
}

function isSafeThumbnail(value) {
  try {
    const url = new URL(value);
    return url.protocol === "https:" && (url.hostname === "i.ytimg.com" || url.hostname.endsWith(".ytimg.com") || url.hostname === "static-cdn.jtvnw.net");
  } catch {
    return false;
  }
}

function option(value, label) {
  const item = document.createElement("option");
  item.value = value;
  item.textContent = label;
  return item;
}

function show(id) {
  for (const panel of [elements.loadingPanel, elements.errorPanel, elements.videoPanel, elements.progressPanel]) panel.classList.add("hidden");
  elements[id].classList.remove("hidden");
}

function showError(message) {
  elements.errorText.textContent = message;
  show("errorPanel");
}

function formatDuration(seconds) {
  if (!Number.isFinite(seconds) || seconds <= 0) return "";
  const total = Math.floor(seconds);
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const rest = total % 60;
  return hours ? `${hours}:${String(minutes).padStart(2, "0")}:${String(rest).padStart(2, "0")}` : `${minutes}:${String(rest).padStart(2, "0")}`;
}

function formatBytes(bytes) {
  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value.toFixed(unit < 2 ? 0 : 1)} ${units[unit]}`;
}

function fileName(path) { return path.split(/[\\/]/).pop() || path; }

async function native(command, payload = {}, id = null) {
  const response = await chrome.runtime.sendMessage({ type: "native", command, payload, id });
  if (!response?.ok) throw new Error(response?.error || "VidDock request failed.");
  return response;
}
