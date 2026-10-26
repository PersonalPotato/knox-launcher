package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// The community server (Cloudflare Worker, source in easyhost/community-server). Anyone can read
// it; uploading, liking and commenting send a Steam ticket so the server knows who you are.

const communityServer = "https://knox-community.daniel-cahill1999.workers.dev" // Dan's Cloudflare worker; Settings can override it

type remoteItem struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	Name      string          `json:"name"`
	Author    string          `json:"author"`
	AuthorID  string          `json:"authorId"`
	About     string          `json:"about"`
	Tags      []string        `json:"tags"`
	Data      json.RawMessage `json:"data"`
	Downloads int             `json:"downloads"`
	Likes     int             `json:"likes"`
	Comments  int             `json:"comments"`
	Created   int64           `json:"created"`
	Updated   int64           `json:"updated"`
	Liked     bool            `json:"liked"`
}

type remoteMe struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Admin bool   `json:"admin"`
}

type remotePackData struct {
	Mods []CommunityMod `json:"mods"`
}

var remote = struct {
	sync.Mutex
	items   []remoteItem
	me      *remoteMe
	err     string
	fetched time.Time
	url     string
	ticket  string
	ident   string
}{}

var communityHTTP = &http.Client{Timeout: 15 * time.Second}

func (a *App) communityURL() string {
	if v := os.Getenv("EZ_COMMUNITY"); v != "" {
		return strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(a.cfg.Get().CommunityURL); v != "" {
		return strings.TrimRight(v, "/")
	}
	return communityServer
}

type webTicketer interface {
	WebTicket(identity string) (ticket, usedIdentity string, err error)
}

// steamTicket signs in once per run; the server remembers a checked ticket for a day.
func (a *App) steamTicket() (string, string, error) {
	remote.Lock()
	t, id := remote.ticket, remote.ident
	remote.Unlock()
	if t != "" {
		return t, id, nil
	}
	wt, ok := a.steam.(webTicketer)
	if !ok || !a.steam.Ready() {
		return "", "", errors.New("Steam isn't running, so you can't post")
	}
	t, id, err := wt.WebTicket("knox")
	if err != nil {
		return "", "", err
	}
	remote.Lock()
	remote.ticket, remote.ident = t, id
	remote.Unlock()
	return t, id, nil
}

// remoteCall talks to the server. auth: 0 none, 1 send a ticket if we can, 2 must sign in.
func (a *App) remoteCall(method, path string, body any, auth int, out any) error {
	base := a.communityURL()
	if base == "" {
		return errors.New("there's no community server set up yet")
	}
	if base == "mock" {
		return a.mockCall(method, path, body, out) // community_mock.go
	}
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, base+path, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth > 0 {
		t, id, err := a.steamTicket()
		if err != nil && auth == 2 {
			return err
		}
		if err == nil {
			req.Header.Set("X-Knox-Ticket", t)
			req.Header.Set("X-Knox-Identity", id)
		}
	}
	res, err := communityHTTP.Do(req)
	if err != nil {
		return errors.New("couldn't reach the community server")
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	var e struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(raw, &e) == nil && e.Error != "" {
		if res.StatusCode == 401 {
			remote.Lock()
			remote.ticket = "" // try a fresh ticket next time
			remote.Unlock()
		}
		return errors.New(e.Error)
	}
	if res.StatusCode >= 300 {
		return errors.New("the community server said " + res.Status)
	}
	if out != nil {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// remoteList returns the server's uploads, cached for a little while.
func (a *App) remoteList(fresh bool) ([]remoteItem, *remoteMe, string) {
	url := a.communityURL()
	if url == "" {
		return nil, nil, ""
	}
	remote.Lock()
	if !fresh && remote.url == url && time.Since(remote.fetched) < 30*time.Second {
		defer remote.Unlock()
		return remote.items, remote.me, remote.err
	}
	remote.Unlock()
	var r struct {
		Items []remoteItem `json:"items"`
		Me    *remoteMe    `json:"me"`
	}
	err := a.remoteCall("GET", "/list", nil, 1, &r)
	remote.Lock()
	defer remote.Unlock()
	remote.url, remote.fetched = url, time.Now()
	if err != nil {
		remote.err = err.Error()
		return remote.items, remote.me, remote.err
	}
	remote.items, remote.me, remote.err = r.Items, r.Me, ""
	return remote.items, remote.me, ""
}

func (a *App) remoteDirty() {
	remote.Lock()
	remote.fetched = time.Time{}
	remote.Unlock()
}

func daysAgo(unix int64) int {
	d := int(time.Since(time.Unix(unix, 0)).Hours() / 24)
	if d < 0 {
		return 0
	}
	return d
}

// remoteStories and remotePacks turn the server's uploads into browser entries.
func remoteStory(it remoteItem, me *remoteMe) (CommunityStory, bool) {
	var b Backstory
	if it.Kind != "story" || json.Unmarshal(it.Data, &b) != nil || len(b.Roles) == 0 {
		return CommunityStory{}, false
	}
	b.ID = "r-" + it.ID
	b.Name = it.Name
	b.Tags = it.Tags
	b.Mod, b.Locked, b.Custom = "", false, false
	b.Building = buildingCategory(b.Building).ID
	return CommunityStory{Backstory: b, Author: it.Author, AuthorID: it.AuthorID, Downloads: it.Downloads, Likes: it.Likes, Days: daysAgo(it.Created),
		Liked: it.Liked, CommentCount: it.Comments, Remote: true, Mine: me != nil && me.ID == it.AuthorID}, true
}

func remotePack(it remoteItem, me *remoteMe) (CommunityPack, bool) {
	var d remotePackData
	if it.Kind != "pack" || json.Unmarshal(it.Data, &d) != nil || len(d.Mods) == 0 {
		return CommunityPack{}, false
	}
	tags := it.Tags
	if tags == nil {
		tags = []string{}
	}
	return CommunityPack{ID: "r-" + it.ID, Name: it.Name, Author: it.Author, AuthorID: it.AuthorID, About: it.About, Tags: tags, Mods: d.Mods,
		Downloads: it.Downloads, Likes: it.Likes, Days: daysAgo(it.Created), Liked: it.Liked, CommentCount: it.Comments,
		Remote: true, Mine: me != nil && me.ID == it.AuthorID}, true
}

func (a *App) findRemote(id string) *remoteItem {
	items, _, _ := a.remoteList(false)
	for i := range items {
		if "r-"+items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

// cleanTags keeps tags short, lowercase and few, like the server does.
func cleanTags(tags []string) []string {
	out := []string{}
	for _, t := range tags {
		t = strings.Join(strings.Fields(strings.ToLower(t)), " ")
		if len(t) > 20 {
			t = t[:20]
		}
		if t != "" && !contains(out, t) {
			out = append(out, t)
		}
		if len(out) >= 8 {
			break
		}
	}
	return out
}
