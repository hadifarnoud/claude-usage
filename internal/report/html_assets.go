package report

// htmlHead is everything before the embedded JSON payload: document head,
// stylesheet, and the empty page skeleton the script fills in.
const htmlHead = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Claude Usage Report</title>
<style>
:root {
  --bg: #f6f7f9;
  --panel: #ffffff;
  --border: #e2e6ec;
  --text: #16191d;
  --muted: #667085;
  --accent: #b45309;
  --bar: #d97757;
  --bar-soft: #f0c9b8;
  --row-hover: #f2f4f7;
  --shadow: 0 1px 2px rgba(16, 24, 40, .06), 0 1px 3px rgba(16, 24, 40, .1);
}
@media (prefers-color-scheme: dark) {
  :root:not([data-theme="light"]) {
    --bg: #14161a;
    --panel: #1b1e24;
    --border: #2c313a;
    --text: #e7eaef;
    --muted: #9aa4b2;
    --accent: #f0b27a;
    --bar: #d97757;
    --bar-soft: #4a3229;
    --row-hover: #232830;
    --shadow: none;
  }
}
:root[data-theme="dark"] {
  --bg: #14161a;
  --panel: #1b1e24;
  --border: #2c313a;
  --text: #e7eaef;
  --muted: #9aa4b2;
  --accent: #f0b27a;
  --bar: #d97757;
  --bar-soft: #4a3229;
  --row-hover: #232830;
  --shadow: none;
}
* { box-sizing: border-box; }
body {
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font: 15px/1.5 ui-sans-serif, -apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
  -webkit-font-smoothing: antialiased;
}
.wrap { max-width: 1180px; margin: 0 auto; padding: 32px 20px 64px; }
header { display: flex; flex-wrap: wrap; align-items: baseline; gap: 12px; margin-bottom: 24px; }
h1 { font-size: 22px; margin: 0; font-weight: 650; letter-spacing: -.01em; }
.gen { color: var(--muted); font-size: 13px; }
.spacer { flex: 1; }
button.theme {
  background: var(--panel); color: var(--muted); border: 1px solid var(--border);
  border-radius: 8px; padding: 6px 12px; font-size: 13px; cursor: pointer;
}
button.theme:hover { color: var(--text); }
.cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(168px, 1fr)); gap: 12px; margin-bottom: 28px; }
.card { background: var(--panel); border: 1px solid var(--border); border-radius: 12px; padding: 16px; box-shadow: var(--shadow); }
.card .label { color: var(--muted); font-size: 12px; text-transform: uppercase; letter-spacing: .06em; }
.card .value { font-size: 26px; font-weight: 650; margin-top: 6px; letter-spacing: -.02em; }
.card .sub { color: var(--muted); font-size: 12px; margin-top: 4px; }
section { background: var(--panel); border: 1px solid var(--border); border-radius: 12px; box-shadow: var(--shadow); margin-bottom: 24px; overflow: hidden; }
section > h2 { font-size: 14px; margin: 0; padding: 14px 18px; border-bottom: 1px solid var(--border); font-weight: 600; }
.chart { padding: 18px; overflow-x: auto; }
.chart svg { display: block; min-width: 100%; }
.chart .barval { fill: var(--muted); font-size: 9px; }
.chart .baraxis { fill: var(--muted); font-size: 10px; }
.tabs { display: flex; gap: 4px; padding: 12px 18px 0; flex-wrap: wrap; }
.tabs button {
  background: transparent; border: 1px solid transparent; border-bottom: none;
  color: var(--muted); padding: 8px 14px; border-radius: 8px 8px 0 0;
  font-size: 14px; cursor: pointer;
}
.tabs button[aria-selected="true"] { background: var(--bg); border-color: var(--border); color: var(--text); font-weight: 600; }
.toolbar { display: flex; gap: 10px; padding: 12px 18px; align-items: center; flex-wrap: wrap; border-top: 1px solid var(--border); }
.toolbar input {
  flex: 1; min-width: 180px; background: var(--bg); color: var(--text);
  border: 1px solid var(--border); border-radius: 8px; padding: 8px 12px; font-size: 14px;
}
.toolbar .count { color: var(--muted); font-size: 13px; }
.tablewrap { overflow-x: auto; }
table { border-collapse: collapse; width: 100%; font-size: 14px; }
th, td { text-align: left; padding: 9px 18px; border-bottom: 1px solid var(--border); white-space: nowrap; }
th { color: var(--muted); font-weight: 600; font-size: 12px; text-transform: uppercase; letter-spacing: .05em; position: sticky; top: 0; background: var(--panel); }
th.sortable { cursor: pointer; user-select: none; }
th.sortable:hover { color: var(--text); }
td.num, th.num { text-align: right; font-variant-numeric: tabular-nums; }
td.name { max-width: 560px; overflow: hidden; text-overflow: ellipsis; }
tbody tr.row { cursor: pointer; }
tbody tr.row:hover { background: var(--row-hover); }
tr.detail > td { background: var(--bg); white-space: normal; padding: 16px 18px; }
.detail { max-width: 900px; }
.detail h3 { font-size: 12px; text-transform: uppercase; letter-spacing: .05em; color: var(--muted); margin: 16px 0 8px; }
.detail h3:first-child { margin-top: 0; }
.detail .meta { text-transform: none; letter-spacing: 0; font-size: 13px; }
.detail .prompt { color: var(--muted); font-size: 13px; margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; }
.detail table { font-size: 13px; }
.detail th, .detail td { padding: 5px 10px; white-space: normal; }
.detail table th:first-child, .detail table td:first-child { padding-left: 0; }
.detail td.agent { overflow-wrap: anywhere; }
.detail td.agent .desc { color: var(--muted); display: block; }
.pill { display: inline-block; background: var(--bar-soft); color: var(--accent); border-radius: 999px; padding: 1px 8px; font-size: 11px; font-weight: 600; margin-left: 6px; }
.empty { padding: 28px 18px; color: var(--muted); }
footer { color: var(--muted); font-size: 12px; text-align: center; margin-top: 8px; }
</style>
</head>
<body>
<div class="wrap">
  <header>
    <h1>Claude Usage Report</h1>
    <span class="gen" id="gen"></span>
    <span class="spacer"></span>
    <button class="theme" id="theme" type="button">Theme</button>
  </header>
  <div class="cards" id="cards"></div>
  <section id="chart-section">
    <h2>Daily cost</h2>
    <div class="chart" id="chart"></div>
  </section>
  <section>
    <div class="tabs" id="tabs" role="tablist"></div>
    <div class="toolbar">
      <input id="search" type="search" placeholder="Filter rows…" autocomplete="off">
      <span class="count" id="count"></span>
    </div>
    <div class="tablewrap" id="table"></div>
  </section>
  <footer>Generated by claude-usage. Costs are estimates at Anthropic list prices.</footer>
</div>
`

// htmlScript renders the page from the embedded JSON payload.
const htmlScript = `<script>
(function () {
  "use strict";
  var DATA = JSON.parse(document.getElementById("usage-data").textContent);

  function money(v) { return "$" + (v || 0).toFixed(2); }
  function num(v) { return (v || 0).toLocaleString("en-US"); }
  function compact(v) {
    v = v || 0;
    if (v >= 1e9) return (v / 1e9).toFixed(1) + "B";
    if (v >= 1e6) return (v / 1e6).toFixed(1) + "M";
    if (v >= 1e3) return (v / 1e3).toFixed(1) + "K";
    return String(v);
  }
  function shortModel(m) { return String(m || "").replace(/^claude-/, ""); }
  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined && text !== null) n.textContent = String(text);
    return n;
  }

  // ---- header + summary cards ----
  document.getElementById("gen").textContent = "generated " + DATA.generatedAt;
  var t = DATA.totals;
  var totalTokens = t.input + t.cacheW + t.cacheR + t.output;
  var cards = [
    { label: "Total cost", value: money(t.cost), sub: t.sessions + " sessions" },
    { label: "Tokens", value: compact(totalTokens), sub: num(totalTokens) + " total" },
    { label: "Input", value: money(t.costIn), sub: compact(t.input) + " tokens" },
    { label: "Cache write", value: money(t.costCacheW), sub: compact(t.cacheW) + " tokens" },
    { label: "Cache read", value: money(t.costCacheR), sub: compact(t.cacheR) + " tokens" },
    { label: "Output", value: money(t.costOut), sub: compact(t.output) + " tokens" }
  ];
  var cardBox = document.getElementById("cards");
  cards.forEach(function (c) {
    var card = el("div", "card");
    card.appendChild(el("div", "label", c.label));
    card.appendChild(el("div", "value", c.value));
    card.appendChild(el("div", "sub", c.sub));
    cardBox.appendChild(card);
  });

  // ---- daily bar chart (inline SVG, no libraries) ----
  function drawChart() {
    var box = document.getElementById("chart");
    var days = DATA.days || [];
    if (!days.length) {
      document.getElementById("chart-section").style.display = "none";
      return;
    }
    var barW = 26, gap = 6, padL = 58, padT = 12, padB = 30, h = 180;
    var w = padL + days.length * (barW + gap) + 12;
    var max = 0;
    days.forEach(function (d) { if (d.cost > max) max = d.cost; });
    if (max <= 0) max = 1;
    var ns = "http://www.w3.org/2000/svg";
    var svg = document.createElementNS(ns, "svg");
    svg.setAttribute("viewBox", "0 0 " + w + " " + (h + padT + padB));
    svg.setAttribute("width", w);
    svg.setAttribute("height", h + padT + padB);
    function mk(tag, attrs, text) {
      var n = document.createElementNS(ns, tag);
      Object.keys(attrs).forEach(function (k) { n.setAttribute(k, attrs[k]); });
      if (text !== undefined) n.textContent = text;
      return n;
    }
    // y axis: zero line and max label
    svg.appendChild(mk("line", { x1: padL, y1: padT + h, x2: w, y2: padT + h, stroke: "currentColor", "stroke-opacity": ".18" }));
    svg.appendChild(mk("text", { x: padL - 10, y: padT + 8, "text-anchor": "end", class: "baraxis" }, "$" + compact(Math.round(max))));
    svg.appendChild(mk("text", { x: padL - 10, y: padT + h + 4, "text-anchor": "end", class: "baraxis" }, "$0"));

    days.forEach(function (d, i) {
      var bh = Math.max(2, Math.round((d.cost / max) * h));
      var x = padL + i * (barW + gap);
      var y = padT + h - bh;
      var bar = mk("rect", { x: x, y: y, width: barW, height: bh, rx: 3, fill: "var(--bar)" });
      bar.appendChild(mk("title", {}, d.day + "  " + money(d.cost) + "  " + num(d.tokens) + " tokens"));
      svg.appendChild(bar);
      // label every bar when there is room, otherwise every other one
      var step = days.length > 24 ? 3 : (days.length > 12 ? 2 : 1);
      if (i % step === 0) {
        svg.appendChild(mk("text", {
          x: x + barW / 2, y: padT + h + 16, "text-anchor": "middle", class: "baraxis"
        }, d.day.slice(5)));
      }
    });
    box.appendChild(svg);
  }
  drawChart();

  // ---- tab definitions ----
  var TABS = [
    {
      id: "sessions", label: "Sessions",
      cols: [
        { key: "title", label: "Session", cls: "name" },
        { key: "cost", label: "Cost", num: true, fmt: money },
        { key: "input", label: "Input", num: true, fmt: compact },
        { key: "cacheW", label: "Cache W", num: true, fmt: compact },
        { key: "cacheR", label: "Cache R", num: true, fmt: compact },
        { key: "output", label: "Output", num: true, fmt: compact },
        { key: "_models", label: "Models" },
        { key: "date", label: "Date" }
      ],
      rows: function () {
        return (DATA.sessions || []).map(function (s) {
          var r = Object.create(s);
          r._models = (s.models || []).map(function (m) { return shortModel(m.model); }).join(", ");
          r._search = [s.title, s.project, s.branch, r._models, s.date].join(" ").toLowerCase();
          return r;
        });
      },
      sort: "cost", expand: true
    },
    {
      id: "models", label: "Models",
      cols: [
        { key: "model", label: "Model", cls: "name" },
        { key: "input", label: "Input", num: true, fmt: num },
        { key: "cacheW", label: "Cache W", num: true, fmt: num },
        { key: "cacheR", label: "Cache R", num: true, fmt: num },
        { key: "output", label: "Output", num: true, fmt: num },
        { key: "turns", label: "Turns", num: true, fmt: num },
        { key: "cost", label: "Cost", num: true, fmt: money }
      ],
      rows: function () {
        return (DATA.models || []).map(function (m) {
          var r = Object.create(m);
          r._search = String(m.model).toLowerCase();
          return r;
        });
      },
      sort: "cost"
    },
    {
      id: "projects", label: "Projects",
      cols: [
        { key: "project", label: "Project", cls: "name" },
        { key: "sessions", label: "Sessions", num: true, fmt: num },
        { key: "tokens", label: "Tokens", num: true, fmt: num },
        { key: "cost", label: "Cost", num: true, fmt: money }
      ],
      rows: function () {
        return (DATA.projects || []).map(function (p) {
          var r = Object.create(p);
          r._search = String(p.project).toLowerCase();
          return r;
        });
      },
      sort: "cost"
    },
    {
      id: "daily", label: "Daily",
      cols: [
        { key: "day", label: "Date" },
        { key: "tokens", label: "Tokens", num: true, fmt: num },
        { key: "cost", label: "Cost", num: true, fmt: money }
      ],
      rows: function () {
        return (DATA.days || []).slice().reverse().map(function (d) {
          var r = Object.create(d);
          r._search = String(d.day).toLowerCase();
          return r;
        });
      },
      sort: "day", desc: true
    }
  ];

  var active = TABS[0];
  var sortKey = active.sort;
  var sortDesc = true;
  var query = "";

  // ---- tab bar ----
  var tabBox = document.getElementById("tabs");
  TABS.forEach(function (tab) {
    var b = el("button", null, tab.label);
    b.type = "button";
    b.setAttribute("role", "tab");
    b.dataset.id = tab.id;
    b.addEventListener("click", function () {
      active = tab;
      sortKey = tab.sort;
      sortDesc = true;
      render();
    });
    tabBox.appendChild(b);
  });

  document.getElementById("search").addEventListener("input", function (e) {
    query = e.target.value.trim().toLowerCase();
    render();
  });

  // ---- expanded session detail ----
  function modelTable(models) {
    var tbl = el("table");
    var thead = el("thead");
    var hr = el("tr");
    ["Model", "Input", "Cache W", "Cache R", "Output", "Cost"].forEach(function (h, i) {
      var th = el("th", i === 0 ? null : "num", h);
      hr.appendChild(th);
    });
    thead.appendChild(hr);
    tbl.appendChild(thead);
    var tb = el("tbody");
    (models || []).forEach(function (m) {
      var tr = el("tr");
      tr.appendChild(el("td", null, shortModel(m.model)));
      [m.input, m.cacheW, m.cacheR, m.output].forEach(function (v) {
        tr.appendChild(el("td", "num", num(v)));
      });
      tr.appendChild(el("td", "num", money(m.cost)));
      tb.appendChild(tr);
    });
    tbl.appendChild(tb);
    return tbl;
  }

  function subagentTable(subs) {
    // The workflow column only earns its width when a workflow spawned some
    // of these agents; plain Task-tool dispatches have no provenance.
    var hasWorkflow = subs.some(function (sa) { return !!sa.workflow; });
    var heads = ["Agent"];
    if (hasWorkflow) heads.push("Workflow");
    heads = heads.concat(["Models", "Turns", "Cost"]);

    var tbl = el("table");
    var thead = el("thead");
    var hr = el("tr");
    heads.forEach(function (h) {
      hr.appendChild(el("th", (h === "Turns" || h === "Cost") ? "num" : null, h));
    });
    thead.appendChild(hr);
    tbl.appendChild(thead);

    var tb = el("tbody");
    subs.forEach(function (sa) {
      var tr = el("tr");
      var td = el("td", "agent", sa.label || sa.agentType || "agent");
      if (sa.description) td.appendChild(el("span", "desc", sa.description));
      tr.appendChild(td);
      if (hasWorkflow) {
        var wf = [];
        if (sa.workflow) wf.push(sa.workflow);
        if (sa.phase) wf.push(sa.phase);
        tr.appendChild(el("td", "agent", wf.join(" · ") || "—"));
      }
      tr.appendChild(el("td", null, (sa.models || []).map(function (m) {
        return shortModel(m.model);
      }).join(", ")));
      tr.appendChild(el("td", "num", num(sa.turns)));
      tr.appendChild(el("td", "num", money(sa.cost)));
      tb.appendChild(tr);
    });
    tbl.appendChild(tb);
    return tbl;
  }

  function detailCell(s) {
    var box = el("div", "detail");
    var meta = [];
    if (s.project) meta.push("project: " + s.project);
    if (s.branch) meta.push("branch: " + s.branch);
    if (s.started) meta.push("started: " + s.started);
    if (s.duration) meta.push("duration: " + s.duration);
    if (s.turns) meta.push("turns: " + s.turns);
    box.appendChild(el("h3", "meta", meta.join("  ·  ")));
    if (s.prompt) {
      box.appendChild(el("h3", null, "First prompt"));
      box.appendChild(el("p", "prompt", s.prompt));
    }
    box.appendChild(el("h3", null, "Models"));
    box.appendChild(modelTable(s.models));
    if (s.subagents && s.subagents.length) {
      box.appendChild(el("h3", null, "Subagents — " + s.subagents.length + " for " + money(s.subCost)));
      box.appendChild(subagentTable(s.subagents));
    }
    return box;
  }

  // ---- table rendering ----
  function render() {
    Array.prototype.forEach.call(tabBox.children, function (b) {
      b.setAttribute("aria-selected", b.dataset.id === active.id ? "true" : "false");
    });

    var rows = active.rows();
    if (query) {
      rows = rows.filter(function (r) { return r._search.indexOf(query) !== -1; });
    }
    rows.sort(function (a, b) {
      var x = a[sortKey], y = b[sortKey];
      var c;
      if (typeof x === "number" && typeof y === "number") c = x - y;
      else c = String(x === undefined ? "" : x).localeCompare(String(y === undefined ? "" : y));
      return sortDesc ? -c : c;
    });

    document.getElementById("count").textContent = rows.length + " rows";

    var host = document.getElementById("table");
    host.textContent = "";
    if (!rows.length) {
      host.appendChild(el("div", "empty", "No rows match this filter."));
      return;
    }

    var tbl = el("table");
    var thead = el("thead");
    var hr = el("tr");
    active.cols.forEach(function (c) {
      var th = el("th", "sortable" + (c.num ? " num" : ""));
      th.textContent = c.label + (sortKey === c.key ? (sortDesc ? " ↓" : " ↑") : "");
      th.addEventListener("click", function () {
        if (sortKey === c.key) sortDesc = !sortDesc;
        else { sortKey = c.key; sortDesc = !!c.num; }
        render();
      });
      hr.appendChild(th);
    });
    thead.appendChild(hr);
    tbl.appendChild(thead);

    var tb = el("tbody");
    rows.forEach(function (r) {
      var tr = el("tr", "row");
      active.cols.forEach(function (c) {
        var raw = r[c.key];
        var td = el("td", (c.num ? "num " : "") + (c.cls || ""));
        td.textContent = c.fmt ? c.fmt(raw) : (raw === undefined || raw === null ? "" : String(raw));
        if (c.key === "title" && r.subagents && r.subagents.length) {
          td.appendChild(el("span", "pill", r.subagents.length + " sub"));
        }
        tr.appendChild(td);
      });
      tb.appendChild(tr);

      if (active.expand) {
        var dr = el("tr", "detail");
        dr.style.display = "none";
        var dc = el("td");
        dc.colSpan = active.cols.length;
        dc.appendChild(detailCell(r));
        dr.appendChild(dc);
        tb.appendChild(dr);
        tr.addEventListener("click", function () {
          dr.style.display = dr.style.display === "none" ? "" : "none";
        });
      }
    });
    tbl.appendChild(tb);
    host.appendChild(tbl);
  }

  // ---- theme toggle ----
  var order = ["auto", "light", "dark"];
  var mode = "auto";
  try { mode = localStorage.getItem("cu-theme") || "auto"; } catch (e) { mode = "auto"; }
  function applyTheme() {
    if (mode === "auto") document.documentElement.removeAttribute("data-theme");
    else document.documentElement.setAttribute("data-theme", mode);
    document.getElementById("theme").textContent = "Theme: " + mode;
  }
  document.getElementById("theme").addEventListener("click", function () {
    mode = order[(order.indexOf(mode) + 1) % order.length];
    try { localStorage.setItem("cu-theme", mode); } catch (e) { /* private mode */ }
    applyTheme();
  });
  applyTheme();

  render();
})();
</script>
`

const htmlTail = `</body>
</html>
`
