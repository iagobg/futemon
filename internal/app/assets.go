package app

import "net/http"

const appJS = `
(() => {
  const controllers = new WeakMap();

  function visibleText(text, elapsedMs, durationMs) {
    const chars = Array.from(text || "");
    if (chars.length === 0) return "";
    if (durationMs <= 0 || elapsedMs >= durationMs) return text;
    const visible = Math.max(1, Math.min(chars.length, Math.floor((elapsedMs / durationMs) * chars.length)));
    return chars.slice(0, visible).join("");
  }

  function hydrateTypewriter(el) {
    const previous = controllers.get(el);
    if (previous) cancelAnimationFrame(previous);

    const fullText = el.dataset.fullText || "";
    const initialElapsedMs = Number(el.dataset.eventElapsedMs || "0");
    const durationMs = Number(el.dataset.durationMs || "1");
    const startedAt = performance.now();

    function frame(now) {
      const elapsedMs = initialElapsedMs + (now - startedAt);
      el.textContent = visibleText(fullText, elapsedMs, durationMs);
      scrollNearestFeed(el);
      if (elapsedMs < durationMs) {
        const handle = requestAnimationFrame(frame);
        controllers.set(el, handle);
        return;
      }
      controllers.delete(el);
    }

    const handle = requestAnimationFrame(frame);
    controllers.set(el, handle);
  }

  function formatClock(totalSeconds) {
    totalSeconds = Math.max(0, Math.round(totalSeconds));
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;
    return String(minutes).padStart(2, "0") + ":" + String(seconds).padStart(2, "0");
  }

  function hydrateBroadcast(root) {
    const panels = [];
    if (root.matches && root.matches("[data-broadcast-state]")) panels.push(root);
    root.querySelectorAll?.("[data-broadcast-state]").forEach((panel) => panels.push(panel));

    panels.forEach((panel) => {
      const clockEl = panel.querySelector("[data-match-clock]");
      const progressEl = panel.querySelector("[data-progress-bar]");
      const clockRunning = panel.dataset.clockRunning === "true";
      const startSecond = Number(panel.dataset.clockStartSecond || "0");
      const endSecond = Number(panel.dataset.clockEndSecond || String(startSecond));
      const initialElapsedMs = Number(panel.dataset.clockElapsedMs || "0");
      const durationMs = Number(panel.dataset.clockDurationMs || "0");
      const startedAt = performance.now();

      function render(now) {
        const elapsedMs = initialElapsedMs + (now - startedAt);
        let currentSecond = startSecond;
        if (clockRunning && durationMs > 0) {
          const ratio = Math.max(0, Math.min(1, elapsedMs / durationMs));
          currentSecond = startSecond + ((endSecond - startSecond) * ratio);
        }
        if (clockEl) clockEl.textContent = formatClock(currentSecond);
        if (progressEl) {
          const progress = Math.max(0, Math.min(100, (currentSecond / (40 * 60)) * 100));
          progressEl.style.width = progress + "%";
        }
        if (clockRunning && elapsedMs < durationMs) {
          requestAnimationFrame(render);
        }
      }

      requestAnimationFrame(render);

      if (panel.dataset.goalLive === "true") {
        burstConfetti(panel);
      }
    });
  }

  function burstConfetti(panel) {
    const host = panel.querySelector("[data-confetti]");
    if (!host) return;
    host.textContent = "";
    const colors = ["#bef264", "#38bdf8", "#fb7185", "#facc15", "#c084fc"];
    for (let i = 0; i < 28; i++) {
      const piece = document.createElement("span");
      piece.className = "confetti-piece absolute block h-2 w-1 rounded-sm";
      piece.style.left = (8 + Math.random() * 84) + "%";
      piece.style.top = "-12px";
      piece.style.backgroundColor = colors[i % colors.length];
      piece.style.animationDelay = (Math.random() * 180) + "ms";
      piece.style.animationDuration = (900 + Math.random() * 700) + "ms";
      host.appendChild(piece);
    }
    window.setTimeout(() => { host.textContent = ""; }, 1800);
  }

  function scrollEventFeeds(root) {
    const feeds = [];
    if (root.matches?.("[data-event-feed]")) feeds.push(root);
    root.querySelectorAll?.("[data-event-feed]").forEach((feed) => feeds.push(feed));
    feeds.forEach((feed) => {
      feed.scrollTop = feed.scrollHeight;
    });
  }

  function scrollNearestFeed(el) {
    const feed = el.closest("[data-event-feed]");
    if (feed) feed.scrollTop = feed.scrollHeight;
  }

  function scheduleBroadcastRefresh(root) {
    const panel = root.matches?.("[data-broadcast-state]") ? root : root.querySelector?.("[data-broadcast-state]");
    if (!panel) return;
    const nextRefreshMs = Number(panel.dataset.nextRefreshMs || "0");
    if (nextRefreshMs <= 0) return;
    const delay = Math.max(250, nextRefreshMs);
    window.setTimeout(() => {
      document.body.dispatchEvent(new Event("broadcast-refresh", { bubbles: true }));
    }, delay);
  }

  function hydrate(root = document) {
    root.querySelectorAll("[data-typewriter]").forEach(hydrateTypewriter);
    hydrateBroadcast(root);
    scrollEventFeeds(root);
    scheduleBroadcastRefresh(root);
  }

  document.addEventListener("DOMContentLoaded", () => hydrate());
  document.body.addEventListener("htmx:afterSwap", (event) => hydrate(event.target));
})();
`

func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/static/app.js" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(appJS))
}
