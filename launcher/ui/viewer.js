// 3D character preview built from the game's own models and textures.
import * as THREE from "./vendor/three.module.js";
import { OrbitControls } from "./vendor/OrbitControls.js";
import { html, useEffect, useRef, useState, S, TOKEN } from "./core.js";

const meshCache = new Map();
const imgCache = new Map();

function loadMesh(path) {
  if (!path) return Promise.resolve(null);
  const key = path.toLowerCase();
  if (!meshCache.has(key)) {
    meshCache.set(key, fetch(`/model?t=${TOKEN}&p=${encodeURIComponent(path)}`)
      .then((r) => (r.ok ? r.json() : null))
      .then((m) => {
        if (!m) return null;
        const g = new THREE.BufferGeometry();
        g.setAttribute("position", new THREE.Float32BufferAttribute(m.p, 3));
        g.setAttribute("uv", new THREE.Float32BufferAttribute(m.uv, 2));
        g.setIndex(m.i);
        g.computeVertexNormals();
        if (m.j && m.j.length === (m.p.length / 3) * 4) {
          g.setAttribute("skinIndex", new THREE.Uint16BufferAttribute(m.j, 4));
          g.setAttribute("skinWeight", new THREE.Float32BufferAttribute(m.w, 4));
          g.userData = { bones: m.b, offsets: m.o, frames: m.f };
        }
        return g;
      })
      .catch(() => null));
  }
  return meshCache.get(key);
}

// makeSkeleton turns the body file's frames into bones, in their bind pose.
function makeSkeleton(frames) {
  const bones = {}, list = [], roots = [];
  for (const f of frames || []) {
    if (f.n === "Body") continue;
    const b = new THREE.Bone();
    b.name = f.n;
    new THREE.Matrix4().fromArray(f.m).decompose(b.position, b.quaternion, b.scale);
    list.push(b);
    bones[f.n] = b;
    const parent = f.p >= 0 ? frames[f.p] : null;
    if (parent && bones[parent.n]) bones[parent.n].add(b);
    else roots.push(b);
  }
  return { bones, roots };
}

// skinnedMesh wires a mesh to the shared bones. Each file brings its own offset matrices.
function skinnedMesh(geo, mat, rig) {
  const d = geo.userData || {};
  if (!rig || !d.bones || !d.bones.every((n) => rig.bones[n])) return new THREE.Mesh(geo, mat);
  const mesh = new THREE.SkinnedMesh(geo, mat);
  const inv = d.bones.map((_, i) => new THREE.Matrix4().fromArray(d.offsets.slice(i * 16, i * 16 + 16)));
  mesh.bind(new THREE.Skeleton(d.bones.map((n) => rig.bones[n]), inv), new THREE.Matrix4());
  mesh.frustumCulled = false;
  return mesh;
}

// Animations ---------------------------------------------------------------------------------------

const animCache = new Map();
export function loadClip(name) {
  if (!animCache.has(name)) {
    animCache.set(name, fetch(`/anim?t=${TOKEN}&n=${encodeURIComponent(name)}`)
      .then((r) => (r.ok ? r.json() : null))
      .then((a) => {
        if (!a || !a.tracks) return null;
        const tracks = [];
        for (const t of a.tracks) {
          if (t.bone === "Translation_Data" || t.bone.endsWith("Nub")) continue;
          if (t.rt) tracks.push(new THREE.QuaternionKeyframeTrack(t.bone + ".quaternion", t.rt, t.r));
          if (t.pt) tracks.push(new THREE.VectorKeyframeTrack(t.bone + ".position", t.pt, t.p));
          if (t.st) tracks.push(new THREE.VectorKeyframeTrack(t.bone + ".scale", t.st, t.s));
        }
        return new THREE.AnimationClip(name, a.length || -1, tracks);
      })
      .catch(() => null));
  }
  return animCache.get(name);
}

// What the survivor can do. loop: keeps going until something else is picked.
export const MOVES = [
  { id: "idle", label: "Idle", anim: "Bob_Idle", loop: true },
  { id: "wave", label: "Wave", anim: "Bob_EmoteWaveHi" },
  { id: "thumbs", label: "Thumbs up", anim: "Bob_EmoteThumbsUp" },
  { id: "clap", label: "Clap", anim: "Bob_EmoteClap" },
  { id: "shrug", label: "Shrug", anim: "Bob_EmoteShrug" },
  { id: "salute", label: "Salute", anim: "Bob_EmoteSalute_Formal" },
  { id: "comehere", label: "Come here", anim: "Bob_EmoteComeHere" },
  { id: "stretch", label: "Stretch", anim: "Bob_EmoteTiredStretch" },
  { id: "yawn", label: "Yawn", anim: "Bob_EmoteYawn" },
  { id: "spooked", label: "Spooked", anim: "Bob_EmoteSpooked" },
  { id: "surrender", label: "Hands up", anim: "Bob_EmoteSurrender" },
  { id: "insult", label: "Rude", anim: "Bob_EmoteInsult" },
  { id: "walk", label: "Walk", anim: "Bob_Walk", loop: true },
  { id: "run", label: "Run", anim: "Bob_Run", loop: true },
  { id: "sneak", label: "Sneak", anim: "Bob_WalkLowSneak", loop: true },
  { id: "sit", label: "Sit", anim: "Bob_SitGround_Idle", loop: true, intro: "Bob_SitGround_In" },
];
const IDLE_EXTRAS = ["Bob_IdleExt01", "Bob_IdleExt02", "Bob_IdleExt03", "Bob_IdleExt04", "Bob_IdleExt05", "Bob_LookLeft", "Bob_LookRight", "Bob_EmoteWipeBrow"];

// Animator plays the game's clips on one character: a looping base, one-shots on top that fade back.
class Animator {
  constructor(root) {
    this.mixer = new THREE.AnimationMixer(root);
    this.base = null;
    this.cur = null;
    this.baseId = "idle";
    this.nextExtra = 6 + Math.random() * 6;
    this.mixer.addEventListener("finished", (e) => {
      if (e.action !== this.cur || e.action === this.base) return;
      if (this.after) { const f = this.after; this.after = null; f(); return; }
      this.fadeTo(this.base, 0.35);
    });
  }
  fadeTo(action, t = 0.3) {
    if (!action || action === this.cur) return;
    action.reset().setEffectiveWeight(1).play();
    if (this.cur) this.cur.crossFadeTo(action, t, false);
    this.cur = action;
  }
  async play(move) {
    const m = typeof move === "string" ? MOVES.find((x) => x.id === move) : move;
    if (!m) return;
    const clip = await loadClip(m.anim);
    if (!clip) return;
    const a = this.mixer.clipAction(clip);
    if (m.loop) {
      a.setLoop(THREE.LoopRepeat, Infinity);
      this.baseId = m.id;
      const start = () => { this.base = a; this.fadeTo(a, 0.3); };
      if (m.intro) {
        const intro = await loadClip(m.intro);
        if (intro) {
          const ia = this.mixer.clipAction(intro);
          ia.setLoop(THREE.LoopOnce, 1);
          ia.clampWhenFinished = true;
          this.base = a;
          this.after = () => { this.cur = ia; this.fadeTo(a, 0.2); };
          this.fadeTo(ia, 0.3);
          return;
        }
      }
      start();
    } else {
      a.setLoop(THREE.LoopOnce, 1);
      a.clampWhenFinished = true;
      this.after = null;
      this.fadeTo(a, 0.25);
    }
  }
  update(dt) {
    this.mixer.update(dt);
    // standing still: now and then do one of the game's idle fidgets
    if (this.baseId === "idle" && this.cur === this.base) {
      this.nextExtra -= dt;
      if (this.nextExtra <= 0) {
        this.nextExtra = 7 + Math.random() * 8;
        this.play({ anim: IDLE_EXTRAS[(Math.random() * IDLE_EXTRAS.length) | 0] });
      }
    }
  }
}

const texLoader = new THREE.TextureLoader();
const bodyCache = new Map();
function loadBodyTexture(url) {
  if (!bodyCache.has(url)) {
    bodyCache.set(url, new Promise((resolve) => texLoader.load(url, (t) => {
      t.flipY = false;
      t.colorSpace = THREE.SRGBColorSpace;
      t.anisotropy = 4;
      t.shared = true;
      resolve(t);
    }, undefined, () => resolve(null))));
  }
  return bodyCache.get(url);
}

function loadImage(path) {
  if (!path) return Promise.resolve(null);
  const key = path.toLowerCase();
  if (!imgCache.has(key)) {
    imgCache.set(key, new Promise((resolve) => {
      const img = new Image();
      img.onload = () => resolve(img);
      img.onerror = () => resolve(null);
      img.src = `/tex?t=${TOKEN}&p=${encodeURIComponent(path)}`;
    }));
  }
  return imgCache.get(key);
}

// tinted copies a texture and multiplies it by a colour, keeping its transparency.
function tinted(img, color) {
  const c = document.createElement("canvas");
  c.width = img.width;
  c.height = img.height;
  const x = c.getContext("2d");
  x.drawImage(img, 0, 0);
  if (color) {
    x.globalCompositeOperation = "multiply";
    x.fillStyle = `rgb(${color.r * 255},${color.g * 255},${color.b * 255})`;
    x.fillRect(0, 0, c.width, c.height);
    x.globalCompositeOperation = "destination-in";
    x.drawImage(img, 0, 0);
  }
  return c;
}

function canvasTexture(canvas) {
  const t = new THREE.CanvasTexture(canvas);
  t.flipY = false;
  t.colorSpace = THREE.SRGBColorSpace;
  t.anisotropy = 4;
  return t;
}

function hashPick(list, seed) {
  if (!list || !list.length) return null;
  let h = 0;
  for (const ch of seed) h = (h * 31 + ch.charCodeAt(0)) >>> 0;
  return list[h % list.length];
}

// The game's body locations, inner to outer (BodyLocations.lua). Clothes draw in this order.
const LAYERS = ["bandage","wound","beltextra","belt","bellybutton","makeupfullface","makeupeyes","makeupeyesshadow","makeuplips","mask","maskeyes","maskfull","underwearbottom","underweartop","underwear","underwearextra1","underwearextra2","hat","fullhat","ears","eartop","nose","torso1","torso1legs1","tanktop","tshirt","shortsleeveshirt","leftwrist","rightwrist","shirt","rightarm","leftarm","neck","necktexture","necklace","necklacelong","rightmiddlefinger","leftmiddlefinger","leftringfinger","rightringfinger","hands","handsleft","handsright","calflefttexture","calfrighttexture","socks","legs1","shoes","codpiece","shortsshort","shortpants","pantsskinny","gaiterright","gaiterleft","pants","skirt","dress","legs5","forearmleft","forearmright","longskirt","longdress","vesttexture","bodycostume","sportshoulderpad","gorget","jersey","sweater","sweaterhat","pantsextra","jacket","jacketdown","jacketbulky","jackethat","jackethatbulky","jacketsuit","fullsuit","boilersuit","fullsuithead","fullsuitheadscba","kneeleft","kneeright","calfleft","calfright","thighleft","thighright","sportshoulderpadontop","shoulderpadright","shoulderpadleft","elbowleft","elbowright","fulltop","bathrobe","fannypackfront","fannypackback","webbing","scba","scbanotank","ammostrap","satchel","shoulderholster","ankleholster","torsoextra","torsoextravest","torsoextravestbullet","cuirass","tail","fullrobe","back","lefteye","righteye","eyes","scarf"];
const layerOf = (loc) => { const i = LAYERS.indexOf(String(loc).replace(/[_\s]/g, "").toLowerCase()); return i < 0 ? LAYERS.length : i; };

const DEFAULT_TINT = { r: 0.75, g: 0.75, b: 0.75 };

// build makes a THREE.Group for a character ({female, skin, hair, hairColor, beard, chestHair, clothing, clothingTint}).
export async function build(c) {
  const cat = S.cat;
  const group = new THREE.Group();
  if (!cat || !c) return group;
  const looks = cat.looks;
  const female = !!c.female;
  const skins = looks.skinColors || [];
  let skinIdx = skins.findIndex((s) => Math.abs(s.r - (c.skin || {}).r) < 0.02 && Math.abs(s.g - (c.skin || {}).g) < 0.02);
  if (skinIdx < 0) skinIdx = 1;
  const bodyTex = `Body/${female ? "Female" : "Male"}Body0${skinIdx + 1}${!female && c.chestHair ? "a" : ""}`;

  const items = Object.entries(c.clothing || {}).filter(([, id]) => id).map(([loc, id]) => ({ loc, id, look: cat.clothing[id] }))
    .sort((a, b) => layerOf(a.loc) - layerOf(b.loc));
  // body texture with "painted on" clothes (t-shirts, socks, shoes), put together by the launcher
  const q = new URLSearchParams({ t: TOKEN, base: bodyTex });
  for (const it of items) {
    if (!it.look || !it.look.baseTextures) continue;
    const tex = hashPick(it.look.baseTextures, it.id);
    const tint = it.look.tint || it.look.hue ? (c.clothingTint || {})[it.loc] || DEFAULT_TINT : null;
    q.append("l", tex + (tint ? `|${tint.r.toFixed(3)},${tint.g.toFixed(3)},${tint.b.toFixed(3)}` : ""));
  }
  // clothes with a 3D model hide the skin under them (the game's body masks)
  for (const it of items) {
    const look = it.look;
    if (!look || !(look.maleModel || look.femaleModel)) continue;
    for (const m of look.masks || []) q.append("m", (look.masksFolder || "") + "|" + m);
  }
  const bodyMap = await loadBodyTexture("/bodytex?" + q.toString());
  const bodyGeo = await loadMesh(`skinned/${female ? "Female" : "Male"}Body`);
  const rig = bodyGeo && bodyGeo.userData.frames ? makeSkeleton(bodyGeo.userData.frames) : null;
  if (rig) for (const b of rig.roots) group.add(b);
  group.userData.rig = rig;
  if (bodyGeo) {
    const mat = new THREE.MeshStandardMaterial({ map: bodyMap, roughness: 0.85, metalness: 0, side: THREE.DoubleSide, alphaTest: bodyMap ? 0.5 : 0, color: bodyMap ? 0xffffff : 0xd8b48c });
    group.add(skinnedMesh(bodyGeo, mat, rig));
  }

  const addModel = async (model, texPath, tint, alpha, offset, bone) => {
    const [geo, img] = await Promise.all([loadMesh(model), loadImage(texPath)]);
    if (!geo) return;
    const mat = new THREE.MeshStandardMaterial({ roughness: 0.9, metalness: 0, side: THREE.DoubleSide, alphaTest: alpha ? 0.4 : 0, transparent: false });
    if (img) mat.map = canvasTexture(tinted(img, tint));
    else if (tint) mat.color = new THREE.Color(tint.r, tint.g, tint.b);
    mat.polygonOffset = true;
    mat.polygonOffsetFactor = -offset * 0.5;
    mat.polygonOffsetUnits = -offset * 2;
    if (bone) {
      // static models (hats, glasses) are modelled in the bone's own space
      const b = rig && rig.bones[bone];
      if (b) b.add(new THREE.Mesh(geo, mat));
      return;
    }
    group.add(skinnedMesh(geo, mat, rig));
  };

  const jobs = [];
  // depth nudge so touching layers don't flicker: hair just over skin, then clothes inner to outer,
  // hats and glasses last
  let layer = 2;
  for (const it of items) {
    const look = it.look;
    if (!look) continue;
    const model = female ? look.femaleModel || look.maleModel : look.maleModel || look.femaleModel;
    if (!model) continue;
    const isStatic = /(^|[\\/])static[\\/]/i.test(model);
    if (isStatic && !(look.bone && rig)) continue;
    const tex = hashPick(look.textures, it.id);
    const tint = look.tint || look.hue ? (c.clothingTint || {})[it.loc] || DEFAULT_TINT : null;
    jobs.push(addModel(model, tex, tint, true, isStatic ? 12 : layer++, isStatic ? look.bone : null));
  }
  const hairList = (female ? looks.hairFemale : looks.hairMale) || [];
  // a hat swaps the hair for the game's under-the-hat version, or hides it
  const hatCat = (items.find((it) => it.look && it.look.hatCategory) || {}).look?.hatCategory || "";
  let hair = hairList.find((x) => x.id === c.hair);
  if (hair && hatCat) {
    if (hatCat === "nohair" || hatCat === "nohairnobeard") hair = null;
    else if (hair.alt && hair.alt[hatCat]) hair = hairList.find((x) => x.id.toLowerCase() === hair.alt[hatCat].toLowerCase()) || hair;
  }
  if (hair && hair.model) jobs.push(addModel(hair.model, hair.texture || "F_Hair_White", c.hairColor, true, 1));
  if (!female && c.beard && hatCat !== "nobeard" && hatCat !== "nohairnobeard") {
    const beard = (looks.beards || []).find((x) => x.id === c.beard);
    if (beard && beard.model) jobs.push(addModel(beard.model, beard.texture || "F_Hair_White", c.hairColor, true, 1));
  }
  await Promise.all(jobs);
  return group;
}

export function sizeHolder(holder, c) {
  const ex = (c && c.extras) || {};
  const h = (ex["vsm.height"] || 175) / 175;
  const w = 1 + (ex["vsm.shape"] || 0) * 0.0018 + (ex["vsm.muscle"] || 0) * 0.0008;
  holder.scale.set(w, h, w);
}

// CharacterView draws one character you can spin with the mouse.
export function CharacterView({ c, height = 360, small, portrait, controls }) {
  const box = useRef(null);
  const st = useRef(null);
  const key = c ? JSON.stringify([c.female, c.skin, c.hair, c.hairColor, c.beard, c.chestHair, c.clothing, c.clothingTint]) : "";
  const ex = (c && c.extras) || {};

  useEffect(() => {
    const el = box.current;
    const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
    renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    renderer.outputColorSpace = THREE.SRGBColorSpace;
    renderer.toneMapping = THREE.ACESFilmicToneMapping;
    el.appendChild(renderer.domElement);
    const scene = new THREE.Scene();
    // portrait: a whole standing figure in a tall card, page scroll left alone
    const camera = new THREE.PerspectiveCamera(portrait ? 31 : small ? 26 : 30, 1, 0.01, 20);
    camera.position.set(portrait ? 0.35 : 0.45, portrait ? 0.7 : 0.62, portrait ? 2.3 : small ? 2.1 : 2.4);
    scene.add(new THREE.HemisphereLight(0xfff1e0, 0x2a2622, 1.6));
    const key = new THREE.DirectionalLight(0xffe2c4, 2.2);
    key.position.set(1.5, 2, 2);
    scene.add(key);
    const rim = new THREE.DirectionalLight(0xff9a5a, 1.2);
    rim.position.set(-2, 1.5, -2);
    scene.add(rim);
    const floor = new THREE.Mesh(new THREE.CircleGeometry(0.42, 48), new THREE.MeshBasicMaterial({ color: 0x000000, transparent: true, opacity: 0.35 }));
    floor.rotation.x = -Math.PI / 2;
    scene.add(floor);
    const holder = new THREE.Group();
    holder.rotation.y = Math.PI; // the models face -Z
    scene.add(holder);
    const controls = new OrbitControls(camera, renderer.domElement);
    controls.target.set(0, portrait ? 0.56 : small ? 0.52 : 0.5, 0);
    controls.enablePan = false;
    if (portrait) controls.enableZoom = false;
    controls.enableDamping = true;
    controls.minDistance = 0.7;
    controls.maxDistance = 3.5;
    controls.minPolarAngle = 0.3;
    controls.maxPolarAngle = Math.PI / 2;
    controls.autoRotate = true;
    controls.autoRotateSpeed = portrait ? 0.8 : 1.2;
    controls.addEventListener("start", () => (controls.autoRotate = false));
    controls.update();
    const resize = () => {
      const w = el.clientWidth || 300, hgt = el.clientHeight || height;
      renderer.setSize(w, hgt);
      camera.aspect = w / hgt;
      camera.updateProjectionMatrix();
    };
    const ro = new ResizeObserver(resize);
    ro.observe(el);
    resize();
    let raf = 0, alive = true, last = performance.now();
    const loop = () => {
      if (!alive) return;
      raf = requestAnimationFrame(loop);
      const now = performance.now(), dt = Math.min(0.1, (now - last) / 1000);
      last = now;
      if (!el.isConnected || document.hidden) return;
      if (st.current && st.current.anim) st.current.anim.update(dt);
      controls.update();
      renderer.render(scene, camera);
    };
    loop();
    st.current = { holder, renderer, anim: null, move: "idle" };
    return () => {
      alive = false;
      cancelAnimationFrame(raf);
      ro.disconnect();
      controls.dispose();
      renderer.dispose();
      renderer.forceContextLoss();
      el.innerHTML = "";
      st.current = null;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    build(c).then((g) => {
      const s = st.current;
      if (cancelled || !s) return;
      for (const old of [...s.holder.children]) {
        s.holder.remove(old);
        old.traverse((o) => { if (o.material) { if (o.material.map && !o.material.map.shared) o.material.map.dispose(); o.material.dispose(); } });
      }
      s.holder.add(g);
      sizeHolder(s.holder, c);
      if (g.userData.rig) {
        s.anim = new Animator(g);
        s.anim.play(s.move === "idle" || MOVES.find((m) => m.id === s.move && m.loop) ? s.move : "idle");
      } else s.anim = null;
    });
    return () => { cancelled = true; };
  }, [key, S.cat && S.cat.version]);

  // Size Matters: rough preview of height and build
  useEffect(() => { if (st.current) sizeHolder(st.current.holder, c); }, [ex["vsm.height"], ex["vsm.shape"], ex["vsm.muscle"]]);

  const [move, setMove] = useState("idle");
  const pick = (m) => {
    setMove(m.loop ? m.id : move);
    if (st.current) {
      if (m.loop) st.current.move = m.id;
      if (st.current.anim) st.current.anim.play(m);
    }
  };
  const view = html`<div class="viewer" ref=${box} style=${{ height: height + "px" }} title="Drag to turn"></div>`;
  if (!controls) return view;
  return html`<div class="viewer-wrap">${view}
    <div class="moves">${MOVES.map((m) => html`<button key=${m.id} class=${"move" + (m.loop ? " loop" : "") + (m.loop && move === m.id ? " on" : "")} onClick=${() => pick(m)}>${m.label}</button>`)}</div>
  </div>`;
}

// freezePose holds a character in one frame of a game animation (for the squad poster).
export async function freezePose(group, anim, at) {
  const clip = await loadClip(anim);
  if (!clip || !group.userData.rig) return;
  const mixer = new THREE.AnimationMixer(group);
  const a = mixer.clipAction(clip);
  a.play();
  mixer.setTime(Math.max(0, Math.min(clip.duration - 0.001, clip.duration * at)));
}
