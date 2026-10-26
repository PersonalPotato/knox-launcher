package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Phases of the app
const (
	PhaseIdle      = "idle"      // no lobby
	PhaseCreating  = "creating"  // waiting for Steam to make the lobby
	PhaseJoining   = "joining"   // waiting to enter a friend's lobby
	PhaseLobby     = "lobby"     // in a lobby, setting things up
	PhaseSyncing   = "syncing"   // getting mods ready before the game opens
	PhaseLaunching = "launching" // about to open the game
	PhaseInGame    = "ingame"    // Project Zomboid is open; the launcher shows the live game
)

type ChatLine struct {
	From uint64 `json:"from,string"`
	Name string `json:"name"`
	Text string `json:"text"`
	Time int64  `json:"time"`
	Sys  bool   `json:"sys,omitempty"`
}

type Member struct {
	ID     uint64          `json:"id,string"`
	Name   string          `json:"name"`
	Host   bool            `json:"host"`
	Me     bool            `json:"me"`
	Ready  bool            `json:"ready"`
	Char   json.RawMessage `json:"char,omitempty"`
	Sync   string          `json:"sync"`
	AppVer string          `json:"appVer"`
	Role   string          `json:"role,omitempty"`
	InGame bool            `json:"inGame"`
	Pack   json.RawMessage `json:"pack,omitempty"` // a mod pack this member shared
	Knox   json.RawMessage `json:"knox,omitempty"` // level, title, badges
}

type SyncState struct {
	Total    int      `json:"total"`
	Done     int      `json:"done"`
	Current  string   `json:"current"`
	Bytes    uint64   `json:"bytes"`
	BytesAll uint64   `json:"bytesAll"`
	Error    string   `json:"error,omitempty"`
	Wait     string   `json:"wait,omitempty"` // mods done, waiting on others (friends' downloads, the host's server)
	Warnings []string `json:"warnings,omitempty"`
}

type Lobby struct {
	app     *App
	mu      sync.Mutex
	phase   string
	id      uint64
	isHost  bool
	chat    []ChatLine
	sync    SyncState
	note    string            // last problem shown to the user
	handled string            // launch id we already reacted to
	cancel  bool              // the user pressed Cancel while waiting to start
	mine    map[string]string // member data we last sent, so unchanged values aren't resent
}

// restPhase is where the app goes back to after a start fails or the game closes.
// Call it with l.mu held (from inside l.set).
func (l *Lobby) restPhase() string {
	if l.id != 0 {
		return PhaseLobby
	}
	return PhaseIdle
}

// setMyData shares one value about you with everyone in the lobby.
func (l *Lobby) setMyData(key, value string) {
	l.mu.Lock()
	id := l.id
	if l.mine == nil {
		l.mine = map[string]string{}
	}
	old, seen := l.mine[key]
	if id != 0 {
		l.mine[key] = value
	}
	l.mu.Unlock()
	if id == 0 || (seen && old == value) {
		return
	}
	l.app.steam.SetMemberData(id, key, value)
}

// roleClaims maps each backstory role to the member who took it (first come, first served).
func (l *Lobby) roleClaims() map[string]string {
	out := map[string]string{}
	for _, m := range l.Members() {
		if m.Role != "" {
			if _, taken := out[m.Role]; !taken {
				out[m.Role] = strconv.FormatUint(m.ID, 10)
			}
		}
	}
	return out
}

func (l *Lobby) set(f func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f()
}

func (l *Lobby) addChat(c ChatLine) {
	l.set(func() {
		c.Time = time.Now().Unix()
		l.chat = append(l.chat, c)
		if len(l.chat) > 200 {
			l.chat = l.chat[len(l.chat)-200:]
		}
	})
}

func (l *Lobby) system(text string) {
	l.addChat(ChatLine{Text: text, Sys: true})
}

// Create a friends-only lobby; friends see it in their Knox Launcher.
func (l *Lobby) Create() error {
	if err := l.app.steamReady(); err != nil {
		return err
	}
	l.set(func() { l.phase, l.isHost, l.chat, l.note, l.mine = PhaseCreating, true, nil, "", nil })
	l.app.steam.CreateLobby(16)
	return nil
}

func (l *Lobby) Join(id uint64) error {
	if err := l.app.steamReady(); err != nil {
		return err
	}
	l.leaveQuiet()
	l.set(func() { l.phase, l.isHost, l.chat, l.id, l.note, l.mine = PhaseJoining, false, nil, id, "", nil })
	l.app.steam.JoinLobby(id)
	return nil
}

func (l *Lobby) leaveQuiet() {
	l.mu.Lock()
	id := l.id
	l.id, l.isHost, l.mine = 0, false, nil
	if l.phase != PhaseInGame {
		l.phase = PhaseIdle
	}
	l.mu.Unlock()
	if id != 0 {
		l.app.steam.LeaveLobby(id)
		l.app.steam.SetRichPresence("status", "")
	}
}

func (l *Lobby) Leave() { l.leaveQuiet() }

func (l *Lobby) ID() uint64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.id
}

func (l *Lobby) Phase() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.phase
}

func (l *Lobby) IsHost() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.isHost
}

func (l *Lobby) onEvent(e SteamEvent) {
	st := l.app.steam
	switch e.Kind {
	case EvLobbyCreated:
		if !e.OK {
			l.set(func() { l.phase, l.note = PhaseIdle, fmt.Sprintf("Steam couldn't make a lobby (code %d)", e.Result) })
			return
		}
		l.set(func() { l.id = e.Lobby })
		st.SetLobbyData(e.Lobby, "ez", "1")
		st.SetLobbyData(e.Lobby, "ver", appVersion)
		st.SetLobbyData(e.Lobby, "state", "open")
		st.SetLobbyData(e.Lobby, "host", st.MyName())
		l.app.publishHostInfo()
	case EvLobbyEntered:
		if !e.OK {
			l.set(func() { l.phase, l.id, l.note = PhaseIdle, 0, "Couldn't join that lobby (it may be full or closed)" })
			return
		}
		l.set(func() {
			l.id = e.Lobby
			if l.phase != PhaseInGame {
				l.phase = PhaseLobby
			}
		})
		host := st.LobbyOwner(e.Lobby) == st.MyID()
		l.set(func() { l.isHost = host })
		l.app.publishMyData()
		st.SetRichPresence("status", "In a Knox Launcher lobby")
		if l.IsHost() {
			l.system("Lobby open.")
		} else {
			l.system("Joined " + st.LobbyData(e.Lobby, "host") + "'s lobby.")
		}
	case EvLobbyChat:
		if e.Lobby != l.ID() {
			return
		}
		switch {
		case strings.HasPrefix(e.Text, "c:"):
			l.addChat(ChatLine{From: e.User, Name: st.PersonaName(e.User), Text: strings.TrimPrefix(e.Text, "c:")})
		case strings.HasPrefix(e.Text, "s:"):
			// a status line from someone's launcher (start problems, game opened)
			l.system(st.PersonaName(e.User) + ": " + strings.TrimPrefix(e.Text, "s:"))
		case strings.HasPrefix(e.Text, "mf:"):
			l.app.gotManifestPart(e.Text)
		case strings.HasPrefix(e.Text, "pk:"):
			// someone added a mod pack a lobby member shared (Knox level XP for its owner)
			l.app.notePackAdd(e.User, strings.TrimPrefix(e.Text, "pk:"))
		case strings.HasPrefix(e.Text, "go:"):
			// the host pressed START; lobby data can arrive late, so the chat line is a second signal
			l.checkLaunch()
		}
	case EvLobbyChanged:
		if e.Lobby == l.ID() {
			l.checkLaunch()
		}
	case EvJoinRequested:
		_ = l.Join(e.Lobby)
	case EvWorkshopChanged:
		l.app.workshopChanged()
	}
}

func (l *Lobby) Say(text string) {
	text = strings.TrimSpace(text)
	if text == "" || l.ID() == 0 {
		return
	}
	if len(text) > 500 {
		text = text[:500]
	}
	l.app.steam.SendLobbyMessage(l.ID(), "c:"+text)
}

// notReady says why the host can't start yet: someone isn't ready or runs another launcher version.
func (l *Lobby) notReady() string {
	var waiting, old []string
	for _, m := range l.Members() {
		if !m.Ready {
			if m.Me {
				waiting = append(waiting, "you")
			} else {
				waiting = append(waiting, m.Name)
			}
		}
		if !m.Me && m.AppVer != appVersion {
			v := m.AppVer
			if v == "" {
				v = "old"
			}
			old = append(old, m.Name+" ("+v+")")
		}
	}
	if len(old) > 0 {
		return "Different launcher version: " + strings.Join(old, ", ") + ". Everyone needs launcher " + appVersion + "."
	}
	if len(waiting) > 0 {
		return "Not ready: " + strings.Join(waiting, ", ") + "."
	}
	return ""
}

// Cancel stops a start that is still waiting on mods or the server.
func (l *Lobby) Cancel() {
	l.set(func() { l.cancel = true })
}

func (l *Lobby) takeCancel() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	c := l.cancel
	l.cancel = false
	return c
}

// announce tells everyone in the lobby something about your launcher.
func (l *Lobby) announce(text string) {
	if id := l.ID(); id != 0 {
		l.app.steam.SendLobbyMessage(id, "s:"+text)
	}
}

func (l *Lobby) Members() []Member {
	id := l.ID()
	if id == 0 {
		return nil
	}
	st := l.app.steam
	owner := st.LobbyOwner(id)
	me := st.MyID()
	var out []Member
	for _, m := range st.LobbyMembers(id) {
		mem := Member{ID: m, Name: st.PersonaName(m), Host: m == owner, Me: m == me}
		mem.Ready = st.MemberData(id, m, "ready") == "1"
		if c := st.MemberData(id, m, "char"); c != "" && json.Valid([]byte(c)) {
			mem.Char = json.RawMessage(c)
		}
		mem.Sync = st.MemberData(id, m, "sync")
		mem.AppVer = st.MemberData(id, m, "app")
		mem.Role = st.MemberData(id, m, "role")
		mem.InGame = st.MemberData(id, m, "game") == "1"
		if p := st.MemberData(id, m, "pack"); p != "" && json.Valid([]byte(p)) {
			mem.Pack = json.RawMessage(p)
		}
		if k := st.MemberData(id, m, "knox"); k != "" && json.Valid([]byte(k)) {
			mem.Knox = json.RawMessage(k)
		}
		out = append(out, mem)
	}
	return out
}

func (l *Lobby) SetReady(ready bool) {
	if id := l.ID(); id != 0 {
		v := "0"
		if ready {
			v = "1"
		}
		l.setMyData("ready", v)
	}
}

// checkLaunch reacts to the host starting the game.
func (l *Lobby) checkLaunch() {
	id := l.ID()
	if id == 0 || l.IsHost() {
		return
	}
	st := l.app.steam
	if st.LobbyData(id, "state") != "launching" {
		return
	}
	launchID := st.LobbyData(id, "launch")
	l.mu.Lock()
	if l.handled == launchID || l.phase == PhaseSyncing || l.phase == PhaseLaunching || l.phase == PhaseInGame {
		l.mu.Unlock()
		return
	}
	l.handled = launchID
	l.phase = PhaseSyncing
	l.cancel = false
	l.mu.Unlock()
	go l.app.guestLaunch(id)
}

// Poll loop: pumps Steam callbacks.
func (l *Lobby) run() {
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	slow := 0
	for range tick.C {
		if l.app.steamOK() {
			l.app.steam.RunCallbacks()
		}
		for {
			select {
			case e := <-l.app.steam.Events():
				l.onEvent(e)
				continue
			default:
			}
			break
		}
		slow++
		if slow%40 == 0 && l.ID() != 0 {
			l.checkLaunch()
		}
	}
}

func parseLobbyCode(code string) (uint64, error) {
	code = strings.TrimSpace(strings.ToLower(code))
	code = strings.TrimPrefix(strings.TrimPrefix(code, "ez-"), "knox-")
	if code == "" {
		return 0, fmt.Errorf("empty code")
	}
	if v, err := strconv.ParseUint(code, 36, 64); err == nil && v > 1<<52 {
		return v, nil
	}
	return strconv.ParseUint(code, 10, 64)
}

func lobbyCode(id uint64) string {
	if id == 0 {
		return ""
	}
	return "KNOX-" + strings.ToUpper(strconv.FormatUint(id, 36))
}

func logf(format string, args ...any) { log.Printf(format, args...) }
