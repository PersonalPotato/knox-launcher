package main

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// Ready-made choices: difficulty presets, starting loadouts, building types to spawn in, and
// squad backstories. Difficulty presets start from the game's own preset files
// (media/lua/shared/Sandbox/*.lua) and only change options that already exist in the game.

type DifficultyPreset struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Tagline string            `json:"tagline"`
	Explain string            `json:"explain"`
	Base    string            `json:"base,omitempty"` // game preset it starts from
	Values  map[string]string `json:"values"`         // every option it sets
	Custom  bool              `json:"custom,omitempty"`
	Mine    bool              `json:"mine,omitempty"` // saved in the launcher (can be deleted)
	Ini     map[string]string `json:"ini,omitempty"`  // server settings it also sets (yours only)
}

type presetDef struct {
	id, name, tagline, explain, base string
	over                             map[string]string
}

var difficultyDefs = []presetDef{
	{"Outbreak", "Initial Infection", "Game preset. Easiest",
		"Low zombie count, poor senses, lots of loot. Power and water stay on for a while.",
		"Outbreak", nil},
	{"Apocalypse", "The First Week", "Game preset. Normal",
		"The default game. Shamblers, normal loot, power and water cut out after a couple of weeks.",
		"Apocalypse", nil},
	{"Rising", "Rising", "Game preset. Building",
		"Fewer zombies and more materials, so there's time to set up a base.",
		"Rising", nil},
	{"Extinction", "Survival", "Game preset. Hard",
		"Big horde, less loot, power and water go fast.",
		"Extinction", nil},
	{"SixMonthsLater", "Six Months Later", "Game preset. Late start",
		"Starts in December. Houses are already looted, everything is overgrown and there's no power or water.",
		"SixMonthsLater", nil},
	{"knox-casual", "Casual Co-op", "Easy night with friends",
		"First Week with low zombies, extra food and meds, 2x XP, utilities last 2 months, +4 trait points.",
		"Apocalypse", map[string]string{"Zombies": "5", "FoodLootNew": "1.4", "CannedFoodLootNew": "1.2", "MedicalLootNew": "1.2", "MultiplierConfig.Global": "2", "WaterShutModifier": "60", "ElecShutModifier": "60", "CharacterFreePoints": "4"}},
	{"knox-builders", "Builders", "Low threat, lots of materials",
		"Low zombies, extra tools and materials, 2 hour days, utilities last 3 months, building XP very high.",
		"Rising", map[string]string{"Zombies": "5", "MaterialLootNew": "2", "ToolLootNew": "1.6", "DayLength": "5", "WaterShutModifier": "90", "ElecShutModifier": "90", "ConstructionBonusPoints": "5"}},
	{"knox-sprinters", "Sprinters", "All zombies run",
		"Survival settings, but every zombie is a sprinter.",
		"Extinction", map[string]string{"ZombieLore.Speed": "1"}},
	{"knox-mixed", "Some Runners", "Shamblers with a few sprinters",
		"First Week with about 1 in 10 zombies sprinting. Keeps you honest.",
		"Apocalypse", map[string]string{"ZombieLore.Speed": "3", "ZombieLore.SprinterPercentage": "10"}},
	{"knox-walkingdead", "Walking Dead", "Slow, dumb, everyone's infected",
		"High zombie count, all shamblers, can't open doors. Anyone who dies turns, bitten or not. 2 hour days.",
		"Apocalypse", map[string]string{"Zombies": "3", "ZombieLore.Speed": "3", "ZombieLore.Cognition": "3", "ZombieLore.Transmission": "3", "DayLength": "5"}},
	{"knox-nights", "Night Shift", "Zombies only come out at night",
		"Zombies are only active at night, with sharp hearing and sight. Nights are pitch black.",
		"Apocalypse", map[string]string{"ZombieLore.ActiveOnly": "2", "ZombieLore.Hearing": "1", "ZombieLore.Sight": "1", "NightDarkness": "1"}},
	{"knox-realism", "Hardcore", "Mistakes hurt",
		"Survival base. Slower XP, worse injuries, tough zombies with long memory, more locked houses and cars.",
		"Extinction", map[string]string{"MultiplierConfig.Global": "0.75", "InjurySeverity": "3", "ZombieLore.Toughness": "1", "ZombieLore.Memory": "1", "LockedHouses": "5", "LockedCar": "5"}},
	{"knox-longhaul", "Long Haul", "For a group that plays a few nights a week",
		"3 hour days, slow food rot and erosion, utilities last 4 months, the horde peaks at day 90.",
		"Apocalypse", map[string]string{"DayLength": "6", "WaterShutModifier": "120", "ElecShutModifier": "120", "FoodRotSpeed": "4", "ErosionSpeed": "5", "ZombieConfig.PopulationPeakDay": "90"}},
	{"knox-winter", "Hard Winter", "Cold start, food is scarce",
		"Starts December 1st. Very cold, less food, power and water off within a month. Bring a coat.",
		"Apocalypse", map[string]string{"StartMonth": "12", "StartDay": "1", "Temperature": "1", "FoodLootNew": "0.5", "CannedFoodLootNew": "0.4", "WaterShutModifier": "20", "ElecShutModifier": "20"}},
	{"knox-scavenger", "Picked Clean", "Not much left to find",
		"First Week zombies, but loot is about half, and towns get more looted over time.",
		"Apocalypse", map[string]string{"FoodLootNew": "0.4", "CannedFoodLootNew": "0.3", "WeaponLootNew": "0.3", "AmmoLootNew": "0.3", "MedicalLootNew": "0.3", "MaximumLooted": "60", "DaysUntilMaximumLooted": "30"}},
	{"knox-guns", "Kentucky Armed", "Lots of guns and ammo",
		"First Week with plenty of guns and ammo. Shots are quieter than normal. Good for shooter squads.",
		"Apocalypse", map[string]string{"WeaponLootNew": "1.5", "RangedWeaponLootNew": "2.5", "AmmoLootNew": "2.5", "FirearmNoiseMultiplier": "0.7"}},
	{"knox-horde", "Horde Night", "Insane zombie count, easy loot",
		"Insane population that keeps respawning, but loot is doubled. Lots of fighting.",
		"Apocalypse", map[string]string{"Zombies": "1", "ZombieRespawn": "1", "FoodLootNew": "1.6", "WeaponLootNew": "1.6", "AmmoLootNew": "1.6", "MedicalLootNew": "1.6"}},
	{"knox-quiet", "Quiet Country", "Almost no zombies",
		"Very low zombies, no respawn, no helicopter. For a farming or building game.",
		"Rising", map[string]string{"Zombies": "5", "ZombieRespawn": "4", "Helicopter": "1", "ZombieConfig.PopulationMultiplier": "0.2"}},
	{"knox-nomad", "Road Warriors", "Lots of cars, little fuel",
		"Lots of cars in good condition but gas is low and pumps run dry. Keep moving.",
		"Apocalypse", map[string]string{"CarSpawnRate": "5", "CarGeneralCondition": "4", "ChanceHasGas": "1", "InitialGas": "2", "FuelStationGasMax": "0.3", "FuelStationGasEmptyChance": "60"}},
	{"knox-oneweek", "One Week Left", "Everything shuts off on day 7",
		"Power and water both cut on day 7. Helicopter comes often. Rush to get ready.",
		"Apocalypse", map[string]string{"WaterShutModifier": "7", "ElecShutModifier": "7", "Helicopter": "4"}},
}

// loadDifficultyPresets reads the game's preset files and applies Knox's tweaks on top.
// Only options the game actually has (known) survive, so a typo can never break a world.
func loadDifficultyPresets(p Paths, known map[string]bool) []DifficultyPreset {
	base := map[string]map[string]string{}
	dir := filepath.Join(p.Game, "media", "lua", "shared", "Sandbox")
	for _, name := range []string{"Outbreak", "Apocalypse", "Rising", "Extinction", "SixMonthsLater"} {
		if vals := readPresetFile(filepath.Join(dir, name+".lua")); len(vals) > 0 {
			base[name] = vals
		}
	}
	var out []DifficultyPreset
	for _, d := range difficultyDefs {
		b, ok := base[d.base]
		if !ok {
			continue
		}
		vals := map[string]string{}
		for k, v := range b {
			if known == nil || known[k] {
				vals[k] = v
			}
		}
		for k, v := range d.over {
			if known == nil || known[k] {
				vals[k] = v
			}
		}
		out = append(out, DifficultyPreset{ID: d.id, Name: d.name, Tagline: d.tagline, Explain: d.explain, Base: d.base, Values: vals})
	}
	// the player's own presets saved from the game's sandbox screen
	presetDir := filepath.Join(p.Zomboid, "Sandbox Presets")
	entries, _ := os.ReadDir(presetDir)
	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		vals := readSandboxFile(filepath.Join(presetDir, e.Name()))
		if len(vals) == 0 {
			vals = readPresetFile(filepath.Join(presetDir, e.Name()))
		}
		if len(vals) == 0 {
			continue
		}
		out = append(out, DifficultyPreset{ID: "file:" + name, Name: name, Tagline: "Your saved preset", Explain: "Saved from the game's sandbox screen.", Values: vals, Custom: true})
	}
	return out
}

// readPresetFile runs a "return { ... }" Lua file and flattens it to "ZombieLore.Speed" -> "1".
func readPresetFile(path string) map[string]string {
	out := map[string]string{}
	L := lua.NewState()
	defer L.Close()
	if err := L.DoFile(path); err != nil {
		return out
	}
	t, ok := L.Get(-1).(*lua.LTable)
	if !ok {
		if g, ok := L.GetGlobal("SandboxVars").(*lua.LTable); ok {
			t = g
		} else {
			return out
		}
	}
	var walk func(prefix string, t *lua.LTable)
	walk = func(prefix string, t *lua.LTable) {
		t.ForEach(func(k, v lua.LValue) {
			key := prefix + k.String()
			switch val := v.(type) {
			case *lua.LTable:
				walk(key+".", val)
			case lua.LBool:
				out[key] = strconv.FormatBool(bool(val))
			case lua.LNumber:
				out[key] = strconv.FormatFloat(float64(val), 'f', -1, 64)
			case lua.LString:
				out[key] = string(val)
			}
		})
	}
	walk("", t)
	delete(out, "Version")
	delete(out, "VERSION")
	return out
}

// Loadouts --------------------------------------------------------------------------------------

type LoadoutPreset struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Explain string    `json:"explain"`
	Items   []KitItem `json:"items"`
	Custom  bool      `json:"custom,omitempty"`
}

func kit(pairs ...any) []KitItem {
	var out []KitItem
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, KitItem{Type: pairs[i].(string), Count: pairs[i+1].(int)})
	}
	return out
}

var builtinLoadouts = []LoadoutPreset{
	{ID: "none", Name: "Nothing", Explain: "Just the clothes you spawn in. Normal game.", Items: nil},
	{ID: "basics", Name: "Basics", Explain: "Water, chips, 2 bandages and a school bag.",
		Items: kit("Base.WaterBottle", 1, "Base.Crisps", 1, "Base.Bandage", 2, "Base.Bag_Schoolbag", 1)},
	{ID: "scavenger", Name: "Looter", Explain: "Crowbar, torch, duffel bag and water.",
		Items: kit("Base.Crowbar", 1, "Base.HandTorch", 1, "Base.Bag_DuffelBag", 1, "Base.WaterBottle", 1)},
	{ID: "builders", Name: "Builders", Explain: "Hammer, saw, 2 boxes of nails and 6 planks.",
		Items: kit("Base.Hammer", 1, "Base.Saw", 1, "Base.NailsBox", 2, "Base.Plank", 6)},
	{ID: "lumberjacks", Name: "Lumberjacks", Explain: "Wood axe, saw, rope and a hiking bag.",
		Items: kit("Base.WoodAxe", 1, "Base.Saw", 1, "Base.Rope", 2, "Base.Bag_NormalHikingBag", 1)},
	{ID: "medics", Name: "Medics", Explain: "First aid kit, disinfectant, painkillers, suture needles, tweezers, 4 bandages.",
		Items: kit("Base.FirstAidKit", 1, "Base.Disinfectant", 1, "Base.Pills", 1, "Base.SutureNeedle", 2, "Base.Tweezers", 1, "Base.Bandage", 4)},
	{ID: "patients", Name: "Patients", Explain: "Hospital gown, slippers and painkillers.",
		Items: kit("Base.HospitalGown", 1, "Base.Shoes_Slippers", 1, "Base.Pills", 1)},
	{ID: "campers", Name: "Camping", Explain: "Big hiking bag, matches, fishing rod, 2 cans of beans, can opener, water.",
		Items: kit("Base.Bag_BigHikingBag", 1, "Base.Matches", 1, "Base.FishingRod", 1, "Base.TinnedBeans", 2, "Base.TinOpener", 1, "Base.WaterBottle", 1)},
	{ID: "farmers", Name: "Farmers", Explain: "Shovel, hoe, carrot, corn, potato and cabbage seeds, water.",
		Items: kit("Base.Shovel", 1, "Base.GardenHoe", 1, "Base.CarrotSeed", 2, "Base.CornSeed", 2, "Base.PotatoSeed", 2, "Base.CabbageSeed", 2, "Base.WaterBottle", 1)},
	{ID: "armed", Name: "Armed", Explain: "Pistol, box of 9mm, baseball bat, walkie talkie.",
		Items: kit("Base.Pistol", 1, "Base.Bullets9mmBox", 1, "Base.BaseballBat", 1, "Base.WalkieTalkie1", 1)},
	{ID: "military", Name: "Army Surplus", Explain: "ALICE pack, machete, hunting rifle, box of .308, area map. Pretty much cheating.",
		Items: kit("Base.Bag_ALICEpack", 1, "Base.Machete", 1, "Base.HuntingRifle", 1, "Base.308Box", 1, "Base.MuldraughMap", 1)},
	{ID: "cops", Name: "Police", Explain: "Revolver, box of .38, nightstick, police walkie.",
		Items: kit("Base.Revolver", 1, "Base.Bullets38Box", 1, "Base.Nightstick", 1, "Base.WalkieTalkie3", 1)},
	{ID: "hunters", Name: "Hunters", Explain: "Shotgun, shells, hunting knife, hiking bag, matches.",
		Items: kit("Base.Shotgun", 1, "Base.ShotgunShellsBox", 1, "Base.HuntingKnife", 1, "Base.Bag_NormalHikingBag", 1, "Base.Matches", 1)},
	{ID: "mechanics", Name: "Mechanics", Explain: "Wrench, screwdriver, lug wrench, jack, gas can, tool box.",
		Items: kit("Base.Wrench", 1, "Base.Screwdriver", 1, "Base.LugWrench", 1, "Base.Jack", 1, "Base.PetrolCan", 1, "Base.Toolbox", 1)},
	{ID: "cooks", Name: "Kitchen", Explain: "Kitchen knife, pot, pan, rice, pasta, oil, salt and a cookbook.",
		Items: kit("Base.KitchenKnife", 1, "Base.Pot", 1, "Base.Pan", 1, "Base.Rice", 1, "Base.Pasta", 1, "Base.OilVegetable", 1, "Base.Salt", 1, "Base.BookCooking1", 1)},
	{ID: "fishers", Name: "Fishing Trip", Explain: "2 fishing rods, fishing line, hunting knife, 2 cans of soup, 2 waters.",
		Items: kit("Base.FishingRod", 2, "Base.FishingLine", 1, "Base.HuntingKnife", 1, "Base.TinnedSoup", 2, "Base.WaterBottle", 2)},
	{ID: "tailors", Name: "Sewing Kit", Explain: "Needle, thread, scissors, ripped sheets and a tailoring book.",
		Items: kit("Base.Needle", 1, "Base.Thread", 2, "Base.Scissors", 1, "Base.RippedSheets", 6, "Base.BookTailoring1", 1)},
	{ID: "electric", Name: "Electrician", Explain: "Screwdriver, electronics scrap, batteries, 2 walkies and an electrical book.",
		Items: kit("Base.Screwdriver", 1, "Base.ElectronicsScrap", 4, "Base.Battery", 4, "Base.WalkieTalkie2", 2, "Base.BookElectrician1", 1)},
	{ID: "welders", Name: "Welders", Explain: "Blow torch, welding mask, welding rods and sheet metal.",
		Items: kit("Base.BlowTorch", 1, "Base.WeldingMask", 1, "Base.WeldingRods", 2, "Base.SheetMetal", 4, "Base.SmallSheetMetal", 4)},
	{ID: "partypack", Name: "Party", Explain: "Whiskey, beer, chips, chocolate, cigarettes. Not useful. Fun though.",
		Items: kit("Base.Whiskey", 1, "Base.BeerCan", 4, "Base.Crisps", 2, "Base.Chocolate", 2, "Base.CigarettePack", 1)},
	{ID: "melee", Name: "Melee Only", Explain: "Axe, sledgehammer, pipe wrench, crowbar. No guns.",
		Items: kit("Base.Axe", 1, "Base.Sledgehammer", 1, "Base.PipeWrench", 1, "Base.Crowbar", 1)},
	{ID: "prepper", Name: "Prepper", Explain: "Big hiking bag, canned food, water, first aid kit, hunting knife, torch, batteries, maps.",
		Items: kit("Base.Bag_BigHikingBag", 1, "Base.TinnedBeans", 3, "Base.CannedChili", 2, "Base.TinOpener", 1, "Base.WaterBottle", 2, "Base.FirstAidKit", 1, "Base.HuntingKnife", 1, "Base.HandTorch", 1, "Base.Battery", 2, "Base.MuldraughMap", 1, "Base.WestpointMap", 1)},
	{ID: "rifles", Name: "Rifle Club", Explain: "Varmint rifle, box of ammo, hunting knife and a Rosewood map.",
		Items: kit("Base.VarmintRifle", 1, "Base.556Box", 1, "Base.HuntingKnife", 1, "Base.RosewoodMap", 1)},
	{ID: "readers", Name: "Bookworms", Explain: "Level 1 books for carpentry, cooking, farming, first aid, mechanics and foraging.",
		Items: kit("Base.BookCarpentry1", 1, "Base.BookCooking1", 1, "Base.BookFarming1", 1, "Base.BookFirstAid1", 1, "Base.BookMechanic1", 1, "Base.BookForaging1", 1)},
	{ID: "trappers", Name: "Trappers", Explain: "Cage traps, mouse traps, twine, hunting knife and a trapping book.",
		Items: kit("Base.TrapCage", 2, "Base.TrapMouse", 2, "Base.Twine", 2, "Base.HuntingKnife", 1, "Base.BookTrapping1", 1)},
}

// Building types you can spawn in (worldmap RoomTone values) --------------------------------------

type BuildingCategory struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Tones []string `json:"-"`
}

var buildingCategories = []BuildingCategory{
	{"house", "A house", []string{"HouseSuburb", "HouseSmall", "HouseGarage", "HouseMedium", "HouseLarge", "HouseCountry", "Townhouse", "RedbrickTownhouse", "BrownbrickTownhouse", "HouseSuburbGarage", "HouseLake", "Residential"}},
	{"bighouse", "A big house", []string{"HouseLarge", "HouseCountry", "Lakehouse"}},
	{"trailer", "A trailer", []string{"Trailer", "TrailerNice"}},
	{"apartment", "An apartment block", []string{"ApartmentBuilding"}},
	{"farm", "A farm", []string{"Farmhouse", "Farmhousing", "Barn"}},
	{"cabin", "A cabin in the woods", []string{"Cabin", "ResortCabin", "CabinAbandoned", "WoodCabin", "Lakehouse"}},
	{"police", "The police station", []string{"Police"}},
	{"fire", "The fire station", []string{"Firestation"}},
	{"hospital", "A hospital or clinic", []string{"Hospital", "Medical"}},
	{"church", "A church", []string{"Church"}},
	{"school", "A school", []string{"School", "SchoolColege"}},
	{"motel", "A motel or hotel", []string{"Motel", "Hotel", "Bedandbreakfast"}},
	{"military", "A military base", []string{"MilitaryHouse", "MilitaryTownhouse", "MilitaryTent", "MilitaryCommunications", "MilitaryTower"}},
	{"prison", "The prison", []string{"Prison"}},
	{"warehouse", "A warehouse or factory", []string{"Warehousestorage", "IndustrialWarehouse", "Factory", "Freight"}},
	{"store", "A store", []string{"Grocery", "Generalstore", "Cornerstore", "Storefront"}},
	{"gas", "A gas station", []string{"Gas"}},
	{"bar", "A bar", []string{"BarWood", "BarStone"}},
	{"camp", "A summer camp", []string{"Summercamp"}},
	{"mall", "The mall", []string{"Mall"}},
}

func buildingCategory(id string) *BuildingCategory {
	for i := range buildingCategories {
		if buildingCategories[i].ID == id {
			return &buildingCategories[i]
		}
	}
	return &buildingCategories[0]
}

// Squad backstories ------------------------------------------------------------------------------

type Role struct {
	Name       string   `json:"name"`
	Profession string   `json:"profession"`
	Traits     []string `json:"traits,omitempty"` // starting traits; the player can change them
	Note       string   `json:"note"`
}

type Backstory struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Min      int       `json:"min"`
	Max      int       `json:"max"`
	Tags     []string  `json:"tags,omitempty"` // easy, hard, town, wild, long
	Mod      string    `json:"mod,omitempty"`  // ModPack id this story needs
	Town     string    `json:"town,omitempty"` // part of a town name to start in (mod maps)
	Locked   bool      `json:"locked,omitempty"`
	Story    string    `json:"story"`
	Building string    `json:"building"` // building category everyone starts in
	Loadout  string    `json:"loadout"`  // loadout preset id
	Kit      []KitItem `json:"kit,omitempty"`
	Roles    []Role    `json:"roles"`
	Tip      string    `json:"tip"`
	Custom   bool      `json:"custom,omitempty"` // one of yours (story editor or community)
	Author   string    `json:"author,omitempty"` // who made a community story
}

func r(name, prof, note string, traits ...string) Role {
	return Role{Name: name, Profession: prof, Note: note, Traits: traits}
}

var backstories = []Backstory{
	{ID: "drifter", Name: "Off the Grid", Min: 1, Max: 1, Building: "cabin", Loadout: "campers", Tags: []string{"wild", "easy"},
		Story: "You've lived alone in a cabin for a month. The radio stopped working a few days ago and you only just noticed.",
		Roles: []Role{r("Hermit", "parkranger", "Good in the woods, bad with people.", "outdoorsman", "hiker", "agoraphobic", "smoker")},
		Tip:   "Quiet start far from town."},
	{ID: "lonecop", Name: "Last Cop in Town", Min: 1, Max: 1, Building: "police", Loadout: "cops", Tags: []string{"town"},
		Story: "Everyone else at the station went home to their families. You stayed.",
		Roles: []Role{r("Officer", "policeofficer", "Has the keys to everything here.", "brave", "smoker", "insomniac")},
		Tip:   "Solo start in the police station with a sidearm."},
	{ID: "trucker", Name: "Long Haul", Min: 1, Max: 2, Building: "gas", Loadout: "mechanics", Tags: []string{"town"},
		Story: "Pulled into a gas station for coffee. The clerk was eating someone.",
		Roles: []Role{r("Trucker", "mechanics", "Knows engines, hates walking.", "lowthirst", "heartyappetite", "sundaydriver"), r("Hitchhiker", "unemployed", "Got picked up two towns back.", "jogger", "inconspicuous", "slowreader")},
		Tip:   "Starts at a gas station. Find a car."},
	{ID: "couple", Name: "Date Night", Min: 2, Max: 2, Building: "house", Loadout: "basics", Tags: []string{"easy", "town"},
		Story: "You had a table booked for eight. You never left the house.",
		Roles: []Role{r("The Planner", "", "Has the car keys.", "organized", "cowardly"), r("The Optimist", "", "Thinks it's a bad flu.", "brave", "fastlearner", "clumsy", "heartyappetite")},
		Tip:   "Normal house, any jobs."},
	{ID: "roommates", Name: "Roommates", Min: 2, Max: 3, Building: "apartment", Loadout: "partypack", Tags: []string{"town"},
		Story: "Rent was due Friday. Guess that's not a problem anymore.",
		Roles: []Role{r("Night Owl", "burgerflipper", "Works nights at Spiffo's.", "needslesssleep", "cook", "smoker"), r("Gamer", "unemployed", "Hasn't been outside in a week.", "fastreader", "unfit", "inconspicuous"), r("Gym Bro", "fitnessinstructor", "Drinks protein shakes. Lifts.", "strong", "heartyappetite", "slowreader", "illiterate")},
		Tip:   "Starts in an apartment block in town. Lots of neighbours, most of them dead."},
	{ID: "cops", Name: "Last Shift", Min: 2, Max: 4, Building: "police", Loadout: "cops", Tags: []string{"town"},
		Story: "Dispatch stopped answering an hour ago. The phones are still ringing.",
		Roles: []Role{r("Sergeant", "policeofficer", "In charge, for now.", "brave", "organized", "smoker", "insomniac", "slowhealer"), r("Rookie", "policeofficer", "First week on the job.", "jogger", "cowardly", "hemophobic"), r("Desk Officer", "securityguard", "Hasn't run in years.", "organized", "unfit"), r("Suspect", "burglar", "Was in a cell when it started.", "inconspicuous", "conspicuous")},
		Tip:   "Everyone starts in the police station."},
	{ID: "firecrew", Name: "Engine 9", Min: 2, Max: 4, Building: "fire", Loadout: "builders", Tags: []string{"town"},
		Story: "Last call came in hours ago. Truck's out of gas. Town's on fire.",
		Roles: []Role{r("Captain", "fireofficer", "Calls the shots.", "brave", "smoker"), r("Firefighter", "fireofficer", "Strong back, short temper.", "strong", "heartyappetite", "slowlearner"), r("Engineer", "mechanics", "Keeps the truck running.", "handy", "unfit"), r("Paramedic", "nurse", "Patches everyone up.", "firstaid", "needsmoresleep")},
		Tip:   "Fire station start with building tools."},
	{ID: "nightshift", Name: "ER Night Shift", Min: 2, Max: 5, Building: "hospital", Loadout: "medics", Tags: []string{"hard", "town"},
		Story: "The ER filled up around midnight. The ones who died got back up. You locked yourselves in the staff room.",
		Roles: []Role{r("Doctor", "doctor", "Has seen what a bite does.", "fastlearner", "smoker", "insomniac"), r("Nurse", "nurse", "Calmer than the doctor.", "firstaid", "resilient", "weakstomach", "hardofhearing"), r("Orderly", "unemployed", "Can hold a door shut.", "strong", "slowreader", "illiterate"), r("Security", "securityguard", "Has a flashlight and a radio.", "keenhearing", "unfit"), r("Patient", "unemployed", "In for a broken arm.", "slowhealer", "pronetoillness")},
		Tip:   "Hospital start. Lots of zombies, lots of meds."},
	{ID: "coma", Name: "Woke Up Alone", Min: 1, Max: 6, Building: "hospital", Loadout: "patients", Tags: []string{"hard", "town"},
		Story: "You wake up in a hospital bed. The drip ran dry days ago. Nobody answers the call button.",
		Roles: []Role{r("Patient", "unemployed", "Out cold for a week.", "slowhealer", "unfit"), r("Visitor", "", "Fell asleep in the chair.", "insomniac", "fastlearner"), r("Night Nurse", "nurse", "Clocked in. Never clocked out.", "firstaid", "needsmoresleep"), r("Janitor", "unemployed", "Has keys to every room.", "organized", "slowreader"), r("Kid in for Stitches", "unemployed", "Still has the lollipop.", "jogger", "cowardly"), r("Old Timer", "veteran", "Seen worse. Probably.", "brave", "hardofhearing", "slowhealer")},
		Tip:   "Hospital start with a gown, slippers and painkillers. Works solo."},
	{ID: "prisonbreak", Name: "Lockdown", Min: 2, Max: 6, Building: "prison", Loadout: "none", Tags: []string{"hard"},
		Story: "The guards stopped coming. Then the cell doors opened.",
		Roles: []Role{r("Guard", "securityguard", "Has the keys. Trusts nobody.", "brave", "insomniac"), r("Lifer", "burglar", "Knows every way out.", "stout", "illiterate", "conspicuous"), r("Kitchen", "chef", "Kept a knife.", "cook", "smoker"), r("New Fish", "unemployed", "Got in yesterday.", "jogger", "cowardly", "weak"), r("Motor Pool", "mechanics", "Worked on the prison vans.", "handy", "smoker", "disorganized"), r("Infirmary", "nurse", "The only one with any meds.", "firstaid", "hemophobic")},
		Tip:   "Prison start with nothing. Hardest one."},
	{ID: "family", Name: "Family Reunion", Min: 3, Max: 6, Building: "bighouse", Loadout: "basics", Tags: []string{"easy"},
		Story: "Grandma's 80th. Everyone drove in. Nobody cut the cake.",
		Roles: []Role{r("Responsible One", "", "Always has a first aid kit.", "firstaid", "organized", "cowardly", "insomniac"), r("Black Sheep", "burglar", "Late, as usual.", "inconspicuous"), r("Handy Uncle", "carpenter", "Already measuring the windows.", "handy", "heartyappetite", "smoker"), r("Annoying Cousin", "", "Won't stop talking.", "jogger", "conspicuous"), r("Grandpa", "veteran", "Doesn't talk about the war.", "brave", "hardofhearing", "shortsighted", "slowhealer"), r("Chef", "chef", "Made enough food for twenty.", "cook", "heartyappetite")},
		Tip:   "Big house, everyone together."},
	{ID: "lakeweekend", Name: "Lake Weekend", Min: 2, Max: 5, Building: "cabin", Loadout: "fishers", Tags: []string{"wild", "easy"},
		Story: "Long weekend at a rented cabin. No phone signal. You drove back into town on Monday and turned right around.",
		Roles: []Role{r("Fisher", "fisherman", "Here for the fish.", "fishing", "lowthirst", "agoraphobic"), r("Organiser", "", "Booked the cabin.", "organized", "unfit"), r("Ranger's Kid", "parkranger", "Knows the trails.", "hiker", "slowreader"), r("Cook", "chef", "Packed real spices.", "cook", "heartyappetite"), r("Plus One", "", "Nobody knows who invited them.", "inconspicuous", "fastlearner", "cowardly", "weakstomach")},
		Tip:   "Cabin in the woods with fishing gear."},
	{ID: "crew", Name: "Job Site", Min: 2, Max: 5, Building: "warehouse", Loadout: "builders", Tags: []string{"town"},
		Story: "Overtime on the warehouse job. The foreman went outside to check a noise.",
		Roles: []Role{r("Foreman", "constructionworker", "Keeps the crew working.", "organized", "smoker"), r("Carpenter", "carpenter", "Fast with a hammer.", "handy", "highthirst", "smoker"), r("Electrician", "electrician", "Can get a generator going.", "tinkerer", "clumsy"), r("Welder", "metalworker", "Makes walls out of car doors.", "stout", "shortsighted"), r("New Guy", "unemployed", "First day.", "fastlearner", "cowardly")},
		Tip:   "Warehouse start with building tools."},
	{ID: "farmhands", Name: "Harvest", Min: 2, Max: 4, Building: "farm", Loadout: "farmers", Tags: []string{"wild", "long"},
		Story: "The crop was nearly ready when the neighbours started acting strange.",
		Roles: []Role{r("Farmer", "farmer", "Knows the land.", "gardener", "outdoorsman", "illiterate"), r("Rancher", "rancher", "Knows the animals.", "stout", "smoker"), r("Farmhand", "unemployed", "Quiet and reliable.", "strong", "slowlearner", "illiterate"), r("Vet", "doctor", "Treats animals. Will treat you.", "firstaid", "cowardly")},
		Tip:   "Farm start with seeds and tools."},
	{ID: "army", Name: "Left Behind", Min: 3, Max: 6, Building: "military", Loadout: "military", Tags: []string{"hard"},
		Story: "Your unit held the line at the exclusion zone. Command left by helicopter. You weren't on the list.",
		Roles: []Role{r("Squad Leader", "veteran", "Still giving orders.", "organized"), r("Rifleman", "securityguard", "Good shot.", "target_shooter", "smoker", "insomniac"), r("Medic", "nurse", "Low on everything.", "firstaid", "needsmoresleep"), r("Engineer", "engineer", "Builds things. Blows things up.", "handy", "slowhealer", "smoker", "unfit"), r("Radio Op", "electrician", "Still listening for the evac call.", "keenhearing", "unfit"), r("Driver", "mechanics", "Wants a truck that works.", "speeddemon", "smoker")},
		Tip:   "Military site with army gear. Big horde nearby."},
	{ID: "congregation", Name: "Sunday Service", Min: 3, Max: 8, Building: "church", Loadout: "basics", Tags: []string{"town"},
		Story: "Half the town came to church when the news got bad. The pews are the barricade.",
		Roles: []Role{r("Pastor", "unemployed", "Keeps everyone calm.", "brave", "pacifist", "fastreader"), r("Groundskeeper", "carpenter", "Has the shed keys.", "handy", "hardofhearing", "smoker"), r("Nurse", "nurse", "Sat in the second row.", "firstaid"), r("Off-duty Cop", "policeofficer", "Left the gun at home.", "brave", "smoker", "heartyappetite"), r("Potluck Cook", "chef", "Brought a casserole.", "cook", "unfit")},
		Tip:   "Church start. Lots of people, not much gear."},
	{ID: "motel", Name: "No Vacancy", Min: 2, Max: 8, Building: "motel", Loadout: "basics", Tags: []string{"town"},
		Story: "Strangers stuck at the same roadside motel when the roads closed.",
		Roles: []Role{r("Trucker", "mechanics", "Truck's out of gas.", "lowthirst", "smoker"), r("Salesman", "unemployed", "Suitcase full of vacuum parts.", "organized", "unfit", "cowardly"), r("Runaway", "burglar", "Paid cash. Fake name.", "inconspicuous", "jogger", "slowreader", "conspicuous"), r("Night Manager", "securityguard", "Has every room key.", "needslesssleep", "smoker"), r("Honeymooners", "", "This wasn't the plan.", "lighteater", "clumsy")},
		Tip:   "Works for any group of randoms."},
	{ID: "campcounselors", Name: "Camp Counselors", Min: 3, Max: 8, Building: "camp", Loadout: "campers", Tags: []string{"wild"},
		Story: "The kids got picked up early. You stayed behind to close up camp. Then the phones died.",
		Roles: []Role{r("Head Counselor", "parkranger", "Has the whistle.", "organized", "agoraphobic"), r("Lifeguard", "fitnessinstructor", "Fastest swimmer.", "athletic", "slowreader", "illiterate"), r("Camp Cook", "chef", "Makes anything edible.", "cook", "unfit"), r("Arts and Crafts", "tailor", "Can sew anything.", "tailor", "dextrous", "cowardly"), r("Maintenance", "repairman", "Fixes the generator.", "handy", "smoker", "hardofhearing")},
		Tip:   "Summer camp out in the woods."},
	{ID: "school", Name: "Detention", Min: 2, Max: 6, Building: "school", Loadout: "none", Tags: []string{"town", "hard"},
		Story: "Saturday detention. The teacher went to get coffee and didn't come back.",
		Roles: []Role{r("Jock", "unemployed", "Starting quarterback.", "athletic", "slowreader", "illiterate"), r("Nerd", "unemployed", "Read every book in the library.", "fastlearner", "fastreader", "weak", "shortsighted"), r("Burnout", "unemployed", "Was already in detention.", "inconspicuous", "smoker", "needsmoresleep"), r("Teacher", "unemployed", "Supposed to be in charge.", "organized", "fastreader", "cowardly", "unfit"), r("Janitor", "repairman", "Has all the keys.", "handy", "insomniac", "smoker", "slowhealer")},
		Tip:   "School start, no gear. Raid the gym and the kitchen."},
	{ID: "barnight", Name: "Last Call", Min: 2, Max: 6, Building: "bar", Loadout: "partypack", Tags: []string{"town"},
		Story: "Nobody at the bar wanted to go home. Then nobody could.",
		Roles: []Role{r("Bartender", "burgerflipper", "Keeps a bat under the counter.", "baseballplayer", "insomniac", "smoker"), r("Regular", "unemployed", "Has a stool with his name on it.", "stout", "heartyappetite", "smoker", "unfit"), r("Bouncer", "securityguard", "Throws people out for a living.", "strong", "slowreader", "illiterate"), r("Band", "unemployed", "Played to an empty room.", "graceful", "smoker", "needsmoresleep"), r("Designated Driver", "", "Only one who's sober.", "eagleeyed", "cowardly")},
		Tip:   "Starts in a bar. Lots of drink, not much else."},
	{ID: "mall", Name: "Mall Rats", Min: 2, Max: 8, Building: "mall", Loadout: "none", Tags: []string{"hard", "town"},
		Story: "You hid in the mall when the riots started. It worked for a few days.",
		Roles: []Role{r("Security", "securityguard", "Mall cop. Takes it seriously.", "brave", "unfit"), r("Food Court", "burgerflipper", "Spiffo's employee of the month.", "cook", "heartyappetite"), r("Shopper", "", "Came in for shoes.", "inconspicuous", "cowardly"), r("Teen", "unemployed", "Skateboards. Fast.", "jogger", "graceful", "slowreader", "smoker"), r("Store Manager", "unemployed", "Wants everyone out by closing.", "organized", "unfit")},
		Tip:   "Mall start. Huge loot, huge horde."},
	{ID: "hunters", Name: "Hunting Season", Min: 1, Max: 4, Building: "cabin", Loadout: "hunters", Tags: []string{"wild"},
		Story: "Opening weekend of deer season. The deer were fine. The other hunters weren't.",
		Roles: []Role{r("Old Hunter", "veteran", "Been hunting here forty years.", "hunter", "hardofhearing", "slowhealer", "smoker", "shortsighted"), r("Son", "lumberjack", "First time with a rifle.", "outdoorsman", "cowardly"), r("Buddy", "unemployed", "Brought the beer.", "fishing", "heartyappetite", "unfit"), r("Guide", "parkranger", "Gets paid either way.", "wildernessknowledge", "agoraphobic", "smoker", "insomniac")},
		Tip:   "Cabin start with a shotgun."},
	{ID: "lumberjacks", Name: "Logging Camp", Min: 2, Max: 5, Building: "cabin", Loadout: "lumberjacks", Tags: []string{"wild", "long"},
		Story: "Two weeks out at the logging site. Supply truck didn't show.",
		Roles: []Role{r("Lumberjack", "lumberjack", "Swings an axe all day.", "strong", "illiterate", "smoker"), r("Faller", "lumberjack", "Fells the big ones.", "stout", "hardofhearing"), r("Mechanic", "mechanics", "Keeps the saws running.", "handy", "unfit"), r("Camp Cook", "chef", "Feeds the crew.", "cook", "heartyappetite"), r("Surveyor", "engineer", "Has the maps.", "eagleeyed", "weak")},
		Tip:   "Woods start with axes."},
	{ID: "doomsday", Name: "Preppers", Min: 1, Max: 4, Building: "farm", Loadout: "prepper", Tags: []string{"easy", "wild", "long"},
		Story: "You've been ready for this for years. Your family thought you were crazy.",
		Roles: []Role{r("Prepper", "veteran", "Told you so.", "organized", "outdoorsman", "agoraphobic", "insomniac"), r("Spouse", "nurse", "Went along with it.", "firstaid", "gardener"), r("Teenager", "unemployed", "Hated the drills.", "jogger", "fastlearner", "cowardly", "weakstomach"), r("Neighbour", "farmer", "Thought it was a bunker for tornadoes.", "gardener", "slowlearner")},
		Tip:   "Farm start with a prepper bag each."},
	{ID: "garage", Name: "Chop Shop", Min: 2, Max: 4, Building: "warehouse", Loadout: "mechanics", Tags: []string{"town"},
		Story: "The garage was doing some side work on stolen cars. Seemed like a bad week to call the cops anyway.",
		Roles: []Role{r("Owner", "mechanics", "Knows every car on the lot.", "mechanics", "smoker", "unfit"), r("Wheelman", "burglar", "Can hotwire anything.", "speeddemon"), r("Welder", "metalworker", "Cuts off VIN plates.", "stout", "shortsighted"), r("Lookout", "unemployed", "Paid in cash.", "eagleeyed", "keenhearing", "cowardly", "smoker", "insomniac")},
		Tip:   "Warehouse start with car tools."},
	{ID: "gunstore", Name: "Gun Shop", Min: 1, Max: 4, Building: "store", Loadout: "armed", Tags: []string{"easy", "town"},
		Story: "The owner let you in to wait it out. Then the owner turned.",
		Roles: []Role{r("Clerk", "securityguard", "Knows the stock.", "target_shooter", "unfit", "smoker"), r("Range Regular", "veteran", "Comes in every Thursday.", "brave", "hardofhearing"), r("Hunter", "lumberjack", "Wanted a new scope.", "hunter", "illiterate", "smoker"), r("Customer", "unemployed", "Bought their first gun yesterday.", "fastlearner", "cowardly", "hemophobic")},
		Tip:   "Store start with a pistol each. Easy mode."},
	{ID: "chefs", Name: "Dinner Rush", Min: 2, Max: 5, Building: "store", Loadout: "cooks", Tags: []string{"town"},
		Story: "Friday dinner rush at the diner. Table six bit the waitress.",
		Roles: []Role{r("Head Chef", "chef", "Runs the kitchen.", "cook", "smoker", "insomniac"), r("Line Cook", "burgerflipper", "Fast with a knife.", "dextrous", "smoker"), r("Waitress", "unemployed", "On her feet all day.", "jogger", "graceful", "unfit"), r("Dishwasher", "unemployed", "Doesn't speak much English.", "strong", "illiterate"), r("Food Critic", "", "Was going to give it two stars.", "fastreader", "weakstomach", "weak")},
		Tip:   "Starts in a store with a kitchen kit."},
	{ID: "bikers", Name: "Road Crew", Min: 2, Max: 6, Building: "bar", Loadout: "melee", Tags: []string{"hard"},
		Story: "The club was meeting at the bar when it all went down. Bikes are outside. Most of them still work.",
		Roles: []Role{r("President", "veteran", "Nobody argues.", "brave", "smoker"), r("Enforcer", "unemployed", "Big guy.", "strong", "stout", "illiterate", "conspicuous", "slowlearner"), r("Wrench", "mechanics", "Fixes the bikes.", "mechanics", "smoker", "shortsighted"), r("Prospect", "unemployed", "Wants in.", "fastlearner", "jogger", "cowardly", "smoker"), r("Old Lady", "nurse", "Patches everyone up.", "firstaid", "smoker")},
		Tip:   "Bar start with melee weapons."},
	{ID: "scientists", Name: "The Lab", Min: 2, Max: 5, Building: "military", Loadout: "medics", Tags: []string{"hard"},
		Story: "You worked at the facility. You know more about the virus than you'd like.",
		Roles: []Role{r("Lead Researcher", "doctor", "Signed off on the trials.", "fastlearner", "fastreader", "weak", "insomniac", "cowardly"), r("Lab Tech", "engineer", "Ran the equipment.", "tinkerer", "shortsighted"), r("Guard", "securityguard", "Paid to keep people out.", "brave", "smoker"), r("Test Subject", "unemployed", "Not infected. Probably.", "resilient", "fasthealer", "pronetoillness", "slowreader", "insomniac")},
		Tip:   "Military site start with medical gear."},
	{ID: "tailors", Name: "Sewing Circle", Min: 2, Max: 5, Building: "house", Loadout: "tailors", Tags: []string{"easy", "long"},
		Story: "Thursday quilting group. The quilt is going to have to wait.",
		Roles: []Role{r("Host", "tailor", "Has the good machine.", "tailor", "organized", "unfit"), r("Gossip", "unemployed", "Knows everyone's business.", "keenhearing", "conspicuous"), r("Retired Nurse", "nurse", "Still sharp.", "firstaid", "slowhealer"), r("Grandson", "unemployed", "Drove her here.", "jogger", "cowardly"), r("Gardener", "farmer", "Brought tomatoes.", "gardener", "hardofhearing")},
		Tip:   "House start with sewing gear. Chill game."},
}

func backstoryByID(id string) *Backstory {
	for i := range backstories {
		if backstories[i].ID == id {
			return &backstories[i]
		}
	}
	for i := range modStories {
		if modStories[i].ID == id {
			return &modStories[i]
		}
	}
	return nil
}

func loadoutByID(id string, custom map[string][]KitItem) []KitItem {
	for _, l := range builtinLoadouts {
		if l.ID == id {
			return l.Items
		}
	}
	return custom[strings.TrimPrefix(id, "my:")]
}

// allLoadouts lists the built-in loadouts (only items this install has) and the player's own.
func allLoadouts(cat *Catalog, custom map[string][]KitItem) []LoadoutPreset {
	var out []LoadoutPreset
	fix := func(items []KitItem) []KitItem {
		var ok []KitItem
		for _, k := range items {
			if cat == nil {
				ok = append(ok, k)
				continue
			}
			if it := cat.itemByID[k.Type]; it != nil {
				k.Name = it.Name
				ok = append(ok, k)
			}
		}
		return ok
	}
	for _, l := range builtinLoadouts {
		l.Items = fix(l.Items)
		out = append(out, l)
	}
	names := make([]string, 0, len(custom))
	for n := range custom {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		out = append(out, LoadoutPreset{ID: "my:" + n, Name: n, Explain: "Your loadout.", Items: fix(custom[n]), Custom: true})
	}
	return out
}
