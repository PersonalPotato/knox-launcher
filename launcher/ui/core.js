// Shared state, server calls and small helpers for every page.
import { html, useState, useEffect, useRef, useMemo } from "./vendor/preact-htm.js";

export { html, useState, useEffect, useRef, useMemo };

export const TOKEN = window.EZ_TOKEN;

// One store for the whole app. Pages read it with useStore() and re-render when it changes.
// Preact only patches what changed, so scroll positions and focus stay where they are.
export const S = {
  tab: "play",
  cfg: null,
  cat: null,
  presets: null,
  state: null,
  chat: [],
  chatLen: 0,
  editedAt: 0, // last local edit; the config isn't reloaded from disk right after one
  toast: null,
  modal: null,
};

window.__knox = { S };
const listeners = new Set();
let queued = false;
export function update(patch) {
  window.__knox.update = update;
  if (patch) Object.assign(S, patch);
  if (queued) return;
  queued = true;
  requestAnimationFrame(() => {
    queued = false;
    for (const l of listeners) l();
  });
}

export function useStore() {
  const [, force] = useState(0);
  useEffect(() => {
    const l = () => force((x) => x + 1);
    listeners.add(l);
    return () => listeners.delete(l);
  }, []);
  return S;
}

export async function rpc(method, args) {
  const r = await fetch("/rpc", {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Token": TOKEN },
    body: JSON.stringify({ method, args: args || {} }),
  });
  const j = await r.json();
  if (j.error) throw new Error(j.error);
  return j.result;
}

let toastTimer;
export function toast(text, bad) {
  clearTimeout(toastTimer);
  update({ toast: { text, bad } });
  toastTimer = setTimeout(() => update({ toast: null }), bad ? 6000 : 3000);
}

// act runs a call and shows any error as a toast. Returns undefined on failure.
export async function act(fn, okMsg) {
  try {
    const r = await fn();
    if (okMsg) toast(okMsg);
    return r === undefined ? null : r;
  } catch (e) {
    toast(e.message || String(e), true);
    return undefined;
  }
}

const timers = {};
export function debounce(key, ms, fn) {
  clearTimeout(timers[key]);
  timers[key] = setTimeout(fn, ms);
}

export async function loadConfig() {
  const cfg = await rpc("config");
  update({ cfg, cfgRev: S.state ? S.state.cfgRev : 0 });
}

export async function loadCatalog() {
  try {
    const r = await rpc("catalog", { version: S.cat ? S.cat.version : "" });
    if (r && r.same) return;
    const cat = r;
    cat.profById = Object.fromEntries(cat.professions.map((p) => [p.id, p]));
    cat.traitById = Object.fromEntries(cat.traits.map((t) => [t.id, t]));
    cat.itemById = Object.fromEntries(cat.items.map((t) => [t.id, t]));
    cat.modById = Object.fromEntries(cat.mods.map((m) => [m.id, m]));
    const old = S.cat;
    update({ cat });
    if (old) {
      // say what the Workshop changed
      const before = new Set(old.mods.map((m) => m.id));
      const after = new Set(cat.mods.map((m) => m.id));
      const added = cat.mods.filter((m) => !before.has(m.id)).map((m) => m.name);
      const gone = old.mods.filter((m) => !after.has(m.id)).map((m) => m.name);
      if (added.length) toast("New mod" + (added.length > 1 ? "s" : "") + " from the Workshop: " + added.join(", "));
      else if (gone.length) toast("Removed: " + gone.join(", "));
    }
    loadPresets();
  } catch (e) {
    update({ catErr: e.message });
  }
}

export async function loadPresets() {
  const presets = await act(() => rpc("presets"));
  if (presets) update({ presets });
}

// Host settings: change locally right away, send only what changed.
export function patchHost(patch, msg) {
  Object.assign(S.cfg.host, patch);
  S.editedAt = Date.now();
  update();
  return act(() => rpc("patchHost", { host: patch }), msg);
}

export function setOption(kind, name, value) {
  const map = kind === "ini" ? S.cfg.host.ini : S.cfg.host.sandbox;
  if (value === "" || value == null) delete map[name];
  else map[name] = String(value);
  S.editedAt = Date.now();
  update();
  debounce("opt-" + kind + name, 250, () => act(() => rpc("patchHost", { host: { [kind]: { [name]: value == null ? "" : String(value) } } })));
}

export function saveCharacter() {
  S.editedAt = Date.now();
  update();
  debounce("char", 300, () => act(() => rpc("saveCharacter", { character: S.cfg.character })));
}

// helpers -----------------------------------------------------------------------------------

export const pick = (a) => a[Math.floor(Math.random() * a.length)];
export const rgbCss = (c) => (c ? `rgb(${Math.round(c.r * 255)},${Math.round(c.g * 255)},${Math.round(c.b * 255)})` : "#888");
export const sameColor = (a, b) => a && b && Math.abs(a.r - b.r) < 0.01 && Math.abs(a.g - b.g) < 0.01 && Math.abs(a.b - b.b) < 0.01;
export const stripTags = (s) => String(s || "").replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();
export const placeText = (spot) =>
  spot ? `${prettyKind(spot.kind) || "Building"} ${spot.name ? "near " + spot.name.replace(", KY", "") : "out in the country"}` : "";
export const prettyKind = (k) => (k ? k.replace(/([a-z])([A-Z])/g, "$1 $2").replace(/^yes$/, "Building") : "");

export function fmtBytes(n) {
  if (!n) return "0 MB";
  if (n > 1e9) return (n / 1e9).toFixed(1) + " GB";
  return Math.max(0.1, n / 1e6).toFixed(1) + " MB";
}

export function ago(sec) {
  if (sec < 60) return "just now";
  if (sec < 3600) return Math.floor(sec / 60) + " min ago";
  if (sec < 86400) return Math.floor(sec / 3600) + " h ago";
  return Math.floor(sec / 86400) + " days ago";
}

export const isHost = () => !S.state || S.state.lobby === "0" || S.state.host;
export const inLobby = () => S.state && S.state.lobby !== "0";

export function profName(id) {
  const p = S.cat && S.cat.profById[id];
  return p ? p.name : id || "Unemployed";
}

// Small shared components ----------------------------------------------------------------------

export function Switch({ on, onChange, disabled }) {
  return html`<label class="switch"><input type="checkbox" checked=${!!on} disabled=${disabled} onChange=${(e) => onChange(e.target.checked)} /><i></i></label>`;
}

export function Toggle({ title, sub, on, onChange, disabled }) {
  return html`<div class="toggle-row"><div class="t"><b>${title}</b>${sub ? html`<small>${sub}</small>` : null}</div><${Switch} on=${on} onChange=${onChange} disabled=${disabled} /></div>`;
}

export function Select({ options, value, onChange, ...rest }) {
  return html`<select value=${value} onChange=${(e) => onChange(e.target.value)} ...${rest}>
    ${options.map((o) => (typeof o === "object" ? html`<option value=${o.value}>${o.label}</option>` : html`<option value=${o}>${o}</option>`))}
  </select>`;
}

export function Avatar({ id, name, small }) {
  const [bad, setBad] = useState(false);
  const letter = (name || "?").slice(0, 1).toUpperCase();
  const style = !bad && id && id !== "0" ? { backgroundImage: `url(/avatar/${id}.png?t=${TOKEN})` } : null;
  useEffect(() => {
    if (!id || id === "0") return;
    const img = new Image();
    img.onerror = () => setBad(true);
    img.src = `/avatar/${id}.png?t=${TOKEN}`;
  }, [id]);
  return html`<div class=${"avatar" + (small ? " sm" : "")} style=${style}>${style ? "" : letter}</div>`;
}

export function PageHead({ title, sub, children }) {
  return html`<div class="page-head"><div><h1>${title}</h1>${sub ? html`<div class="sub">${sub}</div>` : null}</div>${children ? html`<div class="actions">${children}</div>` : null}</div>`;
}

export function Empty({ title, children }) {
  return html`<div class="card empty-state"><h2>${title}</h2>${children}</div>`;
}

// For guests: host-only pages show this instead of controls.
export function HostOnly({ what }) {
  const st = S.state || {};
  return html`<div class="card note-card"><b>${st.hostName || "The host"} sets the ${what}.</b> Updates live.</div>`;
}
