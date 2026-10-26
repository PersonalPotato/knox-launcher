//go:build !windows

package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// mockSteam lets the app run on a dev machine without Steam: two fake friends join any lobby
// you create, say hi and ready up.
type mockSteam struct {
	mu      sync.Mutex
	events  chan SteamEvent
	lobby   uint64
	members []uint64
	data    map[string]string
	member  map[uint64]map[string]string
	owner   uint64
	items   map[uint64]uint32
}

func newSteam(string) Steam {
	return &mockSteam{events: make(chan SteamEvent, 64), data: map[string]string{}, member: map[uint64]map[string]string{}, items: map[uint64]uint32{}}
}

const mockMe = 76561198000000001

var mockNames = map[uint64]string{mockMe: "Dan", 76561198000000002: "Jess", 76561198000000003: "Mikey", 76561198000000004: "Rob"}

func (m *mockSteam) Init() error                    { return nil }
func (m *mockSteam) Shutdown()                      {}
func (m *mockSteam) RunCallbacks()                  {}
func (m *mockSteam) MyID() uint64                   { return mockMe }
func (m *mockSteam) MyName() string                 { return "Dan" }
func (m *mockSteam) Events() <-chan SteamEvent      { return m.events }
func (m *mockSteam) Avatar(uint64) []byte           { return nil }
func (m *mockSteam) PersonaName(id uint64) string   { return mockNames[id] }
func (m *mockSteam) SetRichPresence(string, string) {}

func (m *mockSteam) Friends() []Friend {
	return []Friend{
		{ID: 76561198000000002, Name: "Jess", Online: true, InGame: true, Lobby: 109775241000000002, Persona: 1},
		{ID: 76561198000000003, Name: "Mikey", Online: true, Persona: 1},
		{ID: 76561198000000004, Name: "Rob", Online: false},
	}
}

func (m *mockSteam) CreateLobby(int) {
	m.mu.Lock()
	m.lobby = 109775241000000001
	m.members = []uint64{mockMe}
	m.mu.Unlock()
	m.events <- SteamEvent{Kind: EvLobbyCreated, OK: true, Lobby: m.lobby}
	m.events <- SteamEvent{Kind: EvLobbyEntered, OK: true, Lobby: m.lobby, Result: 1}
	go func() {
		for i, id := range []uint64{76561198000000002, 76561198000000003} {
			time.Sleep(time.Duration(2+i*2) * time.Second)
			m.mu.Lock()
			m.members = append(m.members, id)
			m.member[id] = map[string]string{"char": mockChar(i), "ready": "0", "app": appVersion}
			m.mu.Unlock()
			m.events <- SteamEvent{Kind: EvLobbyChanged, Lobby: m.lobby}
			m.events <- SteamEvent{Kind: EvLobbyChat, Lobby: m.lobby, User: id, Text: "c:yo"}
			time.Sleep(3 * time.Second)
			m.mu.Lock()
			m.member[id]["ready"] = "1"
			m.mu.Unlock()
			m.events <- SteamEvent{Kind: EvLobbyChanged, Lobby: m.lobby}
		}
	}()
}

// Joining any lobby code in dev puts you in Jess's lobby, as a guest.
func (m *mockSteam) JoinLobby(l uint64) {
	m.mu.Lock()
	m.owner = 76561198000000002
	m.data["host"] = "Jess"
	m.data["state"] = "open"
	m.data["info.0"] = `{"world":"JessWorld","newWorld":false,"mods":[{"id":"ZombieBuddy","name":"ZombieBuddy","ws":"3619862853"},{"id":"SomethingNew","name":"Some Mod You Lack","ws":"123"}],"spawn":{"x":10770,"y":10271,"z":0,"name":"Muldraugh, KY","kind":"HouseSuburb"},"spawnMode":"random","spawnType":"house","together":true,"kit":[{"type":"Base.Hammer","name":"Hammer","count":1}],"kitName":"Builders","pvp":false,"sleep":true,"freePoints":4,"difficulty":"knox-casual","sandbox":[{"name":"Zombies","value":"4"}],"ini":[],"respawn":"squad","backstory":"family","lobby":true,"roles":{"The Handy Uncle":"76561198000000002"},"lives":3,"cast":{"world":"JessWorld","updated":1761460000,"day":12,"lives":3,"livesMode":"each","ended":false,"cast":[{"user":"Jess","name":"Darla Pruitt","alive":true,"out":false,"deaths":0,"livesLeft":3,"kills":143,"hours":280,"firstDay":0,"lastDay":12,"lastSeen":1761460000,"died":[]},{"user":"Dan","name":"Wes Hale","alive":true,"out":false,"deaths":1,"livesLeft":2,"kills":87,"hours":120,"firstDay":0,"lastDay":12,"lastSeen":1761460000,"died":[{"day":6,"name":"Jack Hale","hours":150,"kills":40}]},{"user":"Mikey","name":"Cody Bowman","alive":false,"out":true,"deaths":3,"livesLeft":0,"kills":21,"hours":60,"firstDay":0,"lastDay":9,"lastSeen":1761400000,"died":[{"day":2,"name":"Cody Bowman","hours":40,"kills":9},{"day":5,"name":"Cody Bowman","hours":50,"kills":8},{"day":9,"name":"Cody Bowman","hours":20,"kills":4}]}]}}`
	m.data["info.n"] = "1"
	m.lobby = 109775241000000002
	m.member[76561198000000002] = map[string]string{"char": mockChar(0), "ready": "1", "app": appVersion, "role": "The Handy Uncle"}
	m.members = []uint64{76561198000000002, mockMe}
	m.mu.Unlock()
	m.events <- SteamEvent{Kind: EvLobbyEntered, OK: true, Lobby: 109775241000000002, Result: 1}
}
func (m *mockSteam) LeaveLobby(uint64) {
	m.mu.Lock()
	m.lobby, m.members = 0, nil
	m.mu.Unlock()
}
func (m *mockSteam) InviteToLobby(uint64, uint64) bool { return true }
func (m *mockSteam) LobbyOwner(uint64) uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.owner != 0 {
		return m.owner
	}
	return mockMe
}
func (m *mockSteam) LobbyMembers(uint64) []uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]uint64(nil), m.members...)
}
func (m *mockSteam) LobbyData(_ uint64, k string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data[k]
}
func (m *mockSteam) SetLobbyData(_ uint64, k, v string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[k] = v
	if k == "launch" {
		// friends download for a few seconds, then report done
		go func() {
			for i, sync := range []string{"3/10", "done:" + v} {
				time.Sleep(time.Duration(3+i*3) * time.Second)
				m.mu.Lock()
				for id, d := range m.member {
					if id != mockMe {
						d["sync"] = sync
					}
				}
				m.mu.Unlock()
			}
		}()
	}
	return true
}
func (m *mockSteam) SetLobbyJoinable(uint64, bool) {}
func (m *mockSteam) MemberData(_ uint64, id uint64, k string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.member[id] == nil {
		return ""
	}
	return m.member[id][k]
}
func (m *mockSteam) SetMemberData(_ uint64, k, v string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.member[mockMe] == nil {
		m.member[mockMe] = map[string]string{}
	}
	m.member[mockMe][k] = v
}
func (m *mockSteam) SendLobbyMessage(l uint64, msg string) bool {
	m.events <- SteamEvent{Kind: EvLobbyChat, Lobby: l, User: mockMe, Text: msg}
	return true
}
func (m *mockSteam) RequestLobbyData(uint64) {}

func (m *mockSteam) ItemState(item uint64) uint32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	if st, ok := m.items[item]; ok {
		return st
	}
	return ItemSubscribed | ItemInstalled
}
func (m *mockSteam) ItemInstallInfo(item uint64) (string, uint32, bool) {
	return fmt.Sprintf("/tmp/workshop/%d", item), 1760000000, true
}
func (m *mockSteam) ItemDownloadInfo(uint64) (uint64, uint64, bool) { return 1, 1, true }
func (m *mockSteam) SubscribeItem(item uint64) {
	m.mu.Lock()
	m.items[item] = ItemSubscribed | ItemInstalled
	m.mu.Unlock()
}
func (m *mockSteam) DownloadItem(uint64, bool) bool { return true }

// nil = unknown, so every installed Workshop folder counts
func (m *mockSteam) SubscribedItems() []uint64 { return nil }
func (m *mockSteam) UnsubscribeItem(uint64)    {}
func (m *mockSteam) Ready() bool               { return true }

func mockChar(i int) string {
	chars := []Character{
		{Forename: "Jess", Surname: "Haskell", Female: true, Profession: "nurse",
			Skin: RGB{0.98, 0.79, 0.49}, Hair: "Bob", HairColor: RGB{0.62, 0.42, 0.17},
			Clothing:     map[string]string{"Tshirt": "Base.Tshirt_DefaultTEXTURE_TINT", "Pants": "Base.Trousers_Denim", "Shoes": "Base.Shoes_Black"},
			ClothingTint: map[string]*RGB{"Tshirt": {0.3, 0.5, 0.8}}},
		{Forename: "Mikey", Surname: "Dawson", Profession: "fireofficer",
			Skin: RGB{0.54, 0.38, 0.25}, Hair: "CrewCut", HairColor: RGB{0.1, 0.08, 0.06}, Beard: "Chops",
			Clothing: map[string]string{"Jacket": "Base.Jacket_Black", "Pants": "Base.Trousers_Denim", "Shoes": "Base.Shoes_Black"}},
	}
	c := chars[i%len(chars)]
	b, _ := json.Marshal(map[string]any{"name": c.Forename + " " + c.Surname, "prof": c.Profession, "traits": len(c.Traits), "female": c.Female, "c": c})
	return string(b)
}

var mockWorkshopNames = []string{"Brita's Weapon Pack", "Authentic Z", "True Music", "Expanded Helicopter Events", "Raven Creek", "Bedford Falls",
	"Week One", "Bandits", "Tsar's Common Library", "Filibuster Rhymes' Used Cars", "Skill Recovery Journal", "Mod Options",
	"Better Sorting", "Common Sense", "Fancy Handwork", "Eerie Country", "Lingering Voices", "Spongie's Clothing"}

func (m *mockSteam) QueryWorkshop(q WorkshopQuery) (WorkshopPage, error) {
	time.Sleep(300 * time.Millisecond)
	var out WorkshopPage
	t := strings.ToLower(strings.TrimSpace(q.Text))
	for i, n := range mockWorkshopNames {
		if t != "" && !strings.Contains(strings.ToLower(n), t) {
			continue
		}
		id := uint64(2800000000 + i*7919)
		st := m.ItemState(id)
		if i%3 != 0 && st == ItemSubscribed|ItemInstalled {
			st = 0
		}
		out.Items = append(out.Items, WorkshopItem{ID: id, Title: n, Description: "A mod for Build 42. Adds things and changes other things.",
			Tags: "Build 42,Items,Realistic", Subs: uint64(400000 / (i + 1)), VotesUp: uint32(9000 / (i + 1)), VotesDown: uint32(300 / (i + 1)), Score: 0.9,
			Updated: 1761000000 - uint32(i*86400), Subscribed: st&ItemSubscribed != 0, Installed: st&ItemInstalled != 0})
	}
	out.Total = len(out.Items)
	return out, nil
}
