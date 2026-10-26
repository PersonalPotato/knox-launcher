// 3D VHS cases: one case you can turn over, and a shelf of them.
import * as THREE from "./vendor/three.module.js";
import { html, useEffect, useRef, useState } from "./core.js";
import { W, H, SPINE } from "./tape.js";
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

export function dressCase(g, faces) {
  const { mats } = g.userData;
  const set = (i, canvas) => {
    if (!canvas) return;
    if (mats[i].map && mats[i].userData.mine) mats[i].map.dispose();
    mats[i].map = tex(canvas);
    mats[i].userData.mine = true;
    mats[i].color.set(0xffffff);
    mats[i].roughness = 0.42;
    mats[i].needsUpdate = true;
  };
  set(4, faces.front);
  set(5, faces.back);
  set(1, faces.spine);
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
export function ShelfView({ tapes, perRow, rows, faces, picked, onPick }) {
  const box = useRef(null);
  const api = useRef({});
  useEffect(() => {
    const el = box.current;
    const renderer = newRenderer(el);
    const scene = new THREE.Scene();
    scene.add(new THREE.HemisphereLight(0xfff0e0, 0x140808, 0.9));
    const key = new THREE.DirectionalLight(0xffe6d0, 1.4); key.position.set(1.5, 4, 5); scene.add(key);
    const red = new THREE.PointLight(0xff3a2a, 6, 12, 1.6); red.position.set(-3, 2, 2); scene.add(red);
    const camera = new THREE.PerspectiveCamera(32, 1, 0.05, 60);

    // the shelf unit: black metal, like a rental store's
    const unitW = perRow * (CD + GAP) + 0.3;
    const unitH = rows * ROW_H + BOARD;
    const metal = new THREE.MeshStandardMaterial({ color: 0x151314, roughness: 0.7, metalness: 0.3 });
    const back = new THREE.Mesh(new THREE.BoxGeometry(unitW, unitH, 0.05), new THREE.MeshStandardMaterial({ color: 0x0c0a0a, roughness: 0.9 }));
    back.position.set(0, unitH / 2, -CW / 2 - 0.08);
    scene.add(back);
    for (let r = 0; r <= rows; r++) {
      const b = new THREE.Mesh(new THREE.BoxGeometry(unitW + 0.1, BOARD, CW + 0.2), metal);
      b.position.set(0, r * ROW_H + BOARD / 2, 0);
      scene.add(b);
      if (r < rows) { // a red price strip on the front edge
        const strip = new THREE.Mesh(new THREE.BoxGeometry(unitW + 0.1, 0.05, 0.01), new THREE.MeshStandardMaterial({ color: 0x8a120c, roughness: 0.6 }));
        strip.position.set(0, r * ROW_H + BOARD / 2, CW / 2 + 0.105);
        scene.add(strip);
      }
    }
    for (const sx of [-1, 1]) {
      const side = new THREE.Mesh(new THREE.BoxGeometry(0.07, unitH, CW + 0.2), metal);
      side.position.set(sx * (unitW / 2 + 0.05), unitH / 2, 0);
      scene.add(side);
    }
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
      t.spine.then((cv) => cv && dressCase(g, { spine: cv }));
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
      const dist = Math.max((unitH * 1.08) / f, (unitW * 1.08) / (f * camera.aspect), (CH * 1.25) / f + 0.6) + CW / 2;
      camera.position.set(0, unitH / 2 + 0.2, dist);
      camera.lookAt(0, unitH / 2, 0);
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
      // where a held tape floats: in front of the camera, big enough to read
      const fz = camera.position.z - Math.max(CH * 1.25 / (2 * Math.tan((camera.fov * Math.PI) / 360)), 2.2);
      front.set(0, unitH / 2, fz);
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
  }, [tapes.map((t) => t.id).join("|")]);
  useEffect(() => {
    if (picked) api.current.take && api.current.take(picked);
    else api.current.putBack && api.current.putBack();
  }, [picked]);
  return html`<div class="shelf-canvas" ref=${box}></div>${picked ? html`<button class="btn sm shelf-flip" onClick=${() => { api.current.flip && api.current.flip(); sound("flip"); }}>Turn over</button>` : null}`;
}
