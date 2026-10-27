package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const appVersion = "2.7"

type App struct {
	paths  Paths
	cfg    *ConfigStore
	steam  Steam
	lobby  *Lobby
	cat    *CatalogStore
	assets *AssetIndex
	wm     *WorldMap
	server *coopServer

	mu       sync.Mutex
	steamErr string
	steamUp  bool
	game     GameState
	loopOnce sync.Once
	wake     chan struct{}
	quit     func()

	published string              // what the lobby last got, to skip identical updates
	mfChat    map[string][]string // launch id -> manifest parts sent in lobby chat
	mfLast    string              // the launch id of the newest parts
	launch    *launchNote         // the game being hosted, for its record when it closes
	histSum   string              // checksum of the host's shared games we last read
}

// GameState is the Project Zomboid process the launcher started (or found running).
type GameState struct {
	Running  bool   `json:"running"`
	Ours     bool   `json:"ours"` // started by this launcher
	Host     bool   `json:"host"`
	Started  int64  `json:"started"`
	Exited   int64  `json:"exited"`
	Error    string `json:"error,omitempty"`
	Sessions int    `json:"sessions"`
}

func newApp(paths Paths) *App {
	a := &App{paths: paths, cfg: loadConfig(paths.Data), wake: make(chan struct{}, 1), server: &coopServer{}}
	a.steam = newSteam(paths.steamDLL())
	a.lobby = &Lobby{app: a, phase: PhaseIdle}
	a.cat = newCatalogStore(a)
	a.assets = newAssetIndex()
	a.wm = newWorldMap(paths.Game)
	a.connectSteam()
	a.cat.Refresh(true)
	if a.cfg.Get().Host.Mods == nil {
		mods := readDefaultMods(paths)
		if mods == nil {
			mods = []string{}
		}
		_ = a.cfg.Update(func(c *Config) { c.Host.Mods = mods })
	}
	a.startLoop()
	go a.watch()
	return a
}

func (a *App) startLoop() {
	a.loopOnce.Do(func() { go a.lobby.run() })
}

// watch keeps the catalog in step with the disk and Steam, and the lobby in step with the config.
func (a *App) watch() {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	n := 0
	last := time.Now()
	for {
		select {
		case <-tick.C:
		case <-a.wake:
			time.Sleep(300 * time.Millisecond) // let Steam finish writing the folder
		}
		n++
		// a long gap here means this whole PC stopped for a while (on 10-27 the launcher, the server
		// and the game all froze for 30 s at once and everyone was dropped): log it with the memory
		if gap := time.Since(last); gap > 5*time.Second {
			log.Printf("launcher was stalled for %v; %s", gap.Round(time.Second), memoryLine())
		}
		last = time.Now()
		if n%30 == 0 && (a.server.get().Running || gameRunning()) {
			log.Println(memoryLine())
		}
		a.cat.Refresh(false)
		if a.lobby.ID() != 0 {
			a.publishHostInfo()
			a.publishMyData()
		}
		if n%3 == 0 {
			a.checkGame()
		}
	}
}

func (a *App) workshopChanged() {
	select {
	case a.wake <- struct{}{}:
	default:
	}
}

func (a *App) connectSteam() {
	err := a.steam.Init()
	a.mu.Lock()
	defer a.mu.Unlock()
	if err != nil {
		a.steamErr, a.steamUp = err.Error(), false
		log.Println("steam:", err)
		return
	}
	a.steamErr, a.steamUp = "", true
}

func (a *App) steamOK() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.steamUp
}

func (a *App) steamReady() error {
	if !a.steamOK() {
		a.connectSteam()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.steamUp {
		return errors.New(a.steamErr)
	}
	return nil
}

func (a *App) catalog() *Catalog {
	c, _ := a.cat.Get()
	return c
}

func (a *App) gameState() GameState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.game
}

// Mods ------------------------------------------------------------------------------------------

// chosenMods is the wish list: the selected pack, or the mods switched on in the launcher.
func (a *App) chosenMods() []string {
	cfg := a.cfg.Get()
	if pack, ok := cfg.Packs[cfg.Host.Pack]; ok && cfg.Host.Pack != "" {
		return pack
	}
	return cfg.Host.Mods
}

// hostModList is the final load order the host will run.
func (a *App) hostModList() ([]string, map[string]string) {
	c := a.catalog()
	if c == nil {
		return nil, nil
	}
	return resolveMods(c, a.chosenMods())
}

type ModChange struct {
	Added   []string `json:"added"`   // requirements switched on too
	Removed []string `json:"removed"` // mods that needed this one, switched off too
	Missing []string `json:"missing"` // requirements that aren't installed
	Mods    []string `json:"mods"`
}

// SetMod switches one mod on or off, keeping requirements consistent.
func (a *App) SetMod(id string, on bool) (ModChange, error) {
	c := a.catalog()
	var ch ModChange
	if c == nil {
		return ch, errors.New("still reading the game files")
	}
	if id == knoxLinkID && !on {
		return ch, errors.New("Knox Link has to stay on")
	}
	cur := map[string]bool{}
	for _, m := range a.chosenMods() {
		cur[m] = true
	}
	if on {
		if m := c.mod(id); m == nil {
			return ch, errors.New("that mod isn't installed")
		} else if m.Broken != "" {
			return ch, errors.New(m.Name + ": " + m.Broken)
		} else if m.Pending {
			return ch, errors.New(m.Name + " is still downloading")
		}
		have, missing := c.requirementsOf(id)
		if len(missing) > 0 {
			ch.Missing = missing
			return ch, fmt.Errorf("%s needs %s. Subscribe to it first.", c.mod(id).Name, strings.Join(missing, ", "))
		}
		cur[id] = true
		for _, r := range have {
			if !cur[r] {
				cur[r] = true
				ch.Added = append(ch.Added, r)
			}
		}
	} else {
		delete(cur, id)
		for _, d := range c.dependentsOf(id, cur) {
			delete(cur, d)
			ch.Removed = append(ch.Removed, d)
		}
	}
	list := make([]string, 0, len(cur))
	for m := range cur {
		list = append(list, m)
	}
	sort.Strings(list)
	ch.Mods = list
	err := a.cfg.Update(func(cfg *Config) {
		if cfg.Host.Pack != "" {
			if _, ok := cfg.Packs[cfg.Host.Pack]; ok {
				cfg.Packs[cfg.Host.Pack] = list
				return
			}
		}
		cfg.Host.Mods = list
	})
	return ch, err
}

// SetMods replaces the whole list (all on, all off, a pack), adding requirements.
func (a *App) SetMods(want []string) error {
	c := a.catalog()
	if c == nil {
		return errors.New("still reading the game files")
	}
	ids, _ := resolveMods(c, want)
	return a.cfg.Update(func(cfg *Config) {
		if cfg.Host.Pack != "" {
			if _, ok := cfg.Packs[cfg.Host.Pack]; ok {
				cfg.Packs[cfg.Host.Pack] = ids
				return
			}
		}
		cfg.Host.Mods = ids
	})
}

// Lobby data -------------------------------------------------------------------------------------

type ModBrief struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	WorkshopID string `json:"ws,omitempty"`
}

type Override struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HostInfo is everything friends see about the game being set up. It updates live.
type HostInfo struct {
	World       string            `json:"world"`
	NewWorld    bool              `json:"newWorld"`
	Mods        []ModBrief        `json:"mods"`
	Spawn       *SpawnSpot        `json:"spawn"`
	SpawnMode   string            `json:"spawnMode"`
	SpawnTown   string            `json:"spawnTown"`
	SpawnType   string            `json:"spawnType"`
	Together    bool              `json:"together"`
	Kit         []KitItem         `json:"kit"`
	KitName     string            `json:"kitName"`
	PvP         bool              `json:"pvp"`
	Sleep       bool              `json:"sleep"`
	FreePts     int               `json:"freePoints"`
	Difficulty  string            `json:"difficulty"`
	Sandbox     []Override        `json:"sandbox"` // changed from the preset
	Ini         []Override        `json:"ini"`
	Respawn     string            `json:"respawn"`
	Backstory   string            `json:"backstory"`
	Story       *Backstory        `json:"story,omitempty"` // the host's own story, which guests don't have
	Lobby       bool              `json:"lobby"`
	Roles       map[string]string `json:"roles"` // role -> member id
	SquadName   string            `json:"squadName,omitempty"`
	Lives       int               `json:"lives,omitempty"`
	LivesShared bool              `json:"livesShared,omitempty"`
	Cast        *WorldCast        `json:"cast,omitempty"`  // who's in this world already (continuing a tape)
	Goals       *WorldGoals       `json:"goals,omitempty"` // Knox goals of this world (goals.go)
}

func (a *App) hostInfo() HostInfo {
	cfg := a.cfg.Get()
	h := cfg.Host
	info := HostInfo{World: h.World, Kit: h.Kit, KitName: h.KitName, Spawn: h.Spawn, SpawnMode: h.SpawnMode, SpawnTown: h.SpawnTown,
		SpawnType: h.SpawnType, Together: h.Together, Difficulty: h.Difficulty, Respawn: h.Respawn, Backstory: h.Backstory, Lobby: h.Lobby}
	info.NewWorld = !a.worldExists(h.World)
	info.Lives, info.LivesShared = h.Lives, h.LivesShared
	if !info.NewWorld {
		info.Cast = a.castFor(h.World)
		info.Goals = a.goalsFor(h.World)
	}
	info.SquadName = h.SquadName
	info.Story = a.userStory(h.Backstory)
	c := a.catalog()
	ini := map[string]string{}
	sandbox := map[string]string{}
	if c != nil {
		ini, sandbox = optionDefaults(c.Server, false), optionDefaults(c.Sandbox, true)
	}
	for k, v := range a.readServerIni(h.World) {
		ini[k] = v
	}
	for k, v := range a.readSandbox(h.World) {
		sandbox[k] = v
	}
	for k, v := range h.Ini {
		ini[k] = v
		info.Ini = append(info.Ini, Override{k, v})
	}
	var base map[string]string
	if p := a.difficultyPreset(h.Difficulty); p != nil {
		base = p.Values
	}
	for k, v := range h.Sandbox {
		sandbox[k] = v
		if base == nil || base[k] != v {
			info.Sandbox = append(info.Sandbox, Override{k, v})
		}
	}
	sort.Slice(info.Ini, func(i, j int) bool { return info.Ini[i].Name < info.Ini[j].Name })
	sort.Slice(info.Sandbox, func(i, j int) bool { return info.Sandbox[i].Name < info.Sandbox[j].Name })
	info.PvP = ini["PVP"] == "true"
	info.Sleep = ini["SleepAllowed"] == "true" && ini["SleepNeeded"] == "true"
	if n, err := strconv.Atoi(sandbox["CharacterFreePoints"]); err == nil {
		info.FreePts = n
	}
	ids, _ := a.hostModList()
	for _, id := range ids {
		b := ModBrief{ID: id, Name: id}
		if c != nil {
			if m := c.mod(id); m != nil {
				b.Name, b.WorkshopID = m.Name, m.WorkshopID
			}
		}
		info.Mods = append(info.Mods, b)
	}
	return info
}

// optionDefaults reads the game's default for every option in an option list (pages of settings for the sandbox).
func optionDefaults(raw json.RawMessage, paged bool) map[string]string {
	type opt struct {
		Name    string          `json:"name"`
		Default json.RawMessage `json:"default"` // a string, or a number in some exports
	}
	val := func(r json.RawMessage) string { return strings.Trim(string(r), `"`) }
	out := map[string]string{}
	if paged {
		var pages []struct {
			Settings []opt `json:"settings"`
		}
		_ = json.Unmarshal(raw, &pages)
		for _, p := range pages {
			for _, o := range p.Settings {
				out[o.Name] = val(o.Default)
			}
		}
		return out
	}
	var list []opt
	_ = json.Unmarshal(raw, &list)
	for _, o := range list {
		out[o.Name] = val(o.Default)
	}
	return out
}

func (a *App) publishHostInfo() {
	id := a.lobby.ID()
	if id == 0 || !a.lobby.IsHost() {
		return
	}
	info := a.hostInfo()
	info.Roles = a.lobby.roleClaims()
	b, _ := json.Marshal(info)
	var mb []byte
	if c := a.catalog(); c != nil {
		ids, _ := a.hostModList()
		m := buildManifest(c, a.steam, ids)
		a.addLocalWorkshopIDs(&m)
		mb, _ = json.Marshal(m)
	}
	key := string(b) + string(mb)
	a.mu.Lock()
	same := key == a.published
	a.published = key
	a.mu.Unlock()
	if same {
		return
	}
	setLobbyBig(a.steam, id, "info", string(b))
	a.publishHistory()
	if mb != nil {
		setLobbyBig(a.steam, id, "manifest", string(mb))
	}
}

// publishMyData shares your character (all of it, so friends can look at it) with the lobby.
func (a *App) publishMyData() {
	id := a.lobby.ID()
	if id == 0 {
		return
	}
	cfg := a.cfg.Get()
	c := cfg.Character
	b, _ := json.Marshal(map[string]any{
		"name":   strings.TrimSpace(c.Forename + " " + c.Surname),
		"prof":   a.professionName(c.Profession),
		"traits": len(c.Traits),
		"female": c.Female,
		"c":      c,
	})
	a.lobby.setMyData("char", string(b))
	a.lobby.setMyData("app", appVersion)
	a.lobby.setMyData("role", cfg.Role)
	a.lobby.setMyData("knox", a.knoxMemberData())
	if g := a.gameState(); g.Running {
		a.lobby.setMyData("game", "1")
	} else {
		a.lobby.setMyData("game", "")
	}
}

func (a *App) professionName(id string) string {
	if c := a.catalog(); c != nil {
		for _, p := range c.Professions {
			if p.ID == id {
				return p.Name
			}
		}
	}
	return id
}

func (a *App) worldExists(name string) bool {
	return exists(filepath.Join(a.paths.savesDir(), name))
}

// Mods that sit in Zomboid/mods (a local copy) but were uploaded from Zomboid/Workshop/<x> have
// their Workshop id in workshop.txt there. Friends can download those, so they count as Workshop items.
func (a *App) localWorkshopIDs() map[string]string {
	out := map[string]string{}
	dirs, _ := os.ReadDir(filepath.Join(a.paths.Zomboid, "Workshop"))
	for _, d := range dirs {
		base := filepath.Join(a.paths.Zomboid, "Workshop", d.Name())
		b, err := os.ReadFile(filepath.Join(base, "workshop.txt"))
		if err != nil {
			continue
		}
		wid := ""
		for _, line := range strings.Split(string(b), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "id=") {
				wid = strings.TrimSpace(strings.TrimPrefix(line, "id="))
			}
		}
		if wid == "" {
			continue
		}
		mods, _ := os.ReadDir(filepath.Join(base, "Contents", "mods"))
		for _, m := range mods {
			out[m.Name()] = wid
		}
	}
	return out
}

func (a *App) addLocalWorkshopIDs(m *Manifest) {
	if len(m.Local) == 0 {
		return
	}
	ids := a.localWorkshopIDs()
	var still []string
	have := map[string]bool{}
	for _, it := range m.Items {
		have[it.ID] = true
	}
	for _, mod := range m.Local {
		if wid, ok := ids[mod]; ok {
			if !have[wid] {
				item := ManifestItem{ID: wid, Name: mod}
				if n, err := strconv.ParseUint(wid, 10, 64); err == nil {
					if _, ts, ok := a.steam.ItemInstallInfo(n); ok {
						item.TS = ts
					}
				}
				m.Items = append(m.Items, item)
				have[wid] = true
			}
			continue
		}
		still = append(still, mod)
	}
	m.Local = still
}

// Spawning --------------------------------------------------------------------------------------

// Places that have no spawn file of their own (spawn regions only cover towns you can pick in the game).
var extraTowns = []SpawnRegion{
	{Name: "Louisville, KY", Points: []SpawnPoint{{X: 12900, Y: 2300}}},
}

// towns lists every town with its spawn points (or centre), for the town picker and place names.
func (a *App) towns() []SpawnRegion {
	var out []SpawnRegion
	seen := map[string]bool{}
	if c := a.catalog(); c != nil {
		for _, r := range c.Spawns {
			if len(r.Points) > 0 {
				out = append(out, r)
				seen[r.Name] = true
			}
		}
	}
	for _, r := range extraTowns {
		if !seen[r.Name] {
			out = append(out, r)
		}
	}
	return out
}

// RandomHouse picks a building of the chosen type, in a town or anywhere, and returns where to stand.
func (a *App) RandomHouse(town, category string) (*SpawnSpot, error) {
	cat := buildingCategory(category)
	var cx, cy, radius float64
	if town != "" {
		for _, r := range a.towns() {
			if r.Name == town && len(r.Points) > 0 {
				for _, p := range r.Points {
					cx += float64(p.X)
					cy += float64(p.Y)
				}
				cx /= float64(len(r.Points))
				cy /= float64(len(r.Points))
				radius = 900
			}
		}
	}
	b, spot, err := a.wm.RandomBuilding(cat.Tones, cx, cy, radius)
	if err != nil && radius > 0 {
		b, spot, err = a.wm.RandomBuilding(cat.Tones, cx, cy, radius*3)
	}
	if err != nil {
		return nil, fmt.Errorf("no %s found there", strings.ToLower(cat.Name))
	}
	return &SpawnSpot{X: spot[0], Y: spot[1], Z: 0, Name: a.nearestTown(spot[0], spot[1]), Kind: b.Kind, Points: b.Points}, nil
}

func (a *App) nearestTown(x, y int) string {
	best, bd := "", 1<<62
	for _, r := range a.towns() {
		for _, p := range r.Points {
			d := (p.X-x)*(p.X-x) + (p.Y-y)*(p.Y-y)
			if d < bd {
				bd, best = d, r.Name
			}
		}
	}
	if bd > 2500*2500 {
		return "" // out in the country
	}
	return best
}

// Starting the game -------------------------------------------------------------------------------

func kv(lines map[string]string) string {
	keys := make([]string, 0, len(lines))
	for k := range lines {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k + "=" + strings.ReplaceAll(lines[k], "\n", " ") + "\r\n")
	}
	return b.String()
}

func (a *App) writeBridge(name, content string) error {
	_ = os.MkdirAll(a.paths.bridgeDir(), 0755)
	return os.WriteFile(a.paths.bridge(name), []byte(content), 0644)
}

// HostStart: called when the host presses START.
func (a *App) HostStart() error {
	if a.catalog() == nil {
		return errors.New("still loading, try again")
	}
	if a.gameState().Running {
		return errors.New("Zomboid is already running. Close it first.")
	}
	id := a.lobby.ID()
	cfg := a.cfg.Get()
	h := cfg.Host
	if strings.TrimSpace(h.World) == "" {
		return errors.New("pick a world first")
	}
	if c := a.castFor(h.World); c != nil && c.Ended && a.worldExists(h.World) {
		return errors.New("that tape is over (everyone is out). Pick another world or make a new one.")
	}
	if p := a.lobby.Phase(); p == PhaseSyncing || p == PhaseLaunching || p == PhaseInGame {
		return errors.New("already starting")
	}
	if _, problems := a.hostModList(); len(problems) > 0 {
		var list []string
		for id, why := range problems {
			list = append(list, id+": "+why)
		}
		sort.Strings(list)
		a.lobby.system("Left out of this game: " + strings.Join(list, "; "))
	}
	// no building picked yet: pick one now (the pin updates before the game opens)
	if h.Together && h.Spawn == nil {
		if spot, err := a.RandomHouse(h.SpawnTown, h.SpawnType); err == nil {
			_ = a.cfg.Update(func(c *Config) { c.Host.Spawn = spot })
		}
	}
	if id != 0 {
		if why := a.lobby.notReady(); why != "" {
			return errors.New(why)
		}
	}
	a.lobby.set(func() { a.lobby.phase = PhaseSyncing })
	expected := 1
	launchID := strconv.FormatInt(time.Now().UnixNano(), 36)
	a.noteLaunch(launchID)
	a.lobby.set(func() { a.lobby.sync, a.lobby.cancel = SyncState{}, false })
	if id != 0 {
		a.publishHostInfo()
		expected = len(a.steam.LobbyMembers(id))
		a.steam.SetLobbyData(id, "launch", launchID)
		a.steam.SetLobbyData(id, "state", "launching")
		a.sendManifestChat(id, launchID)
		a.steam.SendLobbyMessage(id, "go:"+launchID)
		a.lobby.system("Starting! Everyone's mods are being checked.")
	}
	go func() {
		if err := a.hostLaunch(expected, launchID); err != nil {
			a.lobby.set(func() { a.lobby.phase, a.lobby.note, a.lobby.sync.Wait = a.lobby.restPhase(), err.Error(), "" })
			if id != 0 {
				a.steam.SetLobbyData(id, "state", "open")
				a.lobby.announce("start stopped. " + err.Error())
			}
		}
	}()
	return nil
}

func (a *App) hostLaunch(expected int, launchID string) error {
	cfg := a.cfg.Get()
	c := a.catalog()
	ids, _ := a.hostModList()
	m := buildManifest(c, a.steam, ids)
	a.addLocalWorkshopIDs(&m)
	// an old server left running locks mod files (Steam can't update them) and holds the port
	if !a.gameState().Running && !a.server.get().Running {
		if n := killOrphanServers(filepath.Join(a.paths.Data, "server.pid")); n > 0 {
			log.Printf("stopped %d old server(s) left running", n)
		}
	}
	if err := a.syncItems(m, true); err != nil {
		return err
	}
	if err := a.ownCopyMismatch(c, ids); err != nil {
		return err
	}
	// timestamps can change while Steam updates an item: the server must send the ones on disk now,
	// and friends check their copies against them (a newer copy than the host's can't join)
	for i := range m.Items {
		if n, err := strconv.ParseUint(m.Items[i].ID, 10, 64); err == nil {
			if _, ts, ok := a.steam.ItemInstallInfo(n); ok {
				m.Items[i].TS = ts
			}
		}
	}
	if id := a.lobby.ID(); id != 0 {
		if mb, err := json.Marshal(m); err == nil {
			setLobbyBig(a.steam, id, "manifest", string(mb))
		}
	}
	a.lobby.setMyData("sync", "done:"+launchID)
	if err := writeDefaultMods(a.paths, ids); err != nil {
		return fmt.Errorf("couldn't write the mod list: %v", err)
	}
	if err := writeGamePresets(a.paths, cfg); err != nil {
		return fmt.Errorf("couldn't save your character: %v", err)
	}
	h := cfg.Host
	continuing := a.worldExists(h.World)
	if continuing {
		a.lobby.set(func() { a.lobby.sync.Current = "Backing up " + h.World })
		if _, err := a.backupWorld(h.World); err != nil {
			a.lobby.system("Backup failed: " + err.Error())
		}
	}
	spawn, town := "0,0,0", ""
	if h.Together && h.Spawn != nil {
		spawn, town = fmt.Sprintf("%d,%d,%d", h.Spawn.X, h.Spawn.Y, h.Spawn.Z), h.Spawn.Name
	}
	if err := a.writeServerConfig(h, ids, m, spawn); err != nil {
		return err
	}
	var spots []string
	if h.Respawn == "random" {
		for i := 0; i < 24; i++ {
			if s, err := a.RandomHouse("", "house"); err == nil {
				spots = append(spots, fmt.Sprintf("%d,%d,%d", s.X, s.Y, s.Z))
			}
		}
	}
	respawn := h.Respawn
	if respawn == "" {
		respawn = "base"
	}
	// for Knox Link on the server: lobby flag, town, players to wait for, kit, respawn rule, start, respawn houses,
	// lives, lives mode, world. Continuing a tape doesn't wait for everyone: people drop back in where they were.
	lives, livesMode := livesText(h)
	lobby := []string{boolText(h.Lobby && !continuing), town, strconv.Itoa(expected), kitText(h.Kit), respawn, spawn, strings.Join(spots, ";"),
		lives, livesMode, h.World, goalTier(h.Difficulty), h.Backstory}
	if err := a.writeBridge("lobby.txt", strings.Join(lobby, "\r\n")+"\r\n"); err != nil {
		return err
	}
	_ = a.writeBridge("app_recap.json", "")
	_ = a.writeBridge("app_cast.json", "")
	_ = a.writeBridge("app_goals.json", "")
	_ = a.writeBridge("app_status.json", "")
	_ = a.writeBridge("app_commands.txt", "")
	_ = a.writeBridge("app_launch.txt", "")

	// the host's game opens once the server is up and every friend has the mods.
	// If the network port won't open, try once more on another port.
	for try := 0; ; try++ {
		if err := a.StartServer(h.World, h.Memory); err != nil {
			return err
		}
		err := a.waitForServerAndFriends(launchID)
		if err == nil {
			break
		}
		a.server.Stop()
		a.server.mu.Lock()
		retry := a.server.portFail && try == 0
		a.server.mu.Unlock()
		if !retry {
			return err
		}
		for a.server.get().Running {
			time.Sleep(500 * time.Millisecond)
		}
		a.lobby.system("The server's network port was busy. Trying another one.")
		if err := a.writeServerConfig(h, ids, m, spawn); err != nil {
			return err
		}
		a.server.set(func(st *ServerState) { st.Error = "" })
	}
	srv := a.server.get()
	launch := map[string]string{
		"time":      strconv.FormatInt(time.Now().Unix(), 10),
		"mode":      "join",
		"host":      srv.SteamID,
		"hostName":  "your server",
		"character": gamePresetName,
		"tries":     "60",
	}
	if err := a.writeBridge("app_launch.txt", kv(launch)); err != nil {
		a.server.Stop()
		return err
	}
	a.lobby.set(func() { a.lobby.sync.Wait = "" })
	if err := a.launchGame(true); err != nil {
		a.server.Stop()
		return err
	}
	if id := a.lobby.ID(); id != 0 {
		go a.waitForHostIn(id, launchID)
	}
	return nil
}

// friendsWaiting lists lobby members whose mods aren't ready for this start.
func (a *App) friendsWaiting(launchID string) ([]string, error) {
	var waiting []string
	if a.lobby.ID() == 0 {
		return nil, nil
	}
	for _, m := range a.lobby.Members() {
		if m.Me {
			continue
		}
		switch {
		case m.Sync == "done:"+launchID:
		case strings.HasPrefix(m.Sync, "error: "):
			return nil, fmt.Errorf("%s couldn't get the mods: %s", m.Name, strings.TrimPrefix(m.Sync, "error: "))
		default:
			w := m.Name
			if strings.Contains(m.Sync, "/") {
				w += " (" + m.Sync + ")"
			}
			waiting = append(waiting, w)
		}
	}
	return waiting, nil
}

// waitForServerAndFriends waits for the server to finish loading and for every friend's mods.
func (a *App) waitForServerAndFriends(launchID string) error {
	id := a.lobby.ID()
	deadline := time.Now().Add(45 * time.Minute)
	for {
		srv := a.server.get()
		if !srv.Running {
			if srv.Error != "" {
				return errors.New(srv.Error)
			}
			return errors.New("The server stopped while starting.")
		}
		waiting, err := a.friendsWaiting(launchID)
		if err != nil {
			return err
		}
		if srv.Stage == "up" && srv.SteamID != "" && len(waiting) == 0 {
			return nil
		}
		parts := []string{"Server: " + srv.Text + "."}
		if len(waiting) > 0 {
			parts = append(parts, "Waiting for mods: "+strings.Join(waiting, ", ")+".")
		}
		msg := strings.Join(parts, " ")
		a.lobby.set(func() { a.lobby.sync.Wait = msg })
		if time.Now().After(deadline) {
			return errors.New("Gave up waiting. " + msg)
		}
		if a.lobby.ID() != id {
			return errors.New("left the lobby")
		}
		if a.lobby.takeCancel() {
			return errors.New("Start cancelled.")
		}
		time.Sleep(time.Second)
	}
}

// waitForHostIn opens friends' games once the host is in (the first player in becomes the co-op host,
// who gets the admin powers Knox Link needs). If the host never shows, friends are sent in anyway.
func (a *App) waitForHostIn(id uint64, launchID string) {
	tell := func(msg string) {
		a.steam.SetLobbyData(id, "hostin", "")
		a.steam.SetLobbyData(id, "serverid", a.server.get().SteamID)
		a.steam.SetLobbyData(id, "server", launchID)
		a.steam.SendLobbyMessage(id, "go:"+launchID)
		a.lobby.system(msg)
		log.Println("guests told to join:", msg)
	}
	// friends never wait on the host's own game for long: the server being up is what matters
	deadline := time.Now().Add(90 * time.Second)
	a.steam.SetLobbyData(id, "hostin", strconv.FormatInt(deadline.Unix(), 10))
	defer a.steam.SetLobbyData(id, "hostin", "")
	for time.Now().Before(deadline) {
		if a.lobby.ID() != id || !a.server.get().Running {
			return
		}
		if !a.gameState().Running {
			tell("Your game closed, but the server is still up. Friends' games are opening.")
			return
		}
		if st, ok := readJSONFile(a.paths.bridge("app_status.json")).(map[string]any); ok {
			if players, _ := st["players"].([]any); len(players) > 0 {
				tell("You're in. Friends' games are opening.")
				return
			}
		}
		// the server's own log says when the host's game connects (Knox Link's status file can lag)
		if a.hostConnected() {
			tell("You're in. Friends' games are opening.")
			return
		}
		time.Sleep(2 * time.Second)
	}
	tell("Your game is still loading. Friends' games are opening now.")
}

func boolText(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func kitText(kit []KitItem) string {
	var parts []string
	for _, k := range kit {
		if k.Type != "" && k.Count > 0 {
			parts = append(parts, fmt.Sprintf("%s:%d", clean(k.Type), k.Count))
		}
	}
	return strings.Join(parts, ";")
}

// guestLaunch: the host pressed START; get the same mods, then open the game and join.
// waitForHostServer holds a guest with mods ready until the host's server is up.
// It returns false if the host cancelled, the game closed, or you left.
func (a *App) waitForHostServer(lobby uint64, launchID string) bool {
	st := a.steam
	a.lobby.set(func() { a.lobby.sync.Wait = "Mods ready. Waiting for everyone, then the host's server." })
	deadline := time.Now().Add(60 * time.Minute)
	for time.Now().Before(deadline) {
		if a.lobby.ID() != lobby {
			return false
		}
		if a.lobby.takeCancel() {
			a.lobby.set(func() { a.lobby.phase, a.lobby.sync.Wait = PhaseLobby, "" })
			a.lobby.setMyData("sync", "error: cancelled")
			a.lobby.announce("cancelled the start.")
			return false
		}
		if st.LobbyData(lobby, "server") == launchID {
			a.lobby.set(func() { a.lobby.sync.Wait = "" })
			return true
		}
		if st.LobbyData(lobby, "state") != "launching" || st.LobbyData(lobby, "launch") != launchID {
			a.lobby.set(func() { a.lobby.phase, a.lobby.sync.Wait = PhaseLobby, "" })
			a.lobby.system("The host's start stopped. Back in the lobby.")
			return false
		}
		time.Sleep(time.Second)
	}
	a.lobby.set(func() { a.lobby.phase, a.lobby.sync.Wait = PhaseLobby, "" })
	a.lobby.system("Waited an hour for the host's server. Back in the lobby.")
	return false
}

func (a *App) guestLaunch(lobby uint64) {
	st := a.steam
	fail := func(msg string) {
		a.lobby.set(func() { a.lobby.phase, a.lobby.sync.Error = PhaseLobby, msg })
		a.lobby.setMyData("sync", "error: "+msg)
		a.lobby.announce("couldn't start. " + msg)
	}
	if a.gameState().Running {
		fail("Project Zomboid is already open. Close it, then ask the host to press START again.")
		return
	}
	var m Manifest
	raw := a.waitManifest(lobby, st.LobbyData(lobby, "launch"))
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		fail("The host's mod list didn't come through. Ask them to press START again.")
		return
	}
	if len(m.Local) > 0 {
		a.lobby.set(func() {
			a.lobby.sync.Warnings = append(a.lobby.sync.Warnings, "Not on the Workshop (you may get kicked): "+strings.Join(m.Local, ", "))
		})
	}
	if err := a.syncItems(m, false); err != nil {
		fail(err.Error())
		return
	}
	launchID := st.LobbyData(lobby, "launch")
	a.lobby.setMyData("sync", "done:"+launchID)
	if !a.waitForHostServer(lobby, launchID) {
		return
	}
	cfg := a.cfg.Get()
	if err := writeDefaultMods(a.paths, m.Mods); err != nil {
		fail("Couldn't write the mod list: " + err.Error())
		return
	}
	if err := writeGamePresets(a.paths, cfg); err != nil {
		fail("Couldn't save your character: " + err.Error())
		return
	}
	host := st.LobbyOwner(lobby)
	target := st.LobbyData(lobby, "serverid")
	if target == "" {
		target = strconv.FormatUint(host, 10)
	}
	launch := map[string]string{
		"time":      strconv.FormatInt(time.Now().Unix(), 10),
		"mode":      "join",
		"host":      target,
		"hostName":  clean(st.PersonaName(host)),
		"character": gamePresetName,
		"tries":     "60",
	}
	if err := a.writeBridge("app_launch.txt", kv(launch)); err != nil {
		fail(err.Error())
		return
	}
	a.lobby.setMyData("sync", "launching")
	if err := a.launchGame(false); err != nil {
		fail(err.Error())
		return
	}
	a.lobby.announce("mods ready, game opening.")
}

// syncItems subscribes to and downloads every Workshop item in the manifest, and waits for them.
// For the host (own=true) it only updates items that Steam says are out of date.
// Cancel, leaving the lobby, or (for a guest) the host stopping the start ends the wait.
func (a *App) syncItems(m Manifest, own bool) error {
	st := a.steam
	lobby := a.lobby.ID()
	launch := ""
	if lobby != 0 && !own {
		launch = st.LobbyData(lobby, "launch")
	}
	stopped := func() error {
		if a.lobby.ID() != lobby {
			return errors.New("left the lobby")
		}
		if a.lobby.takeCancel() {
			return errors.New("Start cancelled.")
		}
		if launch != "" && (st.LobbyData(lobby, "state") != "launching" || st.LobbyData(lobby, "launch") != launch) {
			return errors.New("The host's start stopped.")
		}
		return nil
	}
	type item struct {
		id   uint64
		name string
		ts   uint32
	}
	var items []item
	for _, it := range m.Items {
		n, err := strconv.ParseUint(it.ID, 10, 64)
		if err == nil {
			items = append(items, item{n, it.Name, it.TS})
		}
	}
	// the host's own Workshop uploads (Zomboid\Workshop) are already on disk: Steam has nothing to fetch
	local := map[uint64]bool{}
	if own {
		for _, wid := range a.localWorkshopIDs() {
			if n, err := strconv.ParseUint(wid, 10, 64); err == nil {
				local[n] = true
			}
		}
	}
	a.lobby.set(func() { a.lobby.sync = SyncState{Total: len(items)} })
	// the host asks the Workshop for each item's latest version: Steam doesn't always know a mod was
	// updated, and a friend with the newer copy gets kicked ("Lua checksums do not match")
	outdated := map[uint64]bool{}
	if q, ok := st.(interface{ ItemUpdated(uint64) (uint32, error) }); ok && own {
		a.lobby.set(func() { a.lobby.sync.Current = "Checking the Workshop for updates" })
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		// own uploads too: the copy friends get must be the one START compares yours with
		for i := range items {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				latest, err := q.ItemUpdated(items[i].id)
				_, ts, ok := st.ItemInstallInfo(items[i].id)
				if err != nil || !ok || latest <= ts {
					return
				}
				mu.Lock()
				items[i].ts = latest
				outdated[items[i].id] = true
				mu.Unlock()
				log.Printf("sync: %s (%d) has a newer version on the Workshop", items[i].name, items[i].id)
			}(i)
		}
		wg.Wait()
	}
	for _, it := range items {
		// own uploads still get updated when Steam has them installed: the server tells joining games
		// the installed copy's timestamp, so a half-updated copy makes the host's own game refuse to join
		if local[it.id] && st.ItemState(it.id)&ItemInstalled == 0 {
			continue
		}
		state := st.ItemState(it.id)
		if !own && state&ItemSubscribed == 0 {
			st.SubscribeItem(it.id)
		}
		_, ts, _ := st.ItemInstallInfo(it.id)
		if state&ItemInstalled == 0 || state&ItemNeedsUpdate != 0 || (it.ts != 0 && ts < it.ts) {
			st.DownloadItem(it.id, true)
		}
	}
	start := time.Now()
	deadline := start.Add(45 * time.Minute)
	stale := map[uint64]bool{} // installed, but Steam keeps saying "needs update" without fetching it
	busy := map[uint64]bool{}  // Steam was downloading it at some point
	lastLog := time.Time{}
	for {
		done := 0
		var bytes, bytesAll uint64
		current := ""
		for _, it := range items {
			state := st.ItemState(it.id)
			_, ts, ok := st.ItemInstallInfo(it.id)
			finished := ok && state&ItemInstalled != 0 && state&(ItemNeedsUpdate|ItemDownloading|ItemDownloadPending) == 0
			if (!own || outdated[it.id]) && finished && it.ts != 0 && ts < it.ts {
				finished = false
			}
			if own && state&ItemSubscribed == 0 && ok {
				finished = true // a mod the host has installed without a subscription
			}
			if local[it.id] && (!ok || state&ItemInstalled == 0) {
				finished = true
			}
			if state&(ItemDownloading|ItemDownloadPending) != 0 {
				busy[it.id] = true
			} else if !finished && busy[it.id] && ok && state&ItemInstalled != 0 && (it.ts == 0 || ts >= it.ts) {
				// the download finished, but Steam leaves "needs update" set while the game's app id
				// is running (the launcher uses it), so count it as done
				finished = true
			}
			// Steam sometimes flags an item for an update and then never downloads it. After a while,
			// use the copy that's installed instead of waiting forever.
			// a mod the Workshop says is out of date blocks the start: friends with the new copy can't join an old one
			if outdated[it.id] && !finished && time.Since(start) > 2*time.Minute && state&(ItemDownloading|ItemDownloadPending) == 0 {
				return fmt.Errorf("%s has an update on the Workshop, but Steam won't download it. Friends with the new version can't join. Restart Steam and press START again.", it.name)
			}
			// a friend can't join with a newer copy than the host's (Lua checksums don't match). The host
			// may still be updating it, so wait for the host's mods, then check the host's final list.
			if !own && ok && it.ts != 0 && ts > it.ts && state&(ItemDownloading|ItemDownloadPending) == 0 {
				finished = false
				if latest, hostDone := a.hostItemTS(it.id); hostDone {
					if latest != 0 && ts > latest {
						return fmt.Errorf("%s is newer on your PC than on the host's. The host needs to update it: restart Steam, then press START again.", it.name)
					}
					for i := range items {
						if items[i].id == it.id {
							items[i].ts = latest
						}
					}
					finished = latest == 0 || ts == latest
				}
			}
			wait := 40 * time.Second
			if !finished && !outdated[it.id] && ok && state&ItemInstalled != 0 && state&(ItemDownloading|ItemDownloadPending) == 0 && time.Since(start) > wait {
				finished = true
				if !stale[it.id] {
					stale[it.id] = true
					log.Printf("sync: %s (%d) never started updating (state %d), using the installed copy", it.name, it.id, state)
					name := it.name
					a.lobby.set(func() {
						a.lobby.sync.Warnings = append(a.lobby.sync.Warnings, name+": Steam didn't update it, so the copy you have is used.")
					})
				}
			}
			if finished {
				done++
				continue
			}
			if time.Since(lastLog) > 30*time.Second {
				log.Printf("sync: waiting on %s (%d), state %d, installed %v", it.name, it.id, state, ok)
			}
			d, t, _ := st.ItemDownloadInfo(it.id)
			bytes += d
			bytesAll += t
			if current == "" {
				current = it.name
			}
		}
		a.lobby.set(func() {
			a.lobby.sync.Done, a.lobby.sync.Current, a.lobby.sync.Bytes, a.lobby.sync.BytesAll = done, current, bytes, bytesAll
		})
		if time.Since(lastLog) > 30*time.Second {
			lastLog = time.Now()
		}
		a.lobby.setMyData("sync", fmt.Sprintf("%d/%d", done, len(items)))
		if done == len(items) {
			a.lobby.setMyData("sync", "ready")
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("Steam didn't finish downloading %s. Check Steam and try again.", current)
		}
		if err := stopped(); err != nil {
			return err
		}
		time.Sleep(time.Second)
	}
}

// launchGame starts Project Zomboid straight from its folder. The launcher stays open (and in the
// lobby) the whole time, showing the live game, and takes over again when the game closes.
func (a *App) launchGame(host bool) error {
	a.lobby.set(func() { a.lobby.phase = PhaseLaunching })
	direct := true
	if d := a.cfg.Get().Host.DirectLaunch; d != nil {
		direct = *d
	}
	var proc *gameProcess
	var err error
	if direct {
		proc, err = startGameDirect(a.paths.Game)
		if err != nil {
			log.Println("direct start failed, using Steam:", err)
		}
	}
	if proc == nil {
		err = startGameViaSteam()
	}
	if err != nil {
		a.mu.Lock()
		a.game.Error = err.Error()
		a.mu.Unlock()
		return fmt.Errorf("couldn't start Project Zomboid: %v", err)
	}
	a.mu.Lock()
	a.game = GameState{Running: true, Ours: proc != nil, Host: host, Started: time.Now().Unix(), Sessions: a.game.Sessions + 1}
	a.mu.Unlock()
	a.lobby.set(func() { a.lobby.phase = PhaseInGame })
	a.steam.SetRichPresence("status", "Playing with Knox Launcher")
	a.publishMyData()
	log.Printf("game started (direct=%v, host=%v)", proc != nil, host)
	if proc != nil {
		go func() {
			code := proc.Wait()
			log.Println("game exited, code", code)
			a.gameEnded(time.Since(time.Unix(a.gameState().Started, 0)))
		}()
	}
	return nil
}

func (a *App) gameEnded(ran time.Duration) {
	a.mu.Lock()
	a.game.Running = false
	a.game.Exited = time.Now().Unix()
	if ran < 20*time.Second {
		a.game.Error = "Project Zomboid closed straight away. Try starting it once from Steam, then press START again."
	}
	a.mu.Unlock()
	a.lobby.set(func() { a.lobby.phase = a.lobby.restPhase() })
	a.lobby.SetReady(false) // ready up again for the next game
	if a.gameState().Host {
		a.saveCast()
		a.recordGame()
		a.server.Stop()
	}
	if id := a.lobby.ID(); id != 0 && a.lobby.IsHost() {
		a.steam.SetLobbyData(id, "state", "open")
	}
	// random spawn: roll the next building now, so the map already shows it
	if h := a.cfg.Get().Host; a.gameState().Host && h.SpawnMode == "random" {
		if spot, err := a.RandomHouse(h.SpawnTown, h.SpawnType); err == nil {
			_ = a.cfg.Update(func(c *Config) { c.Host.Spawn = spot })
		}
	}
	a.publishMyData()
	a.publishHostInfo()
}

// checkGame notices a game started or closed outside the launcher.
func (a *App) checkGame() {
	g := a.gameState()
	running := gameRunning()
	if g.Running && !running && !g.Ours {
		a.gameEnded(time.Hour)
		return
	}
	if !g.Running && running {
		a.mu.Lock()
		a.game.Running, a.game.Ours, a.game.Started = true, false, time.Now().Unix()
		a.mu.Unlock()
	}
}

func (a *App) shutdown() {
	a.lobby.Leave()
	// the window is closed: let a running server save the world before the launcher goes
	// (Windows kills it with the launcher otherwise, losing everything since the last autosave)
	if a.server.get().Running {
		log.Println("closing: saving and stopping the server")
		a.server.Stop()
		for end := time.Now().Add(45 * time.Second); a.server.get().Running && time.Now().Before(end); {
			time.Sleep(250 * time.Millisecond)
		}
	}
	a.steam.Shutdown()
}

// Steam caps one lobby value at 8 KB, so long values are split over key.0, key.1, ... with key.n = count.
const lobbyChunk = 4000

func setLobbyBig(st Steam, lobby uint64, key, value string) int {
	value = packBig(value)
	sum := bigSum(value)
	size, n, failed := len(value), 0, 0
	for len(value) > 0 || n == 0 {
		cut := len(value)
		if cut > lobbyChunk {
			cut = lobbyChunk
			for cut > 0 && !utf8.RuneStart(value[cut]) {
				cut-- // never split a character in two
			}
		}
		if !st.SetLobbyData(lobby, key+"."+strconv.Itoa(n), value[:cut]) {
			failed++
		}
		value = value[cut:]
		n++
	}
	if !st.SetLobbyData(lobby, key+".sum", sum) {
		failed++
	}
	if !st.SetLobbyData(lobby, key+".n", strconv.Itoa(n)) {
		failed++
	}
	log.Printf("lobby %s: %d bytes in %d parts, %d failed", key, size, n, failed)
	return failed
}

// Long values go out gzipped ("z:" + base64), which keeps the whole lobby well under Steam's limits.
func packBig(v string) string {
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	_, _ = zw.Write([]byte(v))
	_ = zw.Close()
	return "z:" + base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

func unpackBig(v string) string {
	if !strings.HasPrefix(v, "z:") {
		return v
	}
	raw, err := base64.RawURLEncoding.DecodeString(v[2:])
	if err != nil {
		return ""
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return ""
	}
	out, err := io.ReadAll(zr)
	if err != nil {
		return ""
	}
	return string(out)
}

func bigSum(v string) string {
	h := sha1.Sum([]byte(v))
	return hex.EncodeToString(h[:8])
}

// The mod list also goes out as lobby chat lines ("mf:<launch>:<i>/<n>:<part>") when the host
// presses START. Chat lines arrive reliably even when lobby data is slow or too big for Steam.
const chatChunk = 3000

func (a *App) sendManifestChat(lobby uint64, launchID string) (n, size, failed int) {
	ids, _ := a.hostModList()
	m := buildManifest(a.catalog(), a.steam, ids)
	a.addLocalWorkshopIDs(&m)
	b, _ := json.Marshal(m)
	v := packBig(string(b))
	size = len(v)
	var parts []string
	for len(v) > 0 {
		cut := len(v)
		if cut > chatChunk {
			cut = chatChunk
		}
		parts = append(parts, v[:cut])
		v = v[cut:]
	}
	for i, p := range parts {
		if !a.steam.SendLobbyMessage(lobby, fmt.Sprintf("mf:%s:%d/%d:%s", launchID, i, len(parts), p)) {
			log.Printf("manifest chat part %d/%d didn't send", i, len(parts))
			failed++
		}
	}
	return len(parts), size, failed
}

// ModListTest is what the host's self-test found. Steam sends lobby chat back to the sender too,
// so the chat copy coming back proves the trip through Steam without a second player.
type ModListTest struct {
	Mods        int    `json:"mods"`
	Items       int    `json:"items"`
	ChatOK      bool   `json:"chatOK"`
	ChatParts   int    `json:"chatParts"`
	ChatMs      int64  `json:"chatMs"`
	LobbyFailed int    `json:"lobbyFailed"`
	LobbyOK     bool   `json:"lobbyOK"`
	Bytes       int    `json:"bytes"`
	Note        string `json:"note"`
}

func (a *App) testModList() (ModListTest, error) {
	var t ModListTest
	id := a.lobby.ID()
	if id == 0 || !a.lobby.IsHost() {
		return t, errors.New("make a lobby first (you can be the only one in it)")
	}
	if a.catalog() == nil {
		return t, errors.New("still loading, try again")
	}
	ids, _ := a.hostModList()
	m := buildManifest(a.catalog(), a.steam, ids)
	t.Mods, t.Items = len(ids), len(m.Items)

	// lobby data: publish again even if nothing changed, counting what Steam refused
	b, _ := json.Marshal(m)
	t.LobbyFailed = setLobbyBig(a.steam, id, "manifest", string(b))
	v, ok := waitLobbyBig(a.steam, id, "manifest", 5*time.Second)
	t.LobbyOK = ok && t.LobbyFailed == 0 && v == string(b)

	launch := "test" + strconv.FormatInt(time.Now().UnixNano(), 36)
	start := time.Now()
	var failed int
	t.ChatParts, t.Bytes, failed = a.sendManifestChat(id, launch)
	for time.Since(start) < 20*time.Second {
		a.mu.Lock()
		_, got := a.mfChat[launch]
		a.mu.Unlock()
		if got {
			if v := a.chatManifest(launch); v != "" {
				t.ChatOK, t.ChatMs = true, time.Since(start).Milliseconds()
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	switch {
	case failed > 0:
		t.Note = fmt.Sprintf("Steam refused %d chat part(s).", failed)
	case !t.ChatOK:
		t.Note = "The chat copy didn't come back from Steam within 20 seconds."
	}
	log.Printf("mod list test: %+v", t)
	return t, nil
}

func (a *App) gotManifestPart(text string) {
	f := strings.SplitN(strings.TrimPrefix(text, "mf:"), ":", 3)
	if len(f) != 3 {
		return
	}
	var i, n int
	if _, err := fmt.Sscanf(f[1], "%d/%d", &i, &n); err != nil || n < 1 || n > 64 || i < 0 || i >= n {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.mfChat == nil {
		a.mfChat = map[string][]string{}
	}
	parts := a.mfChat[f[0]]
	if len(parts) != n {
		parts = make([]string, n)
	}
	parts[i] = f[2]
	a.mfChat[f[0]] = parts
	a.mfLast = f[0]
}

func (a *App) chatManifest(launchID string) string {
	a.mu.Lock()
	parts, ok := a.mfChat[launchID]
	if !ok {
		parts = a.mfChat[a.mfLast] // lobby data with the launch id hasn't arrived yet
	}
	a.mu.Unlock()
	for _, p := range parts {
		if p == "" {
			return ""
		}
	}
	v := unpackBig(strings.Join(parts, ""))
	if !json.Valid([]byte(v)) {
		return ""
	}
	return v
}

// waitManifest takes the host's mod list from chat or lobby data, whichever is complete first.
func (a *App) waitManifest(lobby uint64, launchID string) string {
	deadline := time.Now().Add(30 * time.Second)
	for {
		if v := a.chatManifest(launchID); v != "" {
			return v
		}
		if v, ok := waitLobbyBig(a.steam, lobby, "manifest", 0); ok {
			return v
		}
		if time.Now().After(deadline) {
			log.Printf("no manifest for launch %s after 30 s", launchID)
			return ""
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func lobbyRaw(st Steam, lobby uint64, key string) string {
	n, _ := strconv.Atoi(st.LobbyData(lobby, key+".n"))
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString(st.LobbyData(lobby, key+"."+strconv.Itoa(i)))
	}
	return sb.String()
}

func lobbyBig(st Steam, lobby uint64, key string) string { return unpackBig(lobbyRaw(st, lobby, key)) }

// waitLobbyBig waits for every part of a long value to arrive. Steam sends lobby data a bit after
// chat messages, so right after the host's "go" a friend can still have none or half of it.
func waitLobbyBig(st Steam, lobby uint64, key string, wait time.Duration) (string, bool) {
	deadline := time.Now().Add(wait)
	for {
		raw := lobbyRaw(st, lobby, key)
		sum := st.LobbyData(lobby, key+".sum")
		if raw != "" && (sum == bigSum(raw) || sum == "") {
			if v := unpackBig(raw); v != "" && json.Valid([]byte(v)) {
				return v, true
			}
		}
		if time.Now().After(deadline) {
			if wait == 0 {
				return "", false
			}
			log.Printf("lobby %s incomplete: %d bytes, sum %q vs %q, n=%s", key, len(raw), sum, bigSum(raw), st.LobbyData(lobby, key+".n"))
			return "", false
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func randomName(female bool) (string, string) {
	m := []string{"Wes", "Hank", "Ray", "Dale", "Earl", "Travis", "Cody", "Luke", "Mitch", "Carl"}
	f := []string{"Joan", "Kate", "Lucy", "Darla", "Tammy", "Rose", "Megan", "Holly", "June", "Beth"}
	s := []string{"Sutton", "Haskell", "Porter", "Mills", "Turner", "Baker", "Cole", "Hayes", "Doyle", "Reed"}
	if female {
		return f[rand.Intn(len(f))], s[rand.Intn(len(s))]
	}
	return m[rand.Intn(len(m))], s[rand.Intn(len(s))]
}

// hostConnected reports whether this run's server log shows the host's game connected.
func (a *App) hostConnected() bool {
	file := filepath.Join(a.paths.Zomboid, "coop-console.txt")
	fi, err := os.Stat(file)
	started := a.server.get().Started
	if err != nil || fi.ModTime().Unix() < started {
		return false
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return false
	}
	// an older run's lines can still be at the top if the server appends to the log
	a.server.mu.Lock()
	off, head := a.server.logOff, a.server.logHead
	a.server.mu.Unlock()
	if off > 0 && off <= int64(len(b)) && bytes.HasPrefix(b, head) {
		b = b[off:]
	}
	return bytes.Contains(b, []byte("isCoopHost=true")) || bytes.Contains(b, []byte("Connected new client"))
}

// hostItemTS says whether the host's mods are ready for this start and, if so, the version (install
// timestamp) of one Workshop item in the host's final mod list.
func (a *App) hostItemTS(item uint64) (uint32, bool) {
	id := a.lobby.ID()
	if id == 0 {
		return 0, false
	}
	owner := a.steam.LobbyOwner(id)
	// "done:" from an earlier start doesn't count: the host may still be updating this one
	if launch := a.steam.LobbyData(id, "launch"); launch == "" || a.steam.MemberData(id, owner, "sync") != "done:"+launch {
		return 0, false
	}
	var m Manifest
	if json.Unmarshal([]byte(lobbyBig(a.steam, id, "manifest")), &m) != nil {
		return 0, false
	}
	want := strconv.FormatUint(item, 10)
	for _, it := range m.Items {
		if it.ID == want {
			return it.TS, true
		}
	}
	return 0, true
}
