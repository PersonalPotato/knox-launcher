package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Backup struct {
	World string `json:"world"`
	File  string `json:"file"`
	Time  int64  `json:"time"`
	Size  int64  `json:"size"`
}

func (a *App) backupDir(world string) string {
	return filepath.Join(a.paths.Data, "backups", clean(world))
}

// backupWorld zips Saves/Multiplayer/<world> and <world>_player, keeping the newest few.
func (a *App) backupWorld(world string) (string, error) {
	dir := a.backupDir(world)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	name := filepath.Join(dir, time.Now().Format("2006-01-02_15-04-05")+".zip")
	f, err := os.Create(name)
	if err != nil {
		return "", err
	}
	zw := zip.NewWriter(f)
	for _, folder := range []string{world, world + "_player"} {
		root := filepath.Join(a.paths.savesDir(), folder)
		if !exists(root) {
			continue
		}
		err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(a.paths.savesDir(), path)
			h, _ := zip.FileInfoHeader(info)
			h.Name = filepath.ToSlash(rel)
			h.Method = zip.Deflate
			w, err := zw.CreateHeader(h)
			if err != nil {
				return err
			}
			src, err := os.Open(path)
			if err != nil {
				return err
			}
			defer src.Close()
			_, err = io.Copy(w, src)
			return err
		})
		if err != nil {
			zw.Close()
			f.Close()
			os.Remove(name)
			return "", err
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	a.pruneBackups(world)
	return name, nil
}

func (a *App) listBackups(world string) []Backup {
	var out []Backup
	entries, _ := os.ReadDir(a.backupDir(world))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".zip") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Backup{World: world, File: e.Name(), Time: info.ModTime().Unix(), Size: info.Size()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	return out
}

func (a *App) pruneBackups(world string) {
	keep := a.cfg.Get().KeepBackups
	list := a.listBackups(world)
	for i := keep; i < len(list); i++ {
		os.Remove(filepath.Join(a.backupDir(world), list[i].File))
	}
}

// restoreBackup puts a backup back. The current save is moved aside, not deleted.
func (a *App) restoreBackup(world, file string) error {
	if gameRunning() {
		return fmt.Errorf("close Project Zomboid first")
	}
	src := filepath.Join(a.backupDir(world), filepath.Base(file))
	zr, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer zr.Close()
	stamp := time.Now().Format("2006-01-02_15-04-05")
	for _, folder := range []string{world, world + "_player"} {
		cur := filepath.Join(a.paths.savesDir(), folder)
		if exists(cur) {
			aside := filepath.Join(a.paths.Data, "replaced", folder+"_"+stamp)
			_ = os.MkdirAll(filepath.Dir(aside), 0755)
			if err := os.Rename(cur, aside); err != nil {
				return fmt.Errorf("couldn't move the current save aside: %v", err)
			}
		}
	}
	base := a.paths.savesDir()
	for _, zf := range zr.File {
		target := filepath.Join(base, filepath.FromSlash(zf.Name))
		if !strings.HasPrefix(target, filepath.Clean(base)+string(os.PathSeparator)) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		r, err := zf.Open()
		if err != nil {
			return err
		}
		w, err := os.Create(target)
		if err != nil {
			r.Close()
			return err
		}
		_, err = io.Copy(w, r)
		w.Close()
		r.Close()
		if err != nil {
			return err
		}
		_ = os.Chtimes(target, zf.Modified, zf.Modified)
	}
	return nil
}
