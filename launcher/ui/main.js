// Knox Launcher UI: app shell, navigation, live polling.
import { render } from "./vendor/preact-htm.js";
import { html, useEffect, useRef, S, update, useStore, rpc, act, loadConfig, loadCatalog, loadPresets, inLobby, isHost } from "./core.js";
import { Play, SyncOverlay, InspectModal, Steps, TitleArt, HostSetup, InviteWindow } from "./play.js";
import { Character, ensureDefaults } from "./character.js";
import { Mods, PacksWindow, OrderWindow, JavaWindow, ZBWindow, ZBBar, loadZB } from "./mods.js";
import { World, Settings } from "./world.js";
import { StoryDetail, findStory } from "./start.js";
import { Community, CommunityStoryDetail, CommunityPackDetail, PasteCode, UploadWindow } from "./community.js";
import { StoryEditor } from "./editor.js";
import { Loadout } from "./loadout.js";
import { Backups } from "./backups.js";
import { NavLevel, KnoxWindow } from "./knox.js";
import { SquadPoster } from "./poster.js";
import { UpdateBar, VersionLine } from "./update.js";
import { ShelfHome } from "./shelf.js";
import { NavTape, TapeMaker } from "./tapes.js";
import { Customize } from "./customize.js";
import { sound, soundOn, setSound } from "./sound.js";

const I = (d) => html`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d=${d} /></svg>`;
const TABS = [
  { id: "play", name: "Home", icon: "M3 11l9-8 9 8M5 9.5V21h14V9.5", page: Play },
  { id: "character", name: "Survivor & start", icon: "M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2M12 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8z", page: Character },
  { id: "mods", name: "Mods", icon: "M21 16V8l-9-5-9 5v8l9 5 9-5zM3.3 7L12 12l8.7-5M12 22V12", page: Mods },
  { id: "community", name: "Community", icon: "M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM23 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8", page: Community },
  { id: "world", name: "World", icon: "M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zM2 12h20M12 2a15 15 0 0 1 0 20M12 2a15 15 0 0 0 0 20", page: World },
  { id: "loadout", name: "Loadout", icon: "M6 7h12l1 14H5L6 7zM9 7V5a3 3 0 0 1 6 0v2", page: Loadout },
  { id: "customize", name: "Customize", icon: "M12 2l3 7h7l-5.5 4.5 2 7.5L12 17l-6.5 4 2-7.5L2 9h7z", page: Customize },
  { id: "settings", name: "Settings", icon: "M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6", page: Settings },
  { id: "backups", name: "Backups", icon: "M3 12a9 9 0 1 0 3-6.7L3 8M3 3v5h5M12 7v5l4 2", page: Backups, hide: true }, // opened from the Backups tile on Home
];

function Nav() {
  const st = S.state || {};
  const members = st.members || [];
  const g = st.game || {};
  return html`<aside id="nav">
    <${NavTape} />
    <nav id="tabs">${TABS.filter((t) => !t.hide).map((t) => html`<button key=${t.id} class=${"tab" + (S.tab === t.id ? " on" : "")} onClick=${() => go(t.id)}>
      ${I(t.icon)}<span>${t.name}</span>
      ${t.id === "play" && inLobby() ? html`<span class="badge">${members.length}</span>` : null}
      ${t.id === "play" && g.running ? html`<span class="live-dot sm"></span>` : null}
      ${!isHost() && ["mods", "world", "loadout", "settings"].includes(t.id) ? html`<span class="tab-note">host's</span>` : null}
    </button>`)}</nav>
    <${NavLevel} />
    <button class="tab howto-tab" onClick=${() => update({ modal: { kind: "guide" } })}>${I("M12 22a10 10 0 1 0 0-20 10 10 0 0 0 0 20zM9.1 9a3 3 0 0 1 5.8 1c0 2-3 3-3 3M12 17h.01")}<span>How it works</span></button>
    <div id="nav-status">
      <div class="stat-line"><span class=${"dot " + (st.steam && st.steam.ok ? "good" : "bad")}></span>${st.steam && st.steam.ok ? "Steam: " + ((st.me && st.me.name) || "connected") : "Steam not running"}</div>
      <div class="stat-line"><span class=${"dot " + (S.cat ? "good" : "warn")}></span>${S.cat ? `${S.cat.mods.length} mods · game ${S.cat.game}` : "Loading game files..."}</div>
      <div class="stat-line"><span class=${"dot " + (g.running ? "good" : "")}></span>${g.running ? "Game running" : "Game closed"}</div>
      ${S.zb ? html`<div class="stat-line click" onClick=${() => update({ modal: { kind: "zb" } })}><span class=${"dot " + (!S.zb.ready ? "bad" : S.zb.outdated ? "warn" : "good")}></span>ZombieBuddy: ${!S.zb.ready ? (S.zb.downloaded ? "not set up" : "missing") : S.zb.outdated ? "update ready" : "OK"}</div>` : null}
      <div class="stat-line click" onClick=${() => { setSound(!soundOn()); sound("button"); update(); }}><span class=${"dot " + (soundOn() ? "good" : "")}></span>VCR sounds: ${soundOn() ? "on" : "off"}</div>
      <${VersionLine} />
    </div>
  </aside>`;
}

function go(tab) {
  if (tab !== S.tab) sound("button");
  update({ tab });
  act(() => rpc("setTab", { name: tab }));
  const m = document.getElementById("main");
  if (m) m.scrollTop = 0;
}

function Modal() {
  const m = S.modal;
  if (!m) return null;
  const close = () => update({ modal: null });
  let body = null, title = "";
  if (m.kind === "inspect") {
    const mem = (S.state.members || []).find((x) => x.id === m.id);
    title = mem ? mem.name + "'s character" : "Character";
    body = html`<${InspectModal} id=${m.id} />`;
  } else if (m.kind === "guide") {
    title = "How Knox Launcher works";
    body = html`<${Guide} close=${close} />`;
  } else if (m.kind === "packs") {
    title = "Mod packs";
    body = html`<${PacksWindow} />`;
  } else if (m.kind === "java") {
    title = "Java mods";
    body = html`<${JavaWindow} />`;
  } else if (m.kind === "invite") {
    title = "Invite friends";
    body = html`<${InviteWindow} />`;
  } else if (m.kind === "zb") {
    title = "ZombieBuddy";
    body = html`<${ZBWindow} />`;
  } else if (m.kind === "order") {
    title = "Load order";
    body = html`<${OrderWindow} order=${m.order} problems=${m.problems} />`;
  } else if (m.kind === "setup") {
    title = "Host's setup";
    body = html`<${HostSetup} info=${S.state.info} />`;
  } else if (m.kind === "story") {
    const b = findStory(m.id);
    title = b ? b.name : "Story";
    body = html`<${StoryDetail} id=${m.id} close=${close} />`;
  } else if (m.kind === "editStory") {
    title = m.story.id ? "Edit story" : "New story";
    body = html`<${StoryEditor} key=${m.story.id || "new"} story=${m.story} close=${close} />`;
  } else if (m.kind === "cstory") {
    const b = S.comm && S.comm.stories.find((x) => x.id === m.id);
    title = b ? b.name : "Story";
    body = html`<${CommunityStoryDetail} id=${m.id} close=${close} />`;
  } else if (m.kind === "cpack") {
    const p = S.comm && S.comm.packs.find((x) => x.id === m.id);
    title = p ? p.name : m.id.replace(/^mine:/, "");
    body = html`<${CommunityPackDetail} id=${m.id} close=${close} />`;
  } else if (m.kind === "tape") {
    title = "Squad tape";
    body = html`<${SquadPoster} members=${(S.state && S.state.members) || []} height=${600} />`;
  } else if (m.kind === "knox") {
    title = "Knox level";
    body = html`<${KnoxWindow} />`;
  } else if (m.kind === "upload") {
    title = "Upload to the community";
    body = html`<${UploadWindow} what=${m.what} id=${m.id} close=${close} />`;
  } else if (m.kind === "pasteCode") {
    title = "Paste a code";
    body = html`<${PasteCode} close=${close} />`;
  }
  return html`<div class="modal-bg" onClick=${(e) => e.target === e.currentTarget && close()}>
    <div class=${"modal" + (m.kind === "invite" || m.kind === "pasteCode" || m.kind === "upload" ? " narrow" : "") + (m.kind === "tape" ? " tape-modal" : "") + (m.kind === "knox" ? " wide" : "")}><div class="modal-head"><h2>${title}</h2><button class="btn sm ghost" onClick=${close}>✕</button></div>${body}</div>
  </div>`;
}

function Guide({ close }) {
  return html`<div class="guide">
    <p class="lead">Set up a Zomboid game with friends here. The game only opens to play.</p>
    <${Steps} />
    <div class="guide-grid">
      <div><b>1. Character</b><p>Job, traits, looks. Pick a story role and it sets your job and traits for you. The game fills it in when it opens.</p></div>
      <div><b>2. Pick a tape</b><p>The launcher opens on your shelf. A tape is a world, solo or squad. Make a new one, or join a friend's that's hosting. No ports or IPs.</p></div>
      <div><b>3. Setup (host)</b><p>Mods, difficulty, story, spawn and loadout. The lobby sees changes right away.</p></div>
      <div><b>4. START</b><p>Everyone downloads the host's mods, then the game opens and joins the host.</p></div>
      <div><b>5. Play</b><p>The launcher stays open and shows who's online. Close the game to get back to the lobby.</p></div>
      <div><b>Needs</b><p>Steam, and the Knox Link mod for everyone. It's always on.</p></div>
    </div>
    <div class="row" style=${{ justifyContent: "flex-end", marginTop: "14px" }}><button class="btn primary" onClick=${() => { act(() => rpc("seenGuide")); S.cfg.seenGuide = true; close(); }}>Got it</button></div>
  </div>`;
}

function App() {
  useStore();
  const main = useRef(null);
  if (!S.cfg || !S.state) return html`<div class="boot"><div class="spinner"></div>Starting Knox Launcher...</div>`;
  const tab = TABS.find((t) => t.id === S.tab) || TABS[0];
  const Page = tab.page;
  const accent = (S.cfg.prefs && S.cfg.prefs.accent) || "";
  if ((document.documentElement.dataset.accent || "") !== accent) document.documentElement.dataset.accent = accent;
  // no tape in and not in anyone's lobby or game: the shelf (or making a tape) fills the window
  const st = S.state;
  const shelf = !S.tape && !inLobby() && !(st.game && st.game.running) && st.phase !== "ingame" && !["creating", "joining", "syncing", "launching"].includes(st.phase);
  if (S.making || shelf) return html`<div id="app" class="shelf-app">
    <main id="main" ref=${main}><${UpdateBar} /><${ZBBar} />${S.making ? html`<${TapeMaker} key=${S.making.edit ? S.making.edit.name : "new"} edit=${S.making.edit} />` : S.customizing ? html`<${Customize} />` : html`<${ShelfHome} />`}</main>
    <${SyncOverlay} />
    <${Modal} />
    ${!S.cat && !S.catErr ? html`<div id="splash"><${TitleArt} /><div class="splash-box"><h1 class="wordmark">KNOX LAUNCHER</h1><div class="spinner"></div><small>Reading your game files</small></div></div>` : null}
    ${S.toast ? html`<div class=${"toast show" + (S.toast.bad ? " bad" : "")}>${S.toast.text}</div>` : null}
  </div>`;
  return html`<div id="app">
    <${Nav} />
    <main id="main" ref=${main}>${S.catErr && !S.cat ? html`<div class="card note-card bad">${S.catErr}</div>` : null}<${UpdateBar} /><${ZBBar} /><${Page} /></main>
    <${SyncOverlay} />
    <${Modal} />
    ${!S.cat && !S.catErr ? html`<div id="splash"><${TitleArt} /><div class="splash-box"><h1 class="wordmark">KNOX LAUNCHER</h1><div class="spinner"></div><small>Reading your game files</small></div></div>` : null}
    ${S.toast ? html`<div class=${"toast show" + (S.toast.bad ? " bad" : "")}>${S.toast.text}</div>` : null}
    ${S.state.note ? null : null}
  </div>`;
}

// polling: the lobby, the game and the files on disk all change without us asking
async function poll() {
  try {
    const st = await rpc("state", { since: S.chatLen });
    if (st.chatLen < S.chatLen) S.chat = st.chat || [];
    else if (st.chat && st.chat.length) S.chat = [...S.chat, ...st.chat].slice(-200);
    S.chatLen = st.chatLen;
    if (st.note) act(() => Promise.reject(new Error(st.note)));
    const catChanged = S.cat && st.catalog && st.catalog !== S.cat.version;
    // the tape rewinds when a game closes
    const was = S.state && S.state.game && S.state.game.running, now = st.game && st.game.running;
    if (was && !now) sound("rewind");
    const cfgChanged = S.state && st.cfgRev !== S.state.cfgRev;
    update({ state: st });
    if (catChanged) { loadCatalog(); loadZB(); }
    if (cfgChanged && Date.now() - S.editedAt > 2000) loadConfig();
  } catch (e) {
    update({ state: { ...(S.state || {}), steam: { ok: false, error: "the launcher's background part stopped" } } });
  }
  setTimeout(poll, 1000);
}

(async function start() {
  render(html`<${App} />`, document.body);
  const [cfg, st] = await Promise.all([rpc("config"), rpc("state", {})]);
  S.chat = st.chat || [];
  S.chatLen = st.chatLen;
  update({ cfg, state: st, tab: "play", modal: cfg.seenGuide ? null : { kind: "guide" } });
  await loadCatalog();
  if (!S.presets) loadPresets();
  // ZombieBuddy: missing or out of date shows a bar on every page (the Workshop is asked every 20 min)
  loadZB();
  setInterval(() => loadZB(), 60000);
  // a first-time player gets a ready-made character, so friends never see a blank one
  if (S.cat && ensureDefaults(S.cfg.character)) {
    await act(() => rpc("saveCharacter", { character: S.cfg.character }));
    update();
  }
  poll();
})();
