(() => {
  const controllers = new WeakMap();
  const syncTimers = new WeakMap();
  const visibleByEvent = new Map();
  const celebratedGoalEvents = new Set();
  let pendingDuelRequests = 0;
  let serverClockOffsetMs = 0;
  let serverClockObserved = false;
  let pendingOffsetCorrectionMs = 0;
  let hardCorrectionOffsetMs = null;

  function visibleCount(length, elapsedMs, durationMs) {
    if (length === 0) return 0;
    if (durationMs <= 0 || elapsedMs >= durationMs) return length;
    return Math.max(1, Math.min(length, Math.floor((elapsedMs / durationMs) * length)));
  }

  function localNowMs() {
    return performance.timeOrigin + performance.now();
  }

  function syncedServerNowMs() {
    return localNowMs() + serverClockOffsetMs;
  }

  function applyObservedServerOffset(observedOffset, options = {}) {
    if (!Number.isFinite(observedOffset)) return;
    if (!serverClockObserved || options.snap || document.hidden) {
      serverClockOffsetMs = observedOffset;
      pendingOffsetCorrectionMs = 0;
      hardCorrectionOffsetMs = null;
      serverClockObserved = true;
      return;
    }

    const drift = observedOffset - serverClockOffsetMs;
    const magnitude = Math.abs(drift);
    if (magnitude < 150) return;
    if (magnitude >= 5000 || (options.deferLarge && magnitude >= 1500)) {
      hardCorrectionOffsetMs = observedOffset;
      return;
    }
    pendingOffsetCorrectionMs += drift;
  }

  function advanceOffsetCorrection() {
    if (pendingOffsetCorrectionMs === 0) return;
    const step = Math.sign(pendingOffsetCorrectionMs) * Math.min(Math.abs(pendingOffsetCorrectionMs), 8);
    serverClockOffsetMs += step;
    pendingOffsetCorrectionMs -= step;
  }

  function observeServerClock(panel) {
    const renderedAtMs = Number(panel?.dataset.renderedAtMs || "0");
    if (!renderedAtMs) return;
    applyObservedServerOffset(renderedAtMs - localNowMs(), { snap: !serverClockObserved });
  }

  function matchKeyFor(el) {
    return el.closest("[data-broadcast-state]")?.dataset.matchId || "latest";
  }

  function eventKeyFor(el) {
    return matchKeyFor(el) + ":" + (el.dataset.eventKey || "live");
  }

  function formatClock(totalSeconds) {
    totalSeconds = Math.max(0, Math.round(totalSeconds));
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;
    return String(minutes).padStart(2, "0") + ":" + String(seconds).padStart(2, "0");
  }

  function updateEventReveal(el, visible, allowEffects) {
    const revealIndex = Number(el.dataset.revealIndex || "0");
    const item = el.closest("[data-event-item]");
    if (!item) return;
    const revealed = visible >= revealIndex;
    item.dataset.revealed = revealed ? "true" : "false";
    item.querySelector("[data-event-label]")?.classList.toggle("hidden", !revealed);
    item.querySelector("[data-event-attribution]")?.classList.toggle("hidden", !revealed);
    item.querySelector("[data-generic-label]")?.classList.toggle("hidden", revealed);
    const key = eventKeyFor(el);
    if (allowEffects && revealed && el.dataset.goalTrigger === "true" && !celebratedGoalEvents.has(key)) {
      celebratedGoalEvents.add(key);
      const panel = el.closest("[data-broadcast-state]");
      if (panel) burstConfetti(panel);
    }
  }

  async function syncMatch(panel, snap = false) {
    const syncURL = panel?.dataset.syncUrl;
    if (!syncURL) return;
    try {
      const response = await fetch(syncURL, { headers: { "Accept": "application/json" } });
      if (!response.ok) return;
      const state = await response.json();
      if (state.match_version && state.match_version !== panel.dataset.matchVersion) {
        window.location.reload();
        return;
      }
      if (state.ended_at_ms) panel.dataset.matchEndedAtMs = String(state.ended_at_ms);
      if (state.server_now_ms) {
        applyObservedServerOffset(Number(state.server_now_ms) - localNowMs(), { snap, deferLarge: true });
      }
    } catch (_error) {
      // A missed sync should never interrupt local playback.
    }
  }

  function scheduleMatchSync(panel) {
    const previous = syncTimers.get(panel);
    if (previous) window.clearInterval(previous);
    if (!panel.dataset.syncUrl) return;
    const handle = window.setInterval(() => syncMatch(panel), 45000);
    syncTimers.set(panel, handle);
  }

  function stopMatchSync(panel) {
    const previous = syncTimers.get(panel);
    if (!previous) return;
    window.clearInterval(previous);
    syncTimers.delete(panel);
  }

  function setEventLive(item, live) {
    item.classList.toggle("border-lime-300", live);
    item.classList.toggle("border-zinc-800", !live);
    const meta = item.querySelector("[data-event-meta]");
    meta?.classList.toggle("text-lime-300", live);
    meta?.classList.toggle("text-zinc-500", !live);
    item.querySelector("[data-now-pill]")?.classList.toggle("hidden", !live);
    item.querySelector("[data-cursor]")?.classList.toggle("hidden", !live);
  }

  function shiftDatasetTime(el, key, deltaMs) {
    const value = Number(el.dataset[key] || "0");
    if (!value) return;
    el.dataset[key] = String(value + deltaMs);
  }

  function prepareReplayPanel(panel) {
    if (panel.dataset.replayPrepared === "true") return;
    const originalStartMs = Number(panel.dataset.matchStartedAtMs || "0");
    if (!originalStartMs) return;
    const replayStartMs = syncedServerNowMs() + 250;
    const deltaMs = replayStartMs - originalStartMs;
    panel.dataset.matchStartedAtMs = String(replayStartMs);
    shiftDatasetTime(panel, "matchEndedAtMs", deltaMs);
    shiftDatasetTime(panel, "renderedAtMs", deltaMs);

    panel.querySelectorAll("[data-event-item]").forEach((item) => {
      ["eventStartedAtMs", "eventTextEndAtMs", "eventPauseEndAtMs", "eventClockEndAtMs", "scoreAtMs"].forEach((key) => shiftDatasetTime(item, key, deltaMs));
      item.classList.add("hidden");
      setEventLive(item, false);
      const textEl = item.querySelector("[data-typewriter]");
      if (!textEl) return;
      shiftDatasetTime(textEl, "eventStartedAtMs", deltaMs);
      textEl.textContent = "";
      const key = eventKeyFor(textEl);
      visibleByEvent.delete(key);
      celebratedGoalEvents.delete(key);
      updateEventReveal(textEl, 0, false);
    });

    const scoreTeamAEl = panel.querySelector("[data-score-team-a]");
    const scoreTeamBEl = panel.querySelector("[data-score-team-b]");
    const clockEl = panel.querySelector("[data-match-clock]");
    const progressEl = panel.querySelector("[data-progress-bar]");
    if (scoreTeamAEl) scoreTeamAEl.textContent = "0";
    if (scoreTeamBEl) scoreTeamBEl.textContent = "0";
    if (clockEl) clockEl.textContent = "00:00";
    if (progressEl) progressEl.style.width = "0%";
    panel.dataset.replayPrepared = "true";
  }

  function hydrateMatchPlayers(root) {
    const panels = [];
    if (root.matches?.("[data-broadcast-state]")) panels.push(root);
    root.querySelectorAll?.("[data-broadcast-state]").forEach((panel) => panels.push(panel));

    panels.forEach((panel) => {
      const previous = controllers.get(panel);
      if (previous) cancelAnimationFrame(previous);
      const playbackMode = panel.dataset.playbackMode || "live";
      if (playbackMode === "replay") {
        prepareReplayPanel(panel);
      } else {
        observeServerClock(panel);
      }

      const clockEl = panel.querySelector("[data-match-clock]");
      const progressEl = panel.querySelector("[data-progress-bar]");
      const badgeEl = panel.querySelector("[data-live-badge]");
      const matchEndedAtMs = Number(panel.dataset.matchEndedAtMs || "0");
      const scoreTeamAEl = panel.querySelector("[data-score-team-a]");
      const scoreTeamBEl = panel.querySelector("[data-score-team-b]");
      const finalScoreTeamA = Number(panel.dataset.finalScoreTeamA || "0");
      const finalScoreTeamB = Number(panel.dataset.finalScoreTeamB || "0");
      const eventItems = Array.from(panel.querySelectorAll("[data-event-item]")).sort((a, b) => {
        return Number(a.dataset.eventKey || "0") - Number(b.dataset.eventKey || "0");
      });
      if (playbackMode === "live") {
        scheduleMatchSync(panel);
      } else {
        stopMatchSync(panel);
      }

      function frame() {
        advanceOffsetCorrection();
        const nowMs = syncedServerNowMs();
        const currentMatchEndedAtMs = Number(panel.dataset.matchEndedAtMs || String(matchEndedAtMs));
        let clockSecond = 0;
        let scoreA = 0;
        let scoreB = 0;
        let shouldContinue = currentMatchEndedAtMs <= 0 || nowMs < currentMatchEndedAtMs;
        let activeItem = null;
        let activeTyping = false;

        eventItems.forEach((item) => {
          const textEl = item.querySelector("[data-typewriter]");
          if (!textEl) return;

          const startedAtMs = Number(item.dataset.eventStartedAtMs || "0");
          const textEndAtMs = Number(item.dataset.eventTextEndAtMs || "0");
          const pauseEndAtMs = Number(item.dataset.eventPauseEndAtMs || "0");
          const clockEndAtMs = Number(item.dataset.eventClockEndAtMs || "0");
          const clockStartSecond = Number(item.dataset.clockStartSecond || "0");
          const clockEndSecond = Number(item.dataset.clockEndSecond || String(clockStartSecond));
          const scoreAtMs = Number(item.dataset.scoreAtMs || "0");
          const goalTeamSide = item.dataset.goalTeamSide || "";
          const isGoalEvent = item.dataset.eventType === "goal";
          const durationMs = Number(textEl.dataset.durationMs || "1");
          const fullText = textEl.dataset.fullText || "";
          const chars = Array.from(fullText);
          const key = eventKeyFor(textEl);

          if (isGoalEvent && goalTeamSide && scoreAtMs > 0 && nowMs >= scoreAtMs) {
            if (goalTeamSide === "a") scoreA += 1;
            if (goalTeamSide === "b") scoreB += 1;
          }

          if (nowMs < startedAtMs) {
            item.classList.add("hidden");
            setEventLive(item, false);
            return;
          }

          item.classList.remove("hidden");
          const elapsedMs = Math.max(0, nowMs - startedAtMs);
          const calculatedVisible = nowMs >= textEndAtMs ? chars.length : visibleCount(chars.length, elapsedMs, durationMs);
          const visible = Math.max(visibleByEvent.get(key) || 0, calculatedVisible);
          visibleByEvent.set(key, visible);
          textEl.textContent = chars.slice(0, visible).join("");

          const isLive = nowMs >= startedAtMs && nowMs < pauseEndAtMs;
          const isTyping = nowMs >= startedAtMs && nowMs < textEndAtMs;
          setEventLive(item, isLive);
          updateEventReveal(textEl, visible, isTyping);
          if (isLive) activeItem = item;
          if (isTyping) activeTyping = true;

          if (nowMs >= startedAtMs && nowMs < pauseEndAtMs) {
            clockSecond = clockStartSecond;
          } else if (nowMs >= pauseEndAtMs && nowMs < clockEndAtMs && clockEndAtMs > pauseEndAtMs) {
            const ratio = Math.max(0, Math.min(1, (nowMs - pauseEndAtMs) / (clockEndAtMs - pauseEndAtMs)));
            clockSecond = clockStartSecond + ((clockEndSecond - clockStartSecond) * ratio);
          } else if (nowMs >= clockEndAtMs) {
            clockSecond = clockEndSecond;
          }
        });

        if (hardCorrectionOffsetMs !== null && (!activeTyping || document.hidden)) {
          serverClockOffsetMs = hardCorrectionOffsetMs;
          pendingOffsetCorrectionMs = 0;
          hardCorrectionOffsetMs = null;
        }

        if (currentMatchEndedAtMs > 0 && nowMs >= currentMatchEndedAtMs) {
          clockSecond = 40 * 60;
          scoreA = finalScoreTeamA;
          scoreB = finalScoreTeamB;
          badgeEl?.classList.remove("bg-red-400", "text-red-950");
          badgeEl?.classList.add("bg-lime-300", "text-zinc-950");
          if (badgeEl) badgeEl.textContent = "ENCERRADO";
        } else if (badgeEl) {
          badgeEl.textContent = "AO VIVO";
        }
        if (scoreTeamAEl) scoreTeamAEl.textContent = String(scoreA);
        if (scoreTeamBEl) scoreTeamBEl.textContent = String(scoreB);

        if (clockEl) clockEl.textContent = formatClock(clockSecond);
        if (progressEl) {
          const progress = Math.max(0, Math.min(100, (clockSecond / (40 * 60)) * 100));
          progressEl.style.width = progress + "%";
        }
        if (activeItem) scrollNearestFeed(activeItem);

        if (shouldContinue) {
          const handle = requestAnimationFrame(frame);
          controllers.set(panel, handle);
          return;
        }
        controllers.delete(panel);
      }

      const handle = requestAnimationFrame(frame);
      controllers.set(panel, handle);
    });
  }


  function parseAbilityOptions(value) {
    try {
      const parsed = JSON.parse(value || "[]");
      return Array.isArray(parsed) ? parsed : [];
    } catch (_error) {
      return [];
    }
  }

  function abilityLabel(name) {
    return String(name || "").split("-").filter(Boolean).map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(" ");
  }

  function selectedPokemonIds(form, exceptHidden) {
    const selected = new Set();
    form?.querySelectorAll("[data-pokemon-id]").forEach((hidden) => {
      if (hidden === exceptHidden) return;
      if (hidden.value) selected.add(hidden.value);
    });
    return selected;
  }


  function updateLineupPreview(slot, option) {
    if (!slot) return;
    const preview = slot.querySelector("[data-lineup-preview]");
    const placeholder = slot.querySelector("[data-lineup-preview-placeholder]");
    const artwork = option?.dataset.artwork || "";
    if (!preview) return;
    if (artwork) {
      preview.src = artwork;
      preview.classList.remove("hidden");
      placeholder?.classList.add("hidden");
    } else {
      preview.removeAttribute("src");
      preview.classList.add("hidden");
      placeholder?.classList.remove("hidden");
    }
  }

  function pokemonOptionFor(picker, id) {
    if (!id) return null;
    return Array.from(picker.querySelectorAll("[data-pokemon-option]")).find((option) => option.dataset.id === String(id)) || null;
  }

  function updateAbilityPickerForSlot(slot, preferredAbility) {
    if (!slot) return;
    const pokemonPicker = slot.querySelector("[data-pokemon-picker]");
    const abilityPicker = slot.querySelector("[data-ability-picker]");
    if (!pokemonPicker || !abilityPicker) return;

    const pokemonID = pokemonPicker.querySelector("[data-pokemon-id]")?.value || "";
    const pokemonOption = pokemonOptionFor(pokemonPicker, pokemonID);
    const abilities = parseAbilityOptions(pokemonOption?.dataset.abilities || "[]");
    abilityPicker._abilities = abilities;

    const input = abilityPicker.querySelector("[data-ability-search]");
    const hidden = abilityPicker.querySelector("[data-ability-id]");
    if (!input || !hidden) return;

    const hasPreferred = typeof preferredAbility === "string" && preferredAbility !== "";
    const selected = hasPreferred ? abilities.find((ability) => ability.name === preferredAbility || ability.label === preferredAbility) : null;
    input.disabled = abilities.length === 0;
    input.placeholder = pokemonID ? (abilities.length === 0 ? "Sem habilidades" : "Escolha uma habilidade") : "Escolha um Pokemon";

    if (selected) {
      hidden.value = selected.name || "";
      input.value = selected.label || abilityLabel(selected.name);
      return;
    }

    hidden.value = "";
    input.value = "";
  }

  function refreshOpenPokemonPickers(form) {
    form?.querySelectorAll("[data-pokemon-picker]").forEach((picker) => {
      if (picker.dataset.menuOpen === "true") picker._renderMatches?.();
    });
  }

  function hydratePokemonPickers(root) {
    const pickers = [];
    if (root.matches?.("[data-pokemon-picker]")) pickers.push(root);
    root.querySelectorAll?.("[data-pokemon-picker]").forEach((picker) => pickers.push(picker));

    pickers.forEach((picker) => {
      if (picker.dataset.hydrated === "true") return;
      picker.dataset.hydrated = "true";
      const input = picker.querySelector("[data-pokemon-search]");
      const hidden = picker.querySelector("[data-pokemon-id]");
      const menu = picker.querySelector("[data-pokemon-options]");
      const options = Array.from(picker.querySelectorAll("[data-pokemon-option]"));
      let activeIndex = -1;
      let visibleOptions = [];

      function closeMenu() {
        menu?.classList.add("hidden");
        picker.dataset.menuOpen = "false";
        activeIndex = -1;
        options.forEach((option) => option.classList.remove("bg-zinc-800"));
      }

      function setActive(index) {
        visibleOptions.forEach((option) => option.classList.remove("bg-zinc-800"));
        activeIndex = index;
        const active = visibleOptions[activeIndex];
        if (active) {
          active.classList.add("bg-zinc-800");
          active.scrollIntoView({ block: "nearest" });
        }
      }

      function choose(option) {
        if (!option) return;
        input.value = option.dataset.name || "";
        hidden.value = option.dataset.id || "";
        updateLineupPreview(picker.closest("[data-lineup-slot]"), option);
        updateAbilityPickerForSlot(picker.closest("[data-lineup-slot]"));
        closeMenu();
        refreshOpenPokemonPickers(picker.closest("form"));
      }

      function renderMatches() {
        const query = (input.value || "").trim().toLowerCase();
        const exact = options.find((option) => option.dataset.name === input.value);
        hidden.value = exact?.dataset.id || hidden.value;
        const selectedElsewhere = selectedPokemonIds(picker.closest("form"), hidden);
        visibleOptions = [];
        options.forEach((option) => {
          const alreadyUsed = selectedElsewhere.has(option.dataset.id || "");
          const match = !alreadyUsed && query.length >= 2 && (option.dataset.search || "").startsWith(query);
          option.classList.toggle("hidden", !match);
          option.classList.toggle("flex", match);
          option.classList.remove("bg-zinc-800");
          if (match && visibleOptions.length < 8) {
            visibleOptions.push(option);
            option.classList.remove("hidden");
            option.classList.add("flex");
          } else if (match) {
            option.classList.add("hidden");
            option.classList.remove("flex");
          }
        });
        if (!menu) return;
        if (visibleOptions.length === 0) {
          closeMenu();
          return;
        }
        menu.classList.remove("hidden");
        picker.dataset.menuOpen = "true";
        setActive(0);
      }

      picker._renderMatches = renderMatches;
      updateAbilityPickerForSlot(picker.closest("[data-lineup-slot]"), picker.closest("[data-lineup-slot]")?.querySelector("[data-ability-id]")?.value || "");

      input?.addEventListener("input", () => {
        hidden.value = "";
        updateLineupPreview(picker.closest("[data-lineup-slot]"), null);
        updateAbilityPickerForSlot(picker.closest("[data-lineup-slot]"));
        renderMatches();
        refreshOpenPokemonPickers(picker.closest("form"));
      });
      input?.addEventListener("focus", renderMatches);
      input?.addEventListener("keydown", (event) => {
        if (event.key === "ArrowDown") {
          event.preventDefault();
          if (visibleOptions.length === 0) renderMatches();
          setActive(Math.min(visibleOptions.length - 1, activeIndex + 1));
        } else if (event.key === "ArrowUp") {
          event.preventDefault();
          setActive(Math.max(0, activeIndex - 1));
        } else if (event.key === "Enter" && activeIndex >= 0 && visibleOptions[activeIndex]) {
          event.preventDefault();
          choose(visibleOptions[activeIndex]);
        } else if (event.key === "Escape") {
          closeMenu();
        }
      });
      options.forEach((option) => option.addEventListener("mousedown", (event) => {
        event.preventDefault();
        choose(option);
      }));
      picker.addEventListener("focusout", () => {
        window.setTimeout(() => {
          if (!picker.contains(document.activeElement)) closeMenu();
        }, 0);
      });
      document.addEventListener("click", (event) => {
        if (!picker.contains(event.target)) closeMenu();
      });
    });
  }

  function hydrateAbilityPickers(root) {
    const pickers = [];
    if (root.matches?.("[data-ability-picker]")) pickers.push(root);
    root.querySelectorAll?.("[data-ability-picker]").forEach((picker) => pickers.push(picker));

    pickers.forEach((picker) => {
      if (picker.dataset.hydrated === "true") return;
      picker.dataset.hydrated = "true";
      const input = picker.querySelector("[data-ability-search]");
      const hidden = picker.querySelector("[data-ability-id]");
      const menu = picker.querySelector("[data-ability-options]");
      let activeIndex = -1;
      let visibleOptions = [];

      function closeMenu() {
        menu?.classList.add("hidden");
        activeIndex = -1;
      }

      function choose(ability) {
        if (!ability) return;
        hidden.value = ability.name || "";
        input.value = ability.label || abilityLabel(ability.name);
        closeMenu();
      }

      function renderMatches(showAll = false) {
        const abilities = picker._abilities || [];
        const query = showAll ? "" : (input.value || "").trim().toLowerCase();
        visibleOptions = abilities.filter((ability) => {
          const search = ability.search || ((ability.label || "") + " " + (ability.name || "") + " " + (ability.description || "")).toLowerCase();
          return query === "" || search.includes(query);
        });
        if (!menu) return;
        menu.textContent = "";
        if (visibleOptions.length === 0) {
          closeMenu();
          return;
        }
        visibleOptions.forEach((ability, index) => {
          const button = document.createElement("button");
          button.type = "button";
          button.className = "block w-full px-3 py-2 text-left hover:bg-zinc-800";
          const label = document.createElement("span");
          label.className = "block font-medium text-zinc-100";
          label.textContent = ability.label || abilityLabel(ability.name);
          button.appendChild(label);
          if (ability.description) {
            const description = document.createElement("span");
            description.className = "mt-0.5 block line-clamp-2 text-xs text-zinc-500";
            description.textContent = ability.description;
            button.appendChild(description);
          }
          button.addEventListener("mousedown", (event) => {
            event.preventDefault();
            choose(ability);
          });
          if (index === activeIndex) button.classList.add("bg-zinc-800");
          menu.appendChild(button);
        });
        activeIndex = Math.max(0, Math.min(activeIndex, visibleOptions.length - 1));
        menu.classList.remove("hidden");
      }

      input?.addEventListener("focus", () => {
        activeIndex = 0;
        renderMatches(true);
      });
      input?.addEventListener("input", () => {
        hidden.value = input.value || "";
        renderMatches();
      });
      input?.addEventListener("keydown", (event) => {
        if (event.key === "ArrowDown") {
          event.preventDefault();
          activeIndex = Math.min(visibleOptions.length - 1, activeIndex + 1);
          renderMatches();
        } else if (event.key === "ArrowUp") {
          event.preventDefault();
          activeIndex = Math.max(0, activeIndex - 1);
          renderMatches();
        } else if (event.key === "Enter" && activeIndex >= 0 && visibleOptions[activeIndex]) {
          event.preventDefault();
          choose(visibleOptions[activeIndex]);
        } else if (event.key === "Escape") {
          closeMenu();
        }
      });
      picker.addEventListener("focusout", () => {
        window.setTimeout(() => {
          if (!picker.contains(document.activeElement)) closeMenu();
        }, 0);
      });
      document.addEventListener("click", (event) => {
        if (!picker.contains(event.target)) closeMenu();
      });
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
      feed.scrollTop = 0;
    });
  }

  function scrollNearestFeed(el) {
    const feed = el.closest("[data-event-feed]");
    if (feed) feed.scrollTop = 0;
  }


  function hydrateLineupClearButtons(root) {
    const buttons = [];
    if (root.matches?.("[data-clear-lineup-slot]")) buttons.push(root);
    root.querySelectorAll?.("[data-clear-lineup-slot]").forEach((button) => buttons.push(button));

    buttons.forEach((button) => {
      if (button.dataset.hydrated === "true") return;
      button.dataset.hydrated = "true";
      button.addEventListener("click", () => {
        const slot = button.closest("[data-lineup-slot]");
        if (!slot) return;
        const pokemonInput = slot.querySelector("[data-pokemon-search]");
        const pokemonHidden = slot.querySelector("[data-pokemon-id]");
        const abilityInput = slot.querySelector("[data-ability-search]");
        const abilityHidden = slot.querySelector("[data-ability-id]");
        if (pokemonInput) pokemonInput.value = "";
        if (pokemonHidden) pokemonHidden.value = "";
        if (abilityInput) abilityInput.value = "";
        if (abilityHidden) abilityHidden.value = "";
        updateLineupPreview(slot, null);
        updateAbilityPickerForSlot(slot);
        slot.querySelector("[data-pokemon-options]")?.classList.add("hidden");
        slot.querySelector("[data-ability-options]")?.classList.add("hidden");
        refreshOpenPokemonPickers(slot.closest("form"));
      });
    });
  }

  function duelFormFor(el) {
    return el?.closest?.("[data-duel-form]");
  }

  function duelCancelWarning() {
    return document.querySelector("[data-duel-form]")?.dataset.duelCancelWarning || "Sair desta pagina cancelara a geracao do duelo.";
  }

  function updateDuelUnloadWarning() {
    document.body.toggleAttribute("data-duel-request-pending", pendingDuelRequests > 0);
  }

  function beginDuelRequest() {
    pendingDuelRequests += 1;
    updateDuelUnloadWarning();
  }

  function endDuelRequest() {
    pendingDuelRequests = Math.max(0, pendingDuelRequests - 1);
    updateDuelUnloadWarning();
  }

  function clearDuelRequests() {
    pendingDuelRequests = 0;
    updateDuelUnloadWarning();
  }

  function setDuelLoading(form, loading) {
    if (!form) return;
    form.classList.toggle("is-loading", loading);
    form.querySelectorAll("button, select, input").forEach((control) => {
      control.toggleAttribute("disabled", loading);
    });
  }

  function showDuelError(form, message) {
    const errorEl = form?.querySelector("[data-duel-error]");
    if (!errorEl) return;
    const text = String(message || "").trim() || "Nao foi possivel gerar a partida.";
    errorEl.textContent = text;
    errorEl.classList.remove("hidden");
  }

  function clearDuelError(form) {
    const errorEl = form?.querySelector("[data-duel-error]");
    if (!errorEl) return;
    errorEl.textContent = "";
    errorEl.classList.add("hidden");
  }

  function duelErrorMessage(xhr) {
    const status = Number(xhr?.status || 0);
    const raw = String(xhr?.responseText || "").trim();
    const contentType = String(xhr?.getResponseHeader?.("Content-Type") || "").toLowerCase();
    const looksLikeHTML = contentType.includes("text/html") || /^<!doctype\s+html/i.test(raw) || /^<html[\s>]/i.test(raw);

    if (status === 401 || status === 403) {
      return "Sessao expirada. Entre novamente para continuar.";
    }
    if (status === 429) {
      return looksLikeHTML ? "Limite temporario atingido. Tente novamente em alguns minutos." : raw;
    }
    if (looksLikeHTML) {
      if (status === 502 || status === 503 || status === 504) {
        return "O servidor demorou demais para gerar a partida. Tente novamente em instantes.";
      }
      return "Nao foi possivel gerar a partida. Recarregue a pagina e tente novamente.";
    }
    return raw;
  }

  function hydrate(root = document) {
    hydrateMatchPlayers(root);
    hydratePokemonPickers(root);
    hydrateAbilityPickers(root);
    hydrateLineupClearButtons(root);
    scrollEventFeeds(root);
  }

  document.addEventListener("DOMContentLoaded", () => hydrate());
  document.addEventListener("visibilitychange", () => {
    if (document.hidden) return;
    document.querySelectorAll("[data-broadcast-state]").forEach((panel) => syncMatch(panel, true));
  });
  document.body.addEventListener("htmx:afterSwap", (event) => hydrate(event.target));
  document.body.addEventListener("htmx:beforeRequest", (event) => {
    const form = duelFormFor(event.target);
    if (!form) return;
    beginDuelRequest();
    clearDuelError(form);
    setDuelLoading(form, true);
  });
  document.body.addEventListener("htmx:beforeOnLoad", (event) => {
    const redirectURL = event.detail?.xhr?.getResponseHeader?.("HX-Redirect");
    if (!redirectURL || pendingDuelRequests <= 0) return;
    clearDuelRequests();
  });
  document.body.addEventListener("htmx:afterRequest", (event) => {
    const form = duelFormFor(event.target);
    if (!form) return;
    endDuelRequest();
    setDuelLoading(form, false);
  });
  document.body.addEventListener("htmx:responseError", (event) => {
    const form = duelFormFor(event.target);
    if (!form) return;
    setDuelLoading(form, false);
    showDuelError(form, duelErrorMessage(event.detail?.xhr));
  });
  window.addEventListener("beforeunload", (event) => {
    if (pendingDuelRequests <= 0) return;
    const message = duelCancelWarning();
    event.preventDefault();
    event.returnValue = message;
    return message;
  });
})();
