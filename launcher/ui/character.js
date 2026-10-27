// Character page: job, traits and looks, with a 3D preview made from the game's own models.
import { html, useState, S, update, rpc, act, toast, saveCharacter, loadConfig, pick, rgbCss, sameColor, stripTags, profName, Select, Switch, PageHead, Empty } from "./core.js";
import { CharacterView } from "./viewer.js";
import { StorySection, WhereYouStart } from "./start.js";
import { castMe } from "./prev.js";
import { SkillList, skillOf, sortTraits } from "./skills.js";

const FIRST_M = ["Jack", "Wes", "Ray", "Dale", "Earl", "Travis", "Hank", "Cody", "Bill", "Floyd", "Gus", "Roy", "Luke", "Darryl"];
const FIRST_F = ["Jess", "Darla", "Kim", "June", "Tammy", "Lori", "Rose", "Faye", "Nina", "Becky", "Marla", "Joan", "Sue", "Ellie"];
const LAST = ["Haskell", "Bowman", "McCoy", "Pruitt", "Dawson", "Whitaker", "Gentry", "Crowe", "Mercer", "Sutton", "Hale", "Royce", "Tanner", "Blevins"];

export function freePoints() {
  const info = S.state && S.state.info;
  return (info && info.freePoints) || 0;
}

export function points(c) {
  const prof = S.cat.profById[c.profession];
  let pts = (prof ? prof.cost : 0) + freePoints();
  for (const id of c.traits || []) {
    const t = S.cat.traitById[id];
    if (t) pts -= t.cost;
  }
  return pts;
}

export function grantedTraits(c) {
  const out = new Set();
  const prof = S.cat.profById[c.profession];
  if (prof) for (const g of prof.granted || []) out.add(g);
  for (const id of c.traits || []) {
    const t = S.cat.traitById[id];
    if (t) for (const g of t.granted || []) out.add(g);
  }
  return out;
}

function blockedBy(c, t) {
  const have = new Set([...(c.traits || []), ...grantedTraits(c)]);
  if (have.has(t.id)) return null;
  for (const id of have) {
    const o = S.cat.traitById[id];
    if ((t.exclusive || []).includes(id) || (o && (o.exclusive || []).includes(t.id))) return o ? o.name : id;
  }
  const info = S.state && S.state.info;
  if (t.sleep && info && !info.sleep) return "sleep is off";
  if (t.noMP) return "off in multiplayer";
  return null;
}

function pref(k, d) {
  try { return localStorage.getItem(k) || d; } catch { return d; }
}

export function xpOf(c) {
  const xp = {};
  const prof = S.cat.profById[c.profession];
  for (const src of [prof, ...(c.traits || []).map((id) => S.cat.traitById[id])]) for (const x of (src && src.xp) || []) {
    const s = skillOf(x);
    xp[s.id] = xp[s.id] || { ...s, level: 0 };
    xp[s.id].level += x.level;
  }
  return xp;
}

function clothingOptions(c) {
  const g = c.female ? "Female" : "Male";
  const defs = S.cat.looks.clothing || {};
  const merged = {};
  const add = (def) => {
    if (!def) return;
    for (const [loc, v] of Object.entries(def)) {
      merged[loc] = merged[loc] || { name: v.name, items: [] };
      for (const it of v.items || []) if (!merged[loc].items.find((x) => x.id === it.id)) merged[loc].items.push(it);
    }
  };
  add(defs.default && defs.default[g]);
  const prof = defs[c.profession];
  if (prof) add(prof[g] && Object.keys(prof[g]).length ? prof[g] : prof.Female);
  return merged;
}

function randomOutfit(c) {
  c.clothing = {};
  c.clothingTint = {};
  for (const [loc, v] of Object.entries(clothingOptions(c))) {
    if (v.items.length && (loc !== "Hat" || Math.random() < 0.3) && (loc !== "Eyes" || Math.random() < 0.2)) c.clothing[loc] = pick(v.items).id;
  }
  if (c.clothing.Dress) { delete c.clothing.Pants; delete c.clothing.Skirt; }
  if (c.clothing.Skirt) delete c.clothing.Pants;
  if (c.clothing.TankTop && c.clothing.Tshirt) delete c.clothing.TankTop;
}

export function ensureDefaults(c) {
  const L = S.cat.looks;
  c.traits = c.traits || [];
  c.clothing = c.clothing || {};
  c.clothingTint = c.clothingTint || {};
  let changed = false;
  if (!c.profession || !S.cat.profById[c.profession]) { c.profession = S.cat.profById.unemployed ? "unemployed" : (S.cat.professions[0] || {}).id; changed = true; }
  if (!c.skin || (!c.skin.r && !c.skin.g)) { c.skin = L.skinColors[1]; changed = true; }
  if (!c.hairColor || (!c.hairColor.r && !c.hairColor.g)) { c.hairColor = (L.hairColors || [])[0] || { r: 0.3, g: 0.2, b: 0.1 }; changed = true; }
  if (!c.hair) { c.hair = ((c.female ? L.hairFemale : L.hairMale) || []).find((h) => h.model && !h.hidden)?.id || ""; changed = true; }
  if (!c.forename) { c.forename = pick(c.female ? FIRST_F : FIRST_M); c.surname = pick(LAST); changed = true; }
  if (!Object.keys(c.clothing).length) { randomOutfit(c); changed = true; }
  return changed;
}

export function Character() {
  const [search, setSearch] = useState("");
  const [lookTab, setLookTab] = useState("body");
  const [hideBlocked, setHideBlocked] = useState(false);
  if (!S.cat) return html`<${Empty} title="Loading game files...">One sec.</${Empty}>`;
  const c = S.cfg.character;
  if (ensureDefaults(c)) saveCharacter();
  const L = S.cat.looks;
  const presets = Object.keys(S.cfg.presets || {}).sort();
  const pts = points(c);
  const granted = [...grantedTraits(c)].map((id) => S.cat.traitById[id]).filter(Boolean);
  const xp = xpOf(c);
  const change = (f) => { f(); saveCharacter(); };
  const setFemale = (f) => change(() => {
    if (c.female === f) return;
    c.female = f;
    c.hair = ((f ? L.hairFemale : L.hairMale) || []).find((h) => h.model && !h.hidden)?.id || "";
    if (f) c.beard = "";
    c.forename = pick(f ? FIRST_F : FIRST_M);
    randomOutfit(c);
  });

  const q = search.toLowerCase();
  const traitSort = pref("knox-trait-sort", "group");
  const traitFrom = pref("knox-trait-from", "all");
  const hasModTraits = S.cat.traits.some((t) => t.mod && !t.free);
  const visible = (t) => !t.free && t.cost !== 0 && (!q || t.name.toLowerCase().includes(q))
    && (traitFrom === "all" || !hasModTraits || (traitFrom === "mods") === !!t.mod)
    && (!hideBlocked || c.traits.includes(t.id) || !blockedBy(c, t));
  const good = sortTraits(S.cat.traits.filter((t) => visible(t) && t.cost > 0), traitSort);
  const bad = sortTraits(S.cat.traits.filter((t) => visible(t) && t.cost < 0), traitSort);
  const traitGroups = (groups) => groups.map(([g, list]) => html`${g ? html`<div key=${"h" + g} class="trait-head">${g}</div>` : null}${list.map(traitRow)}`);
  const setSort = (k, v) => { try { localStorage.setItem(k, v); } catch {} update(); };
  const traitRow = (t) => {
    const on = c.traits.includes(t.id);
    const blocked = on ? null : blockedBy(c, t);
    return html`<div key=${t.id} class=${"trait" + (on ? " on" : "") + (t.cost < 0 ? " neg" : "") + (blocked ? " off" : "")}
      title=${stripTags(t.desc) + (blocked ? `\n\nBlocked by: ${blocked}` : "")}
      onClick=${() => !blocked && change(() => { c.traits = on ? c.traits.filter((x) => x !== t.id) : [...c.traits, t.id]; })}>
      <span class="tn">${t.name}${t.mod ? html` <small class="dim">(${t.mod})</small>` : null}</span>
      <span class=${"cost " + (t.cost > 0 ? "neg" : "pos")}>${t.cost > 0 ? "-" + t.cost : "+" + -t.cost}</span>
    </div>`;
  };
  const profs = S.cat.professions.slice().sort((a, b) => (a.id === "unemployed" ? -1 : b.id === "unemployed" ? 1 : a.name.localeCompare(b.name)));
  const opts = clothingOptions(c);
  const hairs = ((c.female ? L.hairFemale : L.hairMale) || []).filter((h) => !h.hidden);
  const tab = S.charTab || "survivor";

  return html`
    <${PageHead} title="Survivor & start" sub="Your survivor, the story and where you spawn.">
      <${Select} options=${[{ value: "", label: presets.length ? "Load character..." : "No saved characters" }, ...presets]} value="" onChange=${async (v) => {
        if (!v) return;
        const ch = await act(() => rpc("loadPreset", { name: v }), "Loaded " + v);
        if (ch) { S.cfg.character = ch; update(); }
      }} />
      <button class="btn" onClick=${async () => {
        const name = prompt("Save as", `${c.forename} ${c.surname}`.trim());
        if (!name) return;
        await act(() => rpc("saveCharacter", { character: c }));
        if (await act(() => rpc("savePreset", { name }), "Saved " + name) !== undefined) loadConfig();
      }}>Save</button>
      ${presets.length ? html`<button class="btn ghost" onClick=${async () => {
        const name = prompt("Delete which one?\n" + presets.join(", "));
        if (name && await act(() => rpc("deletePreset", { name }), "Deleted") !== undefined) loadConfig();
      }}>Delete</button>` : null}
    </${PageHead}>
    <div class="subnav tabs-char">
      ${CHAR_TABS.map(([k, l]) => html`<a key=${k} class=${tab === k ? "on" : ""} onClick=${() => update({ charTab: k })}>${l}${k === "traits" ? html` <span class=${"chip " + (pts < 0 ? "bad" : "good")}>${pts}</span>` : null}</a>`)}
      ${S.cfg.role ? html`<span class="chip accent" style=${{ marginLeft: "auto" }}>Role: ${S.cfg.role}</span>` : null}
    </div>
    ${(() => {
      // continuing a tape where you're still alive: PZ loads that survivor, this one is only for a new life
      const info = S.state && S.state.info;
      const m = info && info.cast && castMe(info.cast);
      return m && m.alive && tab === "survivor" ? html`<div class="card row tape-surv"><span class="chip good">ALIVE</span>
        <span class="grow">In <b>${info.world}</b> you're <b>${m.name}</b> (day ${m.lastDay}, ${m.kills} kills). You load back in as them. What you set here is only used if you need a new survivor.</span></div>` : null;
    })()}
    ${tab === "story" ? html`<${StorySection} />` : null}
    ${tab === "start" ? html`<${WhereYouStart} />` : null}
    ${tab === "survivor" ? html`<section id="sec-char" class="section"><div class="char-grid two">
      <div class="grid">
        <div class="card">
          <h3>Who</h3>
          <div class="grid" style=${{ gap: "10px" }}>
            <div class="seg">
              <button class=${c.female ? "" : "on"} onClick=${() => setFemale(false)}>Male</button>
              <button class=${c.female ? "on" : ""} onClick=${() => setFemale(true)}>Female</button>
            </div>
            <div class="row">
              <input type="text" class="grow" value=${c.forename} placeholder="First name" onInput=${(e) => change(() => (c.forename = e.target.value))} />
              <input type="text" class="grow" value=${c.surname} placeholder="Last name" onInput=${(e) => change(() => (c.surname = e.target.value))} />
            </div>
            <button class="btn sm" onClick=${() => change(() => { c.forename = pick(c.female ? FIRST_F : FIRST_M); c.surname = pick(LAST); })}>Random name</button>
          </div>
        </div>
        <div class="card">
          <h3>Job</h3>
          <div class="prof-list">
            ${profs.map((p) => html`<div key=${p.id} class=${"prof" + (c.profession === p.id ? " on" : "")} title=${stripTags(p.desc)}
              onClick=${() => change(() => {
                c.profession = p.id;
                const g = new Set(p.granted || []);
                c.traits = c.traits.filter((id) => !g.has(id) && S.cat.traitById[id]);
                randomOutfit(c);
              })}>
              <span class="pn">${p.name}${p.mod ? html` <small class="dim">mod</small>` : null}</span>
              <span class=${"cost " + (p.cost > 0 ? "pos" : p.cost < 0 ? "neg" : "")}>${p.cost > 0 ? "+" + p.cost : p.cost}</span>
            </div>`)}
          </div>
        </div>
      </div>
      <div class="grid">
        <div class="doll">
          <${CharacterView} c=${c} height=${340} controls />
          <div class="doll-name">${c.forename} ${c.surname}</div>
          <div class="muted small">${profName(c.profession)}</div>
        </div>
        <div class="card">
          <div class="row"><h3 class="grow" style=${{ margin: 0 }}>Traits</h3><span class=${"chip " + (pts < 0 ? "bad" : "good")}>${pts} points left</span><button class="btn sm" onClick=${() => update({ charTab: "traits" })}>Change</button></div>
          <div class="chosen" style=${{ marginTop: "10px" }}>
            ${granted.map((t) => html`<span key=${"g" + t.id} class="chip" title="From your job">★ ${t.name}</span>`)}
            ${c.traits.map((id) => S.cat.traitById[id]).filter(Boolean).map((t) => html`<span key=${t.id} class=${"chip " + (t.cost > 0 ? "good" : "bad")}>${t.name}</span>`)}
            ${!granted.length && !c.traits.length ? html`<span class="hint">No traits.</span>` : null}
          </div>
          <${SkillList} xp=${xp} />
        </div>
      </div>
    </div></section>` : null}
    ${tab === "traits" ? html`<section class="section"><div class="grid">
        <div class="card">
          <div class="row" style=${{ alignItems: "flex-end" }}>
            <div><h3 style=${{ marginBottom: "2px" }}>Points left</h3><div class=${"points " + (pts < 0 ? "bad" : "good")}>${pts}</div></div>
            <div class="grow"></div>
            <input type="text" placeholder="Search traits" value=${search} onInput=${(e) => setSearch(e.target.value)} />
            <button class="btn sm ghost" onClick=${() => change(() => (c.traits = []))}>Clear</button>
          </div>
          ${freePoints() ? html`<div class="hint" style=${{ marginTop: "6px" }}>+${freePoints()} from world settings.</div>` : null}
          ${pts < 0 ? html`<div class="chip bad" style=${{ marginTop: "8px" }}>Not enough points. The game won't accept this.</div>` : null}
          <div class="row trait-sort">
            <span class="muted small">Sort</span>
            <div class="seg">${[["group", "Group"], ["cost", "Cost"], ["name", "A-Z"]].map(([k, l]) => html`<button key=${k} class=${traitSort === k ? "on" : ""} onClick=${() => setSort("knox-trait-sort", k)}>${l}</button>`)}</div>
            ${hasModTraits ? html`<div class="seg">${[["all", "All"], ["game", "Game"], ["mods", "Mods"]].map(([k, l]) => html`<button key=${k} class=${traitFrom === k ? "on" : ""} onClick=${() => setSort("knox-trait-from", k)}>${l}</button>`)}</div>` : null}
            <label class="row small muted" style=${{ gap: "6px" }}><${Switch} on=${hideBlocked} onChange=${setHideBlocked} /> Hide blocked</label>
          </div>
          <div class="sep"></div>
          <div class="chosen">
            ${granted.map((t) => html`<span key=${"g" + t.id} class="chip" title="From your job">★ ${t.name}</span>`)}
            ${c.traits.map((id) => S.cat.traitById[id]).filter(Boolean).map((t) => html`<span key=${t.id} class=${"chip click " + (t.cost > 0 ? "good" : "bad")} onClick=${() => change(() => (c.traits = c.traits.filter((x) => x !== t.id)))}>${t.name} ✕</span>`)}
            ${!granted.length && !c.traits.length ? html`<span class="hint">No traits.</span>` : null}
          </div>
          <${SkillList} xp=${xp} />
        </div>
        <div class="trait-cols">
          <div class="card"><h3>Good traits</h3><div class="trait-list">${traitGroups(good)}</div></div>
          <div class="card"><h3>Bad traits</h3><div class="trait-list">${traitGroups(bad)}</div></div>
        </div>
    </div></section>` : null}
    ${tab === "look" ? html`<section class="section"><div class="char-grid two look">
      <div class="grid">
        <div class="doll">
          <${CharacterView} c=${c} height=${340} controls />
          <div class="doll-name">${c.forename} ${c.surname}</div>
          <div class="muted small">${profName(c.profession)}</div>
        </div>
      </div>
        <div class="card">
          <div class="pill-tabs" style=${{ marginBottom: "12px" }}>
            <button class=${lookTab === "body" ? "on" : ""} onClick=${() => setLookTab("body")}>Body & hair</button>
            <button class=${lookTab === "clothes" ? "on" : ""} onClick=${() => setLookTab("clothes")}>Clothes</button>
          </div>
          ${lookTab === "body" ? html`<div class="grid" style=${{ gap: "12px" }}>
            <label class="field"><span>Skin</span><div class="swatches">${L.skinColors.map((col, i) => html`<div key=${i} class=${"swatch" + (sameColor(col, c.skin) ? " on" : "")} style=${{ background: rgbCss(col) }} onClick=${() => change(() => (c.skin = col))}></div>`)}</div></label>
            <label class="field"><span>Hair</span><${Select} options=${hairs.map((x) => ({ value: x.id, label: x.name }))} value=${c.hair} onChange=${(v) => change(() => (c.hair = v))} /></label>
            <label class="field"><span>Hair colour</span><div class="swatches">${(L.hairColors || []).map((col, i) => html`<div key=${i} class=${"swatch" + (sameColor(col, c.hairColor) ? " on" : "")} style=${{ background: rgbCss(col) }} onClick=${() => change(() => (c.hairColor = col))}></div>`)}
              ${extras().some((e) => e.kind === "hairPicker") ? html`<input type="color" class="swatch-pick" title="Any colour (Improved Hair Menu)" value=${toHex(c.hairColor)} onInput=${(e) => change(() => (c.hairColor = fromHex(e.target.value)))} />` : null}</div></label>
            ${!c.female ? html`<label class="field"><span>Beard</span><${Select} options=${L.beards.map((x) => ({ value: x.id, label: x.name }))} value=${c.beard || ""} onChange=${(v) => change(() => (c.beard = v))} /></label>` : null}
            <div class="row">
              <label class="row small"><${Switch} on=${c.stubble} onChange=${(v) => change(() => (c.stubble = v))} />Stubble</label>
              ${!c.female ? html`<label class="row small"><${Switch} on=${c.chestHair} onChange=${(v) => change(() => (c.chestHair = v))} />Chest hair</label>` : null}
            </div>
            <label class="field"><span>Voice</span><${Select} options=${[{ value: "", label: "Random" }, ...(L.voices || []).filter((v) => !v.body || v.body === (c.female ? "1" : "2")).map((x) => ({ value: x.id, label: x.name }))]} value=${c.voice || ""} onChange=${(v) => change(() => (c.voice = v))} /></label>
            <${ModSliders} c=${c} change=${change} />
          </div>` : html`<div class="grid" style=${{ gap: "10px" }}>
            <button class="btn sm" onClick=${() => change(() => randomOutfit(c))}>Random outfit</button>
            ${Object.entries(opts).sort((a, b) => a[1].name.localeCompare(b[1].name)).map(([loc, v]) => {
              const look = c.clothing[loc] && S.cat.clothing[c.clothing[loc]];
              const tintable = look && (look.tint || look.hue);
              return html`<label key=${loc} class="field"><span>${v.name || loc}</span>
                <div class="row">
                  <${Select} class="grow" options=${[{ value: "", label: "None" }, ...v.items.map((it) => ({ value: it.id, label: it.name }))]} value=${c.clothing[loc] || ""}
                    onChange=${(val) => change(() => { if (val) c.clothing[loc] = val; else delete c.clothing[loc]; })} />
                  ${tintable ? html`<input type="color" title="Colour" value=${toHex(c.clothingTint[loc])} onInput=${(e) => change(() => (c.clothingTint[loc] = fromHex(e.target.value)))} />` : null}
                </div></label>`;
            })}
          </div>`}
        </div>
    </div></section>` : null}`;
}

const CHAR_TABS = [["survivor", "Survivor"], ["traits", "Traits"], ["look", "Look"], ["story", "Story"], ["start", "Start"]];

function jump(id) {
  const el = document.getElementById(id);
  if (el) el.scrollIntoView({ behavior: "smooth", block: "start" });
}

const extras = () => (S.presets && S.presets.extras) || [];

// ModSliders: options that character-creator mods add (Size Matters height and build).
function ModSliders({ c, change }) {
  const list = extras().filter((e) => e.kind === "slider");
  if (!list.length) return null;
  c.extras = c.extras || {};
  const byMod = {};
  for (const e of list) (byMod[e.modName] = byMod[e.modName] || []).push(e);
  return html`${Object.entries(byMod).map(([mod, items]) => html`<div key=${mod} class="mod-extras">
    <div class="mod-extras-head">${mod}</div>
    ${items.map((e) => {
      const v = c.extras[e.key] ?? e.default;
      return html`<label key=${e.key} class="field" title=${e.hint || ""}><span>${e.label} <b class="ev">${v}${e.unit ? " " + e.unit : ""}</b></span>
        <input type="range" min=${e.min} max=${e.max} step=${e.step} value=${v} onInput=${(ev) => change(() => (c.extras = { ...c.extras, [e.key]: parseFloat(ev.target.value) }))} />
        ${e.hint ? html`<small class="dim">${e.hint}</small>` : null}</label>`;
    })}
  </div>`)}`;
}

function toHex(c) {
  if (!c) return "#bfbfbf";
  const h = (v) => Math.round(v * 255).toString(16).padStart(2, "0");
  return "#" + h(c.r) + h(c.g) + h(c.b);
}
function fromHex(s) {
  return { r: parseInt(s.slice(1, 3), 16) / 255, g: parseInt(s.slice(3, 5), 16) / 255, b: parseInt(s.slice(5, 7), 16) / 255 };
}

// CharacterCard: read-only summary used when you inspect a friend in the lobby.
export function CharacterSheet({ c, name }) {
  if (!c || !S.cat) return html`<div class="muted">No character yet.</div>`;
  const xp = xpOf(c);
  const granted = [...grantedTraits(c)].map((id) => S.cat.traitById[id]).filter(Boolean);
  const traits = (c.traits || []).map((id) => S.cat.traitById[id] || { id, name: id, cost: 0 });
  return html`<div class="sheet">
    <div class="doll"><${CharacterView} c=${c} height=${380} small /></div>
    <div class="sheet-info">
      <div class="sheet-name">${c.forename} ${c.surname}</div>
      <div class="muted">${name ? name + " · " : ""}${profName(c.profession)} · ${c.female ? "Female" : "Male"}</div>
      <div class="sep"></div>
      <h3>Traits</h3>
      <div class="chosen">
        ${granted.map((t) => html`<span key=${"g" + t.id} class="chip" title=${stripTags(t.desc)}>★ ${t.name}</span>`)}
        ${traits.map((t) => html`<span key=${t.id} class=${"chip " + (t.cost > 0 ? "good" : "bad")} title=${stripTags(t.desc)}>${t.name}</span>`)}
        ${!granted.length && !traits.length ? html`<span class="hint">No traits.</span>` : null}
      </div>
      ${Object.values(xp).some((s) => s.level) ? html`<h3 style=${{ marginTop: "14px" }}>Skills</h3><${SkillList} xp=${xp} />` : null}
      <h3 style=${{ marginTop: "14px" }}>Wearing</h3>
      <div class="chosen">${Object.values(c.clothing || {}).filter(Boolean).map((id) => html`<span key=${id} class="chip">${(S.cat.itemById[id] || {}).name || id}</span>`)}</div>
    </div>
  </div>`;
}
