// "Previously on...": who's in a world you're continuing. Alive, dead, out of lives, THE END.
import { html, S } from "./core.js";

const myName = () => (S.state && S.state.me && S.state.me.name) || "";

export function castMe(cast) {
  const me = myName().toLowerCase();
  return cast && (cast.cast || []).find((m) => (m.user || "").toLowerCase() === me);
}

function livesLine(m, cast) {
  if (!cast.lives) return "";
  if (m.livesLeft === 0) return cast.livesMode === "squad" ? "the squad has no lives left" : "no lives left";
  return m.livesLeft + (cast.livesMode === "squad" ? " squad lives left" : m.livesLeft === 1 ? " life left" : " lives left");
}

// YourSurvivor: one line about you in this tape. What PZ will do when you join.
export function yourSurvivor(cast) {
  if (!cast) return null;
  const m = castMe(cast);
  if (!m) return "You haven't played this tape. You'll make a survivor and start next to the squad.";
  if (m.out) return "You're out of lives in this tape.";
  if (m.alive) return `You load back in as ${m.name}, where you left off.`;
  const lives = livesLine(m, cast);
  return `${m.name} died. You'll make a new survivor` + (lives ? ` (${lives}).` : ".");
}

export function PreviouslyOn({ cast, world }) {
  if (!cast) return null;
  const list = [...(cast.cast || [])].sort((a, b) => (b.alive - a.alive) || (b.kills - a.kills));
  const alive = list.filter((m) => m.alive).length;
  const deaths = list.flatMap((m) => (m.died || []).map((d) => ({ ...d, user: m.user }))).sort((a, b) => b.day - a.day);
  const status = (m) => (m.out ? "OUT" : m.alive ? "ALIVE" : "DEAD");
  return html`<div class=${"card prev-on" + (cast.ended ? " ended" : "")}>
    <h3>${cast.ended ? "THE END" : "Previously on..."}</h3>
    <div class="muted small">${world || cast.world} · day ${cast.day} · ${alive} alive · ${deaths.length} death${deaths.length === 1 ? "" : "s"}${cast.lives ? ` · ${cast.lives} ${cast.livesMode === "squad" ? "squad lives" : "lives each"}` : ""}</div>
    ${cast.ended ? html`<div class="warn-text" style=${{ margin: "8px 0" }}>Everyone is out. This tape is over, so pick another world or start a new one.</div>`
      : html`<div style=${{ margin: "8px 0" }}><b>${yourSurvivor(cast)}</b></div>`}
    <div class="prev-cast">${list.map((m) => html`<div key=${m.user} class="setup-row"><span class=${"chip" + (m.alive ? " good" : m.out ? " bad" : "")}>${status(m)}</span>
      <div><b>${m.name}</b><small>${m.user} · ${m.kills} kills · day ${m.firstDay}-${m.lastDay}${livesLine(m, cast) ? " · " + livesLine(m, cast) : ""}</small></div></div>`)}</div>
    ${deaths.length ? html`<div class="small muted" style=${{ marginTop: "8px" }}>${deaths.slice(0, 5).map((d, i) => html`<div key=${i}>Day ${d.day}: ${d.name} died (${d.kills} kills)</div>`)}</div>` : null}
  </div>`;
}
