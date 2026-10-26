// Story, role and start: picking a squad story, taking a role, where everyone spawns and respawns.
import { html, useState, S, update, rpc, act, toast, patchHost, placeText, profName, saveCharacter, Select, Toggle, isHost, inLobby, TOKEN } from "./core.js";
import { SpawnMap } from "./map.js";
import { points } from "./character.js";
import { openEditor, copyStory } from "./editor.js";
import { copyCode } from "./community.js";

// findStory looks up a story by id. A guest doesn't have the host's own stories, so those come with the host's setup.
export function findStory(id) {
  if (!id) return null;
  const b = S.presets && S.presets.backstories.find((x) => x.id === id);
  if (b) return b;
  const info = S.state && S.state.info;
  return info && info.story && info.story.id === id ? info.story : null;
}

export const RESPAWN = [
  { id: "base", name: "Back at the start", text: "New character spawns in the starting building." },
  { id: "squad", name: "Next to a friend", text: "New character spawns next to someone who's still alive. If nobody is, back at the start." },
  { id: "random", name: "Random house", text: "New character spawns in a random house somewhere on the map." },
];

// Lives per tape: how many characters each player gets in a world, or spare lives the squad shares.
export const LIVES = [
  { id: "0", lives: 0, shared: false, name: "Unlimited", text: "Die as often as you like." },
  { id: "1", lives: 1, shared: false, name: "Dead is dead", text: "One life each. Die and you're out for this tape." },
  { id: "3", lives: 3, shared: false, name: "3 lives each", text: "Your third death puts you out." },
  { id: "3s", lives: 3, shared: true, name: "3 for the squad", text: "The squad shares 3 spare lives. When they're gone, the next death is for good." },
  { id: "5s", lives: 5, shared: true, name: "5 for the squad", text: "The squad shares 5 spare lives." },
];
export function livesOf(h) {
  return LIVES.find((l) => l.lives === (h.lives || 0) && l.shared === !!h.livesShared && (l.lives > 0 || !h.livesShared)) || LIVES[0];
}

export function respawnText(id) {
  const r = RESPAWN.find((x) => x.id === id) || RESPAWN[0];
  return r.name;
}

const TAGS = [
  { id: "easy", name: "Easy" },
  { id: "hard", name: "Hard" },
  { id: "town", name: "Town" },
  { id: "wild", name: "Woods" },
  { id: "long", name: "Long game" },
];

const playerCount = () => Math.max(1, (S.state && S.state.members && S.state.members.length) || 1);
const fits = (b, n) => n >= b.min && n <= b.max;
const players = (b) => (b.min === b.max ? b.min : b.min + "-" + b.max) + (b.max > 1 ? " players" : " player");
const traitName = (id) => (S.cat && S.cat.traitById[id] ? S.cat.traitById[id].name : id);
const packOf = (b) => (S.presets.modPacks || []).find((p) => p.id === b.mod);
const workshopSearch = (q) => `https://steamcommunity.com/workshop/browse/?appid=108600&searchtext=${encodeURIComponent(q)}`;

// takeRole claims a role and sets your job and starting traits from it. You can change them after.
export async function takeRole(role) {
  if ((await act(() => rpc("takeRole", { name: role ? role.name : "" }))) === undefined) return;
  S.cfg.role = role ? role.name : "";
  if (!role) return update();
  const c = S.cfg.character;
  if (role.profession && S.cat.profById[role.profession]) c.profession = role.profession;
  const prof = S.cat.profById[c.profession];
  const granted = new Set((prof && prof.granted) || []);
  c.traits = (role.traits || []).filter((id) => S.cat.traitById[id] && !granted.has(id));
  // your own job may cost more than the role's: drop the priciest good traits until it adds up
  while (points(c) < 0) {
    const good = c.traits.filter((id) => S.cat.traitById[id].cost > 0).sort((a, b) => S.cat.traitById[b].cost - S.cat.traitById[a].cost);
    if (!good.length) break;
    c.traits = c.traits.filter((x) => x !== good[0]);
  }
  saveCharacter();
  toast(`You're the ${role.name}. Job and traits set. You can still change them.`);
}

// StorySection: the chosen story with its roles, then the list of stories.
export function StorySection() {
  const P = S.presets;
  const host = isHost();
  const info = (host ? S.cfg.host : S.state && S.state.info) || {};
  const [browse, setBrowse] = useState(false);
  if (!P) return null;
  const story = findStory(info.backstory);
  return html`<section id="sec-story" class="section">
    <div class="section-head"><h2>Story</h2><span class="muted">Optional. Sets the start building, the loadout and roles to pick.</span>
      <div class="row" style=${{ marginLeft: "auto", gap: "6px" }}>
        <button class="btn sm ghost" onClick=${() => update({ tab: "community", commView: "stories" })}>Community stories</button>
        <button class="btn sm ghost" onClick=${() => openEditor()}>New story</button>
        ${story ? html`<button class="btn sm ghost" onClick=${() => setBrowse(!browse)}>${browse ? "Hide stories" : host ? "Change story" : "See all stories"}</button>` : null}
      </div></div>
    ${story ? html`<${ChosenStory} story=${story} info=${info} />` : html`<div class="hint" style=${{ marginBottom: "10px" }}>${host ? "No story picked. Everyone just spawns as themselves." : "The host hasn't picked a story."}</div>`}
    ${browse || !story ? html`<${StoryBrowser} chosen=${info.backstory} />` : null}
  </section>`;
}

function StoryBrowser({ chosen }) {
  const P = S.presets;
  const host = isHost();
  const n = playerCount();
  const [q, setQ] = useState("");
  const [tag, setTag] = useState("");
  const [onlyFit, setOnlyFit] = useState(false);
  const match = (b) => (!tag || (tag === "mine" ? b.custom : (b.tags || []).includes(tag))) && (!onlyFit || fits(b, n)) && (!q || (b.name + " " + b.story).toLowerCase().includes(q.toLowerCase()));
  const open = P.backstories.filter((b) => !b.locked && match(b)).sort((a, b) => fits(b, n) - fits(a, n));
  const locked = P.backstories.filter((b) => b.locked && match(b));
  const packs = (P.modPacks || []).filter((p) => !p.installed);
  return html`<div class="story-browser">
    <div class="row filters">
      <input type="text" placeholder="Search stories" value=${q} onInput=${(e) => setQ(e.target.value)} />
      <div class="pill-tabs">
        <button class=${!tag ? "on" : ""} onClick=${() => setTag("")}>All</button>
        ${TAGS.map((t) => html`<button key=${t.id} class=${tag === t.id ? "on" : ""} onClick=${() => setTag(tag === t.id ? "" : t.id)}>${t.name}</button>`)}
        ${P.backstories.some((b) => b.custom) ? html`<button class=${tag === "mine" ? "on" : ""} onClick=${() => setTag(tag === "mine" ? "" : "mine")}>Yours</button>` : null}
      </div>
      <label class="row small"><input type="checkbox" checked=${onlyFit} onChange=${(e) => setOnlyFit(e.target.checked)} />Only ${n} player${n > 1 ? "s" : ""}</label>
    </div>
    <div class="story-grid">${open.map((b) => html`<${StoryCard} key=${b.id} b=${b} n=${n} on=${chosen === b.id} />`)}</div>
    ${!open.length ? html`<div class="hint">Nothing matches.</div>` : null}
    ${locked.length || packs.length ? html`<div class="locked-block">
      <h3>Needs another mod</h3>
      <div class="hint" style=${{ marginBottom: "10px" }}>These unlock when the mod is installed. Subscribe on the Workshop and they show up above.</div>
      ${packs.map((p) => {
        const list = locked.filter((b) => b.mod === p.id);
        return html`<div key=${p.id} class="pack">
          <div class="row"><b>${p.name}</b><span class="dim small grow">${p.adds}</span>
            <button class="btn sm ghost" onClick=${() => act(() => rpc("openURL", { url: workshopSearch(p.search) }))}>Find on Workshop</button></div>
          ${list.length ? html`<div class="story-grid">${list.map((b) => html`<${StoryCard} key=${b.id} b=${b} n=${n} />`)}</div>` : null}
        </div>`;
      })}
    </div>` : null}
  </div>`;
}

function StoryCard({ b, n, on }) {
  const P = S.presets;
  const bld = P.buildings.find((x) => x.id === b.building);
  const pack = b.mod && packOf(b);
  return html`<div class=${"scard" + (fits(b, n) ? " fit" : "") + (on ? " on" : "") + (b.locked ? " locked" : "")} onClick=${() => update({ modal: { kind: "story", id: b.id } })}>
    <div class="row"><b class="grow">${b.name}</b>${on ? html`<span class="chip accent">Picked</span>` : b.custom ? html`<span class="chip">${b.author ? "by " + b.author : "Yours"}</span>` : null}</div>
    <div class="scard-meta">
      <span class=${"chip " + (fits(b, n) ? "good" : "")}>${players(b)}</span>
      ${(b.tags || []).map((t) => html`<span key=${t} class="chip">${(TAGS.find((x) => x.id === t) || {}).name || t}</span>`)}
      ${pack ? html`<span class="chip warn">${pack.name}</span>` : null}
    </div>
    <p>${b.story}</p>
    <div class="dim small">${bld ? bld.name : b.building} · ${b.roles.length} role${b.roles.length > 1 ? "s" : ""}</div>
  </div>`;
}

// StoryDetail: shown in a popup when you click a story card.
export function StoryDetail({ id, close }) {
  const P = S.presets;
  const b = P && findStory(id);
  if (!b) return null;
  const host = isHost();
  const info = (host ? S.cfg.host : S.state && S.state.info) || {};
  const bld = P.buildings.find((x) => x.id === b.building);
  const lo = P.loadouts.find((x) => x.id === b.loadout);
  const pack = b.mod && packOf(b);
  const n = playerCount();
  const use = async () => {
    if ((await act(() => rpc("setBackstory", { id: b.id }))) === undefined) return;
    const cfg = await act(() => rpc("config"));
    if (cfg) update({ cfg });
    toast(b.name + " picked");
    close();
  };
  return html`<div class="story-detail">
    <div class="scard-meta"><span class=${"chip " + (fits(b, n) ? "good" : "")}>${players(b)}</span>${!fits(b, n) ? html`<span class="dim small">You have ${n}.</span>` : null}</div>
    <p class="story-text">${b.story}</p>
    <div class="facts">
      <div><span class="dim">Start</span><b>${bld ? bld.name : b.building}${b.town ? " in " + b.town : ""}</b></div>
      <div><span class="dim">Loadout</span><b>${lo ? lo.name : "Nothing"}</b>${lo && lo.items && lo.items.length ? html`<small class="dim">${lo.explain}</small>` : null}</div>
    </div>
    <h3>Roles</h3>
    <div class="grid-roles">${b.roles.map((r) => html`<${RoleCard} key=${r.name} r=${r} />`)}</div>
    <div class="hint">${b.tip}</div>
    <div class="row" style=${{ marginTop: "14px", justifyContent: "flex-end" }}>
      ${pack && b.locked ? html`<span class="chip warn grow">Needs ${pack.name}</span><button class="btn" onClick=${() => act(() => rpc("openURL", { url: workshopSearch(pack.search) }))}>Find on Workshop</button>` : null}
      ${!b.locked && host && info.backstory !== b.id ? html`<button class="btn primary" onClick=${use}>Use this story</button>` : null}
      ${!b.locked && host && info.backstory === b.id ? html`<button class="btn ghost" onClick=${async () => { await act(() => rpc("setBackstory", { id: "" })); S.cfg.host.backstory = ""; update(); close(); }}>Remove story</button>` : null}
    </div>
    <${StoryTools} b=${b} close=${close} />
  </div>`;
}

export function RoleCard({ r, holder, mine, children }) {
  return html`<div class=${"role" + (holder ? " taken" : "") + (mine ? " mine" : "")}>
    <div class="row"><b class="grow">${r.name}</b><span class="dim small">${r.profession ? profName(r.profession) : "Any job"}</span></div>
    <small>${r.note}</small>
    ${r.traits && r.traits.length ? html`<div class="role-traits">${r.traits.map((id) => {
      const t = S.cat && S.cat.traitById[id];
      return html`<span key=${id} class=${"chip " + (t && t.cost > 0 ? "good" : "bad")}>${traitName(id)}</span>`;
    })}</div>` : null}
    ${children}
  </div>`;
}

// StoryTools: edit, copy, share or delete a story. Built-in ones can only be copied.
function StoryTools({ b, close }) {
  const raw = b.custom && ((S.cfg && S.cfg.stories) || []).find((x) => x.id === b.id);
  const del = async () => {
    if (!confirm(`Delete ${b.name}?`)) return;
    if ((await act(() => rpc("deleteStory", { id: b.id }))) === undefined) return;
    const cfg = await act(() => rpc("config"));
    if (cfg) update({ cfg });
    const presets = await act(() => rpc("presets"));
    if (presets) update({ presets });
    toast(b.name + " deleted");
    close();
  };
  return html`<div class="row story-tools">
    ${raw ? html`<span class="dim small grow">${b.author ? "From " + b.author + ". It's yours now, so you can change it." : "Your story."}</span>` : html`<span class="grow"></span>`}
    ${raw ? html`<button class="btn sm ghost" onClick=${() => openEditor(copyStory(raw, true))}>Edit</button>` : null}
    ${!b.locked || raw ? html`<button class="btn sm ghost" onClick=${() => openEditor(copyStory(raw || b))}>Copy and edit</button>` : null}
    ${raw ? html`<button class="btn sm ghost" onClick=${() => copyCode("storyCode", { id: b.id })}>Copy code</button>` : null}
    ${raw ? html`<button class="btn sm ghost danger" onClick=${del}>Delete</button>` : null}
  </div>`;
}

// ChosenStory: the picked story with roles you can take. Taking one sets your job and traits.
export function ChosenStory({ story, info }) {
  const members = (S.state && S.state.members) || [];
  const lobby = inLobby();
  // claims come from the live lobby, so the host's own pick blocks the role for everyone else too
  const roles = (S.state && S.state.info && S.state.info.roles) || (info && info.roles) || {};
  const byId = Object.fromEntries(members.map((m) => [m.id, m]));
  const me = members.find((m) => m.me);
  const myRole = lobby ? me && me.role : S.cfg.role;
  const n = playerCount();
  return html`<div class="card story-chosen">
    <div class="row"><h3 class="grow" style=${{ margin: 0 }}>${story.name}</h3>
      <span class=${"chip " + (fits(story, n) ? "good" : "warn")}>${fits(story, n) ? players(story) : `Made for ${players(story)}, you have ${n}`}</span>
      <button class="btn sm ghost" onClick=${() => update({ modal: { kind: "story", id: story.id } })}>Details</button></div>
    <p class="story-text">${story.story}</p>
    <div class="grid-roles">
      ${story.roles.map((r) => {
        const holder = lobby && roles[r.name] && byId[roles[r.name]];
        const mine = myRole === r.name;
        return html`<${RoleCard} key=${r.name} r=${r} holder=${holder} mine=${mine}>
          <div class="row" style=${{ marginTop: "8px" }}>
            ${holder && !mine ? html`<span class="chip">${holder.name}</span>` : null}
            ${mine ? html`<span class="chip accent">You</span><button class="btn sm ghost" onClick=${() => takeRole(null)}>Drop</button>`
              : !holder ? html`<button class="btn sm" onClick=${() => takeRole(r)}>Take role</button>` : null}
          </div>
        </${RoleCard}>`;
      })}
    </div>
  </div>`;
}

// WhereYouStart: spawn building, respawn rule and the map.
export function WhereYouStart() {
  const host = isHost();
  const info = S.state && S.state.info;
  const h = host ? S.cfg.host : info || {};
  const P = S.presets;
  const [rolling, setRolling] = useState(false);
  if (!P) return null;
  const towns = (P.towns || []).map((s) => s.name);
  const reroll = async (patch) => {
    const town = patch && "spawnTown" in patch ? patch.spawnTown : h.spawnTown;
    const type = patch && "spawnType" in patch ? patch.spawnType : h.spawnType;
    setRolling(true);
    const spot = await act(() => rpc("randomHouse", { town, type }));
    setRolling(false);
    if (spot) {
      Object.assign(S.cfg.host, { spawn: spot, spawnTown: town, spawnType: type });
      update();
    }
  };
  const spot = h.spawn;
  return html`<section id="sec-start" class="section">
    <div class="section-head"><h2>Start</h2>${!host ? html`<span class="muted">Set by ${(S.state && S.state.hostName) || "the host"}.</span>` : null}</div>
    <div class="start-grid">
      <div class="grid">
        <div class="card">
          <h3>Spawn</h3>
          <${Toggle} title="Everyone spawns together" sub="Same building for everyone." on=${h.together} disabled=${!host} onChange=${(v) => patchHost({ together: v })} />
          ${h.together ? html`
            <div class="seg" style=${{ margin: "8px 0 12px" }}>
              <button class=${h.spawnMode !== "pick" ? "on" : ""} disabled=${!host} onClick=${() => patchHost({ spawnMode: "random" })}>Random</button>
              <button class=${h.spawnMode === "pick" ? "on" : ""} disabled=${!host} onClick=${() => patchHost({ spawnMode: "pick" })}>Pick on map</button>
            </div>
            ${h.spawnMode !== "pick" ? html`<div class="grid" style=${{ gap: "10px" }}>
              <label class="field"><span>Building</span>
                <${Select} disabled=${!host} options=${P.buildings.map((b) => ({ value: b.id, label: b.name }))} value=${h.spawnType || "house"} onChange=${(v) => reroll({ spawnType: v })} /></label>
              <label class="field"><span>Town</span>
                <${Select} disabled=${!host} options=${[{ value: "", label: "Anywhere" }, ...towns.map((t) => ({ value: t, label: t }))]} value=${h.spawnTown || ""} onChange=${(v) => reroll({ spawnTown: v })} /></label>
              ${host ? html`<button class="btn" disabled=${rolling} onClick=${() => reroll()}>${rolling ? "Rolling..." : "Reroll"}</button>` : null}
              <div class="hint">Rerolls after every game.</div>
            </div>` : html`<div class="hint">${host ? "Click a building on the map." : "The host picked a building on the map."}</div>`}
            ${spot ? html`<div class="spot-card"><b>${placeText(spot)}</b><div class="dim small">${spot.x}, ${spot.y}</div></div>` : null}
          ` : html`<div class="hint">Everyone picks a town on the normal spawn screen.</div>`}
        </div>
        <div class="card">
          <h3>When you die</h3>
          ${RESPAWN.map((r) => html`<div key=${r.id} class=${"pick-row" + (h.respawn === r.id || (!h.respawn && r.id === "base") ? " on" : "")} onClick=${() => host && patchHost({ respawn: r.id })}>
            <div class="radio"></div><div><b>${r.name}</b><small>${r.text}</small></div></div>`)}
          <label class="field" style=${{ marginTop: "10px" }}><span>Lives in this tape</span>
            <${Select} disabled=${!host} options=${LIVES.map((l) => ({ value: l.id, label: l.name }))} value=${livesOf(h).id}
              onChange=${(v) => { const l = LIVES.find((x) => x.id === v); patchHost({ lives: l.lives, livesShared: l.shared }); }} /></label>
          <div class="hint">${livesOf(h).text} When everyone is out, the tape ends.</div>
        </div>
      </div>
      <${SpawnMap} spot=${h.together ? spot : null} canPick=${host && h.together} height=${470} />
    </div>
  </section>`;
}
