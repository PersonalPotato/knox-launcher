// Community: stories and mod packs other players uploaded, from the community server, plus made-up examples.
import { html, useState, useEffect, S, update, rpc, act, toast, loadConfig, loadPresets, PageHead, isHost, Switch } from "./core.js";
import { openEditor, copyStory } from "./editor.js";
import { RoleCard } from "./start.js";

const fmtNum = (n) => (n >= 1e6 ? (n / 1e6).toFixed(1) + "M" : n >= 1e4 ? Math.round(n / 1e3) + "k" : n >= 1e3 ? (n / 1e3).toFixed(1) + "k" : String(n || 0));
const when = (d) => (d <= 0 ? "today" : d === 1 ? "yesterday" : d < 14 ? d + " days ago" : d < 60 ? Math.round(d / 7) + " weeks ago" : Math.round(d / 30) + " months ago");
const daysSince = (unix) => Math.floor((Date.now() / 1000 - unix) / 86400);
const players = (b) => (b.min === b.max ? b.min : b.min + "-" + b.max) + (b.max > 1 ? " players" : " player");
const playerCount = () => Math.max(1, (S.state && S.state.members && S.state.members.length) || 1);
const myName = () => (S.state && S.state.me && S.state.me.name) || "You";
const workshopSearch = (q) => `https://steamcommunity.com/workshop/browse/?appid=108600&searchtext=${encodeURIComponent(q)}`;

const STORY_TAGS = [["easy", "Easy"], ["hard", "Hard"], ["town", "Town"], ["wild", "Woods"], ["long", "Long game"]];
const SORTS = [["popular", "Popular"], ["top", "Top rated"], ["new", "Newest"]];

export async function loadCommunity(fresh) {
  const c = await act(() => rpc("community", { fresh: !!fresh }));
  if (c) update({ comm: c });
}

// your uploads show up in the lists too, with no downloads yet
function myStoryUploads() {
  const ups = (S.comm && S.comm.uploads) || [];
  const list = (S.presets && S.presets.backstories) || [];
  return ups.filter((u) => u.kind === "story" && !u.remote).map((u) => {
    const b = list.find((x) => x.id === u.id);
    return b && { ...b, author: myName(), downloads: 0, likes: 0, days: daysSince(u.posted), mine: true, have: true };
  }).filter(Boolean);
}

function myPackUploads() {
  const ups = (S.comm && S.comm.uploads) || [];
  const packs = (S.cfg && S.cfg.packs) || {};
  return ups.filter((u) => u.kind === "pack" && !u.remote && packs[u.id]).map((u) => ({
    id: "mine:" + u.id, name: u.id, author: myName(), about: "", tags: [], downloads: 0, likes: 0, days: daysSince(u.posted), mine: true, have: true,
    mods: packs[u.id].map((id) => { const m = S.cat && S.cat.modById[id]; return { id, name: m ? m.name : id, ws: m ? m.workshopID : "", have: !!m }; }),
  }));
}

function sortList(list, sort) {
  const by = { popular: (a, b) => b.downloads - a.downloads, top: (a, b) => b.likes / (b.downloads + 50) - a.likes / (a.downloads + 50), new: (a, b) => a.days - b.days }[sort];
  return [...list].sort(by);
}

export function Community() {
  useEffect(() => { loadCommunity(); }, []);
  const view = S.commView || "stories";
  const setView = (v) => update({ commView: v });
  const c = S.comm;
  return html`<div class="community">
    <${PageHead} title="Community" sub="Stories and mod packs made by other players.">
      <button class="btn" onClick=${() => update({ modal: { kind: "pasteCode" } })}>Paste a code</button>
      <button class="btn primary" onClick=${() => openEditor()}>New story</button>
    </${PageHead}>
    ${c && c.server && c.server.error ? html`<div class="card note-card bad" style=${{ marginBottom: "12px" }}>${c.server.error}. Showing what was there last time.</div>` : null}
    <div class="row" style=${{ marginBottom: "14px" }}>
      <div class="seg">
        <button class=${view === "stories" ? "on" : ""} onClick=${() => setView("stories")}>Stories${c ? " · " + c.stories.length : ""}</button>
        <button class=${view === "packs" ? "on" : ""} onClick=${() => setView("packs")}>Mod packs${c ? " · " + c.packs.length : ""}</button>
        <button class=${view === "mine" ? "on" : ""} onClick=${() => setView("mine")}>Yours</button>
      </div>
      <span class="grow"></span>
      ${c && c.server.url !== "mock" ? html`<label class="row small muted" style=${{ gap: "8px" }}><${Switch} on=${!c.hideExamples} onChange=${async (v) => { await act(() => rpc("setCommunity", { hideExamples: !v })); loadCommunity(); }} />Show examples</label>` : null}
      <button class="btn sm ghost" onClick=${() => loadCommunity(true)}>Refresh</button>
    </div>
    ${!c ? html`<div class="row" style=${{ justifyContent: "center", padding: "30px" }}><div class="spinner sm"></div></div>`
      : view === "stories" ? html`<${StoryBrowse} />` : view === "packs" ? html`<${PackBrowse} />` : html`<${Yours} />`}
  </div>`;
}

function Bar({ q, setQ, sort, setSort, placeholder, children }) {
  return html`<div class="row ws-bar comm-bar">
    <input type="text" class="grow big-search" placeholder=${placeholder} value=${q} onInput=${(e) => setQ(e.target.value)} />
    <div class="pill-tabs">${SORTS.map(([k, l]) => html`<button key=${k} class=${sort === k ? "on" : ""} onClick=${() => setSort(k)}>${l}</button>`)}</div>
    ${children}
  </div>`;
}

function StoryBrowse() {
  const [q, setQ] = useState("");
  const [sort, setSort] = useState("popular");
  const [tag, setTag] = useState("");
  const [fit, setFit] = useState(false);
  const n = playerCount();
  const all = [...myStoryUploads(), ...S.comm.stories];
  const ql = q.trim().toLowerCase();
  const list = sortList(all.filter((b) => (!tag || (b.tags || []).includes(tag)) && (!fit || (n >= b.min && n <= b.max)) &&
    (!ql || (b.name + " " + b.story + " " + b.author).toLowerCase().includes(ql))), sort);
  return html`<div>
    <${Bar} q=${q} setQ=${setQ} sort=${sort} setSort=${setSort} placeholder="Search stories" />
    <div class="row" style=${{ gap: "6px", flexWrap: "wrap", marginTop: "10px" }}>
      ${STORY_TAGS.map(([id, name]) => html`<button key=${id} class=${"chip click" + (tag === id ? " accent" : "")} onClick=${() => setTag(tag === id ? "" : id)}>${name}</button>`)}
      ${topTags(all, STORY_TAGS.map((t) => t[0])).map((t) => html`<button key=${t} class=${"chip click" + (tag === t ? " accent" : "")} onClick=${() => setTag(tag === t ? "" : t)}>${t}</button>`)}
      <button class=${"chip click" + (fit ? " accent" : "")} onClick=${() => setFit(!fit)}>Fits ${n} player${n > 1 ? "s" : ""}</button>
    </div>
    ${list.length ? null : html`<div class="hint" style=${{ padding: "20px" }}>Nothing found.</div>`}
    <div class="comm-grid">${list.map((b) => html`<${StoryCard} key=${b.id} b=${b} />`)}</div>
  </div>`;
}

// the most used tags people typed themselves
function topTags(list, skip) {
  const n = {};
  for (const x of list) for (const t of x.tags || []) if (!skip.includes(t)) n[t] = (n[t] || 0) + 1;
  return Object.keys(n).sort((a, b) => n[b] - n[a]).slice(0, 12);
}

const tagName = (t) => (STORY_TAGS.find((x) => x[0] === t) || [t, t])[1];

const tint = (s) => {
  let h = 0;
  for (const ch of s || "") h = (h * 31 + ch.charCodeAt(0)) % 360;
  return `linear-gradient(135deg, hsl(${h} 28% 26%), hsl(${(h + 40) % 360} 30% 14%))`;
};

function Stats({ x }) {
  return html`<div class="comm-stats"><span title="Downloads">↓ ${fmtNum(x.downloads)}</span><span title="Likes">▲ ${fmtNum(x.likes)}</span>${x.commentCount ? html`<span title="Comments">💬 ${x.commentCount}</span>` : null}<span>${when(x.days)}</span></div>`;
}

async function getStory(b, use) {
  if (!b.have && (await act(() => rpc("getStory", { id: b.id }))) === undefined) return false;
  b.have = true;
  await Promise.all([loadPresets(), loadCommunity(), loadConfig()]);
  if (use) {
    if ((await act(() => rpc("setBackstory", { id: b.id }))) === undefined) return false;
    await loadConfig();
    toast(b.name + " picked");
  } else toast(b.name + " added to your stories");
  return true;
}

function StoryCard({ b }) {
  const bld = S.presets && S.presets.buildings.find((x) => x.id === b.building);
  return html`<div class="comm-card" onClick=${() => update({ modal: { kind: "cstory", id: b.id } })}>
    <div class="comm-banner" style=${{ background: tint(b.name) }}>
      <span class="comm-place">${bld ? bld.name : b.building}</span>
      <span class="comm-players">${players(b)}</span>
      ${b.mine ? html`<span class="ws-got">Your upload</span>` : b.have ? html`<span class="ws-got">Got it</span>` : null}
      ${b.example ? html`<span class="comm-example">Example</span>` : null}
    </div>
    <div class="ws-in">
      <b title=${b.name}>${b.name}</b>
      <div class="dim small">by ${b.author}</div>
      <div class="ws-desc">${b.story}</div>
      <${Stats} x=${b} />
    </div>
  </div>`;
}

export function CommunityStoryDetail({ id, close }) {
  const b = [...myStoryUploads(), ...((S.comm && S.comm.stories) || [])].find((x) => x.id === id);
  if (!b) return null;
  const P = S.presets;
  const bld = P.buildings.find((x) => x.id === b.building);
  const lo = P.loadouts.find((x) => x.id === b.loadout);
  const like = async () => { await act(() => rpc("like", { id: b.id })); loadCommunity(); };
  // your own upload is already in your stories under its own id
  const own = b.mine && ((S.comm.uploads || []).find((u) => u.kind === "story" && "r-" + u.remote === b.id) || {}).id;
  return html`<div class="story-detail">
    <div class="row" style=${{ gap: "10px", flexWrap: "wrap" }}><span class="dim">by <b>${b.author}</b></span><${Stats} x=${b} /><span class="chip">${players(b)}</span>${(b.tags || []).map((t) => html`<span key=${t} class="chip">${tagName(t)}</span>`)}</div>
    <p class="story-text">${b.story}</p>
    <div class="facts">
      <div><span class="dim">Start</span><b>${bld ? bld.name : b.building}${b.town ? " in " + b.town : ""}</b></div>
      <div><span class="dim">Loadout</span><b>${lo ? lo.name : "The host's"}</b>${lo && lo.items && lo.items.length ? html`<small class="dim">${lo.explain}</small>` : null}</div>
    </div>
    <h3>Roles</h3>
    <div class="grid-roles">${b.roles.map((r, i) => html`<${RoleCard} key=${i} r=${r} />`)}</div>
    ${b.tip ? html`<div class="hint">${b.tip}</div>` : null}
    <${Comments} x=${b} />
    <div class="row" style=${{ marginTop: "14px", justifyContent: "flex-end", flexWrap: "wrap" }}>
      <${ModTools} x=${b} close=${close} />
      ${b.mine ? null : html`<button class=${"btn ghost" + (b.liked ? " liked" : "")} onClick=${like}>▲ ${b.liked ? "Liked" : "Like"}</button>`}
      <button class="btn ghost" onClick=${() => openEditor(copyStory(b))}>Copy and edit</button>
      ${own ? html`<span class="chip good">Your story</span>` : b.have ? html`<span class="chip good">In your stories</span>` : html`<button class="btn" onClick=${() => getStory(b)}>Get</button>`}
      ${isHost() ? html`<button class="btn primary" onClick=${async () => { if (await getStory(own ? { ...b, id: own, have: true } : b, true)) close(); }}>${b.have || own ? "Use this story" : "Get and use"}</button>` : null}
    </div>
  </div>`;
}

// Comments: made-up ones for examples, live ones (with a box to write your own) for real uploads.
function Comments({ x }) {
  const [list, setList] = useState(x.remote ? null : (x.comments || []).map((c) => ({ ...c, ago: when(c.days) })));
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const me = S.comm && S.comm.server && S.comm.server.me;
  const load = async () => {
    const r = await act(() => rpc("comments", { id: x.id }));
    if (r) setList(r.map((c) => ({ ...c, ago: when(daysSince(c.created)) })));
  };
  useEffect(() => { if (x.remote) load(); }, [x.id]);
  const send = async () => {
    setBusy(true);
    const ok = await act(() => rpc("comment", { id: x.id, text }));
    setBusy(false);
    if (ok === undefined) return;
    setText("");
    load();
    loadCommunity();
  };
  const del = async (c) => {
    if ((await act(() => rpc("uncomment", { number: c.id }))) === undefined) return;
    load();
  };
  if (!x.remote && (!list || !list.length)) return null;
  return html`<h3>Comments${list && list.length ? " · " + list.length : ""}</h3>
    ${x.remote ? html`<div class="row comm-say">
      <input type="text" class="grow" maxlength="300" placeholder="Say something" value=${text} onInput=${(e) => setText(e.target.value)} onKeyDown=${(e) => e.key === "Enter" && text.trim() && send()} />
      <button class="btn" disabled=${!text.trim() || busy} onClick=${send}>Post</button></div>` : null}
    ${list === null ? html`<div class="spinner sm"></div>` : html`<div class="comm-comments">${list.map((c, i) => html`<div key=${c.id || i} class="comm-comment">
      <div class="row"><b class="grow">${c.author}</b><span class="dim small">${c.ago}</span>
        ${x.remote && me && (me.admin || me.id === c.authorId) ? html`<button class="btn sm ghost" onClick=${() => del(c)}>Delete</button>` : null}</div>
      <div>${c.text}</div></div>`)}</div>`}`;
}

// ModTools: report a real upload, or delete it if it's yours. Admins can also ban the uploader.
function ModTools({ x, close }) {
  if (!x.remote) return null;
  const me = (S.comm && S.comm.server && S.comm.server.me) || {};
  const remove = async () => {
    if (!confirm(`Delete ${x.name} from the community?`)) return;
    if ((await act(() => rpc("removeUpload", { id: x.id }))) === undefined) return;
    await loadCommunity(true);
    toast(x.name + " deleted");
    close();
  };
  const report = async () => {
    const why = prompt("What's wrong with it?", "");
    if (why === null) return;
    if ((await act(() => rpc("report", { id: x.id, text: why }))) !== undefined) toast("Reported. Thanks.");
  };
  const ban = async () => {
    if (!confirm(`Ban ${x.author}? Their uploads and comments get deleted and they can't post again.`)) return;
    if ((await act(() => rpc("ban", { user: x.authorId }))) === undefined) return;
    await loadCommunity(true);
    toast(x.author + " banned");
    close();
  };
  return html`<span class="grow row" style=${{ gap: "6px" }}>
    ${x.mine || me.admin ? html`<button class="btn sm ghost danger" onClick=${remove}>Delete</button>` : html`<button class="btn sm ghost" onClick=${report}>Report</button>`}
    ${me.admin && !x.mine ? html`<button class="btn sm ghost danger" onClick=${ban}>Ban ${x.author}</button>` : null}
  </span>`;
}

function PackBrowse() {
  const [q, setQ] = useState("");
  const [sort, setSort] = useState("popular");
  const [tag, setTag] = useState("");
  const all = [...myPackUploads(), ...S.comm.packs];
  const tags = topTags(all, []);
  const ql = q.trim().toLowerCase();
  const list = sortList(all.filter((p) => (!tag || p.tags.includes(tag)) && (!ql || (p.name + " " + p.about + " " + p.author + " " + p.mods.map((m) => m.name).join(" ")).toLowerCase().includes(ql))), sort);
  return html`<div>
    <${Bar} q=${q} setQ=${setQ} sort=${sort} setSort=${setSort} placeholder="Search packs or mods" />
    <div class="row" style=${{ gap: "6px", flexWrap: "wrap", marginTop: "10px" }}>
      ${tags.map((t) => html`<button key=${t} class=${"chip click" + (tag === t ? " accent" : "")} onClick=${() => setTag(tag === t ? "" : t)}>${t}</button>`)}
    </div>
    ${list.length ? null : html`<div class="hint" style=${{ padding: "20px" }}>Nothing found.</div>`}
    <div class="comm-grid">${list.map((p) => html`<${PackCard} key=${p.id} p=${p} />`)}</div>
  </div>`;
}

function PackCard({ p }) {
  const have = p.mods.filter((m) => m.have).length;
  return html`<div class="comm-card" onClick=${() => update({ modal: { kind: "cpack", id: p.id } })}>
    <div class="comm-banner pack" style=${{ background: tint(p.name) }}>
      <span class="comm-count">${p.mods.length}<small>mods</small></span>
      ${p.mine ? html`<span class="ws-got">Your upload</span>` : p.have ? html`<span class="ws-got">Got it</span>` : null}
      ${p.example ? html`<span class="comm-example">Example</span>` : null}
    </div>
    <div class="ws-in">
      <b title=${p.name}>${p.name}</b>
      <div class="dim small">by ${p.author}${p.tags.length ? " · " + p.tags.join(", ") : ""}</div>
      ${p.about ? html`<div class="ws-desc">${p.about}</div>` : null}
      <div class="comm-have"><i style=${{ width: (100 * have) / p.mods.length + "%" }}></i></div>
      <div class="dim small">You have ${have} of ${p.mods.length}</div>
      <${Stats} x=${p} />
    </div>
  </div>`;
}

export function CommunityPackDetail({ id, close }) {
  const p = [...myPackUploads(), ...((S.comm && S.comm.packs) || [])].find((x) => x.id === id);
  if (!p) return null;
  const missing = p.mods.filter((m) => !m.have);
  const like = async () => { await act(() => rpc("like", { id: p.id })); loadCommunity(); };
  const get = async () => {
    const r = await act(() => rpc("getPack", { id: p.id }));
    if (!r) return;
    await Promise.all([loadConfig(), loadCommunity()]);
    toast(`Saved as the pack "${r.name}". ${r.missing.length ? r.missing.length + " mods still need downloading." : "You have every mod."}`);
  };
  const openMissing = () => missing.forEach((m, i) => setTimeout(() => rpc("openURL", { url: m.ws ? "steam://url/CommunityFilePage/" + m.ws : workshopSearch(m.name) }), i * 400));
  return html`<div class="story-detail">
    <div class="row" style=${{ gap: "10px", flexWrap: "wrap" }}><span class="dim">by <b>${p.author}</b></span><${Stats} x=${p} />${p.tags.map((t) => html`<span key=${t} class="chip">${t}</span>`)}</div>
    ${p.about ? html`<p class="story-text">${p.about}</p>` : null}
    <div class="row" style=${{ margin: "12px 0 6px" }}><h3 class="grow" style=${{ margin: 0 }}>Mods · ${p.mods.length}</h3><span class="dim small">You have ${p.mods.length - missing.length}</span></div>
    <div class="comm-mods">${p.mods.map((m) => html`<div key=${m.id} class="comm-mod">
      <span class=${"dot " + (m.have ? "good" : "")}></span>
      <span class="grow">${m.name}</span>
      ${m.have ? html`<span class="chip good">Installed</span>`
        : m.ws ? html`<button class="btn sm" onClick=${() => act(() => rpc("subscribe", { id: m.ws }), "Subscribed to " + m.name)}>Subscribe</button>`
        : html`<button class="btn sm ghost" onClick=${() => rpc("openURL", { url: workshopSearch(m.name) })}>Find on Workshop</button>`}
    </div>`)}</div>
    <${Comments} x=${p} />
    <div class="row" style=${{ marginTop: "14px", justifyContent: "flex-end", flexWrap: "wrap" }}>
      <${ModTools} x=${p} close=${close} />
      ${p.mine ? null : html`<button class=${"btn ghost" + (p.liked ? " liked" : "")} onClick=${like}>▲ ${p.liked ? "Liked" : "Like"}</button>`}
      ${missing.length ? html`<button class="btn ghost" onClick=${openMissing}>Open ${missing.length} missing on Workshop</button>` : null}
      ${p.mine ? null : p.have ? html`<span class="chip good">In your packs</span>` : html`<button class="btn primary" onClick=${get}>Get pack</button>`}
    </div>
  </div>`;
}

function Yours() {
  const ups = S.comm.uploads || [];
  const stories = ((S.presets && S.presets.backstories) || []).filter((b) => b.custom);
  const packs = Object.keys((S.cfg && S.cfg.packs) || {}).sort();
  const up = (kind, id) => ups.some((u) => u.kind === kind && u.id === id);
  const server = S.comm.server || {};
  const takeDown = async (kind, id, name) => {
    if (!confirm(`Take ${name} down?`)) return;
    if ((await act(() => rpc("unupload", { kind, id }))) === undefined) return;
    await loadCommunity(true);
    toast(name + " taken down");
  };
  const upBtns = (kind, id, name) => up(kind, id)
    ? html`<button class="btn sm" onClick=${() => update({ modal: { kind: "upload", what: kind, id } })}>Update</button><button class="btn sm ghost" onClick=${() => takeDown(kind, id, name)}>Take down</button>`
    : html`<button class="btn sm" onClick=${() => update({ modal: { kind: "upload", what: kind, id } })}>Upload</button>`;
  return html`<div class="grid" style=${{ gap: "16px" }}>
    <${ServerCard} server=${server} />
    <div class="card">
      <div class="row"><h3 class="grow" style=${{ margin: 0 }}>Your stories · ${stories.length}</h3><button class="btn sm" onClick=${() => openEditor()}>New story</button></div>
      ${stories.length ? null : html`<div class="hint" style=${{ marginTop: "8px" }}>None yet. Make one, or press Get on a community story.</div>`}
      ${stories.map((b) => html`<div key=${b.id} class="comm-row">
        <span class="grow click" onClick=${() => update({ modal: { kind: "story", id: b.id } })}><b>${b.name}</b> <span class="dim small">${players(b)} · ${b.roles.length} roles${b.author ? " · by " + b.author : ""}</span></span>
        <button class="btn sm ghost" onClick=${() => openEditor(copyStory((S.cfg.stories || []).find((x) => x.id === b.id) || b, true))}>Edit</button>
        <button class="btn sm ghost" onClick=${() => copyCode("storyCode", { id: b.id })}>Copy code</button>
        ${b.author ? null : upBtns("story", b.id, b.name)}
      </div>`)}
    </div>
    <div class="card">
      <h3 style=${{ margin: 0 }}>Your mod packs · ${packs.length}</h3>
      ${packs.length ? null : html`<div class="hint" style=${{ marginTop: "8px" }}>None yet. Save one on Mods > Packs.</div>`}
      ${packs.map((name) => html`<div key=${name} class="comm-row">
        <span class="grow"><b>${name}</b> <span class="dim small">${S.cfg.packs[name].length} mods</span></span>
        <button class="btn sm ghost" onClick=${() => copyCode("packCode", { name })}>Copy code</button>
        ${upBtns("pack", name, name)}
      </div>`)}
    </div>
  </div>`;
}

export async function copyCode(method, args) {
  const code = await act(() => rpc(method, args));
  if (!code) return;
  try {
    await navigator.clipboard.writeText(code);
    toast("Code copied. Paste it to a friend.");
  } catch (e) {
    prompt("Copy this code:", code);
  }
}

// PasteCode takes a story or pack code from a friend.
export function PasteCode({ close }) {
  const [code, setCode] = useState("");
  const go = async () => {
    const story = code.includes("KNOXSTORY1:");
    const r = await act(() => rpc(story ? "importStory" : "importPack", { code }));
    if (!r) return;
    await Promise.all([loadPresets(), loadConfig(), loadCommunity()]);
    if (story) {
      toast(r.name + " added to your stories");
      update({ modal: { kind: "story", id: r.id } });
    } else {
      toast(`Saved as the pack "${r.name}". ${r.missing.length ? r.missing.length + " mods still need downloading." : "You have every mod."}`);
      close();
    }
  };
  return html`<div>
    <div class="hint" style=${{ marginBottom: "8px" }}>Story codes start with KNOXSTORY1, pack codes with KNOXPACK1.</div>
    <textarea rows="4" style=${{ width: "100%" }} placeholder="Paste the code here" value=${code} onInput=${(e) => setCode(e.target.value)}></textarea>
    <div class="row" style=${{ justifyContent: "flex-end", marginTop: "10px" }}><button class="btn ghost" onClick=${close}>Cancel</button><button class="btn primary" disabled=${!code.trim()} onClick=${go}>Add</button></div>
  </div>`;
}

// ServerCard: who you are on the community server, and where it is.
function ServerCard({ server }) {
  const [edit, setEdit] = useState(false);
  const [url, setUrl] = useState(server.url || "");
  const save = async () => {
    if ((await act(() => rpc("setCommunity", { url }))) === undefined) return;
    setEdit(false);
    loadCommunity(true);
  };
  return html`<div class="card row" style=${{ gap: "10px", flexWrap: "wrap" }}>
    <span class=${"dot " + (server.on && !server.error ? "good" : server.on ? "bad" : "")}></span>
    <span class="grow">${!server.on ? "No community server yet. Uploads only show on this PC."
      : server.error ? "Can't reach the community server."
      : server.me ? html`Signed in with Steam as <b>${server.me.name}</b>${server.me.admin ? " (admin)" : ""}.` : "Connected. You sign in with Steam when you upload."}</span>
    ${edit ? html`<input type="text" style=${{ minWidth: "320px" }} placeholder="https://..." value=${url} onInput=${(e) => setUrl(e.target.value)} /><button class="btn sm" onClick=${save}>Save</button>`
      : html`<button class="btn sm ghost" onClick=${() => setEdit(true)}>Server address</button>`}
  </div>`;
}

// UploadWindow: a short description and tags, then up it goes.
export function UploadWindow({ what, id, close }) {
  const story = what === "story" && ((S.cfg && S.cfg.stories) || []).find((x) => x.id === id);
  const prev = ((S.comm && S.comm.uploads) || []).find((u) => u.kind === what && u.id === id);
  const live = prev && prev.remote && S.comm && [...S.comm.stories, ...S.comm.packs].find((x) => x.id === "r-" + prev.remote);
  const [about, setAbout] = useState((live && live.about) || (story ? story.story : ""));
  const [tags, setTags] = useState((live && live.tags) || (story ? story.tags || [] : []));
  const [busy, setBusy] = useState(false);
  const go = async () => {
    setBusy(true);
    const r = await act(() => rpc("upload", { kind: what, id, about, tags }));
    setBusy(false);
    if (!r) return;
    await loadCommunity(true);
    toast(prev ? "Updated" : "Uploaded");
    close();
  };
  return html`<div class="grid" style=${{ gap: "12px" }}>
    <div><b>${story ? story.name : id}</b> <span class="dim small">${story ? "story" : "mod pack"}</span></div>
    ${what === "pack" ? html`<label class="field"><span>About</span><textarea rows="3" maxlength="400" placeholder="What's in it and who it's for." value=${about} onInput=${(e) => setAbout(e.target.value)}></textarea></label>` : null}
    <div class="field"><span>Tags</span><${TagInput} tags=${tags} setTags=${setTags} /></div>
    <div class="hint">${S.comm && S.comm.server && S.comm.server.on ? "Everyone with Knox Launcher can see it. Your Steam name is shown." : "There's no community server yet, so this only shows on your PC."}</div>
    <div class="row" style=${{ justifyContent: "flex-end" }}><button class="btn ghost" onClick=${close}>Cancel</button><button class="btn primary" disabled=${busy} onClick=${go}>${busy ? "Uploading..." : prev ? "Update" : "Upload"}</button></div>
  </div>`;
}

// TagInput: type a tag and press Enter. Up to 8.
export function TagInput({ tags, setTags, presets }) {
  const [t, setT] = useState("");
  const add = (v) => {
    v = v.toLowerCase().replace(/\s+/g, " ").trim().slice(0, 20);
    if (v && !tags.includes(v) && tags.length < 8) setTags([...tags, v]);
    setT("");
  };
  return html`<div class="row tag-input">
    ${(presets || []).map(([id, name]) => {
      const on = tags.includes(id);
      return html`<button key=${id} class=${"chip click" + (on ? " accent" : "")} onClick=${() => setTags(on ? tags.filter((x) => x !== id) : [...tags, id])}>${name}</button>`;
    })}
    ${tags.filter((x) => !(presets || []).some((p) => p[0] === x)).map((x) => html`<span key=${x} class="chip click accent" title="Remove" onClick=${() => setTags(tags.filter((y) => y !== x))}>${x} ✕</span>`)}
    ${tags.length < 8 ? html`<input type="text" class="tag-new" maxlength="20" placeholder="+ your own tag" value=${t} onInput=${(e) => setT(e.target.value)}
      onKeyDown=${(e) => { if (e.key === "Enter" || e.key === ",") { e.preventDefault(); add(t); } }} onBlur=${() => t.trim() && add(t)} />` : null}
  </div>`;
}
