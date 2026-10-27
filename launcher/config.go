package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type RGB struct {
	R float64 `json:"r"`
	G float64 `json:"g"`
	B float64 `json:"b"`
}

// Character is what the app's creator builds; it is handed to the game as a preset.
type Character struct {
	Forename     string             `json:"forename"`
	Surname      string             `json:"surname"`
	Female       bool               `json:"female"`
	Profession   string             `json:"profession"`
	Traits       []string           `json:"traits"` // chosen traits (not the free ones)
	Skin         RGB                `json:"skin"`
	Hair         string             `json:"hair"`
	HairColor    RGB                `json:"hairColor"`
	Beard        string             `json:"beard"`
	Stubble      bool               `json:"stubble"`
	BeardStubble bool               `json:"beardStubble"`
	ChestHair    bool               `json:"chestHair"`
	Voice        string             `json:"voice"`
	Pitch        float64            `json:"pitch"`
	Clothing     map[string]string  `json:"clothing"` // body location -> item type
	ClothingTint map[string]*RGB    `json:"clothingTint"`
	Extras       map[string]float64 `json:"extras,omitempty"` // creator mod options, by CreatorExtra key
}

type KitItem struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type SpawnSpot struct {
	X      int      `json:"x"`
	Y      int      `json:"y"`
	Z      int      `json:"z"`
	Name   string   `json:"name"`             // nearest town, "" out in the country
	Kind   string   `json:"kind,omitempty"`   // building type
	Points [][2]int `json:"points,omitempty"` // building outline, for the map
}

type HostSettings struct {
	World        string            `json:"world"`
	SquadName    string            `json:"squadName,omitempty"` // "" = made from the story and town
	Memory       int               `json:"memory"`
	Lobby        bool              `json:"lobby"`     // wait in game until everyone's in, then a countdown
	Together     bool              `json:"together"`  // everyone starts in the same building
	Spawn        *SpawnSpot        `json:"spawn"`     // the building everyone starts in
	SpawnMode    string            `json:"spawnMode"` // "pick" (keep this one) | "random" (new one each start)
	SpawnTown    string            `json:"spawnTown"` // "" = anywhere
	SpawnType    string            `json:"spawnType"` // building category
	Town         string            `json:"town"`      // old setting, unused
	Ini          map[string]string `json:"ini"`
	Sandbox      map[string]string `json:"sandbox"`
	Difficulty   string            `json:"difficulty"` // preset the sandbox started from
	Mods         []string          `json:"mods"`       // mods the player turned on (nil = take the game's list)
	Pack         string            `json:"pack"`
	Kit          []KitItem         `json:"kit"`
	KitName      string            `json:"kitName"`
	Respawn      string            `json:"respawn"`               // base (start building) | squad (next to a living friend) | random (a random house)
	Backstory    string            `json:"backstory"`             // squad backstory id
	Lives        int               `json:"lives,omitempty"`       // characters per player in a world, 0 = unlimited
	LivesShared  bool              `json:"livesShared,omitempty"` // Lives is one pool of spare lives for the squad
	DirectLaunch *bool             `json:"directLaunch,omitempty"`
}

type Config struct {
	Character    Character                 `json:"character"`
	Presets      map[string]Character      `json:"presets"`
	Packs        map[string][]string       `json:"packs"`                     // mod pack -> mod ids
	Loadouts     map[string][]KitItem      `json:"loadouts"`                  // your own loadout presets
	Settings     map[string]SettingsPreset `json:"settingsPresets,omitempty"` // your own settings presets
	ModFolders   map[string][]string       `json:"modFolders,omitempty"`      // folder name -> mod ids (Mods page grouping only)
	Host         HostSettings              `json:"host"`
	KeepBackups  int                       `json:"keepBackups"`
	Tab          string                    `json:"tab"`
	SeenGuide    bool                      `json:"seenGuide"`
	Role         string                    `json:"role"`                   // backstory role you took
	Stories      []Backstory               `json:"stories,omitempty"`      // your own stories (editor + community)
	Uploads      []Upload                  `json:"uploads,omitempty"`      // what you put on the community browser
	Liked        map[string]bool           `json:"liked,omitempty"`        // community items you liked
	CommunityURL string                    `json:"communityURL,omitempty"` // community server (empty = the built-in one)
	HideExamples bool                      `json:"hideExamples,omitempty"` // hide the made-up community uploads
	Games        []GameRecord              `json:"games,omitempty"`        // games you played with Knox Launcher (for stats and Knox level)
	Knox         KnoxData                  `json:"knox"`
	Look         Look                      `json:"look"`            // your tape and poster looks
	Tapes        map[string]TapeMeta       `json:"tapes,omitempty"` // each tape's own title and cover (tapes.go)
	Prefs        Prefs                     `json:"prefs"`           // shelf and launcher looks (look.go)
}

// SettingsPreset is a snapshot of the host's world (sandbox) and server (ini) settings.
type SettingsPreset struct {
	Sandbox map[string]string `json:"sandbox"`
	Ini     map[string]string `json:"ini"`
	Saved   int64             `json:"saved"`
}

type ConfigStore struct {
	mu   sync.Mutex
	path string
	cfg  Config
	rev  int // bumps on every change, so the UI and the lobby know to refresh
}

func (s *ConfigStore) Rev() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rev
}

func loadConfig(dir string) *ConfigStore {
	s := &ConfigStore{path: filepath.Join(dir, "config.json")}
	s.cfg = Config{
		Presets: map[string]Character{}, Packs: map[string][]string{}, KeepBackups: 5,
		Host: HostSettings{World: "Knox", Memory: 4096, Lobby: true, Together: true, Ini: map[string]string{}, Sandbox: map[string]string{}},
	}
	if b, err := os.ReadFile(s.path); err == nil {
		b = bytes.ReplaceAll(b, []byte(`"`+oldKnoxLinkID+`"`), []byte(`"`+knoxLinkID+`"`))
		_ = json.Unmarshal(b, &s.cfg)
	}
	if s.cfg.Presets == nil {
		s.cfg.Presets = map[string]Character{}
	}
	if s.cfg.Packs == nil {
		s.cfg.Packs = map[string][]string{}
	}
	if s.cfg.Loadouts == nil {
		s.cfg.Loadouts = map[string][]KitItem{}
	}
	if s.cfg.Host.SpawnMode == "" {
		s.cfg.Host.SpawnMode = "random"
	}
	if s.cfg.Host.SpawnType == "" {
		s.cfg.Host.SpawnType = "house"
	}
	if s.cfg.Host.Respawn == "" {
		s.cfg.Host.Respawn = "base"
	}
	if s.cfg.Host.Ini == nil {
		s.cfg.Host.Ini = map[string]string{}
	}
	if s.cfg.Host.Sandbox == nil {
		s.cfg.Host.Sandbox = map[string]string{}
	}
	if s.cfg.KeepBackups <= 0 {
		s.cfg.KeepBackups = 5
	}
	return s
}

func (s *ConfigStore) Get() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(s.cfg)
	var c Config
	_ = json.Unmarshal(b, &c)
	return c
}

func (s *ConfigStore) Update(f func(*Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(&s.cfg)
	s.rev++
	b, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Game preset files -----------------------------------------------------------

func f3(v float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", v), "0"), ".")
}
func rgbText(c RGB) string { return f3(c.R) + "," + f3(c.G) + "," + f3(c.B) }

func clean(s string) string {
	return strings.NewReplacer(";", "", ":", "", "|", "", "=", "", "\n", "", "\r", "").Replace(s)
}

// outfitString matches CharacterCreationMain:saveBuildStep2.
func (c Character) outfitString() string {
	var b strings.Builder
	gender := 2
	if c.Female {
		gender = 1
	}
	yes := func(v bool) string {
		if v {
			return "1"
		}
		return "2"
	}
	fmt.Fprintf(&b, "gender=%d;", gender)
	fmt.Fprintf(&b, "skincolor=%s;", rgbText(c.Skin))
	fmt.Fprintf(&b, "name=%s|%s;", clean(c.Forename), clean(c.Surname))
	fmt.Fprintf(&b, "hair=%s|%s;", clean(c.Hair), rgbText(c.HairColor))
	fmt.Fprintf(&b, "stubble=%s;", yes(c.Stubble))
	if !c.Female {
		fmt.Fprintf(&b, "chesthair=%s;", yes(c.ChestHair))
		fmt.Fprintf(&b, "beard=%s;", clean(c.Beard))
		fmt.Fprintf(&b, "beardstubble=%s;", yes(c.BeardStubble))
	}
	if c.Voice != "" {
		fmt.Fprintf(&b, "voiceStyle=%s;", clean(c.Voice))
	}
	fmt.Fprintf(&b, "voicePitch=%s;", f3(c.Pitch))
	locs := make([]string, 0, len(c.Clothing))
	for loc := range c.Clothing {
		locs = append(locs, loc)
	}
	sort.Strings(locs)
	for _, loc := range locs {
		item := c.Clothing[loc]
		if item == "" {
			continue
		}
		fmt.Fprintf(&b, "%s=%s", clean(loc), clean(item))
		if t := c.ClothingTint[loc]; t != nil {
			fmt.Fprintf(&b, "|%s", rgbText(*t))
		}
		b.WriteString(";")
	}
	return b.String()
}

// buildString matches CharacterCreationProfession:saveBuildStep2.
func (c Character) buildString() string {
	var b strings.Builder
	b.WriteString(clean(c.Profession) + ";")
	for _, t := range c.Traits {
		b.WriteString(clean(t) + ";")
	}
	return b.String()
}

// writePresetFile replaces the given entries in a "name:value" preset file, keeping the others.
func writePresetFile(path string, header string, entries map[string]string) error {
	var lines []string
	if b, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
			if line == "" || (header != "" && strings.HasPrefix(line, "VERSION=")) {
				continue
			}
			name := line
			if i := strings.Index(line, ":"); i >= 0 {
				name = line[:i]
			}
			if _, replaced := entries[name]; replaced {
				continue
			}
			lines = append(lines, line)
		}
	}
	names := make([]string, 0, len(entries))
	for n := range entries {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		lines = append(lines, n+":"+entries[n])
	}
	out := strings.Join(lines, "\n") + "\n"
	if header != "" {
		out = header + "\n" + out
	}
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(out), 0644)
}

const gamePresetName = "Knox Launcher"

// writeGamePresets puts the current character (as "Knox Launcher") and every app preset into the
// game's own preset lists, so they also show up in the normal character creator.
func writeGamePresets(p Paths, cfg Config) error {
	builds := map[string]string{gamePresetName: cfg.Character.buildString()}
	outfits := map[string]string{gamePresetName: cfg.Character.outfitString()}
	for name, c := range cfg.Presets {
		n := clean(name)
		if n == "" || n == gamePresetName {
			continue
		}
		builds[n] = c.buildString()
		outfits[n] = c.outfitString()
	}
	if err := writePresetFile(filepath.Join(p.luaDir(), "saved_builds.txt"), "", builds); err != nil {
		return err
	}
	_ = os.MkdirAll(p.bridgeDir(), 0755)
	_ = os.WriteFile(p.bridge("app_extras.txt"), []byte(extrasText(cfg.Character)), 0644)
	return writePresetFile(filepath.Join(p.luaDir(), "saved_outfits.txt"), "VERSION=1", outfits)
}
