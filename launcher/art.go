package main

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

// The game's own title screen art (media/ui/Title*.png, four tiles of a 1920x1080 picture),
// put back together for the launcher's backgrounds. "lightning" is the flash frame.

var artCache struct {
	sync.Mutex
	img map[string][]byte
}

func (a *App) titleArt(kind string) []byte {
	artCache.Lock()
	defer artCache.Unlock()
	if b, ok := artCache.img[kind]; ok {
		return b
	}
	names := []string{"Title.png", "Title2.png", "Title3.png", "Title4.png"}
	if kind == "lightning" {
		names = []string{"Title_lightning.png", "Title_lightning2.png", "Title_lightning3.png", "Title_lightning4.png"}
	}
	at := []image.Point{{0, 0}, {1024, 0}, {0, 1024}, {1024, 1024}}
	canvas := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	ok := false
	for i, n := range names {
		f, err := os.Open(filepath.Join(a.paths.Game, "media", "ui", n))
		if err != nil {
			continue
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			continue
		}
		draw.Draw(canvas, img.Bounds().Add(at[i]), img, img.Bounds().Min, draw.Src)
		ok = true
	}
	var out []byte
	if ok {
		var buf bytes.Buffer
		if jpeg.Encode(&buf, canvas, &jpeg.Options{Quality: 85}) == nil {
			out = buf.Bytes()
		}
	}
	if artCache.img == nil {
		artCache.img = map[string][]byte{}
	}
	artCache.img[kind] = out
	return out
}

func (s *Server) art(w http.ResponseWriter, r *http.Request) {
	kind := "title"
	if r.URL.Query().Get("k") == "lightning" {
		kind = "lightning"
	}
	b := s.app.titleArt(kind)
	if len(b) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "max-age=86400")
	_, _ = w.Write(b)
}
