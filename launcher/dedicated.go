package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// The launcher runs the game's own co-op server (zombie.network.GameServer -coop) itself, the same
// way PZ's Host button does (CoopMaster). The server talks to its parent over stdin/stdout in
// "tag[cookie]@payload" lines: it sends status@..., steam-id@..., ping@ping; we answer ping@pong.

// ServerState is what the UI shows about the host's server.
type ServerState struct {
	Running bool   `json:"running"`
	Stage   string `json:"stage"`   // starting, loading, up, stopped
	Text    string `json:"text"`    // last status, in plain words
	SteamID string `json:"steamId"` // the server's own Steam ID: games join this
	Started int64  `json:"started"`
	Up      int64  `json:"up"` // when it finished loading
	Error   string `json:"error,omitempty"`
	World   string `json:"world"`
}

type coopServer struct {
	mu    sync.Mutex
	state ServerState
	cmd   *exec.Cmd
	in    io.WriteCloser
	// portFail: the last start died because the network port wouldn't open (try another port)
	portFail bool
	port     int
	// logOff, logHead: size and first bytes of coop-console.txt before this run, so an appended
	// old run is told apart from this one
	logOff  int64
	logHead []byte
}

func (s *coopServer) get() ServerState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

func (s *coopServer) set(f func(st *ServerState)) {
	s.mu.Lock()
	f(&s.state)
	s.mu.Unlock()
}

func (s *coopServer) send(tag, payload string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.in != nil {
		_, _ = io.WriteString(s.in, tag+"@"+payload+"\n")
	}
}

// Stop asks the server to save and quit (it treats "process-status@eof" as losing its host),
// then kills it if it hasn't gone in a minute.
func (s *coopServer) Stop() {
	s.mu.Lock()
	cmd := s.cmd
	s.mu.Unlock()
	if cmd == nil {
		return
	}
	s.send("process-status", "eof")
	go func() {
		time.Sleep(60 * time.Second)
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.cmd == cmd && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()
}

var serverStatusText = map[string]string{
	"UI_ServerStatus_Initialising":               "Starting",
	"UI_ServerStatus_Downloading_Workshop_Items": "Checking Workshop mods",
	"UI_ServerStatus_LoadingWorld":               "Loading the world",
	"UI_ServerStatus_Loading_Assets":             "Loading game files",
	"UI_ServerStatus_Initing_Checksum":           "Checking files",
	"UI_ServerStatus_Loading_Sandbox_Vars":       "Loading world settings",
	"UI_ServerStatus_Started":                    "Server is up",
	"UI_ServerStatus_Terminated":                 "Server stopped",
}

func plainStatus(payload string) string {
	if t, ok := serverStatusText[payload]; ok {
		return t
	}
	if strings.HasPrefix(payload, "UI_ServerStatus_") {
		return strings.ReplaceAll(strings.TrimPrefix(payload, "UI_ServerStatus_"), "_", " ")
	}
	return payload
}

// StartServer launches the co-op server for the host's world. Config files must already be written.
func (a *App) StartServer(world string, memory int) error {
	srv := a.server
	if srv.get().Running {
		return errors.New("the server is already running")
	}
	cmd, err := a.serverCommand(world, memory)
	if err != nil {
		return err
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var logOff int64
	var logHead []byte
	if b, err := os.ReadFile(filepath.Join(a.paths.Zomboid, "coop-console.txt")); err == nil {
		logOff, logHead = int64(len(b)), b[:min(len(b), 512)]
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("couldn't start the server: %v", err)
	}
	tieToLauncher(cmd.Process, filepath.Join(a.paths.Data, "server.pid"))
	log.Printf("server starting: %s", world)
	srv.mu.Lock()
	srv.cmd, srv.in = cmd, in
	srv.logOff, srv.logHead = logOff, logHead
	srv.state = ServerState{Running: true, Stage: "starting", Text: "Starting", Started: time.Now().Unix(), World: world}
	srv.mu.Unlock()
	go a.readServer(cmd, out)
	go a.watchServerLog(cmd)
	return nil
}

func (a *App) readServer(cmd *exec.Cmd, out io.Reader) {
	srv := a.server
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, "Connection Startup Failed") {
			// RakNet couldn't open its port; the server hangs after this instead of quitting
			log.Println("server: network start failed:", line)
			srv.mu.Lock()
			srv.portFail = true
			port := srv.port
			srv.mu.Unlock()
			markBadPort(port)
			srv.set(func(st *ServerState) {
				st.Error = "The server couldn't open its network port. Another server or game may still be using it. Close it, or restart your PC, then press START again."
			})
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			continue
		}
		at := strings.Index(line, "@")
		if at < 0 {
			continue
		}
		tag, payload := line[:at], line[at+1:]
		if i := strings.Index(tag, "["); i >= 0 {
			tag = tag[:i]
		}
		switch tag {
		case "ping":
			srv.send("ping", "pong")
		case "steam-id":
			srv.set(func(st *ServerState) { st.SteamID = payload })
		case "status":
			text := plainStatus(payload)
			srv.set(func(st *ServerState) {
				st.Text = text
				switch payload {
				case "UI_ServerStatus_Started":
					st.Stage, st.Up = "up", time.Now().Unix()
				case "UI_ServerStatus_Terminated":
					st.Stage = "stopped"
				default:
					if st.Stage != "up" {
						st.Stage = "loading"
					}
				}
			})
			if payload == "UI_ServerStatus_Started" {
				// tell the server who the host is, as the game's Host screen does
				if a.steamOK() {
					srv.send("set-host-steamid", strconv.FormatUint(a.steam.MyID(), 10))
					go a.inviteFriends(cmd)
				}
			}
			if id := a.lobby.ID(); id != 0 && a.lobby.IsHost() {
				a.steam.SetLobbyData(id, "srv", text)
			}
			log.Println("server:", text)
		}
	}
	err := cmd.Wait()
	code := -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	log.Printf("server exited, code %d (%v)", code, err)
	if id := a.lobby.ID(); id != 0 && a.lobby.IsHost() {
		a.steam.SetLobbyData(id, "srv", "")
	}
	srv.mu.Lock()
	if srv.cmd == cmd {
		srv.cmd, srv.in = nil, nil
		srv.state.Running = false
		if srv.state.Stage != "up" && srv.state.Stage != "stopped" {
			if srv.state.Error == "" {
				srv.state.Error = "The server closed while starting. Its log is Zomboid\\coop-console.txt."
			}
		}
		srv.state.Stage, srv.state.Text = "stopped", "Server stopped"
	}
	srv.mu.Unlock()
}

// serverCommand builds the same java command line CoopMaster.launchServer uses.
func (a *App) serverCommand(world string, memory int) (*exec.Cmd, error) {
	if runtime.GOOS != "windows" {
		return devServerCommand(), nil
	}
	game := a.paths.Game
	java := filepath.Join(game, "jre64", "bin", "java.exe")
	if !exists(java) {
		return nil, errors.New("the game's Java (jre64) wasn't found in " + game)
	}
	classPath, libPath := ".;projectzomboid.jar", "win64/;."
	var launcher struct {
		Classpath []string `json:"classpath"`
		VMArgs    []string `json:"vmArgs"`
	}
	if b, err := os.ReadFile(filepath.Join(game, "ProjectZomboid64.json")); err == nil && json.Unmarshal(b, &launcher) == nil {
		if len(launcher.Classpath) > 0 {
			classPath = strings.Join(launcher.Classpath, ";")
		}
		for _, v := range launcher.VMArgs {
			if strings.HasPrefix(v, "-Djava.library.path=") {
				libPath = strings.TrimPrefix(v, "-Djava.library.path=")
			}
		}
	}
	home, _ := os.UserHomeDir()
	memory = max(memory, 2048)
	pw := make([]byte, 16)
	_, _ = rand.Read(pw)
	args := []string{
		"-Xms" + strconv.Itoa(memory) + "m",
		"-Xmx" + strconv.Itoa(memory) + "m",
		"-Djava.library.path=" + libPath,
		"-Djava.class.path=" + classPath,
		"-Duser.dir=" + game,
		"-Duser.home=" + home,
		"-Dzomboid.steam=1",
		"-Djava.awt.headless=true",
		"-XX:-OmitStackTraceInFastThrow",
		// garbage collection pauses, to tell a server freeze from a PC freeze (Logs\server-gc.txt)
		// (quoted, since the drive letter's colon would end the file name otherwise)
		"-Xlog:gc:file=\"" + filepath.Join(a.paths.Zomboid, "Logs", "server-gc.txt") + "\":time,uptime:filecount=2,filesize=5m",
	}
	// Knox Link's helper: runs ZombieBuddy (server-side Java mods) and reuses the host's Workshop folders
	if agent, zb := a.knoxAgentJar(), filepath.Join(game, "ZombieBuddy.jar"); agent != "" && exists(zb) {
		if err := a.writeServerManifest(); err != nil {
			log.Println("server manifest:", err)
		}
		args = append(args, "-javaagent:"+agent+"=zb="+zb+"|cache="+a.paths.Zomboid)
	}
	args = append(args, "zombie.network.GameServer", "-coop",
		"-servername", world,
		"-adminusername", "admin",
		"-adminpassword", hex.EncodeToString(pw),
		"-cachedir="+a.paths.Zomboid)
	cmd := hidden(exec.Command(java, args...))
	cmd.Dir = game
	cmd.Env = append(os.Environ(), "SteamAppId=108600", "SteamGameId=108600")
	return cmd, nil
}

// knoxAgentJar finds Knox Link's KnoxLinkAgent.jar (local mod folder first, then the Workshop copy).
func (a *App) knoxAgentJar() string {
	cands := []string{filepath.Join(a.paths.Zomboid, "mods", "KnoxLink", "42", "media", "java", "agent", "KnoxLinkAgent.jar")}
	if c := a.catalog(); c != nil {
		if m := c.mod(knoxLinkID); m != nil {
			for _, r := range m.roots {
				cands = append(cands, filepath.Join(r, "media", "java", "agent", "KnoxLinkAgent.jar"))
			}
		}
	}
	for _, p := range cands {
		if exists(p) {
			return p
		}
	}
	return ""
}

// writeServerManifest lists every installed Workshop item as "<id>|<timestamp>|<folder>" for the agent.
func (a *App) writeServerManifest() error {
	entries, err := os.ReadDir(a.paths.Workshop)
	if err != nil {
		return err
	}
	var lines []string
	for _, e := range entries {
		id, err := strconv.ParseUint(e.Name(), 10, 64)
		if err != nil || !e.IsDir() {
			continue
		}
		folder, ts, ok := a.steam.ItemInstallInfo(id)
		if !ok || folder == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%d|%d|%s", id, ts, folder))
	}
	return a.writeBridge("server_manifest.txt", strings.Join(lines, "\r\n")+"\r\n")
}

// writeServerConfig writes <world>.ini, _SandboxVars.lua and _spawnregions.lua. An existing world
// keeps its files with our values patched in; a new one gets only our values and the server fills
// in the game's defaults on first start.
func (a *App) writeServerConfig(h HostSettings, mods []string, m Manifest, spawn string) error {
	dir := a.paths.serverDir()
	_ = os.MkdirAll(dir, 0755)
	c := a.catalog()

	ini := map[string]string{}
	for k, v := range h.Ini {
		ini[k] = v
	}
	var items []string
	for _, it := range m.Items {
		items = append(items, it.ID)
	}
	ini["Mods"] = strings.Join(mods, ";")
	ini["WorkshopItems"] = strings.Join(items, ";")
	ini["Password"] = ""
	// friends join through Steam, so any free port works; 16261 can still be held by another server
	game, udp := freeServerPorts()
	ini["DefaultPort"], ini["UDPPort"] = strconv.Itoa(game), strconv.Itoa(udp)
	a.server.mu.Lock()
	a.server.port, a.server.portFail = game, false
	a.server.mu.Unlock()
	// map mods first, then the base game
	var maps []string
	seen := map[string]bool{}
	var regions []string
	for _, id := range mods {
		info := c.mod(id)
		if info == nil {
			continue
		}
		for _, r := range info.roots {
			dirs, _ := os.ReadDir(filepath.Join(r, "media", "maps"))
			for _, d := range dirs {
				// a real map has map.info; folders with only spawn points or overrides don't go in Map=
				if !d.IsDir() || seen[d.Name()] || !exists(filepath.Join(r, "media", "maps", d.Name(), "map.info")) {
					continue
				}
				seen[d.Name()] = true
				maps = append(maps, d.Name())
				if exists(filepath.Join(r, "media", "maps", d.Name(), "spawnpoints.lua")) {
					regions = append(regions, d.Name())
				}
			}
		}
	}
	if !seen["Muldraugh, KY"] {
		maps = append(maps, "Muldraugh, KY")
	}
	ini["Map"] = strings.Join(maps, ";")
	if spawn != "" && spawn != "0,0,0" {
		ini["SpawnPoint"] = spawn
	} else {
		ini["SpawnPoint"] = "0,0,0"
	}
	iniFile := filepath.Join(dir, h.World+".ini")
	isNew := !exists(iniFile)
	if isNew {
		ini["PublicName"] = "Knox Launcher"
		ini["Public"] = "false"
	}
	if err := patchIni(iniFile, ini); err != nil {
		return fmt.Errorf("couldn't write the server settings: %v", err)
	}

	sb := readSandboxFile(filepath.Join(dir, h.World+"_SandboxVars.lua"))
	for k, v := range h.Sandbox {
		sb[k] = v
	}
	if err := os.WriteFile(filepath.Join(dir, h.World+"_SandboxVars.lua"), []byte(sandboxLua(sb)), 0644); err != nil {
		return fmt.Errorf("couldn't write the world settings: %v", err)
	}

	regFile := filepath.Join(dir, h.World+"_spawnregions.lua")
	if isNew || !exists(regFile) {
		var b strings.Builder
		b.WriteString("function SpawnRegions()\n\treturn {\n")
		for _, name := range vanillaRegions {
			fmt.Fprintf(&b, "\t\t{ name = %q, file = %q },\n", name, "media/maps/"+name+"/spawnpoints.lua")
		}
		for _, name := range regions {
			fmt.Fprintf(&b, "\t\t{ name = %q, file = %q },\n", name, "media/maps/"+name+"/spawnpoints.lua")
		}
		b.WriteString("\t}\nend\n")
		if err := os.WriteFile(regFile, []byte(b.String()), 0644); err != nil {
			return err
		}
	}
	return nil
}

// patchIni replaces values in place (keeping the game's comments) and appends missing keys.
func patchIni(file string, vals map[string]string) error {
	var lines []string
	if b, err := os.ReadFile(file); err == nil {
		lines = strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	}
	done := map[string]bool{}
	for i, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if eq := strings.Index(line, "="); eq > 0 {
			if v, ok := vals[line[:eq]]; ok {
				lines[i] = line[:eq] + "=" + v
				done[line[:eq]] = true
			}
		}
	}
	var keys []string
	for k := range vals {
		if !done[k] {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, k := range keys {
		lines = append(lines, k+"="+vals[k])
	}
	return os.WriteFile(file, []byte(strings.Join(lines, "\r\n")+"\r\n"), 0644)
}

// sandboxLua turns "ZombieLore.Speed" -> "2" back into a SandboxVars table.
func sandboxLua(vals map[string]string) string {
	type node struct {
		kids  map[string]*node
		order []string
		value string
		leaf  bool
	}
	root := &node{kids: map[string]*node{}}
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		n := root
		parts := strings.Split(k, ".")
		for i, p := range parts {
			child := n.kids[p]
			if child == nil {
				child = &node{kids: map[string]*node{}}
				n.kids[p] = child
				n.order = append(n.order, p)
			}
			if i == len(parts)-1 {
				child.leaf, child.value = true, vals[k]
			}
			n = child
		}
	}
	var b strings.Builder
	var write func(n *node, depth int)
	write = func(n *node, depth int) {
		pad := strings.Repeat("    ", depth)
		for _, name := range n.order {
			k := n.kids[name]
			if k.leaf {
				fmt.Fprintf(&b, "%s%s = %s,\n", pad, name, luaValue(k.value))
			} else {
				fmt.Fprintf(&b, "%s%s = {\n", pad, name)
				write(k, depth+1)
				fmt.Fprintf(&b, "%s},\n", pad)
			}
		}
	}
	b.WriteString("SandboxVars = {\n    VERSION = 6,\n")
	write(root, 1)
	b.WriteString("}\n")
	return b.String()
}

func luaValue(v string) string {
	if v == "true" || v == "false" {
		return v
	}
	if _, err := strconv.ParseFloat(v, 64); err == nil {
		return v
	}
	return strconv.Quote(v)
}

// badPorts are ports a server failed to open this session (RakNet said "in use" though a test bind worked).
var badPorts = struct {
	sync.Mutex
	m map[int]bool
}{m: map[int]bool{}}

func markBadPort(p int) {
	if p == 0 {
		return
	}
	badPorts.Lock()
	badPorts.m[p], badPorts.m[p+1] = true, true
	badPorts.Unlock()
}

// freeServerPorts finds a free pair of UDP ports for the server, starting at the game's 16261/16262.
func freeServerPorts() (int, int) {
	free := func(p int) bool {
		badPorts.Lock()
		bad := badPorts.m[p]
		badPorts.Unlock()
		if bad {
			return false
		}
		c, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(p))
		if err != nil {
			return false
		}
		_ = c.Close()
		return true
	}
	for p := 16261; p < 16461; p += 2 {
		if free(p) && free(p+1) {
			if p != 16261 {
				log.Printf("server: port 16261 is busy, using %d", p)
			}
			return p, p + 1
		}
	}
	return 16261, 16262
}

// watchServerLog reads Zomboid\coop-console.txt while the server starts. A network start failure only
// shows up there (not on stdout), and the server then hangs on "Loading the world" for good.
func (a *App) watchServerLog(cmd *exec.Cmd) {
	file := filepath.Join(a.paths.Zomboid, "coop-console.txt")
	started := time.Now()
	var off int64
	if fi, err := os.Stat(file); err == nil && fi.ModTime().Before(started) {
		off = fi.Size() // old log from an earlier run (the server may append or start over)
	}
	for {
		time.Sleep(2 * time.Second)
		st := a.server.get()
		a.server.mu.Lock()
		same := a.server.cmd == cmd
		a.server.mu.Unlock()
		if !same || !st.Running || st.Stage == "up" {
			return
		}
		f, err := os.Open(file)
		if err != nil {
			continue
		}
		fi, _ := f.Stat()
		if fi != nil && fi.Size() < off {
			off = 0 // the server started a new log
		}
		_, _ = f.Seek(off, io.SeekStart)
		b, _ := io.ReadAll(f)
		f.Close()
		off += int64(len(b))
		if !strings.Contains(string(b), "Connection Startup Failed") {
			continue
		}
		a.server.mu.Lock()
		port := a.server.port
		a.server.portFail = true
		a.server.mu.Unlock()
		markBadPort(port)
		log.Printf("server: couldn't open network port %d, stopping it", port)
		a.server.set(func(st *ServerState) {
			st.Error = "The server couldn't open its network port. Another program may be using it. Press START to try again on a different port."
		})
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return
	}
}

// inviteFriends lets your Steam friends and everyone in the lobby into the server. A co-op server
// only takes players the host invited ("The host must allow you to join"), and the game's invite
// menu is gone because the launcher starts the server. Late lobby joiners are added as they come.
func (a *App) inviteFriends(cmd *exec.Cmd) {
	srv := a.server
	invited := map[uint64]bool{}
	for {
		srv.mu.Lock()
		same := srv.cmd == cmd
		srv.mu.Unlock()
		if !same {
			return
		}
		var ids []uint64
		for _, f := range a.steam.Friends() {
			ids = append(ids, f.ID)
		}
		for _, m := range a.lobby.Members() {
			ids = append(ids, m.ID)
		}
		n := 0
		for _, id := range ids {
			if id != 0 && !invited[id] {
				invited[id] = true
				srv.send("invite-add", strconv.FormatUint(id, 10))
				n++
			}
		}
		if n > 0 {
			log.Printf("server: invited %d player(s)", n)
		}
		time.Sleep(5 * time.Second)
	}
}
