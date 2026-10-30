package main

import (
	"encoding/json"
	"fmt"
)

// preloadTemplate is the script the app injects into every page of a chat
// before the page's own scripts. It applies the chat's CSS at once, runs
// its JS once the page is ready, and keeps the configuration in the page's
// localStorage, so that a change reaches the live page at once and the
// next load of the page too, without building the web view again.
//
// The arguments are, in order, the chat's ID, its CSS and its JS, as
// JavaScript string literals.
const preloadTemplate = `(function () {
  "use strict";
  if (window.__mychatbox) { return; }
  window.__mychatbox = true;
  var KEY = "mychatbox:inject:" + %s;
  var BAKED = { css: %s, js: %s };
  var ranJS = false;

  function config() {
    try {
      var raw = window.localStorage.getItem(KEY);
      if (raw) {
        var parsed = JSON.parse(raw);
        if (parsed && typeof parsed === "object") {
          return { css: String(parsed.css || ""), js: String(parsed.js || "") };
        }
      }
    } catch (e) {}
    return BAKED;
  }

  function styleElement() {
    var root = document.head || document.documentElement;
    if (!root) { return null; }
    var el = document.getElementById("mychatbox-style");
    if (!el) {
      el = document.createElement("style");
      el.id = "mychatbox-style";
      root.appendChild(el);
    }
    return el;
  }

  function applyCSS(css) {
    var el = styleElement();
    if (!el) {
      document.addEventListener("readystatechange", function () { applyCSS(css); });
      return;
    }
    if (el.textContent !== css) { el.textContent = css; }
  }

  function runJS(js) {
    if (!js) { return; }
    try {
      (0, eval)(js);
    } catch (e) {
      console.error("[chatBox] 注入脚本出错：", e);
    }
  }

  function ready(runScript) {
    var cfg = config();
    applyCSS(cfg.css);
    if (runScript && !ranJS) {
      ranJS = true;
      runJS(cfg.js);
    }
  }

  // The app applies a change of the chat's CSS or JS through this, on the
  // live page.
  window.__mychatboxApply = function (cfg, runScript) {
    cfg = cfg || { css: "", js: "" };
    try {
      window.localStorage.setItem(KEY, JSON.stringify({ css: cfg.css || "", js: cfg.js || "" }));
    } catch (e) {}
    applyCSS(cfg.css || "");
    if (runScript) { runJS(cfg.js || ""); }
  };

  ready(false);
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", function () { ready(true); }, { once: true });
    window.addEventListener("load", function () { ready(true); }, { once: true });
  } else {
    ready(true);
  }
})();
`

// preloadScript returns the script a chat's web view runs in its pages.
// Every chat gets it, empty configuration included, so that CSS and JS the
// user adds later reach the page already open.
func preloadScript(ch *Chat) string {
	return fmt.Sprintf(preloadTemplate, jsLiteral(ch.ID), jsLiteral(ch.CSS), jsLiteral(ch.JS))
}

// applyScript returns the JavaScript that applies a chat's CSS, and runs
// its JS, on its page as it is now.
func applyScript(ch *Chat, runJS bool) string {
	cfg, _ := json.Marshal(struct {
		CSS string `json:"css"`
		JS  string `json:"js"`
	}{ch.CSS, ch.JS})
	run := "false"
	if runJS {
		run = "true"
	}
	return fmt.Sprintf("window.__mychatboxApply && window.__mychatboxApply(%s, %s)", cfg, run)
}

// jsLiteral encodes s as a JavaScript string literal.
func jsLiteral(s string) string {
	data, err := json.Marshal(s)
	if err != nil {
		return `""`
	}
	return string(data)
}
