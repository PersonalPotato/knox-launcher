package main

import (
	"strings"
	"testing"
	"time"
)

type mapSteam struct {
	Steam
	d map[string]string
}

func (m *mapSteam) SetLobbyData(l uint64, k, v string) bool { m.d[k] = v; return true }
func (m *mapSteam) LobbyData(l uint64, k string) string     { return m.d[k] }

func TestLobbyBigUTF8(t *testing.T) {
	st := &mapSteam{d: map[string]string{}}
	v := `{"a":"` + strings.Repeat("Ж", 5000) + `"}`
	setLobbyBig(st, 1, "manifest", v)
	got, ok := waitLobbyBig(st, 1, "manifest", time.Second)
	if !ok || got != v {
		t.Fatal("round trip failed")
	}
	st.d["manifest.0"] = "x"
	if _, ok := waitLobbyBig(st, 1, "manifest", time.Second); ok {
		t.Fatal("accepted a broken value")
	}
}

func TestManifestChat(t *testing.T) {
	a := &App{}
	v := `{"m":["A","B"],"i":[{"id":"1","ts":0,"n":"x"}]}`
	p := packBig(v)
	half := len(p) / 2
	a.gotManifestPart("mf:L1:1/2:" + p[half:])
	if a.chatManifest("L1") != "" {
		t.Fatal("used half a list")
	}
	a.gotManifestPart("mf:L1:0/2:" + p[:half])
	if a.chatManifest("L1") != v || a.chatManifest("") != v {
		t.Fatal("chat manifest didn't round trip")
	}
}
