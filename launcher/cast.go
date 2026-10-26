package main

// The cast of a world: everyone who ever played in it, alive or dead, and lives left. Knox Link keeps it
// in the world's ModData and writes app_cast.json while the server runs; we keep the last copy per world.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type CastDeath struct {
	Day   int     `json:"day"`
	Name  string  `json:"name"`
	Hours float64 `json:"hours"`
	Kills int     `json:"kills"`
}

type CastMember struct {
	User      string      `json:"user"`
	Name      string      `json:"name"` // the survivor's name
	Alive     bool        `json:"alive"`
	Out       bool        `json:"out"` // out of lives
	Deaths    int         `json:"deaths"`
	LivesLeft int         `json:"livesLeft"` // -1 = unlimited
	Kills     int         `json:"kills"`
	Hours     float64     `json:"hours"`
	FirstDay  int         `json:"firstDay"`
	LastDay   int         `json:"lastDay"`
	LastSeen  int64       `json:"lastSeen"`
	Died      []CastDeath `json:"died"`
	Look      *CastLook   `json:"look,omitempty"` // what the survivor wore last (Knox Link sends it about once a minute)
}

// CastLook is the launcher's Character shape plus what the game knows that the creator doesn't:
// the skin texture index and each worn item's texture choice. Skin/Traits/Voice are not filled.
type CastLook struct {
	Character
	SkinIndex   int            `json:"skinIndex"`
	ClothingTex map[string]int `json:"clothingTex,omitempty"` // body location -> texture choice
}

type WorldCast struct {
	World     string       `json:"world"`
	Updated   int64        `json:"updated"`
	Day       int          `json:"day"`
	Lives     int          `json:"lives"`     // 0 = unlimited
	LivesMode string       `json:"livesMode"` // each | squad
	Ended     bool         `json:"ended"`     // everyone is out: THE END
	Cast      []CastMember `json:"cast"`
}

func (a *App) castFile(world string) string {
	return filepath.Join(a.paths.Data, "worlds", clean(world)+".cast.json")
}

func readCast(path string) *WorldCast {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return nil
	}
	var c WorldCast
	if json.Unmarshal(b, &c) != nil || c.World == "" {
		return nil
	}
	if c.Cast == nil {
		c.Cast = []CastMember{}
	}
	return &c
}

// saveCast keeps the server's latest cast for its world (called when the game closes, and when read live).
func (a *App) saveCast() *WorldCast {
	c := readCast(a.paths.bridge("app_cast.json"))
	if c == nil {
		return nil
	}
	if old := readCast(a.castFile(c.World)); old != nil && old.Updated > c.Updated {
		return old
	}
	if b, err := json.Marshal(c); err == nil {
		_ = os.MkdirAll(filepath.Dir(a.castFile(c.World)), 0755)
		_ = os.WriteFile(a.castFile(c.World), b, 0644)
	}
	return c
}

// castFor is the cast of a world as last seen (nil if Knox Link never wrote one).
func (a *App) castFor(world string) *WorldCast {
	if world == "" {
		return nil
	}
	if a.gameState().Running {
		if c := readCast(a.paths.bridge("app_cast.json")); c != nil && c.World == world {
			return c
		}
	}
	return readCast(a.castFile(world))
}

// livesText is the lives rule for lobby.txt: lives (0 = unlimited) and each | squad.
func livesText(h HostSettings) (string, string) {
	mode := "each"
	if h.LivesShared {
		mode = "squad"
	}
	return strconv.Itoa(max(0, h.Lives)), mode
}

// mySurvivor finds you in a world's cast by Steam name (usernames in co-op are Steam names).
func (c *WorldCast) member(name string) *CastMember {
	if c == nil {
		return nil
	}
	for i := range c.Cast {
		if strings.EqualFold(c.Cast[i].User, name) {
			return &c.Cast[i]
		}
	}
	return nil
}
