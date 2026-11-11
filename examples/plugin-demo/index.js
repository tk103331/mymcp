// MyMCP plugin: read and write a fictional agent's MCP configuration.
//
// render(ctx) returns the whole new file content. ctx.mode is:
//   "router"  -> one entry pointing at ctx.endpoint
//   "direct"  -> one entry per server in ctx.servers
//   "detach"  -> remove only MyMCP's entries
// load(ctx) returns servers for `mymcp import`.

const ENTRY = "mymcp";

function parse(content) {
  return content ? JSON.parse(content) : {};
}

function stringify(doc) {
  return JSON.stringify(doc, null, 2) + "\n";
}

export function render(ctx) {
  const doc = parse(ctx.content);
  doc.mcpServers ??= {};

  if (ctx.mode === "detach") {
    delete doc.mcpServers[ENTRY];
    return stringify(doc);
  }

  if (ctx.mode === "router") {
    doc.mcpServers[ENTRY] = { url: ctx.endpoint };
    return stringify(doc);
  }

  for (const server of ctx.servers) {
    if (server.transport === "stdio") {
      doc.mcpServers[server.name] = {
        command: server.command,
        args: server.args || [],
        env: server.env || {},
      };
    } else {
      doc.mcpServers[server.name] = {
        url: server.url,
        headers: server.headers || {},
      };
    }
  }
  return stringify(doc);
}

export function load(ctx) {
  const doc = parse(ctx.content);
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
