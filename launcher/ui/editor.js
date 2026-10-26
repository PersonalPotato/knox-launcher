// Story editor: make your own squad story, or change a copy of any other one.
import { html, useState, S, update, rpc, act, toast, loadPresets, Select } from "./core.js";
import { TagInput } from "./community.js";

const TAGS = [["easy", "Easy"], ["hard", "Hard"], ["town", "Town"], ["wild", "Woods"], ["long", "Long game"]];

const blankRole = () => ({ name: "", profession: "unemployed", traits: [], note: "" });

export function newStory() {
  return { name: "", story: "", min: 1, max: 4, building: "house", town: "", loadout: "basics", tags: [], tip: "", roles: [{ ...blankRole(), name: "Survivor" }] };
}

// copyStory makes an editable copy of any story (built-in, community or yours).
export function copyStory(b, keepId) {
  const c = JSON.parse(JSON.stringify(b));
  if (!keepId) {
    c.id = "";
    c.name = b.name + " (copy)";
  }
  for (const k of ["locked", "mod", "custom", "have", "liked", "downloads", "likes", "days", "comments"]) delete c[k];
  if (!c.roles || !c.roles.length) c.roles = [blankRole()];
  return c;
}

export function openEditor(story) {
  update({ modal: { kind: "editStory", story: story || newStory() } });
}

// rolePoints is what a role has left: the job's points minus its traits, like the game's creator.
function rolePoints(r) {
  const prof = S.cat.profById[r.profession];
  let pts = prof ? prof.cost : 0;
  for (const id of r.traits || []) {
    const t = S.cat.traitById[id];
    if (t) pts -= t.cost;
  }
  return pts;
}

function clashes(r, t) {
  const prof = S.cat.profById[r.profession];
  const have = new Set([...(r.traits || []), ...((prof && prof.granted) || [])]);
  if (have.has(t.id)) return true;
  for (const id of have) {
    const o = S.cat.traitById[id];
    if ((t.exclusive || []).includes(id) || (o && (o.exclusive || []).includes(t.id))) return true;
  }
  return false;
}

export function StoryEditor({ story, close }) {
  const [b, setB] = useState(() => copyStory(story, true));
  const [saving, setSaving] = useState(false);
  const P = S.presets;
  if (!P || !S.cat) return null;
  const set = (patch) => setB({ ...b, ...patch });
  const setRole = (i, patch) => set({ roles: b.roles.map((r, j) => (j === i ? { ...r, ...patch } : r)) });
  const profs = [...S.cat.professions].sort((x, y) => x.name.localeCompare(y.name));
  const towns = (P.towns || []).map((t) => t.name);
  const save = async () => {
    setSaving(true);
    const saved = await act(() => rpc("saveStory", { story: { ...b, min: +b.min || 1, max: +b.max || 1 } }));
    setSaving(false);
    if (!saved) return;
    await loadPresets();
    toast(b.name + " saved");
    update({ modal: { kind: "story", id: saved.id } });
  };
  const bad = !b.name.trim() ? "Needs a name." : !b.roles.some((r) => r.name.trim()) ? "Needs at least one role." : "";
  return html`<div class="editor">
    <div class="ed-grid">
      <label class="field ed-wide"><span>Name</span><input type="text" maxlength="60" placeholder="Last Shift" value=${b.name} onInput=${(e) => set({ name: e.target.value })} /></label>
      <label class="field ed-wide"><span>Story</span><textarea rows="3" placeholder="What happened. One or two lines." value=${b.story} onInput=${(e) => set({ story: e.target.value })}></textarea></label>
      <label class="field"><span>Players</span>
        <div class="row" style=${{ gap: "6px" }}>
          <input type="number" min="1" max="16" value=${b.min} onInput=${(e) => set({ min: e.target.value })} style=${{ width: "70px" }} />
          <span class="dim">to</span>
          <input type="number" min="1" max="16" value=${b.max} onInput=${(e) => set({ max: e.target.value })} style=${{ width: "70px" }} />
        </div></label>
      <label class="field"><span>Start in</span><${Select} options=${P.buildings.map((x) => ({ value: x.id, label: x.name }))} value=${b.building} onChange=${(v) => set({ building: v })} /></label>
      <label class="field"><span>Town</span><${Select} options=${[{ value: "", label: "Anywhere" }, ...towns.map((t) => ({ value: t, label: t }))]} value=${b.town || ""} onChange=${(v) => set({ town: v })} /></label>
      <label class="field"><span>Loadout</span><${Select} options=${[{ value: "", label: "Keep the host's" }, ...P.loadouts.map((l) => ({ value: l.id, label: l.name }))]} value=${b.loadout || ""} onChange=${(v) => set({ loadout: v })} /></label>
      <div class="field ed-wide"><span>Tags</span><${TagInput} tags=${b.tags || []} setTags=${(tags) => set({ tags })} presets=${TAGS} /></div>
      <label class="field ed-wide"><span>Tip</span><input type="text" maxlength="90" placeholder="Shown under the story. Short." value=${b.tip} onInput=${(e) => set({ tip: e.target.value })} /></label>
    </div>

    <div class="row" style=${{ marginTop: "18px" }}><h3 class="grow" style=${{ margin: 0 }}>Roles · ${b.roles.length}</h3>
      <button class="btn sm" onClick=${() => set({ roles: [...b.roles, blankRole()] })}>+ Add role</button></div>
    <div class="hint" style=${{ margin: "4px 0 10px" }}>Each player takes one role in the lobby. It sets their job and starting traits.</div>
    <div class="ed-roles">${b.roles.map((r, i) => html`<${RoleEditor} key=${i} r=${r} profs=${profs}
      set=${(patch) => setRole(i, patch)}
      remove=${b.roles.length > 1 ? () => set({ roles: b.roles.filter((_, j) => j !== i) }) : null} />`)}</div>

    <div class="row ed-foot">
      <span class="grow dim small">${bad}</span>
      <button class="btn ghost" onClick=${close}>Cancel</button>
      <button class="btn primary" disabled=${!!bad || saving} onClick=${save}>${saving ? "Saving..." : b.id ? "Save" : "Save story"}</button>
    </div>
  </div>`;
}

function RoleEditor({ r, profs, set, remove }) {
  const [adding, setAdding] = useState("");
  const pts = rolePoints(r);
  const prof = S.cat.profById[r.profession];
  const granted = new Set((prof && prof.granted) || []);
  const choices = S.cat.traits.filter((t) => !t.free && !t.noMP && !clashes(r, t)).sort((x, y) => (y.cost > 0) - (x.cost > 0) || x.name.localeCompare(y.name));
  const add = (id) => {
    if (id) set({ traits: [...(r.traits || []), id] });
    setAdding("");
  };
  return html`<div class="ed-role">
    <div class="row" style=${{ gap: "8px" }}>
      <input type="text" class="grow" maxlength="40" placeholder="Role name" value=${r.name} onInput=${(e) => set({ name: e.target.value })} />
      <${Select} options=${[{ value: "", label: "Any job" }, ...profs.map((p) => ({ value: p.id, label: `${p.name} (${p.cost > 0 ? "+" : ""}${p.cost})` }))]} value=${r.profession || ""} onChange=${(v) => set({ profession: v, traits: (r.traits || []).filter((id) => !(S.cat.profById[v] && (S.cat.profById[v].granted || []).includes(id))) })} />
      ${remove ? html`<button class="btn sm ghost" title="Remove role" onClick=${remove}>✕</button>` : null}
    </div>
    <input type="text" maxlength="80" placeholder="One line about them" value=${r.note} onInput=${(e) => set({ note: e.target.value })} />
    <div class="role-traits">
      ${[...granted].map((id) => html`<span key=${"g" + id} class="chip" title="Comes with the job">★ ${S.cat.traitById[id] ? S.cat.traitById[id].name : id}</span>`)}
      ${(r.traits || []).map((id) => {
        const t = S.cat.traitById[id];
        return html`<span key=${id} class=${"chip click " + (t && t.cost > 0 ? "good" : "bad")} title="Remove" onClick=${() => set({ traits: r.traits.filter((x) => x !== id) })}>${t ? t.name : id} ✕</span>`;
      })}
      <select class="ed-add" value=${adding} onChange=${(e) => add(e.target.value)}>
        <option value="">+ Trait</option>
        <optgroup label="Good">${choices.filter((t) => t.cost > 0).map((t) => html`<option value=${t.id}>${t.name} (-${t.cost})</option>`)}</optgroup>
        <optgroup label="Bad">${choices.filter((t) => t.cost <= 0).map((t) => html`<option value=${t.id}>${t.name} (+${-t.cost})</option>`)}</optgroup>
      </select>
    </div>
    <div class=${"small " + (pts < 0 ? "bad-text" : "dim")}>${pts < 0 ? `${-pts} points over. Add a bad trait or drop a good one, or the launcher will do it.` : `${pts} points left`}</div>
  </div>`;
}
