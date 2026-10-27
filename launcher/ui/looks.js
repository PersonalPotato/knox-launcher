// Tape looks: who picked what, what's unlocked, and the faces of each kind of tape.
import { html, S, update, rpc, act, toast, useEffect, useRef } from "./core.js";
import { seeded, knoxOf, nicknames, squadName, frameFor } from "./knox.js";
import { drawCover, drawBack, drawSpine, SPINE_INKS, castShot, tbaFig, posePick, sequel, blurb, image, collectorName } from "./tape.js";
import { CaseView } from "./vhs3d.js";
import { PosterFig } from "./poster.js";
import { TOKEN } from "./core.js";

export const looksDefs = () => (S.knoxDefs && S.knoxDefs.looks) || null;

export function lookOf(m) {
  if (!m || m.me) {
    const mine = (S.cfg && S.cfg.look) || {};
    const t = S.tape;
    if (!t || !t.look || t.kind !== "world") return mine;
    // the hosted tape's own cover (the same as tapes.go lobbyLook sends to the lobby); the poster parts stay yours
    const { pose, frame, poster, posterInk, headline, hideNick, plate, collector, ...box } = t.look;
    return { ...mine, ...box, wornMix: false, foilMix: false };
  }
  const k = m.knox || {};
  return k.look || {};
}

// mixLook turns every "mix" pick into one concrete pick for one tape (the same tape always gets the same).
// k is the look owner's Knox profile, which says what they have unlocked.
export function mixLook(look, seed, k) {
  const d = looksDefs();
  if (!d || !look) return look || {};
  const lvl = unlockLevel(k);
  const rnd = seeded(String(seed) + "mix");
  const pick = (list) => {
    const ok = (list || []).filter((u) => u.lvl <= lvl);
    return ok.length ? ok[Math.floor(rnd() * ok.length)].id : "";
  };
  const o = { ...look };
  if (o.genre === "mix") o.genre = pick(d.genres);
  if (o.backdrop === "mix") o.backdrop = pick(d.backdrops);
  if (o.frame === "mix") o.frame = pick(d.frames) || "none";
  if (o.spineInk === "mix") o.spineInk = pick(d.spineInks);
  if (o.poster === "mix") o.poster = pick(d.posters);
  if (o.posterInk === "mix") o.posterInk = pick(d.spineInks);
  if (o.wornMix) o.worn = lvl >= d.worn && rnd() < 0.5;
  if (o.foilMix) o.foil = lvl >= d.foil && rnd() < 0.5;
  if (o.collector === "mix") {
    const ms = (k && k.months) || [];
    o.collector = ms.length && lvl >= d.collector && rnd() < 0.6 ? ms[Math.floor(rnd() * ms.length)] : "";
  }
  return o;
}

// spineOpts: what a spine needs from a look (style, colour, and the styles you have for the mix)
export function spineOpts(look, k = S.knox) {
  const d = looksDefs();
  const lvl = unlockLevel(k);
  const styles = d && d.spines ? d.spines.filter((u) => u.lvl <= lvl).map((u) => u.id) : ["classic", "block"];
  return { style: look.spine || "", ink: look.spineInk || "", styles, backdrop: look.backdrop, text: look.spineText || "", labelInk: look.labelInk || "",
    studio: look.studio || "", wear: look.wear || "" };
}

// boxOpts: the free picks and stickers a cover and back print from a look
export function boxOpts(look) {
  return { tagline: look.tagline || "", titleFont: look.titleFont || "", titleInk: look.titleInk || "", tint: look.tint || "", rating: look.rating || "",
    studio: look.studio || "", price: look.price || "", rewind: !!look.rewind, stickers: look.stickers || [], wear: look.wear || "" };
}
// caseOpts: the plastic around the sleeve (vhs3d dressCase)
export const caseOpts = (look) => ({ case: look.case || "", caseInk: look.caseInk || "" });

export function unlockLevel(k) {
  const d = looksDefs();
  if (!k) return 1;
  return k.cut > 0 ? Math.max(k.lvl || 1, (d && d.cut) || 35) : k.lvl || 1;
}

export function frameOf(m) {
  const l = mixLook(lookOf(m), m && m.id, knoxOf(m));
  if (l.frame === "none") return "";
  if (l.frame) return l.frame;
  return frameFor(((knoxOf(m) || {}).lvl) || 1);
}

export function poseOf(m, seed) {
  const d = looksDefs();
  const pose = lookOf(m).pose;
  return posePick(d && d.poses, pose === "mix" ? "" : pose, seeded(seed + (m ? m.id : "")), unlockLevel(knoxOf(m)));
}

// wornOf: what a member looks like in this world right now. A living survivor in the world's cast wears
// what Knox Link last saw them in; anyone else looks like their creator character.
export function wornOf(m) {
  const c = charOf(m);
  const cast = S.state && S.state.info && S.state.info.cast;
  const name = String(m.name || "").toLowerCase();
  const hit = cast && (cast.cast || []).find((x) => x.alive && x.look && x.look.clothing && String(x.user || "").toLowerCase() === name);
  return hit ? { ...(c || {}), ...hit.look } : c;
}

export function charOf(m) {
  if (m.me) return S.cfg.character;
  try { const ch = typeof m.char === "string" ? JSON.parse(m.char) : m.char; return ch && ch.c; } catch { return null; }
}

const fmtHours = (h) => (!h ? "0" : h < 10 ? h.toFixed(1) : String(Math.round(h)));
const cutOf = (m) => ((knoxOf(m) || {}).cut) || 0;

// The squad's tape (lobby and the "getting ready" screen). The host's look decides the cover.
export function squadSeed(members) {
  const h = members.find((m) => m.host);
  return h ? h.id : "squad";
}

export function squadTape(members) {
  const st = S.state || {};
  const info = st.info || {};
  const squad = st.squad || {};
  const host = members.find((m) => m.host) || members.find((m) => m.me) || {};
  const seed = squadSeed(members);
  const look = mixLook(lookOf(host), seed + (info.world || ""), knoxOf(host));
  const name = squadName(info, seed);
  const played = ((S.cfg && S.cfg.games) || []).filter((g) => g.world === info.world).length; // sessions of this world, not the squad's
  const sq = sequel(name, worldPart(info.world, played), seed);
  const nick = nicknames(members, squad, String(st.lobby || "") + members.map((m) => m.id).join(","));
  const people = members.map((m) => ({ id: m.id, c: wornOf(m), pose: poseOf(m, seed) }));
  const cast = members.map((m) => {
    const c = charOf(m);
    return { name: c && c.forename ? `${c.forename} ${c.surname}` : m.name, nick: lookOf(m).hideNick ? "" : nick[m.id], player: m.name };
  });
  const stats = [["Games", squad.games || 0], ["Kills", squad.kills || 0], ["Best day", squad.bestDay || 0], ["Deaths", squad.deaths || 0], ["Hours", fmtHours(squad.hours)]];
  const best = Math.max(1, ...members.map((m) => ((knoxOf(m) || {}).lvl) || 1));
  const key = JSON.stringify([sq, look, cast, stats, people.map((p) => [p.c && JSON.stringify(p.c).length, p.pose.anim]), cutOf(host)]);
  const make = async () => {
    const fig = await castShot(people, seed);
    const o = { ...sq, ...boxOpts(look), genre: look.genre, backdrop: look.backdrop, fig, cast, stats, foil: look.foil, worn: look.worn,
      frame: look.frame === "none" ? "" : look.frame || frameFor(best), cut: cutOf(host), collector: look.collector, seed };
    const front = await drawCover(o);
    const story = info.backstory && S.presets && (S.presets.backstories || []).find((b) => b.id === info.backstory);
    const back = await drawBack({ ...o, photos: [fig, await image("/art")], player: host.name,
      blurb: blurb({ games: squad.games, kills: squad.kills, deaths: squad.deaths, bestDay: squad.bestDay, cast, town: info.spawn && info.spawn.name, story: story && story.name }),
      rental: String(seed).slice(-4) });
    const spine = await drawSpine({ ...spineOpts(look, knoxOf(host)), seed, title: sq.title, sub: sq.part, genre: look.genre, frame: o.frame, worn: look.worn, num: String(seed).slice(-4) });
    return { front: front.canvas, mask: front.mask, back, spine, ...caseOpts(look) };
  };
  return { key, make, title: sq };
}

// Your own tape: the preview in the Knox level window.
export function myTape(pickedLook) {
  const look = mixLook(pickedLook, "me", S.knox);
  const me = { id: "me", me: true, name: (S.state && S.state.me && S.state.me.name) || "You" };
  const c = S.cfg.character;
  const k = S.knox || {};
  const seed = "me";
  const people = [{ c, pose: posePick(looksDefs() && looksDefs().poses, look.pose === "mix" ? "" : look.pose, seeded("mine"), unlockLevel(k)) }];
  const name = c && c.forename ? `${c.forename} ${c.surname}` : "Your survivor";
  const key = JSON.stringify([look, name, k.lvl, k.cut, c && JSON.stringify(c).length]);
  const make = async () => {
    const fig = await castShot(people, seed + (look.pose || ""));
    const o = { title: name.toUpperCase(), part: "", sub: "", ...boxOpts(look), genre: look.genre, backdrop: look.backdrop, fig, cast: [{ name, nick: k.title, player: me.name }],
      stats: [["Level", k.lvl || 1], ["XP", k.xp || 0], ["Badges", (k.badges || []).length]], foil: look.foil, worn: look.worn,
      frame: look.frame === "none" ? "" : look.frame || frameFor(k.lvl || 1), cut: k.cut, collector: look.collector, seed: name };
    const front = await drawCover(o);
    const back = await drawBack({ ...o, photos: [fig, await image("/art")], player: me.name, blurb: `${name}. ${k.title || "Fresh Meat"}. Level ${k.lvl || 1}.`, rental: "0001" });
    const spine = await drawSpine({ ...spineOpts(look), title: name.toUpperCase(), genre: look.genre, frame: o.frame, worn: look.worn, num: "0001" });
    return { front: front.canvas, mask: front.mask, back, spine, ...caseOpts(look) };
  };
  return { key, make };
}

// Shelf tapes: a solo save or a world you host.
const dateOf = (s) => {
  const m = /^(\d{4})-(\d\d)-(\d\d)_(\d\d)-(\d\d)/.exec(s);
  if (!m) return null;
  return new Date(+m[1], +m[2] - 1, +m[3], +m[4], +m[5]);
};
export function tapeTitle(t) {
  if (t.title) return t.title.toUpperCase();
  if (t.kind === "guest") return `${(t.hostName || "A FRIEND").toUpperCase()}'S WORLD`;
  if (t.kind === "world") return t.name.toUpperCase();
  const d = dateOf(t.name);
  const rnd = seeded(t.mode + t.name);
  const names = ["ALONE IN KNOX", "ONE MAN ARMY", "THE LONER", "SOLO", "LAST ONE STANDING", "NO ONE IS COMING", "PARTY OF ONE", "THE HERMIT", "ON MY OWN", "THE SURVIVOR"];
  return d ? names[Math.floor(rnd() * names.length)] : t.name.toUpperCase();
}
export function tapeLabel(t) {
  if (t.fresh) return "NEW";
  if (t.kind === "guest") return "GUEST";
  if (t.kind === "world") return "HOSTED";
  const d = dateOf(t.name);
  return d ? d.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "2-digit" }).toUpperCase() : t.mode.toUpperCase();
}
export function tapeSub(t) {
  if (t.fresh) return "";
  if (t.kind === "world" || t.kind === "guest") return sequel(t.name, t.part || worldPart(t.name, t.games), t.name).part;
  return t.mode === "Sandbox" ? "" : t.mode.toUpperCase();
}
// worldPart: which part a world's tape is on. Each play session is a part: closing or restarting the
// tape makes the next one PART II, III... The saves code can set it (cfg.series); until then, sessions + 1.
export function worldPart(world, games) {
  const s = world && S.cfg && S.cfg.series && S.cfg.series[world];
  return (s && s.part) || (games || 0) + 1;
}

// a tape's own cover look (made on the shelf), or your default look
export const tapeLook = (t) => mixLook(t.look || (S.cfg && S.cfg.look) || {}, t.kind + t.name, S.knox);

export function shelfSpine(t, i) {
  const look = tapeLook(t);
  return drawSpine({ ...spineOpts(look), seed: t.kind + t.name, title: tapeTitle(t), sub: tapeSub(t), label: tapeLabel(t), genre: look.genre, worn: look.worn, num: String(1000 + i).slice(-4) });
}

export async function shelfFaces(t, i) {
  if (t.fresh) return freshFaces(t, i);
  const look = tapeLook(t);
  const c = S.cfg.character;
  const k = S.knox || {};
  const thumb = t.thumb ? await image(`/savethumb?t=${TOKEN}&mode=${encodeURIComponent(t.mode)}&name=${encodeURIComponent(t.name)}`) : null;
  const poses = looksDefs() && looksDefs().poses;
  // a world's cast in what they last wore in game (Knox Link); otherwise your own survivor
  const fig = t.castLooks && t.castLooks.length ? await castShot(t.castLooks.map((l, j) => ({ id: String(j), c: l, pose: posePick(poses, "", seeded(t.name + j)) })), "shelf" + t.name)
    : t.kind === "solo" || !t.cast || t.cast.length < 2 ? await castShot([{ c, pose: posePick(looksDefs() && looksDefs().poses, look.pose, seeded(t.name)) }], "shelf" + t.name) : null;
  const sub = tapeSub(t);
  const o = { ...boxOpts(look), title: tapeTitle(t), part: sub.startsWith("PART") ? sub : "", sub: sub.startsWith("PART") ? sequel(t.name, t.part || worldPart(t.name, t.games), t.name).sub : "", genre: look.genre, backdrop: look.backdrop, fig, photo: thumb,
    cast: (t.cast && t.cast.length ? t.cast : [(S.state && S.state.me && S.state.me.name) || "You"]).slice(0, 4).map((n) => ({ name: n })),
    stats: [["Nights", t.games], ["Kills", t.kills], ["Best day", t.bestDay], ["Deaths", t.deaths]], foil: look.foil, worn: look.worn,
    frame: look.frame === "none" ? "" : look.frame || frameFor(k.lvl || 1), cut: k.cut, collector: look.collector, seed: t.mode + t.name,
    kicker: t.kind === "guest" ? `${(t.hostName || "A FRIEND").toUpperCase()} PRESENTS` : t.kind === "world" ? "A KNOX LAUNCHER WORLD" : "A KNOX LAUNCHER SOLO GAME" };
  const front = await drawCover(o);
  const back = await drawBack({ ...o, photos: [thumb, fig || (await image("/art"))], player: (S.state && S.state.me && S.state.me.name) || "",
    blurb: blurb({ games: t.games, kills: t.kills, deaths: t.deaths, bestDay: t.bestDay, cast: t.cast, town: t.town, story: t.story }), rental: String(1000 + i).slice(-4) });
  const spine = await shelfSpine(t, i);
  return { front: front.canvas, mask: front.mask, back, spine, ...caseOpts(look) };
}

// freshFaces: a tape made on the shelf that nobody has played yet. The cast is still to come.
async function freshFaces(t, i) {
  const look = tapeLook(t);
  const fig = tbaFig();
  const o = { ...boxOpts(look), title: tapeTitle(t), genre: look.genre, backdrop: look.backdrop, fig, seed: t.kind + t.name, foil: look.foil, worn: look.worn,
    cast: [{ name: "To be announced" }], stats: [["Nights", 0]], kicker: t.kind === "world" ? "A KNOX LAUNCHER WORLD" : "A KNOX LAUNCHER SOLO GAME" };
  const front = await drawCover(o);
  const back = await drawBack({ ...o, photos: [fig, await image("/art")], blurb: "A new world. Nobody has played it yet.", rental: String(1000 + i).slice(-4) });
  const spine = await shelfSpine(t, i);
  return { front: front.canvas, mask: front.mask, back, spine, ...caseOpts(look) };
}

// LooksPanel: pick your tape looks in the Knox level window.
export function LooksPanel() {
  const d = looksDefs();
  const k = S.knox;
  if (!d || !k) return null;
  const lvl = unlockLevel(k);
  const look = (S.cfg && S.cfg.look) || {};
  const set = async (patch) => {
    const next = { ...look, ...patch };
    const r = await act(() => rpc("setLook", { look: next }));
    if (r !== undefined) { S.cfg.look = next; update(); }
  };
  const mixBtn = (field) => html`<button class=${"btn sm" + (look[field] === "mix" ? " primary" : "")} onClick=${() => set({ [field]: "mix" })}>Mix of all I have</button>`;
  const row = (title, list, field, def) => html`<h4>${title}</h4><div class="look-row">${mixBtn(field)}${list.map((u) => {
    const on = (look[field] || def) === u.id;
    const locked = lvl < u.lvl;
    return html`<button key=${u.id} class=${"btn sm" + (on ? " primary" : "")} disabled=${locked} title=${locked ? "Unlocks at level " + u.lvl : ""} onClick=${() => set({ [field]: u.id === def ? "" : u.id })}>${u.name}${locked ? " · Lv " + u.lvl : ""}</button>`;
  })}</div>`;
  // on/off picks (worn, foil): Off, On, or a mix across your tapes
  const tri = (name, field, mixField, need) => {
    const locked = lvl < need;
    const mode = look[mixField] ? "mix" : look[field] ? "on" : "off";
    const b = (m, label, patch) => html`<button class=${"btn sm" + (mode === m ? " primary" : "")} disabled=${locked && m !== "off"} onClick=${() => set(patch)}>${label}</button>`;
    return html`<span class="look-label">${name}${locked ? " · Lv " + need : ""}</span>
      ${b("off", "Off", { [field]: false, [mixField]: false })}${b("on", "On", { [field]: true, [mixField]: false })}${b("mix", "Mix", { [field]: false, [mixField]: true })}`;
  };
  const extraPoses = d.poses.filter((p) => p.lvl > 1);
  const basePoses = d.poses.filter((p) => p.lvl <= 1);
  const tape = myTape(look);
  return html`<div class="looks">
    <div class="looks-preview"><${CaseView} key=${tape.key} make=${tape.make} height=${430} /></div>
    <div class="looks-picks">
      ${row("Cover", d.genres, "genre", "horror")}
      ${row("Backdrop", d.backdrops, "backdrop", "title")}
      <h4>Your pose</h4>
      <div class="look-row">
        <button class=${"btn sm" + (!look.pose || look.pose === "mix" ? " primary" : "")} onClick=${() => set({ pose: "" })}>Mix of all I have</button>
        ${basePoses.map((p) => html`<button key=${p.id} class=${"btn sm" + (look.pose === p.id ? " primary" : "")} onClick=${() => set({ pose: p.id })}>${p.name}</button>`)}
      </div>
      <div class="look-row">${extraPoses.map((p) => html`<button key=${p.id} class=${"btn sm" + (look.pose === p.id ? " primary" : "")} disabled=${lvl < p.lvl} title=${lvl < p.lvl ? "Unlocks at level " + p.lvl : ""} onClick=${() => set({ pose: p.id })}>${p.name}${lvl < p.lvl ? " · Lv " + p.lvl : ""}</button>`)}</div>
      <h4>Spine</h4>
      <div class="look-row">
        <button class=${"btn sm" + (!look.spine ? " primary" : "")} onClick=${() => set({ spine: "" })}>Mix of all I have</button>
        ${(d.spines || []).map((u) => html`<button key=${u.id} class=${"btn sm" + (look.spine === u.id ? " primary" : "")} disabled=${lvl < u.lvl} onClick=${() => set({ spine: u.id })}>${u.name}${lvl < u.lvl ? " · Lv " + u.lvl : ""}</button>`)}
      </div>
      <h4>Spine colour</h4>
      <div class="look-row">
        ${mixBtn("spineInk")}
        <button class=${"btn sm" + (!look.spineInk ? " primary" : "")} onClick=${() => set({ spineInk: "" })}>Match the cover</button>
        ${(d.spineInks || []).map((u) => html`<button key=${u.id} class=${"btn sm inkbtn" + (look.spineInk === u.id ? " primary" : "")} disabled=${lvl < u.lvl} onClick=${() => set({ spineInk: u.id })}><i style=${{ background: SPINE_INKS[u.id] }}></i>${u.name}${lvl < u.lvl ? " · Lv " + u.lvl : ""}</button>`)}
      </div>
      <${SpineRow} look=${look} />
      <h3 class="looks-sub">Lobby poster</h3>
      <div class="poster-pick">
        <div class="poster-preview"><${PosterFig} key=${JSON.stringify([look.poster, look.posterInk, look.pose])} m=${{ id: "me", me: true, name: "You" }} c=${S.cfg.character} seed="preview" /></div>
        <div class="grow">
          <h4>Poster</h4>
          <div class="look-row">
            ${mixBtn("poster")}
            ${(d.posters || []).map((u) => html`<button key=${u.id} class=${"btn sm" + ((look.poster || "spotlight") === u.id ? " primary" : "")} disabled=${lvl < u.lvl} onClick=${() => set({ poster: u.id === "spotlight" ? "" : u.id })}>${u.name}${lvl < u.lvl ? " · Lv " + u.lvl : ""}</button>`)}
          </div>
          <h4>Poster colour</h4>
          <div class="look-row">
            ${mixBtn("posterInk")}
            ${(d.spineInks || []).map((u) => html`<button key=${u.id} class=${"btn sm inkbtn" + ((look.posterInk || "red") === u.id ? " primary" : "")} disabled=${lvl < u.lvl} onClick=${() => set({ posterInk: u.id === "red" ? "" : u.id })}><i style=${{ background: SPINE_INKS[u.id] }}></i>${u.name}${lvl < u.lvl ? " · Lv " + u.lvl : ""}</button>`)}
          </div>
        </div>
      </div>
      <h4>Lobby frame</h4>
      <div class="look-row">
        <button class=${"btn sm" + (!look.frame ? " primary" : "")} onClick=${() => set({ frame: "" })}>Best I have</button>
        ${mixBtn("frame")}
        <button class=${"btn sm" + (look.frame === "none" ? " primary" : "")} onClick=${() => set({ frame: "none" })}>None</button>
        ${d.frames.map((f) => html`<button key=${f.id} class=${"btn sm" + (look.frame === f.id ? " primary" : "")} disabled=${lvl < f.lvl} onClick=${() => set({ frame: f.id })}>${f.name}${lvl < f.lvl ? " · Lv " + f.lvl : ""}</button>`)}
      </div>
      <h4>Case</h4>
      <div class="look-row">
        ${tri("Worn case", "worn", "wornMix", d.worn)}
      </div>
      <div class="look-row">
        ${tri("Foil title", "foil", "foilMix", d.foil)}
      </div>
      <h4>Collector's editions</h4>
      ${lvl < d.collector ? html`<div class="muted small">Unlocks at level ${d.collector}. Then each month you play gets its own edition.</div>`
        : html`<div class="look-row">
          ${mixBtn("collector")}
          <button class=${"btn sm" + (!look.collector ? " primary" : "")} onClick=${() => set({ collector: "" })}>None</button>
          ${(k.months || []).map((m) => html`<button key=${m} class=${"btn sm" + (look.collector === m ? " primary" : "")} onClick=${() => set({ collector: m })}>${collectorName(m)}</button>`)}
          ${(k.months || []).length ? null : html`<span class="muted small">Play a game this month to get one.</span>`}
        </div>`}
      <h4>Director's Cut</h4>
      ${k.lvl >= d.cut ? html`<div class="row"><button class="btn sm primary" onClick=${async () => {
          if (!confirm("Director's Cut: your level goes back to 1. You keep everything you unlocked and your tapes get a Director's Cut label. Go?")) return;
          const r = await act(() => rpc("directorsCut"));
          if (r !== undefined) { toast("Director's Cut. Back to level 1."); const { loadKnox } = await import("./knox.js"); loadKnox(); }
        }}>Take the Director's Cut</button></div>`
        : html`<div class="muted small">At level ${d.cut}: go back to level 1, keep every unlock, and get a Director's Cut label.${k.cut ? ` You have ${k.cut}.` : ""}</div>`}
    </div>
  </div>`;
}

// SpineRow: a few of your shelf spines, flat, so you can see the spine picks.
const SAMPLE = ["THE LONER", "DEAD RECKONERS", "LAST ONE STANDING", "NO ONE IS COMING", "PARTY OF ONE", "ALONE IN KNOX", "THE HERMIT", "ON MY OWN"];
function SpineRow({ look }) {
  const box = useRef(null);
  const key = JSON.stringify([look, S.knox && S.knox.lvl]);
  useEffect(() => {
    let alive = true;
    Promise.all(SAMPLE.map((t, i) => drawSpine({ ...spineOpts(look), seed: "sample" + i, title: t, sub: i === 1 ? "PART IV" : "", label: "OCT " + (10 + i) + ", 26",
      genre: look.genre, worn: look.worn, num: String(1000 + i) }))).then((cs) => {
      if (!alive || !box.current) return;
      box.current.replaceChildren(...cs);
    });
    return () => { alive = false; };
  }, [key]);
  return html`<div class="spine-row" ref=${box}></div>`;
}
