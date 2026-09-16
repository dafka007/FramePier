const port = process.argv[2] || "9333";
const mode = process.argv[3] || "ping";
const requestedVideoId = process.argv[4] || "2PuFyjAs7JA";
const requestedUrl = process.argv[5] || "https://clips.twitch.tv/AmusedMildTitanOSkomodo-hl2PV3WlBurF_8uo";
const extensionId = "kclnooibijmfenaldmpkffdbednfipkk";
const deadline = Date.now() + 15000;
let targets;
while (Date.now() < deadline) {
  try {
    targets = await fetch(`http://127.0.0.1:${port}/json`).then(response => response.json());
    if (targets.some(target => target.url?.startsWith(`chrome-extension://${extensionId}/`))) break;
  } catch {}
  await new Promise(resolve => setTimeout(resolve, 200));
}
const target = targets?.find(item => item.url?.startsWith(`chrome-extension://${extensionId}/`));
if (!target) throw new Error("VidDock extension target was not found");
const socket = new WebSocket(target.webSocketDebuggerUrl);
await new Promise((resolve, reject) => { socket.addEventListener("open", resolve, { once: true }); socket.addEventListener("error", reject, { once: true }); });
let nextId = 1;
function evaluate(expression) {
  const id = nextId++;
  return new Promise((resolve, reject) => {
    const listener = event => {
      const message = JSON.parse(event.data);
      if (message.id !== id) return;
      socket.removeEventListener("message", listener);
      if (message.error || message.result?.exceptionDetails) reject(new Error(JSON.stringify(message.error || message.result.exceptionDetails)));
      else resolve(message.result.result.value);
    };
    socket.addEventListener("message", listener);
    socket.send(JSON.stringify({ id, method: "Runtime.evaluate", params: { expression, awaitPromise: true, returnByValue: true } }));
  });
}
await new Promise(resolve => setTimeout(resolve, 1500));
const expression = mode === "twitch" || mode === "twitch_labels" ? `(async () => {
  const url = ${JSON.stringify(requestedUrl)};
  const parsed = new URL(url);
  const parts = parsed.pathname.split('/').filter(Boolean);
  const videoId = parsed.hostname === 'clips.twitch.tv' ? parts[0] : (parts[0] === 'videos' ? parts[1] : parts[2]);
  const ping = await chrome.runtime.sendMessage({type:'native',command:'ping'});
  const metadata = await chrome.runtime.sendMessage({type:'native',command:'get_video_info',payload:{url,videoId}});
  ${mode === "twitch_labels" ? "if (metadata.ok) renderVideo(metadata.result.video);" : ""}
  return {version:chrome.runtime.getManifest().version,ping,metadata:{ok:metadata.ok,error:metadata.error,video:metadata.result?.video},ui:${mode === "twitch_labels" ? "{source:document.querySelector('#sourceLabel')?.textContent,options:[...document.querySelector('#quality').options].map(option => ({value:option.value,label:option.textContent})),duration:document.querySelector('#duration')?.textContent,estimate:document.querySelector('#estimateText')?.textContent}" : "null"},diagnostics:await chrome.storage.local.get(['lastNativeError'])};
})()` : mode === "labels" ? `(async () => {
  const videoId = ${JSON.stringify(requestedVideoId)};
  const url = 'https://www.youtube.com/watch?v=' + videoId;
  const metadata = await chrome.runtime.sendMessage({type:'native',command:'get_video_info',payload:{url,videoId}});
  renderVideo(metadata.result.video);
  return {version:chrome.runtime.getManifest().version,dimensions:metadata.result.video.qualities,options:[...document.querySelector('#quality').options].map(option => ({value:option.value,label:option.textContent}))};
})()` : mode === "full" ? `(async () => {
  const videoId = '2PuFyjAs7JA';
  const url = 'https://www.youtube.com/watch?v=' + videoId;
  const ping = await chrome.runtime.sendMessage({type:'native',command:'ping'});
  const metadata = await chrome.runtime.sendMessage({type:'native',command:'get_video_info',payload:{url,videoId}});
  const smallest = metadata.result?.video?.qualities?.at(-1);
  const exactQuality = smallest && typeof smallest === 'object' ? smallest.width + 'x' + smallest.height : '144';
  const id = 'browser_smoke_' + Date.now();
  const started = await chrome.runtime.sendMessage({type:'native',command:'start_download',id,payload:{url,videoId,mode:'video',quality:exactQuality,container:'mp4',audioFormat:'best'}});
  let job = null;
  for (let attempt = 0; attempt < 120; attempt++) {
    await new Promise(resolve => setTimeout(resolve, 500));
    job = (await chrome.runtime.sendMessage({type:'job_status',id})).job;
    if (job && ['finished','failed','cancelled'].includes(job.state)) break;
  }
  return {version:chrome.runtime.getManifest().version,ping,metadata:{ok:metadata.ok,id:metadata.result?.video?.id,qualities:metadata.result?.video?.qualities},started,job,diagnostics:await chrome.storage.local.get(['lastNativeError'])};
})()` : `(async () => ({ version: chrome.runtime.getManifest().version, ping: await chrome.runtime.sendMessage({type:'native',command:'ping'}), diagnostics: await chrome.storage.local.get(['lastNativeError']) }))()`;
const result = await evaluate(expression);
console.log(JSON.stringify(result));
socket.close();
