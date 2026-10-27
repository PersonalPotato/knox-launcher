//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// realSteam calls the flat C API of steam_api64.dll (the copy shipped with Project Zomboid).
// Every call goes through one mutex; callbacks are pumped with the manual-dispatch API.
type realSteam struct {
	dllPath string
	mu      sync.Mutex
	dll     *syscall.DLL
	procs   map[string]*syscall.Proc
	pipe    uintptr
	user    uintptr
	friend  uintptr
	mm      uintptr
	ugc     uintptr
	utils   uintptr
	events  chan SteamEvent
	calls   map[uint64]int // async call handle -> expected callback id
	waiters map[uint64]chan []byte // async call handle -> whoever waits for its result (nil on failure)
	avatar  map[uint64][]byte
}

func newSteam(dllPath string) Steam {
	return &realSteam{dllPath: dllPath, events: make(chan SteamEvent, 1024), calls: map[uint64]int{}, waiters: map[uint64]chan []byte{}, avatar: map[uint64][]byte{}}
}

func (s *realSteam) proc(name string) *syscall.Proc {
	if p, ok := s.procs[name]; ok {
		return p
	}
	p, err := s.dll.FindProc(name)
	if err != nil {
		panic("steam_api64.dll has no " + name)
	}
	s.procs[name] = p
	return p
}

// uintptrescapes keeps every pointer turned into a uintptr in a call's arguments (a key string, a local
// buffer) alive and in place until the DLL returns. Without it the garbage collector or a stack move
// can free or shift that memory mid-call.
//
//go:uintptrescapes
func (s *realSteam) call(name string, args ...uintptr) uintptr {
	r, _, _ := s.proc(name).Call(args...)
	return r
}

// cstr makes a C string. Pass it as uintptr(cstr(v)) right in s.call's arguments so it stays alive.
func cstr(v string) unsafe.Pointer {
	b, _ := syscall.BytePtrFromString(v)
	return unsafe.Pointer(b)
}

func gostr(p uintptr) string {
	if p == 0 {
		return ""
	}
	var buf []byte
	for i := 0; i < 1<<20; i++ {
		c := *(*byte)(unsafe.Pointer(p + uintptr(i)))
		if c == 0 {
			break
		}
		buf = append(buf, c)
	}
	return string(buf)
}

func truthy(r uintptr) bool { return r&0xff != 0 }

func (s *realSteam) Init() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dll != nil {
		return nil
	}
	// steam_api reads the app id from steam_appid.txt in the working directory
	dir, _ := os.Getwd()
	_ = os.WriteFile(filepath.Join(dir, "steam_appid.txt"), []byte(fmt.Sprint(PZAppID)), 0644)
	dll, err := syscall.LoadDLL(s.dllPath)
	if err != nil {
		return fmt.Errorf("couldn't load Steam (%s): %v", s.dllPath, err)
	}
	s.dll = dll
	s.procs = map[string]*syscall.Proc{}
	var msg [1024]byte
	r := s.call("SteamAPI_InitFlat", uintptr(unsafe.Pointer(&msg[0])))
	if r != 0 {
		text := string(bytes.TrimRight(msg[:], "\x00"))
		if text == "" {
			text = "is Steam running?"
		}
		s.dll = nil
		return errors.New("Steam didn't start: " + text)
	}
	s.call("SteamAPI_ManualDispatch_Init")
	s.pipe = s.call("SteamAPI_GetHSteamPipe")
	s.user = s.call("SteamAPI_SteamUser_v023")
	s.friend = s.call("SteamAPI_SteamFriends_v018")
	s.mm = s.call("SteamAPI_SteamMatchmaking_v009")
	s.ugc = s.call("SteamAPI_SteamUGC_v021")
	s.utils = s.call("SteamAPI_SteamUtils_v010")
	return nil
}

func (s *realSteam) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dll == nil {
		return
	}
	s.call("SteamAPI_Shutdown")
	s.dll = nil
}

func (s *realSteam) Events() <-chan SteamEvent { return s.events }

func (s *realSteam) emit(e SteamEvent) {
	select {
	case s.events <- e:
	default:
	}
}

// Callback ids
const (
	cbGameLobbyJoinRequested = 333
	cbLobbyEnter             = 504
	cbLobbyDataUpdate        = 505
	cbLobbyChatUpdate        = 506
	cbLobbyChatMsg           = 507
	cbLobbyCreated           = 513
	cbFileSubscribed         = 1321 // RemoteStoragePublishedFileSubscribed_t
	cbFileUnsubscribed       = 1322
	cbItemInstalled          = 3405
	cbDownloadItemResult     = 3406
	cbAPICallCompleted       = 703
)

func le64(b []byte, off int) uint64 { return binary.LittleEndian.Uint64(b[off:]) }
func le32(b []byte, off int) uint32 { return binary.LittleEndian.Uint32(b[off:]) }

func (s *realSteam) RunCallbacks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dll == nil {
		return
	}
	s.call("SteamAPI_ManualDispatch_RunFrame", s.pipe)
	var msg [24]byte // CallbackMsg_t: int32 user, int32 id, ptr param, int32 size
	for truthy(s.call("SteamAPI_ManualDispatch_GetNextCallback", s.pipe, uintptr(unsafe.Pointer(&msg[0])))) {
		id := int(int32(le32(msg[:], 4)))
		ptr := uintptr(le64(msg[:], 8))
		size := int(int32(le32(msg[:], 16)))
		param := make([]byte, size)
		for i := 0; i < size; i++ {
			param[i] = *(*byte)(unsafe.Pointer(ptr + uintptr(i)))
		}
		if id == cbAPICallCompleted && size >= 16 {
			handle := le64(param, 0)
			want := int(int32(le32(param, 8)))
			resSize := int(le32(param, 12))
			res := make([]byte, resSize+8)
			var failed byte
			ok := truthy(s.call("SteamAPI_ManualDispatch_GetAPICallResult", s.pipe, uintptr(handle),
				uintptr(unsafe.Pointer(&res[0])), uintptr(resSize), uintptr(want), uintptr(unsafe.Pointer(&failed))))
			if ch, waiting := s.waiters[handle]; waiting {
				delete(s.waiters, handle)
				if ok && failed == 0 {
					ch <- res[:resSize]
				} else {
					ch <- nil
				}
			} else if _, mine := s.calls[handle]; mine {
				delete(s.calls, handle)
				if ok && failed == 0 {
					s.handle(want, res[:resSize])
				} else if want == cbLobbyCreated {
					s.emit(SteamEvent{Kind: EvLobbyCreated, OK: false})
				} else if want == cbLobbyEnter {
					s.emit(SteamEvent{Kind: EvLobbyEntered, OK: false})
				}
			}
		} else {
			s.handle(id, param)
		}
		s.call("SteamAPI_ManualDispatch_FreeLastCallback", s.pipe)
	}
}

func (s *realSteam) handle(id int, p []byte) {
	switch id {
	case cbLobbyCreated:
		if len(p) >= 16 {
			res := int(int32(le32(p, 0)))
			s.emit(SteamEvent{Kind: EvLobbyCreated, OK: res == 1, Lobby: le64(p, 8), Result: res})
		}
	case cbLobbyEnter:
		if len(p) >= 20 {
			resp := int(le32(p, 16))
			s.emit(SteamEvent{Kind: EvLobbyEntered, Lobby: le64(p, 0), OK: resp == 1, Result: resp})
		}
	case cbLobbyChatMsg:
		if len(p) >= 24 {
			lobby, user, chatID := le64(p, 0), le64(p, 8), le32(p, 20)
			buf := make([]byte, 4096)
			var sender uint64
			var typ int32
			n := int(int32(s.call("SteamAPI_ISteamMatchmaking_GetLobbyChatEntry", s.mm, uintptr(lobby), uintptr(chatID),
				uintptr(unsafe.Pointer(&sender)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&typ)))))
			if n > 0 {
				s.emit(SteamEvent{Kind: EvLobbyChat, Lobby: lobby, User: user, Text: string(bytes.TrimRight(buf[:n], "\x00"))})
			}
		}
	case cbLobbyDataUpdate, cbLobbyChatUpdate:
		if len(p) >= 8 {
			s.emit(SteamEvent{Kind: EvLobbyChanged, Lobby: le64(p, 0)})
		}
	case cbItemInstalled, cbFileSubscribed, cbFileUnsubscribed, cbDownloadItemResult:
		s.emit(SteamEvent{Kind: EvWorkshopChanged})
	case cbWebTicket:
		gotWebTicket(p) // steam_ticket_windows.go
	case cbGameLobbyJoinRequested:
		if len(p) >= 16 {
			s.emit(SteamEvent{Kind: EvJoinRequested, Lobby: le64(p, 0), User: le64(p, 8)})
		}
	}
}

func (s *realSteam) locked(f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dll == nil {
		return
	}
	f()
}

func (s *realSteam) MyID() (id uint64) {
	s.locked(func() { id = uint64(s.call("SteamAPI_ISteamUser_GetSteamID", s.user)) })
	return
}

func (s *realSteam) MyName() (name string) {
	s.locked(func() { name = gostr(s.call("SteamAPI_ISteamFriends_GetPersonaName", s.friend)) })
	return
}

func (s *realSteam) PersonaName(id uint64) (name string) {
	s.locked(func() {
		name = gostr(s.call("SteamAPI_ISteamFriends_GetFriendPersonaName", s.friend, uintptr(id)))
		if name == "" || name == "[unknown]" {
			s.call("SteamAPI_ISteamFriends_RequestUserInformation", s.friend, uintptr(id), 1)
		}
	})
	return
}

const friendFlagImmediate = 4

func (s *realSteam) Friends() (out []Friend) {
	s.locked(func() {
		n := int(int32(s.call("SteamAPI_ISteamFriends_GetFriendCount", s.friend, friendFlagImmediate)))
		for i := 0; i < n; i++ {
			id := uint64(s.call("SteamAPI_ISteamFriends_GetFriendByIndex", s.friend, uintptr(i), friendFlagImmediate))
			f := Friend{ID: id}
			f.Name = gostr(s.call("SteamAPI_ISteamFriends_GetFriendPersonaName", s.friend, uintptr(id)))
			f.Persona = int(int32(s.call("SteamAPI_ISteamFriends_GetFriendPersonaState", s.friend, uintptr(id))))
			f.Online = f.Persona != 0
			var info [24]byte // FriendGameInfo_t: gameID u64, ip u32, port u16, qport u16, lobby u64
			if truthy(s.call("SteamAPI_ISteamFriends_GetFriendGamePlayed", s.friend, uintptr(id), uintptr(unsafe.Pointer(&info[0])))) {
				game := le64(info[:], 0)
				f.InGame = game&0xffffff == PZAppID
				f.Lobby = le64(info[:], 16)
			}
			out = append(out, f)
		}
	})
	return
}

func (s *realSteam) Avatar(id uint64) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if png, ok := s.avatar[id]; ok {
		return png
	}
	if s.dll == nil {
		return nil
	}
	h := int32(s.call("SteamAPI_ISteamFriends_GetMediumFriendAvatar", s.friend, uintptr(id)))
	if h <= 0 {
		return nil // not loaded yet (-1) or none (0)
	}
	var w, hh uint32
	if !truthy(s.call("SteamAPI_ISteamUtils_GetImageSize", s.utils, uintptr(h), uintptr(unsafe.Pointer(&w)), uintptr(unsafe.Pointer(&hh)))) || w == 0 {
		return nil
	}
	img := image.NewNRGBA(image.Rect(0, 0, int(w), int(hh)))
	if !truthy(s.call("SteamAPI_ISteamUtils_GetImageRGBA", s.utils, uintptr(h), uintptr(unsafe.Pointer(&img.Pix[0])), uintptr(len(img.Pix)))) {
		return nil
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	s.avatar[id] = buf.Bytes()
	return s.avatar[id]
}

const lobbyFriendsOnly = 1

func (s *realSteam) CreateLobby(max int) {
	s.locked(func() {
		h := uint64(s.call("SteamAPI_ISteamMatchmaking_CreateLobby", s.mm, lobbyFriendsOnly, uintptr(max)))
		s.calls[h] = cbLobbyCreated
	})
}

func (s *realSteam) JoinLobby(lobby uint64) {
	s.locked(func() {
		h := uint64(s.call("SteamAPI_ISteamMatchmaking_JoinLobby", s.mm, uintptr(lobby)))
		s.calls[h] = cbLobbyEnter
	})
}

func (s *realSteam) LeaveLobby(lobby uint64) {
	s.locked(func() { s.call("SteamAPI_ISteamMatchmaking_LeaveLobby", s.mm, uintptr(lobby)) })
}

func (s *realSteam) InviteToLobby(lobby, friend uint64) (ok bool) {
	s.locked(func() {
		ok = truthy(s.call("SteamAPI_ISteamMatchmaking_InviteUserToLobby", s.mm, uintptr(lobby), uintptr(friend)))
	})
	return
}

func (s *realSteam) LobbyOwner(lobby uint64) (id uint64) {
	s.locked(func() { id = uint64(s.call("SteamAPI_ISteamMatchmaking_GetLobbyOwner", s.mm, uintptr(lobby))) })
	return
}

func (s *realSteam) LobbyMembers(lobby uint64) (out []uint64) {
	s.locked(func() {
		n := int(int32(s.call("SteamAPI_ISteamMatchmaking_GetNumLobbyMembers", s.mm, uintptr(lobby))))
		for i := 0; i < n; i++ {
			out = append(out, uint64(s.call("SteamAPI_ISteamMatchmaking_GetLobbyMemberByIndex", s.mm, uintptr(lobby), uintptr(i))))
		}
	})
	return
}

func (s *realSteam) LobbyData(lobby uint64, key string) (v string) {
	s.locked(func() { v = gostr(s.call("SteamAPI_ISteamMatchmaking_GetLobbyData", s.mm, uintptr(lobby), uintptr(cstr(key)))) })
	return
}

func (s *realSteam) SetLobbyData(lobby uint64, key, value string) (ok bool) {
	s.locked(func() {
		ok = truthy(s.call("SteamAPI_ISteamMatchmaking_SetLobbyData", s.mm, uintptr(lobby), uintptr(cstr(key)), uintptr(cstr(value))))
	})
	return
}

func (s *realSteam) SetLobbyJoinable(lobby uint64, joinable bool) {
	b := uintptr(0)
	if joinable {
		b = 1
	}
	s.locked(func() { s.call("SteamAPI_ISteamMatchmaking_SetLobbyJoinable", s.mm, uintptr(lobby), b) })
}

func (s *realSteam) MemberData(lobby, member uint64, key string) (v string) {
	s.locked(func() {
		v = gostr(s.call("SteamAPI_ISteamMatchmaking_GetLobbyMemberData", s.mm, uintptr(lobby), uintptr(member), uintptr(cstr(key))))
	})
	return
}

func (s *realSteam) SetMemberData(lobby uint64, key, value string) {
	s.locked(func() {
		s.call("SteamAPI_ISteamMatchmaking_SetLobbyMemberData", s.mm, uintptr(lobby), uintptr(cstr(key)), uintptr(cstr(value)))
	})
}

func (s *realSteam) SendLobbyMessage(lobby uint64, msg string) (ok bool) {
	b := append([]byte(msg), 0)
	s.locked(func() {
		ok = truthy(s.call("SteamAPI_ISteamMatchmaking_SendLobbyChatMsg", s.mm, uintptr(lobby), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b))))
	})
	return
}

func (s *realSteam) RequestLobbyData(lobby uint64) {
	s.locked(func() { s.call("SteamAPI_ISteamMatchmaking_RequestLobbyData", s.mm, uintptr(lobby)) })
}

func (s *realSteam) SetRichPresence(key, value string) {
	s.locked(func() { s.call("SteamAPI_ISteamFriends_SetRichPresence", s.friend, uintptr(cstr(key)), uintptr(cstr(value))) })
}

func (s *realSteam) ItemState(item uint64) (st uint32) {
	s.locked(func() { st = uint32(s.call("SteamAPI_ISteamUGC_GetItemState", s.ugc, uintptr(item))) })
	return
}

func (s *realSteam) ItemInstallInfo(item uint64) (folder string, ts uint32, ok bool) {
	s.locked(func() {
		var size uint64
		buf := make([]byte, 1024)
		ok = truthy(s.call("SteamAPI_ISteamUGC_GetItemInstallInfo", s.ugc, uintptr(item), uintptr(unsafe.Pointer(&size)),
			uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&ts))))
		folder = string(bytes.TrimRight(buf, "\x00"))
	})
	return
}

func (s *realSteam) ItemDownloadInfo(item uint64) (done, total uint64, ok bool) {
	s.locked(func() {
		ok = truthy(s.call("SteamAPI_ISteamUGC_GetItemDownloadInfo", s.ugc, uintptr(item), uintptr(unsafe.Pointer(&done)), uintptr(unsafe.Pointer(&total))))
	})
	return
}

func (s *realSteam) SubscribeItem(item uint64) {
	s.locked(func() { s.call("SteamAPI_ISteamUGC_SubscribeItem", s.ugc, uintptr(item)) })
}

func (s *realSteam) DownloadItem(item uint64, high bool) (ok bool) {
	b := uintptr(0)
	if high {
		b = 1
	}
	s.locked(func() { ok = truthy(s.call("SteamAPI_ISteamUGC_DownloadItem", s.ugc, uintptr(item), b)) })
	return
}

// SubscribedItems lists the Workshop items this account is subscribed to (newer SDKs take an extra
// "include locally disabled" flag; passing it to an older DLL is harmless on x64).
func (s *realSteam) SubscribedItems() (out []uint64) {
	s.locked(func() {
		n := uint32(s.call("SteamAPI_ISteamUGC_GetNumSubscribedItems", s.ugc, 0))
		if n == 0 || n > 100000 {
			return
		}
		buf := make([]uint64, n)
		got := uint32(s.call("SteamAPI_ISteamUGC_GetSubscribedItems", s.ugc, uintptr(unsafe.Pointer(&buf[0])), uintptr(n), 0))
		if got > n {
			got = n
		}
		out = buf[:got]
	})
	return
}

func (s *realSteam) UnsubscribeItem(item uint64) {
	s.locked(func() { s.call("SteamAPI_ISteamUGC_UnsubscribeItem", s.ugc, uintptr(item)) })
}

func (s *realSteam) Ready() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dll != nil
}

const cbUGCQueryCompleted = 3401

// UGC query layout (SteamUGCDetails_t, pack 8)
const (
	ugcDetailsSize = 9784
	ugcTitle       = 24
	ugcDesc        = 153
	ugcUpdated     = 8172
	ugcTags        = 8187
	ugcVotesUp     = 9756
	ugcVotesDown   = 9760
	ugcScore       = 9764
)

func cbytes(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

// QueryWorkshop runs one Workshop search page and waits for Steam's answer.
func (s *realSteam) QueryWorkshop(q WorkshopQuery) (WorkshopPage, error) {
	var page WorkshopPage
	kind := map[string]uintptr{"top": 0, "new": 1, "trend": 3, "subs": 12}[q.Sort]
	if q.Sort == "" {
		kind = 3
	}
	if strings.TrimSpace(q.Text) != "" {
		kind = 11 // ranked by text search
	}
	if q.Page < 1 {
		q.Page = 1
	}
	ch := make(chan []byte, 1)
	var qh uint64
	s.locked(func() {
		qh = uint64(s.call("SteamAPI_ISteamUGC_CreateQueryAllUGCRequestPage", s.ugc, kind, 0, uintptr(PZAppID), uintptr(PZAppID), uintptr(q.Page)))
		if qh == 0 || qh == ^uint64(0) {
			qh = 0
			return
		}
		if t := strings.TrimSpace(q.Text); t != "" {
			s.call("SteamAPI_ISteamUGC_SetSearchText", s.ugc, uintptr(qh), uintptr(cstr(t)))
		}
		if q.Tag != "" {
			s.call("SteamAPI_ISteamUGC_AddRequiredTag", s.ugc, uintptr(qh), uintptr(cstr(q.Tag)))
		}
		if kind == 3 {
			s.call("SteamAPI_ISteamUGC_SetRankedByTrendDays", s.ugc, uintptr(qh), 7)
		}
		h := uint64(s.call("SteamAPI_ISteamUGC_SendQueryUGCRequest", s.ugc, uintptr(qh)))
		if h == 0 {
			s.call("SteamAPI_ISteamUGC_ReleaseQueryUGCRequest", s.ugc, uintptr(qh))
			qh = 0
			return
		}
		s.waiters[h] = ch
	})
	if qh == 0 {
		return page, errors.New("Steam didn't take the Workshop search")
	}
	var res []byte
	select {
	case res = <-ch:
	case <-time.After(20 * time.Second):
		return page, errors.New("the Workshop didn't answer in time")
	}
	defer s.locked(func() { s.call("SteamAPI_ISteamUGC_ReleaseQueryUGCRequest", s.ugc, uintptr(qh)) })
	if len(res) < 20 || int32(le32(res, 8)) != 1 {
		return page, errors.New("the Workshop search failed")
	}
	n := le32(res, 12)
	page.Total = int(le32(res, 16))
	s.locked(func() {
		d := make([]byte, ugcDetailsSize+64)
		url := make([]byte, 1024)
		for i := uint32(0); i < n; i++ {
			for j := range d {
				d[j] = 0
			}
			if !truthy(s.call("SteamAPI_ISteamUGC_GetQueryUGCResult", s.ugc, uintptr(qh), uintptr(i), uintptr(unsafe.Pointer(&d[0])))) {
				continue
			}
			it := WorkshopItem{
				ID:          le64(d, 0),
				Title:       cbytes(d[ugcTitle : ugcTitle+129]),
				Description: cbytes(d[ugcDesc : ugcDesc+8000]),
				Tags:        cbytes(d[ugcTags : ugcTags+1025]),
				Updated:     le32(d, ugcUpdated),
				VotesUp:     le32(d, ugcVotesUp),
				VotesDown:   le32(d, ugcVotesDown),
				Score:       math.Float32frombits(le32(d, ugcScore)),
			}
			for j := range url {
				url[j] = 0
			}
			if truthy(s.call("SteamAPI_ISteamUGC_GetQueryUGCPreviewURL", s.ugc, uintptr(qh), uintptr(i), uintptr(unsafe.Pointer(&url[0])), uintptr(len(url)))) {
				it.Preview = cbytes(url)
			}
			var subs uint64
			if truthy(s.call("SteamAPI_ISteamUGC_GetQueryUGCStatistic", s.ugc, uintptr(qh), uintptr(i), 0, uintptr(unsafe.Pointer(&subs)))) {
				it.Subs = subs
			}
			st := uint32(s.call("SteamAPI_ISteamUGC_GetItemState", s.ugc, uintptr(it.ID)))
			it.Subscribed = st&ItemSubscribed != 0
			it.Installed = st&ItemInstalled != 0
			page.Items = append(page.Items, it)
		}
	})
	return page, nil
}
