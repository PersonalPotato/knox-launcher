//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testCatalog(mods ...ModInfo) *Catalog {
	c := &Catalog{modByID: map[string]*ModInfo{}}
	c.Mods = mods
	for i := range c.Mods {
		c.modByID[c.Mods[i].ID] = &c.Mods[i]
	}
	return c
}

// Two mods that can't run together: the first in the load order stays, the other and its dependents go.
func TestResolveIncompatible(t *testing.T) {
	c := testCatalog(
		ModInfo{ID: knoxLinkID, Name: "Knox Link"},
		ModInfo{ID: "A", Name: "Mod A", Order: 1, Incompatible: []string{"B"}},
		ModInfo{ID: "B", Name: "Mod B", Order: 2},
		ModInfo{ID: "C", Name: "Mod C", Order: 3, Require: []string{"B"}},
		ModInfo{ID: "D", Name: "Mod D", Order: 4, Incompatible: []string{knoxLinkID}},
	)
	ids, problems := resolveMods(c, []string{"A", "B", "C", "D"})
	got := strings.Join(ids, ",")
	if strings.Contains(got, "B") || strings.Contains(got, "C") || strings.Contains(got, "D") || !strings.Contains(got, "A") || !strings.Contains(got, knoxLinkID) {
		t.Fatalf("load list %s", got)
	}
	for _, id := range []string{"B", "C", "D"} {
		if problems[id] == "" {
			t.Errorf("no reason for leaving out %s: %v", id, problems)
		}
	}
	if problems["A"] != "" {
		t.Errorf("A has a problem: %s", problems["A"])
	}
}

// A guest's wait for mods ends when the host stops the start or the player cancels.
func TestSyncItemsStops(t *testing.T) {
	a := testApp(t)
	st := a.steam.(*mockSteam)
	a.lobby.set(func() { a.lobby.id = 77 })
	st.SetLobbyData(77, "state", "launching")
	st.SetLobbyData(77, "launch", "L1")
	st.items[123] = ItemSubscribed | ItemInstalled | ItemDownloading // a download that never ends
	m := Manifest{Items: []ManifestItem{{ID: "123", Name: "Slow mod"}}}
	errc := make(chan error, 1)
	go func() { errc <- a.syncItems(m, false) }()
	st.SetLobbyData(77, "state", "open")
	if err := <-errc; err == nil || !strings.Contains(err.Error(), "stopped") {
		t.Fatalf("want the host's start stopped, got %v", err)
	}
	st.SetLobbyData(77, "state", "launching")
	go func() { errc <- a.syncItems(m, false) }()
	a.lobby.Cancel()
	if err := <-errc; err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("want cancelled, got %v", err)
	}
}

func testApp(t *testing.T) *App {
	dir := t.TempDir()
	a := &App{paths: Paths{Zomboid: dir, Data: dir}, cfg: loadConfig(dir), steam: newSteam(""), server: &coopServer{}}
	a.lobby = &Lobby{app: a, phase: PhaseSyncing}
	return a
}

func TestConfigKeepsBadFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"host": {"world": 5}}`), 0644)
	s := loadConfig(dir)
	if s.cfg.Host.World == "" {
		t.Fatal("defaults lost")
	}
	m, _ := filepath.Glob(filepath.Join(dir, "config.json.unreadable-*"))
	if len(m) != 1 {
		t.Fatalf("bad config not kept: %v", m)
	}
}

// The host's own upload of a mod must match the Workshop copy friends download.
func TestOwnCopyMismatch(t *testing.T) {
	a := testApp(t)
	a.paths.Workshop = filepath.Join(a.paths.Zomboid, "steam")
	write := func(p, s string) {
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		_ = os.WriteFile(p, []byte(s), 0644)
	}
	up := filepath.Join(a.paths.Zomboid, "Workshop", "KnoxLauncher", "Contents", "mods", "KnoxLink")
	local := filepath.Join(a.paths.Zomboid, "mods", "KnoxLink")
	pub := filepath.Join(a.paths.Workshop, "999", "mods", "KnoxLink")
	for _, d := range []string{up, local, pub} {
		write(filepath.Join(d, "42", "media", "lua", "client", "A.lua"), "print(1)")
		write(filepath.Join(d, "42", "media", "scripts", "items.txt"), "item")
		write(filepath.Join(d, "42", "poster.png"), d) // images don't count
	}
	c := testCatalog(ModInfo{ID: knoxLinkID, Name: "Knox Link", Dir: local, Source: "local", WorkshopID: "999"})
	if err := a.ownCopyMismatch(c, []string{knoxLinkID}); err != nil {
		t.Fatalf("same files: %v", err)
	}
	write(filepath.Join(up, "42", "media", "lua", "client", "A.lua"), "print(2)")
	err := a.ownCopyMismatch(c, []string{knoxLinkID})
	if err == nil || !strings.Contains(err.Error(), filepath.Join("Workshop", "KnoxLauncher")+")") {
		t.Fatalf("changed upload: %v", err)
	}
	write(filepath.Join(pub, "42", "media", "lua", "client", "A.lua"), "print(2)")
	write(filepath.Join(pub, "42", "media", "lua", "client", "B.lua"), "new")
	if f := checksumDiff(up, pub); f != "42/media/lua/client/b.lua" {
		t.Fatalf("extra file: %q", f)
	}
}
