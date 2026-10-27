//go:build !windows

package main

import (
	"log"
	"os"
	"os/exec"
	"strconv"
	"sync/atomic"
	"time"
)

// Dev stand-in: the "game" runs for EZ_DEV_GAME seconds (default 20).

var devGame atomic.Bool

type gameProcess struct{ done chan struct{} }

func (g *gameProcess) Wait() int { <-g.done; return 0 }

func startGameDirect(game string) (*gameProcess, error) {
	secs, _ := strconv.Atoi(os.Getenv("EZ_DEV_GAME"))
	if secs <= 0 {
		secs = 20
	}
	log.Printf("(dev) Project Zomboid 'runs' for %ds", secs)
	g := &gameProcess{done: make(chan struct{})}
	devGame.Store(true)
	go func() {
		time.Sleep(time.Duration(secs) * time.Second)
		devGame.Store(false)
		close(g.done)
	}()
	return g, nil
}

func startGameViaSteam() error {
	log.Println("(dev) would ask Steam to start Project Zomboid")
	return nil
}

func gameRunning() bool { return devGame.Load() }

func openURL(url string) error { return exec.Command("xdg-open", url).Start() }

// devServerCommand stands in for the co-op server: it prints the same status lines, then idles.
func devServerCommand() *exec.Cmd {
	script := `echo "info@coop mode enabled"; sleep 1; echo "status@UI_ServerStatus_Initialising"; sleep 2; echo "status@UI_ServerStatus_LoadingWorld"; sleep 3; echo "steam-id@90000000000000001"; echo "status@UI_ServerStatus_Started"; while read l; do case "$l" in process-status@eof) exit 0;; esac; done`
	return exec.Command("sh", "-c", script)
}

func hidden(cmd *exec.Cmd) *exec.Cmd { return cmd }

func tieToLauncher(p *os.Process, pidFile string) {}

func killOrphanServers(pidFile string) int { return 0 }

func memoryLine() string { return "memory: unknown" }
