package main

// ZombieBuddy asks before it loads a mod's Java code and remembers each answer by the jar's SHA-256
// in ~/.zombie_buddy/mod_approvals.json. The launcher shows the same question on the Mods page, and
// when the player answers there it writes the answer to that file, so the game doesn't ask again.
// Nothing is allowed unless the player clicks Allow.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var zbMu sync.Mutex

func zbApprovalsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".zombie_buddy", "mod_approvals.json")
}

// zbFile keeps every entry as raw JSON so fields we don't know survive a rewrite.
type zbFile struct {
	FormatVersion int              `json:"formatVersion"`
	Mods          []map[string]any `json:"mods"`
}

func readZB() zbFile {
	f := zbFile{FormatVersion: 2}
	if b, err := os.ReadFile(zbApprovalsPath()); err == nil {
		d := json.NewDecoder(bytes.NewReader(b))
		d.UseNumber() // Steam ids are bigger than a float64 holds exactly
		_ = d.Decode(&f)
	}
	if f.FormatVersion == 0 {
		f.FormatVersion = 2
	}
	return f
}

func (f zbFile) decisions() map[string]bool {
	out := map[string]bool{}
	for _, e := range f.Mods {
		h, _ := e["jar_hash"].(string)
		d, _ := e["decision"].(bool)
		if h != "" {
			out[h] = d
		}
	}
	return out
}

var hashCache sync.Map // path|mtime|size -> sha256

func fileSHA256(p string) string {
	st, err := os.Stat(p)
	if err != nil {
		return ""
	}
	key := fmt.Sprintf("%s|%d|%d", p, st.ModTime().UnixNano(), st.Size())
	if v, ok := hashCache.Load(key); ok {
		return v.(string)
	}
	fh, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer fh.Close()
	h := sha256.New()
	if _, err := io.Copy(h, fh); err != nil {
		return ""
	}
	sum := hex.EncodeToString(h.Sum(nil))
	hashCache.Store(key, sum)
	return sum
}

type JavaMod struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Author     string `json:"author"`
	WorkshopID string `json:"workshopID"`
	Jar        string `json:"jar"`
	Hash       string `json:"hash"`
	Status     string `json:"status"` // "allowed", "blocked" or "new"
}

// javaMods lists the Java mods among ids and what ZombieBuddy has on file for each one's jar.
func (a *App) javaMods(ids []string) []JavaMod {
	c := a.catalog()
	if c == nil {
		return nil
	}
	zbMu.Lock()
	dec := readZB().decisions()
	zbMu.Unlock()
	var out []JavaMod
	for _, id := range ids {
		m := c.mod(id)
		if m == nil || !m.Java || m.jar == "" {
			continue
		}
		jm := JavaMod{ID: m.ID, Name: m.Name, Author: m.Author, WorkshopID: m.WorkshopID, Jar: filepath.Base(m.jar), Hash: fileSHA256(m.jar), Status: "new"}
		if d, ok := dec[jm.Hash]; ok {
			jm.Status = map[bool]string{true: "allowed", false: "blocked"}[d]
		}
		out = append(out, jm)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// setJava records the player's answer for these mods' current jars, in ZombieBuddy's own format.
// A jar that changes (a mod update) gets a new hash, so ZombieBuddy asks about it again.
func (a *App) setJava(ids []string, allow bool) error {
	mods := a.javaMods(ids)
	zbMu.Lock()
	defer zbMu.Unlock()
	f := readZB()
	now := time.Now().Format("2006-01-02T15:04:05")
	for _, jm := range mods {
		if jm.Hash == "" {
			continue
		}
		found := false
		for _, e := range f.Mods {
			if h, _ := e["jar_hash"].(string); h == jm.Hash {
				e["decision"] = allow
				e["time"] = now
				found = true
			}
		}
		if !found {
			f.Mods = append(f.Mods, map[string]any{"id": jm.ID, "jar_hash": jm.Hash, "decision": allow, "time": now})
		}
	}
	b, err := jsonIndent(f)
	if err != nil {
		return err
	}
	p := zbApprovalsPath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	tmp := p + ".knox"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func jsonIndent(f zbFile) ([]byte, error) { return json.MarshalIndent(f, "", "  ") }
