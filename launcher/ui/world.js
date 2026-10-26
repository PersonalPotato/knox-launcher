// World page (which world, difficulty presets, the main rules) and the full settings editor.
import { html, useState, useEffect, S, update, rpc, act, toast, patchHost, setOption, stripTags, PageHead, Select, Switch, Toggle, HostOnly, isHost, loadPresets } from "./core.js";

let worldCache = { name: null, data: null, list: [] };
async function loadWorld(name, force) {
  if (!force && worldCache.name === name && worldCache.data) return;
  const [data, list] = await Promise.all([act(() => rpc("worldSettings", { world: name })), act(() => rpc("worlds"))]);
  worldCache = { name, data: data || { ini: {}, sandbox: {}, exists: false }, list: list || [] };
  update();
}

function useWorld() {
  const name = S.cfg.host.world;
  useEffect(() => { loadWorld(name); }, [name]);
  return worldCache.name === name ? worldCache : { name, data: null, list: worldCache.list };
}

function value(kind, s, w) {
  const over = kind === "ini" ? S.cfg.host.ini : S.cfg.host.sandbox;
  if (over[s.name] != null) return over[s.name];
  const cur = w && (kind === "ini" ? w.ini : w.sandbox);
  if (cur && cur[s.name] != null) return cur[s.name];
  return s.default;
}

const QUICK = [
  { kind: "ini", name: "PVP", title: "Player vs player", sub: "Friends can hurt each other." },
  { kind: "ini", name: "SleepAllowed", title: "Sleeping", sub: "Players can and need to sleep.", also: "SleepNeeded" },
  { kind: "ini", name: "PauseEmpty", title: "Pause when empty", sub: "Time stops when nobody's online." },
  { kind: "ini", name: "PlayerSafehouse", title: "Safehouses", sub: "Players can claim a house." },
  { kind: "ini", name: "GlobalChat", title: "Global chat", sub: "Chat reaches everyone, not just nearby." },
];

export function World() {
  const host = isHost();
  const w = useWorld();
  if (!S.presets || !S.cat) return null;
  const h = host ? S.cfg.host : S.state.info || {};
  const names = w.list.map((x) => x.name);
  if (h.world && !names.includes(h.world)) names.push(h.world);
  const diffs = S.presets.difficulty;
  const groups = [
    { title: "Game presets", list: diffs.filter((d) => !d.custom && !d.id.startsWith("knox-")) },
    { title: "Knox presets", list: diffs.filter((d) => d.id.startsWith("knox-")) },
    { title: "Your presets", list: diffs.filter((d) => d.custom), mine: true },
  ].filter((g) => g.list.length || (g.mine && host));
  const exists = w.data && w.data.exists;
  return html`
    <${PageHead} title="World" sub=${!host ? "" : exists ? `${h.world} (saved world)` : `${h.world} (new, made on START)`} />
    ${!host ? html`<${HostOnly} what="world and difficulty" />` : null}
    <div class="world-top">
      <div class="card"><h3>World</h3>
        ${host && S.tape ? html`<div class="grid" style=${{ gap: "10px" }}>
          <div><b>${S.tape.kind === "world" ? h.world : S.tape.title || S.tape.name}</b>${S.tape.kind === "world" && !exists ? html` <span class="chip">new</span>` : null}<div class="dim small">${S.tape.kind === "world" ? "This tape's world." : "Your solo tape. New solo games use these settings."} Make a new tape on the shelf for another one.</div></div>
          ${S.tape.kind === "world" ? html`<label class="field"><span>Server memory</span><${Select} options=${[2048, 3072, 4096, 6144, 8192, 12288].map((m) => ({ value: m, label: m / 1024 + " GB" }))} value=${h.memory || 4096} onChange=${(v) => patchHost({ memory: parseInt(v, 10) })} /></label>` : null}
        </div>` : host ? html`<div class="grid" style=${{ gap: "10px" }}>
          <${Select} options=${names.map((n) => ({ value: n, label: n + (w.list.find((x) => x.name === n && x.saved) ? "" : " (new)") }))} value=${h.world} onChange=${(v) => patchHost({ world: v })} />
          <button class="btn sm" onClick=${() => {
            let n = 2; while (names.includes("Knox" + n)) n++;
            const name = prompt("World name", "Knox" + n);
            if (name) patchHost({ world: name.replace(/[^A-Za-z0-9_-]/g, "") }, "New world " + name);
          }}>+ New world</button>
          <label class="field"><span>Server memory</span><${Select} options=${[2048, 3072, 4096, 6144, 8192, 12288].map((m) => ({ value: m, label: m / 1024 + " GB" }))} value=${h.memory || 4096} onChange=${(v) => patchHost({ memory: parseInt(v, 10) })} /></label>
        </div>` : html`<b>${h.world}</b>${h.newWorld ? html` <span class="chip">new</span>` : null}`}
        <div class="sep"></div>
        <${Toggle} title="Wait for everyone" sub="Frozen until all players are in, then a 5 second countdown." on=${h.lobby} disabled=${!host} onChange=${(v) => patchHost({ lobby: v })} />
      </div>
      <div class="card rules-card"><h3>Rules</h3>
        ${QUICK.map((q) => {
          const s = (S.cat.server || []).find((x) => x.name === q.name) || { name: q.name, default: "false" };
          const on = host ? String(value("ini", s, w.data)) === "true" : q.name === "PVP" ? h.pvp : q.name === "SleepAllowed" ? h.sleep : (h.ini || []).some((o) => o.name === q.name && o.value === "true");
          return html`<${Toggle} key=${q.name} title=${q.title} sub=${q.sub} on=${on} disabled=${!host} onChange=${(v) => { setOption("ini", q.name, v ? "true" : "false"); if (q.also) setOption("ini", q.also, v ? "true" : "false"); }} />`;
        })}
      </div>
    </div>
    <h2 class="section">Difficulty</h2>
    <div class="hint" style=${{ marginBottom: "12px" }}>All built from the game's own sandbox options. Tweak anything on the Settings page after.</div>
    ${groups.map((g) => html`<div key=${g.title}><div class="group-title">${g.title}</div>
      <div class="preset-grid">${g.mine && host ? html`<${SavePresetTile} />` : null}${g.list.map((d) => html`<div key=${d.id} class=${"preset" + (h.difficulty === d.id ? " on" : "")} onClick=${() => host && act(() => rpc("applyDifficulty", { id: d.id }), d.name + " applied").then(async (r) => { if (r !== undefined) update({ cfg: await rpc("config") }); })}>
        <div class="row"><b class="grow">${d.name}</b>${h.difficulty === d.id ? html`<span class="chip accent">Chosen</span>` : null}
          ${d.mine && host ? html`<button class="btn sm ghost" title="Delete" onClick=${async (e) => { e.stopPropagation(); if (confirm("Delete the preset " + d.name + "?")) { await act(() => rpc("deleteSettingsPreset", { name: d.name }), "Deleted " + d.name); loadPresets(); } }}>✕</button>` : null}</div>
        <div class="tagline">${d.tagline}</div>
        <p>${d.explain}</p>
        ${d.base && d.base !== d.id ? html`<div class="dim small">Based on: ${(diffs.find((x) => x.id === d.base) || {}).name || d.base}</div>` : null}
      </div>`)}</div></div>`)}`;
}

// SavePresetTile saves the current world and server settings as your own preset.
function SavePresetTile() {
  const [name, setName] = useState("");
  const save = async () => {
    const n = name.trim();
    if (!n) return toast("Give the preset a name.", true);
    if ((await act(() => rpc("saveSettingsPreset", { name: n }), "Saved " + n)) !== undefined) {
      setName("");
      await loadPresets();
      update({ cfg: await rpc("config") });
    }
  };
  return html`<div class="preset save-tile">
    <b>Save your settings</b>
    <div class="tagline">Everything you set here and on the Settings page.</div>
    <div class="row" style=${{ marginTop: "10px" }}><input type="text" class="grow" placeholder="Preset name" value=${name} onInput=${(e) => setName(e.target.value)} onKeyDown=${(e) => e.key === "Enter" && save()} />
      <button class="btn sm primary" onClick=${save}>Save</button></div>
  </div>`;
}

// Settings: every server and sandbox option ------------------------------------------------------

const HIDDEN_INI = ["Mods", "WorkshopItems", "Map", "SpawnPoint", "Password", "Public", "PublicName", "DefaultPort", "UDPPort", "ResetID", "ServerPlayerID"];
const COMMON_INI = ["PVP", "PauseEmpty", "MaxPlayers", "SleepAllowed", "SleepNeeded", "GlobalChat", "SafetySystem", "PlayerSafehouse", "ServerWelcomeMessage", "SpawnItems"];

export function Settings() {
  const host = isHost();
  const w = useWorld();
  const [page, setPage] = useState(0);
  const [q, setQ] = useState("");
  const [onlyChanged, setOnlyChanged] = useState(false);
  const [allServer, setAllServer] = useState(false);
  if (!S.cat) return null;
  if (!host) {
    const info = S.state.info || {};
    return html`<${PageHead} title="Settings" sub="What the host changed from the preset." /><${HostOnly} what="world settings" />
      <div class="card">${[...(info.sandbox || []), ...(info.ini || [])].map((o) => html`<div key=${o.name} class="setting"><div class="sl"><b>${labelOf(o.name)}</b><small>${o.name}</small></div><div>${shownValue(o.name, o.value)}</div></div>`)}
      ${!(info.sandbox || []).length && !(info.ini || []).length ? html`<div class="hint">Nothing changed from the preset.</div>` : null}</div>`;
  }
  const pages = [{ name: "Server", server: true, settings: S.cat.server || [] }, ...(S.cat.sandbox || [])];
  const ql = q.toLowerCase();
  const kindOf = (p) => (p.server ? "ini" : "sandbox");
  const changedCount = Object.keys(S.cfg.host.sandbox).length + Object.keys(S.cfg.host.ini).length;
  let rows = [];
  if (ql || onlyChanged) {
    for (const p of pages) for (const s of p.settings) {
      const over = p.server ? S.cfg.host.ini : S.cfg.host.sandbox;
      if (ql && !((s.label || s.name).toLowerCase().includes(ql) || s.name.toLowerCase().includes(ql))) continue;
      if (onlyChanged && over[s.name] == null) continue;
      if (p.server && HIDDEN_INI.includes(s.name)) continue;
      rows.push(html`<${Setting} key=${kindOf(p) + s.name} s=${s} kind=${kindOf(p)} w=${w.data} page=${p.name} />`);
    }
  } else {
    const p = pages[page] || pages[0];
    if (p.server) {
      rows.push(html`<div class="setting-title" key="t1">Most used</div>`);
      for (const n of COMMON_INI) { const s = p.settings.find((x) => x.name === n); if (s) rows.push(html`<${Setting} key=${"ini" + n} s=${s} kind="ini" w=${w.data} />`); }
      const rest = p.settings.filter((s) => !COMMON_INI.includes(s.name) && !HIDDEN_INI.includes(s.name));
      if (allServer) {
        rows.push(html`<div class="setting-title" key="t2">Everything else</div>`);
        for (const s of rest) rows.push(html`<${Setting} key=${"ini" + s.name} s=${s} kind="ini" w=${w.data} />`);
      } else {
        rows.push(html`<button key="more" class="btn" style=${{ marginTop: "14px" }} onClick=${() => setAllServer(true)}>Show ${rest.length} more server settings</button>`);
      }
    } else {
      for (const s of p.settings) {
        if (s.title) rows.push(html`<div class="setting-title" key=${"t" + s.name}>${s.title}</div>`);
        rows.push(html`<${Setting} key=${"sb" + s.name} s=${s} kind="sandbox" w=${w.data} />`);
      }
    }
  }
  return html`
    <${PageHead} title="Settings" sub="Every server and sandbox option, mods included. Changed ones are marked.">
      <label class="row small muted"><${Switch} on=${onlyChanged} onChange=${setOnlyChanged} /> Only changed (${changedCount})</label>
      <button class="btn" onClick=${() => { update({ tab: "world" }); setTimeout(() => { const el = document.querySelector(".save-tile"); if (el) { el.scrollIntoView({ block: "center", behavior: "smooth" }); const i = el.querySelector("input"); if (i) i.focus(); } }, 200); }}>Save as preset</button>
    </${PageHead}>
    <div class="set-layout">
      <div class="set-pages">
        <input type="text" placeholder="Search settings" value=${q} style=${{ marginBottom: "8px" }} onInput=${(e) => setQ(e.target.value)} />
        ${pages.map((p, i) => html`<div key=${p.name} class=${"set-page" + (page === i && !q && !onlyChanged ? " on" : "")} onClick=${() => { setPage(i); setQ(""); setOnlyChanged(false); }}>${p.name}${p.mod ? html` <span class="dim small">mod</span>` : null}</div>`)}
      </div>
      <div class="card">${rows.length ? rows : html`<div class="hint">Nothing here.</div>`}</div>
    </div>`;
}

function shownValue(name, v) {
  for (const p of [{ settings: S.cat.server || [] }, ...(S.cat.sandbox || [])]) for (const s of p.settings) {
    if (s.name !== name) continue;
    if ((s.optType || s.type) === "enum" && s.values && s.values[parseInt(v, 10) - 1]) return s.values[parseInt(v, 10) - 1];
    if (v === "true") return "On";
    if (v === "false") return "Off";
  }
  return v;
}

function labelOf(name) {
  for (const p of [{ settings: S.cat.server || [] }, ...(S.cat.sandbox || [])]) for (const s of p.settings) if (s.name === name) return s.label || name;
  return name;
}

function Setting({ s, kind, w, page }) {
  const over = kind === "ini" ? S.cfg.host.ini : S.cfg.host.sandbox;
  const v = value(kind, s, w);
  const type = kind === "ini" ? s.type : s.optType || s.type;
  const changed = over[s.name] != null && (!w || String((kind === "ini" ? w.ini : w.sandbox)[s.name] ?? s.default) !== String(over[s.name]));
  const set = (x) => setOption(kind, s.name, x);
  let control;
  if (type === "boolean" || type === "checkbox") control = html`<${Switch} on=${String(v) === "true"} onChange=${(on) => set(on ? "true" : "false")} />`;
  else if (type === "enum") control = html`<${Select} options=${(s.values || []).map((x, i) => ({ value: String(i + 1), label: x }))} value=${String(v)} onChange=${set} />`;
  else if (type === "integer" || type === "double" || type === "entry") control = html`<input type="number" value=${v} step=${type === "integer" ? "1" : "any"} min=${s.min} max=${s.max} onInput=${(e) => set(e.target.value)} />`;
  else control = html`<input type="text" value=${v} onInput=${(e) => set(e.target.value)} />`;
  return html`<div class=${"setting" + (changed ? " changed" : "")}>
    <div class="sl"><b>${pretty(s)}</b>${page ? html`<small class="dim">${page}</small>` : null}${s.tooltip ? html`<small class="tip" title=${stripTags(String(s.tooltip).replace(/\\n/g, "\n"))}>${stripTags(String(s.tooltip).replace(/\\n/g, "\n"))}</small>` : null}</div>
    <div class="row" style=${{ flexWrap: "nowrap" }}>${control}${over[s.name] != null ? html`<button class="btn sm ghost" title="Reset" onClick=${() => set("")}>↺</button>` : null}</div>
  </div>`;
}

// pretty turns a raw option name like "PauseEmpty" into "Pause Empty" when the game has no label for it.
function pretty(s) {
  const l = s.label && s.label !== s.name ? s.label : s.name.split(".").pop().replace(/([a-z])([A-Z])/g, "$1 $2").replace(/([A-Z]+)([A-Z][a-z])/g, "$1 $2");
  return l;
}
