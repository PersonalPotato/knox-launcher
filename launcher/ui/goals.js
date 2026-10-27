// Knox goals: what the squad and each player are going for in this world (Knox Link counts them on the
// host's server). Finished goals give launcher XP; story goals give a badge.
import { html, S } from "./core.js";

const KIND = { squad: "Squad goal", personal: "Personal", long: "Long haul", story: "Story goal" };

// lasting: only the goals that carry over (before a session starts, squad and personal ones aren't picked yet)
export function GoalsCard({ goals, title = "Goals", lasting }) {
  if (!goals || !goals.goals) return null;
  const all = goals.goals.filter((g) => g && (!lasting || ((g.kind === "long" || g.kind === "story") && !g.done)));
  if (!all.length) return null;
  const me = String((S.state && S.state.me && S.state.me.name) || "").toLowerCase();
  const order = { squad: 0, story: 1, personal: 2, long: 3 };
  const list = all.slice().sort((a, b) => (order[a.kind] - order[b.kind]) || ((a.user || "").toLowerCase() === me ? -1 : 1));
  return html`<div class="card goals-card">
    <div class="row"><h3 class="grow" style=${{ margin: 0 }}>${title}</h3><span class="dim small">${goals.difficulty === "hard" ? "Hard world: easier goals, more XP" : goals.difficulty === "easy" ? "Easy world: bigger goals" : ""}</span></div>
    ${list.map((g) => html`<div key=${g.id + g.user} class=${"goal" + (g.done ? " done" : "")}>
      <div class="goal-top"><span class="goal-kind">${g.kind === "personal" ? (g.user && g.user.toLowerCase() !== me ? g.user : "Yours") : KIND[g.kind] || g.kind}</span>
        <b class="grow">${g.name}</b><span class="goal-xp">${g.done ? "✓ " : ""}+${g.xp} XP</span></div>
      <div class="goal-text">${g.text}</div>
      ${g.need > 1 ? html`<div class="bar"><i style=${{ width: Math.min(100, Math.round((100 * g.have) / g.need)) + "%" }}></i></div><div class="dim small">${g.have} / ${g.need}</div>` : null}
    </div>`)}
    ${lasting ? html`<div class="dim small" style=${{ marginTop: "6px" }}>A squad goal and one goal each get picked when the game starts.</div>` : null}
  </div>`;
}

// trackLine: a few of Knox Link's counts for one player in a recap
export function trackLine(t) {
  if (!t) return "";
  const out = [];
  if (t.walk) out.push(`${(t.walk / 1000).toFixed(1)} km on foot`);
  if (t.drive) out.push(`${(t.drive / 1000).toFixed(1)} km driven`);
  if (t.bites) out.push(`${t.bites} bite${t.bites > 1 ? "s" : ""}`);
  if (t.bandOthers) out.push(`${t.bandOthers} bandages for friends`);
  if (t.crafted) out.push(`${t.crafted} things made`);
  if (t.sleepMins) out.push(`${Math.round(t.sleepMins / 60)} h asleep`);
  return out.join(" · ");
}
