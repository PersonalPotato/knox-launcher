package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The bin: tapes taken off the shelf. Nothing is deleted, the files move to Zomboid\KnoxBin
// (same drive as the saves, so a move is instant) and can be put back from the Shelf page.

type BinTape struct {
	ID     string    `json:"id"`
	Kind   string    `json:"kind"` // "solo" | "world"
	Mode   string    `json:"mode,omitempty"`
	Name   string    `json:"name"`
	Binned int64     `json:"binned"`
	Meta   *TapeMeta `json:"meta,omitempty"` // its title and cover, put back with it
}

// world files that live in Zomboid\Server next to <world>.ini
var worldFileSuffixes = []string{".ini", "_SandboxVars.lua", "_spawnregions.lua", "_spawnpoints.lua"}

func (a *App) binDir() string { return filepath.Join(a.paths.Zomboid, "KnoxBin") }

func (a *App) binReady() error {
	if a.gameState().Running {
		return errors.New("close Zomboid first")
	}
	if p := a.lobby.Phase(); p == PhaseSyncing || p == PhaseLaunching || p == PhaseInGame {
		return errors.New("a game is starting, try again after")
	}
	return nil
}

func badName(s string) bool {
	return s == "" || strings.ContainsAny(s, `/\`) || strings.Contains(s, "..")
}

func (a *App) binTape(kind, mode, name string) error {
	if err := a.binReady(); err != nil {
		return err
	}
	var moves [][2]string // from, to (relative to the tape's bin folder)
	switch kind {
	case "solo":
		p, err := a.savePath(mode, name)
		if err != nil {
			return err
		}
		moves = append(moves, [2]string{p, "save"})
	case "world":
		if badName(name) {
			return errors.New("unknown world")
		}
		if h := a.cfg.Get().Host; a.lobby.Phase() != PhaseIdle && h.World == name {
			return errors.New("your lobby is using this world. Leave the lobby or pick another world first")
		}
		for _, s := range worldFileSuffixes {
			if f := filepath.Join(a.paths.serverDir(), name+s); exists(f) {
				moves = append(moves, [2]string{f, filepath.Join("server", name+s)})
			}
		}
		if d := filepath.Join(a.paths.savesDir(), name); exists(d) {
			moves = append(moves, [2]string{d, "save"})
		}

	default:
		return errors.New("unknown tape")
	}
	if len(moves) == 0 {
		return errors.New("that tape is already gone")
	}
	t := BinTape{Kind: kind, Mode: mode, Name: name, Binned: time.Now().Unix()}
	t.ID = fmt.Sprintf("%d-%s-%s", t.Binned, kind, name)
	dir := filepath.Join(a.binDir(), t.ID)
	if err := os.MkdirAll(filepath.Join(dir, "server"), 0755); err != nil {
		return err
	}
	for i, m := range moves {
		if err := os.Rename(m[0], filepath.Join(dir, m[1])); err != nil {
			for _, back := range moves[:i] { // put back what already moved
				os.Rename(filepath.Join(dir, back[1]), back[0])
			}
			os.RemoveAll(dir)
			return fmt.Errorf("couldn't move %s (is it open somewhere?)", filepath.Base(m[0]))
		}
	}
	if kind == "world" {
		// its cast and goals go with it (best effort: they may be on another drive), so a new world
		// with the same name starts clean
		for _, f := range []string{a.castFile(name), a.goalsFile(name)} {
			if exists(f) {
				_ = os.Rename(f, filepath.Join(dir, "server", filepath.Base(f)))
			}
		}
	}
	key := tapeKey(kind, mode, name)
	if m, ok := a.cfg.Get().Tapes[key]; ok {
		t.Meta = &m
		_ = a.cfg.Update(func(c *Config) { delete(c.Tapes, key) })
	}
	b, _ := json.MarshalIndent(t, "", "  ")
	return os.WriteFile(filepath.Join(dir, "tape.json"), b, 0644)
}

func (a *App) binList() []BinTape {
	out := []BinTape{}
	dirs, _ := os.ReadDir(a.binDir())
	for _, d := range dirs {
		b, err := os.ReadFile(filepath.Join(a.binDir(), d.Name(), "tape.json"))
		var t BinTape
		if err != nil || json.Unmarshal(b, &t) != nil {
			continue
		}
		t.ID = d.Name()
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Binned > out[j].Binned })
	return out
}

func (a *App) restoreTape(id string) error {
	if badName(id) {
		return errors.New("unknown tape")
	}
	dir := filepath.Join(a.binDir(), id)
	b, err := os.ReadFile(filepath.Join(dir, "tape.json"))
	var t BinTape
	if err != nil || json.Unmarshal(b, &t) != nil || badName(t.Name) || (t.Kind == "solo" && badName(t.Mode)) {
		return errors.New("that tape is gone from the bin")
	}
	var moves [][2]string // from (in bin), to
	if t.Kind == "solo" {
		moves = append(moves, [2]string{filepath.Join(dir, "save"), filepath.Join(a.paths.soloSavesDir(), t.Mode, t.Name)})
	} else {
		for _, s := range worldFileSuffixes {
			moves = append(moves, [2]string{filepath.Join(dir, "server", t.Name+s), filepath.Join(a.paths.serverDir(), t.Name+s)})
		}
		moves = append(moves, [2]string{filepath.Join(dir, "save"), filepath.Join(a.paths.savesDir(), t.Name)})
	}
	for _, m := range moves {
		if exists(m[0]) && exists(m[1]) {
			return fmt.Errorf("there's already a %s called %s. Rename or bin that one first", map[bool]string{true: "save", false: "world"}[t.Kind == "solo"], t.Name)
		}
	}
	for _, m := range moves {
		if !exists(m[0]) {
			continue
		}
		os.MkdirAll(filepath.Dir(m[1]), 0755)
		if err := os.Rename(m[0], m[1]); err != nil {
			return fmt.Errorf("couldn't put back %s", filepath.Base(m[1]))
		}
	}
	if t.Kind == "world" {
		for _, f := range []string{a.castFile(t.Name), a.goalsFile(t.Name)} {
			if from := filepath.Join(dir, "server", filepath.Base(f)); exists(from) {
				_ = os.MkdirAll(filepath.Dir(f), 0755)
				_ = os.Remove(f)
				_ = os.Rename(from, f)
			}
		}
	}
	if t.Meta != nil {
		_ = a.cfg.Update(func(c *Config) {
			if c.Tapes == nil {
				c.Tapes = map[string]TapeMeta{}
			}
			c.Tapes[tapeKey(t.Kind, t.Mode, t.Name)] = *t.Meta
		})
	}
	return os.RemoveAll(dir)
}
