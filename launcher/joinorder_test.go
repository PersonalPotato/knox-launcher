package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The game puts translation mods (and what they need) ahead of the server's list when it joins;
// the host's list has to be in that order already, and stay that way when fed back in.
func TestJoinOrder(t *testing.T) {
	c := &Catalog{modByID: map[string]*ModInfo{}}
	c.Mods = []ModInfo{
		{ID: "ZombieBuddy", Order: 1},
		{ID: "Viewpoint", Order: 2},
		{ID: "Hair", Order: 3},
		{ID: "HairPicker", Order: 4, Require: []string{"Hair"}, translation: true},
		{ID: "Other", Order: 5},
	}
	for i := range c.Mods {
		c.modByID[c.Mods[i].ID] = &c.Mods[i]
	}
	set := map[string]bool{}
	for _, m := range c.Mods {
		set[m.ID] = true
	}
	got := orderMods(c, set)
	want := "ZombieBuddy,Hair,HairPicker,Viewpoint,Other"
	if strings.Join(got, ",") != want {
		t.Fatalf("order %v, want %s", got, want)
	}
	if again := joinOrder(c, got); strings.Join(again, ",") != want {
		t.Fatalf("second pass %v", again)
	}
}

func TestIsTranslationMod(t *testing.T) {
	dir := t.TempDir()
	mk := func(rel string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		_ = os.WriteFile(p, []byte("x"), 0644)
	}
	// Improved Hair Menu's picker: only an empty media/lua/client folder
	empty := filepath.Join(dir, "picker", "42.17")
	_ = os.MkdirAll(filepath.Join(empty, "media", "lua", "client"), 0755)
	mk("picker/42.17/mod.info")
	if !isTranslationMod([]string{empty}) {
		t.Error("empty mod should count as a translation mod")
	}
	mk("tr/42/media/lua/shared/Translate/EN/UI_EN.txt")
	mk("tr/42/media/lua/shared/translate/de/ui.json")
	if !isTranslationMod([]string{filepath.Join(dir, "tr", "42")}) {
		t.Error("translate-only mod not detected")
	}
	mk("lua/42/media/lua/client/x.lua")
	if isTranslationMod([]string{filepath.Join(dir, "tr", "42"), filepath.Join(dir, "lua", "42")}) {
		t.Error("mod with Lua counted as a translation mod")
	}
}
