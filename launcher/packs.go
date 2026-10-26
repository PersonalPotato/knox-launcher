package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
)

// Mod packs can be shared as a code (copy and paste) or straight into the lobby.

type PackMod struct {
	ID string `json:"i"`
	WS string `json:"w,omitempty"` // Workshop item, so a friend can subscribe to what they lack
}

type SharedPack struct {
	Name string    `json:"n"`
	Mods []PackMod `json:"m"`
}

const packCodePrefix = "KNOXPACK1:"

func (a *App) sharedPack(name string) (SharedPack, error) {
	cfg := a.cfg.Get()
	ids, ok := cfg.Packs[name]
	if !ok {
		return SharedPack{}, errors.New("no pack called " + name)
	}
	sp := SharedPack{Name: name}
	c := a.catalog()
	for _, id := range ids {
		pm := PackMod{ID: id}
		if c != nil {
			if m := c.mod(id); m != nil {
				pm.WS = m.WorkshopID
			}
		}
		sp.Mods = append(sp.Mods, pm)
	}
	return sp, nil
}

func packCode(sp SharedPack) string {
	b, _ := json.Marshal(sp)
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(b)
	_ = zw.Close()
	return packCodePrefix + base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

func parsePackCode(code string) (SharedPack, error) {
	var sp SharedPack
	code = strings.TrimSpace(code)
	if i := strings.Index(code, packCodePrefix); i >= 0 {
		code = code[i+len(packCodePrefix):]
	} else if strings.HasPrefix(code, "{") {
		err := json.Unmarshal([]byte(code), &sp)
		return sp, err
	} else {
		return sp, errors.New("that isn't a pack code")
	}
	if f := strings.Fields(code); len(f) > 0 {
		code = f[0]
	}
	raw, err := base64.RawURLEncoding.DecodeString(code)
	if err != nil {
		return sp, errors.New("that pack code is cut off or broken")
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return sp, errors.New("that pack code is cut off or broken")
	}
	b, err := io.ReadAll(io.LimitReader(zr, 1<<20))
	if err != nil {
		return sp, errors.New("that pack code is cut off or broken")
	}
	if err := json.Unmarshal(b, &sp); err != nil || len(sp.Mods) == 0 {
		return sp, errors.New("that pack code is empty")
	}
	for i := range sp.Mods {
		if sp.Mods[i].ID == oldKnoxLinkID {
			sp.Mods[i].ID = knoxLinkID
		}
	}
	return sp, nil
}

// importPack saves a shared pack under a free name and says which mods you don't have.
func (a *App) importPack(sp SharedPack) (map[string]any, error) {
	name := strings.TrimSpace(clean(sp.Name))
	if name == "" {
		name = "Shared pack"
	}
	c := a.catalog()
	var ids []string
	var missing []PackMod
	for _, m := range sp.Mods {
		if m.ID == "" {
			continue
		}
		ids = append(ids, m.ID)
		if c == nil || c.mod(m.ID) == nil {
			missing = append(missing, m)
		}
	}
	sort.Strings(ids)
	cfg := a.cfg.Get()
	final := name
	for n := 2; ; n++ {
		if _, taken := cfg.Packs[final]; !taken {
			break
		}
		final = name + " " + strconv.Itoa(n)
	}
	err := a.cfg.Update(func(c *Config) { c.Packs[final] = ids })
	return map[string]any{"name": final, "count": len(ids), "missing": missing}, err
}

