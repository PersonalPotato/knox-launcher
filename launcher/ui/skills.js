// Sorting for the character page: trait groups and skills in the game's own order.
import { html } from "./core.js";

// skills in the order the game's skill screen shows them: [perk id, English name, group]
const SKILLS = [
  ["Fitness", "Fitness", "Body"], ["Strength", "Strength", "Body"],
  ["Sprinting", "Running", "Agility"], ["Lightfoot", "Lightfooted", "Agility"], ["Nimble", "Nimble", "Agility"], ["Sneak", "Sneaking", "Agility"],
  ["Axe", "Axe", "Melee"], ["Blunt", "Long Blunt", "Melee"], ["SmallBlunt", "Short Blunt", "Melee"], ["LongBlade", "Long Blade", "Melee"],
  ["SmallBlade", "Short Blade", "Melee"], ["Spear", "Spear", "Melee"], ["Maintenance", "Maintenance", "Melee"],
  ["Aiming", "Aiming", "Firearms"], ["Reloading", "Reloading", "Firearms"],
  ["Woodwork", "Carpentry", "Crafting"], ["Carving", "Carving", "Crafting"], ["Cooking", "Cooking", "Crafting"], ["Electricity", "Electrical", "Crafting"],
  ["Glassmaking", "Glassmaking", "Crafting"], ["FlintKnapping", "Knapping", "Crafting"], ["Masonry", "Masonry", "Crafting"], ["Mechanics", "Mechanics", "Crafting"],
  ["Pottery", "Pottery", "Crafting"], ["Blacksmith", "Blacksmithing", "Crafting"], ["Melting", "Smelting", "Crafting"], ["Tailoring", "Tailoring", "Crafting"],
  ["MetalWelding", "Welding", "Crafting"],
  ["Farming", "Agriculture", "Farming"], ["Husbandry", "Animal Care", "Farming"], ["Butchering", "Butchering", "Farming"],
  ["Doctor", "First Aid", "Survival"], ["Fishing", "Fishing", "Survival"], ["PlantScavenging", "Foraging", "Survival"],
  ["Tracking", "Tracking", "Survival"], ["Trapping", "Trapping", "Survival"],
];
const SKILL_GROUPS = ["Body", "Agility", "Melee", "Firearms", "Crafting", "Farming", "Survival", "Other"];
const skillByKey = {};
SKILLS.forEach(([id, name, group], i) => {
  const s = { id, name, group, order: i };
  skillByKey[id.toLowerCase()] = s;
  skillByKey[name.toLowerCase()] = s;
});

// skill info for an xp entry; works for both the game export (names only) and the file read (ids too)
export function skillOf(x) {
  return skillByKey[String(x.id || "").toLowerCase()] || skillByKey[String(x.perk || "").toLowerCase()] || { id: x.id || x.perk, name: x.perk || x.id, group: "Other", order: 999 };
}

// Skills: what the job and traits add, grouped and in game order.
export function SkillList({ xp }) {
  const rows = Object.values(xp).filter((s) => s.level !== 0);
  if (!rows.length) return null;
  rows.sort((a, b) => a.order - b.order || a.name.localeCompare(b.name));
  return html`<div class="skills">
    ${SKILL_GROUPS.map((g) => {
      const list = rows.filter((s) => s.group === g);
      return list.length ? html`<div key=${g} class="skill-group">
        <div class="skill-head">${g}</div>
        ${list.map((s) => html`<div key=${s.id} class=${"skill" + (s.level < 0 ? " neg" : "")}>
          <span class="sk-name">${s.name}</span>
          <span class="pips">${Array.from({ length: 10 }, (_, i) => html`<i class=${i < Math.abs(s.level) ? "on" : ""}></i>`)}</span>
          <span class="sk-lv">${s.level > 0 ? "+" : ""}${s.level}</span>
        </div>`)}
      </div>` : null;
    })}
  </div>`;
}

// trait groups for the game's own traits; mod traits are grouped by mod, others by what they boost
const TRAIT_GROUPS = {
  Body: "athletic fit stout strong feeble weak unfit out obese overweight underweight very emaciated graceful clumsy fasthealer slowhealer thickskinned thinskinned resilient pronetoillness asthmatic smoker",
  Mind: "brave cowardly adrenalinejunkie agoraphobic claustrophobic hemophobic pacifist desensitized fastlearner slowlearner fastreader slowreader illiterate",
  Senses: "eagleeyed shortsighted keenhearing hardofhearing deaf nightvision inconspicuous conspicuous",
  "Food & sleep": "heartyappetite lighteater highthirst lowthirst irongut weakstomach weightgain weightloss nutritionist needslesssleep needsmoresleep insomniac nightowl",
  "Hands & gear": "allthumbs dextrous organized disorganized crafty inventive speeddemon sundaydriver outdoorsman",
};
const traitGroupById = {};
for (const [g, ids] of Object.entries(TRAIT_GROUPS)) for (const id of ids.split(" ")) traitGroupById[id] = g;
const TRAIT_ORDER = ["Body", "Mind", "Senses", "Food & sleep", "Hands & gear", "Skills", "Other"];

export function traitGroup(t) {
  const g = traitGroupById[String(t.id).toLowerCase()];
  if (g) return g;
  if (t.mod) return t.mod;
  if ((t.xp || []).length) return "Skills";
  return "Other";
}

// sortTraits: list of [heading, traits]. by = "group" | "cost" | "name". Heading is "" when not grouped.
export function sortTraits(list, by) {
  const byName = (a, b) => a.name.localeCompare(b.name);
  const byCost = (a, b) => Math.abs(b.cost) - Math.abs(a.cost) || byName(a, b);
  if (by === "name") return [["", list.slice().sort(byName)]];
  if (by === "cost") return [["", list.slice().sort(byCost)]];
  const groups = {};
  for (const t of list) (groups[traitGroup(t)] = groups[traitGroup(t)] || []).push(t);
  const rank = (g) => { const i = TRAIT_ORDER.indexOf(g); return i < 0 ? TRAIT_ORDER.length - 1.5 : i; }; // mods sit before Other
  return Object.keys(groups)
    .sort((a, b) => rank(a) - rank(b) || a.localeCompare(b))
    .map((g) => [g, groups[g].sort(byName)]);
}
