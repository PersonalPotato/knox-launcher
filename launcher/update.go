package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Self-update from GitHub releases. A release has KnoxLauncher.exe and SHA256SUMS.txt
// (made by .github/workflows/build.yml); the exe is only swapped in when its hash matches.

const updateRepo = "PersonalPotato/knox-launcher"
const updateEvery = 6 * time.Hour

var updateCheck struct {
	sync.Mutex
	at      time.Time
	running bool
	latest  string // tag without the "v"
	notes   string
	page    string
	exeURL  string
	sumURL  string
	err     string
	busy    bool // downloading or swapping
}

// A freshly updated launcher waits a moment so the old one has let go of Steam and the window.
func init() {
	if os.Getenv("KNOX_UPDATED") != "" {
		os.Unsetenv("KNOX_UPDATED")
		time.Sleep(2 * time.Second)
	}
}

// updateCall handles the update RPCs, like communityCall.
func (a *App) updateCall(method string, args json.RawMessage) (any, bool, error) {
	switch method {
	case "checkUpdate":
		a.updateRemote(true)
		return updateStatus(), true, nil
	case "applyUpdate":
		a.mu.Lock()
		running := a.game.Running
		a.mu.Unlock()
		if running {
			return nil, true, errors.New("close the game first")
		}
		return nil, true, applyUpdate()
	}
	return nil, false, nil
}

// updateStatus goes into every state poll.
func updateStatus() map[string]any {
	updateCheck.Lock()
	defer updateCheck.Unlock()
	return map[string]any{
		"latest":    updateCheck.latest,
		"available": updateCheck.exeURL != "" && newerVersion(updateCheck.latest, appVersion),
		"notes":     updateCheck.notes,
		"page":      updateCheck.page,
		"error":     updateCheck.err,
		"busy":      updateCheck.busy,
		"checked":   updateCheck.at.Unix(),
	}
}

// updateRemote starts a check when the last one is old. force checks now and waits for it.
func (a *App) updateRemote(force bool) {
	updateCheck.Lock()
	if updateCheck.running || (!force && time.Since(updateCheck.at) < updateEvery) {
		updateCheck.Unlock()
		return
	}
	updateCheck.running = true
	updateCheck.Unlock()
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := fetchLatest()
		updateCheck.Lock()
		defer updateCheck.Unlock()
		updateCheck.running, updateCheck.at = false, time.Now()
		updateCheck.err = ""
		if err != nil {
			log.Printf("update check: %v", err)
			updateCheck.err = err.Error()
		}
	}()
	if force {
		select {
		case <-done:
		case <-time.After(15 * time.Second):
		}
	}
}

func fetchLatest() error {
	url := "https://api.github.com/repos/" + updateRepo + "/releases/latest"
	if u := os.Getenv("KNOX_UPDATE_URL"); u != "" { // for testing against a fake release
		url = u
	}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "KnoxLauncher/"+appVersion)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return errors.New("couldn't reach GitHub")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil // no release yet
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub said %s", resp.Status)
	}
	var rel struct {
		Tag    string `json:"tag_name"`
		Body   string `json:"body"`
		Page   string `json:"html_url"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return errors.New("GitHub sent something odd")
	}
	updateCheck.Lock()
	defer updateCheck.Unlock()
	updateCheck.latest = strings.TrimPrefix(rel.Tag, "v")
	updateCheck.notes, updateCheck.page = rel.Body, rel.Page
	updateCheck.exeURL, updateCheck.sumURL = "", ""
	for _, as := range rel.Assets {
		switch strings.ToLower(as.Name) {
		case "knoxlauncher.exe":
			updateCheck.exeURL = as.URL
		case "sha256sums.txt":
			updateCheck.sumURL = as.URL
		}
	}
	if updateCheck.sumURL == "" {
		updateCheck.exeURL = "" // never install a build we can't check
	}
	return nil
}

// newerVersion compares dotted versions: "2.10" is newer than "2.9".
func newerVersion(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			y, _ = strconv.Atoi(pb[i])
		}
		if x != y {
			return x > y
		}
	}
	return false
}

// applyUpdate downloads the new exe, checks its hash, swaps it in and restarts.
// The current exe is kept as KnoxLauncher.prev.exe.
func applyUpdate() error {
	updateCheck.Lock()
	if updateCheck.busy {
		updateCheck.Unlock()
		return errors.New("already updating")
	}
	exeURL, sumURL, ver := updateCheck.exeURL, updateCheck.sumURL, updateCheck.latest
	if exeURL == "" || !newerVersion(ver, appVersion) {
		updateCheck.Unlock()
		return errors.New("no update to install")
	}
	updateCheck.busy = true
	updateCheck.Unlock()
	defer func() {
		updateCheck.Lock()
		updateCheck.busy = false
		updateCheck.Unlock()
	}()

	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	want, err := wantedHash(sumURL)
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	tmp := filepath.Join(dir, "KnoxLauncher.new.exe")
	got, err := download(exeURL, tmp)
	if err != nil {
		os.Remove(tmp)
		return errors.New("download failed: " + err.Error())
	}
	if !strings.EqualFold(got, want) {
		os.Remove(tmp)
		return errors.New("the download didn't match its SHA256, so it wasn't installed")
	}
	prev := filepath.Join(dir, "KnoxLauncher.prev.exe")
	os.Remove(prev)
	// Windows lets a running exe be renamed, just not overwritten.
	if err := os.Rename(exe, prev); err != nil {
		os.Remove(tmp)
		return errors.New("couldn't replace the launcher: " + err.Error())
	}
	if err := os.Rename(tmp, exe); err != nil {
		os.Rename(prev, exe)
		return errors.New("couldn't replace the launcher: " + err.Error())
	}
	log.Printf("updated %s -> %s (sha256 %s), restarting", appVersion, ver, got)
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "KNOX_UPDATED=1")
	if err := cmd.Start(); err != nil {
		log.Printf("restart after update: %v", err)
		return errors.New("updated, but couldn't restart. Open the launcher again")
	}
	go func() {
		time.Sleep(300 * time.Millisecond) // let the RPC answer get back to the page
		os.Exit(0)
	}()
	return nil
}

// wantedHash reads KnoxLauncher.exe's line from SHA256SUMS.txt.
func wantedHash(url string) (string, error) {
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Get(url)
	if err != nil {
		return "", errors.New("couldn't get the SHA256 file")
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(io.LimitReader(resp.Body, 64<<10))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) == 2 && strings.EqualFold(strings.TrimPrefix(f[1], "*"), "KnoxLauncher.exe") && len(f[0]) == 64 {
			return f[0], nil
		}
	}
	return "", errors.New("the SHA256 file has no line for KnoxLauncher.exe")
}

// download saves url to path and returns its SHA256.
func download(url, path string) (string, error) {
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errors.New(resp.Status)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 200<<20))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
