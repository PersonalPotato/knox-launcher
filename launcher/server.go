package main

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed ui
var uiFiles embed.FS

type Server struct {
	app   *App
	token string
	addr  string
}

func newServer(app *App) *Server {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return &Server{app: app, token: hex.EncodeToString(b)}
}

func (s *Server) Listen(port int) (string, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return "", err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.index)
	sub, _ := fs.Sub(uiFiles, "ui")
	mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.FS(sub))))
	mux.HandleFunc("/rpc", s.rpc)
	mux.HandleFunc("/tile/", s.tile)
	mux.HandleFunc("/avatar/", s.avatar)
	mux.HandleFunc("/poster", s.poster)
	mux.HandleFunc("/savethumb", s.saveThumb)
	mux.HandleFunc("/art", s.art)
	mux.HandleFunc("/model", s.model)
	mux.HandleFunc("/tex", s.tex)
	mux.HandleFunc("/anim", s.anim)
	mux.HandleFunc("/bodytex", s.bodytex)
	go http.Serve(ln, mux)
	s.addr = ln.Addr().String()
	return "http://" + s.addr + "/?t=" + s.token, nil
}

func (s *Server) authed(r *http.Request) bool {
	return r.URL.Query().Get("t") == s.token || r.Header.Get("X-Token") == s.token
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" || !s.authed(r) {
		http.NotFound(w, r)
		return
	}
	b, _ := uiFiles.ReadFile("ui/index.html")
	html := strings.Replace(string(b), "__TOKEN__", s.token, 1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(html))
}

func (s *Server) tile(w http.ResponseWriter, r *http.Request) {
	if !s.authed(r) {
		http.NotFound(w, r)
		return
	}
	var z, x, y int
	if _, err := fmt.Sscanf(strings.TrimPrefix(r.URL.Path, "/tile/"), "%d/%d/%d.png", &z, &x, &y); err != nil {
		http.NotFound(w, r)
		return
	}
	b, err := s.app.wm.Tile(z, x, y)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "max-age=86400")
	_, _ = w.Write(b)
}

func (s *Server) avatar(w http.ResponseWriter, r *http.Request) {
	if !s.authed(r) {
		http.NotFound(w, r)
		return
	}
	id, _ := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/avatar/"), ".png"), 10, 64)
	b := s.app.steam.Avatar(id)
	if b == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(b)
}

// poster serves a mod's poster image from its own folder.
func (s *Server) saveThumb(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dir, err := s.app.savePath(q.Get("mode"), q.Get("name"))
	if !s.authed(r) || err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "max-age=60")
	http.ServeFile(w, r, filepath.Join(dir, "thumb.png"))
}

func (s *Server) poster(w http.ResponseWriter, r *http.Request) {
	if !s.authed(r) {
		http.NotFound(w, r)
		return
	}
	c := s.app.catalog()
	if c == nil {
		http.NotFound(w, r)
		return
	}
	m := c.mod(r.URL.Query().Get("mod"))
	if m == nil || len(m.Posters) == 0 {
		http.NotFound(w, r)
		return
	}
	roots := append([]string{m.Dir}, m.roots...)
	for _, root := range roots {
		if root == "" {
			continue
		}
		p := filepath.Clean(filepath.Join(root, m.Posters[0]))
		if !strings.HasPrefix(p, filepath.Clean(root)) {
			continue
		}
		if b, err := os.ReadFile(p); err == nil {
			w.Header().Set("Cache-Control", "max-age=3600")
			_, _ = w.Write(b)
			return
		}
	}
	http.NotFound(w, r)
}

// model serves one of the game's 3D models (from the game or a mod) as JSON for the preview.
func (s *Server) model(w http.ResponseWriter, r *http.Request) {
	if !s.authed(r) {
		http.NotFound(w, r)
		return
	}
	m, err := s.app.assets.Mesh(s.app.medias(), r.URL.Query().Get("p"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=3600")
	_ = json.NewEncoder(w).Encode(m)
}

// anim serves one of the game's character animations by name ("Bob_Idle").
func (s *Server) anim(w http.ResponseWriter, r *http.Request) {
	if !s.authed(r) {
		http.NotFound(w, r)
		return
	}
	a, err := s.app.assets.Anim(s.app.medias(), r.URL.Query().Get("n"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=3600")
	_ = json.NewEncoder(w).Encode(a)
}

// tex serves a texture by its game name ("Body/MaleBody01", "Clothes/Jacket/Jacket_Black", ...).
func (s *Server) tex(w http.ResponseWriter, r *http.Request) {
	if !s.authed(r) {
		http.NotFound(w, r)
		return
	}
	p := s.app.assets.Texture(s.app.medias(), r.URL.Query().Get("p"))
	if p == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "max-age=3600")
	http.ServeFile(w, r, p)
}

// bodytex serves a body texture with clothes painted on: ?base=Body/MaleBody01&l=path|r,g,b&l=path
func (s *Server) bodytex(w http.ResponseWriter, r *http.Request) {
	if !s.authed(r) {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	var layers []BodyLayer
	for _, l := range q["l"] {
		path, tint, _ := strings.Cut(l, "|")
		layer := BodyLayer{Path: path}
		if parts := strings.Split(tint, ","); len(parts) == 3 {
			var t [3]float64
			for i, p := range parts {
				t[i], _ = strconv.ParseFloat(p, 64)
			}
			layer.Tint = &t
		}
		layers = append(layers, layer)
	}
	var masks []BodyMask
	for _, m := range q["m"] {
		folder, part, _ := strings.Cut(m, "|")
		if n, err := strconv.Atoi(part); err == nil {
			masks = append(masks, BodyMask{Folder: strings.TrimPrefix(strings.ReplaceAll(folder, "\\", "/"), "media/textures/"), Part: n})
		}
	}
	b, err := s.app.assets.BodyTexture(s.app.medias(), q.Get("base"), layers, masks)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "max-age=3600")
	_, _ = w.Write(b)
}

type rpcReq struct {
	Method string          `json:"method"`
	Args   json.RawMessage `json:"args"`
}

func (s *Server) rpc(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || !s.authed(r) {
		http.Error(w, "no", http.StatusForbidden)
		return
	}
	var req rpcReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	res, err := s.call(req.Method, req.Args)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}
	if raw, ok := res.(json.RawMessage); ok {
		_, _ = w.Write([]byte(`{"result":`))
		_, _ = w.Write(raw)
		_, _ = w.Write([]byte(`}`))
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"result": res})
}

func (s *Server) call(method string, args json.RawMessage) (any, error) {
	a := s.app
	var p struct {
		ID        string          `json:"id"`
		Code      string          `json:"code"`
		Name      string          `json:"name"`
		Text      string          `json:"text"`
		Ready     bool            `json:"ready"`
		On        bool            `json:"on"`
		World     string          `json:"world"`
		File      string          `json:"file"`
		Mods      []string        `json:"mods"`
		X         int             `json:"x"`
		Y         int             `json:"y"`
		Cmd       string          `json:"cmd"`
		Arg       string          `json:"arg"`
		URL       string          `json:"url"`
		Since     int             `json:"since"`
		Version   string          `json:"version"`
		Town      string          `json:"town"`
		Type      string          `json:"type"`
		Items     []KitItem       `json:"items"`
		Character *Character      `json:"character"`
		Host      json.RawMessage `json:"host"`
		Keep      int             `json:"keep"`
		Mode      string          `json:"mode"`
		Force     bool            `json:"force"`
		Look      Look            `json:"look"`
	}
	// your stories and the community browser (community.go)
	if r, ok, err := a.communityCall(method, args); ok {
		return r, err
	}
	// per-tape titles and covers (tapes.go)
	if r, ok, err := a.tapeCall(method, args); ok {
		return r, err
	}
	// launcher self-update from GitHub (update.go)
	if r, ok, err := a.updateCall(method, args); ok {
		return r, err
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, err
		}
	}
	changed := func(err error) (any, error) {
		go a.publishHostInfo()
		return nil, err
	}
	switch method {
	case "state":
		return s.state(p.Since), nil
	case "catalog":
		a.cat.Refresh(false)
		c, raw := a.cat.Get()
		if c == nil {
			return nil, errors.New("still reading the game files")
		}
		if p.Version != "" && p.Version == c.Version {
			return map[string]any{"same": true}, nil
		}
		return json.RawMessage(raw), nil
	case "presets":
		cfg := a.cfg.Get()
		stories, packs := storyList(a.catalog())
		stories = append(stories, a.userStories()...)
		return map[string]any{
			"difficulty":  a.difficultyPresets(),
			"loadouts":    allLoadouts(a.catalog(), cfg.Loadouts),
			"backstories": stories,
			"modPacks":    packs,
			"extras":      activeExtras(a.catalog(), nil),
			"buildings":   buildingCategories,
			"towns":       a.towns(),
		}, nil
	case "paths":
		return a.paths, nil
	case "config":
		return a.cfg.Get(), nil
	case "seenGuide":
		return nil, a.cfg.Update(func(c *Config) { c.SeenGuide = true })
	case "setTab":
		return nil, a.cfg.Update(func(c *Config) { c.Tab = p.Name })

	// character
	case "saveCharacter":
		if p.Character == nil {
			return nil, errors.New("no character")
		}
		err := a.cfg.Update(func(c *Config) { c.Character = *p.Character })
		a.publishMyData()
		return nil, err
	case "savePreset":
		name := strings.TrimSpace(clean(p.Name))
		if name == "" || name == gamePresetName {
			return nil, errors.New("pick another name")
		}
		return nil, a.cfg.Update(func(c *Config) { c.Presets[name] = c.Character })
	case "loadPreset":
		var found bool
		err := a.cfg.Update(func(c *Config) {
			if ch, ok := c.Presets[p.Name]; ok {
				c.Character, found = ch, true
			}
		})
		if !found {
			return nil, errors.New("no preset called " + p.Name)
		}
		a.publishMyData()
		return a.cfg.Get().Character, err
	case "deletePreset":
		return nil, a.cfg.Update(func(c *Config) { delete(c.Presets, p.Name) })
	case "takeRole":
		if p.Name != "" && a.steamOK() {
			if who, ok := a.lobby.roleClaims()[p.Name]; ok && who != strconv.FormatUint(a.steam.MyID(), 10) {
				return nil, errors.New("someone already took that role")
			}
		}
		err := a.cfg.Update(func(c *Config) { c.Role = p.Name })
		a.publishMyData()
		return changed(err)

	// host settings
	case "patchHost":
		return changed(a.patchHost(p.Host))
	case "setMod":
		ch, err := a.SetMod(p.ID, p.On)
		go a.publishHostInfo()
		return ch, err
	case "setMods":
		return changed(a.SetMods(p.Mods))
	case "savePack":
		name := strings.TrimSpace(clean(p.Name))
		if name == "" {
			return nil, errors.New("give the pack a name")
		}
		mods := a.chosenMods() // read before Update: Update holds the config lock
		return changed(a.cfg.Update(func(c *Config) { c.Packs[name] = mods; c.Host.Pack = name }))
	case "setModFolder":
		// one mod into one folder ("" takes it out of every folder)
		return changed(a.cfg.Update(func(c *Config) {
			if c.ModFolders == nil {
				c.ModFolders = map[string][]string{}
			}
			for f, ids := range c.ModFolders {
				out := ids[:0]
				for _, id := range ids {
					if id != p.ID {
						out = append(out, id)
					}
				}
				c.ModFolders[f] = out
			}
			if name := strings.TrimSpace(clean(p.Name)); name != "" {
				c.ModFolders[name] = append(c.ModFolders[name], p.ID)
			}
		}))
	case "setModFolders":
		var folders map[string][]string
		if err := json.Unmarshal(args, &struct {
			Folders *map[string][]string `json:"folders"`
		}{&folders}); err != nil {
			return nil, err
		}
		return changed(a.cfg.Update(func(c *Config) { c.ModFolders = folders }))
	case "renameModFolder":
		to := strings.TrimSpace(clean(p.Text))
		if to == "" {
			return nil, errors.New("give the folder a name")
		}
		return changed(a.cfg.Update(func(c *Config) {
			if ids, ok := c.ModFolders[p.Name]; ok {
				delete(c.ModFolders, p.Name)
				c.ModFolders[to] = append(c.ModFolders[to], ids...)
			}
		}))
	case "newModFolder":
		name := strings.TrimSpace(clean(p.Name))
		if name == "" {
			return nil, errors.New("give the folder a name")
		}
		return changed(a.cfg.Update(func(c *Config) {
			if c.ModFolders == nil {
				c.ModFolders = map[string][]string{}
			}
			if _, ok := c.ModFolders[name]; !ok {
				c.ModFolders[name] = []string{}
			}
		}))
	case "deleteModFolder":
		return changed(a.cfg.Update(func(c *Config) { delete(c.ModFolders, p.Name) }))
	case "packCode":
		sp, err := a.sharedPack(p.Name)
		if err != nil {
			return nil, err
		}
		return packCode(sp), nil
	case "importPack":
		sp, err := parsePackCode(p.Code)
		if err != nil {
			return nil, err
		}
		return a.importPack(sp)
	case "sharePack":
		sp, err := a.sharedPack(p.Name)
		if err != nil {
			return nil, err
		}
		if a.lobby.ID() == 0 {
			return nil, errors.New("you're not in a lobby")
		}
		b, _ := json.Marshal(sp)
		a.lobby.setMyData("pack", string(b))
		a.lobby.announce("shared the mod pack " + sp.Name + ".")
		return nil, nil
	case "renamePack":
		to := strings.TrimSpace(clean(p.Text))
		if to == "" {
			return nil, errors.New("give the pack a name")
		}
		return changed(a.cfg.Update(func(c *Config) {
			if mods, ok := c.Packs[p.Name]; ok {
				delete(c.Packs, p.Name)
				c.Packs[to] = mods
				if c.Host.Pack == p.Name {
					c.Host.Pack = to
				}
			}
		}))
	case "usePack":
		return changed(a.cfg.Update(func(c *Config) {
			if _, ok := c.Packs[p.Name]; ok || p.Name == "" {
				c.Host.Pack = p.Name
			}
		}))
	case "deletePack":
		return changed(a.cfg.Update(func(c *Config) {
			delete(c.Packs, p.Name)
			if c.Host.Pack == p.Name {
				c.Host.Pack = ""
			}
		}))
	case "hostMods":
		ids, problems := a.hostModList()
		return map[string]any{"mods": ids, "problems": problems}, nil
	case "unsubscribe":
		id, err := strconv.ParseUint(p.ID, 10, 64)
		if err != nil {
			return nil, errors.New("not a Workshop item")
		}
		a.steam.UnsubscribeItem(id)
		a.workshopChanged()
		return nil, nil
	case "javaMods":
		ids := p.Mods
		if ids == nil {
			ids, _ = a.hostModList()
		}
		return a.javaMods(ids), nil
	case "setJava":
		return nil, a.setJava(p.Mods, p.On)
	case "workshop":
		var q WorkshopQuery
		if len(args) > 0 {
			if err := json.Unmarshal(args, &q); err != nil {
				return nil, err
			}
		}
		if !a.steam.Ready() {
			return nil, errors.New("Steam isn't running")
		}
		return a.steam.QueryWorkshop(q)
	case "subscribe":
		id, err := strconv.ParseUint(p.ID, 10, 64)
		if err != nil {
			return nil, errors.New("not a Workshop item")
		}
		a.steam.SubscribeItem(id)
		a.steam.DownloadItem(id, true)
		a.workshopChanged()
		return nil, nil
	case "saveSettingsPreset":
		name := strings.TrimSpace(clean(p.Name))
		if name == "" {
			return nil, errors.New("give the preset a name")
		}
		h := a.cfg.Get().Host
		sp := SettingsPreset{Sandbox: map[string]string{}, Ini: map[string]string{}, Saved: time.Now().Unix()}
		for k, v := range h.Sandbox {
			sp.Sandbox[k] = v
		}
		for k, v := range h.Ini {
			sp.Ini[k] = v
		}
		return changed(a.cfg.Update(func(c *Config) {
			if c.Settings == nil {
				c.Settings = map[string]SettingsPreset{}
			}
			c.Settings[name] = sp
			c.Host.Difficulty = "mine:" + name
		}))
	case "deleteSettingsPreset":
		return changed(a.cfg.Update(func(c *Config) { delete(c.Settings, p.Name) }))
	case "applyDifficulty":
		return changed(a.applyDifficulty(p.ID))
	case "applyLoadout":
		return changed(a.applyLoadout(p.ID))
	case "saveLoadout":
		name := strings.TrimSpace(clean(p.Name))
		if name == "" {
			return nil, errors.New("give the loadout a name")
		}
		return changed(a.cfg.Update(func(c *Config) {
			c.Loadouts[name] = p.Items
			c.Host.Kit, c.Host.KitName = p.Items, name
		}))
	case "deleteLoadout":
		return nil, a.cfg.Update(func(c *Config) { delete(c.Loadouts, p.Name) })
	case "setBackstory":
		return changed(a.applyBackstory(p.ID))
	case "randomHouse":
		spot, err := a.RandomHouse(p.Town, p.Type)
		if err != nil {
			return nil, err
		}
		_ = a.cfg.Update(func(c *Config) {
			c.Host.Spawn = spot
			c.Host.SpawnTown, c.Host.SpawnType = p.Town, p.Type
		})
		go a.publishHostInfo()
		return spot, nil
	case "setKeepBackups":
		return nil, a.cfg.Update(func(c *Config) { c.KeepBackups = max(1, min(p.Keep, 50)) })

	// lobby
	case "friends":
		if !a.steamOK() {
			return []Friend{}, nil
		}
		return a.steam.Friends(), nil
	case "connectSteam":
		return nil, a.steamReady()
	case "createLobby":
		return nil, a.lobby.Create()
	case "joinLobby":
		id, err := strconv.ParseUint(p.ID, 10, 64)
		if err != nil || id == 0 {
			id, err = parseLobbyCode(p.Code)
			if err != nil {
				return nil, errors.New("that code doesn't look right")
			}
		}
		return nil, a.lobby.Join(id)
	case "leaveLobby":
		a.lobby.Leave()
		return nil, nil
	case "invite":
		id, _ := strconv.ParseUint(p.ID, 10, 64)
		if a.lobby.ID() == 0 {
			return nil, errors.New("make a lobby first")
		}
		if !a.steam.InviteToLobby(a.lobby.ID(), id) {
			return nil, errors.New("Steam didn't send the invite")
		}
		a.noteInvite(id)
		return nil, nil
	case "packAdded": // you added a pack a lobby member shared: tell its owner
		if a.lobby.ID() != 0 && p.ID != "" {
			a.steam.SendLobbyMessage(a.lobby.ID(), "pk:"+p.ID)
		}
		return nil, nil
	case "knox":
		return map[string]any{"me": a.knoxProfile(), "badges": allBadges(), "titles": titles, "looks": lookDefs()}, nil
	case "setLook":
		return nil, a.setLook(p.Look)
	case "directorsCut":
		return nil, a.directorsCut()
	case "shelf":
		return a.shelf(), nil
	case "binTape":
		if p.World != "" {
			return nil, a.binTape("world", "", p.World)
		}
		return nil, a.binTape("solo", p.Mode, p.Name)
	case "binList":
		return a.binList(), nil
	case "restoreTape":
		return nil, a.restoreTape(p.ID)
	case "setSquadName":
		err := a.cfg.Update(func(c *Config) {
			c.Host.SquadName = strings.TrimSpace(p.Name)
			k := tapeKey("world", "", c.Host.World) // the squad name is the hosted tape's title
			if t, ok := c.Tapes[k]; ok && c.Host.SquadName != "" {
				t.Title = c.Host.SquadName
				c.Tapes[k] = t
			}
		})
		a.publishHostInfo()
		return nil, err
	case "stopServer":
		a.server.Stop()
		return nil, nil
	case "cancelStart":
		a.lobby.Cancel()
		return nil, nil
	case "ready":
		a.lobby.SetReady(p.Ready)
		return nil, nil
	case "chat":
		a.lobby.Say(p.Text)
		return nil, nil
	case "start":
		return nil, a.HostStart()
	case "playSolo":
		world, err := a.SoloStart(p.Name)
		if err == nil {
			a.moveSoloTape(p.Name, world)
		}
		return map[string]string{"world": world}, err
	case "testModList":
		return a.testModList()
	case "zbStatus":
		return a.zbStatus(p.Mods, p.Force), nil
	case "zbSetup":
		if err := a.zbSetup(); err != nil {
			return nil, err
		}
		return a.zbStatus(p.Mods, false), nil
	case "saves":
		return a.saves(), nil
	case "loadSave":
		return nil, a.LoadSave(p.Mode, p.Name)

	// worlds
	case "worldSettings":
		return map[string]any{"ini": a.readServerIni(p.World), "sandbox": a.readSandbox(p.World), "exists": a.worldExists(p.World)}, nil
	case "worlds":
		return a.worlds(), nil
	case "backups":
		return a.listBackups(p.World), nil
	case "backupNow":
		if a.gameState().Running {
			return nil, errors.New("close the game first")
		}
		return a.backupWorld(p.World)
	case "restore":
		if a.gameState().Running {
			return nil, errors.New("close the game first")
		}
		return nil, a.restoreBackup(p.World, p.File)
	case "mapInfo":
		return a.wm.Info()
	case "mapPick":
		b, spot, ok := a.wm.BuildingAt(p.X, p.Y)
		if !ok {
			return map[string]any{"ok": false}, nil
		}
		ss := &SpawnSpot{X: spot[0], Y: spot[1], Name: a.nearestTown(spot[0], spot[1]), Kind: b.Kind, Points: b.Points}
		_ = a.cfg.Update(func(c *Config) { c.Host.Spawn, c.Host.SpawnMode = ss, "pick" })
		go a.publishHostInfo()
		return map[string]any{"ok": true, "spot": ss}, nil

	// live game
	case "status":
		return a.liveStatus(), nil
	case "cast":
		// who's in a world: alive, dead, lives left, THE END
		return a.castFor(p.World), nil
	case "command":
		return nil, a.sendCommand(p.Cmd, p.Arg)
	case "openURL":
		if !strings.HasPrefix(p.URL, "https://") && !strings.HasPrefix(p.URL, "steam://") {
			return nil, errors.New("bad link")
		}
		return nil, openURL(p.URL)
	case "openFolder":
		return nil, openURL(a.paths.Zomboid)
	}
	return nil, fmt.Errorf("unknown call %q", method)
}

func (s *Server) state(since int) map[string]any {
	a := s.app
	l := a.lobby
	members := l.Members()
	l.mu.Lock()
	chat := l.chat
	if since > 0 && since <= len(chat) {
		chat = chat[since:]
	} else if since > len(chat) {
		chat = nil
	}
	out := map[string]any{
		"version": appVersion,
		"phase":   l.phase,
		"host":    l.isHost,
		"lobby":   strconv.FormatUint(l.id, 10),
		"code":    lobbyCode(l.id),
		"chat":    chat,
		"chatLen": len(l.chat),
		"sync":    l.sync,
		"note":    l.note,
	}
	l.note = ""
	id := l.id
	l.mu.Unlock()
	out["members"] = members
	a.readHistory()
	out["squad"] = a.squadStats(members)
	a.mu.Lock()
	out["steam"] = map[string]any{"ok": a.steamUp, "error": a.steamErr}
	out["game"] = a.game
	out["server"] = a.server.get()
	a.mu.Unlock()
	out["cfgRev"] = a.cfg.Rev()
	a.updateRemote(false)
	out["update"] = updateStatus()
	if c := a.catalog(); c != nil {
		out["catalog"] = c.Version
	}
	if a.steamOK() {
		out["me"] = map[string]any{"id": strconv.FormatUint(a.steam.MyID(), 10), "name": a.steam.MyName()}
		if id != 0 {
			var info HostInfo
			if l.IsHost() {
				info = a.hostInfo()
				info.Roles = l.roleClaims()
				out["info"] = info
			} else if json.Unmarshal([]byte(lobbyBig(a.steam, id, "info")), &info) == nil {
				out["info"] = info
				a.keepGuestCopy(id, info)
			}
			out["hostName"] = a.steam.LobbyData(id, "host")
			out["lobbyState"] = a.steam.LobbyData(id, "state")
			out["hostServer"] = a.steam.LobbyData(id, "srv")
			if t, err := strconv.ParseInt(a.steam.LobbyData(id, "hostin"), 10, 64); err == nil {
				out["hostIn"] = t // when friends' games open even if the host's game isn't in yet
			}
		}
	}
	if id == 0 {
		// solo: show your own setup the same way a lobby would
		info := a.hostInfo()
		out["info"] = info
	}
	return out
}

type WorldEntry struct {
	Name    string `json:"name"`
	Saved   bool   `json:"saved"`
	Backups int    `json:"backups"`
	Played  int64  `json:"played"`
}

func (a *App) worlds() []WorldEntry {
	seen := map[string]bool{}
	var out []WorldEntry
	add := func(name string) {
		if name == "" || seen[name] || name == "EZHostProbe" {
			return
		}
		seen[name] = true
		e := WorldEntry{Name: name, Saved: a.worldExists(name), Backups: len(a.listBackups(name))}
		if info, err := os.Stat(filepath.Join(a.paths.savesDir(), name)); err == nil {
			e.Played = info.ModTime().Unix()
		}
		out = append(out, e)
	}
	entries, _ := os.ReadDir(a.paths.serverDir())
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".ini") {
			add(strings.TrimSuffix(e.Name(), ".ini"))
		}
	}
	return out
}

// Live game ----------------------------------------------------------------------

func readJSONFile(path string) any {
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return nil
	}
	var v any
	if json.Unmarshal(b, &v) != nil {
		return nil
	}
	return v
}

func (a *App) liveStatus() map[string]any {
	out := map[string]any{"running": a.gameState().Running, "now": time.Now().Unix()}
	out["status"] = readJSONFile(a.paths.bridge("app_status.json"))
	out["recap"] = readJSONFile(a.paths.bridge("app_recap.json"))
	if a.gameState().Host {
		out["goals"] = a.goalsFor(a.cfg.Get().Host.World)
		out["cast"] = a.saveCast()
	} else {
		out["cast"] = readCast(a.paths.bridge("app_cast.json"))
	}
	return out
}

var commandSeq = time.Now().Unix()

// sendCommand asks the host's game to run an admin command (the game reads app_commands.txt).
func (a *App) sendCommand(cmd, arg string) error {
	switch cmd {
	case "kick", "ban", "msg", "save":
	default:
		return errors.New("unknown command")
	}
	commandSeq++
	line := fmt.Sprintf("%d|%s|%s\r\n", commandSeq, cmd, strings.NewReplacer("|", " ", "\n", " ", "\r", " ").Replace(arg))
	_ = os.MkdirAll(a.paths.bridgeDir(), 0755)
	f, err := os.OpenFile(a.paths.bridge("app_commands.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}
