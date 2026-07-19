// portal.js drives the small amount of client-side behavior the
// diagoram HTML portal needs: rendering embedded Mermaid sources with
// mermaid.min.js, rendering report.md with marked.min.js, adding
// "Copy" buttons to source <pre> blocks, and adding zoom/pan controls
// to successfully-rendered Mermaid diagrams. It is loaded from an
// external <script src> on every page that needs it so no page relies
// on inline script, keeping a strict `script-src 'self'` Content-
// Security-Policy workable.
//
// diagoram analyzes third-party Go source, so every string this file
// touches (type names, doc comments, warning text) may be adversarial.
// Two things keep that safe:
//   - Mermaid sources are embedded as the text content of a
//     `<pre class="mermaid">` element, never as innerHTML, and
//     securityLevel is 'strict'.
//   - report.md is rendered through marked.js with its `html` token
//     renderer overridden to escape rather than pass through raw
//     HTML, so a doc comment containing literal HTML/script tags
//     cannot execute.
(function () {
  "use strict";

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
  }

  // copyText copies text to the clipboard, returning a Promise that
  // resolves on success. It prefers the async Clipboard API and falls
  // back to a hidden, off-screen textarea + document.execCommand("copy")
  // when that API is unavailable or rejects (e.g. an insecure or
  // file:// origin in some browsers).
  function copyText(text) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      return navigator.clipboard.writeText(text).catch(function () {
        return copyTextFallback(text);
      });
    }
    return copyTextFallback(text);
  }

  function copyTextFallback(text) {
    return new Promise(function (resolve, reject) {
      var ta = document.createElement("textarea");
      ta.value = text;
      // Keep it out of the visible layout and out of tab order while
      // still selectable by execCommand("copy").
      ta.style.position = "fixed";
      ta.style.top = "-1000px";
      ta.style.left = "-1000px";
      ta.setAttribute("readonly", "");
      document.body.appendChild(ta);
      ta.select();
      ta.setSelectionRange(0, ta.value.length);
      var ok = false;
      try {
        ok = document.execCommand("copy");
      } catch (e) {
        ok = false;
      }
      document.body.removeChild(ta);
      if (ok) {
        resolve();
      } else {
        reject(new Error("copy failed"));
      }
    });
  }

  // addCopyButton wraps `pre` in a `.copy-wrap` container and adds a
  // "Copy" button next to it (never inside it, so the button's own
  // label text is never part of pre.textContent). It is idempotent:
  // calling it again on an already-wrapped `pre` is a no-op, so
  // repeated initCopyButtons() passes (e.g. after the Mermaid fallback
  // converts a block to pre.source) never double-wrap.
  function addCopyButton(pre) {
    var parent = pre.parentElement;
    if (!parent || parent.classList.contains("copy-wrap")) {
      return;
    }
    var wrap = document.createElement("div");
    wrap.className = "copy-wrap";
    parent.insertBefore(wrap, pre);
    wrap.appendChild(pre);

    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "copy-btn";
    btn.textContent = "Copy";
    btn.addEventListener("click", function () {
      copyText(pre.textContent).then(
        function () {
          btn.textContent = "Copied!";
          btn.classList.add("copied");
          setTimeout(function () {
            btn.textContent = "Copy";
            btn.classList.remove("copied");
          }, 1500);
        },
        function () {
          btn.textContent = "Copy failed";
          setTimeout(function () {
            btn.textContent = "Copy";
          }, 1500);
        }
      );
    });
    wrap.appendChild(btn);
  }

  // initCopyButtons adds copy buttons to every source `<pre>` under
  // `root` (document by default): standalone `pre.source` blocks
  // (PlantUML/text/summary pages, and Mermaid blocks that fell back to
  // source view) and non-mermaid `pre > code` blocks rendered inside a
  // report. It deliberately excludes `pre.mermaid`, which holds live
  // or in-flight Mermaid source and must not get a copy button while
  // it does - see the fallback() call in renderMermaidBlocks for how
  // those blocks get buttons once (if) they convert to pre.source.
  function initCopyButtons(root) {
    (root || document).querySelectorAll("pre.source, .report pre > code").forEach(function (el) {
      var pre = el.tagName === "CODE" ? el.parentElement : el;
      if (pre) {
        addCopyButton(pre);
      }
    });
  }

  // Zoom/pan tuning: ZOOM_STEP is the per-click scale multiplier
  // (also used per Ctrl/Cmd+wheel tick), and ZOOM_MIN/ZOOM_MAX bound
  // how far a diagram can be shrunk or enlarged.
  var ZOOM_STEP = 1.25;
  var ZOOM_MIN = 0.2;
  var ZOOM_MAX = 10;

  function clampZoom(scale) {
    return Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, scale));
  }

  // initZoomPan wraps a successfully-rendered `pre.mermaid` (one that
  // now contains an <svg>, not a fallback source block) in a
  // `.zoom-wrap` container with +/-/Reset buttons overlaid, and wires
  // up wheel-zoom (only while Ctrl/Cmd is held, so a plain wheel keeps
  // scrolling the page), drag-to-pan, and button clicks. Zoom/pan
  // state is applied as a CSS transform on the <svg> element itself
  // (translate then scale, transform-origin 0 0), never on the pre or
  // its wrapper, so nothing here touches pre.mermaid's text content.
  // It is idempotent: calling it again on an already-wrapped pre is a
  // no-op.
  function initZoomPan(pre) {
    var svg = pre.querySelector("svg");
    if (!svg) {
      return;
    }
    var parent = pre.parentElement;
    if (!parent || parent.classList.contains("zoom-wrap")) {
      return;
    }

    var wrap = document.createElement("div");
    wrap.className = "zoom-wrap";
    wrap.title = "Ctrl/Cmd+scroll to zoom, drag to pan";
    parent.insertBefore(wrap, pre);
    wrap.appendChild(pre);

    svg.style.transformOrigin = "0 0";
    svg.style.maxWidth = "none";

    var state = { scale: 1, x: 0, y: 0 };

    function apply() {
      svg.style.transform = "translate(" + state.x + "px, " + state.y + "px) scale(" + state.scale + ")";
    }

    function reset() {
      state.scale = 1;
      state.x = 0;
      state.y = 0;
      apply();
    }

    // zoomAt rescales around (clientX, clientY) - a viewport point, e.g.
    // the mouse cursor - so that whatever diagram point sits under it
    // stays under it after the scale changes.
    function zoomAt(factor, clientX, clientY) {
      var newScale = clampZoom(state.scale * factor);
      if (newScale === state.scale) {
        return;
      }
      var rect = wrap.getBoundingClientRect();
      var originX = clientX - rect.left;
      var originY = clientY - rect.top;
      state.x = originX - ((originX - state.x) / state.scale) * newScale;
      state.y = originY - ((originY - state.y) / state.scale) * newScale;
      state.scale = newScale;
      apply();
    }

    function zoomAtCenter(factor) {
      var rect = wrap.getBoundingClientRect();
      zoomAt(factor, rect.left + rect.width / 2, rect.top + rect.height / 2);
    }

    var controls = document.createElement("div");
    controls.className = "zoom-controls";

    var btnIn = document.createElement("button");
    btnIn.type = "button";
    btnIn.className = "zoom-btn";
    btnIn.textContent = "+";
    btnIn.title = "Zoom in";
    btnIn.addEventListener("click", function () {
      zoomAtCenter(ZOOM_STEP);
    });

    var btnOut = document.createElement("button");
    btnOut.type = "button";
    btnOut.className = "zoom-btn";
    btnOut.textContent = "−";
    btnOut.title = "Zoom out";
    btnOut.addEventListener("click", function () {
      zoomAtCenter(1 / ZOOM_STEP);
    });

    var btnReset = document.createElement("button");
    btnReset.type = "button";
    btnReset.className = "zoom-btn";
    btnReset.textContent = "Reset";
    btnReset.title = "Reset zoom and pan";
    btnReset.addEventListener("click", reset);

    controls.appendChild(btnIn);
    controls.appendChild(btnOut);
    controls.appendChild(btnReset);
    wrap.appendChild(controls);

    // Only zoom on Ctrl/Cmd+wheel; a plain wheel is left alone so the
    // page keeps scrolling normally over the diagram.
    wrap.addEventListener(
      "wheel",
      function (e) {
        if (!(e.ctrlKey || e.metaKey)) {
          return;
        }
        e.preventDefault();
        zoomAt(e.deltaY < 0 ? ZOOM_STEP : 1 / ZOOM_STEP, e.clientX, e.clientY);
      },
      { passive: false }
    );

    var dragging = false;
    var lastX = 0;
    var lastY = 0;

    function stopDrag() {
      if (!dragging) {
        return;
      }
      dragging = false;
      wrap.classList.remove("dragging");
    }

    wrap.addEventListener("mousedown", function (e) {
      if (e.button !== 0 || e.target.closest(".zoom-controls")) {
        return;
      }
      dragging = true;
      lastX = e.clientX;
      lastY = e.clientY;
      wrap.classList.add("dragging");
      e.preventDefault();
    });
    document.addEventListener("mousemove", function (e) {
      if (!dragging) {
        return;
      }
      state.x += e.clientX - lastX;
      state.y += e.clientY - lastY;
      lastX = e.clientX;
      lastY = e.clientY;
      apply();
    });
    document.addEventListener("mouseup", stopDrag);
    window.addEventListener("blur", stopDrag);

    apply();
  }

  // initZoomPanForBlocks calls initZoomPan on every block that
  // rendered successfully (still `pre.mermaid`, now containing an
  // <svg>). Blocks the fallback() path converted to `pre.source` are
  // skipped automatically since they no longer hold an <svg>.
  function initZoomPanForBlocks(blocks) {
    blocks.forEach(function (el) {
      if (el.classList.contains("mermaid")) {
        initZoomPan(el);
      }
    });
  }

  // renderMermaidBlocks renders every current `pre.mermaid` element.
  // Each block's raw source is captured before mermaid.run() runs, so
  // a rendering failure (mermaid.parseError, or run() rejecting) can
  // fall back to showing that text instead of leaving a blank or
  // half-rendered element, regardless of what mermaid.js did to the
  // DOM before failing.
  function renderMermaidBlocks() {
    if (typeof mermaid === "undefined") {
      return;
    }
    var blocks = document.querySelectorAll("pre.mermaid");
    if (blocks.length === 0) {
      return;
    }

    var raw = new Map();
    blocks.forEach(function (el) {
      raw.set(el, el.textContent);
    });

    var fell = false;
    function fallback() {
      if (fell) {
        return;
      }
      fell = true;
      blocks.forEach(function (el) {
        if (!raw.has(el)) {
          return;
        }
        el.classList.remove("mermaid");
        el.classList.add("source");
        el.textContent = raw.get(el);
      });
      document.querySelectorAll(".mermaid-fallback-notice").forEach(function (n) {
        n.hidden = false;
      });
      // The blocks above just became pre.source; give them copy
      // buttons too.
      initCopyButtons();
    }

    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      maxTextSize: 900000,
    });
    // Two-tier fallback: mermaid.parseError catches syntax errors
    // surfaced during parsing, and the run() rejection catches
    // anything else (e.g. renderer-level failures).
    mermaid.parseError = fallback;
    try {
      mermaid
        .run({ nodes: Array.prototype.slice.call(blocks) })
        .then(function () {
          initZoomPanForBlocks(blocks);
        })
        .catch(fallback);
    } catch (e) {
      fallback();
    }
  }

  // renderReport turns the raw Markdown text embedded in
  // #report-markdown into HTML inside #report-content, then hands any
  // ```mermaid fenced block marked.js produced off to
  // renderMermaidBlocks so it renders the same way a standalone
  // diagram page does.
  function renderReport() {
    var src = document.getElementById("report-markdown");
    var out = document.getElementById("report-content");
    if (!src || !out || typeof marked === "undefined") {
      return;
    }

    var renderer = new marked.Renderer();
    renderer.html = function (token) {
      var text = token && typeof token === "object" ? (token.text != null ? token.text : token.raw) : token;
      return escapeHtml(text == null ? "" : text);
    };
    marked.use({ renderer: renderer });

    out.innerHTML = marked.parse(src.textContent);

    out.querySelectorAll("pre > code.language-mermaid").forEach(function (code) {
      var pre = code.parentElement;
      var mermaidPre = document.createElement("pre");
      mermaidPre.className = "mermaid";
      mermaidPre.textContent = code.textContent;
      pre.replaceWith(mermaidPre);
    });

    initCopyButtons(out);
    renderMermaidBlocks();
  }

  document.addEventListener("DOMContentLoaded", function () {
    var page = document.body.getAttribute("data-page");
    if (page === "mermaid") {
      renderMermaidBlocks();
    } else if (page === "report") {
      renderReport();
    }
    initCopyButtons();
  });
})();
