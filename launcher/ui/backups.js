// Backups page: every world is backed up before it's hosted; roll back in one click.
import { html, useState, useEffect, S, rpc, act, update, fmtBytes, ago, PageHead, Select, Empty } from "./core.js";

export function Backups() {
  const [world, setWorld] = useState(S.cfg.host.world);
  const [worlds, setWorlds] = useState([]);
  const [list, setList] = useState(null);
  const load = async (w) => {
    const [ws, bs] = await Promise.all([act(() => rpc("worlds")), act(() => rpc("backups", { world: w }))]);
    setWorlds(ws || []);
    setList(bs || []);
  };
  useEffect(() => { load(world); }, [world]);
  const running = S.state && S.state.game && S.state.game.running;
  return html`
    <${PageHead} title="Backups" sub="Made every time you host. Roll back any time.">
      ${worlds.length ? html`<${Select} options=${[...new Set([world, ...worlds.map((w) => w.name)])]} value=${world} onChange=${setWorld} />` : null}
      <label class="row small muted">Keep
        <${Select} options=${[3, 5, 10, 20, 50].map((n) => ({ value: n, label: String(n) }))} value=${S.cfg.keepBackups || 5} onChange=${async (v) => { await act(() => rpc("setKeepBackups", { keep: parseInt(v, 10) }), "Saved"); S.cfg.keepBackups = parseInt(v, 10); update(); }} /></label>
      <button class="btn primary" disabled=${running} onClick=${async () => { if (await act(() => rpc("backupNow", { world }), "Backed up") !== undefined) load(world); }}>Back up now</button>
    </${PageHead}>
    ${running ? html`<div class="card note-card">Close the game to back up or roll back.</div>` : null}
    ${list && !list.length ? html`<${Empty} title="No backups of ${world} yet"><p>One gets made each time you host it, or press Back up now.</p></${Empty}>` : null}
    ${list && list.length ? html`<div class="card">${list.map((b) => {
      const d = new Date(b.time * 1000);
      return html`<div key=${b.file} class="row" style=${{ padding: "10px 0", borderBottom: "1px solid var(--line)" }}>
        <div class="grow"><b>${d.toLocaleString()}</b><div class="dim small">${fmtBytes(b.size)} · ${ago((Date.now() - d) / 1000)}</div></div>
        <button class="btn" disabled=${running} onClick=${async () => {
          if (!confirm(`Roll ${world} back to ${d.toLocaleString()}?\n\nThe current save is moved aside, not deleted.`)) return;
          await act(() => rpc("restore", { world, file: b.file }), "Rolled back.");
        }}>Roll back to this</button></div>`;
    })}</div>` : null}
`;
}
