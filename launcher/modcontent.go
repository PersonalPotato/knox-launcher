package main

import "strings"

// Extra stories for big Workshop mods. A story with Mod set only unlocks when that mod is
// installed; until then it's listed under "needs another mod" with a Workshop search link.

type ModPack struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Adds      string   `json:"adds"`
	Search    string   `json:"search"` // Workshop search text
	Match     []string `json:"-"`      // lowercase bits of a mod id or name
	Installed bool     `json:"installed"`
}

var modPacks = []ModPack{
	{ID: "alife", Name: "Project A-Life", Adds: "NPC survivor groups. 2 stories where you're not the only ones left.", Search: "Project A-Life", Match: []string{"projectalife", "project a-life"}},
	{ID: "weekone", Name: "Week One", Adds: "Starts before the outbreak. 2 stories that begin on a normal day.", Search: "Week One", Match: []string{"weekone", "week one"}},
	{ID: "bandits", Name: "Bandits", Adds: "Hostile human raiders. 1 story about holding out against them.", Search: "Bandits", Match: []string{"bandits"}},
	{ID: "authenticz", Name: "Authentic Z", Adds: "Hundreds of outfits. 2 costume stories.", Search: "Authentic Z", Match: []string{"authenticz", "authentic z"}},
	{ID: "truemusic", Name: "True Music", Adds: "Real music on radios and players. 1 band story.", Search: "True Music", Match: []string{"truemusic", "true music"}},
	{ID: "ehe", Name: "Expanded Helicopter Events", Adds: "Army, news and raider helicopters. 1 missed-evac story.", Search: "Expanded Helicopter Events", Match: []string{"expandedhelicopter", "expanded helicopter"}},
	{ID: "ravencreek", Name: "Raven Creek", Adds: "A big city map. 2 stories set there.", Search: "Raven Creek", Match: []string{"ravencreek", "raven creek"}},
	{ID: "redstone", Name: "Fort Redstone", Adds: "A military fort map. 1 story set there.", Search: "Fort Redstone", Match: []string{"redstone"}},
	{ID: "bedford", Name: "Bedford Falls", Adds: "A small town map. 1 story set there.", Search: "Bedford Falls", Match: []string{"bedford"}},
	{ID: "eerie", Name: "Eerie Country", Adds: "A creepy county map. 1 story set there.", Search: "Eerie Country", Match: []string{"eerie"}},
	{ID: "moretraits", Name: "More Traits", Adds: "Dozens of extra traits. They show up on the Character page.", Search: "More Traits", Match: []string{"moretraits", "more traits"}},
	{ID: "dynamictraits", Name: "Dynamic Traits", Adds: "Traits you gain and lose while playing. They show up on the Character page.", Search: "Dynamic Traits", Match: []string{"dynamictraits", "dynamic traits"}},
	{ID: "soto", Name: "Simple Overhaul: Traits and Occupations", Adds: "Extra jobs and traits. They show up on the Character page.", Search: "Simple Overhaul Traits Occupations", Match: []string{"soto", "simple overhaul"}},
}

var modStories = []Backstory{
	{ID: "alife-notalone", Name: "Not Alone", Mod: "alife", Min: 1, Max: 6, Building: "house", Loadout: "basics", Tags: []string{"town"},
		Story: "There are other survivors out there. Some will trade with you. Some won't.",
		Roles: []Role{r("Talker", "unemployed", "Does the talking.", "inconspicuous", "fastlearner", "cowardly"), r("Muscle", "securityguard", "Stands behind the talker.", "strong", "slowreader", "illiterate"), r("Medic", "nurse", "Everyone needs one.", "firstaid"), r("Scout", "parkranger", "Sees them first.", "eagleeyed", "smoker")},
		Tip:   "Normal start. A-Life fills the world with NPC groups."},
	{ID: "alife-camp", Name: "The Camp", Mod: "alife", Min: 2, Max: 8, Building: "camp", Loadout: "campers", Tags: []string{"wild"},
		Story: "A survivor camp took you in. Now you pull your weight or you leave.",
		Roles: []Role{r("Forager", "parkranger", "Brings back food.", "herbalist", "agoraphobic"), r("Guard", "veteran", "Watches the gate at night.", "brave", "insomniac"), r("Builder", "carpenter", "Keeps the walls up.", "handy", "smoker", "unfit"), r("Cook", "chef", "Feeds everyone.", "cook", "heartyappetite"), r("Trader", "unemployed", "Deals with outsiders.", "organized", "cowardly")},
		Tip:   "Summer camp start. Good with A-Life groups around."},
	{ID: "weekone-normal", Name: "Just Another Monday", Mod: "weekone", Min: 1, Max: 8, Building: "house", Loadout: "none", Tags: []string{"town", "long"},
		Story: "It's a normal day. The news is talking about a flu going around Louisville.",
		Roles: []Role{r("Prepper", "veteran", "Already filling the bathtub.", "organized", "agoraphobic", "insomniac"), r("Cop", "policeofficer", "Getting called in on a day off.", "brave", "smoker", "insomniac"), r("Nurse", "nurse", "Hospital's been busy.", "firstaid", "needsmoresleep"), r("Looter", "burglar", "Sees an opportunity.", "inconspicuous", "conspicuous"), r("Shop Clerk", "unemployed", "Selling out of canned food.", "fastlearner", "unfit")},
		Tip:   "Week One starts before the outbreak. Get ready while you can."},
	{ID: "weekone-riot", Name: "Riot", Mod: "weekone", Min: 2, Max: 6, Building: "store", Loadout: "scavenger", Tags: []string{"town", "hard"},
		Story: "Day three. The cops are gone and people are looting the stores. You're one of them.",
		Roles: []Role{r("Ringleader", "burglar", "Knows which stores have alarms.", "inconspicuous"), r("Getaway Driver", "mechanics", "Engine running.", "speeddemon", "smoker"), r("Heavy", "constructionworker", "Breaks the glass.", "strong", "illiterate", "slowlearner"), r("Lookout", "unemployed", "Watches for cops.", "eagleeyed", "keenhearing", "cowardly", "smoker", "insomniac")},
		Tip:   "Store start in the middle of the panic."},
	{ID: "bandits-holdout", Name: "Holdout", Mod: "bandits", Min: 2, Max: 6, Building: "farm", Loadout: "hunters", Tags: []string{"hard", "long"},
		Story: "The dead are slow. The raiders aren't. You've got a farm worth taking.",
		Roles: []Role{r("Owner", "farmer", "Not leaving.", "brave", "gardener", "insomniac"), r("Shooter", "veteran", "On the roof.", "target_shooter", "hardofhearing"), r("Builder", "carpenter", "Walls first.", "handy", "smoker", "unfit"), r("Medic", "nurse", "Pulls out bullets.", "firstaid", "needsmoresleep")},
		Tip:   "Farm start with shotguns. Bandits will come."},
	{ID: "authz-halloween", Name: "Halloween Party", Mod: "authenticz", Min: 2, Max: 8, Building: "bighouse", Loadout: "partypack", Tags: []string{"easy"},
		Story: "Costume party at a big house out of town. Everyone thought the first zombie was a really good costume.",
		Roles: []Role{r("Host", "unemployed", "Dressed as a vampire.", "organized", "conspicuous"), r("Clown", "unemployed", "Nobody likes the clown.", "graceful", "conspicuous"), r("Fake Cop", "unemployed", "The badge is plastic.", "brave", "unfit"), r("Mascot", "fitnessinstructor", "Can't see out of the head.", "athletic", "shortsighted", "conspicuous", "slowreader", "illiterate"), r("Zombie Costume", "unemployed", "Bad timing.", "inconspicuous", "cowardly")},
		Tip:   "Dress up with Authentic Z outfits before you start."},
	{ID: "authz-convention", Name: "Comic Con", Mod: "authenticz", Min: 2, Max: 8, Building: "school", Loadout: "none", Tags: []string{"town"},
		Story: "The county fan convention was in the school gym this year. Bad weekend for it.",
		Roles: []Role{r("Cosplayer", "tailor", "Made their own armor.", "tailor", "weak"), r("Vendor", "unemployed", "Sells comics.", "organized", "unfit"), r("Security", "securityguard", "Rented polo shirt.", "brave", "smoker"), r("Fan", "unemployed", "Knows exactly what to do. Saw it in a movie.", "fastlearner", "cowardly", "conspicuous")},
		Tip:   "School start. Costumes from Authentic Z."},
	{ID: "truemusic-band", Name: "Band on Tour", Mod: "truemusic", Min: 2, Max: 5, Building: "bar", Loadout: "partypack", Tags: []string{"town"},
		Story: "Third show of the tour. The crowd started biting halfway through the set.",
		Roles: []Role{r("Singer", "unemployed", "Loud.", "graceful", "conspicuous", "smoker"), r("Guitarist", "unemployed", "Won't leave the guitar.", "dextrous", "smoker", "needsmoresleep"), r("Drummer", "unemployed", "Hits things for a living.", "strong", "hardofhearing"), r("Roadie", "electrician", "Keeps the amps working.", "tinkerer", "stout", "illiterate", "smoker"), r("Manager", "unemployed", "Owes everyone money.", "organized", "cowardly")},
		Tip:   "Bar start. Find a tape player."},
	{ID: "ehe-evac", Name: "Missed the Evac", Mod: "ehe", Min: 1, Max: 6, Building: "school", Loadout: "basics", Tags: []string{"town", "hard"},
		Story: "The army set up an evac point at the school. The last chopper took off while you were running across the field.",
		Roles: []Role{r("Soldier", "veteran", "Got left with the civilians.", "brave", "smoker"), r("Pilot", "mechanics", "Crashed nearby. Long story.", "speeddemon", "slowhealer"), r("Reporter", "unemployed", "Still filming.", "fastreader", "inconspicuous", "cowardly"), r("Civilian", "unemployed", "Just wants to get out.", "jogger", "cowardly")},
		Tip:   "Helicopters come by often with this mod. Not all of them help."},
	{ID: "raven-pd", Name: "Raven Creek PD", Mod: "ravencreek", Min: 2, Max: 6, Building: "police", Town: "Raven", Loadout: "cops", Tags: []string{"town"},
		Story: "Big city, small police force. The station is the only place with locks that work.",
		Roles: []Role{r("Detective", "policeofficer", "Working a missing persons case. Lots of them.", "eagleeyed", "smoker", "insomniac"), r("Patrol", "policeofficer", "Knows the streets.", "jogger", "smoker", "slowhealer"), r("Dispatcher", "securityguard", "Heard it all over the radio.", "keenhearing", "unfit"), r("Prisoner", "burglar", "Picked the wrong week.", "inconspicuous", "conspicuous")},
		Tip:   "Starts in Raven Creek."},
	{ID: "raven-towers", Name: "High Rise", Mod: "ravencreek", Min: 1, Max: 6, Building: "apartment", Town: "Raven", Loadout: "basics", Tags: []string{"town", "hard"},
		Story: "Your apartment is on the top floor. The stairs are full.",
		Roles: []Role{r("Tenant", "unemployed", "Rent controlled.", "fastlearner", "unfit"), r("Super", "repairman", "Has every key.", "handy", "smoker", "insomniac"), r("Neighbour", "nurse", "Heard you knocking.", "firstaid"), r("Delivery Guy", "burgerflipper", "Wrong address.", "jogger", "slowreader")},
		Tip:   "Apartment start in Raven Creek."},
	{ID: "redstone-garrison", Name: "Garrison", Mod: "redstone", Min: 2, Max: 6, Building: "military", Town: "Redstone", Loadout: "military", Tags: []string{"hard"},
		Story: "The fort was supposed to be the safe zone. It held for six days.",
		Roles: []Role{r("Officer", "veteran", "Still has the codes.", "organized"), r("Private", "securityguard", "Eighteen years old.", "jogger", "fastlearner", "cowardly", "smoker"), r("Cook", "chef", "Mess hall.", "cook", "heartyappetite"), r("Mechanic", "mechanics", "Motor pool.", "handy", "smoker", "unfit"), r("Refugee", "unemployed", "Made it inside the wall.", "resilient", "slowhealer", "pronetoillness")},
		Tip:   "Starts at Fort Redstone with army gear."},
	{ID: "bedford-locals", Name: "Small Town", Mod: "bedford", Min: 1, Max: 6, Building: "house", Town: "Bedford", Loadout: "basics", Tags: []string{"easy"},
		Story: "Everyone in Bedford Falls knows everyone. Now everyone is trying to eat everyone.",
		Roles: []Role{r("Sheriff", "policeofficer", "One-man department.", "brave", "smoker"), r("Diner Owner", "chef", "Knows everyone's order.", "cook", "unfit"), r("Mechanic", "mechanics", "Only garage in town.", "mechanics", "smoker"), r("Teacher", "unemployed", "Taught half the town.", "fastreader", "organized", "cowardly", "weak")},
		Tip:   "Starts in Bedford Falls."},
	{ID: "eerie-lost", Name: "Lost in Eerie", Mod: "eerie", Min: 1, Max: 4, Building: "cabin", Town: "Eerie", Loadout: "campers", Tags: []string{"hard", "wild"},
		Story: "Took a wrong turn on the way to a camping trip. Ended up in Eerie County. Car died.",
		Roles: []Role{r("Driver", "mechanics", "Swears the map was wrong.", "mechanics", "cowardly"), r("Hiker", "parkranger", "Wanted to camp anyway.", "hiker", "smoker"), r("Skeptic", "unemployed", "Doesn't believe any of this.", "brave", "slowlearner"), r("Believer", "unemployed", "Knew it.", "keenhearing", "eagleeyed", "cowardly", "insomniac", "hemophobic")},
		Tip:   "Starts in Eerie Country."},
}

// installedPacks marks which mod packs are present in the catalog.
func installedPacks(cat *Catalog) map[string]bool {
	have := map[string]bool{}
	if cat == nil {
		return have
	}
	for _, m := range cat.Mods {
		if m.Removed {
			continue
		}
		hay := strings.ToLower(m.ID + " " + m.Name)
		for _, p := range modPacks {
			for _, k := range p.Match {
				if strings.Contains(hay, k) {
					have[p.ID] = true
				}
			}
		}
	}
	return have
}

// storyList returns every story, with mod stories marked Locked when their mod is missing.
func storyList(cat *Catalog) ([]Backstory, []ModPack) {
	have := installedPacks(cat)
	out := append([]Backstory{}, backstories...)
	for _, b := range modStories {
		b.Locked = !have[b.Mod]
		out = append(out, b)
	}
	balanceRoles(cat, out)
	packs := make([]ModPack, len(modPacks))
	for i, p := range modPacks {
		p.Installed = have[p.ID]
		packs[i] = p
	}
	return out, packs
}

// Filler bad traits used to pay for a role's good ones, cheapest flavour first.
var fillerTraits = []string{"smoker", "shortsighted", "clumsy", "weakstomach", "highthirst", "slowreader", "cowardly", "hardofhearing", "heartyappetite", "needsmoresleep", "slowhealer", "pronetoillness", "insomniac"}

// balanceRoles makes every role a legal character for this install: traits the game doesn't have
// or that clash with the job are dropped, then bad traits are added (or the priciest good ones
// dropped) until the points add up. A role without a job is balanced for an unemployed character.
func balanceRoles(cat *Catalog, stories []Backstory) {
	if cat == nil {
		return
	}
	traits := map[string]*Trait{}
	for i := range cat.Traits {
		traits[cat.Traits[i].ID] = &cat.Traits[i]
	}
	profs := map[string]*Profession{}
	for i := range cat.Professions {
		profs[cat.Professions[i].ID] = &cat.Professions[i]
	}
	for si := range stories {
		roles := make([]Role, len(stories[si].Roles))
		copy(roles, stories[si].Roles)
		for ri := range roles {
			roles[ri] = balanceRole(roles[ri], traits, profs, si+ri)
		}
		stories[si].Roles = roles
	}
}

func balanceRole(r Role, traits map[string]*Trait, profs map[string]*Profession, seed int) Role {
	pts := 8
	have := map[string]bool{}
	if p := profs[r.Profession]; p != nil {
		pts = p.Cost
		for _, g := range p.Granted {
			have[g] = true
		}
	} else {
		r.Profession = ""
	}
	clashes := func(id string) bool {
		t := traits[id]
		if t == nil || have[id] {
			return true
		}
		for _, ex := range t.Exclusive {
			if have[ex] {
				return true
			}
		}
		for h := range have {
			if o := traits[h]; o != nil && contains(o.Exclusive, id) {
				return true
			}
		}
		return false
	}
	var kept []string
	for _, id := range r.Traits {
		if clashes(id) {
			continue
		}
		have[id] = true
		kept = append(kept, id)
		pts -= traits[id].Cost
	}
	for i := 0; pts < 0 && i < len(fillerTraits); i++ {
		id := fillerTraits[(i+seed)%len(fillerTraits)]
		if clashes(id) {
			continue
		}
		have[id] = true
		kept = append(kept, id)
		pts -= traits[id].Cost
		if countBad(kept, traits) >= 3 {
			break
		}
	}
	for pts < 0 {
		best := -1
		for i, id := range kept {
			if traits[id].Cost > 0 && (best < 0 || traits[id].Cost > traits[kept[best]].Cost) {
				best = i
			}
		}
		if best < 0 {
			break
		}
		pts += traits[kept[best]].Cost
		kept = append(kept[:best], kept[best+1:]...)
	}
	// an expensive job with no good traits left: keep adding small flaws
	for i := 0; pts < 0 && i < len(fillerTraits); i++ {
		id := fillerTraits[(i+seed)%len(fillerTraits)]
		if clashes(id) {
			continue
		}
		have[id] = true
		kept = append(kept, id)
		pts -= traits[id].Cost
	}
	r.Traits = kept
	return r
}

func countBad(ids []string, traits map[string]*Trait) int {
	n := 0
	for _, id := range ids {
		if traits[id].Cost < 0 {
			n++
		}
	}
	return n
}
