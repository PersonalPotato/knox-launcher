// Tapes: the tape in the VCR. The launcher opens on the shelf (shelf.js); a tape picked there is a world,
// solo or squad, and the rest of the launcher works on that tape. Making a new tape is just its cover.
import { html, useState, useEffect, S, update, rpc, act, patchHost, inLobby, isHost } from "./core.js";
import { CaseView } from "./vhs3d.js";
import { shelfFaces, tapeTitle, looksDefs, unlockLevel } from "./looks.js";
import { SetupTiles, StartButton } from "./play.js";
import { sound } from "./sound.js";

export async function loadTape(t) {
  sound("insert");
  if (t.kind === "world") await patchHost(t.title ? { world: t.name, squadName: t.title } : { world: t.name });
  update({ tape: t, making: null, tab: "play" });
  const m = document.getElementById("main");
  if (m) m.scrollTop = 0;
}

export function ejectTape() {
  if (inLobby()) act(() => rpc("leaveLobby"));
  sound("eject");
  update({ tape: null, making: null, tab: "play" });
}

const busyNow = () => S.state && ["creating", "joining", "syncing", "launching"].includes(S.state.phase);

// NavTape: the top of the sidebar, what's in the VCR and the way back to the shelf.
export function NavTape() {
  const st = S.state || {};
  const t = S.tape;
  const guest = inLobby() && !isHost();
  const running = st.game && st.game.running;
  const title = guest ? (st.info && st.info.squadName) || `${st.hostName || "Host"}'s game` : t ? tapeTitle(t) : "No tape";
  const sub = guest ? `Guest · ${st.hostName || "a friend"}'s world` : t ? (t.kind === "world" ? "Squad tape" : "Solo tape") + (t.fresh ? " · new world" : "") : "";
  return html`<div class="nav-tape">
    <button class="btn sm" disabled=${running || busyNow()} title=${running ? "Close the game first" : ""} onClick=${ejectTape}>◂ Back to shelf</button>
    <div class="nt-k">In the VCR</div><div class="nt-t">${title}</div>${sub ? html`<div class="nt-s">${sub}</div>` : null}
  </div>`;
}

// TapeHome: Home with a tape in. The tape already says solo or squad, so it's one button.
export function TapeHome() {
  const t = S.tape;
  const st = S.state;
  const steamOK = st.steam && st.steam.ok;
  const busy = busyNow();
  const c = S.cfg && S.cfg.character;
  const key = JSON.stringify([t, S.knox && S.knox.lvl, !!looksDefs()]);
  const start = async () => {
    const r = await rpc("playSolo", { name: t.name });
    update({ tape: { ...t, name: (r && r.world) || t.name, fresh: false } });
    return r;
  };
  let go;
  if (t.kind === "world") go = html`<button class="btn primary big" disabled=${!steamOK || busy} onClick=${() => { sound("play"); act(() => rpc("createLobby")); }}>${st.phase === "creating" ? "Opening..." : "Open the lobby"}</button>`;
  else if (t.fresh) go = html`<${StartButton} label="Start" cls="btn primary big" disabled=${busy} onGo=${start} />`;
  else go = html`<button class="btn primary big" disabled=${busy} onClick=${() => { sound("insert"); act(() => rpc("loadSave", { mode: t.mode, name: t.name })); }}>Continue</button>`;
  const note = t.kind === "world"
    ? (t.fresh ? "Friends see it on their shelf once the lobby is open. Everyone in the lobby goes on the cover." : "Friends see it on their shelf once the lobby is open.")
    : t.fresh ? (c && c.forename ? `${c.forename} ${c.surname} goes on the cover once you start.` : "Make your survivor and they go on the cover.")
    : "This save keeps the world settings it was made with.";
  return html`<div class="tape-home">
      <div class="th-case"><${CaseView} key=${key} make=${() => shelfFaces(t, 0)} height=${340} /></div>
      <div class="th-copy">
        <div class="tk">In the VCR</div>
        <h1 class="wordmark">${tapeTitle(t)}</h1>
        <p>${t.kind === "world" ? "Squad tape" : "Solo tape"}${t.fresh ? " · new world" : ""}</p>
        ${!steamOK && t.kind === "world" ? html`<div class="card note-card bad" style=${{ margin: "10px 0 0" }}>Steam isn't connected. Open Steam and <a href="#" onClick=${(e) => { e.preventDefault(); act(() => rpc("connectSteam"), "Connected to Steam"); }}>try again</a>.</div>` : null}
        <div class="row th-go">${go}<button class="btn" onClick=${() => update({ making: { edit: t } })}>Change cover</button></div>
        <div class="dim small" style=${{ marginTop: "10px" }}>${note}</div>
      </div>
    </div>
    <div class="sect-head"><h3>This tape's setup</h3><span>${t.kind === "world" ? "Click a tile to change it. Friends in your lobby see changes live." : "Click a tile to change it."}</span></div>
    <${SetupTiles} info=${st.info} mine />`;
}

const TITLES = ["The Long Winter", "Dead Air", "Muldraugh After Dark", "Rot", "The Quiet Year", "No One Gets Out", "Last Exit West Point", "Static",
  "Knox County Line", "The Last Summer", "Nobody Home", "Boarded Up", "After the Fall", "Riverside Blues", "Low Tide", "Cold Storage"];

// a world or save name made from the title, not already taken
async function freeName(kind, title) {
  const base = kind === "world" ? title.replace(/[^A-Za-z0-9_-]/g, "") || "Knox" : title.replace(/[/\\:*?"<>|=;]/g, "").trim() || "Solo";
  const [shelf, worlds] = await Promise.all([rpc("shelf"), kind === "world" ? rpc("worlds") : Promise.resolve([])]);
  const taken = new Set([...(shelf || []).filter((t) => t.kind === kind).map((t) => t.name.toLowerCase()), ...(worlds || []).map((w) => w.name.toLowerCase())]);
  let name = base;
  for (let i = 2; taken.has(name.toLowerCase()); i++) name = kind === "world" ? base + i : `${base} (${i})`;
  return name;
}

// TapeMaker: a new tape, or a new cover for one you have. Only the cover; poses are random and the
// world, mods and story are set up in the launcher after.
export function TapeMaker({ edit }) {
  const d = looksDefs() || {};
  const lvl = unlockLevel(S.knox);
  const [title, setTitle] = useState(edit ? tapeTitle(edit) : TITLES[Math.floor(Math.random() * TITLES.length)]);
  const [kind, setKind] = useState(edit ? edit.kind : "squad");
  const [look, setLook] = useState(edit ? { ...(edit.look || S.cfg.look || {}) } : { genre: "mix", backdrop: "mix", spine: "mix", spineInk: "" });
  const [saving, setSaving] = useState(false);
  const set = (p) => setLook({ ...look, ...p });
  const tk = kind === "solo" ? "solo" : "world";
  const preview = { ...(edit || {}), kind: tk, mode: edit ? edit.mode : "Sandbox", name: edit ? edit.name : tk === "world" ? title.replace(/[^A-Za-z0-9_-]/g, "") : title.trim(), title, look, fresh: edit ? edit.fresh : true, games: edit ? edit.games : 0 };
  const cover = { genre: look.genre || "", backdrop: look.backdrop || "", spine: look.spine || "", spineInk: look.spineInk || "", worn: !!look.worn, foil: !!look.foil };
  const close = () => update({ making: null });
  const save = async () => {
    if (!title.trim()) return;
    setSaving(true);
    try {
      if (edit) {
        const ok = await act(() => rpc("saveTape", { kind: edit.kind, mode: edit.mode, name: edit.name, title: title.trim(), look: { ...(edit.look || {}), ...cover } }), "Cover saved");
        if (ok === undefined) return;
        sound("clack");
        update({ making: null, tape: S.tape && S.tape.name === edit.name && S.tape.kind === edit.kind ? { ...S.tape, title: ok.title, look: ok.look } : S.tape });
        return;
      }
      const name = await freeName(tk, title.trim());
      const t = await act(() => rpc("saveTape", { kind: tk, mode: tk === "solo" ? "Sandbox" : "", name, title: title.trim(), look: cover }));
      if (t === undefined) return;
      await loadTape({ ...t, fresh: true, games: 0, kills: 0, deaths: 0, bestDay: 0 });
    } finally {
      setSaving(false);
    }
  };
  const row = (name, list, field, extra) => html`<h4>${name}</h4><div class="look-row">
    ${extra || null}
    <button class=${"btn sm" + (look[field] === "mix" ? " primary" : "")} onClick=${() => set({ [field]: "mix" })}>Surprise me</button>
    ${(list || []).map((u) => html`<button key=${u.id} class=${"btn sm" + (look[field] === u.id ? " primary" : "")} disabled=${lvl < u.lvl} onClick=${() => set({ [field]: u.id })}>${u.name}${lvl < u.lvl ? " · Lv " + u.lvl : ""}</button>`)}</div>`;
  return html`<div class="maker">
    <div class="maker-case"><${CaseView} key=${JSON.stringify([title, look, kind])} make=${() => shelfFaces(preview, 0)} height=${600} /></div>
    <div class="maker-side">
      <div class="tk">${edit ? "Change cover" : "New tape"}</div>
      <div class="maker-panel">
        ${edit ? null : html`<div class="look-row">
          <button class=${"btn" + (kind === "squad" ? " primary" : "")} onClick=${() => setKind("squad")}>Squad</button>
          <button class=${"btn" + (kind === "solo" ? " primary" : "")} onClick=${() => setKind("solo")}>Solo</button>
          <span class="dim small">${kind === "squad" ? "You host, friends join." : "Just you."}</span></div>`}
        <h4>Title</h4>
        <div class="row"><input class="maker-title" maxlength="40" value=${title} onInput=${(e) => setTitle(e.target.value)} /><button class="btn" onClick=${() => setTitle(TITLES[Math.floor(Math.random() * TITLES.length)])}>Random</button></div>
        ${row("Cover", d.genres, "genre")}
        ${row("Backdrop", d.backdrops, "backdrop")}
        ${row("Spine", d.spines, "spine")}
        ${row("Spine colour", d.spineInks, "spineInk", html`<button class=${"btn sm" + (!look.spineInk ? " primary" : "")} onClick=${() => set({ spineInk: "" })}>Match cover</button>`)}
        <h4>Case</h4><div class="look-row">
          <button class=${"btn sm" + (look.worn ? " primary" : "")} disabled=${lvl < (d.worn || 1)} onClick=${() => set({ worn: !look.worn })}>Worn${lvl < (d.worn || 1) ? " · Lv " + d.worn : ""}</button>
          <button class=${"btn sm" + (look.foil ? " primary" : "")} disabled=${lvl < (d.foil || 1)} onClick=${() => set({ foil: !look.foil })}>Foil title${lvl < (d.foil || 1) ? " · Lv " + d.foil : ""}</button></div>
        <div class="dim small" style=${{ marginTop: "12px" }}>${edit ? "Poses are picked at random." : "Poses are picked at random. The cast goes on once there are survivors. World, mods and story are set up next."}</div>
        <div class="row" style=${{ marginTop: "16px" }}>
          <button class="btn" onClick=${close}>Cancel</button>
          <button class="btn primary big" disabled=${saving || !title.trim()} onClick=${save}>${edit ? "Save cover" : "Make the tape"}</button>
        </div>
      </div>
    </div>
  </div>`;
}
