// Play page (Home): the tape in the VCR (tapes.js); the lobby with everyone's characters, the squad story and chat.
import { html, useState, useEffect, useRef, S, update, rpc, act, toast, TOKEN, profName, placeText, fmtBytes, ago, Avatar, PageHead, inLobby } from "./core.js";
import { CharacterView } from "./viewer.js";
import { CharacterSheet, points } from "./character.js";
import { Live, ServerStrip } from "./live.js";
import { ChosenStory, respawnText, findStory, livesOf } from "./start.js";
import { PreviouslyOn } from "./prev.js";
import { JavaBanner } from "./mods.js";
import { SquadPoster, SquadTitle, PosterFig } from "./poster.js";
import { frameOf, wornOf, lookOf } from "./looks.js";
import { sound } from "./sound.js";
import { LevelTag, BadgeShelf, knoxOf } from "./knox.js";
import { TapeHome, ejectTape } from "./tapes.js";

export function useFriends(active) {
  const [friends, setFriends] = useState([]);
  useEffect(() => {
    if (!active) return;
    let alive = true;
    const load = async () => {
      const f = await act(() => rpc("friends"));
      if (alive && f) setFriends(f);
    };
    load();
    const t = setInterval(load, 5000);
    return () => { alive = false; clearInterval(t); };
  }, [active]);
  return friends;
}

export function Play() {
  const st = S.state;
  if (!st) return null;
  return html`
    ${st.game && (st.game.running || (st.game.exited && S.hideRecap !== st.game.sessions)) ? html`<${Live} />` : null}
    ${inLobby() ? html`<${Lobby} />` : st.phase === "ingame" ? null : S.tape ? html`<${TapeHome} />` : html`<div class="card note-card">No tape in. <a href="#" onClick=${(e) => { e.preventDefault(); ejectTape(); }}>Go to the shelf</a> and pick one.</div>`}`;
}

const I = (d) => html`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d=${d} /></svg>`;
export const ICONS = {
  play: "M5 3l14 9-14 9V3z",
  world: "M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20zM2 12h20M12 2a15 15 0 0 1 0 20M12 2a15 15 0 0 0 0 20",
  mods: "M21 16V8l-9-5-9 5v8l9 5 9-5zM3.3 7L12 12l8.7-5M12 22V12",
  pin: "M21 10c0 7-9 13-9 13s-9-6-9-13a9 9 0 0 1 18 0zM12 13a3 3 0 1 0 0-6 3 3 0 0 0 0 6z",
  bag: "M6 7h12l1 14H5L6 7zM9 7V5a3 3 0 0 1 6 0v2",
  rules: "M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6",
  backup: "M3 12a9 9 0 1 0 3-6.7L3 8M3 3v5h5M12 7v5l4 2",
  shelf: "M3 4h18M3 12h18M3 20h18M6 4v8M9 4v8M13 4v8M7 12v8M11 12v8M16 12v8",
  users: "M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2M9 11a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM23 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8",
};
export const Icon = ({ name }) => I(ICONS[name]);

// SetupTiles: one tile per part of the setup, each opening its page. Guests get the host's values.
export function SetupTiles({ info, mine, survivor = true }) {
  const [backup, setBackup] = useState(null);
  useEffect(() => {
    if (!mine || !info) return;
    act(() => rpc("backups", { world: info.world })).then((b) => setBackup(b || []));
  }, [mine, info && info.world]);
  if (!info) return html`<div class="hint">Waiting for the host...</div>`;
  const diff = S.presets && S.presets.difficulty.find((d) => d.id === info.difficulty);
  const story = findStory(info.backstory);
  const c = S.cfg && S.cfg.character;
  const go = (to) => { const [tab, charTab] = to.split(":"); mine ? update(charTab ? { tab, charTab } : { tab }) : update({ modal: { kind: "setup" } }); };
  const tile = (icon, k, v, sub, tab) => html`<div class="stile" onClick=${() => go(tab)}>
    <div class="tile-ico"><${Icon} name=${icon} /></div><div class="tk">${k}</div><div class="tv">${v}</div>${sub ? html`<div class="ts">${sub}</div>` : null}<span class="go">›</span></div>`;
  const spawn = !info.together ? "Everyone picks" : info.spawn ? placeText(info.spawn) : info.spawnMode === "random" ? "Random spot, each game" : "Picked on START";
  const folders = Object.keys((S.cfg && S.cfg.modFolders) || {}).length;
  const solo = mine && S.tape && S.tape.kind === "solo"; // backups are for hosted worlds
  const last = backup && backup.length ? backup[0] : null;
  const rules = [info.pvp ? "PvP on" : "PvP off", info.sleep ? "sleep on" : "no sleep", info.sandbox && info.sandbox.length ? info.sandbox.length + " changed" : null].filter(Boolean).join(" · ");
  return html`<div class=${"setup-tiles" + (survivor ? "" : " no-surv")}>
    ${survivor ? html`<div class="stile surv" onClick=${() => update({ tab: "character", charTab: "survivor" })}>
      <${CharacterView} c=${c} height=${300} portrait />
      <div class="surv-in"><div class="tk">Survivor</div><div class="tv">${c && c.forename ? `${c.forename} ${c.surname}` : "Make one"}</div>
        <div class="ts">${c ? profName(c.profession) + ((c.traits || []).length ? " · " + (c.traits || []).length + " traits" : "") : ""}</div></div><span class="go">›</span></div>` : null}
    ${tile("world", "World", solo ? "Solo world" : info.world + (info.newWorld ? " (new)" : ""), diff ? diff.name : (info.difficulty || "World default").replace(/^(mine|file):/, ""), "world")}
    ${tile("mods", "Mods", (info.mods || []).length + " on" + (mine && folders ? ` · ${folders} folders` : ""), mine ? (S.cfg.host.pack ? "Pack: " + S.cfg.host.pack : "My mods") : "Host's list", "mods")}
    ${tile("pin", "Story & start", story ? story.name : "No story", spawn, "character:story")}
    ${tile("bag", "Loadout", info.kit && info.kit.length ? info.kitName || "Custom" : "Nothing", (info.kit || []).slice(0, 3).map((k) => (k.count > 1 ? k.count + "× " : "") + (k.name || k.type)).join(", "), "loadout")}
    ${tile("rules", "Rules", rules, "Respawn: " + respawnText(info.respawn), "settings")}
    ${mine && !solo ? tile("backup", "Backups", last ? ago((Date.now() - last.time * 1000) / 1000) : "None yet", last ? `${backup.length} saved of ${info.world}` : "Made each time you host", "backups") : null}
  </div>`;
}

// Steps explains the whole flow and ticks off what's done.
export function Steps() {
  const st = S.state || {};
  const cat = S.cat;
  const c = S.cfg && S.cfg.character;
  const knox = cat && cat.modById && cat.modById.KnoxLink;
  const items = [
    { done: st.steam && st.steam.ok, title: "Steam", text: st.steam && st.steam.ok ? "Connected" : "Not running" },
    { done: !!knox, title: "Knox Link mod", text: knox ? "Installed" : "Subscribe on the Workshop", link: !knox ? "steam://url/CommunityFilePage/3824457669" : null },
    ...(S.zb && (S.zb.needed || !S.zb.ready || S.zb.outdated) ? [{ done: S.zb.ready && !S.zb.outdated, title: "ZombieBuddy", text: !S.zb.ready ? (S.zb.downloaded ? "Not set up" : "Not downloaded") : S.zb.outdated ? "Update ready" : "Set up", modal: "zb" }] : []),
    { done: c && c.forename && cat && points(c) >= 0, title: "Your character", text: c && c.forename ? `${c.forename} ${c.surname}, ${profName(c.profession)}` : "Make one", tab: "character" },
    { done: inLobby() || st.phase === "ingame", title: "Host or join", text: inLobby() ? (st.host ? "You're hosting" : "In " + (st.hostName || "a") + "'s lobby") : "Make a lobby or join one" },
    { done: st.phase === "ingame", title: "START", text: "Opens the game for everyone" },
  ];
  return html`<div class="steps">${items.map((it, i) => html`<div key=${i} class=${"step" + (it.done ? " done" : "")} onClick=${() => { if (it.tab) update({ tab: it.tab }); else if (it.modal) update({ modal: { kind: it.modal } }); else if (it.link) rpc("openURL", { url: it.link }); }}>
    <div class="step-n">${it.done ? "✓" : i + 1}</div><div><b>${it.title}</b><small>${it.text}</small></div></div>`)}</div>`;
}

// HostSetup: the host's whole setup, live. Everyone in the lobby sees it.
export function HostSetup({ info, mine }) {
  const [showMods, setShowMods] = useState(false);
  if (!info) return html`<div class="card"><h3>Game setup</h3><div class="hint">Waiting for the host...</div></div>`;
  const diff = S.presets && S.presets.difficulty.find((d) => d.id === info.difficulty);
  const cat = S.presets && S.presets.buildings.find((b) => b.id === info.spawnType);
  const story = findStory(info.backstory);
  const go = (tab) => mine && update({ tab });
  const row = (label, value, tab, sub) => html`<div class=${"setup-row" + (mine && tab ? " click" : "")} onClick=${() => go(tab)}><span class="muted">${label}</span><div><b>${value}</b>${sub ? html`<small>${sub}</small>` : null}</div></div>`;
  const spawn = !info.together ? "Everyone picks" : info.spawn ? placeText(info.spawn) : "Picked on START";
  return html`<div class="card setup">
    <h3>Game setup</h3>
    ${row("World", info.world + (info.newWorld ? " (new)" : ""), "world")}
    ${row("Difficulty", diff ? diff.name : (info.difficulty || "World default").replace(/^(mine|file):/, ""), "world", diff ? diff.tagline : null)}
    ${info.sandbox && info.sandbox.length ? row("Changed", info.sandbox.length + " setting" + (info.sandbox.length > 1 ? "s" : ""), "settings", info.sandbox.slice(0, 4).map((o) => o.name.split(".").pop() + " " + o.value).join(", ") + (info.sandbox.length > 4 ? "..." : "")) : null}
    ${row("Story", story ? story.name : "None", "character")}
    ${row("Start", spawn, "character", info.together && info.spawnMode === "random" ? "Random " + (cat ? cat.name.replace(/^(A|An|The) /, "").toLowerCase() : "house") + (info.spawnTown ? " in " + info.spawnTown : "") + ", rerolls each game" : null)}
    ${row("Respawn", respawnText(info.respawn), "character", livesOf(info).name)}
    ${row("Loadout", info.kit && info.kit.length ? (info.kitName || "Custom") : "Nothing", "loadout", (info.kit || []).map((k) => (k.count > 1 ? k.count + "× " : "") + (k.name || k.type)).join(", "))}
    ${row("Rules", [info.pvp ? "PvP on" : "PvP off", info.sleep ? "sleep on" : "no sleep", info.freePoints ? `+${info.freePoints} trait points` : null].filter(Boolean).join(" · "), "settings")}
    <div class=${"setup-row" + (mine ? " click" : "")} onClick=${() => (mine ? go("mods") : setShowMods(!showMods))}><span class="muted">Mods</span><div><b>${(info.mods || []).length} mods</b><small>${mine ? "Mods page" : showMods ? "Hide" : "Show"}</small></div></div>
    ${showMods && !mine ? html`<div class="mod-mini">${(info.mods || []).map((m) => html`<span key=${m.id} class="chip">${m.name}</span>`)}</div>` : null}
  </div>`;
}

// TitleArt: the game's own title screen behind the hero, with its lightning flash now and then.
export function TitleArt({ dim }) {
  const [flash, setFlash] = useState(false);
  const [ok, setOk] = useState(true);
  useEffect(() => {
    let t;
    const next = () => {
      t = setTimeout(() => {
        setFlash(true);
        setTimeout(() => setFlash(false), 90);
        setTimeout(() => setFlash(true), 220);
        setTimeout(() => { setFlash(false); next(); }, 380);
      }, 7000 + Math.random() * 12000);
    };
    next();
    return () => clearTimeout(t);
  }, []);
  if (!ok) return null;
  return html`<div class=${"title-art" + (dim ? " dim" : "")}>
    <img src="/art" alt="" onError=${() => setOk(false)} />
    <img src="/art?k=lightning" alt="" class=${"flash" + (flash ? " on" : "")} onError=${(e) => (e.target.style.display = "none")} />
  </div>`;
}

// Lobby ------------------------------------------------------------------------------------------

function Lobby() {
  const st = S.state;
  const friends = useFriends(!!st.host);
  const invite = () => update({ modal: { kind: "invite" } });
  const members = st.members || [];
  const me = members.find((m) => m.me);
  const readyCount = members.filter((m) => m.ready).length;
  const info = st.info;
  const story = info && findStory(info.backstory);
  const inside = new Set(members.map((m) => m.id));
  const list = blockers();
  const go = (tab) => (st.host ? update({ tab }) : update({ modal: { kind: "setup" } }));
  const diff = info && S.presets && S.presets.difficulty.find((d) => d.id === info.difficulty);
  const dock = (icon, k, v, sub, tab) => html`<div class="dcard" onClick=${() => go(tab)}><div class="tile-ico"><${Icon} name=${icon} /></div>
    <div class="grow" style=${{ minWidth: 0 }}><div class="tk">${k}</div><div class="dv">${v}</div><div class="ts">${sub}</div></div><span class="go">›</span></div>`;
  return html`
    <div class="lobby-banner"><${TitleArt} dim /></div>
    <div class="squad-head">
      <div class="grow"><${SquadTitle} members=${members} />
        <p>${st.host ? "Everyone sets up here. Games open when you press START." : "Get your survivor ready, then ready up."}</p></div>
      <span class="code" title="Click to copy" onClick=${() => { navigator.clipboard && navigator.clipboard.writeText(st.code); toast("Copied " + st.code); }}>${st.code}</span>
      ${st.host ? html`<button class="btn" onClick=${invite}>Invite friends</button>` : null}
      ${st.host ? html`<${TestModList} />` : null}
      <button class="btn" onClick=${() => update({ modal: { kind: "tape" } })}>Squad tape</button>
      <button class="btn ghost" onClick=${() => { sound("eject"); act(() => rpc("leaveLobby")); }}>Leave</button>
    </div>
    ${st.game && st.game.running ? null : html`<${ServerStrip} />`}
    <div class="slots">
      ${members.map((m) => html`<${MemberSlot} key=${m.id} m=${m} story=${story} />`)}
      ${st.host ? html`<div class="slot empty" onClick=${invite}><div class="plus">+</div><b>Invite a friend</b><span>${friends.filter((f) => f.online && !inside.has(f.id)).length} online</span></div>` : null}
    </div>
    <div class="dock">
      ${info ? html`
        ${dock("world", "World", info.world + (info.newWorld ? " (new)" : ""), diff ? diff.name : (info.difficulty || "World default").replace(/^(mine|file):/, ""), "world")}
        ${dock("mods", "Mods", (info.mods || []).length + " mods", st.host ? (S.cfg.host.pack ? "Pack: " + S.cfg.host.pack : "My mods") : "Downloads at START", "mods")}
        ${dock("pin", "Story", story ? story.name : "No story", info.together ? (info.spawn ? placeText(info.spawn) : "Spawn together") : "Everyone picks a spot", "character")}` : html`<div class="dcard"><span class="muted">Waiting for the host's setup...</span></div>`}
      <div class="go-box">
        <button class=${"btn big ready-btn " + (me && me.ready ? "good" : "") + (myBlockers().length && !(me && me.ready) ? " blocked" : "")} onClick=${() => {
          if (!(me && me.ready) && myBlockers().length) return blockedToast(myBlockers());
          act(() => rpc("ready", { ready: !(me && me.ready) }));
        }}>${me && me.ready ? "Ready ✓" : "I'm ready"}</button>
        ${st.host ? html`<${StartButton} label="START" cls="btn primary huge start-btn" disabled=${st.phase !== "lobby"} />` : null}
        <div class="go-note">${readyCount} of ${members.length} ready${list.length ? html` · <span class="warn-text">${list[list.length - 1]}</span>` : st.host ? " · press START" : " · the host starts"}</div>
      </div>
    </div>
    <${JavaBanner} />
    ${list.length > 1 ? html`<div class="blockers">${list.slice(0, -1).map((b) => html`<div key=${b} class="blocker">⚠ ${b}</div>`)}</div>` : null}
    <div class=${"lobby-grid" + (st.host ? " one" : "")}>
      <div class="grid">
        ${info && info.cast ? html`<${PreviouslyOn} cast=${info.cast} world=${info.world} />` : null}
        ${story ? html`<${ChosenStory} story=${story} info=${info} />` : st.host ? html`<div class="card row"><span class="grow muted">No story picked.</span><button class="btn sm" onClick=${() => update({ tab: "character" })}>Pick a story</button></div>` : null}
        <${Chat} />
      </div>
      ${!st.host ? html`<div class="grid"><${HostSetup} info=${info} /></div>` : null}
    </div>`;
}

// myBlockers lists what stops your own character from being accepted by the game.
function myBlockers() {
  const out = [];
  const c = S.cfg && S.cfg.character;
  if (!c || !S.cat) return out;
  const pts = points(c);
  if (pts < 0) out.push(`Your traits are ${-pts} point${pts < -1 ? "s" : ""} over. Drop a good trait or add a bad one.`);
  if (!c.forename || !c.forename.trim()) out.push("Your character needs a name.");
  return out;
}

// blockers adds problems with anyone else in the lobby (the host can't start until they're fixed).
function blockers() {
  const out = myBlockers();
  const st = S.state || {};
  if (!(st.steam && st.steam.ok)) out.push("Steam isn't running.");
  if (!S.cat) return out;
  for (const m of st.members || []) {
    if (m.me) continue;
    const ch = memberChar(m);
    const c = ch && ch.c;
    if (c && points(c) < 0) out.push(`${m.name}'s traits are ${-points(c)} points over.`);
    if (m.appVer !== st.version) out.push(`${m.name} has launcher ${m.appVer || "(old)"}. Everyone needs ${st.version}.`);
  }
  const waiting = (st.members || []).filter((m) => !m.ready).map((m) => (m.me ? "You" : m.name));
  if (inLobby() && waiting.length) out.push(`Not ready: ${waiting.join(", ")}.`);
  return out;
}

function blockedToast(list) {
  toast("Can't start yet: " + list[0], true);
}

// StartButton always shows. If something would stop the game accepting a character, it says what.
export function StartButton({ label, cls, disabled, onGo }) {
  const list = blockers();
  return html`<div class="start-wrap">
    <button class=${cls + (list.length ? " blocked" : "")} disabled=${disabled} title=${list.join("\n")} onClick=${() => {
      if (list.length) {
        blockedToast(list);
        if (myBlockers().length) {
          update({ tab: "character", charTab: "survivor" });
        }
        return;
      }
      sound("insert");
      act(onGo || (() => rpc("start")));
    }}>${label}</button>
    ${list.length && label !== "START" ? html`<div class="blockers small">${list.map((b) => html`<div key=${b} class="blocker">⚠ ${b}</div>`)}</div>` : null}
  </div>`;
}

function memberChar(m) {
  try { return m.char ? (typeof m.char === "string" ? JSON.parse(m.char) : m.char) : null; } catch { return null; }
}

function MemberSlot({ m, story }) {
  const ch = memberChar(m);
  const c = m.me ? S.cfg.character : ch && ch.c;
  const syncing = m.sync && m.sync !== "ready" && m.sync !== "launching" && !m.sync.startsWith("done:") && m.sync !== "error: cancelled";
  const part = syncing && !m.sync.startsWith("error") && m.sync.includes("/") ? m.sync.split("/").map(Number) : null;
  const over = c && S.cat && points(c) < 0;
  const role = m.role && story && story.roles.some((r) => r.name === m.role) && !(c && profName(c.profession) === m.role) ? m.role : null;
  let state;
  if (m.inGame) state = html`<div class="sstate ok">In game</div>`;
  else if (part) state = html`<div class="sstate wait">Mods ${m.sync}<div class="bar"><i style=${{ width: Math.round((100 * part[0]) / Math.max(1, part[1])) + "%" }}></i></div></div>`;
  else if (syncing && m.sync.startsWith("error")) state = html`<div class="sstate bad" title=${m.sync}>Mod problem</div>`;
  else if (over) state = html`<div class="sstate bad">Traits over by ${-points(c)}</div>`;
  else if (m.appVer && S.state && m.appVer !== S.state.version) state = html`<div class="sstate bad">Old launcher v${m.appVer}</div>`;
  else state = m.ready ? html`<div class="sstate ok">✓ Ready</div>` : html`<div class="sstate">Not ready</div>`;
  const frame = frameOf(m);
  return html`<div class=${"slot" + (m.me ? " me" : "") + (m.ready ? " ready" : "") + (frame ? " frame-" + frame : "")} onClick=${() => (m.me ? update({ tab: "character" }) : update({ modal: { kind: "inspect", id: m.id } }))} title=${m.me ? "Edit your survivor" : "Inspect"}>
    ${m.host ? html`<span class="slot-tag">HOST</span>` : null}
    ${m.me ? html`<span class="slot-edit">Edit</span>` : null}
    <div class="slot-fig">${c ? html`<${PosterFig} m=${m} c=${wornOf(m) || c} seed=${(S.state && S.state.lobby) || "lobby"} />` : html`<div class="slot-none">No survivor yet</div>`}</div>
    <div class="slot-info">
      <div class=${"sname" + (lookOf(m).plate ? " plate-" + lookOf(m).plate : "")}>${c && c.forename ? `${c.forename} ${c.surname}` : m.name}</div>
      <div class="ssub">${m.name}${m.me ? " (you)" : ""}${c ? " · " + profName(c.profession) : ""}${role ? " · " + role : ""}</div>
      <${LevelTag} k=${knoxOf(m)} small />
      ${state}
    </div>
  </div>`;
}

export function InspectModal({ id }) {
  const m = (S.state.members || []).find((x) => x.id === id);
  if (!m) return html`<div class="muted">They left.</div>`;
  const ch = memberChar(m);
  const k = knoxOf(m);
  return html`<div>${k ? html`<div class="inspect-knox"><${LevelTag} k=${k} /><${BadgeShelf} have=${k.badges} compact /></div>` : null}<${CharacterSheet} c=${ch && ch.c} name=${m.name} /></div>`;
}

function Chat() {
  const st = S.state;
  const [text, setText] = useState("");
  const log = useRef(null);
  const stick = useRef(true);
  useEffect(() => {
    const el = log.current;
    if (el && stick.current) el.scrollTop = el.scrollHeight;
  }, [S.chat.length]);
  const send = () => {
    if (!text.trim()) return;
    act(() => rpc("chat", { text }));
    setText("");
  };
  return html`<div class="card chat"><h3>Chat</h3>
    <div class="chat-log" ref=${log} onScroll=${(e) => { const el = e.target; stick.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40; }}>
      ${S.chat.map((l, i) => html`<div key=${i} class=${"chat-line" + (l.sys ? " sys" : "")}>${l.sys ? null : html`<b>${l.name}</b>`}${l.text}</div>`)}
    </div>
    <div class="chat-in"><input type="text" value=${text} placeholder="Say something" onInput=${(e) => setText(e.target.value)} onKeyDown=${(e) => e.key === "Enter" && send()} /><button class="btn" onClick=${send}>Send</button></div>
  </div>`;
}

// Overlay while mods sync and the game opens --------------------------------------------------

export function SyncOverlay() {
  const st = S.state;
  if (!st || (st.phase !== "syncing" && st.phase !== "launching")) return null;
  const s = st.sync || {};
  const pct = s.total ? Math.round((s.done / s.total) * 100) : 100;
  if (s.wait) {
    const sv = st.server || {};
    const srvText = st.host ? (sv.running ? sv.text : "Not started") : st.hostServer || "Starting";
    const srvUp = st.host ? sv.stage === "up" : st.hostServer === "Server is up";
    const rows = (st.members || []).map((m) => {
      // your own mods are done before waiting starts, and the host's are done before anyone is told to start
      const done = m.me || m.host || (m.sync || "").startsWith("done:") || m.sync === "ready" || m.sync === "launching";
      const part = (m.sync || "").includes("/") ? m.sync.split("/").map(Number) : null;
      const pct = done ? 100 : part ? Math.round((100 * part[0]) / Math.max(1, part[1])) : 0;
      return { name: m.name + (m.me ? " (you)" : ""), done, pct, text: done ? "Mods ready" : part ? `Mods ${m.sync}` : (m.sync || "").startsWith("error") ? "Mod problem" : "Checking mods" };
    });
    return html`<div id="overlay"><div class="wait-wrap"><${SquadPoster} members=${st.members || []} /><div class="sync-box wait-box">
      <h2>Getting everyone ready</h2>
      <div class="muted small" style=${{ marginBottom: "14px" }}>Games open when the server is up and everyone has the mods.</div>
      <div class="wait-row"><span class=${"wait-ico" + (srvUp ? " ok" : "")}>${srvUp ? "✓" : html`<div class="spinner sm"></div>`}</span><b>${st.host ? "Your server" : "Host's server"}</b><span class="grow muted">${srvText}</span></div>
      ${rows.map((r) => html`<div class="wait-row" key=${r.name}><span class=${"wait-ico" + (r.done ? " ok" : "")}>${r.done ? "✓" : html`<div class="spinner sm"></div>`}</span><b>${r.name}</b>
        <div class="grow"><div class="bar"><i style=${{ width: r.pct + "%" }}></i></div></div><span class="muted small wait-txt">${r.text}</span></div>`)}
      ${(s.warnings || []).map((w, i) => html`<div key=${i} class="chip warn" style=${{ marginTop: "10px", whiteSpace: "normal" }}>${w}</div>`)}
      <button class="btn ghost" style=${{ marginTop: "16px" }} onClick=${() => act(() => rpc("cancelStart"))}>Cancel</button>
    </div></div></div>`;
  }
  return html`<div id="overlay"><div class="sync-box">
    <div class="spinner"></div>
    <h2>${st.phase === "launching" ? "Opening Project Zomboid" : "Getting mods ready"}</h2>
    <div class="muted">${st.phase === "launching" ? "This window stays open while you play." : s.current ? "Downloading " + s.current : "Checking " + (s.total || 0) + " Workshop items"}</div>
    ${st.phase === "syncing" ? html`<div class="bar" style=${{ margin: "16px 0 6px" }}><i style=${{ width: pct + "%" }}></i></div>
      <div class="small dim">${s.done || 0} of ${s.total || 0} items${s.bytesAll ? " · " + fmtBytes(s.bytes) + " of " + fmtBytes(s.bytesAll) : ""}</div>` : null}
    ${(s.warnings || []).map((w, i) => html`<div key=${i} class="chip warn" style=${{ marginTop: "10px", whiteSpace: "normal" }}>${w}</div>`)}
  </div></div>`;
}

// TestModList sends your mod list through Steam and checks it comes back, so you can test alone.
function TestModList() {
  const [busy, setBusy] = useState(false);
  return html`<button class="btn ghost" disabled=${busy} title="Sends your mod list through Steam the way START does and checks it arrives" onClick=${async () => {
    setBusy(true);
    const r = await act(() => rpc("testModList"));
    setBusy(false);
    if (!r) return;
    if (r.chatOK) toast(`Mod list OK: ${r.mods} mods, ${r.items} Workshop items, back from Steam in ${(r.chatMs / 1000).toFixed(1)} s.`);
    else toast("Mod list test failed. " + (r.note || ""), true);
  }}>${busy ? "Testing..." : "Test mod list"}</button>`;
}

// InviteWindow sends Steam lobby invites. Friends get a Steam message; clicking it opens Knox Launcher in the lobby.
export function InviteWindow() {
  const friends = useFriends(true);
  const [q, setQ] = useState("");
  const [sent, setSent] = useState({});
  const inside = new Set(((S.state && S.state.members) || []).map((m) => m.id));
  const rank = (f) => (inside.has(f.id) ? 3 : f.online ? (f.inGame ? 1 : 0) : 2);
  const list = friends.filter((f) => !q || f.name.toLowerCase().includes(q.toLowerCase())).sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name));
  const send = async (f) => {
    const ok = await act(() => rpc("invite", { id: f.id }), "Invite sent to " + f.name);
    if (ok !== undefined) setSent({ ...sent, [f.id]: true });
  };
  return html`<div class="invite-win">
    <input type="text" placeholder="Search friends" value=${q} onInput=${(e) => setQ(e.target.value)} />
    <div class="friends invite-list">
      ${list.length ? list.map((f) => html`<div key=${f.id} class=${"friend" + (f.online ? "" : " off")}><${Avatar} id=${f.id} name=${f.name} small /><div class="fn"><b>${f.name}</b><small>${f.online ? (f.inGame ? "Playing" : "Online") : "Offline"}</small></div>
        ${inside.has(f.id) ? html`<span class="chip good">In lobby</span>` : sent[f.id] ? html`<button class="btn sm" onClick=${() => send(f)}>Sent ✓</button>` : html`<button class="btn sm primary" disabled=${!f.online} onClick=${() => send(f)}>Invite</button>`}</div>`) : html`<div class="hint">${friends.length ? "Nobody matches." : "Loading friends..."}</div>`}
    </div>
    <p class="muted small" style=${{ marginBottom: 0 }}>They get a Steam message. They need Knox Launcher open, or they can type your code: <b>${S.state && S.state.code}</b></p>
  </div>`;
}
