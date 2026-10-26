package main

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// The vanilla map: image tiles (pyramid.zip, the same pictures as the in-game map)
// and building outlines (worldmap.xml), used by the spawn picker.

type WorldMap struct {
	once      sync.Once
	dir       string
	zip       *zip.ReadCloser
	files     map[string]*zip.File
	bounds    [4]int
	buildings []Building
	err       error
}

type Building struct {
	Points                 [][2]int `json:"points"`
	Kind                   string   `json:"kind"`
	minX, minY, maxX, maxY int
}

func newWorldMap(game string) *WorldMap {
	return &WorldMap{dir: filepath.Join(game, "media", "maps", "Muldraugh, KY")}
}

func (w *WorldMap) load() error {
	w.once.Do(func() {
		z, err := zip.OpenReader(filepath.Join(w.dir, "pyramid.zip"))
		if err != nil {
			w.err = fmt.Errorf("map pictures not found: %v", err)
			return
		}
		w.zip = z
		w.files = map[string]*zip.File{}
		for _, f := range z.File {
			w.files[f.Name] = f
		}
		w.bounds = [4]int{0, 0, 19968, 16128}
		if f, ok := w.files["pyramid.txt"]; ok {
			if r, err := f.Open(); err == nil {
				b, _ := io.ReadAll(r)
				r.Close()
				for _, line := range strings.Split(string(b), "\n") {
					if strings.HasPrefix(line, "bounds=") {
						var x0, y0, x1, y1 int
						if _, err := fmt.Sscanf(strings.TrimSpace(line[7:]), "%d %d %d %d", &x0, &y0, &x1, &y1); err == nil {
							w.bounds = [4]int{x0, y0, x1, y1}
						}
					}
				}
			}
		}
		w.buildings = loadBuildings(filepath.Join(w.dir, "worldmap.xml"))
	})
	return w.err
}

func (w *WorldMap) Info() (map[string]any, error) {
	if err := w.load(); err != nil {
		return nil, err
	}
	levels := 0
	for name := range w.files {
		if i := strings.Index(name, "/"); i > 0 {
			if n, err := strconv.Atoi(name[:i]); err == nil && n+1 > levels {
				levels = n + 1
			}
		}
	}
	return map[string]any{"bounds": w.bounds, "levels": levels, "tile": 256}, nil
}

func (w *WorldMap) Tile(z, x, y int) ([]byte, error) {
	if err := w.load(); err != nil {
		return nil, err
	}
	f, ok := w.files[fmt.Sprintf("%d/tile%dx%d.png", z, x, y)]
	if !ok {
		return nil, os.ErrNotExist
	}
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

type xmlPoint struct {
	X int `xml:"x,attr"`
	Y int `xml:"y,attr"`
}

type xmlFeature struct {
	Geometry struct {
		Type   string `xml:"type,attr"`
		Coords []struct {
			Points []xmlPoint `xml:"point"`
		} `xml:"coordinates"`
	} `xml:"geometry"`
	Props []struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	} `xml:"properties>property"`
}

func loadBuildings(path string) []Building {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	dec := xml.NewDecoder(f)
	var out []Building
	cellX, cellY := 0, 0
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch se.Name.Local {
		case "cell":
			for _, a := range se.Attr {
				if a.Name.Local == "x" {
					cellX, _ = strconv.Atoi(a.Value)
				} else if a.Name.Local == "y" {
					cellY, _ = strconv.Atoi(a.Value)
				}
			}
		case "feature":
			var feat xmlFeature
			if err := dec.DecodeElement(&feat, &se); err != nil || feat.Geometry.Type != "Polygon" || len(feat.Geometry.Coords) == 0 {
				continue
			}
			kind, isBuilding := "", false
			for _, p := range feat.Props {
				if p.Name == "building" {
					isBuilding = true
					if kind == "" {
						kind = p.Value
					}
				}
				if p.Name == "RoomTone" {
					kind = p.Value
				}
			}
			if !isBuilding {
				continue
			}
			b := Building{Kind: kind, minX: 1 << 30, minY: 1 << 30, maxX: -1 << 30, maxY: -1 << 30}
			for _, pt := range feat.Geometry.Coords[0].Points {
				x, y := cellX*300+pt.X, cellY*300+pt.Y
				b.Points = append(b.Points, [2]int{x, y})
				b.minX, b.minY = min(b.minX, x), min(b.minY, y)
				b.maxX, b.maxY = max(b.maxX, x), max(b.maxY, y)
			}
			if len(b.Points) >= 3 {
				out = append(out, b)
			}
		}
	}
	return out
}

func insidePoly(pts [][2]int, x, y float64) bool {
	in := false
	for i, j := 0, len(pts)-1; i < len(pts); j, i = i, i+1 {
		xi, yi := float64(pts[i][0]), float64(pts[i][1])
		xj, yj := float64(pts[j][0]), float64(pts[j][1])
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			in = !in
		}
	}
	return in
}

// BuildingAt finds the building under (x, y), or the nearest one within 40 squares,
// and a spot inside it to spawn on.
func (w *WorldMap) BuildingAt(x, y int) (*Building, [2]int, bool) {
	if err := w.load(); err != nil {
		return nil, [2]int{}, false
	}
	var best *Building
	bestD := 40 * 40
	for i := range w.buildings {
		b := &w.buildings[i]
		if x >= b.minX && x <= b.maxX && y >= b.minY && y <= b.maxY && insidePoly(b.Points, float64(x)+0.5, float64(y)+0.5) {
			return b, [2]int{x, y}, true
		}
		cx, cy := (b.minX+b.maxX)/2, (b.minY+b.maxY)/2
		d := (cx-x)*(cx-x) + (cy-y)*(cy-y)
		if d < bestD {
			best, bestD = b, d
		}
	}
	if best == nil {
		return nil, [2]int{}, false
	}
	// a square inside it: try the middle, then scan
	cx, cy := (best.minX+best.maxX)/2, (best.minY+best.maxY)/2
	if insidePoly(best.Points, float64(cx)+0.5, float64(cy)+0.5) {
		return best, [2]int{cx, cy}, true
	}
	for yy := best.minY + 1; yy < best.maxY; yy++ {
		for xx := best.minX + 1; xx < best.maxX; xx++ {
			if insidePoly(best.Points, float64(xx)+0.5, float64(yy)+0.5) {
				return best, [2]int{xx, yy}, true
			}
		}
	}
	return nil, [2]int{}, false
}

// RandomBuilding picks a building whose kind is one of tones, within radius of (cx, cy) when
// radius > 0, and a square well inside it to stand on.
func (w *WorldMap) RandomBuilding(tones []string, cx, cy, radius float64) (*Building, [2]int, error) {
	if err := w.load(); err != nil {
		return nil, [2]int{}, err
	}
	want := map[string]bool{}
	for _, t := range tones {
		want[strings.ToLower(t)] = true
	}
	var pool []*Building
	for i := range w.buildings {
		b := &w.buildings[i]
		if !want[strings.ToLower(b.Kind)] {
			continue
		}
		if radius > 0 {
			bx, by := float64(b.minX+b.maxX)/2, float64(b.minY+b.maxY)/2
			if (bx-cx)*(bx-cx)+(by-cy)*(by-cy) > radius*radius {
				continue
			}
		}
		pool = append(pool, b)
	}
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	for _, b := range pool {
		if spot, ok := b.innerSquare(); ok {
			return b, spot, nil
		}
	}
	return nil, [2]int{}, errors.New("no building of that type found")
}

// innerSquare finds a square inside the outline with its neighbours inside too, so nobody spawns in a wall.
func (b *Building) innerSquare() ([2]int, bool) {
	in := func(x, y int) bool { return insidePoly(b.Points, float64(x)+0.5, float64(y)+0.5) }
	good := func(x, y int) bool { return in(x, y) && in(x-1, y) && in(x+1, y) && in(x, y-1) && in(x, y+1) }
	cx, cy := (b.minX+b.maxX)/2, (b.minY+b.maxY)/2
	if good(cx, cy) {
		return [2]int{cx, cy}, true
	}
	for tries := 0; tries < 200; tries++ {
		x := b.minX + rand.Intn(max(1, b.maxX-b.minX))
		y := b.minY + rand.Intn(max(1, b.maxY-b.minY))
		if good(x, y) {
			return [2]int{x, y}, true
		}
	}
	return [2]int{}, false
}
