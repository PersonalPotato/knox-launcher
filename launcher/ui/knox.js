// Knox level, badges, funny nicknames and squad names. Looks only.
import { html, useEffect, useState, S, update, rpc, act, toast } from "./core.js";
import { LooksPanel } from "./looks.js";

// seeded random, so a poster stays the same while it's on screen
export function seeded(seed) {
  let h = 2166136261;
  for (const ch of String(seed)) h = Math.imul(h ^ ch.charCodeAt(0), 16777619) >>> 0;
  return () => {
    h = Math.imul(h ^ (h >>> 15), 2246822507) >>> 0;
    h = Math.imul(h ^ (h >>> 13), 3266489909) >>> 0;
    return ((h ^= h >>> 16) >>> 0) / 4294967296;
  };
}

export function knoxOf(m) {
  if (!m) return null;
  if (m.me && S.knox) return { lvl: S.knox.lvl, title: S.knox.title, badges: S.knox.badges, xp: S.knox.xp };
  return m.knox || null;
}

export async function loadKnox() {
  const r = await act(() => rpc("knox"));
  if (r) update({ knox: r.me, knoxDefs: r });
}

// Nicknames -----------------------------------------------------------------------------------------

// From job and traits, before anyone has stats (and as extras after).
const FROM_TRAIT = {
  smoker: "Chain Smoker", clumsy: "Butterfingers", cowardly: "Scaredy Cat", heartyappetite: "Bottomless Pit",
  insomniac: "Insomniac", needsmoresleep: "Sleeping Beauty", fastlearner: "Teacher's Pet", slowlearner: "Bit Slow",
  illiterate: "Can't Read Good", shortsighted: "Mr. Magoo", hardofhearing: "Huh?", unfit: "Couch Potato",
  weak: "Noodle Arms", strong: "The Fridge", stout: "The Tank", brave: "Leeroy", agoraphobic: "Homebody",
  claustrophobic: "Open Plan", hemophobic: "Fainting Goat", lucky: "Lucky Idiot", unlucky: "The Jinx",
  conspicuous: "Dinner Bell", inconspicuous: "Ghost", organized: "Hoarder", disorganized: "Junk Drawer",
  outdoorsman: "Mountain Man", hiker: "Trail Mix", cook: "Fry Cook", lowthirst: "Camel", highthirst: "Thirsty",
  pronetoillness: "Patient Zero", slowhealer: "Paper Cut", jogger: "Last Seen Running", sundaydriver: "Sunday Driver",
  speeddemon: "Crash Test Dummy", firstaid: "Doc", weakstomach: "Food Poisoning", graceful: "Ballerina",
  keenhearing: "Big Ears", nightowl: "Night Owl", thickskinned: "Punching Bag", thinskinned: "Snack Pack",
  fishing: "Fisherman Fred", herbalist: "Squirrel", handy: "Handyman", athletic: "Marathon",
};
const FROM_JOB = {
  unemployed: "The Spare", burglar: "Sticky Fingers", fitnessinstructor: "Gym Rat", veteran: "Grandpa War Stories",
  burgerflipper: "Fry Cook", policeofficer: "Officer Doughnut", fireofficer: "Hose Head", doctor: "Dr. Feelgood",
  nurse: "Needles", carpenter: "Handyman", mechanics: "Grease Monkey", parkranger: "Smokey", securityguard: "Mall Cop",
  lumberjack: "Lumberjack Larry", fisherman: "Fisherman Fred", farmer: "Old McDonald", electrician: "Sparky",
  engineer: "Boom Boom", constructionworker: "Hard Hat", repairman: "Duct Tape", metalworker: "Tin Man",
  tailor: "Needles", chef: "Spice Girl", smither: "Hammer Time", rancher: "Cowpoke", blacksmith: "Hammer Time",
  traditional_tailor: "Needles", carpenter2: "Handyman",
};

// nicknames gives every member one funny nickname: stats first, then job and traits. No repeats.
export function nicknames(members, squad, seed) {
  const rnd = seeded(seed);
  const st = (m) => ((squad && squad.players) || {})[m.id] || {};
  const list = members.map((m) => ({ m, s: st(m), c: m.me ? S.cfg.character : (m.char && m.char.c) || null }));
  const withStats = list.filter((x) => x.s.games > 0);
  const top = (key, pick = Math.max) => {
    const vals = withStats.map((x) => x.s[key] || 0);
    if (withStats.length < 2) return null;
    const v = pick(...vals);
    return vals.filter((x) => x === v).length === 1 ? v : null; // only a clear winner earns it
  };
  const lvl = (x) => (knoxOf(x.m) || {}).lvl || 1;
  const rules = [
    ["Zombie Bait", (x) => x.s.deaths > 0 && x.s.deaths === top("deaths")],
    ["Speedrun", (x) => x.s.speedruns > 0],
    ["The Butcher", (x) => x.s.kills > 0 && x.s.kills === top("kills")],
    ["Lawnmower", (x) => x.s.bestKills >= 20 && x.s.bestKills === top("bestKills")],
    ["Cockroach", (x) => x.s.longest >= 24 && x.s.longest === top("longest")],
    ["The Pacifist", (x) => x.s.games > 0 && x.s.deaths === 0 && x.s.kills === top("kills", Math.min)],
    ["Couch Potato", (x) => x.s.games > 0 && x.s.hours === top("hours", Math.min) && x.s.hours < 2],
    ["The Spare", (x) => list.length > 1 && lvl(x) === Math.min(...list.map(lvl)) && list.filter((y) => lvl(y) === lvl(x)).length === 1],
    ["New Guy", (x) => !x.s.games && withStats.length > 0],
  ];
  const out = {};
  const used = new Set();
  // stat nicknames first, in rule order (the funniest first)
  for (const [name, test] of rules) {
    const fits = list.filter((x) => !out[x.m.id] && test(x));
    if (!fits.length || used.has(name)) continue;
    const x = fits[Math.floor(rnd() * fits.length)];
    out[x.m.id] = name;
    used.add(name);
  }
  // then job and traits, random among the ones that fit
  for (const x of list) {
    if (out[x.m.id]) continue;
    const c = x.c || {};
    const opts = [...(c.traits || []).map((t) => FROM_TRAIT[t]), FROM_JOB[c.profession]].filter((n) => n && !used.has(n));
    const name = opts.length ? opts[Math.floor(rnd() * opts.length)] : ["Plus One", "Who Invited You", "Background Extra", "Red Shirt", "The Intern"].find((n) => !used.has(n)) || "Survivor";
    out[x.m.id] = name;
    used.add(name);
  }
  return out;
}

// Squad names -----------------------------------------------------------------------------------------

const NOUNS = ["Misfits", "Mall Rats", "Stragglers", "Leftovers", "Night Crew", "Last Picks", "Wrong Turns", "Bad Ideas", "Snack Pack", "Spare Parts",
  "Lost Causes", "Late Shift", "Loose Ends", "Dead Weight", "Reruns", "Bite Club", "Couch Militia", "Canned Goods", "Door Slammers", "Window Breakers",
  "Noise Makers", "Car Alarms", "Shopping Carts", "Expired Coupons", "Lawn Gnomes", "Beans Brigade", "Leaky Roofs", "Dinner Bells", "Screen Doors",
  "Wet Socks", "Flat Tires", "Spoiled Milk", "Fire Hazards", "Weekend Warriors", "Garage Band", "Bake Sale", "Book Club", "Neighbourhood Watch",
  "Fun Police", "Group Chat", "Carpool", "Night Owls", "Couch Potatoes", "Raccoons", "Possums", "Pigeons", "Seagulls", "Goldfish", "Hamsters",
  "Ducklings", "Clowns", "Interns", "Temps", "Substitutes", "Understudies", "Extras", "Stunt Doubles", "Mascots", "Benchwarmers", "Water Boys",
  "Snack Thieves", "Loot Goblins", "Pantry Raiders", "Fridge Raiders", "Bin Divers", "Trash Pandas", "Tin Can Army", "Sock Puppets", "Paper Tigers",
  "Glass Cannons", "Speed Bumps", "Traffic Cones", "Dead Batteries", "Empty Tanks", "Busted Flashlights", "Broken Compasses", "Wrong Keys",
  "Missing Persons", "Lost Property", "Cold Cases", "Late Fees", "Overdrafts", "Parking Tickets", "Food Poisoning", "Cabin Fever", "Bad Luck Club"];
const ADJ = ["Unkillable", "Mostly Alive", "Underfed", "Very Lost", "Barely", "Doomed", "Unlucky", "Overconfident", "Slightly Bitten", "Sleep Deprived",
  "Underqualified", "Unemployed", "Unwashed", "Undercooked", "Lukewarm", "Hungover", "Panicking", "Tactical", "Professional", "Amateur",
  "Fashionably Late", "Extremely Loud", "Badly Organised", "Half Dead", "Mildly Infected", "Very Brave", "Not Very Brave", "Easily Startled",
  "Crunchy", "Soggy", "Expired", "Discount", "Off Brand", "Budget", "Bargain Bin", "Second Hand", "Free Range", "Low Fat", "Gluten Free", "Unsalted"];
// whole names, no town needed
const WHOLE = ["Zombie Bait", "The Walking Snacks", "Shaun's Dead Mates", "The Knox Event Survivors (For Now)", "Dead Men Walking Slowly",
  "The Last Ones Picked", "Brains Not Included", "Ten Cans of Beans", "No Running Indoors", "Please Don't Bite", "The Undead Unprepared",
  "Fresh Meat Inc.", "Dawn of the Bread", "The Fast and the Curious", "Night of the Living Dads", "28 Days Later-ish", "Weekend at Bitey's",
  "The Breakfast Club Sandwiches", "Kentucky Fried Survivors", "Louisville Slugger Fan Club", "The Muldraugh Mafia", "Rosewood Retirement Home",
  "West Point Wanderers", "Riverside Rejects", "March Ridge Mall Walkers", "Ballistic Grandmas", "Molotov Book Club", "Axe Wound Appreciation Society",
  "Sledgehammer Social", "The Spiffo Fan Club", "Spiffo's Last Customers", "The Bandage Boys", "The Glue Sniffers", "The Generator Whisperers",
  "Duct Tape Division", "The Rain Barrel Crew", "Canned Laughter", "The Peanut Butter Pact", "Team Hide in the Bathroom", "The Barricade Botchers",
  "Plank Specialists", "Tin Foil Hats", "The Sprinter Disappointers", "Horde Magnets", "Professional Screamers", "Scream Team", "Dead Ringers",
  "The Infected Influencers", "Survival of the Dumbest", "Mostly Harmless", "The Wrong Turn Club", "Out of Toilet Paper", "We Forgot the Can Opener"];

const shortTown = (t) => (t || "").replace(/, *[A-Z]{2}$/, "");

export function squadName(info, seed) {
  if (info && info.squadName) return info.squadName;
  return makeSquadName(info, seed || "squad");
}

// makeSquadName invents one (the same seed gives the same name).
export function makeSquadName(info, seed) {
  const rnd = seeded(seed);
  const pick = (a) => a[Math.floor(rnd() * a.length)];
  const town = shortTown(info && info.spawn && info.spawn.name);
  const story = info && S.presets && (S.presets.backstories || []).find((b) => b.id === info.backstory);
  const noun = pick(NOUNS);
  const r = rnd();
  if (story && town && r < 0.15) return `${story.name}, ${town}`;
  if (town && r < 0.4) return `The ${town} ${noun}`;
  if (r < 0.65) return pick(WHOLE);
  return `The ${pick(ADJ)} ${noun}`;
}

// Poster frame by level (K11): the squad's best level picks the border.
export function frameFor(level) {
  if (level >= 20) return "gold";
  if (level >= 12) return "silver";
  if (level >= 5) return "bronze";
  return "";
}

// UI ------------------------------------------------------------------------------------------------

export function LevelTag({ k, small }) {
  if (!k) return null;
  return html`<span class=${"lvl-tag" + (small ? " sm" : "")} title=${(k.xp || 0) + " XP"}>Lv ${k.lvl} · ${k.title}</span>`;
}

// NavLevel sits in the sidebar: level, title and an XP bar. Click for the full profile.
export function NavLevel() {
  useEffect(() => { loadKnox(); const t = setInterval(loadKnox, 60000); return () => clearInterval(t); }, [S.cfgRev]);
  const k = S.knox;
  if (!k) return null;
  const pct = Math.round((100 * (k.xp - k.base)) / Math.max(1, k.next - k.base));
  return html`<button class="nav-level" onClick=${() => update({ modal: { kind: "knox" } })} title="Your Knox level">
    <span class="nl-badge">${k.lvl}</span>
    <span class="nl-txt"><b>${k.title}</b><small>${k.xp} / ${k.next} XP</small><i class="nl-bar"><i style=${{ width: pct + "%" }}></i></i></span>
  </button>`;
}

const ICON = { firstnight: "☾", weekone: "7", sixmonths: "6", hordebreaker: "✕", neveralone: "♥", packrat: "▣", hostmost: "★", recruiter: "+", everystory: "✎", speedrun: "☠", fieldmedic: "✚", roadtrip: "➜" };

export function BadgeShelf({ have, defs, compact }) {
  const all = (defs || (S.knoxDefs && S.knoxDefs.badges) || []);
  const got = new Set(have || []);
  const list = compact ? all.filter((b) => got.has(b.id)) : all;
  if (compact && !list.length) return html`<div class="muted small">No badges yet.</div>`;
  return html`<div class="kbadges">${list.map((b) => html`<div key=${b.id} class=${"kbadge" + (got.has(b.id) ? " got" : "")} title=${b.how + (b.soon ? " (comes with the next Knox Link update)" : "")}>
    <span class="b-ico">${ICON[b.id] || "•"}</span><b>${b.name}</b>${compact ? null : html`<small>${b.how}${b.soon && !got.has(b.id) ? " · soon" : ""}</small>`}
  </div>`)}</div>`;
}

// KnoxWindow: your level, where your XP came from, titles and badges.
export function KnoxWindow() {
  useEffect(() => { loadKnox(); }, []);
  const k = S.knox, d = S.knoxDefs;
  if (!k || !d) return html`<div class="hint">Loading...</div>`;
  const pct = Math.round((100 * (k.xp - k.base)) / Math.max(1, k.next - k.base));
  return html`<div class="knox-win">
    <div class="kw-head"><span class="kw-lvl">${k.lvl}</span><div class="grow"><h3>${k.title}</h3>
      <div class="kw-bar"><i style=${{ width: pct + "%" }}></i></div><div class="muted small">${k.xp} XP · ${k.next - k.xp} to level ${k.lvl + 1}</div></div></div>
    <div class="kw-cols">
      <div><h4>Where your XP came from</h4>
        ${(k.sources || []).length ? html`<div class="kw-src">${k.sources.map((s) => html`<div key=${s.what}><span>${s.what}</span><b>+${s.xp}</b></div>`)}</div>` : html`<div class="muted small">Play, host, invite friends and share packs to earn XP.</div>`}
        <h4>Titles</h4><div class="kw-titles">${d.titles.map((t) => html`<span key=${t.Level} class=${"chip" + (k.lvl >= t.Level ? " accent" : "")}>Lv ${t.Level} ${t.Name}</span>`)}</div>
        <div class="muted small" style=${{ marginTop: "8px" }}>Levels unlock tape looks (below). They never change the game.</div>
      </div>
      <div><h4>Badges · ${(k.badges || []).length} of ${d.badges.length}</h4><${BadgeShelf} have=${k.badges} /></div>
    </div>
    <h3 class="looks-head">Your tape</h3>
    <${LooksPanel} />
  </div>`;
}
