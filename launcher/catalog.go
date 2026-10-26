package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
)

// The catalog is everything the app shows about the game (jobs, traits, looks, items, mods, spawns),
// read straight from the game's and the mods' own files, so Project Zomboid never has to be open.
// Sandbox and server option definitions only exist inside the game's code; those come from the last
// export the Knox Link mod wrote (app_gamedata.json), with a fallback read from the server files.

type Catalog struct {
	Version     string                   `json:"version"`
	Time        int64                    `json:"time"`
	Game        string                   `json:"game"`
	Professions []Profession             `json:"professions"`
	Traits      []Trait                  `json:"traits"`
	Looks       Looks                    `json:"looks"`
	Items       []ItemEntry              `json:"items"`
	Mods        []ModInfo                `json:"mods"`
	Spawns      []SpawnRegion            `json:"spawns"`
	Sandbox     json.RawMessage          `json:"sandbox"`
	Server      json.RawMessage          `json:"server"`
	Worlds      []WorldEntry             `json:"worlds"`
	Problems    []string                 `json:"problems"`
	Clothing    map[string]*ClothingLook `json:"clothing"` // item id -> how it is drawn in the preview
	itemByID    map[string]*ItemEntry
	modByID     map[string]*ModInfo
}

type XPBoost struct {
	Perk  string `json:"perk"`
	Level int    `json:"level"`
}

type Profession struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Desc    string    `json:"desc"`
	Cost    int       `json:"cost"`
	Granted []string  `json:"granted"`
	XP      []XPBoost `json:"xp"`
	Mod     string    `json:"mod,omitempty"`
}

type Trait struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Desc      string    `json:"desc"`
	Cost      int       `json:"cost"`
	Free      bool      `json:"free"`
	Exclusive []string  `json:"exclusive"`
	Granted   []string  `json:"granted"`
	XP        []XPBoost `json:"xp"`
	Sleep     bool      `json:"sleep"`
	NoMP      bool      `json:"noMP,omitempty"`
	Mod       string    `json:"mod,omitempty"`
}

type NamedID struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Looks struct {
	HairMale   []HairStyle                                   `json:"hairMale"`
	HairFemale []HairStyle                                   `json:"hairFemale"`
	Beards     []HairStyle                                   `json:"beards"`
	HairColors []RGB                                         `json:"hairColors"`
	SkinColors []RGB                                         `json:"skinColors"`
	Voices     []Voice                                       `json:"voices"`
	Clothing   map[string]map[string]map[string]ClothingSlot `json:"clothing"` // profession|default -> Male|Female -> location
}

type HairStyle struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Model   string            `json:"model,omitempty"`
	Texture string            `json:"texture,omitempty"`
	Alt     map[string]string `json:"alt,omitempty"`    // hat category -> style worn under that hat
	Hidden  bool              `json:"hidden,omitempty"` // only used under hats, not in the picker
}

type Voice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Body string `json:"body"` // 1 female, 2 male
}

type ClothingSlot struct {
	Name  string    `json:"name"`
	Items []NamedID `json:"items"`
}

type ItemEntry struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Cat      string `json:"cat"`
	Icon     string `json:"icon,omitempty"`
	Location string `json:"loc,omitempty"`
	Mod      string `json:"mod,omitempty"`
	clothing string
}

type SpawnRegion struct {
	Name   string       `json:"name"`
	Map    string       `json:"map"`
	Points []SpawnPoint `json:"points"`
}

type SpawnPoint struct {
	X    int    `json:"x"`
	Y    int    `json:"y"`
	Z    int    `json:"z"`
	Kind string `json:"kind"` // poor_houses, police_station, ...
}

// ClothingLook is what the 3D preview needs to draw one piece of clothing.
type ClothingLook struct {
	MaleModel    string   `json:"maleModel,omitempty"`
	FemaleModel  string   `json:"femaleModel,omitempty"`
	Textures     []string `json:"textures,omitempty"`     // for models
	BaseTextures []string `json:"baseTextures,omitempty"` // painted onto the body
	Tint         bool     `json:"tint"`
	Hue          bool     `json:"hue"`
	Masks        []int    `json:"masks,omitempty"`
	AttachBone   string   `json:"bone,omitempty"` // static models (hats, glasses) hang off this bone
	MasksFolder  string   `json:"masksFolder,omitempty"`
	HatCategory  string   `json:"hatCategory,omitempty"` // swaps the hair for a style that fits under it
}

// --------------------------------------------------------------------------------------------

type CatalogStore struct {
	app      *App
	mu       sync.Mutex
	cat      *Catalog
	json     []byte
	sig      string
	gameSig  string
	modCache map[string]*modContent // content root -> parsed content
	lang     map[string]string      // vanilla translations
	vanilla  *modContent
	export   *GameData
	exportAt time.Time
	rebuilds int
}

type modContent struct {
	stamp       string
	professions []scriptBlock
	traits      []scriptBlock
	items       []scriptBlock
	lang        map[string]string
	clothingXML map[string]string // ClothingItem name -> xml path
}

type scriptBlock struct {
	module string
	kind   string
	name   string
	props  map[string]string
}

func newCatalogStore(a *App) *CatalogStore {
	return &CatalogStore{app: a, modCache: map[string]*modContent{}}
}

func (s *CatalogStore) Get() (*Catalog, []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cat, s.json
}

// Refresh rebuilds the catalog when anything it depends on changed. Cheap when nothing did.
func (s *CatalogStore) Refresh(force bool) bool {
	sig := s.signature()
	s.mu.Lock()
	same := sig == s.sig && s.cat != nil
	s.mu.Unlock()
	if same && !force {
		return false
	}
	start := time.Now()
	cat := s.build()
	built := cat.Time
	cat.Time = 0 // the version only changes when the content does
	b, _ := json.Marshal(cat)
	h := sha1.Sum(b)
	cat.Version = hex.EncodeToString(h[:8])
	cat.Time = built
	b, _ = json.Marshal(cat)
	s.mu.Lock()
	changed := s.cat == nil || s.cat.Version != cat.Version
	s.cat, s.json, s.sig = cat, b, sig
	s.rebuilds++
	s.mu.Unlock()
	logf("catalog rebuilt in %v (%d mods, %d traits, %d items), changed=%v", time.Since(start).Round(time.Millisecond), len(cat.Mods), len(cat.Traits), len(cat.Items), changed)
	return changed
}

// signature is a quick fingerprint of the folders mods live in, plus the Steam subscriptions.
func (s *CatalogStore) signature() string {
	p := s.app.paths
	var b strings.Builder
	for _, dir := range modParents(p) {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if info, err := e.Info(); err == nil {
				fmt.Fprintf(&b, "%s|%d;", e.Name(), info.ModTime().UnixNano())
			}
		}
		b.WriteString("#")
	}
	if st := s.app.steam; st != nil && s.app.steamOK() {
		subs := st.SubscribedItems()
		sort.Slice(subs, func(i, j int) bool { return subs[i] < subs[j] })
		fmt.Fprint(&b, subs)
	}
	if info, err := os.Stat(p.bridge("app_gamedata.json")); err == nil {
		fmt.Fprint(&b, info.ModTime().UnixNano())
	}
	if info, err := os.Stat(p.serverDir()); err == nil {
		fmt.Fprint(&b, info.ModTime().UnixNano())
	}
	if info, err := os.Stat(p.savesDir()); err == nil {
		fmt.Fprint(&b, info.ModTime().UnixNano())
	}
	return b.String()
}

// Folders that hold one mod (or one Workshop item) per entry, in the order the game prefers them.
func modParents(p Paths) []string {
	return []string{
		filepath.Join(p.Zomboid, "mods"),
		filepath.Join(p.Zomboid, "Workshop"),
		p.Workshop,
	}
}

func (s *CatalogStore) build() *Catalog {
	p := s.app.paths
	cat := &Catalog{Time: time.Now().Unix(), Clothing: map[string]*ClothingLook{}, itemByID: map[string]*ItemEntry{}, modByID: map[string]*ModInfo{}}
	if g, err := loadGameData(p); err == nil {
		s.export = g
	}
	cat.Game = s.gameVersion()

	// vanilla content
	if s.vanilla == nil || s.lang == nil {
		s.lang = loadTranslations(filepath.Join(p.Game, "media", "lua", "shared", "Translate", "EN"))
		s.vanilla = parseContent(filepath.Join(p.Game, "media"), "")
		if len(s.vanilla.traits) == 0 {
			cat.Problems = append(cat.Problems, "Couldn't read the game's files at "+p.Game)
		}
	}
	lang := map[string]string{}
	for k, v := range s.lang {
		lang[k] = v
	}

	// mods
	cat.Mods = s.scanMods(cat.Game)
	order := map[string]int{}
	for i, id := range readDefaultMods(p) {
		order[id] = i + 1
	}
	for i := range cat.Mods {
		cat.Mods[i].Order = order[cat.Mods[i].ID]
	}
	for i := range cat.Mods {
		cat.modByID[cat.Mods[i].ID] = &cat.Mods[i]
	}
	contents := []*modContent{s.vanilla}
	owners := []string{""}
	for _, m := range cat.Mods {
		if m.Broken != "" {
			continue
		}
		for _, root := range m.roots {
			c := s.modContentFor(root)
			contents = append(contents, c)
			owners = append(owners, m.ID)
			for k, v := range c.lang {
				lang[k] = v
			}
		}
	}
	tr := func(key, fallback string) string {
		if v, ok := lang[key]; ok && v != "" {
			return v
		}
		return fallback
	}

	// traits and professions (later definitions replace earlier ones, like the game)
	traitIdx := map[string]int{}
	profIdx := map[string]int{}
	for ci, c := range contents {
		for _, b := range c.traits {
			t := Trait{ID: scriptID(b.name), Mod: owners[ci]}
			t.Name = tr(b.props["UIName"], t.ID)
			t.Desc = tr(b.props["UIDescription"], "")
			t.Cost, _ = strconv.Atoi(b.props["Cost"])
			t.Free = strings.EqualFold(b.props["IsProfessionTrait"], "true")
			t.NoMP = strings.EqualFold(b.props["DisabledInMultiplayer"], "true")
			t.Exclusive = splitIDs(b.props["MutuallyExclusiveTraits"])
			t.Granted = splitIDs(b.props["GrantedTraits"])
			t.XP = parseXP(b.props["XPBoosts"], tr)
			switch t.ID {
			case "insomniac", "needslesssleep", "needsmoresleep":
				t.Sleep = true
			}
			if i, ok := traitIdx[t.ID]; ok {
				cat.Traits[i] = t
			} else {
				traitIdx[t.ID] = len(cat.Traits)
				cat.Traits = append(cat.Traits, t)
			}
		}
		for _, b := range c.professions {
			pr := Profession{ID: scriptID(b.name), Mod: owners[ci]}
			pr.Name = tr(b.props["UIName"], pr.ID)
			pr.Cost, _ = strconv.Atoi(b.props["Cost"])
			pr.Granted = splitIDs(b.props["GrantedTraits"])
			pr.XP = parseXP(b.props["XPBoosts"], tr)
			if i, ok := profIdx[pr.ID]; ok {
				cat.Professions[i] = pr
			} else {
				profIdx[pr.ID] = len(cat.Professions)
				cat.Professions = append(cat.Professions, pr)
			}
		}
	}
	// exclusions go both ways
	for i := range cat.Traits {
		for _, ex := range cat.Traits[i].Exclusive {
			if j, ok := traitIdx[ex]; ok && !contains(cat.Traits[j].Exclusive, cat.Traits[i].ID) {
				cat.Traits[j].Exclusive = append(cat.Traits[j].Exclusive, cat.Traits[i].ID)
			}
		}
	}
	traitName := func(id string) string {
		if i, ok := traitIdx[id]; ok {
			return cat.Traits[i].Name
		}
		return id
	}
	for i := range cat.Professions {
		var lines []string
		for _, x := range cat.Professions[i].XP {
			lines = append(lines, fmt.Sprintf("%+d %s", x.Level, x.Perk))
		}
		for _, g := range cat.Professions[i].Granted {
			lines = append(lines, traitName(g))
		}
		cat.Professions[i].Desc = strings.Join(lines, "\n")
	}

	// items
	for ci, c := range contents {
		for _, b := range c.items {
			module := b.module
			if module == "" {
				module = "Base"
			}
			id := module + "." + b.name
			if strings.EqualFold(b.props["hidden"], "true") || strings.EqualFold(b.props["OBSOLETE"], "true") {
				continue
			}
			name := tr(id, b.props["DisplayName"])
			if name == "" {
				name = b.name
			}
			it := ItemEntry{ID: id, Name: name, Cat: b.props["DisplayCategory"], Icon: b.props["Icon"], Location: scriptID(b.props["BodyLocation"]), Mod: owners[ci], clothing: b.props["ClothingItem"]}
			if old, ok := cat.itemByID[id]; ok {
				*old = it
				continue
			}
			cat.Items = append(cat.Items, it)
			cat.itemByID[id] = &cat.Items[len(cat.Items)-1]
		}
	}
	// pointers into a slice that grew are stale; rebuild the index
	cat.itemByID = map[string]*ItemEntry{}
	for i := range cat.Items {
		cat.itemByID[cat.Items[i].ID] = &cat.Items[i]
	}
	sort.Slice(cat.Items, func(i, j int) bool { return cat.Items[i].Name < cat.Items[j].Name })
	cat.itemByID = map[string]*ItemEntry{}
	for i := range cat.Items {
		cat.itemByID[cat.Items[i].ID] = &cat.Items[i]
	}

	// clothing looks (xml files from the game and every mod)
	xmlPaths := map[string]string{}
	for _, c := range contents {
		for k, v := range c.clothingXML {
			xmlPaths[strings.ToLower(k)] = v
		}
	}
	for _, it := range cat.Items {
		if it.clothing == "" {
			continue
		}
		if path, ok := xmlPaths[strings.ToLower(it.clothing)]; ok {
			if look := readClothingXML(path); look != nil {
				cat.Clothing[it.ID] = look
			}
		}
	}

	// looks
	cat.Looks = s.readLooks(p, tr, cat)

	// spawns
	cat.Spawns = readSpawnRegions(p, cat.Mods)

	// sandbox + server options
	if s.export != nil {
		var raw struct {
			Sandbox json.RawMessage `json:"sandbox"`
			Server  json.RawMessage `json:"server"`
		}
		_ = json.Unmarshal(s.export.raw, &raw)
		cat.Sandbox, cat.Server = raw.Sandbox, raw.Server
	}
	if len(cat.Sandbox) == 0 || string(cat.Sandbox) == "[]" {
		cat.Sandbox = fallbackSandbox(p, tr)
	}
	if len(cat.Server) == 0 || string(cat.Server) == "[]" {
		cat.Server = fallbackServerOptions(p)
	}
	cat.Sandbox = withModSandbox(cat.Sandbox, cat.Mods, tr)

	cat.Worlds = s.app.worlds()
	if cat.Professions == nil {
		cat.Professions = []Profession{}
	}
	if cat.Traits == nil {
		cat.Traits = []Trait{}
	}
	return cat
}

var consoleVersionRe = regexp.MustCompile(`version=(\d+\.\d+(?:\.\d+)?)`)

func (s *CatalogStore) gameVersion() string {
	if s.export != nil && s.export.Game != "" {
		return s.export.Game
	}
	if b, err := os.ReadFile(filepath.Join(s.app.paths.Zomboid, "console.txt")); err == nil {
		if len(b) > 200000 {
			b = b[:200000]
		}
		if m := consoleVersionRe.FindSubmatch(b); m != nil {
			return string(m[1])
		}
	}
	return "42.21.0"
}

// Mods -----------------------------------------------------------------------------------------

type modDir struct {
	id         string
	folder     string // the mod's own folder (…/mods/<Folder>)
	workshopID string
	local      bool
}

var versionDirRe = regexp.MustCompile(`^\d+(\.\d+)*$`)

func compareVersions(a, b string) int {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// pickVersionDir chooses the newest "42", "42.x" folder that isn't newer than the game, like B42 does.
func pickVersionDir(folder, game string) string {
	entries, _ := os.ReadDir(folder)
	best := ""
	for _, e := range entries {
		if !e.IsDir() || !versionDirRe.MatchString(e.Name()) {
			continue
		}
		if !strings.HasPrefix(e.Name(), "42") {
			continue
		}
		if compareVersions(e.Name(), game) > 0 {
			continue
		}
		if best == "" || compareVersions(e.Name(), best) > 0 {
			best = e.Name()
		}
	}
	return best
}

func readModInfo(path string) map[string][]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	out := map[string][]string{}
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		line = strings.TrimPrefix(line, "\ufeff")
		eq := strings.Index(line, "=")
		if eq <= 0 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(line[:eq]))
		out[k] = append(out[k], strings.TrimSpace(line[eq+1:]))
	}
	return out
}

func modList(vals []string) []string {
	var out []string
	for _, v := range vals {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(part), "\\"))
			if part != "" && !contains(out, part) {
				out = append(out, part)
			}
		}
	}
	return out
}

func (s *CatalogStore) scanMods(game string) []ModInfo {
	p := s.app.paths
	var subs map[uint64]bool
	if s.app.steamOK() {
		if list := s.app.steam.SubscribedItems(); list != nil {
			subs = map[uint64]bool{}
			for _, id := range list {
				subs[id] = true
			}
		}
	}
	removed := map[string]bool{}
	if s.export != nil {
		for _, m := range s.export.Mods {
			if m.Removed && m.WorkshopID != "" {
				removed[m.WorkshopID] = true
			}
		}
	}
	localIDs := s.app.localWorkshopIDs()

	var found []ModInfo
	seen := map[string]bool{}
	add := func(folder, workshopID, source string) {
		m := ModInfo{Dir: folder, WorkshopID: workshopID, Source: source}
		ver := pickVersionDir(folder, game)
		var infoPath string
		var roots []string
		if ver != "" {
			infoPath = filepath.Join(folder, ver, "mod.info")
			roots = append(roots, filepath.Join(folder, ver))
		}
		common := filepath.Join(folder, "common")
		if exists(common) {
			roots = append([]string{common}, roots...)
			if infoPath == "" || !exists(infoPath) {
				if exists(filepath.Join(common, "mod.info")) {
					infoPath = filepath.Join(common, "mod.info")
				}
			}
		}
		if infoPath == "" || !exists(infoPath) {
			if exists(filepath.Join(folder, "mod.info")) {
				infoPath = filepath.Join(folder, "mod.info")
				if ver == "" && !exists(common) {
					m.Broken = "Build 41 only"
					roots = []string{folder}
				}
			}
		}
		info := readModInfo(infoPath)
		if info == nil {
			return
		}
		first := func(k string) string {
			if v := info[k]; len(v) > 0 {
				return v[0]
			}
			return ""
		}
		m.ID = first("id")
		if m.ID == "" || seen[m.ID] {
			return
		}
		seen[m.ID] = true
		m.Name = first("name")
		if m.Name == "" {
			m.Name = m.ID
		}
		m.Description = first("description")
		m.Author = first("author")
		m.Version = first("modversion")
		m.Require = modList(info["require"])
		m.Incompatible = modList(info["incompatible"])
		m.LoadAfter = modList(info["loadmodafter"])
		if jar := first("javajarfile"); jar != "" {
			m.Java = true
			// the jar sits next to mod.info, or in the version folder when mod.info is in common
			dirs := []string{filepath.Dir(infoPath)}
			if len(roots) > 0 {
				dirs = append([]string{roots[len(roots)-1]}, dirs...)
			}
			for _, d := range dirs {
				if p := filepath.Join(d, filepath.FromSlash(jar)); exists(p) {
					m.jar = p
					break
				}
			}
		}
		if poster := first("poster"); poster != "" {
			for _, root := range append([]string{filepath.Dir(infoPath)}, roots...) {
				if path := filepath.Join(root, poster); exists(path) {
					m.Posters = []string{path}
					break
				}
			}
		}
		if vmin := first("versionmin"); vmin != "" && compareVersions(vmin, game) > 0 && m.Broken == "" {
			m.Broken = "Needs game version " + vmin
		}
		if vmax := first("versionmax"); vmax != "" && compareVersions(vmax, game) < 0 && m.Broken == "" && !strings.HasPrefix(game, vmax) {
			m.Broken = "Only works up to game version " + vmax
		}
		if m.WorkshopID == "" {
			m.WorkshopID = localIDs[filepath.Base(folder)]
		}
		if m.WorkshopID != "" && removed[m.WorkshopID] {
			m.Removed = true
		}
		m.roots = roots
		found = append(found, m)
	}
	// local mods: Zomboid/mods/<Folder>
	entries, _ := os.ReadDir(filepath.Join(p.Zomboid, "mods"))
	for _, e := range entries {
		if e.IsDir() {
			add(filepath.Join(p.Zomboid, "mods", e.Name()), "", "local")
		}
	}
	// your own uploads: Zomboid/Workshop/<Item>/Contents/mods/<Folder>
	entries, _ = os.ReadDir(filepath.Join(p.Zomboid, "Workshop"))
	for _, e := range entries {
		base := filepath.Join(p.Zomboid, "Workshop", e.Name(), "Contents", "mods")
		mods, _ := os.ReadDir(base)
		for _, m := range mods {
			if m.IsDir() {
				add(filepath.Join(base, m.Name()), localIDs[m.Name()], "upload")
			}
		}
	}
	// subscribed Workshop items
	entries, _ = os.ReadDir(p.Workshop)
	for _, e := range entries {
		wid, err := strconv.ParseUint(e.Name(), 10, 64)
		if err != nil {
			continue
		}
		if subs != nil && !subs[wid] {
			continue // unsubscribed: Steam deletes the folder later, but it's gone for us now
		}
		base := filepath.Join(p.Workshop, e.Name(), "mods")
		mods, _ := os.ReadDir(base)
		for _, m := range mods {
			if m.IsDir() {
				add(filepath.Join(base, m.Name()), e.Name(), "workshop")
			}
		}
	}
	// subscribed but not downloaded yet
	if subs != nil {
		have := map[string]bool{}
		for _, m := range found {
			have[m.WorkshopID] = true
		}
		for id := range subs {
			ws := strconv.FormatUint(id, 10)
			if !have[ws] && !exists(filepath.Join(p.Workshop, ws)) {
				found = append(found, ModInfo{ID: "workshop:" + ws, Name: "Downloading Workshop item " + ws, WorkshopID: ws, Source: "workshop", Pending: true})
			}
		}
	}
	sort.Slice(found, func(i, j int) bool { return strings.ToLower(found[i].Name) < strings.ToLower(found[j].Name) })
	return found
}

func (s *CatalogStore) modContentFor(root string) *modContent {
	stamp := ""
	if info, err := os.Stat(root); err == nil {
		stamp = info.ModTime().String()
	}
	if info, err := os.Stat(filepath.Join(root, "media")); err == nil {
		stamp += info.ModTime().String()
	}
	if c, ok := s.modCache[root]; ok && c.stamp == stamp {
		return c
	}
	c := parseContent(filepath.Join(root, "media"), stamp)
	s.modCache[root] = c
	return c
}

// parseContent reads one media folder: scripts (traits, jobs, items), clothing xml, translations.
func parseContent(media, stamp string) *modContent {
	c := &modContent{stamp: stamp, clothingXML: map[string]string{}}
	scripts := filepath.Join(media, "scripts")
	_ = filepath.WalkDir(scripts, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(strings.ToLower(path), ".txt") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, blk := range parseScript(string(b)) {
			switch blk.kind {
			case "character_trait_definition":
				c.traits = append(c.traits, blk)
			case "character_profession_definition":
				c.professions = append(c.professions, blk)
			case "item":
				c.items = append(c.items, blk)
			}
		}
		return nil
	})
	items, _ := os.ReadDir(filepath.Join(media, "clothing", "clothingItems"))
	for _, e := range items {
		if strings.HasSuffix(strings.ToLower(e.Name()), ".xml") {
			c.clothingXML[strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))] = filepath.Join(media, "clothing", "clothingItems", e.Name())
		}
	}
	c.lang = loadTranslations(filepath.Join(media, "lua", "shared", "Translate", "EN"))
	return c
}

// Scripts ----------------------------------------------------------------------------------------

var commentRe = regexp.MustCompile(`(?s)/\*.*?\*/`)

// parseScript reads "module X { kind name { key = value, ... } }" files. Nested blocks inside an
// entry (components, fluids...) are skipped.
func parseScript(src string) []scriptBlock {
	src = commentRe.ReplaceAllString(src, "")
	var out []scriptBlock
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	module := ""
	depth := 0
	var cur *scriptBlock
	curDepth := 0
	pendingHeader := ""
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if i := strings.Index(line, "//"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		opens := strings.Count(line, "{")
		closes := strings.Count(line, "}")
		header := strings.TrimSpace(strings.TrimSuffix(line, "{"))
		if opens > 0 && header == "" && pendingHeader != "" {
			header = pendingHeader
		}
		if opens > 0 {
			fields := strings.Fields(header)
			if depth == 0 && len(fields) >= 2 && fields[0] == "module" {
				module = fields[1]
			} else if cur == nil && depth == 1 && len(fields) >= 2 {
				cur = &scriptBlock{module: module, kind: fields[0], name: fields[1], props: map[string]string{}}
				curDepth = depth + 1
			}
			depth += opens
			depth -= closes
			pendingHeader = ""
			continue
		}
		if cur != nil && depth == curDepth {
			if eq := strings.Index(line, "="); eq > 0 {
				k := strings.TrimSpace(line[:eq])
				v := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(line[eq+1:]), ","))
				cur.props[k] = v
			}
		}
		if closes > 0 {
			depth -= closes
			if cur != nil && depth < curDepth {
				out = append(out, *cur)
				cur = nil
			}
			if depth < 0 {
				depth = 0
			}
			pendingHeader = ""
			continue
		}
		if !strings.Contains(line, "=") {
			pendingHeader = line
		}
	}
	return out
}

// "base:athletic" -> "athletic"; mod namespaces are kept ("mymod:trait").
func scriptID(v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(strings.ToLower(v), "base:") {
		return v[5:]
	}
	return v
}

func splitIDs(v string) []string {
	out := []string{}
	for _, part := range strings.Split(v, ";") {
		if part = scriptID(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func parseXP(v string, tr func(string, string) string) []XPBoost {
	out := []XPBoost{}
	for _, part := range strings.Split(v, ";") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		perk := strings.TrimSpace(kv[0])
		lv, _ := strconv.Atoi(strings.TrimSpace(kv[1]))
		out = append(out, XPBoost{Perk: tr("IGUI_perks_"+perk, perk), Level: lv})
	}
	return out
}

// Translations -----------------------------------------------------------------------------------

var txtPairRe = regexp.MustCompile(`([A-Za-z0-9_.\-]+)\s*=\s*"((?:[^"\\]|\\.)*)"`)

func loadTranslations(dir string) map[string]string {
	out := map[string]string{}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		b = []byte(strings.TrimPrefix(string(b), "\ufeff"))
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".json":
			var m map[string]any
			if json.Unmarshal(b, &m) == nil {
				for k, v := range m {
					if s, ok := v.(string); ok {
						out[k] = s
					}
				}
			}
		case ".txt":
			for _, m := range txtPairRe.FindAllStringSubmatch(string(b), -1) {
				out[m[1]] = strings.ReplaceAll(m[2], `\"`, `"`)
			}
		}
	}
	return out
}

// Looks ------------------------------------------------------------------------------------------

type hairXML struct {
	Male   []hairXMLEntry `xml:"male"`
	Female []hairXMLEntry `xml:"female"`
	Style  []hairXMLEntry `xml:"style"`
}

type hairXMLEntry struct {
	Name     string `xml:"name"`
	Model    string `xml:"model"`
	Texture  string `xml:"texture"`
	NoChoose bool   `xml:"noChoose"`
	Alt      []struct {
		Category string `xml:"category,attr"`
		Style    string `xml:"style,attr"`
	} `xml:"alternate"`
}

type voiceXML struct {
	Styles []struct {
		Name string `xml:"name"`
		Body string `xml:"bodyTypeDefault"`
	} `xml:"style"`
}

// the game's common hair colours (CharacterCreationMain uses SurvivorDesc.getCommonHairColor)
var defaultHairColors = []RGB{
	{0.831, 0.671, 0.271}, {0.663, 0.522, 0.318}, {0.624, 0.424, 0.169}, {0.412, 0.271, 0.122},
	{0.235, 0.157, 0.071}, {0.169, 0.110, 0.059}, {0.094, 0.071, 0.047}, {0.722, 0.278, 0.165},
	{0.557, 0.200, 0.110}, {0.910, 0.835, 0.651}, {0.682, 0.682, 0.682}, {0.420, 0.420, 0.420},
}

var defaultSkinColors = []RGB{{1, 0.91, 0.72}, {0.98, 0.79, 0.49}, {0.8, 0.65, 0.45}, {0.54, 0.38, 0.25}, {0.36, 0.25, 0.14}}

func (s *CatalogStore) readLooks(p Paths, tr func(string, string) string, cat *Catalog) Looks {
	media := filepath.Join(p.Game, "media")
	var lk Looks
	var hx hairXML
	if b, err := os.ReadFile(filepath.Join(media, "hairStyles", "hairStyles.xml")); err == nil {
		_ = xml.Unmarshal(b, &hx)
	}
	conv := func(list []hairXMLEntry, prefix string) []HairStyle {
		out := []HairStyle{}
		for _, h := range list {
			id := h.Name
			name := tr(prefix+id, id)
			if id == "" || strings.EqualFold(id, "bald") {
				name = tr("IGUI_Hair_Bald", "Bald")
			}
			st := HairStyle{ID: id, Name: name, Model: h.Model, Texture: h.Texture, Hidden: h.NoChoose}
			for _, a := range h.Alt {
				if st.Alt == nil {
					st.Alt = map[string]string{}
				}
				st.Alt[a.Category] = a.Style
			}
			out = append(out, st)
		}
		return out
	}
	lk.HairMale = conv(hx.Male, "IGUI_Hair_")
	lk.HairFemale = conv(hx.Female, "IGUI_Hair_")
	var bx hairXML
	if b, err := os.ReadFile(filepath.Join(media, "hairStyles", "beardStyles.xml")); err == nil {
		_ = xml.Unmarshal(b, &bx)
	}
	lk.Beards = []HairStyle{{ID: "", Name: tr("IGUI_Beard_None", "None")}}
	for _, h := range conv(bx.Style, "IGUI_Beard_") {
		if h.ID != "" && !strings.EqualFold(h.ID, "none") && !h.Hidden {
			lk.Beards = append(lk.Beards, h)
		}
	}
	var vx voiceXML
	if b, err := os.ReadFile(filepath.Join(media, "voiceStyles", "voiceStyles.xml")); err == nil {
		_ = xml.Unmarshal(b, &vx)
	}
	for _, v := range vx.Styles {
		lk.Voices = append(lk.Voices, Voice{ID: v.Name, Name: tr("IGUI_Voice_"+v.Name, v.Name), Body: v.Body})
	}
	lk.SkinColors = defaultSkinColors
	lk.HairColors = defaultHairColors
	if s.export != nil {
		var raw struct {
			Looks struct {
				HairColors []RGB `json:"hairColors"`
			} `json:"looks"`
		}
		if json.Unmarshal(s.export.raw, &raw) == nil && len(raw.Looks.HairColors) > 0 {
			lk.HairColors = raw.Looks.HairColors
		}
	}
	lk.Clothing = readClothingSelection(filepath.Join(media, "lua", "shared", "Definitions", "ClothingSelectionDefinitions.lua"), cat, tr)
	return lk
}

// readClothingSelection runs ClothingSelectionDefinitions.lua in a small Lua VM and reads the table.
func readClothingSelection(path string, cat *Catalog, tr func(string, string) string) map[string]map[string]map[string]ClothingSlot {
	out := map[string]map[string]map[string]ClothingSlot{}
	L := lua.NewState(lua.Options{SkipOpenLibs: false})
	defer L.Close()
	if err := L.DoFile(path); err != nil {
		logf("clothing definitions: %v", err)
		return out
	}
	defs, ok := L.GetGlobal("ClothingSelectionDefinitions").(*lua.LTable)
	if !ok {
		return out
	}
	defs.ForEach(func(k, v lua.LValue) {
		job, ok := v.(*lua.LTable)
		if !ok {
			return
		}
		byGender := map[string]map[string]ClothingSlot{}
		for _, gender := range []string{"Male", "Female"} {
			gt, ok := job.RawGetString(gender).(*lua.LTable)
			if !ok {
				continue
			}
			slots := map[string]ClothingSlot{}
			gt.ForEach(func(lk, lv lua.LValue) {
				lt, ok := lv.(*lua.LTable)
				if !ok {
					return
				}
				loc := lk.String()
				slot := ClothingSlot{Name: tr("UI_ClothingType_"+loc, loc), Items: []NamedID{}}
				if items, ok := lt.RawGetString("items").(*lua.LTable); ok {
					items.ForEach(func(_, iv lua.LValue) {
						id := iv.String()
						name := id
						if it := cat.itemByID[id]; it != nil {
							name = it.Name
						}
						slot.Items = append(slot.Items, NamedID{ID: id, Name: name})
					})
				}
				slots[loc] = slot
			})
			byGender[gender] = slots
		}
		// "if there's no difference between male/female outfit, just create the female one"
		if byGender["Male"] == nil {
			byGender["Male"] = byGender["Female"]
		}
		if byGender["Female"] == nil {
			byGender["Female"] = byGender["Male"]
		}
		out[k.String()] = byGender
	})
	return out
}

type clothingItemXML struct {
	MaleModel    string   `xml:"m_MaleModel"`
	FemaleModel  string   `xml:"m_FemaleModel"`
	BaseTextures []string `xml:"m_BaseTextures"`
	Textures     []string `xml:"textureChoices"`
	Tint         bool     `xml:"m_AllowRandomTint"`
	Hue          bool     `xml:"m_AllowRandomHue"`
	Masks        []int    `xml:"m_Masks"`
	AttachBone   string   `xml:"m_AttachBone"`
	MasksFolder  string   `xml:"m_MasksFolder"`
	HatCategory  string   `xml:"m_HatCategory"`
}

func readClothingXML(path string) *ClothingLook {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	b = []byte(strings.TrimPrefix(string(b), "\ufeff"))
	var x clothingItemXML
	if xml.Unmarshal(b, &x) != nil {
		return nil
	}
	norm := func(v string) string {
		v = strings.TrimSpace(strings.ReplaceAll(v, "\\", "/"))
		if strings.EqualFold(v, "null") {
			return ""
		}
		return v
	}
	look := &ClothingLook{MaleModel: norm(x.MaleModel), FemaleModel: norm(x.FemaleModel), Tint: x.Tint, Hue: x.Hue, Masks: x.Masks, AttachBone: norm(x.AttachBone), MasksFolder: norm(x.MasksFolder), HatCategory: norm(x.HatCategory)}
	for _, t := range x.Textures {
		if t = norm(t); t != "" {
			look.Textures = append(look.Textures, t)
		}
	}
	for _, t := range x.BaseTextures {
		if t = norm(t); t != "" {
			look.BaseTextures = append(look.BaseTextures, t)
		}
	}
	return look
}

// Spawns -----------------------------------------------------------------------------------------

var (
	tableHeaderRe = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*=\s*\{\s*\{`)
	pointRe       = regexp.MustCompile(`\{([^{}]*pos[XY][^{}]*)\}`)
	numFieldRe    = regexp.MustCompile(`(worldX|worldY|posX|posY|posZ)\s*=\s*(-?\d+)`)
)

// parseSpawnPoints reads a spawnpoints.lua. Points are named after the table they sit in
// (poor_houses, police_station, or a profession for maps that list points per job).
func parseSpawnPoints(src string) []SpawnPoint {
	var out []SpawnPoint
	seen := map[[3]int]bool{}
	headers := tableHeaderRe.FindAllStringSubmatchIndex(src, -1)
	for _, pm := range pointRe.FindAllStringSubmatchIndex(src, -1) {
		kind := "houses"
		for _, h := range headers {
			if h[0] < pm[0] {
				kind = src[h[2]:h[3]]
			}
		}
		f := map[string]int{}
		for _, nm := range numFieldRe.FindAllStringSubmatch(src[pm[2]:pm[3]], -1) {
			f[nm[1]], _ = strconv.Atoi(nm[2])
		}
		x, y := f["posX"], f["posY"]
		if _, ok := f["worldX"]; ok {
			x, y = f["worldX"]*300+f["posX"], f["worldY"]*300+f["posY"]
		}
		key := [3]int{x, y, f["posZ"]}
		if x == 0 && y == 0 || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, SpawnPoint{X: x, Y: y, Z: f["posZ"], Kind: kind})
	}
	return out
}

var vanillaRegions = []string{"Muldraugh, KY", "Riverside, KY", "Rosewood, KY", "West Point, KY", "Echo Creek, KY", "Brandenburg, KY", "Ekron, KY", "Fallas Lake, KY", "Irvington, KY", "March Ridge, KY", "Valley Station, KY"}

func readSpawnRegions(p Paths, mods []ModInfo) []SpawnRegion {
	var out []SpawnRegion
	for _, name := range vanillaRegions {
		b, err := os.ReadFile(filepath.Join(p.Game, "media", "maps", name, "spawnpoints.lua"))
		if err != nil {
			continue
		}
		out = append(out, SpawnRegion{Name: name, Map: name, Points: parseSpawnPoints(string(b))})
	}
	// map mods with their own spawn points
	for _, m := range mods {
		if m.Broken != "" {
			continue
		}
		for _, root := range m.roots {
			maps, _ := os.ReadDir(filepath.Join(root, "media", "maps"))
			for _, d := range maps {
				b, err := os.ReadFile(filepath.Join(root, "media", "maps", d.Name(), "spawnpoints.lua"))
				if err != nil {
					continue
				}
				out = append(out, SpawnRegion{Name: d.Name(), Map: d.Name(), Points: parseSpawnPoints(string(b))})
			}
		}
	}
	return out
}

// Fallback option lists --------------------------------------------------------------------------

// fallbackSandbox builds sandbox pages from a SandboxVars.lua file when the game never exported them.
func fallbackSandbox(p Paths, tr func(string, string) string) json.RawMessage {
	entries, _ := os.ReadDir(p.serverDir())
	var vars map[string]string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_SandboxVars.lua") {
			vars = readSandboxFile(filepath.Join(p.serverDir(), e.Name()))
			if len(vars) > 0 {
				break
			}
		}
	}
	type opt struct {
		Name    string   `json:"name"`
		Label   string   `json:"label"`
		Tooltip string   `json:"tooltip"`
		Type    string   `json:"type"`
		OptType string   `json:"optType"`
		Default string   `json:"default"`
		Values  []string `json:"values"`
	}
	pages := map[string][]opt{}
	for k, v := range vars {
		if k == "VERSION" {
			continue
		}
		page := "General"
		if i := strings.Index(k, "."); i > 0 {
			page = k[:i]
		}
		short := k[strings.LastIndex(k, ".")+1:]
		t := "string"
		if v == "true" || v == "false" {
			t = "boolean"
		} else if _, err := strconv.Atoi(v); err == nil {
			t = "integer"
		} else if _, err := strconv.ParseFloat(v, 64); err == nil {
			t = "double"
		}
		pages[page] = append(pages[page], opt{Name: k, Label: tr("Sandbox_"+short, short), Type: t, OptType: t, Default: v, Values: []string{}})
	}
	type pg struct {
		Name     string `json:"name"`
		Settings []opt  `json:"settings"`
	}
	var out []pg
	for name, list := range pages {
		sort.Slice(list, func(i, j int) bool { return list[i].Label < list[j].Label })
		out = append(out, pg{Name: name, Settings: list})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	b, _ := json.Marshal(out)
	return b
}

var iniCommentRe = regexp.MustCompile(`^#\s*(.*)$`)

// fallbackServerOptions reads the option list (and the game's own comments) from a server .ini.
func fallbackServerOptions(p Paths) json.RawMessage {
	type opt struct {
		Name    string   `json:"name"`
		Label   string   `json:"label"`
		Type    string   `json:"type"`
		Default string   `json:"default"`
		Tooltip string   `json:"tooltip"`
		Values  []string `json:"values"`
	}
	var out []opt
	entries, _ := os.ReadDir(p.serverDir())
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".ini") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(p.serverDir(), e.Name()))
		if err != nil {
			continue
		}
		var comment []string
		for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
			if m := iniCommentRe.FindStringSubmatch(line); m != nil {
				comment = append(comment, m[1])
				continue
			}
			eq := strings.Index(line, "=")
			if eq <= 0 {
				comment = nil
				continue
			}
			k, v := line[:eq], line[eq+1:]
			t := "string"
			if v == "true" || v == "false" {
				t = "boolean"
			} else if _, err := strconv.Atoi(v); err == nil {
				t = "integer"
			} else if _, err := strconv.ParseFloat(v, 64); err == nil {
				t = "double"
			}
			out = append(out, opt{Name: k, Label: k, Type: t, Default: v, Tooltip: strings.Join(comment, "\n"), Values: []string{}})
			comment = nil
		}
		if len(out) > 0 {
			break
		}
	}
	b, _ := json.Marshal(out)
	return b
}

// withModSandbox adds a page for every mod that has its own sandbox options (media/sandbox-options.txt).
func withModSandbox(base json.RawMessage, mods []ModInfo, tr func(string, string) string) json.RawMessage {
	var pages []map[string]any
	if json.Unmarshal(base, &pages) != nil {
		return base
	}
	have := map[string]bool{}
	for _, pg := range pages {
		if set, ok := pg["settings"].([]any); ok {
			for _, s := range set {
				if m, ok := s.(map[string]any); ok {
					if n, ok := m["name"].(string); ok {
						have[n] = true
					}
				}
			}
		}
	}
	for _, m := range mods {
		if m.Broken != "" {
			continue
		}
		var settings []map[string]any
		for _, root := range m.roots {
			b, err := os.ReadFile(filepath.Join(root, "media", "sandbox-options.txt"))
			if err != nil {
				continue
			}
			for _, blk := range parseScript(string(b)) {
				if blk.kind != "option" || have[blk.name] {
					continue
				}
				have[blk.name] = true
				pr := blk.props
				typ := strings.ToLower(pr["type"])
				entry := map[string]any{"name": blk.name, "label": tr(pr["translation"], blk.name), "tooltip": tr(pr["translation"]+"_tooltip", ""),
					"type": typ, "optType": typ, "default": pr["default"], "values": []string{}, "mod": m.ID}
				if typ == "enum" {
					n, _ := strconv.Atoi(pr["numValues"])
					var vals []string
					for i := 1; i <= n; i++ {
						vals = append(vals, tr(pr["valueTranslation"]+"_option"+strconv.Itoa(i), strconv.Itoa(i)))
					}
					entry["values"] = vals
				}
				if v, err := strconv.ParseFloat(pr["min"], 64); err == nil {
					entry["min"] = v
				}
				if v, err := strconv.ParseFloat(pr["max"], 64); err == nil {
					entry["max"] = v
				}
				if page := pr["page"]; page != "" {
					entry["title"] = tr("Sandbox_"+page, page)
				}
				settings = append(settings, entry)
			}
		}
		if len(settings) > 0 {
			pages = append(pages, map[string]any{"name": m.Name, "mod": m.ID, "settings": settings})
		}
	}
	b, _ := json.Marshal(pages)
	return b
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
