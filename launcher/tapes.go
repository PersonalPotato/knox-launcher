package main

// Tapes: each solo save and hosted world can have its own title and cover look. A tape made on the
// shelf ("New tape") is kept here until its world is first played, so it sits on the shelf already.

import (
	"encoding/json"
	"errors"
	"strings"
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
		l.Genre, l.Backdrop, l.Worn, l.Foil, l.WornMix, l.FoilMix = t.Look.Genre, t.Look.Backdrop, t.Look.Worn, t.Look.Foil, false, false
		l.Spine, l.SpineInk = t.Look.Spine, t.Look.SpineInk
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
