/* shint docs: the Performance page. docs/build.py embeds .profiling/'s runs as JSON (#perf-data);
   this draws, for one machine at a time: the machine, two builds compared, and every scenario over
   the builds. No library - SVG by hand, colours from the site's theme. */
(function () {
  "use strict";
  var root = document.getElementById("perf"), src = document.getElementById("perf-data");
  if (!root || !src) return;
  var D = JSON.parse(src.textContent);
  var SVGNS = "http://www.w3.org/2000/svg";
  var METRICS = { w: ["Wall time", "ms"], c: ["CPU time", "ms"], r: ["Peak memory", "MiB"] };

  function el(tag, attrs, kids) {
    var e = document.createElement(tag);
    for (var k in attrs || {}) {
      if (k === "text") e.textContent = attrs[k]; else e.setAttribute(k, attrs[k]);
    }
    (kids || []).forEach(function (c) { if (c) e.appendChild(c); });
    return e;
  }
  function sv(tag, attrs) {
    var e = document.createElementNS(SVGNS, tag);
    for (var k in attrs) e.setAttribute(k, attrs[k]);
    return e;
  }
  function fmt(v) { return v >= 100 ? v.toFixed(0) : v >= 10 ? v.toFixed(1) : v.toFixed(2); }
  function mib(bytes) { return bytes / 1048576; }

  // Runs on a machine, one per build (the latest), in version order - D.runs is already sorted.
  function builds(mid) {
    var seen = {}, out = [];
    D.runs.forEach(function (r) {
      if (r.machine !== mid) return;
      if (seen[r.tag] !== undefined) out[seen[r.tag]] = r; else { seen[r.tag] = out.length; out.push(r); }
    });
    return out;
  }
  var counts = {};
  D.runs.forEach(function (r) { counts[r.machine] = (counts[r.machine] || 0) + 1; });
  var machineIds = Object.keys(counts).sort(function (a, b) { return counts[b] - counts[a] || (a < b ? -1 : 1); });
  if (!machineIds.length) { root.appendChild(el("p", { text: "No profiling runs yet." })); return; }

  var state = { machine: machineIds[0], metric: "w", a: null, b: null };
  function resetPair() {
    var bs = builds(state.machine);
    state.b = bs[bs.length - 1].tag;
    state.a = bs[Math.max(0, bs.length - 2)].tag;
  }
  resetPair();

  function machineName(mid) {
    var m = D.machines[mid] || {};
    return (m.label || m.model || m.cpu || "machine") + " \u00b7 " + mid;
  }

  function select(label, options, value, onchange) {
    var s = el("select");
    options.forEach(function (o) {
      var opt = el("option", { value: o[0], text: o[1] });
      if (o[0] === value) opt.selected = true;
      s.appendChild(opt);
    });
    s.addEventListener("change", function () { onchange(s.value); render(); });
    return el("label", {}, [el("span", { text: label }), s]);
  }

  function controls() {
    var bs = builds(state.machine).map(function (r) { return [r.tag, r.tag + (r.kind === "backfill" ? " (backfill)" : "")]; });
    return el("div", { class: "perf-controls" }, [
      select("Machine", machineIds.map(function (m) { return [m, machineName(m) + " (" + counts[m] + " runs)"]; }), state.machine,
        function (v) { state.machine = v; resetPair(); }),
      select("Metric", Object.keys(METRICS).map(function (k) { return [k, METRICS[k][0]]; }), state.metric, function (v) { state.metric = v; }),
      select("Compare", bs, state.a, function (v) { state.a = v; }),
      select("with", bs, state.b, function (v) { state.b = v; })
    ]);
  }

  function machineCard() {
    var m = D.machines[state.machine] || {}, bs = builds(state.machine);
    var cores = (m.cores || "?") + " logical" + (m.physical ? ", " + m.physical + " physical" : "") +
      (m.performance ? " (" + m.performance + " performance + " + (m.efficiency || 0) + " efficiency)" : "");
    var rows = [["Model", m.model], ["CPU", m.cpu], ["Cores", cores],
      ["Memory", m.memory ? (m.memory / 1073741824).toFixed(0) + " GiB" : ""], ["System", [m.os, m.arch].filter(Boolean).join(", ")],
      ["Go", m.go], ["Builds", bs.length + " (" + bs[0].tag + " \u2192 " + bs[bs.length - 1].tag + ")"], ["Id", state.machine]];
    var dl = el("dl");
    rows.forEach(function (r) { if (r[1]) { dl.appendChild(el("dt", { text: r[0] })); dl.appendChild(el("dd", { text: r[1] })); } });
    return el("div", { class: "perf-card" }, [dl]);
  }

  // A change is "within noise" when it is smaller than either run's own spread (max - min) / median.
  function delta(a, b) {
    if (!a || !b || !a[0]) return null;
    var d = (b[0] - a[0]) / a[0], noise = Math.max((a[2] - a[1]) / a[0], (b[2] - b[1]) / (b[0] || 1));
    return { d: d, cls: Math.abs(d) <= noise ? "same" : d < 0 ? "better" : "worse" };
  }

  function compareTable() {
    var bs = builds(state.machine), A, B;
    bs.forEach(function (r) { if (r.tag === state.a) A = r; if (r.tag === state.b) B = r; });
    var unit = METRICS[state.metric][1];
    var body = el("tbody");
    D.scenarios.forEach(function (s) {
      var a = A.s[s[0]] && A.s[s[0]][state.metric], b = B.s[s[0]] && B.s[s[0]][state.metric], x = delta(a, b);
      body.appendChild(el("tr", {}, [
        el("td", {}, [el("code", { text: s[0] })]),
        el("td", { text: a ? fmt(a[0]) : "n/a" }), el("td", { text: b ? fmt(b[0]) : "n/a" }),
        el("td", { class: "perf-delta " + (x ? x.cls : ""), text: x ? (x.d > 0 ? "+" : "") + (x.d * 100).toFixed(1) + "%" + (x.cls === "same" ? " ~" : "") : "" })
      ]));
    });
    var sizeA = A.size, sizeB = B.size;
    if (sizeA && sizeB) {
      var ds = (sizeB - sizeA) / sizeA;
      body.appendChild(el("tr", {}, [el("td", { text: "binary size (MiB)" }), el("td", { text: fmt(mib(sizeA)) }),
        el("td", { text: fmt(mib(sizeB)) }), el("td", { class: "perf-delta " + (Math.abs(ds) < 0.001 ? "same" : ds < 0 ? "better" : "worse"), text: (ds > 0 ? "+" : "") + (ds * 100).toFixed(1) + "%" })]));
    }
    var head = el("thead", {}, [el("tr", {}, [el("th", { text: "scenario" }), el("th", { text: state.a }),
      el("th", { text: state.b }), el("th", { text: "change" })])]);
    return el("div", {}, [
      el("h3", { text: METRICS[state.metric][0] + " (" + unit + "): " + state.a + " \u2192 " + state.b }),
      el("div", { class: "tablewrap" }, [el("table", { class: "perf-table" }, [head, body])]),
      el("p", { class: "perf-note", text: "Medians. Lower is better. ~ marks a change smaller than the runs' own spread (their max - min), i.e. noise." })
    ]);
  }

  // One small chart: points = [{tag, kind, v: [median, min, max]}], the band is min..max.
  function chart(title, sub, points, unit) {
    var W = 360, H = 150, L = 42, R = 14, T = 12, Bm = 30;
    var svg = sv("svg", { viewBox: "0 0 " + W + " " + H, role: "img", "aria-label": title + " over the builds" });
    var top = 0;
    points.forEach(function (p) { top = Math.max(top, p.v[2]); });
    top = top > 0 ? top * 1.1 : 1;
    var n = points.length;
    function x(i) { return n < 2 ? (L + W - R) / 2 : L + i * (W - L - R) / (n - 1); }
    function y(v) { return T + (H - T - Bm) * (1 - v / top); }
    [0, top / 2, top].forEach(function (v) {
      svg.appendChild(sv("line", { x1: L, x2: W - R, y1: y(v), y2: y(v), style: "stroke:var(--border)", "stroke-width": 1 }));
      var t = sv("text", { x: L - 6, y: y(v) + 4, "text-anchor": "end", style: "fill:var(--muted);font-size:10px" });
      t.textContent = fmt(v);
      svg.appendChild(t);
    });
    if (n) {
      var band = points.map(function (p, i) { return x(i) + "," + y(p.v[2]); })
        .concat(points.slice().reverse().map(function (p, j) { return x(n - 1 - j) + "," + y(p.v[1]); }));
      svg.appendChild(sv("polygon", { points: band.join(" "), style: "fill:var(--accent-soft);stroke:none" }));
      svg.appendChild(sv("polyline", { points: points.map(function (p, i) { return x(i) + "," + y(p.v[0]); }).join(" "),
        style: "fill:none;stroke:var(--accent)", "stroke-width": 2 }));
    }
    var step = Math.max(1, Math.ceil(n / 5));
    points.forEach(function (p, i) {
      var picked = p.tag === state.a || p.tag === state.b;
      var c = sv("circle", { cx: x(i), cy: y(p.v[0]), r: picked ? 4.5 : 3,
        style: "stroke:var(--accent);stroke-width:2;fill:" + (p.kind === "backfill" ? "var(--bg)" : "var(--accent)") });
      var tip = sv("title", {});
      tip.textContent = p.tag + (p.kind === "backfill" ? " (backfill)" : "") + ": " + fmt(p.v[0]) + " " + unit +
        " (" + fmt(p.v[1]) + " to " + fmt(p.v[2]) + ")";
      c.appendChild(tip);
      svg.appendChild(c);
      // every step-th build, and the last one - anchored at its end so it stays inside the chart
      if (i === n - 1 || (i % step === 0 && n - 1 - i >= step / 2 + 0.5)) {
        var t = sv("text", { x: x(i), y: H - 10, "text-anchor": i === n - 1 && n > 1 ? "end" : "middle", style: "fill:var(--muted);font-size:10px" });
        t.textContent = p.tag.replace(/^v/, "");
        svg.appendChild(t);
      }
    });
    return el("div", { class: "perf-chart" }, [el("h4", { text: title }), el("p", { text: sub }), svg]);
  }

  function charts() {
    var bs = builds(state.machine), unit = METRICS[state.metric][1], grid = el("div", { class: "perf-grid" });
    D.scenarios.forEach(function (s) {
      var pts = [];
      bs.forEach(function (r) { if (r.s[s[0]]) pts.push({ tag: r.tag, kind: r.kind, v: r.s[s[0]][state.metric] }); });
      if (pts.length) grid.appendChild(chart(s[0], s[1] + " \u00b7 " + METRICS[state.metric][0].toLowerCase() + ", " + unit, pts, unit));
    });
    var size = bs.filter(function (r) { return r.size; }).map(function (r) { var v = mib(r.size); return { tag: r.tag, kind: r.kind, v: [v, v, v] }; });
    if (size.length) grid.appendChild(chart("binary size", "this machine's platform, stripped, MiB", size, "MiB"));
    [["go_tests", "go test", "every package, without the race detector, s"], ["battery", "black-box battery", "447+ cases, s"]].forEach(function (k) {
      var pts = bs.filter(function (r) { return r[k[0]]; }).map(function (r) { return { tag: r.tag, kind: r.kind, v: [r[k[0]], r[k[0]], r[k[0]]] }; });
      if (pts.length) grid.appendChild(chart(k[1], k[2], pts, "s"));
    });
    return el("div", {}, [el("h3", { text: "Every build, " + METRICS[state.metric][0].toLowerCase() }),
      el("p", { class: "perf-note", text: "Median line, min-max band. Hollow points were measured after the fact from the published binary (backfill). Hover for values." }),
      grid]);
  }

  function render() {
    root.textContent = "";
    [controls(), machineCard(), compareTable(), charts()].forEach(function (x) { root.appendChild(x); });
  }
  render();
})();
