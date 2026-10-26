package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCatalog(t *testing.T) {
	if os.Getenv("EZ_DEV") == "" {
		t.Skip("set EZ_DEV")
	}
	a := &App{paths: detectPaths(), steam: newSteam(""), cfg: loadConfig(t.TempDir())}
	s := newCatalogStore(a)
	s.Refresh(true)
	c, b := s.Get()
	t.Logf("game %s, %d bytes, mods %d, traits %d, profs %d, items %d, clothing looks %d, spawns %d, problems %v",
		c.Game, len(b), len(c.Mods), len(c.Traits), len(c.Professions), len(c.Items), len(c.Clothing), len(c.Spawns), c.Problems)
	for _, sp := range c.Spawns {
		t.Logf("spawn %s: %d points", sp.Name, len(sp.Points))
	}
	for _, m := range c.Mods {
		if m.Broken != "" || len(m.Require) > 0 {
			t.Logf("mod %s (%s) ws=%s broken=%q req=%v src=%s", m.ID, m.Name, m.WorkshopID, m.Broken, m.Require, m.Source)
		}
	}
	j, _ := json.MarshalIndent(c.Traits[:3], "", " ")
	t.Log(string(j))
	j, _ = json.MarshalIndent(c.Professions[:2], "", " ")
	t.Log(string(j))
	t.Logf("hair m %d f %d beards %d voices %d clothing jobs %d", len(c.Looks.HairMale), len(c.Looks.HairFemale), len(c.Looks.Beards), len(c.Looks.Voices), len(c.Looks.Clothing))
	j, _ = json.Marshal(c.Clothing["Base.Tshirt_DefaultTEXTURE_TINT"])
	t.Log(string(j))
	j, _ = json.Marshal(c.Clothing["Base.Jacket_Black"])
	t.Log(string(j))
	t.Log(len(c.Sandbox), len(c.Server))
	s.Refresh(false)
}
