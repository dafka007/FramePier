const byId = (id) => document.getElementById(id);
const ids = ["defaultQuality", "defaultContainer", "defaultAudioFormat", "downloadPath", "browseButton", "autoOpen", "saveButton", "saveStatus", "nativeStatus", "helperVersion", "ytdlpVersion", "ffmpegVersion", "extensionVersion", "browserName", "refreshButton", "updateButton", "openFolderButton", "openLogsButton"];
const el = Object.fromEntries(ids.map((id) => [id, byId(id)]));

document.addEventListener("DOMContentLoaded", initialize);
el.saveButton.addEventListener("click", save);
el.refreshButton.addEventListener("click", refresh);
el.updateButton.addEventListener("click", updateYTDLP);
el.browseButton.addEventListener("click", chooseDownloadFolder);
el.openFolderButton.addEventListener("click", () => openFolder("open_download_folder", el.openFolderButton));
el.openLogsButton.addEventListener("click", () => openFolder("open_logs_folder", el.openLogsButton));
chrome.runtime.onMessage.addListener((message) => {
  if (message?.type === "update_status") setStatus(message.message || "VidDock was updated. Reloading…");
});

async function initialize() {
  el.extensionVersion.textContent = chrome.runtime.getManifest().version;
  el.browserName.textContent = detectBrowser();
  const defaults = await chrome.storage.sync.get({ defaultQuality: "best", defaultContainer: "mp4", defaultAudioFormat: "best" });
  el.defaultQuality.value = defaults.defaultQuality;
  el.defaultContainer.value = defaults.defaultContainer;
  el.defaultAudioFormat.value = defaults.defaultAudioFormat;
  await refresh();
  const { extensionUpdateMessage = "" } = await chrome.storage.local.get({ extensionUpdateMessage: "" });
  if (extensionUpdateMessage) setStatus(extensionUpdateMessage);
}

async function refresh() {
  setStatus("Checking…");
  try {
    await native("ping");
    const [versions, settings] = await Promise.all([native("get_versions"), native("get_settings")]);
    const value = versions.result.versions;
    el.nativeStatus.textContent = "Connected";
    el.helperVersion.textContent = `Installed · ${value.helper.version}`;
    el.ytdlpVersion.textContent = value.ytDlp.installed ? `Installed · ${value.ytDlp.version || "version unknown"}` : "Missing";
    el.ffmpegVersion.textContent = value.ffmpeg.installed ? `Installed · ${shortFFmpegVersion(value.ffmpeg.version)}` : "Missing";
    el.downloadPath.value = settings.result.settings.downloadPath;
    el.autoOpen.checked = Boolean(settings.result.settings.autoOpen);
    setStatus("");
  } catch (error) {
    el.nativeStatus.textContent = "Disconnected";
    setStatus(error.message, true);
  }
}

async function chooseDownloadFolder() {
  const previousPath = el.downloadPath.value;
  el.browseButton.disabled = true;
  setStatus("Choose a folder in the Windows dialog…");
  try {
    const result = await native("choose_download_folder");
    if (result.result.cancelled) {
      el.downloadPath.value = previousPath;
      setStatus("");
      return;
    }
    el.downloadPath.value = result.result.path;
    setStatus("Folder selected. Save settings to apply it.");
  } catch (error) {
    el.downloadPath.value = previousPath;
    setStatus(error.message, true);
  } finally {
    el.browseButton.disabled = false;
  }
}

async function openFolder(command, button) {
  button.disabled = true;
  setStatus("Opening folder…");
  try {
    await native(command);
    setStatus("");
  } catch (error) {
    console.error(`VidDock ${command} failed`, error.message);
    setStatus(error.message || "VidDock couldn't open the folder.", true);
  } finally {
    button.disabled = false;
  }
}

async function save() {
  el.saveButton.disabled = true;
  setStatus("Saving…");
  try {
    await chrome.storage.sync.set({ defaultQuality: el.defaultQuality.value, defaultContainer: el.defaultContainer.value, defaultAudioFormat: el.defaultAudioFormat.value });
    await native("set_settings", { downloadPath: el.downloadPath.value.trim(), autoOpen: el.autoOpen.checked });
    setStatus("Settings saved.");
  } catch (error) {
    setStatus(error.message, true);
  } finally {
    el.saveButton.disabled = false;
  }
}

async function updateYTDLP() {
  el.updateButton.disabled = true;
  setStatus("Checking for a yt-dlp update…");
  try {
    await native("update_ytdlp");
    setStatus("yt-dlp update check completed.");
    await refresh();
  } catch (error) {
    setStatus(error.message, true);
  } finally {
    el.updateButton.disabled = false;
  }
}

function detectBrowser() {
  const ua = navigator.userAgent;
  if (/Edg\//.test(ua)) return "Microsoft Edge";
  if (navigator.brave) return "Brave";
  if (/Chrome\//.test(ua)) return "Google Chrome";
  return "Chromium";
}

function setStatus(message, error = false) {
  el.saveStatus.textContent = message;
  el.saveStatus.style.color = error ? "#ff929e" : "#65d8b0";
}

function shortFFmpegVersion(value = "") {
  return value.match(/ffmpeg version\s+([0-9]+(?:\.[0-9]+){1,3})/i)?.[1] || "version unknown";
}

async function native(command, payload = {}) {
  const response = await chrome.runtime.sendMessage({ type: "native", command, payload });
  if (!response?.ok) throw new Error(response?.error || "VidDock request failed.");
  return response;
}
