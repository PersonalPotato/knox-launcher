package main

// The shelf: every solo save and every world you host, as a tape. The back of each box comes
// from the games Knox Launcher recorded in that world.

import (
	"sort"
	"strconv"
	"strings"
)

type ShelfTape struct {
	Kind     string   `json:"kind"` // "solo" or "world"
	Mode     string   `json:"mode,omitempty"`
	Name     string   `json:"name"`
	Played   int64    `json:"played"`
	Thumb    bool     `json:"thumb,omitempty"`
	Games    int      `json:"games"`
	Kills    int      `json:"kills"`
	Deaths   int      `json:"deaths"`
	BestDay  int      `json:"bestDay"`
	Hours    float64  `json:"hours"`
	Cast     []string `json:"cast,omitempty"` // everyone who played in it, most games first
	Story    string   `json:"story,omitempty"`
	Town     string   `json:"town,omitempty"`
	Backups  int      `json:"backups,omitempty"`
	Title    string   `json:"title,omitempty"` // the tape's own title (tapes.go)
	Look     *Look    `json:"look,omitempty"`  // the tape's own cover; nil = your default look
	Fresh    bool     `json:"fresh,omitempty"` // made on the shelf, not played yet
	Host     string   `json:"host,omitempty"`  // guest tapes: the host's Steam ID and name
	HostName string   `json:"hostName,omitempty"`
}

func (a *App) shelf() []ShelfTape {
	games := a.cfg.Get().Games
	fill := func(t *ShelfTape, match func(GameRecord) bool) { a.fillTape(t, games, match) }
	me := ""
	if a.steam != nil {
		me = strconv.FormatUint(a.steam.MyID(), 10)
	}
	var out []ShelfTape
	for _, s := range a.saves() {
		t := ShelfTape{Kind: "solo", Mode: s.Mode, Name: s.Name, Played: s.Played, Thumb: s.Thumb}
		fill(&t, func(g GameRecord) bool { return strings.EqualFold(g.World, s.Name) })
		out = append(out, t)
	}
	for _, w := range a.worlds() {
		if !w.Saved {
			continue
		}
		t := ShelfTape{Kind: "world", Name: w.Name, Played: w.Played, Backups: w.Backups}
		fill(&t, func(g GameRecord) bool { return g.World == w.Name && (g.Host == "" || g.Host == me) })
		out = append(out, t)
	}
	metas := a.cfg.Get().Tapes
	seen := map[string]bool{}
	for i := range out {
		k := tapeKey(out[i].Kind, out[i].Mode, out[i].Name)
		seen[k] = true
		if m, ok := metas[k]; ok {
			l := m.Look
			out[i].Title, out[i].Look = m.Title, &l
		}
	}
	for k, m := range metas {
		if seen[k] {
			continue
		}
		if m.Kind == "world" && a.worldExists(m.Name) {
			continue // saved but not listed (binned or moved): leave it off
		}
		l := m.Look
		out = append(out, ShelfTape{Kind: m.Kind, Mode: m.Mode, Name: m.Name, Played: m.Made, Title: m.Title, Look: &l, Fresh: true})
	}
	out = append(out, a.guestTapes(games)...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Played > out[j].Played })
	return out
}

// guestTapes: worlds friends hosted that you played in. Their launchers send the game records.
func (a *App) guestTapes(games []GameRecord) []ShelfTape {
	if a.steam == nil {
		return nil
	}
	me := strconv.FormatUint(a.steam.MyID(), 10)
	byKey := map[string]*ShelfTape{}
	var keys []string
	for _, g := range games {
		if g.Host == "" || g.Host == me || g.World == "" {
			continue
		}
		in := false
		for _, p := range g.Players {
			in = in || p.ID == me
		}
		if !in {
			continue
		}
		k := g.Host + "/" + g.World
		t := byKey[k]
		if t == nil {
			t = &ShelfTape{Kind: "guest", Name: g.World, Host: g.Host}
			byKey[k] = t
			keys = append(keys, k)
		}
		if g.Time > t.Played {
			t.Played = g.Time
			if g.Title != "" {
				t.Title = g.Title
			}
		}
		for _, p := range g.Players {
			if p.ID == g.Host && p.Name != "" {
				t.HostName = p.Name
			}
		}
	}
	var out []ShelfTape
	for _, k := range keys {
		t := byKey[k]
		a.fillTape(t, games, func(g GameRecord) bool { return g.Host == t.Host && g.World == t.Name })
		out = append(out, *t)
	}
	return out
}

// fillTape adds up the games that match into the tape's stats and cast.
func (a *App) fillTape(t *ShelfTape, games []GameRecord, match func(GameRecord) bool) {
	count := map[string]int{}
	for _, g := range games {
		if !match(g) {
			continue
		}
		t.Games++
		t.BestDay = max(t.BestDay, g.Day)
		t.Hours += float64(g.Mins) / 60
		if g.Story != "" {
			t.Story = g.Story
		}
		if g.Town != "" {
			t.Town = g.Town
		}
		for _, p := range g.Players {
			t.Kills += p.Kills
			t.Deaths += p.Deaths
			if p.Name != "" {
				count[p.Name]++
			}
		}
	}
	for n := range count {
		t.Cast = append(t.Cast, n)
	}
	sort.Slice(t.Cast, func(i, j int) bool {
		if count[t.Cast[i]] != count[t.Cast[j]] {
			return count[t.Cast[i]] > count[t.Cast[j]]
		}
		return t.Cast[i] < t.Cast[j]
	})
}
