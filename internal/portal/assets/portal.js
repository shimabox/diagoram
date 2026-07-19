// portal.js drives the small amount of client-side behavior the
// diagoram HTML portal needs: rendering embedded Mermaid sources with
// mermaid.min.js, and rendering report.md with marked.min.js. It is
// loaded from an external <script src> on every page that needs it so
// no page relies on inline script, keeping a strict
// `script-src 'self'` Content-Security-Policy workable.
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
      mermaid.run({ nodes: Array.prototype.slice.call(blocks) }).catch(fallback);
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

    renderMermaidBlocks();
  }

  document.addEventListener("DOMContentLoaded", function () {
    var page = document.body.getAttribute("data-page");
    if (page === "mermaid") {
      renderMermaidBlocks();
    } else if (page === "report") {
      renderReport();
    }
  });
})();
