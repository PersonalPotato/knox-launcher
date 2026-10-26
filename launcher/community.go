package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// Your own stories (made in the story editor or downloaded from the community) and the community
// browser for stories and mod packs. The community uploads are made up for now: there is no
// server yet, so "uploading" only lists the item under Your uploads on this PC.

type Upload struct {
	Kind   string `json:"kind"` // story | pack
	ID     string `json:"id"`   // story id or pack name
	Name   string `json:"name"`
	Posted int64  `json:"posted"`
	Remote string `json:"remote,omitempty"` // id on the community server
}

type CommunityComment struct {
	Author string `json:"author"`
	Text   string `json:"text"`
	Days   int    `json:"days"`
}

type CommunityStory struct {
	Backstory
	Author       string             `json:"author"`
	Downloads    int                `json:"downloads"`
	Likes        int                `json:"likes"`
	Days         int                `json:"days"` // posted N days ago
	Comments     []CommunityComment `json:"comments,omitempty"`
	Have         bool               `json:"have"`
	Liked        bool               `json:"liked"`
	AuthorID     string             `json:"authorId,omitempty"`
	CommentCount int                `json:"commentCount"`
	Remote       bool               `json:"remote,omitempty"`  // from the community server (ids start with r-)
	Example      bool               `json:"example,omitempty"` // made up
	Mine         bool               `json:"mine,omitempty"`
}

type CommunityMod struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	WS   string `json:"ws,omitempty"`
	Have bool   `json:"have,omitempty"`
}

type CommunityPack struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Author       string             `json:"author"`
	About        string             `json:"about"`
	Tags         []string           `json:"tags"`
	Mods         []CommunityMod     `json:"mods"`
	Downloads    int                `json:"downloads"`
	Likes        int                `json:"likes"`
	Days         int                `json:"days"`
	Comments     []CommunityComment `json:"comments,omitempty"`
	Have         bool               `json:"have"` // a pack of yours has this name
	Liked        bool               `json:"liked"`
	AuthorID     string             `json:"authorId,omitempty"`
	CommentCount int                `json:"commentCount"`
	Remote       bool               `json:"remote,omitempty"`
	Example      bool               `json:"example,omitempty"`
	Mine         bool               `json:"mine,omitempty"`
}

// userStories returns your own stories, checked against this install like the built-in ones.
func (a *App) userStories() []Backstory {
	cfg := a.cfg.Get()
	out := make([]Backstory, 0, len(cfg.Stories))
	for _, b := range cfg.Stories {
		b.Custom = true
		out = append(out, b)
	}
	balanceRoles(a.catalog(), out)
	return out
}

func (a *App) userStory(id string) *Backstory {
	if id == "" {
		return nil
	}
	for _, b := range a.userStories() {
		if b.ID == id {
			return &b
		}
	}
	return nil
}

// storyByID finds a story of yours first, then a built-in one.
func (a *App) storyByID(id string) *Backstory {
	if b := a.userStory(id); b != nil {
		return b
	}
	return backstoryByID(id)
}

func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteByte('-')
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > 24 {
		out = out[:24]
	}
	if out == "" {
		out = "story"
	}
	return out
}

// tidyStory checks a story from the editor and fills in what's missing.
func tidyStory(b Backstory) (Backstory, error) {
	b.Name = strings.TrimSpace(b.Name)
	b.Story = strings.TrimSpace(b.Story)
	b.Tip = strings.TrimSpace(b.Tip)
	if b.Name == "" {
		return b, errors.New("give the story a name")
	}
	if len(b.Name) > 60 {
		b.Name = b.Name[:60]
	}
	var roles []Role
	for _, r := range b.Roles {
		r.Name = strings.TrimSpace(clean(r.Name))
		r.Note = strings.TrimSpace(r.Note)
		if r.Name == "" {
			continue
		}
		roles = append(roles, r)
	}
	if len(roles) == 0 {
		return b, errors.New("add at least one role")
	}
	seen := map[string]bool{}
	for _, r := range roles {
		if seen[strings.ToLower(r.Name)] {
			return b, errors.New("two roles are called " + r.Name)
		}
		seen[strings.ToLower(r.Name)] = true
	}
	b.Roles = roles
	if b.Min < 1 {
		b.Min = 1
	}
	if b.Max < b.Min {
		b.Max = b.Min
	}
	if b.Max > 16 {
		b.Max = 16
	}
	if b.Building == "" {
		b.Building = "house"
	}
	b.Building = buildingCategory(b.Building).ID
	b.Mod, b.Locked, b.Custom = "", false, false
	return b, nil
}

func (a *App) saveStory(b Backstory) (Backstory, error) {
	b, err := tidyStory(b)
	if err != nil {
		return b, err
	}
	if b.ID == "" || backstoryByID(b.ID) != nil {
		base := "my-" + slug(b.Name)
		b.ID = base
		for n := 2; a.userStory(b.ID) != nil; n++ {
			b.ID = fmt.Sprintf("%s-%d", base, n)
		}
	}
	err = a.cfg.Update(func(c *Config) {
		for i := range c.Stories {
			if c.Stories[i].ID == b.ID {
				c.Stories[i] = b
				return
			}
		}
		c.Stories = append(c.Stories, b)
	})
	return b, err
}

func (a *App) deleteStory(id string) error {
	return a.cfg.Update(func(c *Config) {
		for i := range c.Stories {
			if c.Stories[i].ID == id {
				c.Stories = append(c.Stories[:i], c.Stories[i+1:]...)
				break
			}
		}
		if c.Host.Backstory == id {
			c.Host.Backstory = ""
		}
		for i := range c.Uploads {
			if c.Uploads[i].Kind == "story" && c.Uploads[i].ID == id {
				c.Uploads = append(c.Uploads[:i], c.Uploads[i+1:]...)
				break
			}
		}
	})
}

// Story codes work like pack codes: copy one, paste it into a friend's launcher.

const storyCodePrefix = "KNOXSTORY1:"

func storyCode(b Backstory) string {
	b.Custom, b.Locked = false, false
	raw, _ := json.Marshal(b)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(raw)
	_ = zw.Close()
	return storyCodePrefix + base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

func parseStoryCode(code string) (Backstory, error) {
	var b Backstory
	code = strings.TrimSpace(code)
	i := strings.Index(code, storyCodePrefix)
	if i < 0 {
		return b, errors.New("that isn't a story code")
	}
	code = code[i+len(storyCodePrefix):]
	if f := strings.Fields(code); len(f) > 0 {
		code = f[0]
	}
	bad := errors.New("that story code is cut off or broken")
	raw, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil {
		return b, bad
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return b, bad
	}
	js, err := io.ReadAll(io.LimitReader(zr, 1<<20))
	if err != nil || json.Unmarshal(js, &b) != nil {
		return b, bad
	}
	return b, nil
}

// community lists the server's uploads and the made-up examples, with what you already have.
func (a *App) community(fresh bool) map[string]any {
	cfg := a.cfg.Get()
	mine := map[string]bool{}
	for _, b := range cfg.Stories {
		mine[b.ID] = true
	}
	items, me, rerr := a.remoteList(fresh)
	var stories []CommunityStory
	var packs []CommunityPack
	for _, it := range items {
		if cs, ok := remoteStory(it, me); ok {
			stories = append(stories, cs)
		} else if cp, ok := remotePack(it, me); ok {
			packs = append(packs, cp)
		}
	}
	if !cfg.HideExamples && a.communityURL() != "mock" { // the mock server already has them
		for _, cs := range communityStories {
			cs.Example = true
			cs.CommentCount = len(cs.Comments)
			if cfg.Liked[cs.ID] {
				cs.Liked = true
				cs.Likes++
			}
			stories = append(stories, cs)
		}
		for _, p := range communityPacks {
			p.Example = true
			p.CommentCount = len(p.Comments)
			if cfg.Liked[p.ID] {
				p.Liked = true
				p.Likes++
			}
			packs = append(packs, p)
		}
	}
	plain := make([]Backstory, len(stories))
	for i := range stories {
		plain[i] = stories[i].Backstory
	}
	balanceRoles(a.catalog(), plain)
	for i := range stories {
		stories[i].Backstory = plain[i]
		stories[i].Have = mine[stories[i].ID]
	}
	installed := map[string]bool{}
	if c := a.catalog(); c != nil {
		for _, m := range c.Mods {
			if !m.Removed {
				installed[m.ID] = true
			}
		}
	}
	for i := range packs {
		p := &packs[i]
		p.Mods = append([]CommunityMod{}, p.Mods...)
		for j := range p.Mods {
			p.Mods[j].Have = installed[p.Mods[j].ID]
		}
		_, p.Have = cfg.Packs[p.Name]
	}
	uploads := cfg.Uploads
	if uploads == nil {
		uploads = []Upload{}
	}
	if stories == nil {
		stories = []CommunityStory{}
	}
	if packs == nil {
		packs = []CommunityPack{}
	}
	return map[string]any{"stories": stories, "packs": packs, "uploads": uploads,
		"server": map[string]any{"on": a.communityURL() != "", "url": a.communityURL(), "error": rerr, "me": me}, "hideExamples": cfg.HideExamples}
}

func (a *App) communityStory(id string) *CommunityStory {
	if it := a.findRemote(id); it != nil {
		_, me, _ := a.remoteList(false)
		if cs, ok := remoteStory(*it, me); ok {
			return &cs
		}
	}
	for i := range communityStories {
		if communityStories[i].ID == id {
			return &communityStories[i]
		}
	}
	return nil
}

func (a *App) communityPack(id string) *CommunityPack {
	if it := a.findRemote(id); it != nil {
		if cp, ok := remotePack(*it, nil); ok {
			return &cp
		}
	}
	for i := range communityPacks {
		if communityPacks[i].ID == id {
			return &communityPacks[i]
		}
	}
	return nil
}

func (a *App) communityCall(method string, args json.RawMessage) (any, bool, error) {
	var p struct {
		ID     string     `json:"id"`
		Kind   string     `json:"kind"`
		Name   string     `json:"name"`
		Code   string     `json:"code"`
		Story  *Backstory `json:"story"`
		Fresh  bool       `json:"fresh"`
		Text   string     `json:"text"`
		About  string     `json:"about"`
		Tags   []string   `json:"tags"`
		User   string     `json:"user"`
		URL    *string    `json:"url"`
		Hide   *bool      `json:"hideExamples"`
		Number int64      `json:"number"`
	}
	switch method {
	case "community", "saveStory", "deleteStory", "storyCode", "importStory", "getStory", "getPack", "upload", "unupload", "like",
		"comments", "comment", "uncomment", "report", "ban", "removeUpload", "setCommunity":
	default:
		return nil, false, nil
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, true, err
		}
	}
	switch method {
	case "community":
		return a.community(p.Fresh), true, nil
	case "saveStory":
		if p.Story == nil {
			return nil, true, errors.New("no story")
		}
		b, err := a.saveStory(*p.Story)
		if err == nil && a.cfg.Get().Host.Backstory == b.ID {
			go a.publishHostInfo()
		}
		return b, true, err
	case "deleteStory":
		err := a.deleteStory(p.ID)
		go a.publishHostInfo()
		return nil, true, err
	case "storyCode":
		b := a.storyByID(p.ID)
		if b == nil {
			if cs := a.communityStory(p.ID); cs != nil {
				b = &cs.Backstory
			}
		}
		if b == nil {
			return nil, true, errors.New("no story with that id")
		}
		return storyCode(*b), true, nil
	case "importStory":
		b, err := parseStoryCode(p.Code)
		if err != nil {
			return nil, true, err
		}
		if a.userStory(b.ID) != nil {
			b.ID = "" // a copy, not an overwrite
		}
		b, err = a.saveStory(b)
		return b, true, err
	case "getStory":
		cs := a.communityStory(p.ID)
		if cs == nil {
			return nil, true, errors.New("that story is gone")
		}
		b := cs.Backstory
		b.Author = cs.Author
		if a.userStory(b.ID) != nil {
			return b, true, nil
		}
		if cs.Remote {
			go a.remoteCall("POST", "/download", map[string]string{"id": strings.TrimPrefix(b.ID, "r-")}, 1, nil)
			a.remoteDirty()
		}
		err := a.cfg.Update(func(c *Config) { c.Stories = append(c.Stories, b) })
		return b, true, err
	case "getPack":
		cp := a.communityPack(p.ID)
		if cp == nil {
			return nil, true, errors.New("that pack is gone")
		}
		sp := SharedPack{Name: cp.Name}
		for _, m := range cp.Mods {
			sp.Mods = append(sp.Mods, PackMod{ID: m.ID, WS: m.WS})
		}
		r, err := a.importPack(sp)
		if err == nil && cp.Remote {
			go a.remoteCall("POST", "/download", map[string]string{"id": strings.TrimPrefix(cp.ID, "r-")}, 1, nil)
			a.remoteDirty()
		}
		return r, true, err
	case "upload":
		return a.uploadItem(p.Kind, p.ID, p.About, p.Tags)
	case "unupload":
		cfg := a.cfg.Get()
		for _, u := range cfg.Uploads {
			if u.Kind == p.Kind && u.ID == p.ID && u.Remote != "" {
				if err := a.remoteCall("POST", "/delete", map[string]string{"id": u.Remote}, 2, nil); err != nil {
					return nil, true, err
				}
				a.remoteDirty()
			}
		}
		err := a.cfg.Update(func(c *Config) {
			for i, u := range c.Uploads {
				if u.Kind == p.Kind && u.ID == p.ID {
					c.Uploads = append(c.Uploads[:i], c.Uploads[i+1:]...)
					return
				}
			}
		})
		return nil, true, err
	case "removeUpload": // yours or (for admins) anyone's, straight from the browser
		err := a.remoteCall("POST", "/delete", map[string]string{"id": strings.TrimPrefix(p.ID, "r-")}, 2, nil)
		if err == nil {
			_ = a.cfg.Update(func(c *Config) {
				for i, u := range c.Uploads {
					if "r-"+u.Remote == p.ID {
						c.Uploads = append(c.Uploads[:i], c.Uploads[i+1:]...)
						return
					}
				}
			})
		}
		a.remoteDirty()
		return nil, true, err
	case "comments":
		var r struct {
			Comments []struct {
				ID       int64  `json:"id"`
				Author   string `json:"author"`
				AuthorID string `json:"authorId"`
				Text     string `json:"text"`
				Created  int64  `json:"created"`
			} `json:"comments"`
		}
		err := a.remoteCall("GET", "/comments?item="+strings.TrimPrefix(p.ID, "r-"), nil, 0, &r)
		return r.Comments, true, err
	case "comment":
		err := a.remoteCall("POST", "/comment", map[string]string{"id": strings.TrimPrefix(p.ID, "r-"), "text": p.Text}, 2, nil)
		a.remoteDirty()
		return nil, true, err
	case "uncomment":
		err := a.remoteCall("POST", "/uncomment", map[string]int64{"id": p.Number}, 2, nil)
		a.remoteDirty()
		return nil, true, err
	case "report":
		err := a.remoteCall("POST", "/report", map[string]string{"id": strings.TrimPrefix(p.ID, "r-"), "reason": p.Text}, 2, nil)
		a.remoteDirty()
		return nil, true, err
	case "ban":
		err := a.remoteCall("POST", "/ban", map[string]string{"user": p.User}, 2, nil)
		a.remoteDirty()
		return nil, true, err
	case "setCommunity":
		err := a.cfg.Update(func(c *Config) {
			if p.URL != nil {
				c.CommunityURL = strings.TrimSpace(*p.URL)
			}
			if p.Hide != nil {
				c.HideExamples = *p.Hide
			}
		})
		a.remoteDirty()
		return nil, true, err
	case "like":
		if strings.HasPrefix(p.ID, "r-") {
			err := a.remoteCall("POST", "/like", map[string]string{"id": strings.TrimPrefix(p.ID, "r-")}, 2, nil)
			a.remoteDirty()
			return nil, true, err
		}
		err := a.cfg.Update(func(c *Config) {
			if c.Liked == nil {
				c.Liked = map[string]bool{}
			}
			if c.Liked[p.ID] {
				delete(c.Liked, p.ID)
			} else {
				c.Liked[p.ID] = true
			}
		})
		return nil, true, err
	}
	return nil, false, nil
}

// uploadItem puts one of your stories or packs on the community server (or, with no server, just
// lists it on this PC). Uploading again updates it.
func (a *App) uploadItem(kind, id, about string, tags []string) (any, bool, error) {
	cfg := a.cfg.Get()
	var prev *Upload
	for i := range cfg.Uploads {
		if cfg.Uploads[i].Kind == kind && cfg.Uploads[i].ID == id {
			prev = &cfg.Uploads[i]
		}
	}
	var name string
	var data any
	switch kind {
	case "story":
		var raw *Backstory
		for i := range cfg.Stories {
			if cfg.Stories[i].ID == id {
				raw = &cfg.Stories[i]
			}
		}
		if raw == nil {
			return nil, true, errors.New("that story isn't yours")
		}
		name, data = raw.Name, raw
		if about == "" {
			about = raw.Story
		}
		if tags == nil {
			tags = raw.Tags
		}
	case "pack":
		ids, ok := cfg.Packs[id]
		if !ok {
			return nil, true, errors.New("no pack called " + id)
		}
		var d remotePackData
		c := a.catalog()
		for _, mid := range ids {
			m := CommunityMod{ID: mid, Name: mid}
			if c != nil {
				if cm := c.mod(mid); cm != nil {
					m.Name, m.WS = cm.Name, cm.WorkshopID
				}
			}
			d.Mods = append(d.Mods, m)
		}
		name, data = id, d
	default:
		return nil, true, errors.New("can't upload that")
	}
	rec := Upload{Kind: kind, ID: id, Name: name, Posted: time.Now().Unix()}
	if prev != nil {
		rec.Posted, rec.Remote = prev.Posted, prev.Remote
	}
	if a.communityURL() != "" {
		body := map[string]any{"kind": kind, "name": name, "about": about, "tags": cleanTags(tags), "data": data}
		if rec.Remote != "" {
			body["id"] = rec.Remote
		}
		var r struct {
			ID string `json:"id"`
		}
		err := a.remoteCall("POST", "/upload", body, 2, &r)
		if err != nil && rec.Remote != "" && strings.Contains(err.Error(), "gone") {
			delete(body, "id") // taken down on the server: upload it fresh
			err = a.remoteCall("POST", "/upload", body, 2, &r)
		}
		if err != nil {
			return nil, true, err
		}
		rec.Remote = r.ID
		a.remoteDirty()
	}
	err := a.cfg.Update(func(c *Config) {
		for i := range c.Uploads {
			if c.Uploads[i].Kind == kind && c.Uploads[i].ID == id {
				c.Uploads[i] = rec
				return
			}
		}
		c.Uploads = append(c.Uploads, rec)
	})
	return rec, true, err
}
