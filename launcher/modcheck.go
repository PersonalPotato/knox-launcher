package main

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// The host's game and server load a mod from Zomboid\Workshop (your own upload) or Zomboid\mods
// when it's there, but friends download the Workshop copy. When their Lua or scripts differ, the
// game kicks friends ("Lua checksums do not match"), so START stops first.
func (a *App) ownCopyMismatch(c *Catalog, ids []string) error {
	if c == nil || a.paths.Workshop == "" {
		return nil
	}
	for _, id := range ids {
		m := c.mod(id)
		if m == nil || m.WorkshopID == "" || (m.Source != "local" && m.Source != "upload") {
			continue
		}
		folder := filepath.Base(m.Dir)
		published := filepath.Join(a.paths.Workshop, m.WorkshopID, "mods", folder)
		if !exists(published) {
			log.Printf("mod check: %s has no Workshop copy installed, can't compare", m.Name)
			continue
		}
		// the game picks your upload folder ahead of Zomboid\mods
		own := m.Dir
		items, _ := os.ReadDir(filepath.Join(a.paths.Zomboid, "Workshop"))
		for _, it := range items {
			if p := filepath.Join(a.paths.Zomboid, "Workshop", it.Name(), "Contents", "mods", folder); exists(p) {
				own = p
				break
			}
		}
		if f := checksumDiff(own, published); f != "" {
			log.Printf("mod check: %s differs from the Workshop copy (%s vs %s): %s", m.Name, own, published, f)
			where, err := filepath.Rel(filepath.Dir(a.paths.Zomboid), own)
			if err != nil {
				where = own
			}
			where = strings.TrimSuffix(where, string(filepath.Separator)+filepath.Join("Contents", "mods", folder))
			return fmt.Errorf("Your copy of %s (%s) isn't the same as the Workshop one, so friends would get kicked. Upload it to the Workshop, then press START again.", m.Name, where)
		}
	}
	return nil
}

// checksumDiff: the first Lua or script file that differs between two copies of a mod, or "".
func checksumDiff(a, b string) string {
	ha, hb := checkedFiles(a), checkedFiles(b)
	for f, h := range ha {
		if hb[f] != h {
			return f
		}
	}
	for f := range hb {
		if _, ok := ha[f]; !ok {
			return f
		}
	}
	return ""
}

// checkedFiles: the files the game checksums (Lua and scripts), by path inside the mod folder
func checkedFiles(root string) map[string]string {
	out := map[string]string{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = strings.ToLower(filepath.ToSlash(rel))
		if !strings.HasSuffix(rel, ".lua") && !strings.Contains(rel, "media/scripts/") {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			out[rel] = "unreadable"
			return nil
		}
		h := sha256.New()
		_, _ = io.Copy(h, f)
		f.Close()
		out[rel] = fmt.Sprintf("%x", h.Sum(nil))
		return nil
	})
	return out
}
