// The shelf: where the launcher opens. Every solo save and hosted world is a tape, friends hosting right
// now have a tape you can join, and the blank tape makes a new one. Pick a tape to put it in the VCR.
import { html, useEffect, useMemo, useState, S, update, rpc, act } from "./core.js";
import { ShelfView, shelfAspect } from "./vhs3d.js";
import { shelfSpine, shelfFaces, tapeTitle, tapeLabel } from "./looks.js";
import { drawCover, drawBack, drawSpine, image, tbaFig, fonts, SPINE, W, H } from "./tape.js";
import { useFriends } from "./play.js";
import { NavLevel } from "./knox.js";
import { loadTape } from "./tapes.js";
import { sound } from "./sound.js";

// one fixed size of shelf, so the view never zooms; more tapes go on more shelves (pages)
const PER_ROW = 20, ROWS = 2;
const ASPECT = shelfAspect(PER_ROW, ROWS) * 1.06;

function canvas(w, h) { const c = document.createElement("canvas"); c.width = w; c.height = h; return c; }

// the blank tape at the start of the shelf
async function blankSpine() {
  await fonts();
  const c = canvas(SPINE, H), x = c.getContext("2d");
  x.fillStyle = "#121012"; x.fillRect(0, 0, SPINE, H);
  x.fillStyle = "#efe9dc"; x.fillRect(14, 120, SPINE - 28, H - 240);
  x.save(); x.translate(SPINE / 2, H - 150); x.rotate(-Math.PI / 2);
  x.fillStyle = "#c4140c"; x.font = "400 74px 'Patrick Hand'"; x.textBaseline = "middle"; x.fillText("+ NEW TAPE", 0, 0);
  x.restore();
  return c;
}
async function blankFaces() {
  await fonts();
  const f = canvas(W, H), x = f.getContext("2d");
  x.fillStyle = "#121012"; x.fillRect(0, 0, W, H);
  x.fillStyle = "#efe9dc"; x.fillRect(60, 300, W - 120, 300);
  x.fillStyle = "#c4140c"; x.font = "400 80px 'Patrick Hand'"; x.textAlign = "center"; x.fillText("+ NEW TAPE", W / 2, 470);
  x.fillStyle = "#555"; x.font = "400 30px 'Patrick Hand'"; x.fillText("a new world", W / 2, 540);
  return { front: f, back: f, spine: await blankSpine() };
}

// a friend hosting right now: their tape pokes out with a JOIN sticker
const friendSpine = (f) => joinSticker(drawSpine({ style: "block", seed: f.id, title: `${f.name.toUpperCase()}'S GAME`, label: "LIVE", genre: "horror", num: "LIVE" }));
async function joinSticker(spine) {
  const c = await spine;
  const x = c.getContext("2d");
  x.fillStyle = "#e3241b"; x.fillRect(8, 20, SPINE - 16, 64);
  x.fillStyle = "#fff"; x.font = "700 30px Oswald"; x.textAlign = "center"; x.textBaseline = "middle"; x.fillText("JOIN", SPINE / 2, 54);
  return c;
}
async function friendFaces(f) {
  const fig = tbaFig("HOSTING RIGHT NOW");
  const o = { title: `${f.name.toUpperCase()}'S GAME`, genre: "horror", backdrop: "title", fig, seed: f.id, cast: [], stats: [], kicker: `${f.name.toUpperCase()} PRESENTS` };
  const front = await drawCover(o);
  const back = await drawBack({ ...o, photos: [fig, await image("/art")], player: f.name, blurb: `${f.name} is hosting. Join to see their world.`, rental: "LIVE" });
  return { front: front.canvas, mask: front.mask, back, spine: await friendSpine(f) };
}

export function ShelfHome() {
  const [tapes, setTapes] = useState(null);
  const [page, setPage] = useState(0);
  const [picked, setPicked] = useState(null);
  const [sure, setSure] = useState(false);
  const [bin, setBin] = useState(null); // binned tapes, shown when the bin is open
  const [binOpen, setBinOpen] = useState(false);
  const [reload, setReload] = useState(0);
  const [code, setCode] = useState("");
  const st = S.state || {};
  const friends = useFriends(true);
  const hosting = friends.filter((f) => f.lobby && f.lobby !== "0");
  const online = friends.filter((f) => f.online).length;
  const steamOK = st.steam && st.steam.ok;
  const running = st.game && st.game.running;
  // spines depend on your look and on which spine styles your level has unlocked
  const lookKey = JSON.stringify([(S.cfg && S.cfg.look) || {}, S.knox && S.knox.lvl, S.knox && S.knox.cut, !!(S.knoxDefs && S.knoxDefs.looks)]);
  const hostKey = hosting.map((f) => f.id + f.lobby).join(",");
  useEffect(() => { act(() => rpc("shelf")).then((v) => setTapes(v || [])); }, [running, reload, S.making]);
  useEffect(() => { act(() => rpc("binList")).then((v) => setBin(v || [])); }, [reload]);
  useEffect(() => setSure(false), [picked]);
  const all = useMemo(() => {
    if (!tapes) return [];
    // a friend hosting a world you've played in lights up that guest tape; anyone else gets a live tape
    const live = (t) => t.kind === "guest" && hosting.find((f) => f.id === t.host);
    const known = new Set(tapes.filter(live).map((t) => t.host));
    return [
      { id: "new", kind: "new", spine: blankSpine() },
      ...hosting.filter((f) => !known.has(f.id)).map((f) => ({ id: "f:" + f.id, kind: "friend", f, out: true, spine: friendSpine(f) })),
      ...tapes.map((t, i) => {
        const f = live(t);
        return { id: t.kind + ":" + (t.host || t.mode || "") + "/" + t.name, kind: "own", t, i, f, out: !!f, spine: f ? joinSticker(shelfSpine(t, i)) : shelfSpine(t, i) };
      }),
    ];
  }, [tapes, lookKey, hostKey]);
  const per = PER_ROW * ROWS;
  const pages = Math.max(1, Math.ceil(all.length / per));
  useEffect(() => { if (page > 0 && page >= pages) setPage(pages - 1); }, [pages]);
  const shown = all.slice(page * per, page * per + per);
  useEffect(() => {
    if (!picked) return;
    const onKey = (e) => e.key === "Escape" && setPicked(null);
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [picked]);
  const sel = shown.find((x) => x.id === picked);
  const busy = ["creating", "joining", "syncing", "launching"].includes(st.phase);
  const join = (body) => { sound("insert"); act(() => rpc("joinLobby", body)); };
  const binIt = async (t) => {
    const ok = t.fresh ? await act(() => rpc("dropTape", { kind: t.kind, mode: t.mode, name: t.name }), "Binned " + tapeTitle(t))
      : await act(() => rpc("binTape", t.kind === "world" ? { world: t.name } : { mode: t.mode, name: t.name }), "Binned " + tapeTitle(t));
    if (ok === undefined) return;
    sound("eject");
    setPicked(null);
    setReload(reload + 1);
  };
  const restore = async (b) => {
    const ok = await act(() => rpc("restoreTape", { id: b.id }), "Put back " + b.name);
    if (ok === undefined) return;
    sound("clack");
    setReload(reload + 1);
  };
  const faces = (x) => x.kind === "new" ? blankFaces() : x.kind === "friend" ? friendFaces(x.f) : shelfFaces(x.t, x.i);

  let card = null;
  if (sel && sel.kind === "new") card = html`<div class="tape-card"><div class="tk">Blank tape</div><h2>New tape</h2>
    <div class="muted small">A new world, solo or with your squad. Pick the cover, then set it up.</div>
    <div class="row" style=${{ marginTop: "12px" }}><button class="btn primary big" onClick=${() => { sound("clack"); update({ making: {} }); }}>Make a new tape</button><button class="btn" onClick=${() => setPicked(null)}>Put back</button></div></div>`;
  else if (sel && sel.kind === "friend") card = html`<div class="tape-card"><div class="tk">${sel.f.name}'s world</div><h2>${sel.f.name.toUpperCase()}'S GAME</h2>
    <div class="live-line">● Hosting right now</div>
    <div class="row" style=${{ marginTop: "12px" }}><button class="btn primary big" disabled=${busy || !steamOK} onClick=${() => join({ id: sel.f.lobby })}>${st.phase === "joining" ? "Joining..." : "Join"}</button><button class="btn" onClick=${() => setPicked(null)}>Put back</button></div></div>`;
  else if (sel && sel.t.kind === "guest") {
    const t = sel.t, host = t.hostName || "Your friend";
    card = html`<div class="tape-card">
      <div class="tk">Guest tape · ${host}'s world</div>
      <h2>${tapeTitle(t)}</h2>
      <div class="tape-stats">
        <div><b>${t.games}</b><small>Nights</small></div><div><b>${t.kills}</b><small>Kills</small></div>
        <div><b>${t.bestDay}</b><small>Best day</small></div><div><b>${t.deaths}</b><small>Deaths</small></div>
      </div>
      ${t.cast && t.cast.length ? html`<div class="muted small">With ${t.cast.slice(0, 6).join(", ")}</div>` : null}
      ${sel.f ? html`<div class="live-line">● ${host} is hosting this right now</div>
        <div class="row" style=${{ marginTop: "10px" }}><button class="btn primary big" disabled=${busy || !steamOK} onClick=${() => join({ id: sel.f.lobby })}>${st.phase === "joining" ? "Joining..." : "Join"}</button><button class="btn" onClick=${() => setPicked(null)}>Put back</button></div>`
        : html`<div class="muted" style=${{ marginTop: "10px" }}>${host} isn't hosting right now. It lights up here when they open the lobby.</div>
        <div class="row" style=${{ marginTop: "10px" }}><button class="btn" onClick=${() => setPicked(null)}>Put back</button></div>`}
    </div>`;
  } else if (sel) {
    const t = sel.t;
    card = html`<div class="tape-card">
      <div class="tk">${t.kind === "world" ? "Squad tape" : "Solo tape"}${t.fresh ? " · new" : ""}</div>
      <h2>${tapeTitle(t)}</h2>
      <div class="muted small">${t.kind === "world" ? "World " + t.name : tapeLabel(t) + " · " + t.mode}${t.backups ? ` · ${t.backups} backups` : ""}</div>
      ${t.fresh ? html`<div class="muted small" style=${{ marginTop: "8px" }}>Not played yet.</div>` : html`<div class="tape-stats">
        <div><b>${t.games}</b><small>Nights</small></div><div><b>${t.kills}</b><small>Kills</small></div>
        <div><b>${t.bestDay}</b><small>Best day</small></div><div><b>${t.deaths}</b><small>Deaths</small></div>
      </div>`}
      ${t.cast && t.cast.length ? html`<div class="muted small">With ${t.cast.slice(0, 6).join(", ")}</div>` : null}
      <div class="row" style=${{ marginTop: "12px" }}>
        <button class="btn primary big" disabled=${busy} onClick=${() => loadTape(t)}>Put it in</button>
        <button class="btn" onClick=${() => update({ making: { edit: t } })}>Change cover</button>
      </div>
      <div class="row" style=${{ marginTop: "8px" }}>
        <button class="btn ghost" onClick=${() => setPicked(null)}>Put back</button>
        <button class=${"btn" + (sure ? " danger" : " ghost")} disabled=${busy} onClick=${() => sure ? binIt(t) : setSure(true)}>${sure ? "Sure? Bin it" : "Bin"}</button>
      </div>
      <div class="dim small" style=${{ marginTop: "8px" }}>Drag the tape to turn it. Esc puts it back.</div>
    </div>`;
  }

  return html`<div class="shelf-home">
    <header class="sh-top">
      <b class="sh-word">KNOX LAUNCHER</b>
      <div class="sh-level"><${NavLevel} /></div>
      <span class="sh-chip"><i class=${"dot " + (steamOK ? "good" : "bad")}></i>${steamOK ? `${online} friend${online === 1 ? "" : "s"} online` : "Steam not running"}</span>
      <span class="grow"></span>
      <input type="text" class="sh-code" placeholder="Paste a KNOX- code" value=${code} onInput=${(e) => setCode(e.target.value)} onKeyDown=${(e) => e.key === "Enter" && code.trim() && join({ code })} />
      <button class="btn sm" disabled=${!code.trim() || busy} onClick=${() => join({ code })}>Join</button>
      ${bin && bin.length ? html`<button class=${"btn sm" + (binOpen ? " primary" : " ghost")} onClick=${() => setBinOpen(!binOpen)}>Bin (${bin.length})</button>` : null}
    </header>
    ${!steamOK ? html`<div class="card note-card bad sh-note">Steam isn't connected (${st.steam && st.steam.error}). Open Steam and <a href="#" onClick=${(e) => { e.preventDefault(); act(() => rpc("connectSteam"), "Connected to Steam"); }}>try again</a>.</div>` : null}
    ${binOpen && bin && bin.length ? html`<div class="bin-list sh-note">
      ${bin.map((b) => html`<div class="bin-row" key=${b.id}>
        <b>${tapeTitle(b.meta ? { ...b, title: b.meta.title } : b)}</b><span class="muted small">${b.kind === "world" ? "Hosted world " + b.name : "Solo save · " + tapeLabel(b) + " · " + b.mode} · binned ${new Date(b.binned * 1000).toLocaleDateString()}</span>
        <button class="btn sm" onClick=${() => restore(b)}>Put back on shelf</button>
      </div>`)}
      <div class="dim small">Binned tapes sit in ${"Zomboid\\KnoxBin"}. Delete that folder to get rid of them for good.</div>
    </div>` : null}
    <div class="sh-shelf">
      ${tapes === null ? html`<div class="hint">Loading your tapes...</div>` : html`<div class=${"shelf-wrap" + (sel ? " holding" : "")} style=${{ aspectRatio: String(ASPECT), maxWidth: `calc((100vh - 160px) * ${ASPECT})` }}>
        <${ShelfView} tapes=${shown} perRow=${PER_ROW} rows=${ROWS} picked=${picked} onPick=${(id) => { sound("clack"); setPicked(id); }} faces=${faces} />
        ${card}
      </div>`}
      ${pages > 1 ? html`<div class="sh-pages"><button class="btn sm" disabled=${page === 0} onClick=${() => { setPicked(null); setPage(page - 1); }}>‹ Back</button><span class="muted small">Shelf ${page + 1} of ${pages}</span><button class="btn sm" disabled=${page >= pages - 1} onClick=${() => { setPicked(null); setPage(page + 1); }}>Next ›</button></div>` : null}
    </div>
  </div>`;
}
