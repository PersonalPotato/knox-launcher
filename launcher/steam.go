package main

// Steam is everything the app needs from Steam. On Windows it talks to steam_api64.dll from the
// Project Zomboid install (steam_windows.go); elsewhere a fake is used for UI work (steam_mock.go).
type Steam interface {
	Init() error
	Shutdown()
	RunCallbacks()

	MyID() uint64
	MyName() string
	Friends() []Friend
	Avatar(id uint64) []byte // PNG, nil when unknown
	PersonaName(id uint64) string

	CreateLobby(maxMembers int) // result arrives as an event
	JoinLobby(lobby uint64)
	LeaveLobby(lobby uint64)
	InviteToLobby(lobby, friend uint64) bool
	LobbyOwner(lobby uint64) uint64
	LobbyMembers(lobby uint64) []uint64
	LobbyData(lobby uint64, key string) string
	SetLobbyData(lobby uint64, key, value string) bool
	SetLobbyJoinable(lobby uint64, joinable bool)
	MemberData(lobby, member uint64, key string) string
	SetMemberData(lobby uint64, key, value string)
	SendLobbyMessage(lobby uint64, msg string) bool
	RequestLobbyData(lobby uint64)
	SetRichPresence(key, value string)

	ItemState(item uint64) uint32
	ItemInstallInfo(item uint64) (folder string, timestamp uint32, ok bool)
	ItemDownloadInfo(item uint64) (done, total uint64, ok bool)
	SubscribeItem(item uint64)
	DownloadItem(item uint64, highPriority bool) bool
	SubscribedItems() []uint64
	UnsubscribeItem(item uint64)
	Ready() bool
	QueryWorkshop(q WorkshopQuery) (WorkshopPage, error)

	Events() <-chan SteamEvent
}

type Friend struct {
	ID      uint64 `json:"id,string"`
	Name    string `json:"name"`
	Online  bool   `json:"online"`
	InGame  bool   `json:"inGame"` // playing Project Zomboid (or in an Knox Launcher)
	Lobby   uint64 `json:"lobby,string"`
	Persona int    `json:"persona"`
}

// Workshop item state flags (EItemState)
const (
	ItemSubscribed      = 1
	ItemLegacy          = 2
	ItemInstalled       = 4
	ItemNeedsUpdate     = 8
	ItemDownloading     = 16
	ItemDownloadPending = 32
)

type SteamEventKind int

const (
	EvLobbyCreated SteamEventKind = iota
	EvLobbyEntered
	EvLobbyChat
	EvLobbyChanged
	EvJoinRequested
	EvWorkshopChanged
)

type SteamEvent struct {
	Kind   SteamEventKind
	Lobby  uint64
	User   uint64
	OK     bool
	Text   string
	Result int
}

const PZAppID = 108600

// WorkshopQuery is one page of a Workshop search for Project Zomboid items.
type WorkshopQuery struct {
	Text string `json:"text"`
	Sort string `json:"sort"` // "trend", "top", "new", "subs"
	Page int    `json:"page"` // from 1
	Tag  string `json:"tag"`  // e.g. "Build 42"; empty for any
}

type WorkshopItem struct {
	ID          uint64 `json:"id,string"`
	Title       string `json:"title"`
	Description string `json:"desc"`
	Preview     string `json:"preview"`
	Tags        string `json:"tags"`
	Subs        uint64 `json:"subs"`
	VotesUp     uint32 `json:"up"`
	VotesDown   uint32 `json:"down"`
	Score       float32 `json:"score"`
	Updated     uint32 `json:"updated"`
	Subscribed  bool   `json:"subscribed"`
	Installed   bool   `json:"installed"`
}

type WorkshopPage struct {
	Items []WorkshopItem `json:"items"`
	Total int            `json:"total"`
}
