package main

import (
	"os"
	"path/filepath"
	"regexp"
)

// Paths are the folders the app works with.
type Paths struct {
	Steam    string `json:"steam"`
	Game     string `json:"game"`     // ...\steamapps\common\ProjectZomboid
	Workshop string `json:"workshop"` // ...\steamapps\workshop\content\108600
	Zomboid  string `json:"zomboid"`  // %USERPROFILE%\Zomboid
	Data     string `json:"data"`     // app data (%APPDATA%\KnoxLauncher)
}

var libraryPathRe = regexp.MustCompile(`"path"\s+"([^"]+)"`)

// steamLibraries lists every Steam library folder (the main one first).
func steamLibraries(steam string) []string {
	libs := []string{steam}
	b, err := os.ReadFile(filepath.Join(steam, "steamapps", "libraryfolders.vdf"))
	if err != nil {
		return libs
	}
	for _, m := range libraryPathRe.FindAllStringSubmatch(string(b), -1) {
		p := filepath.Clean(unescapeVDF(m[1]))
		dup := false
		for _, l := range libs {
			if samePath(l, p) {
				dup = true
			}
		}
		if !dup {
			libs = append(libs, p)
		}
	}
	return libs
}

func unescapeVDF(s string) string {
	out := []rune{}
	esc := false
	for _, r := range s {
		if esc {
			out = append(out, r)
			esc = false
		} else if r == '\\' {
			esc = true
		} else {
			out = append(out, r)
		}
	}
	return string(out)
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func detectPaths() Paths {
	p := Paths{Steam: detectSteamPath(), Zomboid: detectZomboidDir(), Data: detectDataDir()}
	if v := os.Getenv("EZ_GAME"); v != "" {
		p.Game = v
	}
	if v := os.Getenv("EZ_WORKSHOP"); v != "" {
		p.Workshop = v
	}
	for _, lib := range steamLibraries(p.Steam) {
		if p.Game == "" && exists(filepath.Join(lib, "steamapps", "appmanifest_108600.acf")) {
			p.Game = filepath.Join(lib, "steamapps", "common", "ProjectZomboid")
			if p.Workshop == "" {
				p.Workshop = filepath.Join(lib, "steamapps", "workshop", "content", "108600")
			}
		}
	}
	if p.Workshop == "" && p.Steam != "" {
		p.Workshop = filepath.Join(p.Steam, "steamapps", "workshop", "content", "108600")
	}
	_ = os.MkdirAll(p.Data, 0755)
	// Knox Link used to be called EasyHost: keep its bridge files.
	if old := filepath.Join(p.Zomboid, "Lua", "EasyHost"); p.Zomboid != "" && exists(old) && !exists(p.bridgeDir()) {
		_ = os.Rename(old, p.bridgeDir())
	}
	return p
}

func (p Paths) luaDir() string            { return filepath.Join(p.Zomboid, "Lua") }
func (p Paths) bridgeDir() string         { return filepath.Join(p.Zomboid, "Lua", "KnoxLink") }
func (p Paths) bridge(name string) string { return filepath.Join(p.bridgeDir(), name) }
func (p Paths) serverDir() string         { return filepath.Join(p.Zomboid, "Server") }
func (p Paths) savesDir() string          { return filepath.Join(p.Zomboid, "Saves", "Multiplayer") }
func (p Paths) steamDLL() string          { return filepath.Join(p.Game, "steam_api64.dll") }
