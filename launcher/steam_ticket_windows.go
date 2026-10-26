//go:build windows

package main

import (
	"encoding/hex"
	"errors"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Steam sign-in for the community server: a ticket the server checks with Steam's Web API.

const cbWebTicket = 168 // GetTicketForWebApiResponse_t

var webTickets = struct {
	sync.Mutex
	wait map[uint32]chan string
}{wait: map[uint32]chan string{}}

// gotWebTicket runs inside RunCallbacks: HAuthTicket, EResult, int size, then the ticket bytes.
func gotWebTicket(p []byte) {
	if len(p) < 12 {
		return
	}
	h, res, n := le32(p, 0), int32(le32(p, 4)), int(int32(le32(p, 8)))
	ticket := ""
	if res == 1 && n > 0 && 12+n <= len(p) {
		ticket = hex.EncodeToString(p[12 : 12+n])
	}
	webTickets.Lock()
	ch := webTickets.wait[h]
	delete(webTickets.wait, h)
	webTickets.Unlock()
	if ch != nil {
		ch <- ticket
	}
}

// WebTicket returns a ticket and the identity it was made for ("-" for an old-style session ticket).
func (s *realSteam) WebTicket(identity string) (string, string, error) {
	ch := make(chan string, 1)
	var old string
	ok := false
	s.locked(func() {
		ok = true
		if p, err := s.dll.FindProc("SteamAPI_ISteamUser_GetAuthTicketForWebApi"); err == nil {
			id, _ := syscall.BytePtrFromString(identity)
			webTickets.Lock()
			r, _, _ := p.Call(s.user, uintptr(unsafe.Pointer(id)))
			runtime.KeepAlive(id)
			if uint32(r) != 0 {
				webTickets.wait[uint32(r)] = ch
			}
			webTickets.Unlock()
			if uint32(r) != 0 {
				return
			}
		}
		// older steam_api64.dll: a session ticket works with the same Web API call
		buf := make([]byte, 2048)
		var size uint32
		r := s.call("SteamAPI_ISteamUser_GetAuthSessionTicket", s.user, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&size)), 0)
		if uint32(r) != 0 && size > 0 && int(size) <= len(buf) {
			old = hex.EncodeToString(buf[:size])
		}
	})
	if !ok {
		return "", "", errors.New("Steam isn't running")
	}
	if old != "" {
		return old, "-", nil
	}
	select {
	case t := <-ch:
		if t == "" {
			return "", "", errors.New("Steam wouldn't sign you in")
		}
		return t, identity, nil
	case <-time.After(15 * time.Second):
		return "", "", errors.New("Steam didn't answer the sign-in")
	}
}
