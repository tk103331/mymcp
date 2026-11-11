// MyMCP plugin: read and write a fictional agent's MCP configuration.
//
// render(ctx) returns the complete document. ctx.mode is:
//   "router"  -> one entry pointing at ctx.endpoint
//   "direct"  -> one entry per server in ctx.servers
//   "detach"  -> remove only the entries MyMCP wrote
// load(ctx) returns servers for `mymcp import`.

const ENTRY = "mymcp";

export function render(ctx) {
  const doc = ctx.document || {};
  doc.mcpServers ??= {};
  const servers = doc.mcpServers;

  for (const name of ctx.remove || []) delete servers[name];

  if (ctx.mode === "detach") {
    delete servers[ctx.entry || ENTRY];
    return doc;
  }

  if (ctx.mode === "router") {
    servers[ctx.entry || ENTRY] = { url: ctx.endpoint };
    return doc;
  }

  for (const server of ctx.servers || []) {
    const key = server.key || server.name;
    if (server.transport === "stdio") {
      servers[key] = {
        command: server.command,
        args: server.args || [],
        env: server.env || {},
      };
    } else {
      servers[key] = {
        url: server.url,
        headers: server.headers || {},
      };
    }
  }
  return doc;
}

export function load(ctx) {
  const doc = ctx.document || {};
  const servers = [];
  for (const [name, entry] of Object.entries(doc.mcpServers || {})) {
    if (name === ENTRY) continue; // MyMCP's own router entry
    if (entry.url) {
      servers.push({ name, transport: "sse", url: entry.url, headers: entry.headers || {} });
    } else if (entry.command) {
      servers.push({
        name,
        transport: "stdio",
        command: entry.command,
        args: entry.args || [],
        env: entry.env || {},
      });
    }
  }
  return servers;
}
