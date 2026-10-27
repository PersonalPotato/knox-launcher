package main

// Knox level, badges and lifetime squad stats. Looks only: nothing here changes the game.
//
// The host's launcher writes down every game it hosts (from Knox Link's app_recap.json) and shares
// the recent ones with the lobby, so every player's launcher builds the same history. A player's
// level, title and badges are worked out from that history plus what only they know (who they
// invited, who added their mod pack), and go out as lobby member data ("knox").

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type GamePlayer struct {
	ID     string             `json:"id,omitempty"` // Steam ID; "" if the name didn't match a lobby member
	Name   string             `json:"name"`
	Kills  int                `json:"kills"`
	Deaths int                `json:"deaths"`
	Hours  float64            `json:"hours"`           // hours the character had survived when the game closed
	Goals  []GoalDone         `json:"goals,omitempty"` // Knox goals finished this game (goals.go)
	Track  map[string]float64 `json:"track,omitempty"` // what Knox Link counted: walk, drive, bandOthers...
}

type GameRecord struct {
	ID      string       `json:"id"`
	Host    string       `json:"host"`
	Time    int64        `json:"time"`
	Mins    int          `json:"mins"`
	Day     int          `json:"day"`
	Story   string       `json:"story,omitempty"`
	World   string       `json:"world"`
	Title   string       `json:"title,omitempty"` // the host's tape title for that world
	Town    string       `json:"town,omitempty"`
	Players []GamePlayer `json:"players"`
}

// KnoxData is what only this player's launcher knows.
type KnoxData struct {
	Invited  map[string]int64 `json:"invited,omitempty"`  // friends you invited -> when
	PackAdds map[string]int64 `json:"packAdds,omitempty"` // friends who added your mod pack -> when
	Cut      int              `json:"cut,omitempty"`      // Director's Cuts taken
	CutXP    int              `json:"cutXP,omitempty"`    // XP spent on them (your level counts from there)
}

const maxGames = 400

type launchNote struct {
	ID      string
	Started int64
	Story   string
	World   string
	Town    string
	Members []Member
}

func (a *App) noteLaunch(launchID string) {
	h := a.cfg.Get().Host
	n := &launchNote{ID: launchID, Started: time.Now().Unix(), Story: h.Backstory, World: h.World, Members: a.lobby.Members()}
	if h.Spawn != nil {
		n.Town = h.Spawn.Name
	}
	a.mu.Lock()
	a.launch = n
	a.mu.Unlock()
}

// recordGame turns the host's session recap into a game record, once the game closes.
func (a *App) recordGame() {
	a.mu.Lock()
	n := a.launch
	a.launch = nil
	a.mu.Unlock()
	if n == nil || a.steam == nil {
		return
	}
	b, err := os.ReadFile(a.paths.bridge("app_recap.json"))
	if err != nil || len(b) == 0 {
		return
	}
	var recap struct {
		Started int64 `json:"started"`
		Updated int64 `json:"updated"`
		Day     int   `json:"day"`
		Players []struct {
			Name    string             `json:"name"`
			SteamID string             `json:"steamID"`
			Kills   int                `json:"kills"`
			Deaths  int                `json:"deaths"`
			Hours   float64            `json:"hours"`
			Goals   []GoalDone         `json:"goals"`
			Track   map[string]float64 `json:"track"`
		} `json:"players"`
	}
	if json.Unmarshal(b, &recap) != nil || len(recap.Players) == 0 || recap.Started < n.Started-120 {
		return // nothing from this game
	}
	g := GameRecord{ID: n.ID, Host: strconv.FormatUint(a.steam.MyID(), 10), Time: time.Now().Unix(), Day: recap.Day,
		Story: n.Story, World: n.World, Town: n.Town, Title: a.cfg.Get().Tapes[tapeKey("world", "", n.World)].Title}
	if recap.Updated > recap.Started {
		g.Mins = int((recap.Updated - recap.Started) / 60)
	}
	byName := map[string]string{}
	for _, m := range n.Members {
		byName[strings.ToLower(strings.TrimSpace(m.Name))] = strconv.FormatUint(m.ID, 10)
	}
	if len(n.Members) == 0 { // hosted without a lobby: the only player is you
		byName[strings.ToLower(a.steam.MyName())] = g.Host
	}
	for _, p := range recap.Players {
		id := p.SteamID
		if id == "" || id == "0" {
			id = byName[strings.ToLower(strings.TrimSpace(p.Name))]
		}
		g.Players = append(g.Players, GamePlayer{ID: id, Name: p.Name, Kills: p.Kills, Deaths: p.Deaths, Hours: p.Hours, Goals: p.Goals, Track: p.Track})
	}
	a.mergeGames([]GameRecord{g})
	a.publishHistory()
	a.publishMyData()
}

// mergeGames adds records we don't have yet (a newer copy of the same game replaces the old one).
func (a *App) mergeGames(in []GameRecord) bool {
	changed := false
	_ = a.cfg.Update(func(c *Config) {
		idx := map[string]int{}
		for i, g := range c.Games {
			idx[g.ID] = i
		}
		for _, g := range in {
			if g.ID == "" || len(g.Players) == 0 {
				continue
			}
			if i, ok := idx[g.ID]; ok {
				if c.Games[i].Time < g.Time {
					c.Games[i] = g
					changed = true
				}
				continue
			}
			idx[g.ID] = len(c.Games)
			c.Games = append(c.Games, g)
			changed = true
		}
		sort.Slice(c.Games, func(i, j int) bool { return c.Games[i].Time < c.Games[j].Time })
		if len(c.Games) > maxGames {
			c.Games = c.Games[len(c.Games)-maxGames:]
		}
	})
	return changed
}

// publishHistory shares the recent games of everyone in the lobby (host only).
func (a *App) publishHistory() {
	id := a.lobby.ID()
	if id == 0 || !a.lobby.IsHost() {
		return
	}
	in := map[string]bool{}
	for _, m := range a.lobby.Members() {
		in[strconv.FormatUint(m.ID, 10)] = true
	}
	games := a.cfg.Get().Games
	var out []GameRecord
	for i := len(games) - 1; i >= 0 && len(out) < 60; i-- {
		for _, p := range games[i].Players {
			if in[p.ID] {
				out = append(out, games[i])
				break
			}
		}
	}
	b, _ := json.Marshal(out)
	setLobbyBig(a.steam, id, "hist", string(b))
}

// readHistory picks up the host's shared games (guests).
func (a *App) readHistory() {
	id := a.lobby.ID()
	if id == 0 || a.lobby.IsHost() {
		return
	}
	sum := a.steam.LobbyData(id, "hist.sum")
	a.mu.Lock()
	same := sum == "" || sum == a.histSum
	a.mu.Unlock()
	if same {
		return
	}
	raw := lobbyRaw(a.steam, id, "hist")
	if bigSum(raw) != sum {
		return // still arriving
	}
	var games []GameRecord
	if json.Unmarshal([]byte(unpackBig(raw)), &games) != nil {
		return
	}
	a.mu.Lock()
	a.histSum = sum
	a.mu.Unlock()
	if a.mergeGames(games) {
		a.publishMyData()
	}
}

// Player stats ------------------------------------------------------------------------------------

type PlayerStats struct {
	Games     int                `json:"games"`
	Hosted    int                `json:"hosted"`
	Kills     int                `json:"kills"`
	Deaths    int                `json:"deaths"`
	Hours     float64            `json:"hours"`     // best character life per world, added up
	BestKills int                `json:"bestKills"` // most kills in one game
	Longest   float64            `json:"longest"`   // longest character life
	Speedruns int                `json:"speedruns"` // died within an in-game hour
	BestDay   int                `json:"bestDay"`
	Stories   []string           `json:"stories,omitempty"`
	GoalXP    int                `json:"goalXP,omitempty"`
	Goals     []string           `json:"goals,omitempty"` // ids of goals finished (story goals give badges)
	Track     map[string]float64 `json:"track,omitempty"` // Knox Link counts added up (nicknames)
}

func statsFor(games []GameRecord, id string) PlayerStats {
	var s PlayerStats
	best := map[string]float64{} // world -> longest life there
	stories := map[string]bool{}
	for _, g := range games {
		if g.Host == id {
			s.Hosted++
		}
		for _, p := range g.Players {
			if p.ID != id {
				continue
			}
			s.Games++
			s.Kills += p.Kills
			s.Deaths += p.Deaths
			s.BestKills = max(s.BestKills, p.Kills)
			s.Longest = math.Max(s.Longest, p.Hours)
			best[g.World] = math.Max(best[g.World], p.Hours)
			if p.Deaths > 0 && p.Hours < 1 {
				s.Speedruns++
			}
			s.BestDay = max(s.BestDay, g.Day)
			for _, gd := range p.Goals {
				s.GoalXP += gd.XP
				s.Goals = append(s.Goals, gd.ID)
			}
			for k, v := range p.Track {
				if s.Track == nil {
					s.Track = map[string]float64{}
				}
				s.Track[k] += v
			}
			if g.Story != "" {
				stories[g.Story] = true
			}
		}
	}
	for _, h := range best {
		s.Hours += h
	}
	for k := range stories {
		s.Stories = append(s.Stories, k)
	}
	sort.Strings(s.Stories)
	return s
}

// Knox level ---------------------------------------------------------------------------------------

type Badge struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	How  string `json:"how"`
	Soon bool   `json:"soon,omitempty"` // needs the next Knox Link update
}

var badges = []Badge{
	{"firstnight", "First Night", "Survive your first in-game night", false},
	{"weekone", "Week One", "Survive 7 days with one character", false},
	{"sixmonths", "Six Months Later", "Survive 180 days", false},
	{"hordebreaker", "Horde Breaker", "100 kills in one game", false},
	{"neveralone", "Never Alone", "Play 10 games with the same friend", false},
	{"packrat", "Pack Rat", "3 friends added your mod pack", false},
	{"hostmost", "Host with the Most", "Host 10 games", false},
	{"recruiter", "Recruiter", "Bring 3 friends into Knox Launcher", false},
	{"everystory", "Every Story", "Play every story once", false},
	{"speedrun", "Speedrun", "Die in the first in-game hour (shame badge)", false},
	{"fieldmedic", "Field Medic", "Bandage squad mates 25 times", false},
	{"roadtrip", "Road Trip", "Drive 50 km total", false},
}

var titles = []struct {
	Level int
	Name  string
}{{1, "Fresh Meat"}, {3, "Scavenger"}, {5, "Survivor"}, {8, "Looter"}, {11, "Barricader"}, {15, "Road Warrior"}, {20, "Veteran"}, {26, "Knox Legend"}, {35, "Patient Zero"}}

// xpForLevel is the total XP a level needs: 0, 100, 300, 600, 1000, ...
func xpForLevel(l int) int { return 50 * l * (l - 1) }

type KnoxProfile struct {
	XP      int      `json:"xp"`
	Level   int      `json:"lvl"`
	Next    int      `json:"next"` // XP the next level needs
	Base    int      `json:"base"` // XP this level started at
	Title   string   `json:"title"`
	Badges  []string `json:"badges"`
	Sources []XPLine `json:"sources,omitempty"`
	Cut     int      `json:"cut,omitempty"`    // Director's Cuts taken
	Months  []string `json:"months,omitempty"` // months you played (collector's editions)
}

type XPLine struct {
	What string `json:"what"`
	XP   int    `json:"xp"`
}

func (a *App) knoxProfile() KnoxProfile {
	cfg := a.cfg.Get()
	if a.steam == nil {
		return KnoxProfile{Level: 1, Title: titles[0].Name, Next: xpForLevel(2)}
	}
	me := strconv.FormatUint(a.steam.MyID(), 10)
	games := cfg.Games
	s := statsFor(games, me)
	var p KnoxProfile
	add := func(what string, xp int) {
		if xp > 0 {
			p.Sources = append(p.Sources, XPLine{what, xp})
			p.XP += xp
		}
	}
	// K1: 25 per game, 10 per in-game day survived (counted once per world)
	add("Games played", 25*s.Games)
	add("Days survived", 10*int(s.Hours/24))
	// K4: hosting
	add("Games hosted", 25*s.Hosted)
	// K5: friends you invited who played a game you hosted
	recruited := 0
	for fid := range cfg.Knox.Invited {
		for _, g := range games {
			if g.Host != me {
				continue
			}
			found := false
			for _, pl := range g.Players {
				found = found || pl.ID == fid
			}
			if found {
				recruited++
				break
			}
		}
	}
	add("Friends you brought in", 100*recruited)
	// K3: friends who added your mod pack
	add("Friends who added your pack", 50*len(cfg.Knox.PackAdds))
	// K2, K6, K9, K10: goals Knox Link counted in your games
	add("Goals", s.GoalXP)
	// badges
	with := map[string]int{}
	for _, g := range games {
		in := false
		for _, pl := range g.Players {
			in = in || pl.ID == me
		}
		if !in {
			continue
		}
		for _, pl := range g.Players {
			if pl.ID != "" && pl.ID != me {
				with[pl.ID]++
			}
		}
	}
	mostWith := 0
	for _, n := range with {
		mostWith = max(mostWith, n)
	}
	allStories := 0
	if c := a.catalog(); c != nil {
		st, _ := storyList(c)
		allStories = len(st)
	}
	got := map[string]bool{
		"firstnight":   s.Longest >= 24,
		"weekone":      s.Longest >= 24*7,
		"sixmonths":    s.Longest >= 24*180,
		"hordebreaker": s.BestKills >= 100,
		"neveralone":   mostWith >= 10,
		"packrat":      len(cfg.Knox.PackAdds) >= 3,
		"hostmost":     s.Hosted >= 10,
		"recruiter":    recruited >= 3,
		"everystory":   allStories > 0 && len(s.Stories) >= allStories,
		"speedrun":     s.Speedruns > 0,
		"fieldmedic":   s.Track["bandOthers"] >= 25,
		"roadtrip":     s.Track["drive"] >= 50000,
	}
	for _, id := range s.Goals {
		if strings.HasPrefix(id, "story:") {
			got["story-"+strings.TrimPrefix(id, "story:")] = true
		}
	}
	for _, b := range allBadges() {
		if got[b.ID] {
			p.Badges = append(p.Badges, b.ID)
		}
	}
	add("Badges", 100*len(p.Badges))
	// testing: a level number in knox_test_level.txt (app data folder) tops XP up to that level
	if b, err := os.ReadFile(filepath.Join(a.paths.Data, "knox_test_level.txt")); err == nil {
		if l, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && l > 1 {
			add("Testing", xpForLevel(min(l, 99))+cfg.Knox.CutXP-p.XP)
		}
	}
	if cfg.Knox.CutXP > 0 {
		p.Sources = append(p.Sources, XPLine{"Director's Cut", -cfg.Knox.CutXP})
		p.XP = max(0, p.XP-cfg.Knox.CutXP)
	}
	p.Cut = cfg.Knox.Cut
	p.Months = playedMonths(games, me)
	p.Level = 1
	for xpForLevel(p.Level+1) <= p.XP {
		p.Level++
	}
	p.Base, p.Next = xpForLevel(p.Level), xpForLevel(p.Level+1)
	p.Title = titleFor(p.Level)
	return p
}

func titleFor(level int) string {
	t := titles[0].Name
	for _, x := range titles {
		if level >= x.Level {
			t = x.Name
		}
	}
	return t
}

// knoxMemberData is what other players see of you: level, title and badges.
func (a *App) knoxMemberData() string {
	p := a.knoxProfile()
	b, _ := json.Marshal(map[string]any{"xp": p.XP, "lvl": p.Level, "title": p.Title, "badges": p.Badges, "cut": p.Cut, "look": a.lobbyLook()})
	return string(b)
}

// Squad view for the poster: lifetime stats of everyone in the lobby, and the squad's totals.

type SquadStats struct {
	Players map[string]PlayerStats `json:"players"`
	Games   int                    `json:"games"` // games with at least two of you
	Kills   int                    `json:"kills"`
	Deaths  int                    `json:"deaths"`
	Hours   float64                `json:"hours"`
	BestDay int                    `json:"bestDay"`
}

func (a *App) squadStats(members []Member) SquadStats {
	games := a.cfg.Get().Games
	out := SquadStats{Players: map[string]PlayerStats{}}
	in := map[string]bool{}
	for _, m := range members {
		id := strconv.FormatUint(m.ID, 10)
		in[id] = true
		out.Players[id] = statsFor(games, id)
	}
	if len(members) == 0 && a.steam != nil {
		id := strconv.FormatUint(a.steam.MyID(), 10)
		in[id] = true
		out.Players[id] = statsFor(games, id)
	}
	for _, g := range games {
		n := 0
		for _, p := range g.Players {
			if in[p.ID] {
				n++
			}
		}
		if n == 0 || (len(in) > 1 && n < 2) {
			continue
		}
		out.Games++
		out.BestDay = max(out.BestDay, g.Day)
		for _, p := range g.Players {
			if in[p.ID] {
				out.Kills += p.Kills
				out.Deaths += p.Deaths
			}
		}
		out.Hours += float64(g.Mins) / 60
	}
	return out
}

// noteInvite remembers who you invited (K5 counts them once they play a game you host).
func (a *App) noteInvite(friend uint64) {
	_ = a.cfg.Update(func(c *Config) {
		if c.Knox.Invited == nil {
			c.Knox.Invited = map[string]int64{}
		}
		k := strconv.FormatUint(friend, 10)
		if _, ok := c.Knox.Invited[k]; !ok {
			c.Knox.Invited[k] = time.Now().Unix()
		}
	})
}

// notePackAdd: a friend in the lobby said they added your pack ("pk:<your id>" in lobby chat).
func (a *App) notePackAdd(from uint64, owner string) {
	if a.steam == nil || owner != strconv.FormatUint(a.steam.MyID(), 10) || from == a.steam.MyID() {
		return
	}
	_ = a.cfg.Update(func(c *Config) {
		if c.Knox.PackAdds == nil {
			c.Knox.PackAdds = map[string]int64{}
		}
		k := strconv.FormatUint(from, 10)
		if _, ok := c.Knox.PackAdds[k]; !ok {
			c.Knox.PackAdds[k] = time.Now().Unix()
		}
	})
	a.publishMyData()
}
