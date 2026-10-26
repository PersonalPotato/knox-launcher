// Mods page: a live list of everything installed. Subscribing or unsubscribing on the Workshop shows
// up here by itself, and a mod can only be on together with the mods it needs.
import { html, useState, useEffect, S, update, rpc, act, toast, TOKEN, PageHead, Select, Switch, HostOnly, isHost, inLobby } from "./core.js";

function chosen() {
  const h = S.cfg.host;
  const pack = h.pack && S.cfg.packs[h.pack];
  return new Set(pack || h.mods || []);
}

function workshopSearch(text) {
  rpc("openURL", { url: "steam://openurl/https://steamcommunity.com/workshop/browse/?appid=108600&searchtext=" + encodeURIComponent(text) });
}

async function setMod(m, on) {
  const name = (id) => (S.cat.modById[id] || {}).name || id;
  try {
    const ch = await rpc("setMod", { id: m.id, on });
    S.cfg.host.pack && S.cfg.packs[S.cfg.host.pack] ? (S.cfg.packs[S.cfg.host.pack] = ch.mods) : (S.cfg.host.mods = ch.mods);
    S.editedAt = Date.now();
    update();
    if (ch.added && ch.added.length) toast(`${m.name} needs ${ch.added.map(name).join(", ")}, so that's on too.`);
    else if (ch.removed && ch.removed.length) toast(`Also turned off ${ch.removed.map(name).join(", ")}: they need ${m.name}.`);
    refreshOrder();
  } catch (e) {
    toast(e.message, true);
  }
}

let orderCache = null;
async function refreshOrder() {
  orderCache = await act(() => rpc("hostMods"));
  update();
}

export function Mods() {
  const [q, setQ] = useState("");
  const [show, setShow] = useState("all");
  useEffect(() => { refreshOrder(); }, [S.cat && S.cat.version, S.cfg.host.pack]);
  if (!S.cat) return null;
  if (!isHost()) return html`<${GuestMods} />`;
  const on = chosen();
  const order = (orderCache && orderCache.mods) || [];
  const problems = (orderCache && orderCache.problems) || {};
  const inOrder = new Set(order);
  const mods = S.cat.mods.slice().sort((a, b) => a.name.localeCompare(b.name));
  const needers = {};
  for (const m of mods) for (const r of m.require || []) (needers[r] = needers[r] || []).push(m);
  const ql = q.toLowerCase();
  const list = mods.filter((m) => {
    if (ql && !(m.name.toLowerCase().includes(ql) || m.id.toLowerCase().includes(ql) || (m.author || "").toLowerCase().includes(ql))) return false;
    if (show === "on") return inOrder.has(m.id);
    if (show === "off") return !inOrder.has(m.id);
    if (show === "problems") return m.broken || problems[m.id] || (m.require || []).some((r) => !S.cat.modById[r]);
    return true;
  });
  const view = S.modsView || "mine";
  const folder = S.modFolder || "";
  const setView = (v) => update({ modsView: v });
  const inFolder = folder === "" ? list : folder === "-" ? list.filter((m) => !folderOf(m.id)) : list.filter((m) => folderOf(m.id) === folder);
  const h = S.cfg.host;
  return html`
    <div class="mods-top">
      <div class="seg">
        <button class=${view === "mine" ? "on" : ""} onClick=${() => setView("mine")}>${h.pack ? h.pack : "My mods"} · ${order.length}</button>
        <button class=${view === "workshop" ? "on" : ""} onClick=${() => setView("workshop")}>Workshop</button>
        <button class=${view === "packs" ? "on" : ""} onClick=${() => setView("packs")}>Packs</button>
      </div>
      ${view === "mine" ? html`<input type="text" class="grow big-search" placeholder="Search your mods" value=${q} onInput=${(e) => setQ(e.target.value)} />` : html`<div class="grow"></div>`}
      ${view === "mine" ? html`<button class="btn" onClick=${() => update({ modal: { kind: "order", order, problems } })}>Load order${Object.keys(problems).length ? ` (${Object.keys(problems).length} left out)` : ""}</button>` : null}
    </div>
    ${view === "packs" ? html`<div class="card row comm-link"><span class="grow">Find packs other players made.</span><button class="btn sm" onClick=${() => update({ tab: "community", commView: "packs" })}>Community packs</button></div><div class="card"><${PacksWindow} /></div>` : view === "workshop" ? html`<${Workshop} />` : html`
    <${JavaBanner} />
    <${FolderTiles} mods=${mods} inOrder=${inOrder} folder=${folder} />
    <div class="row mods-filter">
      <div class="pill-tabs">${[["all", "All"], ["on", "On"], ["off", "Off"], ["problems", "Problems"]].map(([k, l]) => html`<button key=${k} class=${show === k ? "on" : ""} onClick=${() => setShow(k)}>${l}</button>`)}</div>
      <span class="grow muted">${inFolder.length} shown · ${order.length} of ${mods.length} on</span>
      ${folder ? null : html`
      <button class="btn sm" onClick=${async () => { await act(() => rpc("setMods", { mods: mods.filter((m) => !m.broken && !m.pending).map((m) => m.id) }), "Everything that can load is on"); const cfg = await rpc("config"); update({ cfg }); refreshOrder(); }}>All on</button>
      <button class="btn sm ghost" onClick=${async () => { await act(() => rpc("setMods", { mods: [] }), "All off (Knox Link stays)"); const cfg = await rpc("config"); update({ cfg }); refreshOrder(); }}>All off</button>`}
    </div>
    <div class="mod-list">
      ${(folder ? [{ name: null, mods: inFolder }] : groupByFolder(list)).map((g) => html`<div key=${g.name ?? "-"} class=${"folder" + (g.loose ? " loose" : "")}>
        ${g.name !== null || (g.loose && folderNames().length) ? html`<${FolderHead} g=${g} inOrder=${inOrder} />` : null}
        ${!(g.name !== null || g.loose) || !S.closedFolders[g.name ?? "-"] ? g.mods.map((m) => html`<${ModRow} key=${m.id} m=${m} on=${on.has(m.id) || inOrder.has(m.id)} loads=${inOrder.has(m.id)} problem=${problems[m.id]} needers=${needers[m.id] || []} />`) : null}
      </div>`)}
      ${!inFolder.length ? html`<div class="hint" style=${{ padding: "20px" }}>Nothing matches.</div>` : null}
    </div>`}`;
}

export function OrderWindow({ order, problems }) {
  return html`<div>
    <div class="hint" style=${{ marginBottom: "8px" }}>Worked out for you: every mod loads after the mods it needs.</div>
    <ol class="order">${order.map((id) => html`<li key=${id}>${(S.cat.modById[id] || {}).name || id}</li>`)}</ol>
    ${Object.keys(problems).length ? html`<div class="sep"></div><h3>Left out</h3>${Object.entries(problems).map(([id, why]) => html`<div key=${id} class="small" style=${{ marginBottom: "6px" }}><b>${(S.cat.modById[id] || {}).name || id}</b><div class="muted">${why}</div></div>`)}` : null}
  </div>`;
}

function initials(name) {
  const words = name.replace(/\[[^\]]*\]|\([^)]*\)/g, "").split(/[\s\-:_]+/).filter((w) => /^[A-Za-z]/.test(w));
  return ((words[0] || name)[0] + (words[1] ? words[1][0] : "")).toUpperCase();
}

function tileColor(name) {
  let h = 0;
  for (const ch of name) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  const hue = h % 360;
  return `linear-gradient(135deg, hsl(${hue} 35% 26%), hsl(${(hue + 40) % 360} 30% 16%))`;
}

function ModRow({ m, on, loads, problem, needers }) {
  const missing = (m.require || []).filter((r) => !S.cat.modById[r]);
  const [poster, setPoster] = useState(true);
  const status = m.broken ? html`<span class="chip bad">${m.broken}</span>`
    : m.pending ? html`<span class="chip warn">Downloading...</span>`
    : missing.length ? html`<span class="chip bad">Needs ${missing.join(", ")}</span>` : problem ? html`<span class="chip bad">${problem}</span>` : null;
  const turnOnBlocked = m.broken || m.pending || missing.length;
  return html`<div class=${"mod" + (loads ? "" : " off")}>
    <div class="poster" style=${poster && m.posters && m.posters.length ? { backgroundImage: `url(/poster?t=${TOKEN}&mod=${encodeURIComponent(m.id)})` } : { background: tileColor(m.name) }}>
      ${poster && m.posters && m.posters.length ? html`<img src=${`/poster?t=${TOKEN}&mod=${encodeURIComponent(m.id)}`} style=${{ display: "none" }} onError=${() => setPoster(false)} />` : initials(m.name)}
    </div>
    <div style=${{ minWidth: 0 }}>
      <div class="mn">${m.name}</div>
      <div class="mid">${m.id}${m.author ? " · by " + m.author : ""}${m.version ? " · v" + m.version : ""}</div>
      <div class="chips">
        <span class="chip">${m.source === "workshop" ? "Workshop" : m.source === "upload" ? "Your upload" : "Local"}</span>
        ${m.java ? html`<span class="chip warn" title="Has Java code (loads through ZombieBuddy)">Java</span>` : null}
        ${(m.require || []).filter((r) => S.cat.modById[r]).map((r) => html`<span key=${r} class="chip" title="Needs this mod">needs ${S.cat.modById[r].name}</span>`)}
        ${needers.length ? html`<span class="chip accent" title=${needers.map((x) => x.name).join(", ")}>${needers.length} mod${needers.length > 1 ? "s" : ""} need this</span>` : null}
        ${status}
        ${missing.map((r) => html`<button key=${r} class="btn sm" onClick=${() => workshopSearch(r)}>Find ${r} on the Workshop</button>`)}
      </div>
    </div>
    <div class="row" style=${{ flexWrap: "nowrap" }}>
      <select class="folder-pick hover-act" title="Folder" value=${folderOf(m.id) || ""} onChange=${(e) => moveToFolder(m.id, e.target.value)}>
        <option value="">No folder</option>
        ${folderNames().map((f) => html`<option key=${f} value=${f}>${f}</option>`)}
      </select>
      ${m.workshopID ? html`<button class="btn sm ghost hover-act" title="Workshop page" onClick=${() => rpc("openURL", { url: "steam://url/CommunityFilePage/" + m.workshopID })}>Page</button>` : null}
      ${m.workshopID && m.source === "workshop" ? html`<button class="btn sm ghost danger hover-act" title="Unsubscribe on Steam" onClick=${() => { if (confirm(`Unsubscribe from ${m.name}? Steam removes it, and it disappears from this list.`)) act(() => rpc("unsubscribe", { id: m.workshopID }), "Unsubscribed. Steam removes it in a moment."); }}>Unsubscribe</button>` : null}
      <${Switch} on=${loads} disabled=${m.id === "KnoxLink" || (!loads && turnOnBlocked)} onChange=${(v) => setMod(m, v)} />
    </div>
  </div>`;
}

function GuestMods() {
  const info = S.state.info;
  const mine = new Set(S.cat.mods.map((m) => m.id));
  const mods = (info && info.mods) || [];
  const have = mods.filter((m) => mine.has(m.id)).length;
  return html`<${PageHead} title="Mods" sub=${`Host runs ${mods.length} mods. You have ${have}, the rest download on START.`} />
    <${HostOnly} what="mods" />
    <div class="mod-list">${mods.map((m) => html`<div key=${m.id} class="mod">
      <div class="poster">${m.name.slice(0, 1)}</div>
      <div><div class="mn">${m.name}</div><div class="mid">${m.id}</div></div>
      <div>${mine.has(m.id) ? html`<span class="chip good">Installed</span>` : m.ws ? html`<span class="chip warn">Downloads at START</span>` : html`<span class="chip bad">Not on the Workshop</span>`}</div>
    </div>`)}</div>`;
}

// Mod packs window: save, switch, rename, delete, share as a code or into the lobby, import.
const reloadCfg = async () => { const cfg = await rpc("config"); update({ cfg }); refreshOrder(); };
const subscribeURL = (ws) => "steam://url/CommonFilesPage/" + ws;

export function PacksWindow() {
  const [name, setName] = useState("");
  const [code, setCode] = useState("");
  const [shown, setShown] = useState(null); // {name, code}
  const [renaming, setRenaming] = useState(null);
  const [result, setResult] = useState(null); // last import
  const packs = Object.keys(S.cfg.packs || {}).sort();
  const cur = S.cfg.host.pack || "";
  const host = isHost() || !inLobby();
  const shared = ((S.state && S.state.members) || []).filter((m) => !m.me && m.pack);

  const save = async () => {
    const n = name.trim();
    if (!n) return toast("Give the pack a name.", true);
    if ((await act(() => rpc("savePack", { name: n }), "Saved " + n)) !== undefined) { setName(""); reloadCfg(); }
  };
  const copyCode = async (p) => {
    const c = await act(() => rpc("packCode", { name: p }));
    if (!c) return;
    setShown({ name: p, code: c });
    try { await navigator.clipboard.writeText(c); toast("Code for " + p + " copied. Paste it to a friend."); } catch { toast("Copy the code below."); }
  };
  const doImport = async (c, from) => {
    const r = await act(() => rpc("importPack", { code: c }));
    if (!r) return;
    if (from) rpc("packAdded", { id: from }).catch(() => {}); // Knox level XP for whoever shared it
    setResult(r); setCode("");
    toast(`Added ${r.name}${r.missing && r.missing.length ? `. You're missing ${r.missing.length} mod${r.missing.length > 1 ? "s" : ""}.` : "."}`);
    reloadCfg();
  };

  return html`<div class="packs">
    ${host ? html`<div class="pack-save row">
      <input type="text" class="grow" placeholder="Name a new pack (saves the mods that are on now)" value=${name} onInput=${(e) => setName(e.target.value)} onKeyDown=${(e) => e.key === "Enter" && save()} />
      <button class="btn primary" onClick=${save}>Save pack</button>
    </div>` : null}

    <div class="pack-list">
      <div class=${"pack-row" + (cur === "" ? " on" : "")}>
        <div class="grow"><b>My mods</b><div class="dim small">${(S.cfg.host.mods || []).length} mods · what you switch on in the list</div></div>
        ${cur === "" ? html`<span class="chip accent">In use</span>` : host ? html`<button class="btn sm" onClick=${async () => { await act(() => rpc("usePack", { name: "" })); reloadCfg(); }}>Use</button>` : null}
      </div>
      ${packs.map((p) => html`<div key=${p} class=${"pack-row" + (cur === p ? " on" : "")}>
        <div class="grow">${renaming === p
          ? html`<input type="text" autofocus value=${p} onKeyDown=${async (e) => { if (e.key === "Enter") { await act(() => rpc("renamePack", { name: p, text: e.target.value })); setRenaming(null); reloadCfg(); } if (e.key === "Escape") setRenaming(null); }} />`
          : html`<b>${p}</b>`}
          <div class="dim small">${S.cfg.packs[p].length} mods</div></div>
        ${cur === p ? html`<span class="chip accent">In use</span>` : host ? html`<button class="btn sm" onClick=${async () => { await act(() => rpc("usePack", { name: p })); reloadCfg(); }}>Use</button>` : null}
        <button class="btn sm ghost" onClick=${() => copyCode(p)}>Copy code</button>
        ${inLobby() ? html`<button class="btn sm ghost" onClick=${() => act(() => rpc("sharePack", { name: p }), "Sent " + p + " to the lobby")}>Send to lobby</button>` : null}
        <button class="btn sm ghost" onClick=${() => setRenaming(p)}>Rename</button>
        <button class="btn sm ghost" onClick=${async () => { if (confirm("Delete the pack " + p + "?")) { await act(() => rpc("deletePack", { name: p }), "Deleted " + p); reloadCfg(); } }}>Delete</button>
      </div>`)}
    </div>
    ${shown ? html`<div class="pack-code"><div class="dim small">Code for ${shown.name}</div><textarea readonly rows="3" onFocus=${(e) => e.target.select()}>${shown.code}</textarea></div>` : null}

    ${shared.length ? html`<h3 class="pack-h">Shared in this lobby</h3>
      ${shared.map((m) => { const p = m.pack; return html`<div key=${m.id} class="pack-row">
        <div class="grow"><b>${p.n}</b><div class="dim small">From ${m.name} · ${p.m.length} mods</div></div>
        <button class="btn sm" onClick=${() => doImport(JSON.stringify(p), m.id)}>Add to my packs</button>
      </div>`; })}` : null}

    <h3 class="pack-h">Add a pack from a friend</h3>
    <div class="row"><input type="text" class="grow" placeholder="Paste a KNOXPACK code" value=${code} onInput=${(e) => setCode(e.target.value)} onKeyDown=${(e) => e.key === "Enter" && doImport(code)} />
      <button class="btn" disabled=${!code.trim()} onClick=${() => doImport(code)}>Add</button></div>
    ${result && result.missing && result.missing.length ? html`<div class="pack-missing">
      <div class="row"><b class="grow">${result.name} needs ${result.missing.length} mod${result.missing.length > 1 ? "s" : ""} you don't have</b>
        <button class="btn sm" onClick=${() => result.missing.filter((m) => m.w).forEach((m, i) => setTimeout(() => rpc("openURL", { url: subscribeURL(m.w) }), i * 400))}>Open all on Workshop</button></div>
      ${result.missing.map((m) => html`<div key=${m.i} class="row small"><span class="grow">${m.i}</span>${m.w ? html`<button class="btn sm ghost" onClick=${() => rpc("openURL", { url: subscribeURL(m.w) })}>Workshop page</button>` : html`<span class="dim">Not on the Workshop</span>`}</div>`)}
    </div>` : null}
  </div>`;
}

// Mod folders: only for tidying this page; they don't change what loads.
S.closedFolders = S.closedFolders || {};
const folderNames = () => Object.keys(S.cfg.modFolders || {}).sort((a, b) => a.localeCompare(b));
function folderOf(id) {
  for (const [f, ids] of Object.entries(S.cfg.modFolders || {})) if ((ids || []).includes(id)) return f;
  return null;
}
async function moveToFolder(id, name) {
  await act(() => rpc("setModFolder", { id, name }));
  update({ cfg: await rpc("config") });
}

// groupByFolder: folders A-Z, then the mods in no folder (name null). Empty folders still show.
function groupByFolder(list) {
  const groups = new Map(folderNames().map((f) => [f, { name: f, mods: [] }]));
  const loose = { name: null, loose: true, mods: [] };
  for (const m of list) {
    const f = folderOf(m.id);
    (f && groups.has(f) ? groups.get(f) : loose).mods.push(m);
  }
  const out = [...groups.values()];
  return [...out, loose].filter((g) => g.mods.length || g.name !== null);
}

function FolderHead({ g, inOrder }) {
  const [renaming, setRenaming] = useState(false);
  const onCount = g.mods.filter((m) => inOrder.has(m.id)).length;
  const key = g.name ?? "-", label = g.loose ? "No folder" : g.name;
  const closed = !!S.closedFolders[key];
  const setAll = async (v) => {
    const cur = chosen();
    for (const m of g.mods) if (v && !m.broken && !m.pending) cur.add(m.id); else if (!v) cur.delete(m.id);
    await act(() => rpc("setMods", { mods: [...cur] }), v ? `${label}: all on` : `${label}: all off`);
    reloadCfg();
  };
  return html`<div class=${"folder-head" + (closed ? " closed" : "")}>
    <button class="folder-toggle" onClick=${() => { S.closedFolders[key] = !closed; update(); }}>
      <span class="caret">${closed ? "▸" : "▾"}</span>
      ${renaming ? html`<input type="text" autofocus value=${g.name} onClick=${(e) => e.stopPropagation()} onKeyDown=${async (e) => {
          if (e.key === "Enter") { await act(() => rpc("renameModFolder", { name: g.name, text: e.target.value })); setRenaming(false); update({ cfg: await rpc("config") }); }
          if (e.key === "Escape") setRenaming(false);
        }} />` : html`<b>${label}</b>`}
      <span class="muted small">${onCount} of ${g.mods.length} on</span>
    </button>
    <div class="row hover-act">
      <button class="btn sm ghost" onClick=${() => setAll(true)}>All on</button>
      <button class="btn sm ghost" onClick=${() => setAll(false)}>All off</button>
      ${g.loose ? null : html`<button class="btn sm ghost" onClick=${() => setRenaming(true)}>Rename</button>`}
      ${g.loose ? null : html`<button class="btn sm ghost" onClick=${async () => { if (confirm(`Delete the folder ${g.name}? The mods stay installed.`)) { await act(() => rpc("deleteModFolder", { name: g.name })); update({ cfg: await rpc("config") }); } }}>Delete</button>`}
    </div>
  </div>`;
}

// FolderTiles: All, each folder, mods in no folder, and New folder. Picking one filters the list.
function FolderTiles({ mods, inOrder, folder }) {
  const names = folderNames();
  const pick = (f) => update({ modFolder: f });
  const byName = {};
  for (const m of mods) byName[m.name.replace(/\s*\[[^\]]*\]\s*/g, " ").trim()] = m;
  const loose = mods.filter((m) => !folderOf(m.id));
  const count = (ms) => `${ms.filter((m) => inOrder.has(m.id)).length} of ${ms.length} on`;
  const make = async () => {
    const n = (prompt("Name the new folder") || "").trim();
    if (!n) return;
    await act(() => rpc("newModFolder", { name: n }), "Folder " + n + " made");
    update({ cfg: await rpc("config"), modFolder: n });
  };
  const tile = (key, label, ms, art) => html`<button key=${key} class=${"ftile" + (folder === key ? " on" : "")} onClick=${() => pick(key)}>
    <span class="fart" style=${{ background: (art && art.posters && art.posters.length ? `url(/poster?t=${TOKEN}&mod=${encodeURIComponent(art.id)}) center/cover no-repeat, ` : "") + tileColor(label) }}>${art && art.posters && art.posters.length ? null : initials(label)}</span>
    <b>${label}</b><small>${count(ms)}</small></button>`;
  const cur = folder && folder !== "-" ? folder : null;
  const curMods = cur ? mods.filter((m) => folderOf(m.id) === cur) : [];
  return html`<div class="ftiles">
    <button class=${"ftile all" + (folder === "" ? " on" : "")} onClick=${() => pick("")}><span class="fart">ALL</span><b>All mods</b><small>${count(mods)}</small></button>
    ${names.map((f) => tile(f, f, mods.filter((m) => folderOf(m.id) === f), byName[f]))}
    ${names.length && loose.length ? tile("-", "No folder", loose, null) : null}
    <button class="ftile new" onClick=${make}><span class="fart">+</span><b>New folder</b><small>Group mods your way</small></button>
  </div>
  <div class="row folder-acts">
    ${cur ? html`
      <b>${cur}</b>
      <button class="btn sm" onClick=${() => setFolderMods(curMods, true)}>All on</button>
      <button class="btn sm ghost" onClick=${() => setFolderMods(curMods, false)}>All off</button>
      <button class="btn sm ghost" onClick=${async () => { const t = (prompt("Rename " + cur, cur) || "").trim(); if (!t || t === cur) return; await act(() => rpc("renameModFolder", { name: cur, text: t })); update({ cfg: await rpc("config"), modFolder: t }); }}>Rename</button>
      <button class="btn sm ghost" onClick=${async () => { if (confirm(`Delete the folder ${cur}? The mods stay installed.`)) { await act(() => rpc("deleteModFolder", { name: cur })); update({ cfg: await rpc("config"), modFolder: "" }); } }}>Delete folder</button>
      <span class="muted small">Move a mod with the folder menu on its row.</span>` : html`
      <span class="muted small grow">Folders only tidy this page. They don't change what loads.</span>
      <button class="btn sm ghost" onClick=${() => autoSort(mods)}>Sort into folders for me</button>
      ${names.length ? html`<button class="btn sm ghost" onClick=${() => { const all = names.every((f) => S.closedFolders[f]); for (const f of names) S.closedFolders[f] = !all; update(); }}>Open/close all</button>` : null}`}
  </div>`;
}

async function setFolderMods(ms, v) {
  const cur = chosen();
  for (const m of ms) if (v && !m.broken && !m.pending) cur.add(m.id); else if (!v) cur.delete(m.id);
  await act(() => rpc("setMods", { mods: [...cur] }), v ? "All on" : "All off");
  reloadCfg();
}

// autoSort: a "hub" is a mod that 3 or more others build on (Viewpoint, A-Life, ...). Each mod goes in the
// folder of the nearest hub it needs, and a hub goes in its own folder.
async function autoSort(mods) {
  const byId = S.cat.modById;
  const deps = {};
  for (const m of mods) for (const r of m.require || []) deps[r] = (deps[r] || 0) + 1;
  const hub = (id) => (deps[id] || 0) >= 3;
  const key = (id, seen = new Set()) => {
    if (hub(id)) return id;
    if (seen.has(id)) return null;
    seen.add(id);
    const req = ((byId[id] && byId[id].require) || []).filter((r) => byId[r]).sort((x, y) => (deps[y] || 0) - (deps[x] || 0));
    for (const r of req) { const k = key(r, seen); if (k) return k; }
    return null;
  };
  const folders = {};
  for (const m of mods) {
    const k = key(m.id);
    if (!k) continue;
    const fname = ((byId[k] && byId[k].name) || k).replace(/\s*\[[^\]]*\]\s*/g, " ").trim();
    (folders[fname] = folders[fname] || []).push(m.id);
  }
  if (!confirm(`Sort into ${Object.keys(folders).length} folders by the main mod each one needs? Your own folders get replaced.`)) return;
  await act(() => rpc("setModFolders", { folders }), "Sorted into folders");
  update({ cfg: await rpc("config") });
}

// Workshop: browse and subscribe to Project Zomboid mods straight from Steam.
const fmtNum = (n) => (n >= 1e6 ? (n / 1e6).toFixed(1) + "M" : n >= 1e3 ? Math.round(n / 1e3) + "k" : String(n || 0));
S.ws = S.ws || { text: "", sort: "trend", b42: true, items: null, page: 1, total: 0, busy: false, err: "" };

async function wsLoad(more) {
  const w = S.ws;
  const page = more ? w.page + 1 : 1;
  update({ ws: { ...w, busy: true, err: "" } });
  try {
    const r = await rpc("workshop", { text: w.text, sort: w.sort, page, tag: w.b42 ? "Build 42" : "" });
    const items = more ? [...(S.ws.items || []), ...(r.items || [])] : r.items || [];
    update({ ws: { ...S.ws, items, page, total: r.total, busy: false } });
  } catch (e) {
    update({ ws: { ...S.ws, busy: false, err: e.message } });
  }
}

function Workshop() {
  const w = S.ws;
  useEffect(() => { if (!w.items) wsLoad(); }, []);
  const set = (patch) => { S.ws = { ...S.ws, ...patch }; wsLoad(); };
  const have = new Set(S.cat.mods.map((m) => m.workshopID).filter(Boolean));
  return html`<div class="ws">
    <div class="row ws-bar">
      <input type="text" class="grow big-search" placeholder="Search the Workshop (Enter)" value=${w.text} onInput=${(e) => { S.ws.text = e.target.value; update(); }} onKeyDown=${(e) => e.key === "Enter" && wsLoad()} />
      <div class="pill-tabs">${[["trend", "Trending"], ["subs", "Most subscribed"], ["top", "Top rated"], ["new", "Newest"]].map(([k, l]) => html`<button key=${k} class=${w.sort === k && !w.text.trim() ? "on" : ""} onClick=${() => set({ sort: k, text: "" })}>${l}</button>`)}</div>
      <label class="row small muted" style=${{ gap: "8px" }}><${Switch} on=${w.b42} onChange=${(v) => set({ b42: v })} />Build 42 only</label>
    </div>
    ${w.err ? html`<div class="card note-card bad">${w.err}</div>` : null}
    ${w.items && !w.items.length && !w.busy ? html`<div class="hint" style=${{ padding: "20px" }}>Nothing found.</div>` : null}
    <div class="ws-grid">
      ${(w.items || []).map((it) => html`<${WsCard} key=${it.id} it=${it} have=${have.has(it.id)} />`)}
    </div>
    ${w.busy ? html`<div class="row" style=${{ justifyContent: "center", padding: "20px" }}><div class="spinner sm"></div><span class="muted">Asking Steam...</span></div>`
      : w.items && w.items.length < w.total ? html`<div class="row" style=${{ justifyContent: "center", padding: "16px" }}><button class="btn" onClick=${() => wsLoad(true)}>Show more (${fmtNum(w.total - w.items.length)} left)</button></div>` : null}
  </div>`;
}

function WsCard({ it, have }) {
  const [img, setImg] = useState(!!it.preview);
  const votes = it.up + it.down;
  const sub = async () => {
    await act(() => rpc("subscribe", { id: it.id }), "Subscribed to " + it.title + ". Steam is downloading it.");
    it.subscribed = true;
    update();
  };
  const unsub = async () => {
    if (!confirm(`Unsubscribe from ${it.title}?`)) return;
    await act(() => rpc("unsubscribe", { id: it.id }), "Unsubscribed");
    it.subscribed = false;
    update();
  };
  return html`<div class="ws-card">
    <div class="ws-img" style=${img ? null : { background: tileColor(it.title) }}>
      ${img ? html`<img src=${it.preview} alt="" loading="lazy" onError=${() => setImg(false)} />` : html`<span>${initials(it.title)}</span>`}
      ${it.subscribed || have ? html`<span class="ws-got">${have ? "Installed" : "Subscribed"}</span>` : null}
    </div>
    <div class="ws-in">
      <b title=${it.title}>${it.title}</b>
      <div class="ws-meta">${fmtNum(it.subs)} subscribers${votes ? ` · ${Math.round((100 * it.up) / votes)}% liked` : ""}</div>
      <div class="ws-desc">${stripBB(it.desc)}</div>
      <div class="row ws-acts">
        ${it.subscribed || have ? html`<button class="btn sm ghost" onClick=${unsub}>Unsubscribe</button>` : html`<button class="btn sm primary" onClick=${sub}>Subscribe</button>`}
        <button class="btn sm ghost" onClick=${() => rpc("openURL", { url: "steam://url/CommunityFilePage/" + it.id })}>Page</button>
      </div>
    </div>
  </div>`;
}

const stripBB = (s) => String(s || "").replace(/\[\/?[a-z0-9*]+(=[^\]]*)?\]/gi, " ").replace(/\s+/g, " ").trim().slice(0, 160);

// Java mods: ZombieBuddy asks before it runs a mod's Java code. Answer here and the game won't ask.
const javaIds = () => (!isHost() && S.state && S.state.info ? (S.state.info.mods || []).map((m) => m.id) : null);
export async function loadJava() {
  const ids = javaIds();
  const list = await rpc("javaMods", ids ? { mods: ids } : {}).catch(() => null);
  if (list) update({ java: list });
}

// loadZB checks ZombieBuddy (and, every 20 minutes, the Workshop for a newer one). force checks the Workshop now.
export async function loadZB(force) {
  const ids = javaIds();
  const zb = await rpc("zbStatus", { ...(ids ? { mods: ids } : {}), force: !!force }).catch(() => null);
  if (zb) update({ zb });
  return zb;
}

// zbSetup does what the ZombieBuddy window lists. Only runs when the player presses a button.
export async function zbSetup(okMsg) {
  const ids = javaIds();
  const r = await act(() => rpc("zbSetup", ids ? { mods: ids } : {}), okMsg);
  if (r) { update({ zb: r }); loadJava(); }
  return r;
}

// ZBBar sits on top of every page when ZombieBuddy is missing or has an update.
export function ZBBar() {
  const [busy, setBusy] = useState(false);
  const zb = S.zb;
  if (!zb || S.zbLater === zbKey(zb)) return null;
  const java = zb.needed || (S.java || []).length;
  const later = html`<button class="btn ghost" onClick=${() => update({ zbLater: zbKey(zb) })}>Later</button>`;
  if (!zb.ready) {
    return html`<div class=${"java-banner zb-bar" + (java ? " bad" : "")}>
      <div class="tile-ico">⚠</div>
      <div class="grow"><b>${!zb.downloaded ? "You don't have ZombieBuddy" : "ZombieBuddy isn't set up"}${java ? ", so Java mods won't load" : ""}</b>
        <div class="muted small">${java ? "" : "Only needed for Java mods. "}${zb.steps.join(". ")}.</div></div>
      ${java ? null : later}
      <button class="btn primary" onClick=${() => update({ modal: { kind: "zb" } })}>Set it up</button>
    </div>`;
  }
  if (!zb.outdated) return null;
  const run = async () => { setBusy(true); await zbSetup("ZombieBuddy updated"); setBusy(false); };
  const running = S.state && S.state.game && S.state.game.running;
  return html`<div class="java-banner zb-bar">
    <div class="tile-ico">↻</div>
    <div class="grow"><b>ZombieBuddy update ready</b>
      <div class="muted small">${zb.steps.join(". ")}.${running ? " Close the game first." : ""}</div></div>
    ${later}
    <button class="btn ghost" onClick=${() => update({ modal: { kind: "zb" } })}>Details</button>
    <button class="btn primary" disabled=${busy || running} onClick=${run}>${busy ? "Updating..." : "Update"}</button>
  </div>`;
}
const zbKey = (zb) => [zb.ready, zb.outdated, zb.updated].join("|");

export function JavaBanner() {
  const sig = (S.cat && S.cat.version) + "|" + ((S.cfg && S.cfg.host.pack) || "") + "|" + ((S.cfg && (S.cfg.host.mods || []).length) || 0) + "|" + ((S.state && S.state.info && (S.state.info.mods || []).length) || 0);
  const running = S.state && S.state.game && S.state.game.running;
  useEffect(() => { loadJava(); loadZB(); }, [sig, running]);
  const fresh = (S.java || []).filter((j) => j.status === "new");
  if (S.zb && !S.zb.ready && (S.zb.needed || (S.java || []).length)) return null; // the bar on top says it
  if (!fresh.length) return null;
  return html`<div class="java-banner">
    <div class="tile-ico">☕</div>
    <div class="grow"><b>${fresh.length} Java mod${fresh.length > 1 ? "s" : ""} need${fresh.length > 1 ? "" : "s"} your OK</b>
      <div class="muted small">ZombieBuddy asks before it runs a mod's Java code. Answer here and the game won't stop to ask.</div></div>
    <button class="btn primary" onClick=${() => update({ modal: { kind: "java" } })}>Review</button>
  </div>`;
}

export function JavaWindow() {
  useEffect(() => { loadJava(); }, []);
  const list = S.java || [];
  const fresh = list.filter((j) => j.status === "new");
  const set = async (ids, on) => {
    await act(() => rpc("setJava", { mods: ids, on }), on ? "Allowed" : "Blocked");
    loadJava();
  };
  return html`<div class="java-win">
    <p class="muted" style=${{ marginTop: 0 }}>Java mods run code outside the game's Lua sandbox, so they can do anything a program on your PC can. Only allow mods you trust. Your answer is saved for this exact version of each mod; an update asks again.</p>
    ${!list.length ? html`<div class="hint">No Java mods in this mod list.</div>` : null}
    ${list.map((j) => html`<div key=${j.id} class="java-row">
      <div class="grow" style=${{ minWidth: 0 }}><b>${j.name}</b><div class="dim small">${j.author ? "by " + j.author + " · " : ""}${j.jar}${j.workshopID ? "" : " · not from the Workshop"}</div></div>
      ${j.workshopID ? html`<button class="btn sm ghost" onClick=${() => rpc("openURL", { url: "steam://url/CommunityFilePage/" + j.workshopID })}>Page</button>` : null}
      <span class=${"chip " + (j.status === "allowed" ? "good" : j.status === "blocked" ? "bad" : "warn")}>${j.status === "allowed" ? "Allowed" : j.status === "blocked" ? "Blocked" : "Not answered"}</span>
      ${j.status !== "allowed" ? html`<button class="btn sm" onClick=${() => set([j.id], true)}>Allow</button>` : null}
      ${j.status !== "blocked" ? html`<button class="btn sm ghost" onClick=${() => set([j.id], false)}>Block</button>` : null}
    </div>`)}
    ${fresh.length > 1 ? html`<div class="row" style=${{ justifyContent: "flex-end", marginTop: "12px" }}><button class="btn primary" onClick=${() => set(fresh.map((j) => j.id), true)}>Allow the ${fresh.length} not answered</button></div>` : null}
  </div>`;
}

// ZBWindow lists what setup changes, then does it when you press the button.
export function ZBWindow() {
  const [busy, setBusy] = useState(false);
  const [checking, setChecking] = useState(false);
  useEffect(() => { loadZB(); }, []);
  const zb = S.zb;
  if (!zb) return html`<div class="hint">Checking...</div>`;
  const run = async () => {
    setBusy(true);
    await zbSetup(zb.ready ? "ZombieBuddy updated" : "ZombieBuddy is set up");
    setBusy(false);
  };
  const check = async () => {
    setChecking(true);
    const r = await loadZB(true);
    setChecking(false);
    if (r && r.ready && !r.outdated) toast("ZombieBuddy is up to date");
  };
  const label = zb.ready ? (busy ? "Updating..." : "Update") : (busy ? "Setting up..." : "Set it up");
  const row = (ok, text) => html`<div class="java-row"><span class=${"chip " + (ok ? "good" : "warn")}>${ok ? "OK" : "To do"}</span><div class="grow">${text}</div></div>`;
  return html`<div class="java-win">
    <p class="muted" style=${{ marginTop: 0 }}>ZombieBuddy lets Java mods run. It has to start with the game, so it needs two files in the game folder and one line in the game's launch settings. This is the same thing ZombieBuddy's own installer does.</p>
    ${row(zb.downloaded, "ZombieBuddy downloaded from the Workshop")}
    ${zb.downloaded ? row(!zb.update, zb.update ? "A newer ZombieBuddy is on the Workshop" + (zb.updated ? " (" + new Date(zb.updated * 1000).toLocaleDateString() + ")" : "") : zb.busy ? "Steam is downloading it" : "Up to date" + (zb.updated ? " (Workshop version from " + new Date(zb.updated * 1000).toLocaleDateString() + ")" : "")) : null}
    ${row(zb.files === "ok", zb.files === "old" ? "Game folder files are older than the Workshop copy" : "zbNative.dll and ZombieBuddy.jar in the game folder")}
    ${row(zb.launch, "-agentlib:zbNative in ProjectZomboid64.json")}
    ${zb.ready && !zb.outdated ? html`<div class="row" style=${{ marginTop: "12px", gap: "8px" }}>
        <div class="hint grow">All set. Java mods still ask for your OK on the Mods page.</div>
        <button class="btn ghost" disabled=${checking} onClick=${check}>${checking ? "Checking..." : "Check for updates"}</button></div>`
      : html`<div class="row" style=${{ justifyContent: "flex-end", marginTop: "12px", gap: "8px" }}>
        <span class="muted small grow">Close the game first.</span>
        <button class="btn primary" disabled=${busy} onClick=${run}>${label}</button></div>`}
  </div>`;
}
