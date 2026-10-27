package main

// Solo games run as a normal single-player game: no server. Knox Link reads app_launch.txt on the
// main menu and either makes a new Sandbox world (mode=solo) or loads a save (mode=load).

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type SaveEntry struct {
	Mode   string `json:"mode"`
	Name   string `json:"name"`
	Played int64  `json:"played"`
	Thumb  bool   `json:"thumb"`
}

func (p Paths) soloSavesDir() string { return filepath.Join(p.Zomboid, "Saves") }

// saves lists single-player saves, newest first. Multiplayer saves belong to servers.
func (a *App) saves() []SaveEntry {
	var out []SaveEntry
	modes, _ := os.ReadDir(a.paths.soloSavesDir())
	for _, m := range modes {
		if !m.IsDir() || m.Name() == "Multiplayer" {
			continue
		}
		dir := filepath.Join(a.paths.soloSavesDir(), m.Name())
		worlds, _ := os.ReadDir(dir)
		for _, w := range worlds {
			if !w.IsDir() {
				continue
			}
			base := filepath.Join(dir, w.Name())
			if !exists(filepath.Join(base, "map_t.bin")) && !exists(filepath.Join(base, "map_ver.bin")) {
				continue // not a finished save
			}
			e := SaveEntry{Mode: m.Name(), Name: w.Name(), Thumb: exists(filepath.Join(base, "thumb.png"))}
			if st, err := os.Stat(base); err == nil {
				e.Played = st.ModTime().Unix()
			}
			for _, f := range []string{"map_t.bin", "map_p.bin"} {
				if st, err := os.Stat(filepath.Join(base, f)); err == nil && st.ModTime().Unix() > e.Played {
					e.Played = st.ModTime().Unix()
				}
			}
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Played > out[j].Played })
	return out
}

func (a *App) savePath(mode, name string) (string, error) {
	if mode == "" || name == "" || mode == "Multiplayer" || strings.ContainsAny(mode+name, `/\`) || strings.Contains(mode+name, "..") {
		return "", errors.New("unknown save")
	}
	p := filepath.Join(a.paths.soloSavesDir(), mode, name)
	if !exists(p) {
		return "", errors.New("that save is gone")
	}
	return p, nil
}

func (a *App) soloReady() error {
	if a.catalog() == nil {
		return errors.New("still loading, try again")
	}
	if a.gameState().Running {
		return errors.New("Zomboid is already running. Close it first.")
	}
	if p := a.lobby.Phase(); p == PhaseSyncing || p == PhaseLaunching || p == PhaseInGame {
		return errors.New("already starting")
	}
	if a.lobby.ID() != 0 {
		return errors.New("leave the lobby first")
	}
	return nil
}

// soloName picks a world name nobody has used yet.
func (a *App) soloName(want string) string {
	want = strings.TrimSpace(strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|=;`, r) || r < 32 {
			return -1
		}
		return r
	}, want))
	if want == "" {
		want = "Solo " + time.Now().Format("Jan 2")
	}
	name := want
	for i := 2; exists(filepath.Join(a.paths.soloSavesDir(), "Sandbox", name)); i++ {
		name = fmt.Sprintf("%s (%d)", want, i)
	}
	return name
}

// SoloStart makes a new single-player world with your setup: mods, world settings, start building,
// loadout and survivor.
func (a *App) SoloStart(name string) (string, error) {
	if err := a.soloReady(); err != nil {
		return "", err
	}
	cfg := a.cfg.Get()
	h := cfg.Host
	if h.Together && h.Spawn == nil {
		if spot, err := a.RandomHouse(h.SpawnTown, h.SpawnType); err == nil {
			_ = a.cfg.Update(func(c *Config) { c.Host.Spawn = spot })
			h.Spawn = spot
		}
	}
	world := a.soloName(name)

	// world settings: the difficulty preset, the host world's file, then the launcher's own changes
	sb := map[string]string{}
	if p := a.difficultyPreset(h.Difficulty); p != nil {
		for k, v := range p.Values {
			sb[k] = v
		}
	}
	for k, v := range a.readSandbox(h.World) {
		sb[k] = v
	}
	for k, v := range h.Sandbox {
		sb[k] = v
	}
	delete(sb, "WaterShutModifier") // the game rolls these from WaterShut/ElecShut
	delete(sb, "ElecShutModifier")
	keys := make([]string, 0, len(sb))
	for k := range sb {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var lines []string
	for _, k := range keys {
		lines = append(lines, "sandbox."+k+"="+sb[k])
	}
	spawn := ""
	if h.Together && h.Spawn != nil {
		spawn = fmt.Sprintf("%d,%d,%d", h.Spawn.X, h.Spawn.Y, h.Spawn.Z)
	}
	// read in game once the new character exists
	lines = append(lines, "world="+world, "spawn="+spawn, "kit="+kitText(h.Kit))

	a.lobby.set(func() { a.lobby.phase, a.lobby.note, a.lobby.cancel = PhaseSyncing, "", false })
	go func() {
		if err := a.soloLaunch(lines, world); err != nil {
			a.lobby.set(func() { a.lobby.phase, a.lobby.note, a.lobby.sync.Wait = a.lobby.restPhase(), err.Error(), "" })
		}
	}()
	return world, nil
}

func (a *App) soloLaunch(lines []string, world string) error {
	cfg := a.cfg.Get()
	ids, _ := a.hostModList()
	m := buildManifest(a.catalog(), a.steam, ids)
	if err := a.syncItems(m, true); err != nil {
		return err
	}
	if err := writeDefaultMods(a.paths, ids); err != nil {
		return fmt.Errorf("couldn't write the mod list: %v", err)
	}
	if err := writeGamePresets(a.paths, cfg); err != nil {
		return fmt.Errorf("couldn't save your character: %v", err)
	}
	if err := a.writeBridge("app_solo.txt", strings.Join(lines, "\r\n")+"\r\n"); err != nil {
		return err
	}
	launch := map[string]string{
		"time":      strconv.FormatInt(time.Now().Unix(), 10),
		"mode":      "solo",
		"world":     world,
		"character": gamePresetName,
	}
	if err := a.writeBridge("app_launch.txt", kv(launch)); err != nil {
		return err
	}
	return a.launchGame(false)
}

// LoadSave opens the game straight into a single-player save. The save keeps its own mod list.
func (a *App) LoadSave(mode, name string) error {
	if err := a.soloReady(); err != nil {
		return err
	}
	if _, err := a.savePath(mode, name); err != nil {
		return err
	}
	_ = a.writeBridge("app_solo.txt", "")
	launch := map[string]string{
		"time":     strconv.FormatInt(time.Now().Unix(), 10),
		"mode":     "load",
		"gameMode": mode,
		"save":     name,
	}
	if err := a.writeBridge("app_launch.txt", kv(launch)); err != nil {
		return err
	}
	return a.launchGame(false)
}
