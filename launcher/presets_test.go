package main

import (
	"encoding/json"
	"testing"
)

// TestPresets checks every story role, loadout and difficulty preset against the real game data.
func TestPresets(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	a := &App{paths: detectPaths(), steam: newSteam(""), cfg: loadConfig(t.TempDir())}
	s := newCatalogStore(a)
	s.Refresh(true)
	c, _ := s.Get()
	if c == nil || len(c.Traits) == 0 {
		t.Skip("no game data (set EZ_DEV)")
	}
	traits := map[string]*Trait{}
	for i := range c.Traits {
		traits[c.Traits[i].ID] = &c.Traits[i]
	}
	profs := map[string]*Profession{}
	for i := range c.Professions {
		profs[c.Professions[i].ID] = &c.Professions[i]
	}
	all, _ := storyList(c)
	for _, b := range append(append([]Backstory{}, backstories...), modStories...) {
		for _, r := range b.Roles {
			for _, id := range r.Traits {
				if traits[id] == nil {
					t.Errorf("%s/%s: unknown trait %s", b.ID, r.Name, id)
				}
			}
		}
	}
	ids := map[string]bool{}
	for _, b := range all {
		if ids[b.ID] {
			t.Errorf("duplicate story id %s", b.ID)
		}
		ids[b.ID] = true
		if len(b.Roles) < b.Min {
			t.Errorf("%s: %d roles for min %d", b.ID, len(b.Roles), b.Min)
		}
		if b.Loadout != "" && loadoutByID(b.Loadout, nil) == nil && b.Loadout != "none" {
			t.Errorf("%s: unknown loadout %s", b.ID, b.Loadout)
		}
		for _, r := range b.Roles {
			pts := 0
			have := map[string]bool{}
			if r.Profession != "" {
				p := profs[r.Profession]
				if p == nil {
					t.Errorf("%s/%s: unknown job %s", b.ID, r.Name, r.Profession)
					continue
				}
				pts = p.Cost
				for _, g := range p.Granted {
					have[g] = true
				}
			} else {
				pts = 8 // the player keeps their own job; assume unemployed
			}
			for _, id := range r.Traits {
				tr := traits[id]
				if tr == nil {
					t.Errorf("%s/%s: unknown trait %s", b.ID, r.Name, id)
					continue
				}
				if have[id] {
					t.Errorf("%s/%s: %s already comes with the job", b.ID, r.Name, id)
				}
				for _, ex := range tr.Exclusive {
					if have[ex] {
						t.Errorf("%s/%s: %s clashes with %s", b.ID, r.Name, id, ex)
					}
				}
				have[id] = true
				pts -= tr.Cost
			}
			if pts < 0 {
				t.Errorf("%s/%s: %d points over", b.ID, r.Name, -pts)
			}
		}
	}
	for _, l := range builtinLoadouts {
		for _, k := range l.Items {
			if c.itemByID[k.Type] == nil {
				t.Errorf("loadout %s: unknown item %s", l.ID, k.Type)
			}
		}
	}
	known := map[string]bool{}
	var pages []struct {
		Settings []struct{ Name string } `json:"settings"`
	}
	json.Unmarshal(c.Sandbox, &pages)
	for _, p := range pages {
		for _, o := range p.Settings {
			known[o.Name] = true
		}
	}
	for _, d := range difficultyDefs {
		for k := range d.over {
			if !known[k] {
				t.Errorf("preset %s: unknown option %s", d.id, k)
			}
		}
	}
	t.Logf("%d stories, %d loadouts, %d presets", len(all), len(builtinLoadouts), len(difficultyDefs))
}
