//go:build windows

package main

import (
	"errors"
	"time"
	"unsafe"
)

// ItemUpdated asks the Workshop when an item was last updated (Unix time).
func (s *realSteam) ItemUpdated(id uint64) (uint32, error) {
	ch := make(chan []byte, 1)
	var qh uint64
	ids := []uint64{id}
	s.locked(func() {
		qh = uint64(s.call("SteamAPI_ISteamUGC_CreateQueryUGCDetailsRequest", s.ugc, uintptr(unsafe.Pointer(&ids[0])), 1))
		if qh == 0 || qh == ^uint64(0) {
			qh = 0
			return
		}
		s.call("SteamAPI_ISteamUGC_SetAllowCachedResponse", s.ugc, uintptr(qh), 0)
		h := uint64(s.call("SteamAPI_ISteamUGC_SendQueryUGCRequest", s.ugc, uintptr(qh)))
		if h == 0 {
			s.call("SteamAPI_ISteamUGC_ReleaseQueryUGCRequest", s.ugc, uintptr(qh))
			qh = 0
			return
		}
		s.waiters[h] = ch
	})
	if qh == 0 {
		return 0, errors.New("Steam didn't take the Workshop check")
	}
	var res []byte
	select {
	case res = <-ch:
	case <-time.After(20 * time.Second):
		return 0, errors.New("the Workshop didn't answer in time")
	}
	defer s.locked(func() { s.call("SteamAPI_ISteamUGC_ReleaseQueryUGCRequest", s.ugc, uintptr(qh)) })
	if len(res) < 20 || int32(le32(res, 8)) != 1 || le32(res, 12) < 1 {
		return 0, errors.New("the Workshop check failed")
	}
	var updated uint32
	s.locked(func() {
		d := make([]byte, ugcDetailsSize+64)
		if truthy(s.call("SteamAPI_ISteamUGC_GetQueryUGCResult", s.ugc, uintptr(qh), 0, uintptr(unsafe.Pointer(&d[0])))) && le64(d, 0) == id && int32(le32(d, 8)) == 1 {
			updated = le32(d, ugcUpdated)
		}
	})
	if updated == 0 {
		return 0, errors.New("the Workshop check failed")
	}
	return updated, nil
}
