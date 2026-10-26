package main

import (
	"strconv"
	"strings"
)

// Options that character-creator mods add. The launcher shows them in its own creator, and Knox
// Link sets the mod's control (a field on CharacterCreationMain) before clicking through.

type CreatorExtra struct {
	Key     string  `json:"key"`
	Mod     string  `json:"mod"`
	ModName string  `json:"modName"`
	Kind    string  `json:"kind"` // slider | hairPicker
	Label   string  `json:"label"`
	Hint    string  `json:"hint,omitempty"`
	Unit    string  `json:"unit,omitempty"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Step    float64 `json:"step"`
	Default float64 `json:"default"`
	Field   string  `json:"-"` // CharacterCreationMain field holding the mod's slider
}

var creatorExtras = []CreatorExtra{
	{Key: "vsm.height", Mod: "ViewpointSizeMatters", Kind: "slider", Label: "Height", Unit: "cm", Min: 150, Max: 200, Step: 1, Default: 175, Field: "vsmHeight",
		Hint: "Height traits and world settings can narrow this in game."},
	{Key: "vsm.shape", Mod: "ViewpointSizeMatters", Kind: "slider", Label: "Body shape", Min: -100, Max: 100, Step: 1, Default: 0, Field: "vsmShape",
		Hint: "Thin to heavy. Looks only."},
	{Key: "vsm.muscle", Mod: "ViewpointSizeMatters", Kind: "slider", Label: "Muscular", Min: 0, Max: 100, Step: 1, Default: 0, Field: "vsmMuscle",
		Hint: "Looks only, doesn't change Strength."},
	{Key: "ihm.hair", Mod: "improvedhairmenubuild42", Kind: "hairPicker", Label: "Any hair colour"},
}

// activeExtras lists the extras whose mod is installed and switched on.
func activeExtras(cat *Catalog, on map[string]bool) []CreatorExtra {
	var out []CreatorExtra
	if cat == nil {
		return out
	}
	for _, e := range creatorExtras {
		m := cat.modByID[e.Mod]
		if m == nil || m.Removed || (on != nil && !on[e.Mod]) {
			continue
		}
		e.ModName = m.Name
		out = append(out, e)
	}
	return out
}

// extrasText is app_extras.txt: "field=value" for every slider the character set.
func extrasText(c Character) string {
	lines := map[string]string{}
	for _, e := range creatorExtras {
		if e.Field == "" {
			continue
		}
		if v, ok := c.Extras[e.Key]; ok {
			lines[e.Field] = strconv.FormatFloat(v, 'f', -1, 64)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(kv(lines)) + "\r\n"
}
