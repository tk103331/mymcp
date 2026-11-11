function section(doc, key) {
  if (!doc[key] || typeof doc[key] !== "object" || Array.isArray(doc[key])) doc[key] = {};
  return doc[key];
}
function sse(url) {
  url = String(url || "").replace(/\/+$/, "");
  return url.endsWith("/sse") ? url : url + "/sse";
}
function bridge(ctx, url) {
  return { command: ctx.bridge || "mcp-pipe", args: ["--mode", "sse2stdio", "--url", sse(url)] };
}
function withEnv(entry, server) {
  if (server.env && Object.keys(server.env).length) entry.env = server.env;
  return entry;
}
function withHeaders(entry, server) {
  if (server.headers && Object.keys(server.headers).length) entry.headers = server.headers;
  return entry;
}
function prune(ctx, servers) {
  for (const name of (ctx.remove || [])) delete servers[name];
}
function key(server) { return server.key || server.name; }
function entryName(ctx) { return ctx.entry || "mymcp"; }
function simpleStdio(server) {
  return withEnv({ command: server.command, args: server.args || [] }, server);
}
function simpleRemote(server) {
  return withHeaders({ url: server.url }, server);
}
function loadGeneric(name, entry) {
  if (entry.command || entry.cmd) {
    return {
      name, transport: "stdio",
      command: entry.command || entry.cmd,
      args: entry.args || [],
      env: entry.env || entry.envs || entry.environment || {},
    };
  }
  const url = entry.serverUrl || entry.httpUrl || entry.url || entry.uri;
  if (!url) return null;
  const kind = String(entry.type || "").toLowerCase();
  let transport = "sse";
  if (kind === "http" || kind === "streamablehttp" || kind === "streamable-http" || entry.httpUrl) transport = "http";
  else if (kind === "sse" || kind === "streamable_http") transport = "sse";
  return { name, transport, url, headers: entry.headers || {} };
}

export function render(ctx) {
  const doc = ctx.document || {};
  const servers = section(doc, "mcp");
  prune(ctx, servers);
  if (ctx.mode === "detach") {
    if (!(ctx.remove || []).length) delete servers[entryName(ctx)];
    return doc;
  }
  if (ctx.mode === "router") {
    servers[entryName(ctx)] = { type: "remote", url: ctx.endpoint, enabled: true };
    return doc;
  }
  for (const server of ctx.servers || []) {
    servers[key(server)] = (server.transport === "stdio" ? withEnv({ type: "local", command: [server.command].concat(server.args || []), enabled: true }, server) : withHeaders({ type: "remote", url: server.url, enabled: true }, server));
  }
  return doc;
}

export function load(ctx) {
  const servers = section(ctx.document || {}, "mcp");
  const out = [];
  for (const [name, raw] of Object.entries(servers)) {
    if (!raw || typeof raw !== "object" || name === "mymcp") continue;
    const entry = raw.transport && typeof raw.transport === "object" ? raw.transport : raw;
    if (String(entry.type).toLowerCase() === "local") { const cmd = entry.command || []; out.push({name, transport: "stdio", command: cmd[0] || "", args: cmd.slice(1), env: entry.environment || {}}); } else if (entry.url) { out.push({name, transport: "sse", url: entry.url, headers: entry.headers || {}}); }
  }
  return out;
}

