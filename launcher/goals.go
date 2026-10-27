package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Goals (Knox Link round 2): the host's Knox Link hands out squad, personal, long and story goals,
// counts them on the server and puts what each player finished in app_recap.json. The record of
// every game carries them, so every launcher in the squad works out the same XP and badges.

// GoalDone is one goal a player finished in a game.
type GoalDone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	XP   int    `json:"xp"`
}

// goalTier tells Knox Link how hard the world is, so goals ask the same effort on every preset:
// easy worlds ask for more, hard ones for less and pay more.
func goalTier(preset string) string {
	switch preset {
	case "Outbreak", "Rising", "knox-casual", "knox-builders":
		return "easy"
	case "Extinction", "knox-sprinters":
		return "hard"
	}
	return "normal"
}

// story goals give a badge each (K10)
var storyBadges = []Badge{
	{"story-prisonbreak", "Jailbreak", "Lockdown: get out of the prison alive", false},
	{"story-ehe-evac", "Made the Evac", "Missed the Evac: reach the military base", false},
	{"story-farmhands", "Green Thumb", "Harvest: bring in 20 crops", false},
	{"story-firecrew", "Fire Crew", "Engine 9: put out a fire", false},
}

// WorldGoals is Knox Link's app_goals.json: the goals of the world being played and how far along they are.
type WorldGoals struct {
	Difficulty string `json:"difficulty"`
	Story      string `json:"story,omitempty"`
	Day        int    `json:"day"`
	Goals      []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Text string `json:"text"`
		Kind string `json:"kind"` // squad, personal, long, story
		User string `json:"user,omitempty"`
		XP   int    `json:"xp"`
		Done bool   `json:"done"`
		Have int    `json:"have"`
		Need int    `json:"need"`
	} `json:"goals"`
}

func (a *App) goalsFile(world string) string {
	return filepath.Join(a.paths.Data, "worlds", clean(world)+".goals.json")
}

// goalsFor: the hosted world's goals, live while the server runs (and kept for later), else the last copy.
func (a *App) goalsFor(world string) *WorldGoals {
	if world == "" {
		return nil
	}
	if gs := a.gameState(); gs.Running && gs.Host {
		if b, err := os.ReadFile(a.paths.bridge("app_goals.json")); err == nil && len(b) > 0 {
			var g WorldGoals
			if json.Unmarshal(b, &g) == nil && len(g.Goals) > 0 {
				_ = os.MkdirAll(filepath.Dir(a.goalsFile(world)), 0755)
				_ = os.WriteFile(a.goalsFile(world), b, 0644)
				return &g
			}
		}
	}
	b, err := os.ReadFile(a.goalsFile(world))
	if err != nil {
		return nil
	}
	var g WorldGoals
	if json.Unmarshal(b, &g) != nil {
		return nil
	}
	return &g
}

func allBadges() []Badge {
	out := append([]Badge{}, badges...)
	return append(out, storyBadges...)
}
