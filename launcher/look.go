package main

// Tape looks: how your VHS covers and your lobby poster look. Some come with Knox levels.
// Looks only, never the game.

import (
	"errors"
	"strconv"
	"time"
)

// Look is what you picked. Empty fields mean the default. "mix" means each tape gets one of the
// picks you have unlocked (the launcher picks per tape).
type Look struct {
	Genre     string `json:"genre,omitempty"`     // cover layout
	Backdrop  string `json:"backdrop,omitempty"`  // what's behind the cast
	Pose      string `json:"pose,omitempty"`      // your pose on posters ("" = a random one)
	Frame     string `json:"frame,omitempty"`     // lobby slot frame: "" = best you have, "none", "bronze", ...
	Worn      bool   `json:"worn,omitempty"`      // scuffed, sun-faded case
	Foil      bool   `json:"foil,omitempty"`      // foil title
	Collector string `json:"collector,omitempty"` // "2026-10" = that month's collector's edition
	WornMix   bool   `json:"wornMix,omitempty"`   // some tapes worn, some not
	FoilMix   bool   `json:"foilMix,omitempty"`   // some tapes foil, some not
	Poster    string `json:"poster,omitempty"`    // lobby character poster style ("" = Spotlight)
	PosterInk string `json:"posterInk,omitempty"` // poster colour ("" = blood red)
	Spine     string `json:"spine,omitempty"`     // spine style on the shelf ("" = a mix of every style you have)
	SpineInk  string `json:"spineInk,omitempty"`  // spine colour ("" = matches the cover genre)

	// free picks (2026-10-26 customization round)
	Tagline   string `json:"tagline,omitempty"`   // your own line ("" = one of the built-in ones)
	TitleFont string `json:"titleFont,omitempty"` // "" = the cover style's own
	TitleInk  string `json:"titleInk,omitempty"`  // a spine colour id ("" = the cover style's own)
	Tint      string `json:"tint,omitempty"`      // "", warm, cold, green, bw
	Rating    string `json:"rating,omitempty"`    // "" (R on the back), G, PG, PG13, R, UNRATED
	Studio    string `json:"studio,omitempty"`    // "" = a seeded one
	Price     string `json:"price,omitempty"`     // price sticker text ("" = none)
	Rewind    bool   `json:"rewind,omitempty"`    // Be Kind Rewind sticker
	SpineText string `json:"spineText,omitempty"` // your own words on the spine label
	LabelInk  string `json:"labelInk,omitempty"`  // rental sticker colour: "", white, cream, yellow, pink, blue
	CaseInk   string `json:"caseInk,omitempty"`   // "", grey, white
	Headline  string `json:"headline,omitempty"`  // poster headline in place of WANTED etc.
	HideNick  bool   `json:"hideNick,omitempty"`  // no nickname under your name
	Plate     string `json:"plate,omitempty"`     // lobby name plate: "", dogtag, nametape, band
	// level picks
	Case     string   `json:"case,omitempty"`     // "", clamshell, rental, slipcover, shrink
	Stickers []string `json:"stickers,omitempty"` // staff, newrelease, banned, holo
	Wear     string   `json:"wear,omitempty"`     // "", sun, water (Worn is the scuffed one)
}

// Prefs: how the shelf and the launcher look. Yours only, never sent to the lobby.
type Prefs struct {
	Shelf  string   `json:"shelf,omitempty"`  // dark, pine, metal, wire, crate, gun
	Light  string   `json:"light,omitempty"`  // warm, cold, candle
	Sort   string   `json:"sort,omitempty"`   // played, name, solo, squad
	Props  []string `json:"props,omitempty"`  // bobblehead, plant, bat, radio
	Accent string   `json:"accent,omitempty"` // red, amber, green, blue
}

// Unlock is one thing you can pick and the level it needs.
type Unlock struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Level int     `json:"lvl"`
	Anim  string  `json:"anim,omitempty"` // poses: the game animation
	At    float64 `json:"at,omitempty"`   // poses: where in the animation to hold
}

var genres = []Unlock{
	{ID: "horror", Name: "Horror", Level: 1},
	{ID: "slasher", Name: "Slasher", Level: 3},
	{ID: "kungfu", Name: "Kung Fu", Level: 4},
	{ID: "action", Name: "80s Action", Level: 6},
	{ID: "creature", Name: "Creature Feature", Level: 9},
	{ID: "western", Name: "Western", Level: 11},
	{ID: "tv", Name: "Made-for-TV", Level: 14},
	{ID: "scifi", Name: "Sci-fi B-movie", Level: 16},
	{ID: "teen", Name: "Teen Comedy", Level: 20},
	{ID: "truecrime", Name: "True Crime", Level: 26},
}

var backdrops = []Unlock{
	{ID: "title", Name: "Title screen", Level: 1},
	{ID: "tower", Name: "Water tower", Level: 2},
	{ID: "city", Name: "Louisville burning", Level: 4},
	{ID: "mall", Name: "The mall", Level: 7},
	{ID: "farm", Name: "Farm at dusk", Level: 10},
	{ID: "gas", Name: "Gas station", Level: 13},
	{ID: "docks", Name: "Riverside docks", Level: 5},
	{ID: "hospital", Name: "Hospital", Level: 9},
	{ID: "prison", Name: "Prison", Level: 12},
	{ID: "fire", Name: "Fire station", Level: 15},
	{ID: "woods", Name: "The woods", Level: 17},
	{ID: "blockade", Name: "Army blockade", Level: 20},
	{ID: "heli", Name: "Helicopter crash", Level: 25},
}

// Base poses are free. The rest come with levels.
var poses = []Unlock{
	{ID: "idle", Name: "Standing", Level: 1, Anim: "Bob_Idle", At: 0.1},
	{ID: "run", Name: "Running", Level: 1, Anim: "Bob_Run", At: 0.3},
	{ID: "spooked", Name: "Spooked", Level: 1, Anim: "Bob_EmoteSpooked", At: 0.45},
	{ID: "handsup", Name: "Hands up", Level: 1, Anim: "Bob_EmoteSurrender", At: 0.5},
	{ID: "comehere", Name: "Come here", Level: 1, Anim: "Bob_EmoteComeHere", At: 0.35},
	{ID: "sneak", Name: "Sneaking", Level: 1, Anim: "Bob_WalkLowSneak", At: 0.25},
	{ID: "salute", Name: "Salute", Level: 1, Anim: "Bob_EmoteSalute_Formal", At: 0.5},
	{ID: "thumbs", Name: "Thumbs up", Level: 1, Anim: "Bob_EmoteThumbsUp", At: 0.5},
	{ID: "shrug", Name: "Shrug", Level: 1, Anim: "Bob_EmoteShrug", At: 0.5},
	{ID: "rude", Name: "Rude", Level: 1, Anim: "Bob_EmoteInsult", At: 0.55},
	{ID: "wave", Name: "Wave", Level: 1, Anim: "Bob_EmoteWaveHi", At: 0.4},
	{ID: "clap", Name: "Clap", Level: 1, Anim: "Bob_EmoteClap", At: 0.4},
	{ID: "followme", Name: "Follow me", Level: 1, Anim: "Bob_EmoteFollowMe", At: 0.45},
	{ID: "stop", Name: "Stop", Level: 1, Anim: "Bob_EmoteStop", At: 0.5},
	{ID: "shout", Name: "Shout", Level: 1, Anim: "Bob_EmoteShout", At: 0.45},
	{ID: "thumbsdown", Name: "Thumbs down", Level: 1, Anim: "Bob_EmoteThumbsDown", At: 0.5},
	{ID: "casual", Name: "Casual salute", Level: 1, Anim: "Bob_EmoteSalute_Casual", At: 0.5},
	{ID: "undecided", Name: "Undecided", Level: 1, Anim: "Bob_EmoteUndecided", At: 0.5},
	{ID: "stretch", Name: "Stretch", Level: 1, Anim: "Bob_EmoteTiredStretch", At: 0.45},
	{ID: "sit", Name: "Sitting", Level: 1, Anim: "Bob_SitGround_Idle", At: 0.2},
	{ID: "bent", Name: "Out of breath", Level: 3, Anim: "Bob_EmoteBentDouble", At: 0.5},
	{ID: "gun", Name: "Finger gun", Level: 5, Anim: "Bob_IdleAimHandgun", At: 0.2},
	{ID: "faint", Name: "Feeling faint", Level: 7, Anim: "Bob_EmoteFeelFeint", At: 0.5},
	{ID: "facepalm", Name: "Facepalm", Level: 9, Anim: "Bob_EmoteHoldBridgeNose", At: 0.5},
	{ID: "batter", Name: "Batter up", Level: 11, Anim: "Bob_IdleAimBat", At: 0.2},
	{ID: "gag", Name: "Bad smell", Level: 13, Anim: "Bob_EmoteSmellGag", At: 0.5},
	{ID: "vomit", Name: "Throwing up", Level: 16, Anim: "Bob_EmoteVomit", At: 0.5},
	{ID: "shiver", Name: "Freezing", Level: 19, Anim: "Bob_EmoteShiver", At: 0.5},
	{ID: "headache", Name: "Headache", Level: 22, Anim: "Bob_EmotePainHead", At: 0.5},
	{ID: "rifle", Name: "Aiming", Level: 25, Anim: "Bob_IdleAimRifle", At: 0.2},
	{ID: "passout", Name: "Passed out", Level: 30, Anim: "Bob_EmotePassOut_Back", At: 0.95},
}

var spines = []Unlock{
	{ID: "classic", Name: "Classic", Level: 1},
	{ID: "block", Name: "Studio block", Level: 1},
	{ID: "tape", Name: "Masking tape", Level: 3},
	{ID: "art", Name: "Art strip", Level: 4},
	{ID: "stripes", Name: "80s stripes", Level: 6},
	{ID: "rental", Name: "Rental label", Level: 10},
	{ID: "barcode", Name: "Library barcode", Level: 13},
	{ID: "neon", Name: "Neon", Level: 15},
	{ID: "stencil", Name: "Army stencil", Level: 18},
	{ID: "tiedye", Name: "Tie-dye", Level: 21},
	{ID: "chrome", Name: "Chrome", Level: 24},
	{ID: "glitch", Name: "Glitch", Level: 27},
}

// Lobby character posters: their own look, not the tape's.
var posters = []Unlock{
	{ID: "spotlight", Name: "Spotlight", Level: 1},
	{ID: "wanted", Name: "Wanted", Level: 1},
	{ID: "missing", Name: "Missing", Level: 4},
	{ID: "onesheet", Name: "Theater one-sheet", Level: 5},
	{ID: "propaganda", Name: "Knox needs you", Level: 7},
	{ID: "card", Name: "Trading card", Level: 9},
	{ID: "mugshot", Name: "Mugshot", Level: 11},
	{ID: "obit", Name: "Obituary", Level: 13},
	{ID: "comic", Name: "Comic cover", Level: 15},
	{ID: "newspaper", Name: "Front page", Level: 17},
	{ID: "fightcard", Name: "Fight night", Level: 19},
	{ID: "polaroid", Name: "Polaroid", Level: 23},
	{ID: "graffiti", Name: "Graffiti", Level: 26},
}

var spineInks = []Unlock{
	{ID: "red", Name: "Blood red", Level: 1},
	{ID: "white", Name: "Bone white", Level: 2},
	{ID: "yellow", Name: "Caution yellow", Level: 3},
	{ID: "blue", Name: "Police blue", Level: 5},
	{ID: "green", Name: "Toxic green", Level: 8},
	{ID: "pink", Name: "Hot pink", Level: 12},
	{ID: "orange", Name: "Hunter orange", Level: 16},
	{ID: "purple", Name: "Bruise purple", Level: 21},
	{ID: "gold", Name: "Gold", Level: 28},
}

var frames = []Unlock{
	{ID: "wood", Name: "Wood", Level: 3},
	{ID: "bronze", Name: "Bronze", Level: 5},
	{ID: "silver", Name: "Silver", Level: 12},
	{ID: "barbed", Name: "Barbed wire", Level: 15},
	{ID: "gold", Name: "Gold", Level: 20},
	{ID: "neon", Name: "Neon", Level: 25},
}

var cases = []Unlock{
	{ID: "clamshell", Name: "Clamshell", Level: 6},
	{ID: "rental", Name: "Rental case", Level: 9},
	{ID: "slipcover", Name: "Slipcover", Level: 12},
	{ID: "shrink", Name: "Shrink wrap", Level: 18},
}

var stickers = []Unlock{
	{ID: "staff", Name: "Staff Pick", Level: 7},
	{ID: "newrelease", Name: "New Release", Level: 10},
	{ID: "banned", Name: "Banned in Kentucky", Level: 22},
	{ID: "holo", Name: "Hologram seal", Level: 29},
}

var wears = []Unlock{
	{ID: "sun", Name: "Sun bleached", Level: 14},
	{ID: "water", Name: "Water damaged", Level: 19},
}

var shelves = []Unlock{
	{ID: "dark", Name: "Dark wood", Level: 1},
	{ID: "pine", Name: "Pine", Level: 1},
	{ID: "metal", Name: "Metal rack", Level: 1},
	{ID: "wire", Name: "Video store rack", Level: 10},
	{ID: "crate", Name: "Wooden crates", Level: 18},
	{ID: "gun", Name: "Gun cabinet", Level: 28},
}

var props = []Unlock{
	{ID: "bobblehead", Name: "Bobblehead", Level: 2},
	{ID: "plant", Name: "Pot plant", Level: 6},
	{ID: "bat", Name: "Baseball bat", Level: 14},
	{ID: "radio", Name: "Ham radio", Level: 24},
}

// free choices: anyone can pick these
var freeChoices = map[string][]string{
	"titleFont": {"block", "typewriter", "marker", "stencil", "serif"},
	"tint":      {"warm", "cold", "green", "bw"},
	"rating":    {"G", "PG", "PG13", "R", "UNRATED"},
	"studio":    {"knox", "muldraugh", "westpoint", "riverside"},
	"labelInk":  {"white", "cream", "yellow", "pink", "blue"},
	"caseInk":   {"grey", "white"},
	"plate":     {"dogtag", "nametape", "band"},
	"light":     {"warm", "cold", "candle"},
	"sort":      {"played", "name", "solo", "squad"},
	"accent":    {"red", "amber", "green", "blue"},
}

func freePick(kind, v string) error {
	if v == "" || v == "mix" {
		return nil
	}
	for _, x := range freeChoices[kind] {
		if x == v {
			return nil
		}
	}
	return errors.New("unknown pick: " + v)
}

const (
	wornLevel      = 8
	foilLevel      = 20
	collectorLevel = 5
	cutLevel       = 35 // Director's Cut: reset to level 1 and keep everything
)

// lookLevel is the level that counts for unlocks. After a Director's Cut everything stays unlocked.
func lookLevel(p KnoxProfile) int {
	if p.Cut > 0 {
		return max(p.Level, cutLevel)
	}
	return p.Level
}

func findUnlock(list []Unlock, id string) *Unlock {
	for i := range list {
		if list[i].ID == id {
			return &list[i]
		}
	}
	return nil
}

// checkLook refuses anything you haven't unlocked yet.
func checkLook(l Look, p KnoxProfile) error {
	lvl := lookLevel(p)
	need := func(list []Unlock, id string) error {
		if id == "" || id == "none" || id == "mix" {
			return nil
		}
		u := findUnlock(list, id)
		if u == nil {
			return errors.New("unknown pick: " + id)
		}
		if lvl < u.Level {
			return errors.New(u.Name + " unlocks at level " + strconv.Itoa(u.Level))
		}
		return nil
	}
	anyInk := func(id string) error { // colours are free on titles and posters
		if id == "" || id == "mix" || findUnlock(spineInks, id) != nil {
			return nil
		}
		return errors.New("unknown colour: " + id)
	}
	checks := []error{need(genres, l.Genre), need(backdrops, l.Backdrop), need(poses, l.Pose), need(frames, l.Frame), need(spines, l.Spine), need(spineInks, l.SpineInk),
		need(posters, l.Poster), anyInk(l.PosterInk), anyInk(l.TitleInk), need(cases, l.Case), need(wears, l.Wear),
		freePick("titleFont", l.TitleFont), freePick("tint", l.Tint), freePick("rating", l.Rating), freePick("studio", l.Studio),
		freePick("labelInk", l.LabelInk), freePick("caseInk", l.CaseInk), freePick("plate", l.Plate)}
	for _, st := range l.Stickers {
		checks = append(checks, need(stickers, st))
	}
	for _, e := range checks {
		if e != nil {
			return e
		}
	}
	for _, t := range []struct {
		v   string
		max int
	}{{l.Tagline, 60}, {l.Price, 8}, {l.SpineText, 14}, {l.Headline, 16}} {
		if len([]rune(t.v)) > t.max {
			return errors.New("that text is too long (" + strconv.Itoa(t.max) + " letters max)")
		}
	}
	if l.Worn && lvl < wornLevel {
		return errors.New("worn cases unlock at level " + strconv.Itoa(wornLevel))
	}
	if l.Foil && lvl < foilLevel {
		return errors.New("foil prints unlock at level " + strconv.Itoa(foilLevel))
	}
	if l.Collector == "mix" {
		if lvl < collectorLevel {
			return errors.New("collector's editions unlock at level " + strconv.Itoa(collectorLevel))
		}
	} else if l.Collector != "" {
		if lvl < collectorLevel {
			return errors.New("collector's editions unlock at level " + strconv.Itoa(collectorLevel))
		}
		ok := false
		for _, m := range p.Months {
			ok = ok || m == l.Collector
		}
		if !ok {
			return errors.New("you didn't play that month")
		}
	}
	return nil
}

// playedMonths lists the months ("2026-10") you played a game in, newest first.
func playedMonths(games []GameRecord, me string) []string {
	seen := map[string]bool{}
	var out []string
	for i := len(games) - 1; i >= 0; i-- {
		g := games[i]
		in := false
		for _, pl := range g.Players {
			in = in || pl.ID == me
		}
		if !in {
			continue
		}
		m := time.Unix(g.Time, 0).Format("2006-01")
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

func (a *App) setLook(l Look) error {
	if err := checkLook(l, a.knoxProfile()); err != nil {
		return err
	}
	err := a.cfg.Update(func(c *Config) { c.Look = l })
	a.publishMyData()
	return err
}

// directorsCut resets your level to 1. You keep every unlock and get a Director's Cut label.
func (a *App) directorsCut() error {
	p := a.knoxProfile()
	if p.Level < cutLevel {
		return errors.New("Director's Cut needs level " + strconv.Itoa(cutLevel))
	}
	err := a.cfg.Update(func(c *Config) {
		c.Knox.Cut++
		c.Knox.CutXP += p.XP
	})
	a.publishMyData()
	return err
}

func lookDefs() map[string]any {
	return map[string]any{"genres": genres, "backdrops": backdrops, "poses": poses, "frames": frames, "spines": spines, "spineInks": spineInks, "posters": posters,
		"cases": cases, "stickers": stickers, "wears": wears, "shelves": shelves, "props": props, "free": freeChoices,
		"worn": wornLevel, "foil": foilLevel, "collector": collectorLevel, "cut": cutLevel}
}

// checkPrefs: shelf types and props unlock with your level; the rest is free.
func checkPrefs(p Prefs, k KnoxProfile) error {
	lvl := lookLevel(k)
	need := func(list []Unlock, id string) error {
		if id == "" {
			return nil
		}
		u := findUnlock(list, id)
		if u == nil {
			return errors.New("unknown pick: " + id)
		}
		if lvl < u.Level {
			return errors.New(u.Name + " unlocks at level " + strconv.Itoa(u.Level))
		}
		return nil
	}
	checks := []error{need(shelves, p.Shelf), freePick("light", p.Light), freePick("sort", p.Sort), freePick("accent", p.Accent)}
	for _, id := range p.Props {
		checks = append(checks, need(props, id))
	}
	for _, e := range checks {
		if e != nil {
			return e
		}
	}
	return nil
}
