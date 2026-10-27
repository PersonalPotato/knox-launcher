package main

// Tapes: each solo save and hosted world can have its own title and cover look. A tape made on the
// shelf ("New tape") is kept here until its world is first played, so it sits on the shelf already.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TapeMeta struct {
	Kind  string `json:"kind"` // "solo" or "world"
	Mode  string `json:"mode,omitempty"`
	Name  string `json:"name"` // save or world name
	Title string `json:"title,omitempty"`
	Look  Look   `json:"look"`
	Made  int64  `json:"made"`
}

func tapeKey(kind, mode, name string) string {
	if kind == "world" {
		return "world/" + name
	}
	if mode == "" {
		mode = "Sandbox"
	}
	return "solo/" + mode + "/" + name
}

func (a *App) tapeCall(method string, args json.RawMessage) (any, bool, error) {
	switch method {
	case "saveTape", "dropTape":
	case "setPrefs":
		var in struct {
			Prefs Prefs `json:"prefs"`
		}
		if err := json.Unmarshal(args, &in); err != nil {
			return nil, true, err
		}
		if err := checkPrefs(in.Prefs, a.knoxProfile()); err != nil {
			return nil, true, err
		}
		return nil, true, a.cfg.Update(func(c *Config) { c.Prefs = in.Prefs })
	default:
		return nil, false, nil
	}
	var t TapeMeta
	if len(args) > 0 {
		if err := json.Unmarshal(args, &t); err != nil {
			return nil, true, err
		}
	}
	if t.Name == "" || (t.Kind != "solo" && t.Kind != "world") {
		return nil, true, errors.New("which tape?")
	}
	if t.Kind == "solo" && t.Mode == "" {
		t.Mode = "Sandbox"
	}
	key := tapeKey(t.Kind, t.Mode, t.Name)
	if method == "dropTape" {
		return nil, true, a.cfg.Update(func(c *Config) { delete(c.Tapes, key) })
	}
	if err := checkLook(t.Look, a.knoxProfile()); err != nil {
		return nil, true, err
	}
	t.Title = strings.TrimSpace(t.Title)
	if len([]rune(t.Title)) > 40 {
		t.Title = string([]rune(t.Title)[:40])
	}
	err := a.cfg.Update(func(c *Config) {
		if c.Tapes == nil {
			c.Tapes = map[string]TapeMeta{}
		}
		if old, ok := c.Tapes[key]; ok {
			t.Made = old.Made
		} else {
			t.Made = time.Now().Unix()
		}
		c.Tapes[key] = t
	})
	a.publishMyData()
	return t, true, err
}

// lobbyLook: your look as the lobby sees it. The cover parts come from the tape of the world you host.
func (a *App) lobbyLook() Look {
	cfg := a.cfg.Get()
	l := cfg.Look
	if t, ok := cfg.Tapes[tapeKey("world", "", cfg.Host.World)]; ok {
		// everything on the box comes from the tape; the poster parts stay yours
		box := t.Look
		box.Pose, box.Frame, box.Poster, box.PosterInk, box.Headline, box.HideNick, box.Plate, box.Collector = l.Pose, l.Frame, l.Poster, l.PosterInk, l.Headline, l.HideNick, l.Plate, l.Collector
		box.WornMix, box.FoilMix = false, false
		l = box
	}
	return l
}

// moveSoloTape: a new solo tape was made for a save name, but the game had to use another one.
func (a *App) moveSoloTape(want, got string) {
	if want == "" || want == got {
		return
	}
	_ = a.cfg.Update(func(c *Config) {
		from := tapeKey("solo", "Sandbox", want)
		if t, ok := c.Tapes[from]; ok {
			delete(c.Tapes, from)
			t.Name = got
			c.Tapes[tapeKey("solo", "Sandbox", got)] = t
		}
	})
}

// Guest copies: while you're in a friend's lobby, the launcher keeps their tape's title, cover look and
// cast (with what everyone wears), so the tape on your shelf looks like theirs when they're offline.

type GuestCopy struct {
	Host     string     `json:"host"` // Steam ID
	HostName string     `json:"hostName"`
	World    string     `json:"world"`
	Title    string     `json:"title,omitempty"`
	Look     *Look      `json:"look,omitempty"`
	Cast     *WorldCast `json:"cast,omitempty"`
	Seen     int64      `json:"seen"`
}

var guestSaved sync.Map // file -> last JSON written (without Seen)

func (a *App) guestFile(host, world string) string {
	return filepath.Join(a.paths.Data, "guests", clean(host)+"_"+clean(world)+".json")
}

func (a *App) keepGuestCopy(lobby uint64, info HostInfo) {
	if info.World == "" || a.steam == nil {
		return
	}
	owner := a.steam.LobbyOwner(lobby)
	if owner == 0 || owner == a.steam.MyID() {
		return
	}
	g := GuestCopy{Host: strconv.FormatUint(owner, 10), HostName: a.steam.LobbyData(lobby, "host"), World: info.World, Title: info.SquadName, Cast: info.Cast}
	var k struct {
		Look *Look `json:"look"`
	}
	if json.Unmarshal([]byte(a.steam.MemberData(lobby, owner, "knox")), &k) == nil {
		g.Look = k.Look
	}
	b, _ := json.Marshal(g)
	file := a.guestFile(g.Host, g.World)
	if old, ok := guestSaved.Load(file); ok && old.(string) == string(b) {
		return
	}
	guestSaved.Store(file, string(b))
	g.Seen = time.Now().Unix()
	b, _ = json.Marshal(g)
	_ = os.MkdirAll(filepath.Dir(file), 0755)
	_ = os.WriteFile(file, b, 0644)
}

func (a *App) guestCopies() []GuestCopy {
	var out []GuestCopy
	files, _ := filepath.Glob(filepath.Join(a.paths.Data, "guests", "*.json"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		var g GuestCopy
		if err != nil || json.Unmarshal(b, &g) != nil || g.Host == "" || g.World == "" {
			continue
		}
		out = append(out, g)
	}
	return out
}

// castLooks: up to 4 of a world's cast for the cover, in what they wore last. The living first, then most hours.
func castLooks(c *WorldCast) []CastLook {
	if c == nil {
		return nil
	}
	list := append([]CastMember(nil), c.Cast...)
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Alive != list[j].Alive {
			return list[i].Alive
		}
		return list[i].Hours > list[j].Hours
	})
	var out []CastLook
	for _, m := range list {
		if m.Look != nil && len(out) < 4 {
			out = append(out, *m.Look)
		}
	}
	return out
}
