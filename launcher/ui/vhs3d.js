// 3D VHS cases: one case you can turn over, and a shelf of them.
import * as THREE from "./vendor/three.module.js";
import { html, useEffect, useRef, useState } from "./core.js";
import { W, H, SPINE, CASE_INKS } from "./tape.js";
import { sound } from "./sound.js";

const CW = 1.35, CH = (CW * H) / W, CD = (CW * SPINE) / W; // case size in metres-ish, same shape as the art

function tex(canvas) {
  const t = new THREE.CanvasTexture(canvas);
  t.colorSpace = THREE.SRGBColorSpace;
  t.anisotropy = 8;
  return t;
}

const plastic = () => new THREE.MeshStandardMaterial({ color: 0x0b0b0c, roughness: 0.55, metalness: 0.0 });

// shine is a diagonal light streak; it slides over the foil title as the case turns
let shineTex = null;
function shine() {
  if (shineTex) return shineTex;
  const c = document.createElement("canvas");
  c.width = 512; c.height = 64;
  const x = c.getContext("2d");
  const g = x.createLinearGradient(0, 0, 512, 0);
  g.addColorStop(0, "rgba(0,0,0,1)"); g.addColorStop(0.42, "rgba(40,36,30,1)"); g.addColorStop(0.5, "rgba(255,248,225,1)");
  g.addColorStop(0.58, "rgba(40,36,30,1)"); g.addColorStop(1, "rgba(0,0,0,1)");
  x.fillStyle = g; x.fillRect(0, 0, 512, 64);
  shineTex = new THREE.CanvasTexture(c);
  shineTex.wrapS = THREE.RepeatWrapping;
  return shineTex;
}

// makeCase builds one case. faces: { front, back, spine, mask } canvases (any can be missing yet).
export function makeCase() {
  const mats = [plastic(), plastic(), plastic(), plastic(), plastic(), plastic()];
  const mesh = new THREE.Mesh(new THREE.BoxGeometry(CW, CH, CD), mats);
  const g = new THREE.Group();
  g.add(mesh);
  // the open side shows the tape inside: a dark window and two reels
  const edge = document.createElement("canvas");
  edge.width = 64; edge.height = 512;
  const ex = edge.getContext("2d");
  ex.fillStyle = "#111"; ex.fillRect(0, 0, 64, 512);
  ex.fillStyle = "#1c1c1e"; ex.fillRect(10, 30, 44, 452);
  ex.fillStyle = "#2a2a2c"; ex.fillRect(14, 120, 36, 40); ex.fillRect(14, 352, 36, 40);
  mats[0].map = tex(edge);
  mats[0].color.set(0xffffff);
  g.userData = { mesh, mats, foil: null };
  return g;
}

// rimmed: the sleeve sits inside a plastic case, so the case colour shows round the edge
function rimmed(canvas, ink, pad, word) {
  const c = document.createElement("canvas");
  c.width = canvas.width; c.height = canvas.height;
  const x = c.getContext("2d");
  const p = Math.round(Math.min(c.width, c.height) * pad);
  x.fillStyle = ink; x.fillRect(0, 0, c.width, c.height);
  x.drawImage(canvas, p, p, c.width - p * 2, c.height - p * 2);
  if (word && c.width > 200) {
    x.fillStyle = "rgba(255,255,255,.18)"; x.font = `700 ${Math.round(p * 0.7)}px Oswald`; x.textAlign = "center"; x.textBaseline = "middle";
    x.fillText(word, c.width / 2, p / 2); x.fillText(word, c.width / 2, c.height - p / 2);
  }
  return c;
}

// streaks: shrink wrap glare
let wrapTex = null;
function streaks() {
  if (wrapTex) return wrapTex;
  const c = document.createElement("canvas");
  c.width = 256; c.height = 512;
  const x = c.getContext("2d");
  x.strokeStyle = "rgba(255,255,255,.55)";
  for (let i = 0; i < 14; i++) { x.lineWidth = 1 + Math.random() * 4; x.beginPath(); const y = Math.random() * 512; x.moveTo(0, y); x.bezierCurveTo(80, y - 40, 170, y + 50, 256, y + (Math.random() - 0.5) * 80); x.stroke(); }
  wrapTex = new THREE.CanvasTexture(c);
  return wrapTex;
}

export function dressCase(g, faces) {
  const { mats } = g.userData;
  const kind = faces.case || "";
  const ink = CASE_INKS[faces.caseInk || ""] ?? CASE_INKS[""];
  const inkCss = "#" + ink.toString(16).padStart(6, "0");
  const rough = kind === "slipcover" ? 0.85 : kind === "clamshell" || kind === "shrink" ? 0.18 : 0.42;
  const set = (i, canvas) => {
    if (!canvas) return;
    if (kind === "clamshell") canvas = rimmed(canvas, inkCss, 0.03);
    else if (kind === "rental") canvas = rimmed(canvas, inkCss, 0.08, i === 1 ? "" : "VIDEO");
    if (mats[i].map && mats[i].userData.mine) mats[i].map.dispose();
    mats[i].map = tex(canvas);
    mats[i].userData.mine = true;
    mats[i].color.set(0xffffff);
    mats[i].roughness = rough;
    mats[i].needsUpdate = true;
  };
  set(4, faces.front);
  set(5, faces.back);
  set(1, faces.spine);
  for (const i of [2, 3]) { mats[i].color.set(ink); mats[i].roughness = rough; }
  if (g.userData.wrap) { g.remove(g.userData.wrap); g.userData.wrap = null; }
  if (kind === "shrink") {
    const m = new THREE.MeshStandardMaterial({ color: 0xffffff, map: streaks(), transparent: true, opacity: 0.22, roughness: 0.05, depthWrite: false });
    const w = new THREE.Mesh(new THREE.BoxGeometry(CW * 1.01, CH * 1.01, CD * 1.03), m);
    g.add(w);
    g.userData.wrap = w;
  }
  if (g.userData.foil) { g.remove(g.userData.foil); g.userData.foil = null; }
  if (faces.mask) {
    const am = tex(faces.mask);
    am.colorSpace = THREE.NoColorSpace;
    const m = new THREE.MeshBasicMaterial({ map: shine(), alphaMap: am, transparent: true, blending: THREE.AdditiveBlending, depthWrite: false, opacity: 0.9 });
    const p = new THREE.Mesh(new THREE.PlaneGeometry(CW, CH), m);
    p.position.z = CD / 2 + 0.002;
    g.add(p);
    g.userData.foil = p;
  }
}

function lights(scene) {
  scene.add(new THREE.HemisphereLight(0xffffff, 0x221111, 1.1));
  const key = new THREE.DirectionalLight(0xffffff, 1.6); key.position.set(2, 3, 4); scene.add(key);
  const rim = new THREE.DirectionalLight(0xff5040, 1.2); rim.position.set(-3, 1, -2); scene.add(rim);
}

function newRenderer(el) {
  const r = new THREE.WebGLRenderer({ antialias: true, alpha: true });
  r.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  r.outputColorSpace = THREE.SRGBColorSpace;
  el.appendChild(r.domElement);
  return r;
}

// spinner lets you turn a case with the mouse; it drifts back to facing you when you let go
function spinner(el, target, opts = {}) {
  const s = { ry: 0, rx: 0, vy: 0, vx: 0, goal: 0, drag: false, last: 0, px: 0, py: 0 };
  const down = (e) => { s.drag = true; s.px = e.clientX; s.py = e.clientY; s.vy = s.vx = 0; el.setPointerCapture(e.pointerId); };
  const move = (e) => {
    if (!s.drag) return;
    const dx = e.clientX - s.px, dy = e.clientY - s.py;
    s.px = e.clientX; s.py = e.clientY;
    s.vy = dx * 0.012; s.vx = dy * 0.008;
    s.ry += s.vy; s.rx = Math.max(-0.7, Math.min(0.7, s.rx + s.vx));
    s.last = performance.now();
  };
  const up = () => {
    if (!s.drag) return;
    s.drag = false;
    // settle on whichever side is nearer
    s.goal = Math.round((s.ry + s.vy * 8) / Math.PI) * Math.PI;
  };
  el.addEventListener("pointerdown", down);
  el.addEventListener("pointermove", move);
  el.addEventListener("pointerup", up);
  el.addEventListener("pointercancel", up);
  s.flip = () => { s.goal = Math.round(s.ry / Math.PI) * Math.PI + Math.PI; };
  s.tick = (t, dt) => {
    if (!s.drag) {
      const idle = performance.now() - s.last > 2500;
      s.vy *= 0.92;
      s.ry += s.vy;
      s.ry += (s.goal + (idle && opts.sway ? Math.sin(t * 0.6) * 0.18 : 0) - s.ry) * Math.min(1, dt * 3);
      s.rx += ((opts.sway ? Math.sin(t * 0.45) * 0.05 : 0) - s.rx) * Math.min(1, dt * 3);
    }
    target.rotation.y = s.ry;
    target.rotation.x = s.rx;
  };
  s.dispose = () => {
    el.removeEventListener("pointerdown", down);
    el.removeEventListener("pointermove", move);
    el.removeEventListener("pointerup", up);
    el.removeEventListener("pointercancel", up);
  };
  return s;
}

function foilTick(g, angle) {
  const f = g.userData.foil;
  if (f) f.material.map.offset.x = -angle * 0.35;
}

// CaseView shows one case. make() resolves to { front, back, spine, mask }; it runs again when key changes.
export function CaseView({ make, key: k, height = 520, sway = true }) {
  const box = useRef(null);
  const ctl = useRef(null);
  const [ready, setReady] = useState(false);
  useEffect(() => {
    const el = box.current;
    const renderer = newRenderer(el);
    const scene = new THREE.Scene();
    lights(scene);
    const camera = new THREE.PerspectiveCamera(28, 1, 0.05, 50);
    const g = makeCase();
    scene.add(g);
    const sp = spinner(renderer.domElement, g, { sway });
    ctl.current = sp;
    let alive = true;
    make().then((faces) => { if (alive && faces) { dressCase(g, faces); setReady(true); } });
    const resize = () => {
      const w = el.clientWidth || 300, h = el.clientHeight || 400;
      renderer.setSize(w, h);
      camera.aspect = w / h;
      const f = 2 * Math.tan((camera.fov * Math.PI) / 360);
      camera.position.set(0, 0, Math.max((CH * 1.15) / f, (CW * 1.6) / (f * camera.aspect)));
      camera.lookAt(0, 0, 0);
      camera.updateProjectionMatrix();
    };
    const ro = new ResizeObserver(resize);
    ro.observe(el);
    resize();
    let raf = 0, prev = performance.now();
    const t0 = prev;
    const loop = () => {
      if (!alive) return;
      raf = requestAnimationFrame(loop);
      if (!el.isConnected || document.hidden) return;
      const now = performance.now();
      sp.tick((now - t0) / 1000, Math.min(0.05, (now - prev) / 1000));
      prev = now;
      foilTick(g, g.rotation.y + g.rotation.x);
      renderer.render(scene, camera);
    };
    loop();
    return () => {
      alive = false;
      cancelAnimationFrame(raf);
      ro.disconnect();
      sp.dispose();
      renderer.dispose();
      renderer.forceContextLoss();
      el.innerHTML = "";
    };
  }, [k]);
  return html`<div class="case-view" style=${{ height: height + "px" }}>
    <div class="case-canvas" ref=${box}></div>
    ${ready ? null : html`<div class="case-wait"><div class="spinner sm"></div></div>`}
    <button class="btn sm case-flip" onClick=${() => { ctl.current && ctl.current.flip(); sound("flip"); }}>Turn over</button>
  </div>`;
}

// Shelf ------------------------------------------------------------------------------------------------

const GAP = 0.025, ROW_H = CH + 0.32, BOARD = 0.07;

// shelfAspect: width / height of a shelf unit, so the page can size the view to fit it exactly
export function shelfAspect(perRow, rows) {
  return (perRow * (CD + GAP) + 0.3) / (rows * ROW_H + BOARD);
}

// ShelfView: tapes stand spine out. Hover pulls one out a little; click takes it off the shelf.
// tapes: [{ id, spine: Promise<canvas> }], faces(tape) -> Promise<{front, back, spine, mask}>
// SHELF_KIT: what each shelf type is made of. prefs: { shelf, light, props }
const SHELF_KIT = {
  metal: { board: 0x151314, back: 0x0c0a0a, strip: 0x8a120c, metal: 0.3, rough: 0.7 },
  dark: { board: 0x2a1810, back: 0x120a06, strip: 0x8a6a2a, metal: 0, rough: 0.8 },
  pine: { board: 0xc9a36a, back: 0x6a5034, strip: 0x3a2a18, metal: 0, rough: 0.75 },
  wire: { board: 0x9a9ca0, back: null, strip: 0xd8d8d8, metal: 0.8, rough: 0.35, wire: true },
  crate: { board: 0x7a5a36, back: 0x4a3420, strip: null, metal: 0, rough: 0.9, slats: true },
  gun: { board: 0x3a2214, back: 0x0e2a16, strip: 0xb08a3a, metal: 0, rough: 0.6, glass: true },
};
const LIGHT_KIT = {
  "": { hemi: [0xfff0e0, 0x140808, 0.9], key: [0xffe6d0, 1.4], point: [0xff3a2a, 6] },
  warm: { hemi: [0xffe0b0, 0x201008, 1.0], key: [0xffc890, 1.5], point: [0xff8a3a, 5] },
  cold: { hemi: [0xd0e0ff, 0x080c14, 0.95], key: [0xb0d0ff, 1.4], point: [0x3a8aff, 5] },
  candle: { hemi: [0xffb070, 0x080402, 0.35], key: [0xffb070, 0.45], point: [0xff9a40, 9] },
};

// shelfProp: a small thing sitting on top of the shelf
function shelfProp(id) {
  const g = new THREE.Group();
  const m = (c, r = 0.6, mt = 0) => new THREE.MeshStandardMaterial({ color: c, roughness: r, metalness: mt });
  if (id === "bobblehead") {
    const body = new THREE.Mesh(new THREE.CylinderGeometry(0.06, 0.08, 0.18, 16), m(0x2a5aa8)); body.position.y = 0.09; g.add(body);
    const head = new THREE.Mesh(new THREE.SphereGeometry(0.11, 20, 16), m(0xf0c8a0)); head.position.y = 0.28; g.add(head);
    const base = new THREE.Mesh(new THREE.CylinderGeometry(0.1, 0.1, 0.03, 20), m(0x111111)); g.add(base);
    g.userData.bob = head;
  } else if (id === "plant") {
    const pot = new THREE.Mesh(new THREE.CylinderGeometry(0.12, 0.09, 0.18, 16), m(0xa8502a, 0.9)); pot.position.y = 0.09; g.add(pot);
    for (let i = 0; i < 7; i++) {
      const leaf = new THREE.Mesh(new THREE.ConeGeometry(0.035, 0.32, 6), m(0x3a7a2a, 0.8));
      leaf.position.set(Math.cos(i) * 0.04, 0.3, Math.sin(i) * 0.04); leaf.rotation.set(Math.sin(i * 2) * 0.5, 0, Math.cos(i * 2) * 0.5); g.add(leaf);
    }
  } else if (id === "bat") {
    const bat = new THREE.Mesh(new THREE.CylinderGeometry(0.035, 0.018, 0.9, 12), m(0xc8a070, 0.5)); bat.position.y = 0.42; bat.rotation.z = 0.35; g.add(bat);
    for (let i = 0; i < 4; i++) { const n = new THREE.Mesh(new THREE.CylinderGeometry(0.005, 0.005, 0.09, 4), m(0x888888, 0.3, 0.8)); n.position.set(-0.12 + i * 0.015, 0.7 - i * 0.04, 0.03); n.rotation.x = Math.PI / 2; g.add(n); }
  } else if (id === "radio") {
    const box = new THREE.Mesh(new THREE.BoxGeometry(0.42, 0.22, 0.2), m(0x3a3d2a, 0.7)); box.position.y = 0.11; g.add(box);
    const dial = new THREE.Mesh(new THREE.CylinderGeometry(0.04, 0.04, 0.02, 16), m(0xd8d0b0)); dial.rotation.x = Math.PI / 2; dial.position.set(0.12, 0.12, 0.105); g.add(dial);
    const grill = new THREE.Mesh(new THREE.PlaneGeometry(0.18, 0.14), m(0x111111)); grill.position.set(-0.08, 0.11, 0.101); g.add(grill);
    const ant = new THREE.Mesh(new THREE.CylinderGeometry(0.005, 0.005, 0.5, 6), m(0xaaaaaa, 0.3, 0.9)); ant.position.set(0.16, 0.45, 0); ant.rotation.z = -0.4; g.add(ant);
  }
  return g;
}

export function ShelfView({ tapes, perRow, rows, faces, picked, onPick, prefs = {} }) {
  const box = useRef(null);
  const api = useRef({});
  useEffect(() => {
    const el = box.current;
    const renderer = newRenderer(el);
    const scene = new THREE.Scene();
    const lk = LIGHT_KIT[prefs.light || ""] || LIGHT_KIT[""];
    scene.add(new THREE.HemisphereLight(...lk.hemi));
    const key = new THREE.DirectionalLight(...lk.key); key.position.set(1.5, 4, 5); scene.add(key);
    const red = new THREE.PointLight(lk.point[0], lk.point[1], 12, 1.6); red.position.set(-3, 2, 2); scene.add(red);
    const camera = new THREE.PerspectiveCamera(32, 1, 0.05, 60);

    // the shelf unit: black metal, like a rental store's
    const unitW = perRow * (CD + GAP) + 0.3;
    const unitH = rows * ROW_H + BOARD;
    const kit = SHELF_KIT[prefs.shelf] || SHELF_KIT.metal;
    const metal = new THREE.MeshStandardMaterial({ color: kit.board, roughness: kit.rough, metalness: kit.metal });
    if (kit.back !== null) {
      const backMat = new THREE.MeshStandardMaterial({ color: kit.back, roughness: 0.9 });
      if (kit.slats) { // crate: planks with gaps
        for (let y = 0.1; y < unitH; y += 0.32) { const sl = new THREE.Mesh(new THREE.BoxGeometry(unitW, 0.24, 0.05), backMat); sl.position.set(0, y, -CW / 2 - 0.08); scene.add(sl); }
      } else {
        const back = new THREE.Mesh(new THREE.BoxGeometry(unitW, unitH, 0.05), backMat);
        back.position.set(0, unitH / 2, -CW / 2 - 0.08);
        scene.add(back);
      }
    }
    for (let r = 0; r <= rows; r++) {
      if (kit.wire) { // wire rack: bars, not boards
        for (let k = 0; k < 5; k++) { const bar = new THREE.Mesh(new THREE.CylinderGeometry(0.012, 0.012, unitW + 0.1, 6), metal); bar.rotation.z = Math.PI / 2; bar.position.set(0, r * ROW_H + BOARD / 2, -CW / 2 + k * CW / 4); scene.add(bar); }
      } else {
        const b = new THREE.Mesh(new THREE.BoxGeometry(unitW + 0.1, BOARD, CW + 0.2), metal);
        b.position.set(0, r * ROW_H + BOARD / 2, 0);
        scene.add(b);
      }
      if (r < rows && kit.strip !== null) { // a price strip on the front edge
        const strip = new THREE.Mesh(new THREE.BoxGeometry(unitW + 0.1, 0.05, 0.01), new THREE.MeshStandardMaterial({ color: kit.strip, roughness: 0.6, metalness: kit.wire ? 0.6 : 0 }));
        strip.position.set(0, r * ROW_H + BOARD / 2, CW / 2 + 0.105);
        scene.add(strip);
      }
    }
    for (const sx of [-1, 1]) {
      const side = new THREE.Mesh(new THREE.BoxGeometry(kit.wire ? 0.025 : kit.slats ? 0.12 : 0.07, unitH, CW + 0.2), metal);
      side.position.set(sx * (unitW / 2 + 0.05), unitH / 2, 0);
      scene.add(side);
    }
    if (kit.glass) { // gun cabinet: a glass door
      const glass = new THREE.Mesh(new THREE.PlaneGeometry(unitW + 0.1, unitH), new THREE.MeshStandardMaterial({ color: 0xaaccdd, transparent: true, opacity: 0.08, roughness: 0.05, depthWrite: false }));
      glass.position.set(0, unitH / 2, CW / 2 + 0.16);
      scene.add(glass);
    }
    const propList = (prefs.props || []).slice(0, 4);
    const bobs = [];
    propList.forEach((id, i) => {
      const pr = shelfProp(id);
      pr.scale.setScalar(1.7);
      pr.position.set(-unitW / 2 + 0.3 + (i + 0.5) * ((unitW - 0.6) / propList.length), unitH, 0);
      scene.add(pr);
      if (pr.userData.bob) bobs.push(pr.userData.bob);
    });
    // a dark curtain that drops between the shelf and a tape you took out
    const dim = new THREE.Mesh(new THREE.PlaneGeometry(40, 40), new THREE.MeshBasicMaterial({ color: 0x000000, transparent: true, opacity: 0, depthWrite: false }));
    dim.position.set(0, unitH / 2, CW / 2 + 0.3);
    scene.add(dim);

    const cases = tapes.map((t, i) => {
      const g = makeCase();
      const r = rows - 1 - Math.floor(i / perRow), c = i % perRow;
      const home = new THREE.Vector3(-unitW / 2 + 0.15 + CD / 2 + c * (CD + GAP), r * ROW_H + BOARD + CH / 2 + 0.001, t.out ? 0.28 : 0); // a friend hosting now pokes out
      g.position.copy(home);
      g.rotation.y = Math.PI / 2;
      g.userData.home = home;
      g.userData.tape = t;
      g.userData.pull = 0;
      scene.add(g);
      t.spine.then((cv) => cv && dressCase(g, { spine: cv, case: t.case, caseInk: t.caseInk }));
      return g;
    });

    let held = null, sp = null, hover = null;
    const ray = new THREE.Raycaster(), mouse = new THREE.Vector2();
    const hit = (e) => {
      const r = renderer.domElement.getBoundingClientRect();
      mouse.set(((e.clientX - r.left) / r.width) * 2 - 1, -((e.clientY - r.top) / r.height) * 2 + 1);
      ray.setFromCamera(mouse, camera);
      const hits = ray.intersectObjects(cases.map((g) => g.userData.mesh));
      return hits.length ? cases.find((g) => g.userData.mesh === hits[0].object) : null;
    };
    const onMove = (e) => {
      if (held) return;
      hover = hit(e);
      renderer.domElement.style.cursor = hover ? "pointer" : "default";
    };
    const onClick = (e) => {
      if (held) return;
      const g = hit(e);
      if (g) onPick(g.userData.tape.id);
    };
    renderer.domElement.addEventListener("pointermove", onMove);
    renderer.domElement.addEventListener("click", onClick);

    const take = (id) => {
      const g = cases.find((c) => c.userData.tape.id === id);
      if (held === g) return;
      if (held) putBack();
      if (!g) return;
      held = g;
      held.userData.flying = 0;
      sp = spinner(renderer.domElement, { rotation: { y: 0, x: 0 } }, {});
      held.userData.sp = sp;
      sound("clack");
      faces(g.userData.tape).then((f) => f && held === g && dressCase(g, f));
    };
    const putBack = () => {
      if (!held) return;
      held.userData.sp.dispose();
      held.userData.returning = 0;
      held = null;
      sp = null;
      sound("clack");
    };
    api.current = { take, putBack, flip: () => sp && sp.flip() };

    const resize = () => {
      const w = el.clientWidth || 600, h = el.clientHeight || 500;
      renderer.setSize(w, h);
      camera.aspect = w / h;
      const f = 2 * Math.tan((camera.fov * Math.PI) / 360);
      // far enough back that a tape you take out fits between you and the curtain
      const top = unitH + (propList.length ? 0.9 : 0); // props stand on top of the unit
      const dist = Math.max((top * 1.08) / f, (unitW * 1.08) / (f * camera.aspect), (CH * 1.25) / f + 0.6) + CW / 2;
      camera.position.set(0, top / 2 + 0.2, dist);
      camera.lookAt(0, top / 2, 0);
      camera.updateProjectionMatrix();
      camera.userData.dist = dist;
    };
    const ro = new ResizeObserver(resize);
    ro.observe(el);
    resize();

    let alive = true, raf = 0, prev = performance.now();
    const t0 = prev;
    const front = new THREE.Vector3();
    const loop = () => {
      if (!alive) return;
      raf = requestAnimationFrame(loop);
      if (!el.isConnected || document.hidden) return;
      const now = performance.now(), dt = Math.min(0.05, (now - prev) / 1000), t = (now - t0) / 1000;
      prev = now;
      dim.material.opacity += ((held ? 0.78 : 0) - dim.material.opacity) * Math.min(1, dt * 6);
      for (const b of bobs) b.rotation.z = Math.sin(t * 3) * 0.12;
      if (prefs.light === "candle") red.intensity = lk.point[1] * (0.85 + Math.sin(t * 9) * 0.06 + Math.sin(t * 23) * 0.05);
      // where a held tape floats: in front of the camera, big enough to read
      const fz = camera.position.z - Math.max(CH * 1.25 / (2 * Math.tan((camera.fov * Math.PI) / 360)), 2.2);
      front.set(0, camera.position.y - 0.2, fz);
      for (const g of cases) {
        const u = g.userData;
        if (g === held) {
          u.flying = Math.min(1, u.flying + dt * 2.2);
          const k = 1 - Math.pow(1 - u.flying, 3);
          g.position.lerpVectors(u.home, front, k);
          sp.tick(t, dt);
          const s = u.sp;
          // turn from spine-out to face-on while it flies
          g.rotation.y = Math.PI / 2 * (1 - k) + s.ry * k;
          g.rotation.x = s.rx * k;
          foilTick(g, g.rotation.y);
          continue;
        }
        if (u.returning !== undefined) {
          u.returning = Math.min(1, u.returning + dt * 2.5);
          const k = 1 - Math.pow(1 - u.returning, 3);
          g.position.lerp(u.home, k);
          g.rotation.y += (Math.PI / 2 - g.rotation.y) * k;
          g.rotation.x *= 1 - k;
          if (u.returning >= 1) delete u.returning;
          continue;
        }
        u.pull += ((g === hover ? 0.22 : 0) - u.pull) * Math.min(1, dt * 10);
        g.position.set(u.home.x, u.home.y, u.home.z + u.pull);
      }
      renderer.render(scene, camera);
    };
    loop();
    return () => {
      alive = false;
      cancelAnimationFrame(raf);
      ro.disconnect();
      if (sp) sp.dispose();
      renderer.domElement.removeEventListener("pointermove", onMove);
      renderer.domElement.removeEventListener("click", onClick);
      renderer.dispose();
      renderer.forceContextLoss();
      el.innerHTML = "";
    };
  }, [tapes.map((t) => t.id + (t.case || "") + (t.caseInk || "")).join("|") + JSON.stringify(prefs)]);
  useEffect(() => {
    if (picked) api.current.take && api.current.take(picked);
    else api.current.putBack && api.current.putBack();
  }, [picked]);
  return html`<div class="shelf-canvas" ref=${box}></div>${picked ? html`<button class="btn sm shelf-flip" onClick=${() => { api.current.flip && api.current.flip(); sound("flip"); }}>Turn over</button>` : null}`;
}
