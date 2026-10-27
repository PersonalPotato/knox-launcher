// VCR sounds, made on the fly (no sound files): tape in, play, eject, rewind, case clack.
let ctx = null;
let muted = false;
try { muted = localStorage.getItem("knox-mute") === "1"; } catch {}

let vol = 0.8;
try { const v = parseFloat(localStorage.getItem("knox-vol")); if (v >= 0 && v <= 1) vol = v; } catch {}
let master = null;

export function soundOn() { return !muted; }
export function soundVol() { return vol; }
export function setVol(v) {
  vol = Math.max(0, Math.min(1, v));
  if (master) master.gain.value = vol;
  try { localStorage.setItem("knox-vol", String(vol)); } catch {}
}
// out: every sound goes through one volume knob
function out(c) {
  if (!master) { master = c.createGain(); master.gain.value = vol; master.connect(c.destination); }
  return master;
}
export function setSound(on) {
  muted = !on;
  try { localStorage.setItem("knox-mute", muted ? "1" : "0"); } catch {}
}

function ac() {
  if (!ctx) {
    try { ctx = new (window.AudioContext || window.webkitAudioContext)(); } catch { return null; }
  }
  if (ctx.state === "suspended") ctx.resume();
  return ctx;
}

function noise(c, secs) {
  const b = c.createBuffer(1, Math.max(1, Math.floor(c.sampleRate * secs)), c.sampleRate);
  const d = b.getChannelData(0);
  for (let i = 0; i < d.length; i++) d[i] = Math.random() * 2 - 1;
  const s = c.createBufferSource();
  s.buffer = b;
  return s;
}

// thunk: a short filtered knock (mechanism parts hitting home)
function thunk(c, at, freq, gain, len = 0.08) {
  const n = noise(c, len);
  const f = c.createBiquadFilter(); f.type = "bandpass"; f.frequency.value = freq; f.Q.value = 2.5;
  const g = c.createGain();
  g.gain.setValueAtTime(gain, at); g.gain.exponentialRampToValueAtTime(0.001, at + len);
  n.connect(f).connect(g).connect(out(c));
  n.start(at); n.stop(at + len);
  const o = c.createOscillator(); o.type = "sine";
  o.frequency.setValueAtTime(freq / 6, at); o.frequency.exponentialRampToValueAtTime(40, at + len);
  const og = c.createGain(); og.gain.setValueAtTime(gain * 0.8, at); og.gain.exponentialRampToValueAtTime(0.001, at + len);
  o.connect(og).connect(out(c)); o.start(at); o.stop(at + len);
}

// whirr: motor noise sweeping in pitch
function whirr(c, at, secs, from, to, gain) {
  const n = noise(c, secs);
  const f = c.createBiquadFilter(); f.type = "bandpass"; f.Q.value = 6;
  f.frequency.setValueAtTime(from, at); f.frequency.linearRampToValueAtTime(to, at + secs);
  const g = c.createGain();
  g.gain.setValueAtTime(0.0001, at); g.gain.linearRampToValueAtTime(gain, at + 0.15);
  g.gain.setValueAtTime(gain, at + secs - 0.2); g.gain.linearRampToValueAtTime(0.0001, at + secs);
  n.connect(f).connect(g).connect(out(c));
  n.start(at); n.stop(at + secs);
  const o = c.createOscillator(); o.type = "sawtooth";
  o.frequency.setValueAtTime(from / 8, at); o.frequency.linearRampToValueAtTime(to / 8, at + secs);
  const lp = c.createBiquadFilter(); lp.type = "lowpass"; lp.frequency.value = 600;
  const og = c.createGain(); og.gain.setValueAtTime(0.0001, at); og.gain.linearRampToValueAtTime(gain * 0.25, at + 0.15); og.gain.linearRampToValueAtTime(0.0001, at + secs);
  o.connect(lp).connect(og).connect(out(c)); o.start(at); o.stop(at + secs);
}

const SOUNDS = {
  // tape slides in, the door drops, the heads load, play
  insert: (c, t) => { whirr(c, t, 0.35, 900, 500, 0.06); thunk(c, t + 0.35, 900, 0.5); thunk(c, t + 0.6, 400, 0.35); whirr(c, t + 0.7, 0.7, 300, 380, 0.035); thunk(c, t + 1.4, 1200, 0.3, 0.05); },
  // the big chunky play key
  play: (c, t) => { thunk(c, t, 1500, 0.35, 0.05); thunk(c, t + 0.06, 500, 0.45); whirr(c, t + 0.1, 0.5, 320, 360, 0.03); },
  eject: (c, t) => { thunk(c, t, 700, 0.35); whirr(c, t + 0.05, 0.5, 450, 800, 0.05); thunk(c, t + 0.6, 1100, 0.4, 0.06); },
  rewind: (c, t) => { thunk(c, t, 900, 0.3, 0.05); whirr(c, t + 0.05, 1.8, 600, 1900, 0.06); thunk(c, t + 1.9, 600, 0.45); },
  clack: (c, t) => { thunk(c, t, 2200, 0.25, 0.04); thunk(c, t + 0.03, 900, 0.2, 0.05); },
  flip: (c, t) => { whirr(c, t, 0.18, 2500, 1500, 0.03); },
  button: (c, t) => { thunk(c, t, 1800, 0.18, 0.03); },
};

export function sound(name) {
  if (muted || !SOUNDS[name]) return;
  const c = ac();
  if (!c) return;
  try { SOUNDS[name](c, c.currentTime + 0.01); } catch {}
}
