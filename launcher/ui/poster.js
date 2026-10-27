// Squad tape and lobby posters: the squad as a VHS case, and each survivor posing on their own poster.
import * as THREE from "./vendor/three.module.js";
import { html, useEffect, useRef, useState, S, rpc, act, update } from "./core.js";
import { squadName, makeSquadName, knoxOf } from "./knox.js";
import { CaseView } from "./vhs3d.js";
import { drawPoster, stageCast, sequel } from "./tape.js";
import { squadTape, lookOf, poseOf, worldPart, mixLook } from "./looks.js";

// the made-up squad name stays the same for one host
function squadSeed(members) {
  const h = members.find((m) => m.host);
  return h ? h.id : "squad";
}

// the squad name is also the hosted tape's title (tapes.go)
async function setSquad(name) {
  await act(() => rpc("setSquadName", { name }));
  if (name && S.tape && S.tape.kind === "world") update({ tape: { ...S.tape, title: name } });
}

function useRename() {
  const [edit, setEdit] = useState(null);
  const save = async () => {
    const v = (edit || "").trim();
    setEdit(null);
    await setSquad(v);
  };
  return [edit, setEdit, save];
}

function RenameInput({ edit, setEdit, save }) {
  return html`<input class="squad-rename" autofocus value=${edit} maxlength="40" placeholder="Empty = a made up name" onInput=${(e) => setEdit(e.target.value)} onKeyDown=${(e) => { if (e.key === "Enter") save(); if (e.key === "Escape") setEdit(null); }} onBlur=${save} />`;
}

// SquadTitle is the lobby heading: the squad's name, which the host can change.
export function SquadTitle({ members }) {
  const st = S.state || {};
  const info = st.info || {};
  const [edit, setEdit, save] = useRename();
  const name = squadName(info, squadSeed(members));
  if (edit !== null) return html`<${RenameInput} edit=${edit} setEdit=${setEdit} save=${save} />`;
  const reroll = () => setSquad(makeSquadName(info, String(Math.random())));
  const played = ((S.cfg && S.cfg.games) || []).filter((g) => g.world === info.world).length;
  const sq = sequel(name, worldPart(info.world, played), squadSeed(members));
  return html`<h1 class="squad-name">${name}${sq.part ? html`<span class="squad-part">${sq.part} - ${sq.sub}</span>` : null}${st.host ? html`<span class="squad-tools"><button class="btn sm ghost" onClick=${() => setEdit(info.squadName || name)}>Rename</button><button class="btn sm ghost" onClick=${reroll}>Random</button></span>` : null}</h1>`;
}

// SquadPoster is the squad's tape: a 3D VHS case you can turn over.
export function SquadPoster({ members, height = 560 }) {
  const t = squadTape(members);
  return html`<div class="squad-case"><${CaseView} key=${t.key} make=${t.make} height=${height} /></div>`;
}

// PosterFig is one survivor posing in front of their backdrop: the lobby slot's poster.
export function PosterFig({ m, c, seed }) {
  const box = useRef(null);
  const look = mixLook(lookOf(m), m.id, knoxOf(m));
  const pose = poseOf(m, seed);
  useEffect(() => {
    const el = box.current;
    let alive = true;
    // the poster is a flat picture behind the 3D survivor (its own look, not the tape's)
    const paint = async () => {
      const w = Math.max(100, el.clientWidth || 230), h = Math.max(100, el.clientHeight || 280);
      const bg = document.createElement("canvas");
      bg.width = w * 2; bg.height = h * 2;
      await drawPoster(bg.getContext("2d"), { style: look.poster, ink: look.posterInk, headline: look.headline, name: c && c.forename ? `${c.forename} ${c.surname}` : m.name, seed: m.id });
      if (alive) el.style.backgroundImage = `url(${bg.toDataURL("image/jpeg", 0.88)})`;
    };
    paint();
    if (!c) return () => { alive = false; };
    const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    el.appendChild(renderer.domElement);
    const st = stageCast([{ c, pose }], seed + m.id);
    st.scene.fog = null;
    const draw = () => {
      const w = el.clientWidth || 230, h = el.clientHeight || 300;
      renderer.setSize(w, h);
      st.fit(w, h);
      // posters with a headline leave room above the head
      const head = ["wanted", "missing", "propaganda", "mugshot", "comic", "fightcard", "onesheet", "card", "obit", "newspaper", "graffiti"].includes(look.poster) || !!look.headline;
      st.camera.position.z += head ? -0.12 : -0.35;
      if (head) st.camera.position.y += 0.3;
      renderer.render(st.scene, st.camera);
    };
    st.ready.then(() => alive && draw());
    const ro = new ResizeObserver(() => alive && draw());
    ro.observe(el);
    return () => {
      alive = false;
      ro.disconnect();
      renderer.dispose();
      renderer.forceContextLoss();
      if (renderer.domElement.parentNode) renderer.domElement.remove();
    };
  }, [S.cat && S.cat.version, c && JSON.stringify(c), pose.anim, look.poster, look.posterInk, look.headline]);
  return html`<div class="poster-fig" ref=${box}></div>`;
}

