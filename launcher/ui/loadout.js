// Loadout page: what everyone carries when their character first spawns.
import { html, useState, S, update, rpc, act, toast, patchHost, PageHead, HostOnly, isHost, loadPresets } from "./core.js";

export function Loadout() {
  const host = isHost();
  const [q, setQ] = useState("");
  if (!S.presets || !S.cat) return null;
  const h = host ? S.cfg.host : S.state.info || {};
  const kit = h.kit || [];
  const name = (it) => it.name || (S.cat.itemById[it.type] || {}).name || it.type;
  const setKit = (k, label) => patchHost({ kit: k, kitName: label === undefined ? "Custom" : label });
  const hits = q.length >= 2 ? S.cat.items.filter((it) => it.name.toLowerCase().includes(q.toLowerCase()) || it.id.toLowerCase().includes(q.toLowerCase())).slice(0, 40) : [];
  const builtin = S.presets.loadouts.filter((l) => !l.custom);
  const mine = S.presets.loadouts.filter((l) => l.custom);
  const card = (l) => html`<div key=${l.id} class=${"preset" + (h.kitName === l.name ? " on" : "")} onClick=${() => host && act(() => rpc("applyLoadout", { id: l.id }), l.name + " loadout").then(async (r) => { if (r !== undefined) update({ cfg: await rpc("config") }); })}>
    <div class="row"><b class="grow">${l.name}</b>${h.kitName === l.name ? html`<span class="chip accent">Chosen</span>` : null}
      ${l.custom && host ? html`<button class="btn sm ghost" title="Delete" onClick=${async (e) => { e.stopPropagation(); if (confirm("Delete the loadout " + l.name + "?")) { await act(() => rpc("deleteLoadout", { name: l.name }), "Deleted"); loadPresets(); } }}>✕</button>` : null}</div>
    <p>${l.explain}</p>
    ${l.explain ? null : html`<div class="kit-chips">${(l.items || []).map((it) => html`<span key=${it.type} class="chip">${it.count > 1 ? it.count + "× " : ""}${name(it)}</span>`)}${!(l.items || []).length ? html`<span class="dim small">Nothing</span>` : null}</div>`}
  </div>`;
  return html`
    <${PageHead} title="Loadout" sub="What each player spawns with." />
    ${!host ? html`<${HostOnly} what="loadout" />` : null}
    <div class="loadout-layout">
      <div>
        <div class="group-title">Presets</div>
        <div class="preset-grid">${builtin.map(card)}</div>
        ${mine.length ? html`<div class="group-title">Your loadouts</div><div class="preset-grid">${mine.map(card)}</div>` : null}
      </div>
      <div class="card kit-card">
        <h3>Everyone gets${h.kitName ? ": " + h.kitName : ""}</h3>
        ${kit.map((k, i) => html`<div key=${k.type + i} class="kit-row">
          <div title=${k.type}><b>${name(k)}</b></div>
          <input type="number" min="1" max="50" value=${k.count} disabled=${!host} onInput=${(e) => { const next = kit.map((x, j) => (j === i ? { ...x, count: Math.max(1, Math.min(50, parseInt(e.target.value, 10) || 1)) } : x)); setKit(next); }} />
          ${host ? html`<button class="btn sm ghost" onClick=${() => setKit(kit.filter((_, j) => j !== i))}>✕</button>` : html`<span></span>`}
        </div>`)}
        ${!kit.length ? html`<div class="hint">Nothing yet. Pick a preset or add items.</div>` : null}
        ${host ? html`
          <div style=${{ position: "relative", marginTop: "10px" }}>
            <input type="text" style=${{ width: "100%" }} placeholder="Add item" value=${q} onInput=${(e) => setQ(e.target.value)} />
            ${hits.length ? html`<div class="search-pop">${hits.map((it) => html`<div key=${it.id} onClick=${() => { setKit([...kit, { type: it.id, name: it.name, count: 1 }]); setQ(""); }}>${it.name}<span class="dim small">  ${it.cat || ""} · ${it.id}</span></div>`)}</div>` : null}
          </div>
          <div class="row" style=${{ marginTop: "12px" }}>
            <button class="btn" disabled=${!kit.length} onClick=${async () => {
              const n = prompt("Save as", h.kitName && h.kitName !== "Custom" ? h.kitName : "");
              if (!n) return;
              if (await act(() => rpc("saveLoadout", { name: n, items: kit }), "Saved " + n) !== undefined) { S.cfg.host.kitName = n; update(); loadPresets(); }
            }}>Save as my loadout</button>
            <button class="btn ghost" disabled=${!kit.length} onClick=${() => setKit([], "")}>Clear</button>
          </div>` : null}
      </div>
    </div>`;
}
