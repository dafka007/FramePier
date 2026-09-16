const [portText, targetFragment, ...expressionParts] = process.argv.slice(2);
if (!portText || !targetFragment || expressionParts.length === 0) {
  console.error("Usage: node cdp-eval.mjs <port> <target-url-fragment> <expression>");
  process.exit(2);
}

const targets = await (await fetch(`http://127.0.0.1:${Number(portText)}/json/list`)).json();
const target = targets.find((item) => item.url.includes(targetFragment));
if (!target) {
  console.error(`Target not found: ${targetFragment}`);
  console.error(targets.map((item) => `${item.type} ${item.url}`).join("\n"));
  process.exit(3);
}

const socket = new WebSocket(target.webSocketDebuggerUrl);
await new Promise((resolve, reject) => {
  socket.addEventListener("open", resolve, { once: true });
  socket.addEventListener("error", reject, { once: true });
});

const response = await new Promise((resolve, reject) => {
  const id = 1;
  const timer = setTimeout(() => reject(new Error("CDP evaluation timed out")), 120000);
  socket.addEventListener("message", (event) => {
    const message = JSON.parse(event.data);
    if (message.id !== id) return;
    clearTimeout(timer);
    if (message.error) reject(new Error(message.error.message));
    else resolve(message.result);
  });
  socket.send(JSON.stringify({
    id,
    method: "Runtime.evaluate",
    params: { expression: expressionParts.join(" "), awaitPromise: true, returnByValue: true }
  }));
});

socket.close();
console.log(JSON.stringify(response.result, null, 2));
