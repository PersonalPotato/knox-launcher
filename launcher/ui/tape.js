// VHS tapes: cover, back of the box and spine, drawn on canvases. Also sequel titles and posed cast shots.
import * as THREE from "./vendor/three.module.js";
import { S } from "./core.js";
import { build, freezePose, sizeHolder } from "./viewer.js";
import { seeded } from "./knox.js";

export const W = 600, H = 1046, SPINE = 136; // a VHS sleeve: 135 x 235 x 30 mm

// Sequels ---------------------------------------------------------------------------------------------

const SUBS = ["DOOMSDAY", "THE RECKONING", "BLOOD MOON", "NO WAY OUT", "THE RETURN", "DEAD OF WINTER", "LAST LIGHT", "THE HUNGER", "AFTERSHOCK",
  "ROAD TO LOUISVILLE", "THE FINAL CHAPTER", "REVENGE", "THE AWAKENING", "NIGHTFALL", "BAD BLOOD", "NO ESCAPE", "THE SIEGE", "THE OUTBREAK",
  "JUDGMENT DAY", "FRESH MEAT", "BACK FROM THE DEAD", "THE LAST STAND", "BURN IT DOWN", "THE LONG WINTER", "ROT", "SCORCHED EARTH", "THE HORDE",
  "EXTINCTION", "HELLFIRE", "THE DARK", "BLACKOUT", "LOCKDOWN", "QUARANTINE", "FALLOUT", "THE CURE", "PATIENT ZERO", "BITE BACK", "DEAD END",
  "THE MALL", "SUMMER CAMP", "THE HOSPITAL", "THE PRISON", "HIGHWAY TO HELL", "THE BRIDGE", "GROUND ZERO", "THE STORM", "DEAD SILENCE",
  "NO MERCY", "BLOOD HARVEST", "THE FOG", "UNDERGROUND", "CABIN FEVER", "THE EXPERIMENT", "BRAIN DEAD", "SPRING BREAK", "THE BUNKER",
  "HOMECOMING", "THE FLOOD", "DEAD RECKONING", "MIDNIGHT", "THE EVACUATION", "GHOST TOWN", "FINAL DESTINATION KNOX", "BLOOD AND BEANS",
  "THE GENERATOR", "OUT OF GAS", "THE SPRINTERS", "SCREAM AGAIN", "RUN", "THE LAST CAN", "NO SLEEP", "THE WALL", "HOLD THE LINE", "THE CROSSING",
  "FIRE IN THE HOLE", "THE SCAVENGERS", "HEADSHOT", "THE NEIGHBOURS", "BURIED ALIVE", "THE CHURCH", "DEAD WEIGHT", "SHOTGUN WEDDING",
  "BLOODBATH", "GRAVEYARD SHIFT", "THE COLD", "THE INFECTED", "UNFINISHED BUSINESS", "THE BIG ONE", "TOTAL CARNAGE", "THE HELICOPTER",
  "NOWHERE TO HIDE", "THE TRUCK STOP", "BAD MOON", "THE SECOND WAVE", "HELL'S KITCHEN", "BLOOD TIES", "DAY ZERO", "FULL MOON", "THE ALARM",
  "THE DEAD ZONE", "TERMINAL", "THE BASEMENT", "BREAKOUT", "LAST CALL", "THE SURVIVORS", "THE REMATCH", "MAXIMUM CARNAGE", "BLOODLINES",
  "THE SEQUEL NOBODY ASKED FOR", "THIS TIME IT'S PERSONAL", "THE MOVIE", "ON ICE", "IN SPACE", "TOKYO DRIFT", "REVENGE OF THE BEANS",
  "THE FINAL FINAL CHAPTER", "THE PREQUEL", "REBOOT", "DIRECTOR'S REGRET", "STRAIGHT TO VIDEO", "BE KIND REWIND", "LATE FEES",
  "THE LOST TAPE", "UNRATED", "ROADKILL", "THE CARNIVAL", "DEAD MAN'S PARTY", "OVERTIME", "THE SUBURBS", "THE MOTEL", "DRIVE-IN",
  "THE RAPTURE", "SALVATION", "ONE LAST JOB", "THE WEEKEND", "HAUNTED", "THE CALLING", "DEADLINE", "THE COLLAPSE", "WHITEOUT",
  "THE QUIET", "RIOT", "THE SCHOOL", "THE FARM", "CORNFIELD", "SWAMP THING", "THE RIVER", "TRAIN TO MULDRAUGH", "THE AIRPORT"];

const ROMAN = [[1000, "M"], [900, "CM"], [500, "D"], [400, "CD"], [100, "C"], [90, "XC"], [50, "L"], [40, "XL"], [10, "X"], [9, "IX"], [5, "V"], [4, "IV"], [1, "I"]];
export function roman(n) {
  let s = "";
  for (const [v, r] of ROMAN) while (n >= v) { s += r; n -= v; }
  return s;
}

// sequel: part 1 is the original, part 2 is PART II, and each part has its own subtitle.
// A part is one play session of a world: closing or restarting the tape starts the next part.
export function sequel(name, part, seed) {
  part = part || 1;
  if (part < 2) return { title: name.toUpperCase(), part: "", sub: "" };
  const rnd = seeded(String(seed || name) + "#" + part);
  return { title: name.toUpperCase(), part: "PART " + roman(part), sub: SUBS[Math.floor(rnd() * SUBS.length)] };
}

export function sequelText(s) {
  return s.part ? `${s.title} ${s.part} - ${s.sub}` : s.title;
}

// Looks -----------------------------------------------------------------------------------------------

const TAGS = {
  horror: ["Some doors should stay shut.", "Nobody gets out of Knox County.", "They were only going for supplies.", "Day one was the easy part.", "Don't open the door."],
  slasher: ["Tonight, nobody is safe.", "Count the survivors. Then count again.", "They should have stayed home.", "Something is in the house."],
  action: ["No backup. No retreat. No beans.", "They didn't start this fight.", "Lock and load.", "Ten tons of trouble."],
  creature: ["It came from Knox County.", "Not even the dead rest here.", "Keep watching the treeline.", "Something is hungry."],
  tv: ["A Knox TV world premiere.", "Based on true events.", "Tonight at 9, 8 central.", "Viewer discretion advised."],
};

const MONTHS = ["JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"];
const MONTH_INK = ["#7fd3ff", "#ff7fb6", "#8be38b", "#ffd23f", "#ff9a3c", "#ffe066", "#ff4d4d", "#4dd2c1", "#c79bff", "#ff7a1a", "#d9a066", "#e8f4ff"];
export function collectorName(m) {
  const [y, mo] = (m || "").split("-");
  return mo ? `${MONTHS[+mo - 1]} ${y}` : "";
}
const FRAME_INK = { bronze: "#b0763a", silver: "#c9ced6", gold: "#e0b43a" };

// fonts must be loaded before a canvas can use them
let fontsReady = null;
export function fonts() {
  if (!fontsReady) {
    fontsReady = Promise.all(["700 40px Oswald", "500 40px Oswald", "400 30px 'Special Elite'", "700 30px 'Barlow Condensed'", "500 30px 'Barlow Condensed'",
      "400 30px Barlow", "700 30px Barlow", "400 30px 'Saira Stencil One'", "400 30px VT323", "400 30px 'Patrick Hand'"].map((f) => document.fonts.load(f).catch(() => null)));
  }
  return fontsReady;
}

const imgs = new Map();
export function image(src) {
  if (!src) return Promise.resolve(null);
  if (!imgs.has(src)) {
    imgs.set(src, new Promise((res) => { const i = new Image(); i.onload = () => res(i); i.onerror = () => res(null); i.src = src; }));
  }
  return imgs.get(src);
}

function canvas(w, h) {
  const c = document.createElement("canvas");
  c.width = w;
  c.height = h;
  return c;
}

function cover(ctx, img, x, y, w, h, ax = 0.5, ay = 0.4) {
  const s = Math.max(w / img.width, h / img.height);
  const iw = img.width * s, ih = img.height * s;
  ctx.drawImage(img, x + (w - iw) * ax, y + (h - ih) * ay, iw, ih);
}

// fitText shrinks a font until the text fits the width.
function fitFont(ctx, text, font, size, maxW, min = 14) {
  let s = size;
  do { ctx.font = font.replace("%", s); s -= 2; } while (ctx.measureText(text).width > maxW && s > min);
  return s + 2;
}

// wrap splits words over lines no wider than maxW
function wrap(ctx, text, maxW) {
  const words = String(text).split(/\s+/);
  const lines = [];
  let line = "";
  for (const w of words) {
    const t = line ? line + " " + w : w;
    if (ctx.measureText(t).width > maxW && line) { lines.push(line); line = w; } else line = t;
  }
  if (line) lines.push(line);
  return lines;
}

// Backdrops: drawn shapes, a night or a fire behind the cast.
function sky(ctx, w, h, stops) {
  const g = ctx.createLinearGradient(0, 0, 0, h);
  stops.forEach(([at, c]) => g.addColorStop(at, c));
  ctx.fillStyle = g;
  ctx.fillRect(0, 0, w, h);
}
function moon(ctx, x, y, r, c = "rgba(240,235,220,.9)") {
  ctx.fillStyle = c;
  ctx.beginPath(); ctx.arc(x, y, r, 0, Math.PI * 2); ctx.fill();
}
function trees(ctx, w, base, rnd, c, n = 26, hmax = 180) {
  ctx.fillStyle = c;
  for (let i = 0; i < n; i++) {
    const x = rnd() * w, th = 60 + rnd() * hmax, tw = 26 + rnd() * 40;
    ctx.beginPath(); ctx.moveTo(x, base - th); ctx.lineTo(x - tw, base); ctx.lineTo(x + tw, base); ctx.fill();
  }
  ctx.fillRect(0, base - 4, w, 999);
}

export async function drawBackdrop(ctx, id, w, h, seed) {
  const rnd = seeded(seed + "bd" + id);
  if (id === "tower") {
    sky(ctx, w, h, [[0, "#05060c"], [0.6, "#14182a"], [1, "#05050a"]]);
    moon(ctx, w * 0.72, h * 0.18, 46);
    trees(ctx, w, h * 0.72, rnd, "#030305");
    ctx.fillStyle = "#020203";
    const x = w * 0.3, y = h * 0.2;
    ctx.fillRect(x - 70, y, 140, 70);
    ctx.beginPath(); ctx.ellipse(x, y, 70, 22, 0, Math.PI, 0); ctx.fill();
    ctx.lineWidth = 7; ctx.strokeStyle = "#020203";
    for (const dx of [-55, -20, 20, 55]) { ctx.beginPath(); ctx.moveTo(x + dx, y + 70); ctx.lineTo(x + dx * 1.4, h * 0.72); ctx.stroke(); }
    ctx.lineWidth = 3;
    for (let i = 1; i < 4; i++) { const yy = y + 70 + i * (h * 0.72 - y - 70) / 4; ctx.beginPath(); ctx.moveTo(x - 60 - i * 6, yy); ctx.lineTo(x + 60 + i * 6, yy); ctx.stroke(); }
  } else if (id === "city") {
    sky(ctx, w, h, [[0, "#120404"], [0.45, "#5a1406"], [0.68, "#d0480f"], [0.75, "#2a0a04"], [1, "#050202"]]);
    for (let i = 0; i < 9; i++) { // smoke
      ctx.fillStyle = `rgba(20,8,6,${0.25 + rnd() * 0.3})`;
      ctx.beginPath(); ctx.ellipse(rnd() * w, h * (0.15 + rnd() * 0.35), 120 + rnd() * 120, 50 + rnd() * 40, 0, 0, Math.PI * 2); ctx.fill();
    }
    ctx.fillStyle = "#070303";
    let x = -10;
    while (x < w) { const bw = 30 + rnd() * 60, bh = 120 + rnd() * 260; ctx.fillRect(x, h * 0.74 - bh, bw, bh + 400); x += bw + 2; }
    ctx.fillStyle = "rgba(255,170,60,.55)";
    for (let i = 0; i < 40; i++) ctx.fillRect(rnd() * w, h * 0.45 + rnd() * h * 0.28, 4, 6);
  } else if (id === "mall") {
    sky(ctx, w, h, [[0, "#06070b"], [0.7, "#1a1d26"], [1, "#07070a"]]);
    ctx.fillStyle = "#0b0b0e"; ctx.fillRect(0, h * 0.4, w, h * 0.35);
    ctx.fillStyle = "#111117"; ctx.fillRect(w * 0.15, h * 0.32, w * 0.7, h * 0.1);
    ctx.font = "700 46px Oswald"; ctx.textAlign = "center"; ctx.fillStyle = "rgba(255,90,70,.75)";
    ctx.fillText("MALL", w / 2, h * 0.395);
    ctx.fillStyle = "rgba(210,220,255,.18)";
    for (let i = 0; i < 6; i++) ctx.fillRect(w * 0.08 + i * w * 0.15, h * 0.5, w * 0.1, h * 0.14);
    ctx.fillStyle = "#050507"; ctx.fillRect(0, h * 0.74, w, h);
    for (const lx of [w * 0.12, w * 0.88]) { ctx.fillRect(lx - 3, h * 0.45, 6, h * 0.3); moon(ctx, lx, h * 0.45, 10, "rgba(255,230,170,.8)"); }
  } else if (id === "farm") {
    sky(ctx, w, h, [[0, "#1a0f2a"], [0.4, "#6a2a3a"], [0.62, "#e07a3a"], [0.7, "#2a1208"], [1, "#0a0503"]]);
    moon(ctx, w * 0.5, h * 0.62, 70, "rgba(255,190,110,.75)");
    ctx.fillStyle = "#0a0503";
    ctx.fillRect(0, h * 0.68, w, h);
    const bx = w * 0.62, by = h * 0.68;
    ctx.beginPath(); ctx.moveTo(bx - 110, by); ctx.lineTo(bx - 110, by - 140); ctx.lineTo(bx - 60, by - 200); ctx.lineTo(bx + 60, by - 200); ctx.lineTo(bx + 110, by - 140); ctx.lineTo(bx + 110, by); ctx.fill();
    ctx.fillRect(w * 0.2, by - 260, 60, 260);
    ctx.beginPath(); ctx.ellipse(w * 0.2 + 30, by - 260, 30, 18, 0, Math.PI, 0); ctx.fill();
    ctx.lineWidth = 4; ctx.strokeStyle = "#0a0503";
    for (let i = 0; i < 12; i++) { ctx.beginPath(); ctx.moveTo(i * 55, by + 20); ctx.lineTo(i * 55, by - 30); ctx.stroke(); }
    ctx.beginPath(); ctx.moveTo(0, by - 20); ctx.lineTo(w, by - 20); ctx.stroke();
  } else if (id === "gas") {
    sky(ctx, w, h, [[0, "#04050a"], [0.7, "#0e1220"], [1, "#040406"]]);
    ctx.fillStyle = "#08080a"; ctx.fillRect(0, h * 0.72, w, h);
    ctx.fillStyle = "#16161a"; ctx.fillRect(w * 0.05, h * 0.3, w * 0.9, 34);
    ctx.fillStyle = "rgba(255,240,200,.85)";
    for (let i = 0; i < 5; i++) ctx.fillRect(w * 0.12 + i * w * 0.17, h * 0.3 + 34, 50, 6);
    const g = ctx.createLinearGradient(0, h * 0.33, 0, h * 0.72);
    g.addColorStop(0, "rgba(255,240,200,.18)"); g.addColorStop(1, "rgba(255,240,200,0)");
    ctx.fillStyle = g; ctx.fillRect(w * 0.05, h * 0.33, w * 0.9, h * 0.4);
    ctx.fillStyle = "#0a0a0c";
    for (const px of [w * 0.15, w * 0.85]) ctx.fillRect(px - 8, h * 0.33, 16, h * 0.4);
    ctx.fillStyle = "#121216"; ctx.fillRect(w * 0.3, h * 0.6, 40, 80); ctx.fillRect(w * 0.62, h * 0.6, 40, 80);
  } else if (id === "woods") {
    sky(ctx, w, h, [[0, "#020404"], [0.6, "#0c1512"], [1, "#020303"]]);
    moon(ctx, w * 0.25, h * 0.15, 30, "rgba(220,230,220,.7)");
    trees(ctx, w, h * 0.78, rnd, "#060a08", 22, 420);
    trees(ctx, w, h * 0.85, rnd, "#020302", 16, 520);
    const f = ctx.createLinearGradient(0, h * 0.55, 0, h * 0.85);
    f.addColorStop(0, "rgba(160,180,170,0)"); f.addColorStop(1, "rgba(160,180,170,.18)");
    ctx.fillStyle = f; ctx.fillRect(0, 0, w, h);
  } else {
    const art = await image("/art");
    sky(ctx, w, h, [[0, "#0a0606"], [1, "#050303"]]);
    if (art) cover(ctx, art, 0, 0, w, h, 0.62, 0.4);
  }
}

// grade colours the whole picture the genre's way (print, not effects)
function grade(ctx, w, h, genre) {
  const tint = { horror: "#a81c1c", slasher: "#6a7aa0", action: "#e0884a", creature: "#5e9a52", tv: "#6a84b8" }[genre] || "#a81c1c";
  ctx.save();
  ctx.globalCompositeOperation = "saturation"; ctx.fillStyle = "rgba(128,128,128,.55)"; ctx.fillRect(0, 0, w, h);
  ctx.globalCompositeOperation = "multiply"; ctx.fillStyle = tint; ctx.fillRect(0, 0, w, h);
  ctx.restore();
}

function vignette(ctx, w, h, a = 0.85) {
  const g = ctx.createRadialGradient(w / 2, h * 0.5, h * 0.2, w / 2, h * 0.5, h * 0.75);
  g.addColorStop(0, "rgba(0,0,0,0)"); g.addColorStop(1, `rgba(0,0,0,${a})`);
  ctx.fillStyle = g; ctx.fillRect(0, 0, w, h);
}

function spaced(ctx, text, x, y, sp, align = "center") {
  const chars = [...text];
  const widths = chars.map((c) => ctx.measureText(c).width);
  const total = widths.reduce((a, b) => a + b, 0) + sp * (chars.length - 1);
  let cx = align === "center" ? x - total / 2 : align === "right" ? x - total : x;
  const prev = ctx.textAlign;
  ctx.textAlign = "left";
  chars.forEach((c, i) => { ctx.fillText(c, cx, y); cx += widths[i] + sp; });
  ctx.textAlign = prev;
}

// worn: a sun-faded, scuffed case
function wear(ctx, w, h, seed) {
  const rnd = seeded(seed + "worn");
  ctx.save();
  ctx.globalCompositeOperation = "saturation"; ctx.fillStyle = "rgba(128,128,128,.3)"; ctx.fillRect(0, 0, w, h);
  ctx.globalCompositeOperation = "screen"; ctx.fillStyle = "rgba(60,50,30,.18)"; ctx.fillRect(0, 0, w, h);
  ctx.globalCompositeOperation = "source-over";
  ctx.strokeStyle = "rgba(255,255,255,.18)";
  for (let i = 0; i < 26; i++) {
    ctx.lineWidth = 0.5 + rnd() * 1.2;
    const x = rnd() * w, y = rnd() * h, l = 20 + rnd() * 140, a = rnd() * Math.PI;
    ctx.beginPath(); ctx.moveTo(x, y); ctx.lineTo(x + Math.cos(a) * l, y + Math.sin(a) * l); ctx.stroke();
  }
  ctx.fillStyle = "rgba(235,225,200,.35)";
  for (const [cx, cy] of [[0, 0], [w, 0], [0, h], [w, h]]) {
    for (let i = 0; i < 6; i++) { ctx.beginPath(); ctx.arc(cx + (rnd() - 0.5) * 30, cy + (rnd() - 0.5) * 30, 3 + rnd() * 9, 0, Math.PI * 2); ctx.fill(); }
  }
  ctx.fillStyle = "rgba(240,235,215,.5)"; // sticker residue
  ctx.fillRect(w * (0.55 + rnd() * 0.2), h * (0.03 + rnd() * 0.05), 70, 26);
  ctx.restore();
}

function frameBorder(ctx, w, h, frame, collectorInk) {
  const ink = FRAME_INK[frame] || collectorInk;
  if (!ink) return;
  ctx.strokeStyle = ink;
  ctx.lineWidth = 14; ctx.strokeRect(7, 7, w - 14, h - 14);
  ctx.lineWidth = 2; ctx.strokeStyle = "rgba(0,0,0,.6)"; ctx.strokeRect(17, 17, w - 34, h - 34);
}

function ribbon(ctx, w, text) {
  ctx.save();
  ctx.translate(w - 92, 92);
  ctx.rotate(Math.PI / 4);
  ctx.fillStyle = "#0a0a0a"; ctx.fillRect(-150, -22, 300, 44);
  ctx.strokeStyle = "#e0b43a"; ctx.lineWidth = 2; ctx.strokeRect(-150, -18, 300, 36);
  ctx.fillStyle = "#e0b43a"; ctx.font = "700 20px Oswald"; ctx.textAlign = "center"; ctx.textBaseline = "middle";
  spaced(ctx, text, 0, 1, 3);
  ctx.restore();
}

function stickerBox(ctx, x, y, lines, bg, fg, rot = -0.08) {
  ctx.save();
  ctx.translate(x, y); ctx.rotate(rot);
  ctx.font = "700 20px Oswald";
  const wMax = Math.max(...lines.map((l) => ctx.measureText(l).width)) + 28;
  const hh = lines.length * 24 + 14;
  ctx.fillStyle = bg; ctx.fillRect(-wMax / 2, -hh / 2, wMax, hh);
  ctx.strokeStyle = fg; ctx.lineWidth = 2; ctx.strokeRect(-wMax / 2 + 4, -hh / 2 + 4, wMax - 8, hh - 8);
  ctx.fillStyle = fg; ctx.textAlign = "center"; ctx.textBaseline = "middle";
  lines.forEach((l, i) => ctx.fillText(l, 0, -hh / 2 + 19 + i * 24));
  ctx.restore();
}

// drawTitle draws the title block; called twice for a foil print (once in ink, once as the foil mask)
function titleBlock(ctx, o, style, ink, mask) {
  const { genre } = o;
  const T = o.title || "UNTITLED";
  ctx.textAlign = "center";
  ctx.textBaseline = "alphabetic";
  const setInk = (c) => { ctx.fillStyle = mask ? "#fff" : c; };
  if (genre === "action") {
    ctx.save();
    ctx.translate(W / 2, style.y);
    ctx.transform(1, 0, -0.18, 1, 0, 0);
    const lines = wrapTitle(ctx, T, "700 %px 'Barlow Condensed'", 112, W - 70, 2);
    lines.forEach((l, i) => {
      ctx.font = `700 ${lines.size}px 'Barlow Condensed'`;
      const y = i * lines.size * 0.86;
      if (!mask) { ctx.fillStyle = "#7a0f0a"; ctx.fillText(l, 6, y + 6); ctx.lineWidth = 6; ctx.strokeStyle = "#140404"; ctx.strokeText(l, 0, y); }
      setInk(ink); ctx.fillText(l, 0, y);
    });
    ctx.restore();
    return lines.length * lines.size * 0.86;
  }
  const font = genre === "creature" ? "400 %px 'Saira Stencil One'" : genre === "tv" ? "700 %px Barlow" : "700 %px Oswald";
  const lines = wrapTitle(ctx, T, font, genre === "tv" ? 64 : 92, W - 80, 3);
  lines.forEach((l, i) => {
    ctx.font = font.replace("%", lines.size);
    const y = style.y + i * lines.size * 0.92;
    if (!mask && genre === "creature") { ctx.lineWidth = 8; ctx.strokeStyle = "#041004"; ctx.strokeText(l, W / 2, y); }
    if (!mask && genre === "slasher") { ctx.fillStyle = "#000"; ctx.fillText(l, W / 2 + 4, y + 4); }
    setInk(ink); ctx.fillText(l, W / 2, y);
  });
  return lines.length * lines.size * 0.92;
}

function wrapTitle(ctx, text, font, size, maxW, maxLines) {
  let s = size, lines;
  for (;;) {
    ctx.font = font.replace("%", s);
    lines = wrap(ctx, text, maxW);
    if ((lines.length <= maxLines && lines.every((l) => ctx.measureText(l).width <= maxW)) || s <= 30) break;
    s -= 4;
  }
  lines.size = s;
  return lines;
}

function partLine(ctx, o, y, ink, mask) {
  if (!o.part) return 0;
  ctx.textAlign = "center";
  ctx.font = "700 30px Oswald";
  ctx.fillStyle = mask ? "#fff" : ink;
  spaced(ctx, o.part, W / 2, y, 6);
  fitFont(ctx, o.sub, "400 %px 'Special Elite'", 30, W - 80);
  ctx.fillText(o.sub, W / 2, y + 36);
  return 50;
}

// drawCover makes the front: { title, part, sub, genre, backdrop, fig, photo, cast, stats, foil, worn, frame, cut, collector, seed, kicker }
export async function drawCover(o) {
  await fonts();
  const c = canvas(W, H), x = c.getContext("2d");
  const m = o.foil ? canvas(W, H) : null, mx = m && m.getContext("2d");
  if (mx) { mx.fillStyle = "#000"; mx.fillRect(0, 0, W, H); }
  const genre = o.genre || "horror";
  const seed = o.seed || o.title;
  const rnd = seeded(seed + "tag");
  const tag = o.tagline || TAGS[genre][Math.floor(rnd() * TAGS[genre].length)];
  const kicker = o.kicker || "KNOX LAUNCHER PRESENTS";
  const collectorInk = o.collector ? MONTH_INK[+(o.collector.split("-")[1] || 1) - 1] : null;
  x.fillStyle = "#050303"; x.fillRect(0, 0, W, H);

  // the picture
  const pic = canvas(W, H), px = pic.getContext("2d");
  await drawBackdrop(px, o.backdrop || "title", W, H, seed);
  if (o.photo) { px.globalAlpha = 0.9; cover(px, o.photo, 0, H * 0.18, W, H * 0.5); px.globalAlpha = 1; }
  grade(px, W, H, genre);
  if (genre === "action") { // sunset behind everything
    const g = px.createLinearGradient(0, 0, 0, H);
    g.addColorStop(0, "rgba(40,10,60,.6)"); g.addColorStop(0.5, "rgba(255,120,30,.35)"); g.addColorStop(1, "rgba(20,5,5,.8)");
    px.fillStyle = g; px.fillRect(0, 0, W, H);
  }
  vignette(px, W, H, genre === "tv" ? 0.5 : 0.85);

  const ink = { horror: "#ff3a2a", slasher: "#f4efe6", action: "#ffd23a", creature: "#9be04a", tv: "#ffffff" }[genre];
  const figBox = { horror: [0, 250, W, 560], slasher: [0, 120, W, 640], action: [0, 170, W, 560], creature: [0, 270, W, 540], tv: [70, 200, W - 140, 470] }[genre];

  if (genre === "tv") {
    x.fillStyle = "#0d1630"; x.fillRect(0, 0, W, H);
    x.fillStyle = "#1d3a8a"; x.fillRect(0, 0, W, 120);
    x.fillStyle = "#fff"; x.font = "700 38px Barlow"; x.textAlign = "left"; x.fillText("SUNDAY NIGHT MOVIE", 34, 76);
    x.strokeStyle = "#fff"; x.lineWidth = 3; x.strokeRect(W - 150, 36, 116, 50);
    x.font = "700 26px Barlow"; x.textAlign = "center"; x.fillText("KNOX TV", W - 92, 70);
    const [fx, fy, fw, fh] = figBox;
    x.save(); x.beginPath(); x.rect(fx, fy, fw, fh); x.clip(); x.drawImage(pic, 0, 0); if (o.fig) cover(x, o.fig, fx, fy, fw, fh, 0.5, 0.35); x.restore();
    x.strokeStyle = "#fff"; x.lineWidth = 8; x.strokeRect(fx, fy, fw, fh);
    x.fillStyle = "#c9d4f0"; x.font = "400 24px 'Special Elite'"; x.fillText(`"${tag}"`, W / 2, 168);
  } else {
    x.drawImage(pic, 0, 0);
    if (o.fig) { const [fx, fy, fw, fh] = figBox; cover(x, o.fig, fx, fy, fw, fh, 0.5, 0.35); }
    // darken where the text goes
    const g = x.createLinearGradient(0, 0, 0, H);
    g.addColorStop(0, "rgba(0,0,0,.75)"); g.addColorStop(0.22, "rgba(0,0,0,0)"); g.addColorStop(0.68, "rgba(0,0,0,0)"); g.addColorStop(0.82, "rgba(0,0,0,.85)"); g.addColorStop(1, "rgba(0,0,0,.95)");
    x.fillStyle = g; x.fillRect(0, 0, W, H);
  }

  // kicker + VHS box
  x.fillStyle = genre === "tv" ? "#c9d4f0" : "#d8c8b8";
  x.font = "500 17px Oswald"; x.textAlign = "left";
  if (genre !== "tv") {
    spaced(x, kicker, 34, 46, 6, "left");
    x.strokeStyle = "#d8c8b8"; x.lineWidth = 2; x.strokeRect(W - 86, 26, 54, 28);
    x.textAlign = "center"; x.font = "600 17px Oswald"; x.fillText("VHS", W - 59, 47);
  }

  // title + tagline + cast, per genre
  const cast = (o.cast || []).slice(0, 4);
  const drawTitleAt = (y) => {
    const st = { y };
    const h = titleBlock(x, { ...o, genre }, st, ink, false);
    if (mx) titleBlock(mx, { ...o, genre }, st, ink, true);
    return h;
  };
  const castRow = (y, color, sub) => {
    if (!cast.length) return;
    const cw = (W - 60) / cast.length;
    cast.forEach((p, i) => {
      const cx = 30 + cw * (i + 0.5);
      x.textAlign = "center"; x.fillStyle = color;
      fitFont(x, p.name.toUpperCase(), "700 %px Oswald", 24, cw - 10); x.fillText(p.name.toUpperCase(), cx, y);
      if (p.nick) { x.fillStyle = sub; fitFont(x, `"${p.nick}"`, "400 %px 'Special Elite'", 19, cw - 6); x.fillText(`"${p.nick}"`, cx, y + 23); }
      if (p.player) { x.fillStyle = "#a8998c"; x.font = "400 14px Barlow"; x.fillText(p.player, cx, y + (p.nick ? 43 : 22)); }
    });
  };
  const statsStrip = (stats, line) => {
    if (!stats || !stats.length) return;
    const y0 = H - 92;
    x.fillStyle = "rgba(0,0,0,.6)"; x.fillRect(0, y0, W, 92);
    x.fillStyle = line; x.fillRect(0, y0, W, 3);
    const cw = W / stats.length;
    stats.forEach(([k, v], i) => {
      x.textAlign = "center"; x.fillStyle = "#f3ece4"; x.font = "700 32px Oswald"; x.fillText(String(v), cw * (i + 0.5), y0 + 44);
      x.fillStyle = "#a8998c"; x.font = "500 12px Oswald"; spaced(x, k.toUpperCase(), cw * (i + 0.5), y0 + 68, 2);
    });
  };

  if (genre === "horror") {
    const th = drawTitleAt(150);
    if (o.part) { partLine(x, o, 150 + th + 4, "#ff6a4a", false); if (mx) partLine(mx, o, 150 + th + 4, "#fff", true); }
    castRow(H - 210, "#f3ece4", "#ff6a4a");
    statsStrip(o.stats, "#ff3a2a");
  } else if (genre === "slasher") {
    x.save(); x.translate(W / 2, H - 330); x.rotate(-0.09); x.fillStyle = "#c4140c"; x.fillRect(-W, -60, W * 2, 120); x.restore();
    x.fillStyle = "#ff4a3a"; x.font = "400 24px 'Special Elite'"; x.textAlign = "center"; x.fillText(tag, W / 2, 100);
    const th = drawTitleAt(H - 345);
    if (o.part) { partLine(x, o, H - 330 + th - 50, "#f4efe6", false); if (mx) partLine(mx, o, H - 330 + th - 50, "#fff", true); }
    x.fillStyle = "#f4efe6"; x.font = "500 16px Oswald";
    spaced(x, cast.map((p) => p.name.toUpperCase()).join("   "), W / 2, H - 120, 3);
    if (o.stats) { x.fillStyle = "#9a8f88"; x.font = "500 14px Oswald"; spaced(x, o.stats.map(([k, v]) => `${v} ${k}`).join("  ·  ").toUpperCase(), W / 2, H - 70, 2); }
  } else if (genre === "action") {
    x.fillStyle = "#fff"; x.font = "700 24px 'Barlow Condensed'"; x.textAlign = "center";
    spaced(x, cast.map((p) => p.name.toUpperCase()).join("  ·  "), W / 2, 110, 2);
    const th = drawTitleAt(H - 300);
    if (o.part) { partLine(x, o, H - 300 + th - 40, "#ffffff", false); if (mx) partLine(mx, o, H - 300 + th - 40, "#fff", true); }
    x.fillStyle = "#ffd23a"; x.font = "400 22px 'Special Elite'"; x.fillText(tag, W / 2, H - 70);
  } else if (genre === "creature") {
    x.fillStyle = "#cfe8b8"; x.font = "400 22px 'Special Elite'"; x.textAlign = "center"; x.fillText(tag.toUpperCase(), W / 2, 112);
    const th = drawTitleAt(200);
    if (o.part) { partLine(x, o, 200 + th - 20, "#cfe8b8", false); if (mx) partLine(mx, o, 200 + th - 20, "#fff", true); }
    castRow(H - 210, "#e8f2dc", "#9be04a");
    statsStrip(o.stats, "#9be04a");
  } else if (genre === "tv") {
    x.fillStyle = "#0d1630"; x.fillRect(0, 690, W, H - 690);
    const th = drawTitleAt(760);
    if (o.part) { partLine(x, o, 760 + th - 30, "#c9d4f0", false); if (mx) partLine(mx, o, 760 + th - 30, "#fff", true); }
    x.textAlign = "center"; x.fillStyle = "#c9d4f0"; x.font = "400 20px Barlow";
    const starring = cast.length ? "Starring " + cast.map((p) => p.name).join(", ") : "";
    wrap(x, starring, W - 80).slice(0, 2).forEach((l, i) => x.fillText(l, W / 2, H - 120 + i * 26));
    stickerBox(x, W - 110, H - 40, ["AS SEEN", "ON TV"], "#ffd23a", "#111", -0.1);
  }

  // stickers and labels
  if (o.collector) stickerBox(x, 110, H - (genre === "horror" || genre === "creature" ? 130 : 160), ["COLLECTOR'S EDITION", collectorName(o.collector)], "#0a0a0a", collectorInk, 0.06);
  if (o.cut) ribbon(x, W, o.cut > 1 ? `DIRECTOR'S CUT ${roman(o.cut)}` : "DIRECTOR'S CUT");
  if (o.worn) wear(x, W, H, seed);
  frameBorder(x, W, H, o.frame, collectorInk);
  return { canvas: c, mask: m };
}

// drawBack: the back of the box. { title, part, sub, genre, blurb, stats, cast, photos, player, worn, seed, rental }
export async function drawBack(o) {
  await fonts();
  const c = canvas(W, H), x = c.getContext("2d");
  const genre = o.genre || "horror";
  const ink = { horror: "#ff3a2a", slasher: "#e8e2d8", action: "#ffd23a", creature: "#9be04a", tv: "#9fb4ff" }[genre];
  x.fillStyle = genre === "tv" ? "#0d1630" : "#0a0707"; x.fillRect(0, 0, W, H);
  x.textAlign = "center";
  x.fillStyle = ink;
  fitFont(x, o.title, "700 %px Oswald", 52, W - 60); x.fillText(o.title, W / 2, 74);
  let y = 74;
  if (o.part) { x.font = "500 22px Oswald"; spaced(x, `${o.part} - ${o.sub}`, W / 2, y + 36, 3); y += 36; }
  // two "screenshots"
  const shots = (o.photos || []).filter(Boolean);
  const sy = y + 30, sw = (W - 90) / 2, sh = 170;
  for (let i = 0; i < 2; i++) {
    const sx = 30 + i * (sw + 30);
    x.fillStyle = "#1a1414"; x.fillRect(sx, sy, sw, sh);
    const img = shots[i % Math.max(1, shots.length)];
    if (img) { x.save(); x.beginPath(); x.rect(sx, sy, sw, sh); x.clip(); cover(x, img, sx, sy, sw, sh, i ? 0.8 : 0.3, 0.5); grade(x, W, H, genre); x.restore(); }
    x.strokeStyle = "#e8e2d8"; x.lineWidth = 4; x.strokeRect(sx, sy, sw, sh);
  }
  y = sy + sh + 50;
  // the blurb
  x.textAlign = "left"; x.fillStyle = "#e8e2d8"; x.font = "400 21px 'Special Elite'";
  for (const l of wrap(x, o.blurb || "", W - 70).slice(0, 7)) { x.fillText(l, 35, y); y += 29; }
  y += 14;
  // stats
  const stats = o.stats || [];
  if (stats.length) {
    x.fillStyle = ink; x.fillRect(35, y, W - 70, 3);
    const cw = (W - 70) / stats.length;
    stats.forEach(([k, v], i) => {
      x.textAlign = "center"; x.fillStyle = "#f3ece4"; x.font = "700 34px Oswald"; x.fillText(String(v), 35 + cw * (i + 0.5), y + 48);
      x.fillStyle = "#a8998c"; x.font = "500 11px Oswald"; spaced(x, k.toUpperCase(), 35 + cw * (i + 0.5), y + 70, 2);
    });
    y += 100;
  }
  // credits block
  x.fillStyle = "#a8998c"; x.textAlign = "center"; x.font = "500 15px 'Barlow Condensed'";
  const credits = `KNOX LAUNCHER PRESENTS A ${(o.player || "SURVIVOR").toUpperCase()} PRODUCTION  "${o.title}"  STARRING ${(o.cast || []).map((p) => p.name.toUpperCase()).join("  ") || "NOBODY YET"}  MUSIC BY THE GENERATOR  FILMED ON LOCATION IN KNOX COUNTY, KENTUCKY`;
  for (const l of wrap(x, credits, W - 80).slice(0, 5)) { x.fillText(l, W / 2, y); y += 19; }
  // rating box, barcode, rental sticker
  const by = H - 150;
  x.strokeStyle = "#e8e2d8"; x.lineWidth = 3; x.strokeRect(35, by, 150, 100);
  x.fillStyle = "#e8e2d8"; x.font = "700 64px Oswald"; x.textAlign = "center"; x.fillText("R", 75, by + 74);
  x.font = "600 12px Oswald"; x.textAlign = "left";
  ["RESTRICTED", "ZOMBIES, BAD", "DECISIONS, BEANS"].forEach((l, i) => x.fillText(l, 108, by + 30 + i * 18));
  x.fillStyle = "#fff"; x.fillRect(W - 215, by + 10, 180, 90);
  const rnd = seeded((o.seed || o.title) + "bar");
  x.fillStyle = "#000";
  let bx = W - 205;
  while (bx < W - 45) { const bw = 1 + Math.floor(rnd() * 4); x.fillRect(bx, by + 18, bw, 60); bx += bw + 1 + Math.floor(rnd() * 3); }
  x.font = "500 12px Barlow"; x.textAlign = "center"; x.fillText("0 " + Math.floor(rnd() * 9e10).toString().padStart(11, "0"), W - 125, by + 94);
  stickerBox(x, W / 2 + 10, by + 50, ["KNOX VIDEO", `#${o.rental || "0451"}`], "#f2c230", "#111", 0.05);
  if (o.worn) { wear(x, W, H, (o.seed || "") + "back"); stickerBox(x, W - 120, 160, ["BE KIND", "REWIND"], "#e8e2d8", "#c4140c", 0.12); }
  return c;
}

// drawSpine: the side you see on a shelf. { title, sub, genre, label, num, worn, frame }
// Spines ---------------------------------------------------------------------------------------------

export const SPINE_INKS = { red: "#e3241b", white: "#efe6d8", yellow: "#ffd23a", blue: "#3b86ff", green: "#8be04a", pink: "#ff4fa3",
  orange: "#ff7a1a", purple: "#a868ff", gold: "#e0b43a" };
const GENRE_INK = { horror: "#ff3a2a", slasher: "#e8e2d8", action: "#ffd23a", creature: "#9be04a", tv: "#7fa6ff" };
const STUDIOS = ["KNOX VIDEO", "MULDRAUGH PICTURES", "RIVERSIDE HOME VIDEO", "WEST POINT FILMS", "LOUISVILLE ENT.", "MARCH RIDGE VIDEO",
  "ROSEWOOD RELEASING", "ECHO CREEK FILMS"];
const STYLES = ["classic", "block", "art", "stripes", "rental", "neon", "stencil", "chrome"];

// vtitle writes a title up the spine (reading bottom to top), as big as the spine allows.
// It returns the font size used.
function vtitle(x, text, bottom, top, font, maxSize, paint, offset = 0) {
  x.save();
  x.translate(SPINE / 2 + offset, bottom);
  x.rotate(-Math.PI / 2);
  x.textAlign = "left"; x.textBaseline = "middle";
  const size = fitFont(x, text, font, maxSize, bottom - top, 16);
  paint(x, size);
  x.restore();
  return size;
}

function studioMark(x, name, y, ink, paper) {
  x.save();
  x.fillStyle = ink; x.beginPath(); x.arc(SPINE / 2, y, 30, 0, Math.PI * 2); x.fill();
  x.fillStyle = paper; x.font = "700 26px Oswald"; x.textAlign = "center"; x.textBaseline = "middle";
  x.fillText(name.split(" ").map((w) => w[0]).join("").slice(0, 2), SPINE / 2, y + 1);
  x.font = "500 11px Oswald"; x.fillStyle = ink;
  const words = name.split(" ");
  words.forEach((w, i) => x.fillText(w, SPINE / 2, y + 46 + i * 13));
  x.restore();
}

function rentalSticker(x, num, y, paper = "#f2c230", inkc = "#111") {
  x.save();
  x.fillStyle = paper; x.fillRect(14, y, SPINE - 28, 58);
  x.fillStyle = "rgba(0,0,0,.18)"; x.fillRect(14, y + 54, SPINE - 28, 4);
  x.fillStyle = inkc; x.textAlign = "center"; x.textBaseline = "alphabetic";
  x.font = "700 13px Oswald"; x.fillText("KNOX VIDEO", SPINE / 2, y + 21);
  x.font = "400 26px 'Patrick Hand'"; x.fillText("#" + num, SPINE / 2, y + 48);
  x.restore();
}

function vhsBox(x, y, c) {
  x.save();
  x.strokeStyle = c; x.lineWidth = 2; x.strokeRect(SPINE / 2 - 30, y, 60, 28);
  x.fillStyle = c; x.font = "600 17px Oswald"; x.textAlign = "center"; x.textBaseline = "middle"; x.fillText("VHS", SPINE / 2, y + 15);
  x.restore();
}

function genreDot(x, genre, y, ink) {
  const word = { horror: "HORROR", slasher: "THRILLER", action: "ACTION", creature: "SCI-FI", tv: "TV MOVIE" }[genre] || "HORROR";
  x.save();
  x.fillStyle = ink; x.beginPath(); x.arc(SPINE / 2, y, 34, 0, Math.PI * 2); x.fill();
  x.fillStyle = "rgba(0,0,0,.85)"; x.font = "700 13px Oswald"; x.textAlign = "center"; x.textBaseline = "middle"; x.fillText(word, SPINE / 2, y);
  x.restore();
}

// drawSpine: o.style picks the layout ("" = a seeded pick from o.styles, the styles you have),
// o.ink the colour ("" = the genre's colour).
export async function drawSpine(o) {
  await fonts();
  const c = canvas(SPINE, H), x = c.getContext("2d");
  const genre = o.genre || "horror";
  const seed = String(o.seed || o.title || "") + (o.num || "");
  const rnd = seeded(seed + "spine");
  const have = (o.styles && o.styles.length ? o.styles : ["classic", "block"]).filter((st) => STYLES.includes(st));
  let hash = 2166136261; // FNV-1a: similar names still land on different styles
  for (const ch of seed) hash = Math.imul(hash ^ ch.charCodeAt(0), 16777619);
  hash ^= hash >>> 15; hash = Math.imul(hash, 0x2c1b3c6d); hash ^= hash >>> 12; // FNV's low bits barely move
  const style = STYLES.includes(o.style) ? o.style : have[(hash >>> 0) % have.length] || "classic";
  const ink = SPINE_INKS[o.ink] || GENRE_INK[genre] || "#ff3a2a";
  const studio = STUDIOS[Math.floor(rnd() * STUDIOS.length)];
  const num = o.num || "0001";
  const title = o.title || "UNTITLED";
  const sub = o.sub || "";
  const bottomTxt = H - 170;

  if (style === "classic") {
    x.fillStyle = "#0a0707"; x.fillRect(0, 0, SPINE, H);
    x.fillStyle = ink; x.fillRect(0, 0, SPINE, 10); x.fillRect(0, H - 10, SPINE, 10);
    x.fillRect(0, 96, SPINE, 3);
    vhsBox(x, 34, "#d8c8b8");
    vtitle(x, title, bottomTxt, 130, "700 %px Oswald", 92, (x, size) => {
      x.fillStyle = ink; x.fillText(title, 0, sub ? -12 : 0);
      if (sub) { x.fillStyle = "#d8c8b8"; fitFont(x, sub, "500 %px Oswald", 22, bottomTxt - 130); x.fillText(sub, 0, size / 2 + 8); }
    });
  } else if (style === "block") {
    x.fillStyle = "#070505"; x.fillRect(0, 0, SPINE, H);
    x.fillStyle = ink; x.fillRect(0, 0, SPINE, 250);
    x.fillStyle = "#070505"; x.beginPath(); x.moveTo(0, 250); x.lineTo(SPINE, 210); x.lineTo(SPINE, 250); x.fill();
    studioMark(x, studio, 70, "#070505", ink);
    x.fillStyle = ink; x.fillRect(6, 250, 4, H - 260); x.fillRect(SPINE - 10, 250, 4, H - 260);
    vtitle(x, title, bottomTxt, 280, "700 %px Oswald", 96, (x, size) => {
      x.fillStyle = "#f4ede2"; x.fillText(title, 0, sub ? -12 : 0);
      if (sub) { x.fillStyle = ink; fitFont(x, sub, "700 %px Oswald", 22, bottomTxt - 280); x.fillText(sub, 0, size / 2 + 8); }
    });
  } else if (style === "art") {
    const art = canvas(300, H), ax = art.getContext("2d");
    await drawBackdrop(ax, o.backdrop || "title", 300, H, seed);
    x.filter = "brightness(1.9) contrast(1.2) saturate(1.3)";
    x.drawImage(art, 0, 0, 300, H, 0, 0, SPINE, H);
    x.filter = "none";
    const g = x.createLinearGradient(0, 0, SPINE, 0);
    g.addColorStop(0, "rgba(0,0,0,.55)"); g.addColorStop(0.5, "rgba(0,0,0,0)"); g.addColorStop(1, "rgba(0,0,0,.55)");
    x.fillStyle = g; x.fillRect(0, 0, SPINE, H);
    x.fillStyle = ink; x.fillRect(0, 0, SPINE, 74);
    vhsBox(x, 22, "#111");
    genreDot(x, genre, 130, ink);
    vtitle(x, title, bottomTxt, 190, "700 %px Oswald", 92, (x, size) => {
      x.shadowColor = "#000"; x.shadowBlur = 14; x.shadowOffsetX = 4; x.shadowOffsetY = 4;
      x.fillStyle = "#fff"; x.fillText(title, 0, sub ? -12 : 0);
      x.shadowBlur = 0; x.shadowOffsetX = 0; x.shadowOffsetY = 0;
      x.lineWidth = 2; x.strokeStyle = ink; x.strokeText(title, 0, sub ? -12 : 0);
      if (sub) { x.fillStyle = ink; fitFont(x, sub, "700 %px Oswald", 22, bottomTxt - 190); x.fillText(sub, 0, size / 2 + 8); }
    });
  } else if (style === "stripes") {
    x.fillStyle = "#0b0909"; x.fillRect(0, 0, SPINE, H);
    const bands = [ink, "#f4ede2", ink, "#2a2222"];
    bands.forEach((b, i) => {
      x.fillStyle = b; x.beginPath();
      const y = 40 + i * 34;
      x.moveTo(0, y + 28); x.lineTo(SPINE, y); x.lineTo(SPINE, y + 22); x.lineTo(0, y + 50); x.fill();
    });
    x.fillStyle = ink; x.fillRect(0, H - 40, SPINE, 40);
    vtitle(x, title, bottomTxt, 220, "700 %px Oswald", 96, (x, size) => {
      x.transform(1, 0, -0.18, 1, 0, 0);
      x.fillStyle = ink; x.fillText(title, 4, (sub ? -12 : 0) + 4);
      x.fillStyle = "#f4ede2"; x.fillText(title, 0, sub ? -12 : 0);
      if (sub) { x.fillStyle = ink; fitFont(x, sub, "700 %px Oswald", 22, bottomTxt - 220); x.fillText(sub, 0, size / 2 + 8); }
    });
  } else if (style === "rental") {
    x.fillStyle = "#101010"; x.fillRect(0, 0, SPINE, H);
    x.fillStyle = "#ece3cf"; x.fillRect(12, 60, SPINE - 24, H - 230);
    x.fillStyle = "rgba(255,255,255,.35)"; x.fillRect(30, 48, SPINE - 60, 26); x.fillRect(30, H - 184, SPINE - 60, 26); // tape
    x.fillStyle = ink; x.beginPath(); x.arc(SPINE / 2, 120, 22, 0, Math.PI * 2); x.fill();
    x.save(); x.translate(SPINE / 2, 210); x.rotate(-0.25); x.strokeStyle = "rgba(200,30,20,.7)"; x.lineWidth = 3; x.strokeRect(-46, -16, 92, 32);
    x.fillStyle = "rgba(200,30,20,.75)"; x.font = "700 18px Oswald"; x.textAlign = "center"; x.textBaseline = "middle"; x.fillText("RENTAL", 0, 1); x.restore();
    vtitle(x, title, H - 200, 250, "400 %px 'Patrick Hand'", 84, (x, size) => {
      x.fillStyle = "#151515"; x.fillText(title, 0, sub ? -12 : 0);
      if (sub) { x.fillStyle = "#b8160e"; fitFont(x, sub, "400 %px 'Patrick Hand'", 26, H - 450); x.fillText(sub, 0, size / 2 + 8); }
    });
  } else if (style === "neon") {
    x.fillStyle = "#07060f"; x.fillRect(0, 0, SPINE, H);
    x.strokeStyle = "rgba(160,90,255,.35)"; x.lineWidth = 1;
    for (let i = 0; i < 9; i++) { const y = H - 170 + i * i * 2.4; x.beginPath(); x.moveTo(0, y); x.lineTo(SPINE, y); x.stroke(); }
    x.fillStyle = ink; x.shadowColor = ink; x.shadowBlur = 16;
    x.font = "400 34px VT323"; x.textAlign = "center"; x.fillText("VHS", SPINE / 2, 62);
    x.shadowBlur = 0;
    vtitle(x, title, bottomTxt, 110, "400 %px VT323", 120, (x, size) => {
      x.shadowColor = ink; x.shadowBlur = 22; x.fillStyle = ink; x.fillText(title, 0, sub ? -12 : 0);
      x.shadowBlur = 6; x.fillStyle = "#fff"; x.globalAlpha = 0.55; x.fillText(title, 0, sub ? -12 : 0); x.globalAlpha = 1;
      if (sub) { x.shadowBlur = 10; x.fillStyle = "#ff4fa3"; x.shadowColor = "#ff4fa3"; fitFont(x, sub, "400 %px VT323", 30, bottomTxt - 110); x.fillText(sub, 0, size / 2 + 6); }
    });
  } else if (style === "stencil") {
    x.fillStyle = "#3a3d2a"; x.fillRect(0, 0, SPINE, H);
    const hazard = (y) => { x.save(); x.beginPath(); x.rect(0, y, SPINE, 36); x.clip(); x.fillStyle = ink; x.fillRect(0, y, SPINE, 36);
      x.fillStyle = "#111"; for (let i = -2; i < 8; i++) { x.beginPath(); x.moveTo(i * 30, y + 36); x.lineTo(i * 30 + 15, y + 36); x.lineTo(i * 30 + 51, y); x.lineTo(i * 30 + 36, y); x.fill(); } x.restore(); };
    hazard(20); hazard(H - 56);
    x.fillStyle = "rgba(0,0,0,.3)"; x.font = "400 16px 'Saira Stencil One'"; x.textAlign = "center"; x.fillText("PROPERTY OF", SPINE / 2, 90); x.fillText("KNOX ARMY", SPINE / 2, 110);
    vtitle(x, title, bottomTxt, 140, "400 %px 'Saira Stencil One'", 90, (x, size) => {
      x.fillStyle = ink; x.fillText(title, 0, sub ? -12 : 0);
      if (sub) { x.fillStyle = "#d8d2b8"; fitFont(x, sub, "400 %px 'Saira Stencil One'", 22, bottomTxt - 140); x.fillText(sub, 0, size / 2 + 8); }
    });
  } else { // chrome
    x.fillStyle = "#050405"; x.fillRect(0, 0, SPINE, H);
    x.save(); x.translate(SPINE / 2, 80); x.fillStyle = ink;
    for (let i = 0; i < 16; i++) { x.rotate(Math.PI / 8); x.beginPath(); x.moveTo(0, 0); x.lineTo(5, -60); x.lineTo(-5, -60); x.fill(); }
    x.fillStyle = "#050405"; x.beginPath(); x.arc(0, 0, 22, 0, Math.PI * 2); x.fill(); x.restore();
    vtitle(x, title, bottomTxt, 160, "700 %px Oswald", 98, (x, size) => {
      const g = x.createLinearGradient(0, -size / 2, 0, size / 2);
      g.addColorStop(0, "#ffffff"); g.addColorStop(0.45, "#9aa4b4"); g.addColorStop(0.5, "#2a2e38"); g.addColorStop(0.62, ink); g.addColorStop(1, "#ffffff");
      x.lineWidth = 6; x.strokeStyle = "#000"; x.strokeText(title, 0, sub ? -12 : 0);
      x.fillStyle = g; x.fillText(title, 0, sub ? -12 : 0);
      x.lineWidth = 1.5; x.strokeStyle = "rgba(255,255,255,.6)"; x.strokeText(title, 0, sub ? -12 : 0);
      if (sub) { x.fillStyle = ink; fitFont(x, sub, "700 %px Oswald", 22, bottomTxt - 160); x.fillText(sub, 0, size / 2 + 8); }
    });
  }

  // the bottom of every tape: date, the shop's rental sticker
  x.textAlign = "center"; x.textBaseline = "alphabetic";
  if (o.label) {
    x.fillStyle = style === "rental" ? "#d8d0bc" : "#a8998c";
    fitFont(x, o.label, "500 %px Oswald", 18, SPINE - 16); x.fillText(o.label, SPINE / 2, H - 128);
  }
  rentalSticker(x, num, H - 110 - (style === "stencil" || style === "stripes" ? 6 : 0));
  if (FRAME_INK[o.frame]) { x.fillStyle = FRAME_INK[o.frame]; x.fillRect(0, 0, 6, H); x.fillRect(SPINE - 6, 0, 6, H); }
  if (o.worn) wear(x, SPINE, H, (o.title || "") + "spine");
  return c;
}

// Character posters (lobby slots) -------------------------------------------------------------------
// The survivor is drawn on top in 3D; this is everything behind them. o: { style, ink, name, player, seed, lvl }

function halftone(x, w, h, col, step = 10) {
  x.fillStyle = col;
  for (let yy = 0; yy < h; yy += step) for (let xx = (yy / step) % 2 ? step / 2 : 0; xx < w; xx += step) {
    const r = step * 0.18 + step * 0.22 * (yy / h);
    x.beginPath(); x.arc(xx, yy, r, 0, Math.PI * 2); x.fill();
  }
}

function centerText(x, text, y, font, size, w, fill, min = 10) {
  fitFont(x, text, font, size, w, min);
  x.fillStyle = fill; x.textAlign = "center"; x.textBaseline = "alphabetic"; x.fillText(text, x.canvas.width / 2, y);
}

export async function drawPoster(x, o) {
  await fonts();
  const w = x.canvas.width, h = x.canvas.height, u = w / 230; // u: one css pixel of a slot
  const ink = SPINE_INKS[o.ink] || SPINE_INKS.red;
  const rnd = seeded(String(o.seed) + "poster");
  const name = String(o.name || "SURVIVOR").toUpperCase();
  const style = o.style || "spotlight";
  x.save();
  if (style === "wanted") {
    x.fillStyle = "#d9c49a"; x.fillRect(0, 0, w, h);
    const g = x.createRadialGradient(w / 2, h / 2, h * 0.2, w / 2, h / 2, h * 0.75); g.addColorStop(0, "rgba(0,0,0,0)"); g.addColorStop(1, "rgba(90,50,10,.55)");
    x.fillStyle = g; x.fillRect(0, 0, w, h);
    for (let i = 0; i < 40; i++) { x.fillStyle = `rgba(80,45,10,${0.05 + rnd() * 0.1})`; x.beginPath(); x.arc(rnd() * w, rnd() * h, 2 + rnd() * 14 * u, 0, 7); x.fill(); }
    x.strokeStyle = "#3a2410"; x.lineWidth = 3 * u; x.strokeRect(8 * u, 8 * u, w - 16 * u, h - 16 * u);
    centerText(x, "WANTED", 78 * u, "700 %px Oswald", 58 * u, w - 30 * u, "#2a1708");
    centerText(x, "DEAD OR ALIVE", 100 * u, "400 %px 'Special Elite'", 18 * u, w - 30 * u, ink === SPINE_INKS.white ? "#2a1708" : ink);
    x.fillStyle = "rgba(60,35,10,.25)"; x.fillRect(30 * u, 110 * u, w - 60 * u, 1.5 * u);
  } else if (style === "missing") {
    x.fillStyle = "#f1efe8"; x.fillRect(0, 0, w, h);
    x.globalAlpha = 0.25; halftone(x, w, h, "#888", 7 * u); x.globalAlpha = 1;
    x.fillStyle = "rgba(255,255,255,.75)"; x.fillRect(12 * u, 92 * u, w - 24 * u, h - 100 * u);
    centerText(x, "MISSING", 60 * u, "700 %px Oswald", 54 * u, w - 24 * u, ink === SPINE_INKS.white ? "#c4140c" : ink);
    centerText(x, "HAVE YOU SEEN ME?", 84 * u, "700 %px Oswald", 16 * u, w - 24 * u, "#111");
    x.fillStyle = "rgba(230,220,170,.75)"; x.save(); x.translate(26 * u, 14 * u); x.rotate(-0.5); x.fillRect(-26 * u, -8 * u, 52 * u, 16 * u); x.restore();
    x.save(); x.translate(w - 26 * u, 14 * u); x.rotate(0.5); x.fillRect(-26 * u, -8 * u, 52 * u, 16 * u); x.restore();
  } else if (style === "propaganda") {
    x.fillStyle = "#e9dcc0"; x.fillRect(0, 0, w, h);
    x.save(); x.translate(w / 2, h * 0.52); x.fillStyle = ink;
    for (let i = 0; i < 18; i++) { x.rotate(Math.PI / 9); x.beginPath(); x.moveTo(0, 0); x.lineTo(-w * 0.12, -h); x.lineTo(w * 0.12, -h); x.fill(); }
    x.restore();
    x.fillStyle = "#1a1410"; x.fillRect(0, 0, w, 46 * u);
    centerText(x, "KNOX NEEDS YOU", 32 * u, "700 %px Oswald", 30 * u, w - 20 * u, "#e9dcc0");
  } else if (style === "mugshot") {
    x.fillStyle = "#c9ccc6"; x.fillRect(0, 0, w, h);
    x.strokeStyle = "rgba(20,20,20,.55)"; x.fillStyle = "rgba(20,20,20,.7)"; x.font = `700 ${11 * u}px Oswald`; x.textAlign = "left";
    for (let i = 0; i < 16; i++) { const y = h - i * 18 * u - 10 * u; x.lineWidth = i % 2 ? 1 * u : 2 * u; x.beginPath(); x.moveTo(0, y); x.lineTo(i % 2 ? 30 * u : w, y); x.stroke(); if (!(i % 2)) x.fillText(`${4 + (i >> 1)}'`, 6 * u, y - 3 * u); }
    x.fillStyle = "#121212"; x.fillRect(0, 0, w, 34 * u);
    centerText(x, "KNOX CO. SHERIFF", 24 * u, "700 %px Oswald", 18 * u, w - 20 * u, ink);
  } else if (style === "comic") {
    x.fillStyle = ink; x.fillRect(0, 0, w, h);
    x.globalAlpha = 0.25; halftone(x, w, h, "#000", 9 * u); x.globalAlpha = 1;
    x.fillStyle = "#fff"; x.fillRect(0, 0, w, 50 * u); x.fillStyle = "#111"; x.fillRect(0, 50 * u, w, 4 * u);
    centerText(x, "KNOX TALES", 38 * u, "400 %px 'Saira Stencil One'", 34 * u, w - 70 * u, "#c4140c");
    x.fillStyle = "#ffd23a"; x.strokeStyle = "#111"; x.lineWidth = 2 * u; x.beginPath();
    const cx = w - 34 * u, cy = 82 * u;
    for (let i = 0; i < 20; i++) { const a = (i / 20) * Math.PI * 2, r = (i % 2 ? 16 : 28) * u; x.lineTo(cx + Math.cos(a) * r, cy + Math.sin(a) * r); }
    x.closePath(); x.fill(); x.stroke();
    x.fillStyle = "#111"; x.font = `700 ${11 * u}px Oswald`; x.textAlign = "center"; x.fillText("No." + (1 + Math.floor(rnd() * 99)), cx, cy + 4 * u);
  } else if (style === "fightcard") {
    x.fillStyle = "#0b0b14"; x.fillRect(0, 0, w, h);
    for (let i = 0; i < 60; i++) { x.fillStyle = `rgba(255,255,255,${rnd() * 0.6})`; x.fillRect(rnd() * w, rnd() * h * 0.7, 1.5 * u, 1.5 * u); }
    const g = x.createLinearGradient(0, h * 0.3, 0, h); g.addColorStop(0, "rgba(0,0,0,0)"); g.addColorStop(1, ink); x.globalAlpha = 0.6; x.fillStyle = g; x.fillRect(0, 0, w, h); x.globalAlpha = 1;
    centerText(x, "TONIGHT ONLY", 26 * u, "700 %px Oswald", 16 * u, w - 20 * u, "#ffd23a");
    centerText(x, "LIVE FROM KNOX", 50 * u, "700 %px Oswald", 26 * u, w - 20 * u, "#fff");
    x.fillStyle = ink; x.fillRect(20 * u, 58 * u, w - 40 * u, 3 * u);
  } else if (style === "polaroid") {
    x.fillStyle = "#2a2220"; x.fillRect(0, 0, w, h);
    x.save(); x.translate(w / 2, h / 2); x.rotate((rnd() - 0.5) * 0.06); x.translate(-w / 2, -h / 2);
    x.fillStyle = "#f4f1ea"; x.fillRect(14 * u, 12 * u, w - 28 * u, h - 20 * u);
    const pg = x.createLinearGradient(0, 24 * u, 0, h - 60 * u); pg.addColorStop(0, "#3a3f4a"); pg.addColorStop(1, ink);
    x.fillStyle = pg; x.fillRect(24 * u, 22 * u, w - 48 * u, h - 90 * u);
    x.restore();
    x.fillStyle = "rgba(230,220,170,.8)"; x.save(); x.translate(w / 2, 14 * u); x.rotate(-0.04); x.fillRect(-30 * u, -9 * u, 60 * u, 18 * u); x.restore();
    x.save(); x.translate(w / 2, h - 34 * u); x.rotate(-0.03);
    fitFont(x, o.name || "", "400 %px 'Patrick Hand'", 24 * u, w - 50 * u); x.fillStyle = "#1a1a2a"; x.textAlign = "center"; x.fillText(o.name || "", 0, 0);
    x.restore();
  } else { // spotlight
    x.fillStyle = "#0b0707"; x.fillRect(0, 0, w, h);
    const g = x.createRadialGradient(w / 2, h * 0.42, 10 * u, w / 2, h * 0.42, h * 0.6); g.addColorStop(0, ink); g.addColorStop(0.55, "rgba(0,0,0,.0)");
    x.globalAlpha = 0.7; x.fillStyle = g; x.fillRect(0, 0, w, h); x.globalAlpha = 1;
    x.fillStyle = "rgba(255,255,255,.06)"; x.beginPath(); x.moveTo(w * 0.42, 0); x.lineTo(w * 0.58, 0); x.lineTo(w * 0.85, h); x.lineTo(w * 0.15, h); x.fill();
  }
  x.restore();
}

// Posed cast ------------------------------------------------------------------------------------------

// posePick picks each person's pose: theirs if they chose one, else a seeded pick from the poses
// their level has unlocked (lvl 1 = the base poses).
export function posePick(defs, chosen, rnd, lvl = 1) {
  const list = (defs || []).filter((p) => p.lvl <= lvl);
  if (chosen) { const p = (defs || []).find((d) => d.id === chosen); if (p) return p; }
  return list.length ? list[Math.floor(rnd() * list.length)] : { anim: "Bob_Idle", at: 0.1 };
}

// stageCast puts people on a poster stage (lights and spots) and returns the scene and camera.
export function stageCast(people, seed, { fogColor = 0x12060a } = {}) {
  const scene = new THREE.Scene();
  scene.fog = new THREE.Fog(fogColor, 2.5, 7);
  const camera = new THREE.PerspectiveCamera(30, 1, 0.01, 30);
  scene.add(new THREE.HemisphereLight(0x9fb4ff, 0x1a0505, 0.8));
  const top = new THREE.DirectionalLight(0xdfe6ff, 2.2); top.position.set(0.6, 3, 1.4); scene.add(top);
  const red = new THREE.DirectionalLight(0xff2a1a, 3.0); red.position.set(-1.5, 1, -2.5); scene.add(red);
  const red2 = new THREE.DirectionalLight(0xff5a2a, 1.4); red2.position.set(2, 0.4, -1.5); scene.add(red2);
  const fill = new THREE.DirectionalLight(0xffe0d0, 0.9); fill.position.set(0, 1, 3); scene.add(fill);
  const n = Math.max(1, people.length);
  const rnd = seeded(seed + "pose");
  const span = Math.min(2.4, 0.55 * (n - 1));
  const ready = [];
  people.forEach((p, i) => {
    if (!p.c) return;
    const t = n === 1 ? 0 : i / (n - 1) - 0.5;
    const spot = { x: t * span, z: -Math.abs(t) * 0.6 + (n > 2 && Math.abs(t) < 0.01 ? 0.15 : 0), ry: Math.PI - t * 0.7 };
    const holder = new THREE.Group();
    holder.position.set(spot.x, 0, spot.z);
    holder.rotation.y = spot.ry + (rnd() - 0.5) * 0.3;
    scene.add(holder);
    const pose = p.pose || { anim: "Bob_Idle", at: 0.1 };
    ready.push(build(p.c).then(async (g) => {
      holder.add(g);
      sizeHolder(holder, p.c);
      await freezePose(g, pose.anim, pose.at);
    }));
  });
  const fit = (w, h) => {
    camera.aspect = w / h;
    const f = 2 * Math.tan((camera.fov * Math.PI) / 360);
    camera.userData.dist = Math.max(1.35 / f, (span + 0.75) / (f * camera.aspect));
    camera.position.set(0, 0.3, camera.userData.dist + 0.1);
    camera.lookAt(0, 0.62, 0);
    camera.updateProjectionMatrix();
  };
  return { scene, camera, ready: Promise.all(ready), fit, red };
}

// castShot renders people once to a transparent canvas (for covers).
const shotCache = new Map();
export function castShot(people, seed, w = 600, h = 700) {
  const key = seed + "|" + w + "x" + h + "|" + people.map((p) => JSON.stringify(p.c || {}).length + ":" + (p.pose && p.pose.anim)).join(",");
  if (!shotCache.has(key)) {
    shotCache.set(key, (async () => {
      if (!people.some((p) => p.c)) return null;
      const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true, preserveDrawingBuffer: true });
      renderer.setPixelRatio(1);
      renderer.setSize(w, h, false);
      renderer.outputColorSpace = THREE.SRGBColorSpace;
      renderer.toneMapping = THREE.ACESFilmicToneMapping;
      const st = stageCast(people, seed);
      st.scene.fog = null;
      await st.ready;
      st.fit(w, h);
      renderer.render(st.scene, st.camera);
      const out = canvas(w, h);
      out.getContext("2d").drawImage(renderer.domElement, 0, 0);
      renderer.dispose();
      renderer.forceContextLoss();
      return out;
    })());
    if (shotCache.size > 40) shotCache.delete(shotCache.keys().next().value);
  }
  return shotCache.get(key);
}

export function blurb({ games, kills, deaths, bestDay, cast, town, story }) {
  const who = !cast || cast.length < 2 ? "One survivor" : cast.length === 2 ? "Two survivors" : cast.length === 3 ? "Three survivors" : `${cast.length} survivors`;
  const where = town ? ` in ${town}` : " in Knox County";
  if (!games) return `${who}${where}. No food, no plan, no idea what happens next. This tape hasn't been played yet.`;
  const d = deaths === 0 ? "Nobody has died yet." : deaths === 1 ? "One of them didn't make it." : `${deaths} deaths so far.`;
  return `${who}${where}. ${games} night${games === 1 ? "" : "s"} on this tape, ${kills} zombies down, and the best run lasted to day ${bestDay}. ${d}${story ? " Story: " + story + "." : ""}`;
}

// tbaFig: the cast spot on a tape nobody has played yet (three blank silhouettes, maybe a stamp).
export function tbaFig(stamp) {
  const c = document.createElement("canvas");
  c.width = 600; c.height = 900;
  const x = c.getContext("2d");
  const body = (cx, s, a) => {
    x.globalAlpha = a; x.fillStyle = "#0d0b0b";
    x.beginPath(); x.arc(cx, 330 * s + 120, 70 * s, 0, Math.PI * 2); x.fill();
    x.beginPath(); x.moveTo(cx - 130 * s, 900); x.quadraticCurveTo(cx - 130 * s, 430 * s + 120, cx, 420 * s + 120); x.quadraticCurveTo(cx + 130 * s, 430 * s + 120, cx + 130 * s, 900); x.fill();
    x.globalAlpha = 1; x.fillStyle = "#3a3434"; x.font = `700 ${80 * s}px Oswald`; x.textAlign = "center"; x.textBaseline = "middle"; x.fillText("?", cx, 330 * s + 122);
  };
  body(150, 0.85, 0.85); body(450, 0.85, 0.85); body(300, 1, 0.95);
  if (!stamp) return c;
  x.save(); x.translate(300, 690); x.rotate(-0.08);
  x.font = "700 44px Oswald";
  const w = Math.max(470, x.measureText(stamp).width + 50);
  x.fillStyle = "rgba(239,233,220,0.92)"; x.fillRect(-w / 2, -42, w, 84);
  x.fillStyle = "#c4140c"; x.textAlign = "center"; x.textBaseline = "middle"; x.fillText(stamp, 0, 3);
  x.restore();
  return c;
}
