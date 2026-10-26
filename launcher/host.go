package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Host settings changes that come from the UI: small patches, presets, loadouts and backstories.

// patchHost applies only the fields the UI sent. "ini" and "sandbox" are merged key by key,
// and an empty value puts that option back to what the world already has.
func (a *App) patchHost(raw json.RawMessage) error {
	if len(raw) == 0 {
		return errors.New("nothing to change")
	}
	var patch map[string]json.RawMessage
	if err := json.Unmarshal(raw, &patch); err != nil {
		return err
	}
	return a.cfg.Update(func(c *Config) {
		for _, key := range []string{"ini", "sandbox"} {
			v, ok := patch[key]
			if !ok {
				continue
			}
			delete(patch, key)
			var m map[string]string
			if json.Unmarshal(v, &m) != nil {
				continue
			}
			dst := c.Host.Ini
			if key == "sandbox" {
				dst = c.Host.Sandbox
			}
			for k, val := range m {
				if val == "" {
					delete(dst, k)
				} else {
					dst[k] = val
				}
			}
		}
		b, _ := json.Marshal(c.Host)
		var cur map[string]json.RawMessage
		_ = json.Unmarshal(b, &cur)
		for k, v := range patch {
			cur[k] = v
		}
		b, _ = json.Marshal(cur)
		var h HostSettings
		if json.Unmarshal(b, &h) == nil {
			if h.Ini == nil {
				h.Ini = map[string]string{}
			}
			if h.Sandbox == nil {
				h.Sandbox = map[string]string{}
			}
			c.Host = h
		}
	})
}

var presetCache struct {
	sync.Mutex
	version string
	list    []DifficultyPreset
}

// difficultyPresets: the game's presets, Knox's, and the player's saved ones. Only options the
// installed game (and mods) actually have are kept.
// difficultyPresets: the game's and Knox presets, then the settings presets you saved in the launcher.
func (a *App) difficultyPresets() []DifficultyPreset {
	base := a.builtinDifficulty()
	out := append([]DifficultyPreset{}, base...)
	mine := a.cfg.Get().Settings
	names := make([]string, 0, len(mine))
	for n := range mine {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		sp := mine[n]
		out = append(out, DifficultyPreset{ID: "mine:" + n, Name: n, Tagline: "Saved in the launcher", Explain: fmt.Sprintf("%d world settings, %d server settings.", len(sp.Sandbox), len(sp.Ini)), Values: sp.Sandbox, Ini: sp.Ini, Custom: true, Mine: true})
	}
	return out
}

func (a *App) builtinDifficulty() []DifficultyPreset {
	c := a.catalog()
	ver := ""
	if c != nil {
		ver = c.Version
	}
	presetCache.Lock()
	defer presetCache.Unlock()
	if presetCache.list != nil && presetCache.version == ver {
		return presetCache.list
	}
	var known map[string]bool
	if c != nil {
		var pages []struct {
			Settings []struct {
				Name string `json:"name"`
			} `json:"settings"`
		}
		if json.Unmarshal(c.Sandbox, &pages) == nil {
			known = map[string]bool{}
			for _, pg := range pages {
				for _, s := range pg.Settings {
					known[s.Name] = true
				}
			}
			if len(known) < 20 { // option list missing: trust the preset files
				known = nil
			}
		}
	}
	presetCache.list = loadDifficultyPresets(a.paths, known)
	presetCache.version = ver
	return presetCache.list
}

func (a *App) difficultyPreset(id string) *DifficultyPreset {
	for _, p := range a.difficultyPresets() {
		if p.ID == id {
			p := p
			return &p
		}
	}
	return nil
}

func (a *App) applyDifficulty(id string) error {
	p := a.difficultyPreset(id)
	if p == nil {
		return errors.New("unknown preset")
	}
	return a.cfg.Update(func(c *Config) {
		c.Host.Difficulty = id
		c.Host.Sandbox = map[string]string{}
		for k, v := range p.Values {
			c.Host.Sandbox[k] = v
		}
		if p.Mine {
			c.Host.Ini = map[string]string{}
			for k, v := range p.Ini {
				c.Host.Ini[k] = v
			}
		}
	})
}

func (a *App) applyLoadout(id string) error {
	cfg := a.cfg.Get()
	for _, l := range allLoadouts(a.catalog(), cfg.Loadouts) {
		if l.ID == id {
			return a.cfg.Update(func(c *Config) { c.Host.Kit, c.Host.KitName = l.Items, l.Name })
		}
	}
	return errors.New("unknown loadout")
}

// applyBackstory sets the squad's story: where everyone starts and what they carry.
func (a *App) applyBackstory(id string) error {
	if id == "" {
		return a.cfg.Update(func(c *Config) { c.Host.Backstory = "" })
	}
	b := a.storyByID(id)
	if b == nil {
		return errors.New("unknown backstory")
	}
	if b.Mod != "" && !installedPacks(a.catalog())[b.Mod] {
		return errors.New(b.Name + " needs a mod you don't have")
	}
	town := ""
	if b.Town != "" {
		for _, t := range a.towns() {
			if strings.Contains(strings.ToLower(t.Name), strings.ToLower(b.Town)) {
				town = t.Name
				break
			}
		}
	}
	spot, _ := a.RandomHouse(town, b.Building)
	if err := a.cfg.Update(func(c *Config) {
		c.Host.Backstory = id
		c.Host.Together = true
		c.Host.SpawnMode, c.Host.SpawnType, c.Host.SpawnTown = "random", b.Building, town
		if spot != nil {
			c.Host.Spawn = spot
		}
	}); err != nil {
		return err
	}
	if b.Loadout != "" {
		return a.applyLoadout(b.Loadout)
	}
	return nil
}

var mediaCache struct {
	sync.Mutex
	version string
	list    []string
}

// medias lists the game's media folder and every mod's, lowest priority first.
func (a *App) medias() []string {
	c := a.catalog()
	ver := ""
	if c != nil {
		ver = c.Version
	}
	mediaCache.Lock()
	defer mediaCache.Unlock()
	if mediaCache.list != nil && mediaCache.version == ver {
		return mediaCache.list
	}
	list := []string{filepath.Join(a.paths.Game, "media")}
	if c != nil {
		for _, m := range c.Mods {
			if m.Broken != "" {
				continue
			}
			for _, r := range m.roots {
				if exists(filepath.Join(r, "media")) {
					list = append(list, filepath.Join(r, "media"))
				}
			}
		}
	}
	mediaCache.list, mediaCache.version = list, ver
	return list
}
