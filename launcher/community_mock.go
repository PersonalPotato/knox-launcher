package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"
)

// EZ_COMMUNITY=mock runs a pretend community server inside the launcher, filled with the made-up
// uploads as if real people posted them (no "Example" labels). Uploads, likes, comments, reports
// and deletes all work and last until the launcher closes. For the dev build and trailer footage.

type mockComment struct {
	ID       int64  `json:"id"`
	Item     string `json:"-"`
	Author   string `json:"author"`
	AuthorID string `json:"authorId"`
	Text     string `json:"text"`
	Created  int64  `json:"created"`
}

var mockServer = struct {
	sync.Mutex
	items    []remoteItem
	comments []mockComment
	likes    map[string]bool
	seq      int64
	seeded   bool
}{likes: map[string]bool{}}

func fakeUserID(name string) string {
	h := uint64(0)
	for _, r := range name {
		h = h*31 + uint64(r)
	}
	return fmt.Sprint(76561197900000000 + h%100000000)
}

func mockSeed() {
	if mockServer.seeded {
		return
	}
	mockServer.seeded = true
	now := time.Now().Unix()
	add := func(id, kind, name, author, about string, tags []string, data any, dl, likes, days int, cs []CommunityComment) {
		raw, _ := json.Marshal(data)
		created := now - int64(days)*86400 - int64(len(name))*937
		mockServer.items = append(mockServer.items, remoteItem{ID: id, Kind: kind, Name: name, Author: author, AuthorID: fakeUserID(author), About: about,
			Tags: tags, Data: raw, Downloads: dl, Likes: likes, Comments: len(cs), Created: created, Updated: created})
		for _, c := range cs {
			mockServer.seq++
			mockServer.comments = append(mockServer.comments, mockComment{ID: mockServer.seq, Item: id, Author: c.Author, AuthorID: fakeUserID(c.Author), Text: c.Text, Created: now - int64(c.Days)*86400 - mockServer.seq*611})
		}
	}
	for _, s := range communityStories {
		b := s.Backstory
		add(strings.TrimPrefix(b.ID, "c-"), "story", b.Name, s.Author, b.Story, b.Tags, b, s.Downloads, s.Likes, s.Days, s.Comments)
	}
	for _, p := range communityPacks {
		mods := make([]CommunityMod, len(p.Mods))
		copy(mods, p.Mods)
		tags := make([]string, len(p.Tags))
		for i, t := range p.Tags {
			tags[i] = strings.ToLower(t)
		}
		add(strings.TrimPrefix(p.ID, "p-"), "pack", p.Name, p.Author, p.About, tags, remotePackData{Mods: mods}, p.Downloads, p.Likes, p.Days, p.Comments)
	}
}

// mockCall answers like worker.js does.
func (a *App) mockCall(method, path string, body any, out any) error {
	mockServer.Lock()
	defer mockServer.Unlock()
	mockSeed()
	me := remoteMe{ID: fmt.Sprint(a.steam.MyID()), Name: a.steam.MyName(), Admin: true}
	var b map[string]any
	if body != nil {
		raw, _ := json.Marshal(body)
		_ = json.Unmarshal(raw, &b)
	}
	str := func(k string) string { s, _ := b[k].(string); return s }
	find := func(id string) int {
		for i := range mockServer.items {
			if mockServer.items[i].ID == id {
				return i
			}
		}
		return -1
	}
	u, _ := url.Parse(path)
	var res any = map[string]any{"ok": true}
	switch method + " " + u.Path {
	case "GET /list":
		items := make([]remoteItem, len(mockServer.items))
		copy(items, mockServer.items)
		for i := range items {
			items[i].Liked = mockServer.likes[items[i].ID]
		}
		res = map[string]any{"items": items, "me": me}
	case "GET /comments":
		list := []mockComment{}
		for i := len(mockServer.comments) - 1; i >= 0; i-- {
			if c := mockServer.comments[i]; c.Item == u.Query().Get("item") {
				list = append(list, c)
			}
		}
		res = map[string]any{"comments": list}
	case "POST /upload":
		data, _ := json.Marshal(b["data"])
		var tags []string
		if t, ok := b["tags"].([]any); ok {
			for _, x := range t {
				tags = append(tags, fmt.Sprint(x))
			}
		}
		now := time.Now().Unix()
		if i := find(str("id")); i >= 0 {
			it := &mockServer.items[i]
			it.Name, it.About, it.Tags, it.Data, it.Updated = str("name"), str("about"), cleanTags(tags), data, now
			res = map[string]any{"id": it.ID}
			break
		}
		mockServer.seq++
		id := fmt.Sprintf("m%05d", mockServer.seq)
		mockServer.items = append([]remoteItem{{ID: id, Kind: str("kind"), Name: str("name"), Author: me.Name, AuthorID: me.ID, About: str("about"),
			Tags: cleanTags(tags), Data: data, Created: now, Updated: now}}, mockServer.items...)
		res = map[string]any{"id": id}
	case "POST /delete":
		if i := find(str("id")); i >= 0 {
			mockServer.items = append(mockServer.items[:i], mockServer.items[i+1:]...)
		}
	case "POST /like":
		id := str("id")
		if i := find(id); i >= 0 {
			mockServer.likes[id] = !mockServer.likes[id]
			if mockServer.likes[id] {
				mockServer.items[i].Likes++
			} else {
				mockServer.items[i].Likes--
			}
		}
	case "POST /download":
		if i := find(str("id")); i >= 0 {
			mockServer.items[i].Downloads++
		}
	case "POST /comment":
		i := find(str("id"))
		if i < 0 {
			return errors.New("that upload is gone")
		}
		if strings.TrimSpace(str("text")) == "" {
			return errors.New("write something first")
		}
		mockServer.seq++
		mockServer.comments = append(mockServer.comments, mockComment{ID: mockServer.seq, Item: str("id"), Author: me.Name, AuthorID: me.ID, Text: str("text"), Created: time.Now().Unix()})
		mockServer.items[i].Comments++
	case "POST /uncomment":
		id, _ := b["id"].(float64)
		for i, c := range mockServer.comments {
			if c.ID == int64(id) {
				mockServer.comments = append(mockServer.comments[:i], mockServer.comments[i+1:]...)
				if j := find(c.Item); j >= 0 {
					mockServer.items[j].Comments--
				}
				break
			}
		}
	case "POST /report", "POST /ban":
	default:
		return errors.New("not found")
	}
	if out != nil {
		raw, _ := json.Marshal(res)
		return json.Unmarshal(raw, out)
	}
	return nil
}
