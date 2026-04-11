(function (global) {
    "use strict";

    var echartsLoading = null;

    function isAgentRenderEnvelope(spec) {
        return !!spec &&
            typeof spec === "object" &&
            spec.__agentRender === true &&
            spec.schema === "agent-render/v1" &&
            (spec.renderer === "viz.tui" || spec.renderer === "advn.tui") &&
            (spec.mode === "blocks" || spec.mode === "lines");
    }

    function rowsToText(rows) {
        if (!Array.isArray(rows) || rows.length === 0) return "";
        var lines = [];
        for (var i = 0; i < rows.length; i++) {
            var row = rows[i];
            if (Array.isArray(row)) {
                lines.push(row.map(function (x) { return String(x); }).join(" | "));
            } else {
                lines.push(String(row));
            }
        }
        return lines.join("\n");
    }

    function appendPre(target, label, text) {
        var pre = document.createElement("pre");
        pre.textContent = (label ? label + "\n" : "") + String(text || "");
        target.appendChild(pre);
    }

    function renderVizFallbackNotice(container, spec) {
        var box = document.createElement("section");
        box.className = "viz-host";

        var label = document.createElement("div");
        label.className = "label";
        label.textContent = "vizspec(unavailable)";
        box.appendChild(label);

        var body = document.createElement("div");
        body.className = "canvas-wrap";
        body.style.padding = "10px";
        body.style.color = "#3a4c40";

        var title = "";
        try {
            title = String((((spec || {}).meta || {}).title) || "");
        } catch (e) {
            title = "";
        }

        var lines = [];
        lines.push("Visualization data was received, but the client could not normalize this spec.");
        lines.push("Please try changing Vizspec Normalize mode or Viz Format.");
        if (title) {
            lines.push("title: " + title);
        }
        body.textContent = lines.join("\n");

        box.appendChild(body);
        container.appendChild(box);
    }

    function renderVizspecEnvelope(container, spec) {
        if (spec.mode === "lines") {
            var lines = Array.isArray(spec.lines) ? spec.lines.map(String).join("\n") : "";
            appendPre(container, "vizspec.lines", lines || "(empty)");
            return;
        }

        var blocks = Array.isArray(spec.blocks) ? spec.blocks : [];
        if (blocks.length === 0) {
            appendPre(container, "vizspec.blocks", "(empty)");
            return;
        }

        for (var i = 0; i < blocks.length; i++) {
            var b = blocks[i] || {};
            var out = [];
            if (b.title) out.push(String(b.title));

            var stats = Array.isArray(b.stats) ? b.stats : [];
            for (var j = 0; j < stats.length; j++) {
                var one = stats[j] || {};
                out.push(String(one.label || "") + ": " + String(one.value || ""));
            }

            var lineList = Array.isArray(b.lines) ? b.lines : [];
            for (var k = 0; k < lineList.length; k++) {
                out.push(String(lineList[k]));
            }

            var rowText = rowsToText(b.rows);
            if (rowText) out.push(rowText);

            appendPre(container, "vizspec.block[" + i + "]", out.join("\n"));
        }
    }

    function clonePlainObject(obj) {
        try {
            return JSON.parse(JSON.stringify(obj || {}));
        } catch (e) {
            return obj && typeof obj === "object" ? obj : {};
        }
    }

    function normalizeVizspecSpec(spec) {
        if (!spec || typeof spec !== "object") return null;
        if (spec.schema !== "vizspec/v1" && spec.schema !== "advn/v1") return null;

        var x = [];
        var series = [];
        var data = spec.data || {};

        if (Array.isArray(data.x) && Array.isArray(data.series)) {
            x = data.x.slice();
            for (var i = 0; i < data.series.length; i++) {
                var s = data.series[i] || {};
                if (!Array.isArray(s.data)) continue;
                series.push({ name: String(s.name || ("series-" + (i + 1))), data: s.data.slice() });
            }
        } else if (Array.isArray(spec.x) && Array.isArray(spec.series)) {
            x = spec.x.slice();
            for (var j = 0; j < spec.series.length; j++) {
                var ss = spec.series[j] || {};
                if (!Array.isArray(ss.data)) continue;
                series.push({ name: String(ss.name || ("series-" + (j + 1))), data: ss.data.slice() });
            }
        } else if (Array.isArray(data.rows) && data.xKey) {
            var yKeys = Array.isArray(data.yKeys) ? data.yKeys.slice() : [];
            if (yKeys.length === 0 && data.rows.length > 0) {
                var first = data.rows[0] || {};
                var keys = Object.keys(first);
                for (var k = 0; k < keys.length; k++) {
                    if (keys[k] !== data.xKey) yKeys.push(keys[k]);
                }
            }
            for (var yi = 0; yi < yKeys.length; yi++) {
                series.push({ name: String(yKeys[yi]), data: [] });
            }
            for (var r = 0; r < data.rows.length; r++) {
                var row = data.rows[r] || {};
                x.push(row[data.xKey]);
                for (var y = 0; y < series.length; y++) {
                    series[y].data.push(row[series[y].name]);
                }
            }
        }

        if (x.length === 0 || series.length === 0) return null;
        return { x: x, series: series, title: String((spec.meta && spec.meta.title) || spec.title || "") };
    }

    function resolveVizFormat(spec, context) {
        var sel = String((context && context.vizFormat) || "auto").toLowerCase();
        if (sel === "echarts" || sel === "svg" || sel === "png") return sel;

        var preferred = (((spec || {}).meta || {}).preferred) || [];
        for (var i = 0; i < preferred.length; i++) {
            var p = String(preferred[i] || "").toLowerCase();
            if (p === "echarts" || p === "svg" || p === "png") return p;
        }
        return "svg";
    }

    function createVizHost(target, label) {
        var host = document.createElement("section");
        host.className = "viz-host";

        var tag = document.createElement("div");
        tag.className = "label";
        tag.textContent = label;
        host.appendChild(tag);

        var wrap = document.createElement("div");
        wrap.className = "canvas-wrap";
        host.appendChild(wrap);
        target.appendChild(host);
        return wrap;
    }

    function getSeriesRange(vizspec) {
        var min = Number.POSITIVE_INFINITY;
        var max = Number.NEGATIVE_INFINITY;
        for (var i = 0; i < vizspec.series.length; i++) {
            var values = vizspec.series[i].data || [];
            for (var j = 0; j < values.length; j++) {
                var n = Number(values[j]);
                if (!isFinite(n)) continue;
                if (n < min) min = n;
                if (n > max) max = n;
            }
        }
        if (!isFinite(min) || !isFinite(max)) return { min: 0, max: 1 };
        if (min === max) return { min: min - 1, max: max + 1 };
        return { min: min, max: max };
    }

    function colorAt(i) {
        var palette = ["#2f7cff", "#1f8f63", "#cc6a2d", "#8a43c7", "#e43f5a", "#1584a3"];
        return palette[i % palette.length];
    }

    function renderVizspecSvg(canvasWrap, vizspec) {
        var width = Math.max(640, Math.min(980, canvasWrap.clientWidth || 760));
        var height = 280;
        var padL = 48;
        var padR = 16;
        var padT = 20;
        var padB = 34;
        var innerW = width - padL - padR;
        var innerH = height - padT - padB;
        var count = vizspec.x.length;
        var stepX = count > 1 ? innerW / (count - 1) : innerW;
        var range = getSeriesRange(vizspec);

        var svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
        svg.setAttribute("viewBox", "0 0 " + width + " " + height);
        svg.setAttribute("width", String(width));
        svg.setAttribute("height", String(height));

        var bg = document.createElementNS("http://www.w3.org/2000/svg", "rect");
        bg.setAttribute("x", "0");
        bg.setAttribute("y", "0");
        bg.setAttribute("width", String(width));
        bg.setAttribute("height", String(height));
        bg.setAttribute("fill", "#ffffff");
        svg.appendChild(bg);

        var axis = document.createElementNS("http://www.w3.org/2000/svg", "path");
        axis.setAttribute("d", "M " + padL + " " + (padT + innerH) + " L " + (padL + innerW) + " " + (padT + innerH) + " M " + padL + " " + padT + " L " + padL + " " + (padT + innerH));
        axis.setAttribute("stroke", "#8fa08f");
        axis.setAttribute("stroke-width", "1");
        axis.setAttribute("fill", "none");
        svg.appendChild(axis);

        for (var i = 0; i < vizspec.series.length; i++) {
            var s = vizspec.series[i];
            var d = [];
            for (var j = 0; j < count; j++) {
                var n = Number(s.data[j]);
                if (!isFinite(n)) continue;
                var x = padL + stepX * j;
                var y = padT + innerH * (1 - ((n - range.min) / (range.max - range.min)));
                d.push((d.length === 0 ? "M" : "L") + " " + x + " " + y);
            }
            if (d.length === 0) continue;
            var line = document.createElementNS("http://www.w3.org/2000/svg", "path");
            line.setAttribute("d", d.join(" "));
            line.setAttribute("stroke", colorAt(i));
            line.setAttribute("stroke-width", "2");
            line.setAttribute("fill", "none");
            svg.appendChild(line);
        }

        canvasWrap.innerHTML = "";
        canvasWrap.appendChild(svg);
    }

    function drawVizspecCanvas(canvas, vizspec) {
        var ctx = canvas.getContext("2d");
        var width = canvas.width;
        var height = canvas.height;
        var padL = 48;
        var padR = 16;
        var padT = 20;
        var padB = 34;
        var innerW = width - padL - padR;
        var innerH = height - padT - padB;
        var count = vizspec.x.length;
        var stepX = count > 1 ? innerW / (count - 1) : innerW;
        var range = getSeriesRange(vizspec);

        ctx.clearRect(0, 0, width, height);
        ctx.fillStyle = "#ffffff";
        ctx.fillRect(0, 0, width, height);

        ctx.strokeStyle = "#8fa08f";
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.moveTo(padL, padT + innerH);
        ctx.lineTo(padL + innerW, padT + innerH);
        ctx.moveTo(padL, padT);
        ctx.lineTo(padL, padT + innerH);
        ctx.stroke();

        for (var i = 0; i < vizspec.series.length; i++) {
            var s = vizspec.series[i];
            ctx.strokeStyle = colorAt(i);
            ctx.lineWidth = 2;
            ctx.beginPath();
            var started = false;
            for (var j = 0; j < count; j++) {
                var n = Number(s.data[j]);
                if (!isFinite(n)) continue;
                var x = padL + stepX * j;
                var y = padT + innerH * (1 - ((n - range.min) / (range.max - range.min)));
                if (!started) {
                    ctx.moveTo(x, y);
                    started = true;
                } else {
                    ctx.lineTo(x, y);
                }
            }
            if (started) ctx.stroke();
        }
    }

    function renderVizspecPng(canvasWrap, vizspec) {
        var width = Math.max(640, Math.min(980, canvasWrap.clientWidth || 760));
        var height = 280;
        var canvas = document.createElement("canvas");
        canvas.width = width;
        canvas.height = height;
        drawVizspecCanvas(canvas, vizspec);
        var img = document.createElement("img");
        img.alt = "vizspec-png";
        img.src = canvas.toDataURL("image/png");
        img.style.maxWidth = "100%";
        canvasWrap.innerHTML = "";
        canvasWrap.appendChild(img);
    }

    function ensureEcharts() {
        if (global.echarts) return Promise.resolve(global.echarts);
        if (echartsLoading) return echartsLoading;
        echartsLoading = new Promise(function (resolve, reject) {
            var script = document.createElement("script");
            script.src = "https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js";
            script.onload = function () { resolve(global.echarts); };
            script.onerror = function () { reject(new Error("failed to load echarts")); };
            document.head.appendChild(script);
        }).finally(function () {
            if (!global.echarts) echartsLoading = null;
        });
        return echartsLoading;
    }

    function renderVizspecEcharts(canvasWrap, vizspec) {
        return ensureEcharts().then(function (echarts) {
            var dom = document.createElement("div");
            dom.style.width = "100%";
            dom.style.height = "300px";
            canvasWrap.innerHTML = "";
            canvasWrap.appendChild(dom);
            var chart = echarts.init(dom, null, { renderer: "canvas" });
            var series = [];
            for (var i = 0; i < vizspec.series.length; i++) {
                series.push({
                    name: vizspec.series[i].name,
                    type: "line",
                    showSymbol: false,
                    data: vizspec.series[i].data
                });
            }
            chart.setOption({
                animation: false,
                tooltip: { trigger: "axis" },
                legend: { top: 4 },
                grid: { left: 42, right: 20, top: 32, bottom: 28 },
                xAxis: { type: "category", data: vizspec.x },
                yAxis: { type: "value" },
                series: series
            });
        });
    }

    function renderVizspecSpec(container, spec, vizspec, context) {
        var format = resolveVizFormat(spec, context);
        var title = "vizspec(" + format + ")";
        var wrap = createVizHost(container, title + (vizspec.title ? " - " + vizspec.title : ""));

        if (format === "png") {
            renderVizspecPng(wrap, vizspec);
            return;
        }
        if (format === "echarts") {
            renderVizspecEcharts(wrap, vizspec).catch(function (err) {
                if (context && typeof context.log === "function") {
                    context.log("echarts render failed, fallback to svg", err && err.message ? err.message : err);
                }
                renderVizspecSvg(wrap, vizspec);
            });
            return;
        }
        renderVizspecSvg(wrap, vizspec);
    }

    function buildVizspecRenderRequest(spec, context) {
        var req = clonePlainObject(spec);
        if (!req || typeof req !== "object") {
            req = {};
        }

        var fmt = resolveVizFormat(req, context);
        var preferred = [];
        if (fmt === "echarts" || fmt === "svg" || fmt === "png") {
            preferred.push(fmt);
        }

        if (!req.meta || typeof req.meta !== "object") {
            req.meta = {};
        }
        var existing = Array.isArray(req.meta.preferred) ? req.meta.preferred : [];
        for (var i = 0; i < existing.length; i++) {
            var one = String(existing[i] || "").toLowerCase();
            if (one === "echarts" || one === "svg" || one === "png") {
                if (preferred.indexOf(one) < 0) preferred.push(one);
            }
        }
        if (preferred.length > 0) {
            req.meta.preferred = preferred;
        }

        req.clientHint = {
            renderer: "vizspec",
            preferred: preferred,
            viewport: {
                width: Math.max(0, global.innerWidth || 0),
                height: Math.max(0, global.innerHeight || 0)
            },
            theme: "light"
        };
        return req;
    }

    function normalizeVizspecByRpc(spec, context) {
        if (!context || typeof context.callRpcRaw !== "function") {
            return Promise.resolve(spec);
        }
        return context.callRpcRaw("vizspec.render", [spec]).then(function (normalized) {
            if (!normalized || typeof normalized !== "object") {
                return spec;
            }
            return normalized;
        });
    }

    function renderVizspecResolved(container, spec, context) {
        var vizspec = normalizeVizspecSpec(spec);
        if (!vizspec) {
            renderVizFallbackNotice(container, spec);
            if (context && typeof context.log === "function") {
                context.log("vizspec normalize failed", {
                    schema: spec && spec.schema ? spec.schema : "",
                    hasData: !!(spec && spec.data)
                });
            }
            return false;
        }
        renderVizspecSpec(container, spec, vizspec, context);
        return true;
    }

    function renderVizspecWithRpc(container, spec, context) {
        var pending = document.createElement("div");
        pending.className = "md-block";

        var pendingLabel = document.createElement("div");
        pendingLabel.className = "label";
        pendingLabel.textContent = "vizspec(normalize:rpc)";

        var pendingBody = document.createElement("div");
        pendingBody.className = "content";
        pendingBody.textContent = "(normalizing with vizspec.render...)";

        pending.appendChild(pendingLabel);
        pending.appendChild(pendingBody);
        container.appendChild(pending);

        var reqSpec = buildVizspecRenderRequest(spec, context);
        normalizeVizspecByRpc(reqSpec, context)
            .then(function (normalized) {
                if (pending.parentNode) {
                    pending.parentNode.removeChild(pending);
                }
                renderVizspecResolved(container, normalized, context);
            })
            .catch(function (err) {
                if (pending.parentNode) {
                    pending.parentNode.removeChild(pending);
                }
                if (context && typeof context.log === "function") {
                    context.log("vizspec.render failed; fallback to client normalize", err && err.message ? err.message : err);
                }
                renderVizspecResolved(container, spec, context);
            });
    }

    function render(container, spec, context) {
        var ctx = context || {};

        if (isAgentRenderEnvelope(spec)) {
            renderVizspecEnvelope(container, spec);
            return;
        }

        var normalizeMode = String(ctx.vizspecNormalize || "rpc").toLowerCase();
        if (normalizeMode === "rpc" && ctx.wsConnected && typeof ctx.callRpcRaw === "function") {
            renderVizspecWithRpc(container, spec, ctx);
            return;
        }
        renderVizspecResolved(container, spec, ctx);
    }

    global.RpcLlmDemoViz = {
        render: render,
        normalizeVizspecSpec: normalizeVizspecSpec
    };
})(window);
