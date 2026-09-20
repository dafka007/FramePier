const HOST_NAME = "com.viddock.helper";
const MAX_PENDING_MS = 120000;
const ACTIVE_VERSION = chrome.runtime.getManifest().version;
const UPDATE_ALARM = "viddock_update_check";
const MAX_UPDATE_RETRIES = 6;

let nativePort = null;
let connectionError = "";
const pending = new Map();
const jobs = new Map();
let versionCheck = Promise.resolve();

function connectNative() {
  if (nativePort) return nativePort;

  try {
    nativePort = chrome.runtime.connectNative(HOST_NAME);
    connectionError = "";
    nativePort.onMessage.addListener(handleNativeMessage);
    nativePort.onDisconnect.addListener(() => {
      const message = chrome.runtime.lastError?.message || "Native Messaging communication pipe closed.";
      logNativeError("disconnect", message);
      connectionError = friendlyNativeError(message, "disconnect");
      nativePort = null;
      for (const item of pending.values()) {
        clearTimeout(item.timer);
        item.reject(new Error(connectionError));
      }
      pending.clear();
      broadcast({ type: "connection", connected: false, error: connectionError });
      scheduleUpdateCheck();
    });
    return nativePort;
  } catch (error) {
    logNativeError("connect", error.message);
    connectionError = friendlyNativeError(error.message, "connect");
    nativePort = null;
    throw new Error(connectionError);
  }
}

function handleNativeMessage(message) {
  if (!message || typeof message !== "object" || (message.event !== "progress" && typeof message.id !== "string")) {
    logNativeError("response", "Native host returned an invalid response frame.");
    rejectPending("FramePier helper returned an invalid response.");
    return;
  }

  if (message.event === "progress" && message.job?.id) {
    jobs.set(message.job.id, message.job);
    const terminal = new Set(["finished", "failed", "cancelled"]);
    const completed = [...jobs].filter(([, job]) => terminal.has(job.state));
    for (const [id] of completed.slice(0, Math.max(0, completed.length - 100))) jobs.delete(id);
    broadcast({ type: "progress", job: message.job });
    return;
  }

  const item = pending.get(message.id);
  if (!item) return;
  clearTimeout(item.timer);
  pending.delete(message.id);
  if (message.ok) {
    const packageVersion = message.installedPackageVersion || message.installedExtensionVersion || message.versions?.extensionPackage?.version;
    if (packageVersion) {
      considerInstalledVersion(packageVersion).catch(() => {}).finally(() => item.resolve(message));
    } else {
      item.resolve(message);
    }
  }
  else item.reject(new Error(message.error || "The FramePier helper rejected the request."));
}

function nativeRequest(command, payload = {}, requestedId = null) {
  const id = requestedId || crypto.randomUUID().replaceAll("-", "_");
  if (pending.has(id)) return Promise.reject(new Error("A request for this job is already pending. Please retry."));
  const port = connectNative();

  return new Promise((resolve, reject) => {
    const timeout = command === "choose_download_folder" ? 10 * 60 * 1000 : MAX_PENDING_MS;
    const timer = setTimeout(() => {
      pending.delete(id);
      logNativeError("timeout", `No response for command ${command}.`);
      reject(new Error("FramePier helper started but did not return a valid response in time."));
    }, timeout);
    pending.set(id, { resolve, reject, timer });
    try {
      port.postMessage({ id, command, ...payload });
    } catch (error) {
      clearTimeout(timer);
      pending.delete(id);
      logNativeError("send", error.message);
      reject(new Error(friendlyNativeError(error.message, "send")));
    }
  });
}

function friendlyNativeError(message = "", stage = "connect") {
  if (/host not found|specified native messaging host/i.test(message)) {
    return "Native host not found. Run Repair Browser Integration, then restart this browser if it was open during an upgrade.";
  }
  if (/manifest.*(missing|invalid|read)|failed to read.*manifest/i.test(message)) {
    return "FramePier's Native Messaging manifest is missing or invalid. Run Repair Browser Integration.";
  }
  if (/access.*forbidden|not allowed/i.test(message)) {
    return "This FramePier extension origin is not allowed by the installed Native Messaging host.";
  }
  if (/failed to start|could not start|cannot start/i.test(message)) {
    return "The FramePier native host was found but its helper executable could not start.";
  }
  if (/host.*exited|pipe.*closed|native messaging.*closed/i.test(message) || stage === "disconnect") {
    return "The FramePier helper communication pipe closed. Retry once; if it repeats, run Repair Browser Integration.";
  }
  if (/invalid|communicat/i.test(message)) {
    return "The FramePier helper returned an invalid response.";
  }
  return "Could not connect to the FramePier helper.";
}

function rejectPending(message) {
  for (const item of pending.values()) {
    clearTimeout(item.timer);
    item.reject(new Error(message));
  }
  pending.clear();
}

function logNativeError(stage, rawMessage = "") {
  const entry = { timestamp: new Date().toISOString(), stage, message: String(rawMessage).slice(0, 1000) };
  console.error("FramePier Native Messaging error", entry);
  chrome.storage.local.get({ nativeDiagnostics: [] }).then(({ nativeDiagnostics }) => {
    const entries = Array.isArray(nativeDiagnostics) ? nativeDiagnostics.slice(-19) : [];
    entries.push(entry);
    return chrome.storage.local.set({ nativeDiagnostics: entries, lastNativeError: entry });
  }).catch(() => {});
}

function broadcast(message) {
  chrome.runtime.sendMessage(message).catch(() => {});
}

function validVersion(value) {
  return typeof value === "string" && /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/.test(value);
}

function compareVersions(left, right) {
  if (!validVersion(left) || !validVersion(right)) return null;
  const a = left.split(".").map(BigInt);
  const b = right.split(".").map(BigInt);
  for (let index = 0; index < 3; index++) {
    if (a[index] !== b[index]) return a[index] > b[index] ? 1 : -1;
  }
  return 0;
}

function considerInstalledVersion(packageVersion) {
  // Serialize storage read/check/write so simultaneous responses cannot both
  // schedule a reload before either has saved the target-version guard.
  const check = versionCheck.then(() => checkInstalledVersion(packageVersion));
  versionCheck = check.catch(() => {});
  return check;
}

async function checkInstalledVersion(packageVersion) {
  const comparison = compareVersions(packageVersion, ACTIVE_VERSION);
  if (comparison === null || comparison <= 0) {
    if (comparison === 0) await chrome.storage.local.remove(["extensionReloadGuard", "extensionUpdateMessage", "updateRetryCount"]);
    return false;
  }
  const now = Date.now();
  const { extensionReloadGuard } = await chrome.storage.local.get({ extensionReloadGuard: null });
  if (extensionReloadGuard?.target === packageVersion && extensionReloadGuard?.active === ACTIVE_VERSION) {
    void recordUpdateEvent("reload_guarded", ACTIVE_VERSION, packageVersion);
    await setUpdateStatus(`FramePier ${packageVersion} is installed. Completely exit and reopen ${browserLabel()} to activate it.`);
    return false;
  }
  await recordUpdateEvent("mismatch_detected", ACTIVE_VERSION, packageVersion);
  await chrome.storage.local.set({ extensionReloadGuard: { active: ACTIVE_VERSION, target: packageVersion, at: now } });
  await setUpdateStatus("FramePier was updated. Reloading…");
  await recordUpdateEvent("reload_requested", ACTIVE_VERSION, packageVersion);
  setTimeout(() => chrome.runtime.reload(), 250);
  return true;
}

async function setUpdateStatus(message) {
  await chrome.storage.local.set({ extensionUpdateMessage: message });
  broadcast({ type: "update_status", message });
}

function browserLabel() {
  if (/Edg\//.test(navigator.userAgent)) return "Edge";
  if (navigator.brave) return "Brave";
  if (/Chrome\//.test(navigator.userAgent)) return "Chrome";
  return "Chromium";
}

async function recordUpdateEvent(updateEvent, runningVersion, targetVersion) {
  try {
    await nativeRequest("record_update_event", { updateEvent, runningVersion, targetVersion, browser: browserLabel() });
  } catch {}
}

function scheduleUpdateCheck(delayInMinutes = 0.5) {
  chrome.alarms.create(UPDATE_ALARM, { delayInMinutes });
}

chrome.alarms.onAlarm.addListener(async (alarm) => {
  if (alarm.name !== UPDATE_ALARM) return;
  try {
    const ping = await nativeRequest("ping");
    await considerInstalledVersion(ping.installedPackageVersion || ping.installedExtensionVersion || "");
    await chrome.storage.local.remove("updateRetryCount");
  } catch {
    const { updateRetryCount = 0 } = await chrome.storage.local.get({ updateRetryCount: 0 });
    if (updateRetryCount < MAX_UPDATE_RETRIES) {
      await chrome.storage.local.set({ updateRetryCount: updateRetryCount + 1 });
      scheduleUpdateCheck();
    }
  }
});

chrome.runtime.onInstalled.addListener((details) => {
  if (details.reason !== "update") return;
  chrome.storage.local.remove(["extensionReloadGuard", "extensionUpdateMessage", "updateRetryCount"]).catch(() => {});
  void recordUpdateEvent("activated", ACTIVE_VERSION, ACTIVE_VERSION);
});

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (!message || typeof message.type !== "string") return false;

  if (message.type === "native") {
    const { command, payload, id } = message;
    nativeRequest(command, payload || {}, id || null)
      .then((result) => sendResponse({ ok: true, result }))
      .catch((error) => sendResponse({ ok: false, error: error.message }));
    return true;
  }

  if (message.type === "connection_status") {
    sendResponse({ ok: true, connected: Boolean(nativePort), error: connectionError });
    return false;
  }

  if (message.type === "job_status") {
    sendResponse({ ok: true, job: jobs.get(message.id) || null });
    return false;
  }

  if (message.type === "open_settings") {
    chrome.runtime.openOptionsPage();
    sendResponse({ ok: true });
    return false;
  }

  return false;
});
