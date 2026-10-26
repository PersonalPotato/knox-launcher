package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// GameData is app_gamedata.json, written by Knox Link every time the game reaches the main menu.
type GameData struct {
	raw     json.RawMessage
	Time    int64     `json:"time"`
	Game    string    `json:"game"`
	SteamID string    `json:"steamID"`
	Mods    []ModInfo `json:"mods"`
	Worlds  []struct {
		Name   string `json:"name"`
		Exists bool   `json:"exists"`
	} `json:"worlds"`
	Spawns []struct {
		Name   string `json:"name"`
		Points []struct {
			X, Y, Z int
		} `json:"points"`
	} `json:"spawns"`
}

type ModInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	WorkshopID   string   `json:"workshopID"`
	Dir          string   `json:"dir"`
	Description  string   `json:"description"`
	Posters      []string `json:"posters"`
	Require      []string `json:"require"`
	Incompatible []string `json:"incompatible"`
	Broken       string   `json:"broken"`
	Removed      bool     `json:"removed"`
	On           bool     `json:"on"`
	Order        int      `json:"order"`
	Source       string   `json:"source,omitempty"` // local | upload | workshop
	Author       string   `json:"author,omitempty"`
	Version      string   `json:"version,omitempty"`
	LoadAfter    []string `json:"loadAfter,omitempty"`
	Java         bool     `json:"java,omitempty"`
	Pending      bool     `json:"pending,omitempty"` // subscribed, still downloading
	roots        []string // content folders (common + version folder)
	jar          string   // Java mods: the jar ZombieBuddy loads
}

func loadGameData(p Paths) (*GameData, error) {
	b, err := os.ReadFile(p.bridge("app_gamedata.json"))
	if err != nil {
		return nil, err
	}
	var g GameData
	if err := json.Unmarshal(b, &g); err != nil {
		return nil, fmt.Errorf("app_gamedata.json: %v", err)
	}
	g.raw = b
	return &g, nil
}

func (g *GameData) mod(id string) *ModInfo {
	for i := range g.Mods {
		if g.Mods[i].ID == id {
			return &g.Mods[i]
		}
	}
	return nil
}

// Mods that must load first (ZombieBuddy patches the game before anything else).
var pinnedMods = []string{"ZombieBuddy", "zdk", "ZModUnbork"}

// Knox Link is the in-game half of the launcher, so it's always on.
const knoxLinkID = "KnoxLink"

// oldKnoxLinkID is the id Knox Link had before 2026-10-26. Old configs and pack codes still name it.
const oldKnoxLinkID = "EasyHost"

func (c *Catalog) mod(id string) *ModInfo { return c.modByID[id] }

func (c *Catalog) usable(id string) bool {
	m := c.mod(id)
	return m != nil && m.Broken == "" && !m.Pending
}

// requirementsOf lists everything a mod needs, all the way down.
func (c *Catalog) requirementsOf(id string) (have []string, missing []string) {
	seen := map[string]bool{id: true}
	var walk func(id string)
	walk = func(id string) {
		m := c.mod(id)
		if m == nil {
			return
		}
		for _, r := range m.Require {
			if seen[r] {
				continue
			}
			seen[r] = true
			if c.usable(r) {
				have = append(have, r)
				walk(r)
			} else {
				missing = append(missing, r)
			}
		}
	}
	walk(id)
	return
}

// dependentsOf lists every mod (from the set) that needs id, directly or not.
func (c *Catalog) dependentsOf(id string, set map[string]bool) []string {
	var out []string
	seen := map[string]bool{id: true}
	for changed := true; changed; {
		changed = false
		for other := range set {
			if seen[other] {
				continue
			}
			m := c.mod(other)
			if m == nil {
				continue
			}
			for _, r := range m.Require {
				if seen[r] {
					seen[other] = true
					out = append(out, other)
					changed = true
					break
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// resolveMods turns a wish list into a list the game can load: requirements are added, mods that
// can't load (broken, missing a requirement, still downloading) are left out and explained.
func resolveMods(c *Catalog, want []string) (ids []string, problems map[string]string) {
	problems = map[string]string{}
	set := map[string]bool{}
	for _, id := range want {
		if id == oldKnoxLinkID {
			continue
		}
		if !c.usable(id) {
			if m := c.mod(id); m != nil && m.Broken != "" {
				problems[id] = m.Broken
			} else if m != nil && m.Pending {
				problems[id] = "Still downloading"
			}
			continue
		}
		have, missing := c.requirementsOf(id)
		if len(missing) > 0 {
			problems[id] = "Needs " + strings.Join(missing, ", ") + " (not installed)"
			continue
		}
		set[id] = true
		for _, r := range have {
			set[r] = true
		}
	}
	if c.usable(knoxLinkID) {
		set[knoxLinkID] = true
	}
	for id := range set {
		for _, x := range c.mod(id).Incompatible {
			if set[x] {
				problems[id] = "Doesn't work together with " + x
			}
		}
	}
	return orderMods(c, set), problems
}

// orderMods sorts a set into a load order: pinned mods first, then the order the player had,
// with every requirement (and loadModAfter) placed before the mods that need it.
func orderMods(c *Catalog, set map[string]bool) []string {
	ids := make([]string, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	rank := func(id string) int {
		for i, p := range pinnedMods {
			if p == id {
				return -100 + i
			}
		}
		if m := c.mod(id); m != nil && m.Order > 0 {
			return m.Order
		}
		return 10000
	}
	sort.SliceStable(ids, func(a, b int) bool {
		ra, rb := rank(ids[a]), rank(ids[b])
		if ra != rb {
			return ra < rb
		}
		return strings.ToLower(ids[a]) < strings.ToLower(ids[b])
	})
	placed := map[string]bool{}
	var out []string
	var visit func(id string, depth int)
	visit = func(id string, depth int) {
		if placed[id] || depth > 50 {
			return
		}
		if m := c.mod(id); m != nil {
			for _, r := range append(append([]string{}, m.Require...), m.LoadAfter...) {
				if set[r] {
					visit(r, depth+1)
				}
			}
		}
		if !placed[id] {
			placed[id] = true
			out = append(out, id)
		}
	}
	for _, id := range ids {
		visit(id, 0)
	}
	return out
}

var defaultModRe = regexp.MustCompile(`(?m)^\s*mod\s*=\s*\\?([^,\r\n]+?)\s*,?\s*$`)

// readDefaultMods reads the mod list the game itself last used (Zomboid/mods/default.txt).
func readDefaultMods(p Paths) []string {
	b, err := os.ReadFile(filepath.Join(p.Zomboid, "mods", "default.txt"))
	if err != nil {
		return nil
	}
	var out []string
	for _, m := range defaultModRe.FindAllStringSubmatch(string(b), -1) {
		out = append(out, strings.TrimSpace(m[1]))
	}
	return out
}

// writeDefaultMods writes Zomboid/mods/default.txt, which the game reads at start-up
// (Java mods via ZombieBuddy only load from this list, so it has to be right before the game opens).
func writeDefaultMods(p Paths, ids []string) error {
	path := filepath.Join(p.Zomboid, "mods", "default.txt")
	backup := path + ".before-ezhost"
	if !exists(backup) {
		if b, err := os.ReadFile(path); err == nil {
			_ = os.WriteFile(backup, b, 0644)
		}
	}
	var b strings.Builder
	b.WriteString("VERSION = 1,\r\n\r\nmods\r\n{\r\n")
	for _, id := range ids {
		b.WriteString("    mod = " + id + ",\r\n")
	}
	b.WriteString("}\r\n\r\nmaps\r\n{\r\n}\r\n")
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(b.String()), 0644)
}

// The host's mod manifest, shared through the lobby.
type Manifest struct {
	Mods  []string       `json:"m"`           // load order
	Items []ManifestItem `json:"i"`           // Workshop items to have
	Local []string       `json:"l,omitempty"` // mods that aren't on the Workshop
}

type ManifestItem struct {
	ID   string `json:"id"`
	TS   uint32 `json:"ts"`
	Name string `json:"n"`
}

func buildManifest(c *Catalog, st Steam, ids []string) Manifest {
	m := Manifest{Mods: ids}
	seen := map[string]bool{}
	for _, id := range ids {
		info := c.mod(id)
		if info == nil {
			continue
		}
		if info.WorkshopID == "" || info.Removed {
			m.Local = append(m.Local, id)
			continue
		}
		if seen[info.WorkshopID] {
			continue
		}
		seen[info.WorkshopID] = true
		item := ManifestItem{ID: info.WorkshopID, Name: info.Name}
		if wid, err := strconv.ParseUint(info.WorkshopID, 10, 64); err == nil {
			if _, ts, ok := st.ItemInstallInfo(wid); ok {
				item.TS = ts
			}
		}
		m.Items = append(m.Items, item)
	}
	return m
}
