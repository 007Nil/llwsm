(function () {
	"use strict";
	var cfg = window.SYSMON || { interval: 2 };
	var es = null;
	var pollTimer = null;

	function $(id) {
		return document.getElementById(id);
	}

	function fmtBytes(n) {
		if (n === null || n === undefined || isNaN(n)) {
			return "N/A";
		}
		var units = ["B", "KB", "MB", "GB", "TB"];
		var i = 0;
		n = Number(n);
		while (n >= 1024 && i < units.length - 1) {
			n /= 1024;
			i++;
		}
		return (i === 0 ? Math.round(n) : n.toFixed(1)) + " " + units[i];
	}

	function fmtUptime(sec) {
		if (sec === null || sec === undefined || isNaN(sec)) {
			return "N/A";
		}
		sec = Math.floor(Number(sec));
		var d = Math.floor(sec / 86400); sec -= d * 86400;
		var h = Math.floor(sec / 3600); sec -= h * 3600;
		var m = Math.floor(sec / 60);
		if (d > 0) {
			return d + "d " + h + "h " + m + "m";
		}
		if (h > 0) {
			return h + "h " + m + "m";
		}
		return m + "m " + (sec % 60) + "s";
	}

	function setBar(el, pct) {
		pct = Number(pct) || 0;
		if (pct < 0) {
			pct = 0;
		}
		if (pct > 100) {
			pct = 100;
		}
		el.style.width = pct + "%";
		el.className = "fill" + (pct >= 95 ? " crit" : pct >= 80 ? " warn" : "");
	}

	function render(d) {
		$("hostname").textContent = d.hostname || "unknown";
		var meta = [];
		if (d.model && d.model !== "N/A") {
			meta.push(d.model);
		}
		if (d.kernel) {
			meta.push(d.kernel);
		}
		if (d.arch) {
			meta.push(d.arch);
		}
		$("meta").textContent = meta.join(" \u2022 ");

		var st = $("status");
		st.className = "status " + String(d.status || "unknown").toLowerCase();
		$("status-text").textContent = d.status || "UNKNOWN";

		if (d.cpu) {
			$("cpu-usage").textContent = Math.round(d.cpu.usage) + "%";
			setBar($("cpu-bar"), d.cpu.usage);
			$("cpu-cores").textContent = d.cpu.cores + " cores";
			if (d.cpu.load) {
				$("cpu-load").textContent = "Load " +
					d.cpu.load.map(function (x) { return x.toFixed(2); }).join(" ");
			}
			var modelText = "";
			if (d.cpu.model && d.cpu.model !== "N/A") {
				modelText = d.cpu.model;
				if (d.cpu.frequency_mhz > 0) {
					modelText += " @ " + Math.round(d.cpu.frequency_mhz) + " MHz";
				}
			}
			$("cpu-model").textContent = modelText;
			var pc = $("cpu-percore");
			pc.innerHTML = "";
			if (d.cpu.per_core && d.cpu.per_core.length) {
				d.cpu.per_core.forEach(function (u, i) {
					var outer = document.createElement("div");
					outer.className = "corebar";
					outer.title = "core " + i + ": " + Math.round(u) + "%";
					var inner = document.createElement("div");
					setBar(inner, u);
					outer.appendChild(inner);
					pc.appendChild(outer);
				});
			}
		}

		if (d.memory) {
			var m = d.memory;
			$("mem-big").textContent = fmtBytes(m.used) + " / " + fmtBytes(m.total);
			setBar($("mem-bar"), m.percent);
			$("mem-percent").textContent = Math.round(m.percent) + "% used";
			$("mem-available").textContent = "available " + fmtBytes(m.available);
			if (m.swap_total > 0) {
				$("swap").textContent = "Swap " + fmtBytes(m.swap_used) + " / " +
					fmtBytes(m.swap_total) + " (" + Math.round(m.swap_percent) + "%)";
			} else {
				$("swap").textContent = "Swap N/A";
			}
		}

		$("uptime").textContent = "Uptime " + fmtUptime(d.uptime);

		renderStorage(d.storage);
		renderNetwork(d.network);
		renderSensors(d.sensors);
		renderContainers(d.containers);
		renderServices(d.services);
	}

	function fsRow(el, name, pct, detail, extraClass) {
		var row = document.createElement("div");
		row.className = "fsrow";
		var n = document.createElement("span");
		n.className = "fsname" + (extraClass ? " " + extraClass : "");
		n.textContent = name;
		var bar = document.createElement("div");
		bar.className = "bar";
		var fill = document.createElement("div");
		setBar(fill, pct);
		bar.appendChild(fill);
		var v = document.createElement("span");
		v.className = "fsval";
		v.textContent = detail;
		row.appendChild(n);
		row.appendChild(bar);
		row.appendChild(v);
		el.appendChild(row);
	}

	function fsRowSimple(el, name, detail, extraClass) {
		var row = document.createElement("div");
		row.className = "fsrow";
		var n = document.createElement("span");
		n.className = "fsname" + (extraClass ? " " + extraClass : "");
		n.textContent = name;
		var spacer = document.createElement("div");
		spacer.style.flex = "1";
		var v = document.createElement("span");
		v.className = "fsval";
		v.textContent = detail;
		row.appendChild(n);
		row.appendChild(spacer);
		row.appendChild(v);
		el.appendChild(row);
	}

	function renderStorage(list) {
		var el = $("storage");
		el.innerHTML = "";
		if (!list || !list.length) {
			el.innerHTML = '<div class="row small">N/A</div>';
			return;
		}
		list.forEach(function (fs) {
			fsRow(el, fs.mount, fs.percent,
				fmtBytes(fs.used) + " / " + fmtBytes(fs.total) +
				"  (" + Math.round(fs.percent) + "%)");
		});
	}

	function renderNetwork(list) {
		var el = $("network");
		el.innerHTML = "";
		if (!list || !list.length) {
			el.innerHTML = '<div class="row small">N/A</div>';
			return;
		}
		list.forEach(function (ni) {
			var label = ni.name;
			if (ni.state === "down") {
				label += "  (down)";
			}
			var ips = (ni.ipv4 || []).concat(ni.ipv6 || []).join(", ");
			fsRow(el, label, ni.state === "down" ? 0 : 100,
				ips + "    \u2193 " + fmtBytes(ni.rx_rate) + "/s   \u2191 " +
				fmtBytes(ni.tx_rate) + "/s",
				ni.state === "down" ? "off" : "");
		});
	}

	function renderSensors(list) {
		var el = $("sensors");
		el.innerHTML = "";
		if (!list || !list.length) {
			el.innerHTML = '<div class="row small">N/A</div>';
			return;
		}
		list.forEach(function (s) {
			fsRowSimple(el, s.name, Math.round(s.temperature) + " \u00B0C");
		});
	}

	function renderContainers(c) {
		var el = $("containers");
		el.innerHTML = "";
		if (!c || !c.available) {
			el.innerHTML = '<div class="row small">Docker not available</div>';
			return;
		}
		if (!c.containers || !c.containers.length) {
			el.innerHTML = '<div class="row small">No containers</div>';
			return;
		}
		c.containers.forEach(function (ctr) {
			var running = ctr.state === "running";
			var bits = [running ? "Running" : (ctr.status || "Stopped")];
			if (running) {
				bits.push("CPU " + Math.round(ctr.cpu_percent * 10) / 10 + "%");
				bits.push("RAM " + fmtBytes(ctr.memory_bytes));
			}
			fsRowSimple(el, ctr.name, bits.join("  "), running ? "" : "off");
		});
	}

	function renderServices(list) {
		var el = $("services");
		el.innerHTML = "";
		if (!list || !list.length) {
			el.innerHTML = '<div class="row small">no services configured</div>';
			return;
		}
		list.forEach(function (s) {
			var row = document.createElement("div");
			row.className = "svc";
			var dot = document.createElement("span");
			dot.className = "svc-dot " + s.status;
			var name = document.createElement("span");
			name.textContent = s.name;
			var det = document.createElement("span");
			det.className = "svc-detail";
			det.textContent = s.status === "up" ? "Running" : (s.detail || "down");
			row.appendChild(dot);
			row.appendChild(name);
			row.appendChild(det);
			el.appendChild(row);
		});
	}

	function fetchStatus() {
		return fetch("/api/status", { cache: "no-store" })
			.then(function (r) {
				if (!r.ok) {
					throw new Error("http " + r.status);
				}
				return r.json();
			})
			.then(render)
			.catch(function () {
				$("status").className = "status offline";
				$("status-text").textContent = "OFFLINE";
			});
	}

	function startPolling() {
		if (pollTimer) {
			return;
		}
		var ms = Math.max(1000, (cfg.interval || 2) * 1000);
		fetchStatus();
		pollTimer = setInterval(fetchStatus, ms);
	}

	function connectStream() {
		if (typeof EventSource === "undefined") {
			startPolling();
			return;
		}
		es = new EventSource("/api/stream");
		es.onmessage = function (e) {
			try {
				render(JSON.parse(e.data));
			} catch (err) {
			}
		};
		es.onerror = function () {
			es.close();
			es = null;
			startPolling();
		};
	}

	connectStream();
})();
