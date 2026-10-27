// Customize: every look in one place. Tabs for the tape, your lobby poster, the shelf and the launcher.
// Each row is picture cards: Surprise me, the free picks, what your level unlocked, then what's still locked.
import { html, useState, useEffect, S, update, rpc, act, inLobby } from "./core.js";
import { CaseView, ShelfView, shelfAspect } from "./vhs3d.js";
import { looksDefs, unlockLevel, myTape, spineOpts } from "./looks.js";
import { PosterFig } from "./poster.js";
import { collectorName } from "./tape.js";
import { drawCover, drawBackdrop, drawSpine, drawPoster, fonts, SPINE_INKS, TITLE_FONTS, STUDIO_NAMES, LABEL_INKS, CASE_INKS, W, H, SPINE } from "./tape.js";
import { sound, soundOn, setSound, soundVol, setVol } from "./sound.js";

const hex = (n) => "#" + n.toString(16).padStart(6, "0");
const ACCENTS = { red: "#e3241b", amber: "#e08a1a", green: "#3aa63a", blue: "#2a6ae3" };
const TINTS = { warm: "linear-gradient(135deg,#ff9a3c,#7a2a0a)", cold: "linear-gradient(135deg,#7fb0ff,#0a2a5a)", green: "linear-gradient(135deg,#8be38b,#0a3a14)", bw: "linear-gradient(135deg,#eee,#222)" };
const NAMES = {
  titleFont: { block: "Block", typewriter: "Typewriter", marker: "Marker", stencil: "Stencil", serif: "Serif" },
  tint: { warm: "Warm", cold: "Cold", green: "Night vision", bw: "Black and white" },
  rating: { G: "G", PG: "PG", PG13: "PG-13", R: "R", UNRATED: "Unrated" },
  studio: { knox: "Knox Pictures", muldraugh: "Muldraugh Home Video", westpoint: "West Point Films", riverside: "Riverside Releasing" },
  labelInk: { white: "White", cream: "Cream", yellow: "Yellow", pink: "Pink", blue: "Blue" },
  caseInk: { grey: "Grey", white: "White" },
  plate: { dogtag: "Dog tag", nametape: "Name tape", band: "Hospital band" },
  light: { warm: "Warm", cold: "Cold", candle: "Candle" },
  sort: { played: "Last played", name: "Name", solo: "Solo first", squad: "Squad first" },
  accent: { red: "Red", amber: "Amber", green: "Green", blue: "Blue" },
};
const free = (kind) => (((looksDefs() || {}).free || {})[kind] || []).map((id) => ({ id, name: (NAMES[kind] || {})[id] || id, lvl: 1 }));

// thumbs are painted once and kept as pictures
const thumbs = new Map();
function Thumb({ k, paint, w = 90, h = 157 }) {
  const [src, setSrc] = useState(thumbs.get(k) || "");
  useEffect(() => {
    if (thumbs.has(k)) { setSrc(thumbs.get(k)); return; }
    let alive = true;
    (async () => {
      await fonts();
      const c = await paint();
      const out = document.createElement("canvas");
      out.width = w * 2; out.height = h * 2;
      out.getContext("2d").drawImage(c, 0, 0, out.width, out.height);
      const url = out.toDataURL("image/jpeg", 0.85);
      thumbs.set(k, url);
      if (alive) setSrc(url);
    })();
    return () => { alive = false; };
  }, [k]);
  return html`<div class="cz-thumb" style=${{ width: w + "px", height: h + "px", backgroundImage: src ? `url(${src})` : "" }}></div>`;
}
const canvas = (w, h) => { const c = document.createElement("canvas"); c.width = w; c.height = h; return c; };
const coverThumb = (o, crop) => async () => {
  const c = (await drawCover({ title: "KNOX", genre: "horror", backdrop: "woods", seed: "thumb", cast: [], stats: [], ...o })).canvas;
  if (!crop) return c;
  const [x, y, w, h] = crop, out = canvas(w, h);
  out.getContext("2d").drawImage(c, x, y, w, h, 0, 0, w, h);
  return out;
};
// caseThumb: a cover inside the case you picked (flat)
const caseThumb = (kind, ink) => async () => {
  const c = await coverThumb({})();
  const out = canvas(W, H), x = out.getContext("2d");
  const p = kind === "rental" ? 60 : kind === "clamshell" ? 22 : 0;
  x.fillStyle = hex(CASE_INKS[ink || ""]); x.fillRect(0, 0, W, H);
  x.drawImage(c, p, p, W - p * 2, H - p * 2);
  if (kind === "slipcover") { x.fillStyle = "rgba(210,190,150,.18)"; x.fillRect(0, 0, W, H); x.fillStyle = "rgba(0,0,0,.4)"; x.fillRect(0, 0, 18, H); }
  if (kind === "shrink" || kind === "clamshell") {
    const g = x.createLinearGradient(0, 0, W, H); g.addColorStop(0.2, "rgba(255,255,255,0)"); g.addColorStop(0.35, "rgba(255,255,255,.35)"); g.addColorStop(0.5, "rgba(255,255,255,0)");
    x.fillStyle = g; x.fillRect(0, 0, W, H);
  }
  return out;
};

// Row: one set of picture cards. items: [{ id, name, lvl, thumb?, swatch?, sample? }]
function Row({ title, hint, items, value, onPick, lvl, mix, none, multi }) {
  const isOn = (id) => (multi ? (value || []).includes(id) : (value || "") === id);
  const sorted = [...items].sort((a, b) => {
    const g = (u) => (u.lvl <= 1 ? 0 : u.lvl <= lvl ? 1 : 2);
    return g(a) - g(b) || (g(a) === 2 ? a.lvl - b.lvl : 0);
  });
  const next = Math.min(...items.filter((u) => u.lvl > lvl).map((u) => u.lvl));
  const card = (u, locked) => html`<button key=${u.id} class=${"cz-card" + (isOn(u.id) ? " on" : "") + (locked ? " locked" : "") + (locked && u.lvl === next ? " next" : "")}
      disabled=${locked} title=${locked ? `Unlocks at level ${u.lvl}` : ""} onClick=${() => { sound("button"); onPick(u.id); }}>
    ${u.thumb ? html`<${Thumb} ...${u.thumb} />` : u.swatch ? html`<div class="cz-swatch" style=${{ background: u.swatch }}></div>` : u.sample ? html`<div class="cz-sample" style=${u.sampleStyle || {}}>${u.sample}</div>` : null}
    <span class="cz-name">${u.name}</span>
    ${locked ? html`<span class="cz-lock">🔒 Lv ${u.lvl}${u.lvl === next ? " · next" : ""}</span>` : null}
  </button>`;
  return html`<div class="cz-row">
    <div class="cz-row-head"><h4>${title}</h4>${hint ? html`<span class="dim small">${hint}</span>` : null}</div>
    <div class="cz-cards">
      ${mix ? html`<button class=${"cz-card" + ((value || "") === mix.id ? " on" : "")} onClick=${() => onPick(mix.id)}><div class="cz-sample cz-mix">?</div><span class="cz-name">${mix.name || "Surprise me"}</span></button>` : null}
      ${none ? html`<button class=${"cz-card" + (!value || (multi && !value.length) ? " on" : "")} onClick=${() => onPick("")}>${none.swatch ? html`<div class="cz-swatch" style=${{ background: none.swatch }}></div>` : html`<div class="cz-sample">${none.sample || "—"}</div>`}<span class="cz-name">${none.name}</span></button>` : null}
      ${sorted.map((u) => card(u, u.lvl > lvl))}
    </div>
  </div>`;
}

// Text: a short text pick, saved when you leave the box or press Enter
function Text({ title, hint, value, max, onSave, placeholder }) {
  const [v, setV] = useState(value || "");
  useEffect(() => setV(value || ""), [value]);
  const save = () => v !== (value || "") && onSave(v.trim());
  return html`<div class="cz-row"><div class="cz-row-head"><h4>${title}</h4>${hint ? html`<span class="dim small">${hint}</span>` : null}</div>
    <input class="cz-text" maxlength=${max} placeholder=${placeholder || ""} value=${v} onInput=${(e) => setV(e.target.value)} onBlur=${save} onKeyDown=${(e) => e.key === "Enter" && e.target.blur()} /></div>`;
}

const inkItems = (d) => (d.spineInks || []).map((u) => ({ ...u, lvl: 1, swatch: SPINE_INKS[u.id] }));

// TapeRows: everything on the box. Used here for your default cover and in the TapeMaker for one tape.
export function TapeRows({ look, set, lvl, onlyCover }) {
  const d = looksDefs() || {};
  const k = S.knox || {};
  const g = look.genre && look.genre !== "mix" ? look.genre : "horror";
  const bd = look.backdrop && look.backdrop !== "mix" ? look.backdrop : "woods";
  const toggle = (field) => (id) => { const cur = look[field] || []; set({ [field]: cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id] }); };
  return html`
    <h3 class="cz-sect">Front</h3>
    <${Row} title="Cover style" lvl=${lvl} value=${look.genre} onPick=${(id) => set({ genre: id })} mix=${{ id: "mix" }}
      items=${(d.genres || []).map((u) => ({ ...u, thumb: { k: "g" + u.id, paint: coverThumb({ genre: u.id, backdrop: "woods" }) } }))} />
    <${Row} title="Backdrop" lvl=${lvl} value=${look.backdrop} onPick=${(id) => set({ backdrop: id })} mix=${{ id: "mix" }}
      items=${(d.backdrops || []).map((u) => ({ ...u, thumb: { k: "b" + u.id, w: 90, h: 120, paint: async () => { const c = canvas(300, 400); await drawBackdrop(c.getContext("2d"), u.id, 300, 400, "thumb"); return c; } } }))} />
    <${Row} title="Title font" hint="Free" lvl=${lvl} value=${look.titleFont} onPick=${(id) => set({ titleFont: id })} none=${{ name: "Cover's own", sample: "Aa" }}
      items=${free("titleFont").map((u) => ({ ...u, sample: "Aa", sampleStyle: { font: TITLE_FONTS[u.id].replace("%", "30") } }))} />
    <${Row} title="Title colour" hint="Free" lvl=${lvl} value=${look.titleInk} onPick=${(id) => set({ titleInk: id })} none=${{ name: "Cover's own", sample: "Aa" }} items=${inkItems(d)} />
    <${Row} title="Print colour" hint="Free" lvl=${lvl} value=${look.tint} onPick=${(id) => set({ tint: id })} none=${{ name: "Normal", sample: "—" }}
      items=${free("tint").map((u) => ({ ...u, swatch: TINTS[u.id] }))} />
    ${onlyCover ? null : html`<${Text} title="Tagline" hint="Free · your own line on the cover" max="60" value=${look.tagline} placeholder="Leave empty for a random one" onSave=${(v) => set({ tagline: v })} />`}
    <${Row} title="Studio" hint="Free" lvl=${lvl} value=${look.studio} onPick=${(id) => set({ studio: id })} none=${{ name: "Knox Launcher", sample: "KL" }}
      items=${free("studio").map((u) => ({ ...u, sample: STUDIO_NAMES[u.id].split(" ").map((w) => w[0]).join(""), sampleStyle: { font: "700 22px Oswald" } }))} />
    <${Row} title="Rating" hint="Free" lvl=${lvl} value=${look.rating} onPick=${(id) => set({ rating: id })} none=${{ name: "None", sample: "—" }}
      items=${free("rating").map((u) => ({ ...u, sample: u.name, sampleStyle: { font: "700 22px Oswald", background: "#f4f1ea", color: "#111" } }))} />
    <h3 class="cz-sect">Stickers</h3>
    <${Row} title="Stickers" hint="Pick as many as you like" lvl=${lvl} value=${look.stickers} multi onPick=${(id) => (id ? toggle("stickers")(id) : set({ stickers: [] }))} none=${{ name: "None" }}
      items=${(d.stickers || []).map((u) => ({ ...u, thumb: { k: "st" + u.id, w: 90, h: 90, paint: coverThumb({ stickers: [u.id] }, [W - 230, 40, 230, 230]) } }))} />
    <div class="cz-pair">
      <${Text} title="Price sticker" hint="Free" max="8" value=${look.price} placeholder="$2.99" onSave=${(v) => set({ price: v })} />
      <div class="cz-row"><div class="cz-row-head"><h4>Be Kind Rewind</h4><span class="dim small">Free</span></div>
        <div class="cz-cards"><button class=${"cz-card" + (look.rewind ? " on" : "")} onClick=${() => set({ rewind: !look.rewind })}><div class="cz-sample" style=${{ background: "#3b86ff", color: "#fff", font: "700 12px Oswald" }}>BE KIND</div><span class="cz-name">${look.rewind ? "On" : "Off"}</span></button></div></div>
    </div>
    <h3 class="cz-sect">Spine</h3>
    <${Row} title="Spine" lvl=${lvl} value=${look.spine} onPick=${(id) => set({ spine: id })} none=${{ name: "Mix of mine", sample: "?" }}
      items=${(d.spines || []).map((u) => ({ ...u, thumb: { k: "sp" + u.id, w: 30, h: 157, paint: () => drawSpine({ style: u.id, title: "KNOX", sub: "PART II", seed: "thumb", genre: "horror" }) } }))} />
    <${Row} title="Spine colour" lvl=${lvl} value=${look.spineInk} onPick=${(id) => set({ spineInk: id })} mix=${{ id: "mix" }} none=${{ name: "Match cover", sample: "=" }}
      items=${(d.spineInks || []).map((u) => ({ ...u, swatch: SPINE_INKS[u.id] }))} />
    ${onlyCover ? null : html`<${Text} title="Spine label" hint="Free · the little line at the bottom" max="14" value=${look.spineText} placeholder="Date or tape number" onSave=${(v) => set({ spineText: v })} />`}
    <${Row} title="Tape label colour" hint="Free" lvl=${lvl} value=${look.labelInk} onPick=${(id) => set({ labelInk: id })} none=${{ name: "Shop yellow", swatch: "#f2c230" }}
      items=${free("labelInk").map((u) => ({ ...u, swatch: LABEL_INKS[u.id] }))} />
    <h3 class="cz-sect">Case</h3>
    <${Row} title="Case" lvl=${lvl} value=${look.case} onPick=${(id) => set({ case: id })} none=${{ name: "Sleeve", sample: "—" }}
      items=${(d.cases || []).map((u) => ({ ...u, thumb: { k: "c" + u.id + (look.caseInk || ""), paint: caseThumb(u.id, look.caseInk) } }))} />
    <${Row} title="Case colour" hint="Free" lvl=${lvl} value=${look.caseInk} onPick=${(id) => set({ caseInk: id })} none=${{ name: "Black", swatch: hex(CASE_INKS[""]) }}
      items=${free("caseInk").map((u) => ({ ...u, swatch: hex(CASE_INKS[u.id]) }))} />
    <${Row} title="Wear" lvl=${lvl} value=${look.wear} onPick=${(id) => set({ wear: id })} none=${{ name: "Clean", sample: "—" }}
      items=${(d.wears || []).map((u) => ({ ...u, thumb: { k: "w" + u.id, paint: coverThumb({ wear: u.id }) } }))} />
    <div class="cz-row"><div class="cz-row-head"><h4>Extras</h4></div><div class="cz-cards">
      ${[["worn", "Scuffed", d.worn], ["foil", "Foil title", d.foil]].map(([f, name, need]) => html`<button key=${f} class=${"cz-card" + (look[f] ? " on" : "") + (lvl < need ? " locked" : "")} disabled=${lvl < need} onClick=${() => set({ [f]: !look[f], [f + "Mix"]: false })}>
        <div class="cz-sample">${f === "foil" ? "✦" : "≈"}</div><span class="cz-name">${name}${look[f] ? " · on" : ""}</span>${lvl < need ? html`<span class="cz-lock">🔒 Lv ${need}</span>` : null}</button>`)}
      ${onlyCover || lvl < d.collector ? null : html`${(k.months || []).map((m) => html`<button key=${m} class=${"cz-card" + (look.collector === m ? " on" : "")} onClick=${() => set({ collector: look.collector === m ? "" : m })}><div class="cz-sample">★</div><span class="cz-name">${collectorName(m)}</span></button>`)}`}
    </div></div>`;
}

function PosterRows({ look, set, lvl }) {
  const d = looksDefs() || {};
  return html`
    <${Row} title="Poster" lvl=${lvl} value=${look.poster || "spotlight"} onPick=${(id) => set({ poster: id === "spotlight" ? "" : id })} mix=${{ id: "mix" }}
      items=${(d.posters || []).map((u) => ({ ...u, thumb: { k: "p" + u.id + (look.posterInk || "") + (look.headline || ""), w: 80, h: 104, paint: async () => { const c = canvas(230, 300); await drawPoster(c.getContext("2d"), { style: u.id, ink: look.posterInk, headline: look.headline, name: "YOUR NAME", seed: "thumb" }); return c; } } }))} />
    <${Row} title="Poster colour" hint="Free" lvl=${lvl} value=${look.posterInk || "red"} onPick=${(id) => set({ posterInk: id === "red" ? "" : id })} mix=${{ id: "mix" }} items=${inkItems(d)} />
    <${Text} title="Headline" hint="Free · replaces the word at the top" max="16" value=${look.headline} placeholder="WANTED" onSave=${(v) => set({ headline: v })} />
    <${Row} title="Name plate" hint="Free" lvl=${lvl} value=${look.plate} onPick=${(id) => set({ plate: id })} none=${{ name: "Plain", sample: "Name" }}
      items=${free("plate").map((u) => ({ ...u, sample: html`<span class=${"sname plate-" + u.id}>Name</span>` }))} />
    <div class="cz-row"><div class="cz-row-head"><h4>Nickname</h4><span class="dim small">Free · your nickname on the squad tape</span></div><div class="cz-cards">
      <button class=${"cz-card" + (!look.hideNick ? " on" : "")} onClick=${() => set({ hideNick: false })}><div class="cz-sample">"Ace"</div><span class="cz-name">Show it</span></button>
      <button class=${"cz-card" + (look.hideNick ? " on" : "")} onClick=${() => set({ hideNick: true })}><div class="cz-sample">—</div><span class="cz-name">Hide it</span></button>
    </div></div>
    <${Row} title="Pose" lvl=${lvl} value=${look.pose} onPick=${(id) => set({ pose: id })} none=${{ name: "Mix of mine", sample: "?" }}
      items=${(d.poses || []).map((u) => ({ ...u, sample: "☺" }))} />
    <${Row} title="Lobby frame" lvl=${lvl} value=${look.frame} onPick=${(id) => set({ frame: id })} mix=${{ id: "mix" }} none=${{ name: "Best I have", sample: "★" }}
      items=${[{ id: "none", name: "None", lvl: 1, sample: "—" }, ...(d.frames || []).map((u) => ({ ...u, swatch: { wood: "#6b4426", bronze: "#b0763a", silver: "#c9ced6", barbed: "#8a8a8a", gold: "#e0b43a", neon: "#ff4fa3" }[u.id] }))]} />`;
}

function ShelfRows({ prefs, setP, lvl }) {
  const d = looksDefs() || {};
  const toggle = (id) => { const cur = prefs.props || []; setP({ props: id === "" ? [] : cur.includes(id) ? cur.filter((x) => x !== id) : [...cur, id] }); };
  const SH = { dark: "#2a1810", pine: "#c9a36a", metal: "#151314", wire: "repeating-linear-gradient(90deg,#9a9ca0 0 2px,transparent 2px 10px)", crate: "repeating-linear-gradient(0deg,#7a5a36 0 12px,#4a3420 12px 14px)", gun: "linear-gradient(#3a2214,#0e2a16)" };
  return html`
    <${Row} title="Shelf" lvl=${lvl} value=${prefs.shelf || "metal"} onPick=${(id) => setP({ shelf: id === "metal" ? "" : id })} items=${(d.shelves || []).map((u) => ({ ...u, swatch: SH[u.id] }))} />
    <${Row} title="Lighting" hint="Free" lvl=${lvl} value=${prefs.light} onPick=${(id) => setP({ light: id })} none=${{ name: "Red neon", swatch: "#e3241b" }}
      items=${free("light").map((u) => ({ ...u, swatch: { warm: "#ffb070", cold: "#7fb0ff", candle: "radial-gradient(#ffcf70,#3a1a04)" }[u.id] }))} />
    <${Row} title="Order" hint="Free" lvl=${lvl} value=${prefs.sort || "played"} onPick=${(id) => setP({ sort: id === "played" ? "" : id })} items=${free("sort").map((u) => ({ ...u, sample: "≡" }))} />
    <${Row} title="Props on top" hint="Pick as many as you like" lvl=${lvl} value=${prefs.props} multi onPick=${toggle} none=${{ name: "None" }}
      items=${(d.props || []).map((u) => ({ ...u, sample: { bobblehead: "☻", plant: "✿", bat: "/", radio: "📻" }[u.id] }))} />`;
}

function LauncherRows({ prefs, setP }) {
  const [, redraw] = useState(0);
  return html`
    <${Row} title="Accent colour" hint="Free" lvl=${1} value=${prefs.accent || "red"} onPick=${(id) => setP({ accent: id === "red" ? "" : id })} items=${free("accent").map((u) => ({ ...u, swatch: ACCENTS[u.id] }))} />
    <div class="cz-row"><div class="cz-row-head"><h4>VCR sounds</h4><span class="dim small">Free</span></div>
      <div class="row">
        <button class=${"btn sm" + (soundOn() ? " primary" : "")} onClick=${() => { setSound(true); sound("button"); redraw((n) => n + 1); }}>On</button>
        <button class=${"btn sm" + (!soundOn() ? " primary" : "")} onClick=${() => { setSound(false); redraw((n) => n + 1); }}>Off</button>
        <input type="range" min="0" max="100" value=${Math.round(soundVol() * 100)} disabled=${!soundOn()} onChange=${(e) => { setVol(+e.target.value / 100); sound("clack"); redraw((n) => n + 1); }} />
        <span class="muted small">${Math.round(soundVol() * 100)}%</span>
      </div></div>`;
}

const SAMPLE = ["THE LONER", "DEAD RECKONERS", "LAST ONE STANDING", "NO ONE IS COMING", "PARTY OF ONE", "ALONE IN KNOX", "THE HERMIT", "ON MY OWN"];

export function Customize() {
  const [tab, setTab] = useState("tape");
  const d = looksDefs();
  const k = S.knox;
  if (!d || !k) return html`<div class="hint">Loading...</div>`;
  const lvl = unlockLevel(k);
  const look = (S.cfg && S.cfg.look) || {};
  const prefs = (S.cfg && S.cfg.prefs) || {};
  const set = async (patch) => {
    const next = { ...look, ...patch };
    const r = await act(() => rpc("setLook", { look: next }));
    if (r !== undefined) { S.cfg.look = next; update(); }
  };
  const setP = async (patch) => {
    const next = { ...prefs, ...patch };
    const r = await act(() => rpc("setPrefs", { prefs: next }));
    if (r !== undefined) { S.cfg.prefs = next; update(); }
  };
  const tape = myTape(look);
  const shelfTapes = SAMPLE.map((t, i) => ({ id: "s" + i, case: look.case, caseInk: look.caseInk,
    spine: drawSpine({ ...spineOpts(look), seed: "sample" + i, title: t, sub: i === 1 ? "PART IV" : "", label: "OCT " + (10 + i) + ", 26", genre: look.genre === "mix" ? "" : look.genre, worn: look.worn, num: String(1000 + i) }) }));
  let preview;
  if (tab === "tape") preview = html`<${CaseView} key=${tape.key} make=${tape.make} height=${520} />`;
  else if (tab === "poster") preview = html`<div class="cz-poster"><div class="slot me"><div class="slot-fig"><${PosterFig} key=${JSON.stringify([look.poster, look.posterInk, look.pose, look.headline])} m=${{ id: "me", me: true, name: "You" }} c=${S.cfg.character} seed="preview" /></div>
    <div class="slot-info"><div class=${"sname" + (look.plate ? " plate-" + look.plate : "")}>${S.cfg.character && S.cfg.character.forename ? `${S.cfg.character.forename} ${S.cfg.character.surname}` : "Your survivor"}</div></div></div></div>`;
  else if (tab === "shelf") preview = html`<div class="cz-shelf" style=${{ aspectRatio: String(shelfAspect(8, 1) * 0.8) }}><${ShelfView} key=${JSON.stringify(prefs) + look.case + look.caseInk} tapes=${shelfTapes} perRow=${8} rows=${1} prefs=${prefs} picked=${null} onPick=${() => {}} faces=${async () => null} /></div>`;
  else preview = html`<div class="cz-mock"><div class="cz-mock-nav"><b>KNOX</b><span class="on">Home</span><span>Mods</span><span>World</span></div><div class="cz-mock-main"><button class="btn primary">Open the lobby</button><div class="kw-bar"><i style=${{ width: "60%" }}></i></div></div></div>`;
  const TABS = [["tape", "Tape"], ["poster", "Poster"], ["shelf", "Shelf"], ["launcher", "Launcher"]];
  return html`<div class="cz">
    <div class="cz-top">
      ${S.tape || inLobby() ? null : html`<button class="btn sm" onClick=${() => update({ customizing: false })}>◂ Back to shelf</button>`}
      <h2 class="wordmark">Customize</h2>
      <span class="muted small">Level ${k.lvl}. Free picks are open to everyone; the rest unlock as you level up.</span>
    </div>
    <div class="cz-tabs">${TABS.map(([id, name]) => html`<button key=${id} class=${"cz-tab" + (tab === id ? " on" : "")} onClick=${() => { sound("button"); setTab(id); }}>${name}</button>`)}</div>
    <div class="cz-body">
      <div class="cz-picks">
        ${tab === "tape" ? html`<div class="dim small">Your default cover. New tapes start with it; change one tape with Change cover.</div><${TapeRows} look=${look} set=${set} lvl=${lvl} />` : null}
        ${tab === "poster" ? html`<div class="dim small">Your poster in the lobby. Friends see it.</div><${PosterRows} look=${look} set=${set} lvl=${lvl} />` : null}
        ${tab === "shelf" ? html`<div class="dim small">Only you see your shelf.</div><${ShelfRows} prefs=${prefs} setP=${setP} lvl=${lvl} />` : null}
        ${tab === "launcher" ? html`<${LauncherRows} prefs=${prefs} setP=${setP} />` : null}
      </div>
      <div class="cz-preview">${preview}</div>
    </div>
  </div>`;
}
