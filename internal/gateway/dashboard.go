package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (m *Manager) usageSummary(w http.ResponseWriter, r *http.Request) {
	since := sinceOf(r, 24*time.Hour)
	summary, err := m.store.UsageSummary(r.Context(), time.Now().Add(-since))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, summary)
}

func (m *Manager) usageSeries(w http.ResponseWriter, r *http.Request) {
	since := sinceOf(r, 24*time.Hour)
	points, err := m.store.UsageSeries(r.Context(), time.Now().Add(-since))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, points)
}

func (m *Manager) usageCalls(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	calls, err := m.store.RecentCallsSince(r.Context(), time.Now().Add(-sinceOf(r, 24*time.Hour)), limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, calls)
}

func (m *Manager) metrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	stats, err := m.store.CallStats(r.Context(), time.Unix(0, 0))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprintln(w, "# HELP mymcp_calls_total MCP tool calls recorded by MyMCP.")
	fmt.Fprintln(w, "# TYPE mymcp_calls_total counter")
	for _, stat := range stats {
		fmt.Fprintf(w, "mymcp_calls_total{server=%q,tool=%q,status=%q} %d\n",
			escapeLabel(stat.Server), escapeLabel(stat.Tool), escapeLabel(stat.Status), stat.Calls)
	}
	fmt.Fprintln(w, "# HELP mymcp_call_latency_ms_sum Sum of MCP call latency in milliseconds.")
	fmt.Fprintln(w, "# TYPE mymcp_call_latency_ms_sum counter")
	for _, stat := range stats {
		fmt.Fprintf(w, "mymcp_call_latency_ms_sum{server=%q,tool=%q,status=%q} %.1f\n",
			escapeLabel(stat.Server), escapeLabel(stat.Tool), escapeLabel(stat.Status), stat.AvgMS*float64(stat.Calls))
	}
	fmt.Fprintln(w, "# HELP mymcp_upstreams_running MCP servers currently running per profile.")
	fmt.Fprintln(w, "# TYPE mymcp_upstreams_running gauge")
	for profile, status := range m.Statuses() {
		fmt.Fprintf(w, "mymcp_upstreams_running{profile=%q} %d\n", escapeLabel(profile), status.Running)
	}
}

func (m *Manager) dashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

func sinceOf(r *http.Request, fallback time.Duration) time.Duration {
	raw := r.URL.Query().Get("since")
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	if days, err := strconv.Atoi(strings.TrimSuffix(raw, "d")); err == nil && days > 0 {
		return time.Duration(days) * 24 * time.Hour
	}
	return fallback
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func escapeLabel(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}

const dashboardHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>MyMCP usage</title>
<style>
:root{color-scheme:light dark;--bg:#fff;--surface:#f5f5f7;--text:#1d1d1f;--muted:#6e6e73;--accent:#2563eb;--danger:#dc2626}
@media (prefers-color-scheme:dark){:root{--bg:#1c1c1e;--surface:#2c2c2e;--text:#f5f5f7;--muted:#98989d;--accent:#60a5fa;--danger:#f87171}}
*{box-sizing:border-box}body{margin:0;background:var(--bg);color:var(--text);font:14px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif}
main{max-width:1100px;margin:0 auto;padding:28px}
h1{font-size:22px;margin:0 0 4px}.sub{color:var(--muted);margin-bottom:22px}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px;margin-bottom:20px}
.card{background:var(--surface);border-radius:12px;padding:14px 16px}.card b{display:block;font-size:24px}.card span{color:var(--muted);font-size:12px}
section{background:var(--surface);border-radius:12px;padding:16px;margin-bottom:16px}h2{font-size:14px;margin:0 0 12px}
.bars{display:flex;align-items:flex-end;gap:3px;height:120px}.bar{flex:1;min-width:2px;background:var(--accent);border-radius:2px 2px 0 0;opacity:.85}
table{width:100%;border-collapse:collapse;font-size:12.5px}th,td{text-align:left;padding:6px 8px;border-bottom:1px solid rgba(128,128,128,.2)}th{color:var(--muted);font-weight:500}
.err{color:var(--danger)}.muted{color:var(--muted)}code{font:12px ui-monospace,SFMono-Regular,Menlo,monospace}
</style>
</head>
<body><main>
<h1 id="title"></h1><div class="sub"><span id="subtitle"></span> · <a href="/dashboard?since=24h">24h</a> · <a href="/dashboard?since=7d">7d</a></div>
<div class="cards" id="cards"></div>
<section><h2 id="perhour"></h2><div class="bars" id="bars"></div></section>
<section><h2 id="top"></h2><table id="tools"></table></section>
<section><h2 id="recent"></h2><table id="calls"></table></section>
<script>
const zh = (navigator.language || 'en').toLowerCase().startsWith('zh');
const L = zh ? {
  title: 'MyMCP 用量', subtitle: '本地 MCP 调用', calls: '调用次数', errors: '错误', p95: 'P95 延迟', avg: '平均延迟',
  servers: '服务器', tools: '工具', perHour: '每小时调用', top: '热门工具', recent: '最近调用',
  noCalls: '还没有调用记录', nothing: '暂无数据', time: '时间', server: '服务器', tool: '工具',
  status: '状态', latency: '延迟', callsShort: '调用', avgShort: '平均'
} : {
  title: 'MyMCP usage', subtitle: 'Local MCP calls', calls: 'Calls', errors: 'Errors', p95: 'P95 latency', avg: 'Avg latency',
  servers: 'Servers', tools: 'Tools', perHour: 'Calls per hour', top: 'Top tools', recent: 'Recent calls',
  noCalls: 'No calls yet', nothing: 'Nothing yet', time: 'Time', server: 'Server', tool: 'Tool',
  status: 'Status', latency: 'Latency', callsShort: 'Calls', avgShort: 'Avg'
};
for (const [id, key] of [['title','title'],['subtitle','subtitle'],['perhour','perHour'],['top','top'],['recent','recent']]) {
  document.getElementById(id).textContent = L[key];
}
document.title = L.title;
const q = new URLSearchParams(location.search); const since = q.get('since') || '24h';
const esc = s => String(s ?? '').replace(/[&<>"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]));
async function json(url){ const r = await fetch(url); if(!r.ok) throw new Error(await r.text()); return r.json(); }
function cards(s){
  const err = s.calls ? (s.errors/s.calls*100).toFixed(1) : '0.0';
  document.querySelector('#cards').innerHTML = [
    [L.calls, s.calls], [L.errors, s.errors + ' (' + err + '%)'], [L.p95, Math.round(s.p95Ms) + ' ms'],
    [L.avg, Math.round(s.avgMs) + ' ms'], [L.servers, s.servers], [L.tools, s.tools]
  ].map(([k,v]) => '<div class="card"><b>'+esc(v)+'</b><span>'+k+'</span></div>').join('');
}
function bars(points){
  const max = Math.max(1, ...points.map(p => p.calls));
  document.querySelector('#bars').innerHTML = points.length
    ? points.map(p => '<div class="bar" title="'+esc(p.bucket)+': '+p.calls+' calls" style="height:'+(p.calls/max*100)+'%"></div>').join('')
    : '<div class="muted">'+esc(L.noCalls)+'</div>';
}
function table(id, rows, build){
  document.querySelector(id).innerHTML = rows.length
    ? build(rows)
    : '<tr><td class="muted">'+esc(L.nothing)+'</td></tr>';
}
async function load(){
  try {
    const s = await json('/api/usage/summary?since='+encodeURIComponent(since));
    cards(s);
    bars(await json('/api/usage/series?since='+encodeURIComponent(since)));
    table('#tools', s.top || [], rows => '<tr><th>'+L.server+'</th><th>'+L.tool+'</th><th>'+L.callsShort+'</th><th>'+L.avgShort+' ms</th><th>'+L.status+'</th></tr>' +
      rows.map(r => '<tr><td>'+esc(r.server)+'</td><td><code>'+esc(r.tool)+'</code></td><td>'+r.calls+'</td><td>'+Math.round(r.avgMs)+'</td><td>'+esc(r.status)+'</td></tr>').join(''));
    table('#calls', await json('/api/usage/calls?limit=50&since='+since), rows => '<tr><th>'+L.time+'</th><th>'+L.server+'</th><th>'+L.tool+'</th><th>'+L.status+'</th><th>ms</th></tr>' +
      rows.map(r => '<tr><td>'+esc(new Date(r.time).toLocaleTimeString())+'</td><td>'+esc(r.server)+'</td><td><code>'+esc(r.tool)+'</code></td><td class="'+(r.status==='ok'?'':'err')+'">'+esc(r.status)+'</td><td>'+r.latencyMs+'</td></tr>').join(''));
  } catch(e) { document.querySelector('#cards').innerHTML = '<div class="card err">'+esc(e.message)+'</div>'; }
}
load(); setInterval(load, 5000);
</script></main></body></html>`
