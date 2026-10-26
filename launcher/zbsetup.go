package main

// ZombieBuddy check and setup. Java mods only load when the game starts with ZombieBuddy's agent:
// zbNative.dll and ZombieBuddy.jar in the game folder, and -agentlib:zbNative in ProjectZomboid64.json.
// The launcher starts ProjectZomboid64.exe itself, so Steam launch options don't count; the JSON does.
// This does the same steps as ZombieBuddy's own installer, only when the player presses the button.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	zbWorkshopID = "3619862853"
	zbLaunchArg  = "-agentlib:zbNative"
)

var zbFiles = []string{"zbNative.dll", "ZombieBuddy.jar"}

type ZBStatus struct {
	Needed     bool     `json:"needed"`     // a Java mod is in the run list
	Downloaded bool     `json:"downloaded"` // the Workshop item is on disk
	Files      string   `json:"files"`      // "ok", "missing" or "old"
	Launch     bool     `json:"launch"`     // ProjectZomboid64.json starts it
	Ready      bool     `json:"ready"`
	Update     bool     `json:"update"`   // the Workshop has a newer ZombieBuddy than the one downloaded
	Outdated   bool     `json:"outdated"` // Update, or the game folder has an older copy
	Busy       bool     `json:"busy"`     // Steam is downloading it right now
	Updated    int64    `json:"updated"`  // when the Workshop item was last updated (Unix time, 0 = not checked)
	Steps      []string `json:"steps"`    // what "Set up" would change
}

// The Workshop check runs in the background now and then; zbStatus only reads the last answer.
var zbCheck struct {
	sync.Mutex
	at      time.Time
	updated uint32
	handled uint32 // a Workshop version we already downloaded, in case Steam's install time never catches up
	running bool
}

const zbCheckEvery = 20 * time.Minute

// zbRemote returns when the Workshop item was last updated, as of the last check, and starts a new
// check when that one is old. force checks now (the player pressed "Check now").
func (a *App) zbRemote(force bool) uint32 {
	zbCheck.Lock()
	defer zbCheck.Unlock()
	if !zbCheck.running && (force || time.Since(zbCheck.at) > zbCheckEvery) {
		q, ok := a.steam.(interface{ ItemUpdated(uint64) (uint32, error) })
		if ok && a.steam.Ready() {
			zbCheck.running = true
			id, _ := strconv.ParseUint(zbWorkshopID, 10, 64)
			done := make(chan struct{})
			go func() {
				defer close(done)
				t, err := q.ItemUpdated(id)
				zbCheck.Lock()
				defer zbCheck.Unlock()
				zbCheck.running, zbCheck.at = false, time.Now()
				if err != nil {
					log.Printf("ZombieBuddy update check: %v", err)
					return
				}
				zbCheck.updated = t
			}()
			if force { // the player is waiting for this answer
				zbCheck.Unlock()
				select {
				case <-done:
				case <-time.After(15 * time.Second):
				}
				zbCheck.Lock()
			}
		}
	}
	return zbCheck.updated
}

// zbSource is ZombieBuddy's libs folder: the Workshop copy, or a copy in Zomboid/mods.
func (a *App) zbSource() string {
	for _, dir := range []string{
		filepath.Join(a.paths.Workshop, zbWorkshopID, "mods", "ZombieBuddy", "libs"),
		filepath.Join(a.paths.Zomboid, "mods", "ZombieBuddy", "libs"),
	} {
		if exists(filepath.Join(dir, "ZombieBuddy.jar")) && exists(filepath.Join(dir, "zbNative.dll")) {
			return dir
		}
	}
	return ""
}

func (a *App) zbLauncherJSON() string { return filepath.Join(a.paths.Game, "ProjectZomboid64.json") }

func zbHasArg(args []any) bool {
	for _, v := range args {
		if s, ok := v.(string); ok && (s == zbLaunchArg || strings.HasPrefix(s, zbLaunchArg+"=")) {
			return true
		}
	}
	return false
}

func readLauncherJSON(p string) (map[string]any, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var cfg map[string]any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("ProjectZomboid64.json is broken: %v", err)
	}
	return cfg, nil
}

// zbNewer says if the Workshop has a newer ZombieBuddy than the downloaded one.
func (a *App) zbNewer(force bool) (newer, busy bool, remote uint32) {
	st := a.steam
	if st == nil {
		return false, false, 0
	}
	id, _ := strconv.ParseUint(zbWorkshopID, 10, 64)
	state := st.ItemState(id)
	busy = state&(ItemDownloading|ItemDownloadPending) != 0
	remote = a.zbRemote(force)
	if state&ItemNeedsUpdate != 0 {
		return true, busy, remote
	}
	zbCheck.Lock()
	handled := zbCheck.handled
	zbCheck.Unlock()
	if _, local, ok := st.ItemInstallInfo(id); ok && remote != 0 && local != 0 && remote > local && remote > handled {
		return true, busy, remote
	}
	return false, busy, remote
}

// zbStatus checks ZombieBuddy for this mod list (nil = the host's own list).
func (a *App) zbStatus(ids []string, force bool) ZBStatus {
	var s ZBStatus
	if c := a.catalog(); c != nil {
		if ids == nil {
			ids, _ = a.hostModList()
		}
		for _, id := range ids {
			if m := c.mod(id); m != nil && m.Java {
				s.Needed = true
				break
			}
		}
	}
	src := a.zbSource()
	s.Downloaded = src != ""
	if !s.Downloaded {
		s.Steps = append(s.Steps, "Subscribe to ZombieBuddy on the Workshop and download it")
	} else {
		var remote uint32
		s.Update, s.Busy, remote = a.zbNewer(force)
		s.Updated = int64(remote)
		if s.Update {
			s.Steps = append(s.Steps, "Download the new ZombieBuddy from the Workshop")
		}
	}
	s.Files = "ok"
	for _, f := range zbFiles {
		dst := filepath.Join(a.paths.Game, f)
		switch {
		case !exists(dst):
			s.Files = "missing"
		case src != "" && s.Files == "ok" && fileSHA256(dst) != fileSHA256(filepath.Join(src, f)):
			s.Files = "old"
		}
	}
	switch s.Files {
	case "missing":
		s.Steps = append(s.Steps, "Copy zbNative.dll and ZombieBuddy.jar into the game folder")
	case "old":
		s.Steps = append(s.Steps, "Update zbNative.dll and ZombieBuddy.jar in the game folder (yours are older than the Workshop copy)")
	default:
		if s.Update {
			s.Steps = append(s.Steps, "Copy the new zbNative.dll and ZombieBuddy.jar into the game folder")
		}
	}
	if cfg, err := readLauncherJSON(a.zbLauncherJSON()); err == nil {
		args, _ := cfg["vmArgs"].([]any)
		s.Launch = zbHasArg(args)
	}
	if !s.Launch {
		s.Steps = append(s.Steps, "Add "+zbLaunchArg+" to ProjectZomboid64.json (a backup is kept next to it)")
	}
	s.Ready = s.Downloaded && s.Files != "missing" && s.Launch // an old copy still loads Java mods
	s.Outdated = s.Update || s.Files == "old"
	return s
}

// zbSetup does the steps zbStatus lists. The game has to be closed: it holds the jar open.
func (a *App) zbSetup() error {
	if a.gameState().Running || gameRunning() {
		return errors.New("close Project Zomboid first")
	}
	if newer, _, remote := a.zbNewer(false); a.zbSource() == "" || newer {
		if err := a.zbDownload(remote); err != nil {
			return err
		}
	}
	src := a.zbSource()
	if src == "" {
		return errors.New("ZombieBuddy downloaded but its files aren't there. Check the item in Steam")
	}
	for _, f := range zbFiles {
		dst := filepath.Join(a.paths.Game, f)
		if exists(dst) && fileSHA256(dst) == fileSHA256(filepath.Join(src, f)) {
			continue
		}
		if err := copyFileAtomic(filepath.Join(src, f), dst); err != nil {
			return fmt.Errorf("couldn't copy %s into the game folder: %v", f, err)
		}
	}
	return a.zbPatchJSON()
}

// zbDownload gets ZombieBuddy from the Workshop and waits until Steam has a copy at least as new as want.
func (a *App) zbDownload(want uint32) error {
	id, _ := strconv.ParseUint(zbWorkshopID, 10, 64)
	st := a.steam
	if st == nil {
		return errors.New("Steam isn't connected")
	}
	st.SubscribeItem(id)
	st.DownloadItem(id, true)
	start := time.Now()
	deadline := start.Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		state := st.ItemState(id)
		_, ts, ok := st.ItemInstallInfo(id)
		// Steam's install time should reach the Workshop's update time; give up waiting for that after 15 s
		if ok && (ts >= want || time.Since(start) > 15*time.Second) && state&ItemInstalled != 0 && state&(ItemDownloading|ItemDownloadPending|ItemNeedsUpdate) == 0 && a.zbSource() != "" {
			zbCheck.Lock()
			zbCheck.handled = want
			zbCheck.Unlock()
			return nil
		}
		time.Sleep(time.Second)
	}
	return errors.New("Steam didn't finish downloading ZombieBuddy. Check Steam and try again")
}

func (a *App) zbPatchJSON() error {
	p := a.zbLauncherJSON()
	cfg, err := readLauncherJSON(p)
	if err != nil {
		return err
	}
	args, _ := cfg["vmArgs"].([]any)
	if zbHasArg(args) {
		return nil
	}
	orig, _ := os.ReadFile(p)
	if err := os.WriteFile(p+".knox-backup", orig, 0644); err != nil {
		return fmt.Errorf("couldn't back up ProjectZomboid64.json: %v", err)
	}
	cfg["vmArgs"] = append([]any{zbLaunchArg}, args...)
	b, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0644)
}

func copyFileAtomic(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	tmp := dst + ".knox"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
