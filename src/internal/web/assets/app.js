// probaci web UI: the whole of its own JavaScript. Every page works without
// it; this only adds conveniences on top. It never builds HTML from strings:
// every value goes in through textContent.
(function () {
  "use strict";

  var tb = window.tabler || {};

  // A form with data-confirm asks first, in Tabler's danger modal
  // (#confirm-modal in the layout): data-confirm is the body text,
  // data-confirm-title and data-confirm-button the heading and the button.
  // Without the modal script it falls back to the browser's own confirm.
  var modalEl = document.getElementById("confirm-modal");
  var pending = null;
  document.addEventListener("submit", function (ev) {
    var form = ev.target;
    var msg = form.getAttribute && form.getAttribute("data-confirm");
    if (!msg || form.getAttribute("data-confirmed") === "1") {
      return;
    }
    ev.preventDefault();
    if (!modalEl || !tb.Modal) {
      if (window.confirm(msg)) { form.setAttribute("data-confirmed", "1"); form.requestSubmit(ev.submitter || undefined); }
      return;
    }
    pending = { form: form, submitter: ev.submitter || null };
    document.getElementById("confirm-modal-title").textContent = form.getAttribute("data-confirm-title") || "Are you sure?";
    document.getElementById("confirm-modal-body").textContent = msg;
    document.getElementById("confirm-modal-ok").textContent = form.getAttribute("data-confirm-button") || "Confirm";
    tb.Modal.getOrCreateInstance(modalEl).show();
  });
  if (modalEl) {
    document.getElementById("confirm-modal-ok").addEventListener("click", function () {
      if (!pending) { return; }
      var p = pending;
      pending = null;
      tb.Modal.getOrCreateInstance(modalEl).hide();
      p.form.setAttribute("data-confirmed", "1");
      if (p.submitter) { p.form.requestSubmit(p.submitter); } else { p.form.requestSubmit(); }
    });
    modalEl.addEventListener("hidden.bs.modal", function () { pending = null; });
  }

  if (tb.Tooltip) {
    document.querySelectorAll("[data-bs-toggle=tooltip]").forEach(function (n) { tb.Tooltip.getOrCreateInstance(n); });
  }

  var log = document.getElementById("run-log");
  var follow = document.getElementById("log-follow");
  function scrollLog() {
    if (log && (!follow || follow.checked)) { log.scrollTop = log.scrollHeight; }
  }
  scrollLog();

  // A run in progress streams its output and stage results.
  var run = document.getElementById("run");
  if (!run || run.getAttribute("data-live") !== "1" || !window.EventSource) {
    return;
  }

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) { n.className = cls; }
    if (text !== undefined) { n.textContent = text; }
    return n;
  }
  function stageClass(status) {
    switch (status) {
      case "pass": return "badge badge-ok";
      case "skip": case "pending": return "badge badge-muted";
      case "running": return "badge badge-info";
      default: return "badge badge-bad";
    }
  }
  function clock(iso) {
    var d = new Date(iso);
    return isNaN(d) ? "" : d.toLocaleTimeString([], { hour12: false });
  }
  function base(p) {
    var parts = String(p || "").split(/[\\/]/);
    return parts[parts.length - 1];
  }
  function row(repo, stage) {
    var rows = run.querySelectorAll("tr[data-stage]");
    for (var i = 0; i < rows.length; i++) {
      if (rows[i].getAttribute("data-repo") === repo && rows[i].getAttribute("data-stage") === stage) { return rows[i]; }
    }
    return null;
  }

  var status = document.getElementById("run-status");
  var handlers = {
    "line": function (e) {
      var empty = log.querySelector("[data-role=empty]");
      if (empty) { empty.remove(); }
      var li = el("li", "log-line");
      var t = el("time", "", clock(e.time));
      t.setAttribute("datetime", e.time);
      li.appendChild(t);
      li.appendChild(document.createTextNode(" "));
      li.appendChild(el("span", "log-src", base(e.repo) + " " + e.stage));
      li.appendChild(document.createTextNode(" " + e.text));
      log.appendChild(li);
      scrollLog();
    },
    "stage": function (e) {
      var tr = row(e.repo, e.stage);
      if (!tr) { return; }
      var b = tr.querySelector("[data-role=status]");
      b.className = stageClass(e.status);
      b.textContent = e.status;
      tr.querySelector("[data-role=summary]").textContent = e.summary || "";
      if (e.status !== "pending" && e.status !== "running" && e.duration && e.duration !== "0s") {
        tr.querySelector("[data-role=duration]").textContent = e.duration;
      }
      if (e.status === "running") { status.textContent = "Running: " + base(e.repo) + " " + e.stage; }
    },
    "done": function (e) {
      status.textContent = "Run " + e.status + (e.error ? ": " + e.error : "") + ". Loading the final results.";
      source.close();
      window.setTimeout(function () { window.location.reload(); }, 1000);
    }
  };

  var url = run.getAttribute("data-events-url") + "?after=" + encodeURIComponent(run.getAttribute("data-last-id") || "0");
  var source = new EventSource(url);
  Object.keys(handlers).forEach(function (type) {
    source.addEventListener(type, function (msg) {
      var data;
      try { data = JSON.parse(msg.data); } catch (err) { return; }
      handlers[type](data);
    });
  });
})();
