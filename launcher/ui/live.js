// The live game: shown while Project Zomboid is open, and a recap after it closes.
import { html, useState, useEffect, S, rpc, act, toast, update } from "./core.js";

function townOf(x, y) {
  let best = "", bd = Infinity;
  for (const r of (S.presets && S.presets.towns) || []) for (const p of r.points) {
    const d = (p.x - x) ** 2 + (p.y - y) ** 2;
    if (d < bd) { bd = d; best = r.name; }
  }
  return bd > 2500 * 2500 ? "out in the country" : best.replace(", KY", "");
}

// ServerStrip: the host's server, live. The host sees its own; friends see what the host's launcher reports.
export function ServerStrip() {
  const st = S.state || {};
  const sv = st.server || {};
  const text = st.host || !st.hostServer ? (sv.running ? sv.text : sv.error) : st.hostServer;
  if (!text) return null;
  const up = st.host ? sv.stage === "up" : text === "Server is up";
  const bad = st.host && !sv.running;
  const mins = sv.up ? Math.max(0, Math.floor((Date.now() / 1000 - sv.up) / 60)) : 0;
  return html`<div class=${"server-strip" + (up ? " up" : bad ? " bad" : "")}>
    ${up ? html`<div class="live-dot"></div>` : bad ? html`<span class="sdot"></span>` : html`<div class="spinner sm"></div>`}
    <b>${st.host ? "Your server" : "Host's server"}</b>
    <span class="grow">${up ? (st.host ? `Up for ${mins} min` : "Up") : text}</span>
    ${st.host && sv.running ? html`<button class="btn sm ghost" onClick=${() => { if (confirm("Stop the server? It saves first.")) act(() => rpc("stopServer"), "Stopping the server"); }}>Stop</button>` : null}
  </div>`;
}

const time = (h) => Math.floor(h / 24) + "d " + Math.floor(h % 24) + "h";

export function Live() {
  const st = S.state;
  const g = st.game || {};
  const [live, setLive] = useState(null);
  const [msg, setMsg] = useState("");
  useEffect(() => {
    let alive = true;
    const load = async () => {
      const r = await act(() => rpc("status"));
      if (alive && r) setLive(r);
    };
    load();
    const t = setInterval(load, 3000);
    return () => { alive = false; clearInterval(t); };
  }, []);
  const running = g.running;
  const status = live && live.status;
  const fresh = status && live.now - status.time < 20;
  const hosting = g.host;
  if (!running) {
    const recap = live && live.recap;
    return html`<div class="card live-card">
      <div class="row"><h3 class="grow" style=${{ margin: 0 }}>Game closed</h3><button class="btn sm ghost" onClick=${() => update({ hideRecap: g.sessions })}>Hide</button></div>
      ${g.error ? html`<div class="chip warn" style=${{ margin: "8px 0", whiteSpace: "normal" }}>${g.error}</div>` : html`<div class="hint">You're back in the launcher. Change anything and press START for the next round.</div>`}
      ${recap && recap.players && recap.players.length ? html`<${Recap} recap=${recap} />` : null}
    </div>`;
  }
  return html`<div class="card live-card">
    <${ServerStrip} />
    <div class="row">
      <div class="live-dot"></div>
      <h3 class="grow" style=${{ margin: 0 }}>Project Zomboid is running${fresh ? ` · day ${status.day}, ${String(status.hour).padStart(2, "0")}:00 · ${status.players.length} online${status.lobby ? " · in-game lobby open" : ""}` : ""}</h3>
      ${hosting ? html`<button class="btn sm" onClick=${() => act(() => rpc("command", { cmd: "save" }), "Saving the world")}>Save now</button>` : null}
    </div>
    ${!hosting ? html`<div class="hint" style=${{ marginTop: "8px" }}>The game is joining ${st.hostName || "the host"}'s server and filling in your character. Keep this window open: when the game closes you're back in the lobby.</div>`
      : !fresh ? html`<div class="hint" style=${{ marginTop: "8px" }}>You're joining your server. Your friends' games open once you're in.</div>` : html`
      <table class="players"><tr>${["Player", "Health", "Kills", "Survived", "Where", ""].map((x) => html`<th>${x}</th>`)}</tr>
        ${status.players.map((p) => html`<tr key=${p.name}>
          <td><b>${p.display}</b><div class="dim small">${p.name}</div></td>
          <td style=${{ width: "170px" }}>${p.dead ? html`<span class="chip bad">Dead</span>` : html`<div class=${"bar " + (p.health < 35 ? "bad" : "good")}><i style=${{ width: Math.max(2, p.health) + "%" }}></i></div>`}</td>
          <td>${p.kills}</td><td>${time(p.hours)}</td><td class="muted small">${townOf(p.x, p.y)}</td>
          <td><div class="row">
            <button class="btn sm" onClick=${() => confirm("Kick " + p.name + "?") && act(() => rpc("command", { cmd: "kick", arg: p.name }), "Kicked " + p.name)}>Kick</button>
            <button class="btn sm danger" onClick=${() => confirm("Ban " + p.name + "? They can't come back.") && act(() => rpc("command", { cmd: "ban", arg: p.name }), "Banned " + p.name)}>Ban</button>
          </div></td></tr>`)}
      </table>
      <div class="row" style=${{ marginTop: "10px" }}><input type="text" class="grow" placeholder="Message everyone on the server" value=${msg} onInput=${(e) => setMsg(e.target.value)}
        onKeyDown=${(e) => { if (e.key === "Enter" && msg.trim()) { act(() => rpc("command", { cmd: "msg", arg: msg }), "Sent"); setMsg(""); } }} />
        <button class="btn" onClick=${() => { if (msg.trim()) { act(() => rpc("command", { cmd: "msg", arg: msg }), "Sent"); setMsg(""); } }}>Send</button></div>`}
  </div>`;
}

function Recap({ recap }) {
  const players = recap.players.slice().sort((a, b) => b.kills - a.kills);
  const mins = Math.max(1, Math.round((recap.updated - recap.started) / 60));
  const total = players.reduce((n, p) => n + p.kills, 0);
  return html`<div style=${{ marginTop: "12px" }}>
    <div class="group-title">${mins} min · day ${recap.day} · ${total} zombies down</div>
    <div class="recap">${players.map((p, i) => html`<div key=${p.name} class="card">
      <div class="row"><b style=${{ fontSize: "16px" }}>${p.name}</b>${i === 0 && p.kills ? html`<span class="chip accent">Top killer</span>` : null}</div>
      <div class="big">${p.kills}</div><div class="muted small">kills · ${p.deaths} deaths · ${time(p.hours)}</div>
    </div>`)}</div></div>`;
}
