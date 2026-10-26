//go:build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const createNoWindow = 0x08000000

func hidden(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
}

type gameProcess struct{ cmd *exec.Cmd }

// Wait blocks until the game closes and returns its exit code.
func (g *gameProcess) Wait() int {
	_ = g.cmd.Wait()
	if g.cmd.ProcessState != nil {
		return g.cmd.ProcessState.ExitCode()
	}
	return -1
}

// startGameDirect runs ProjectZomboid64.exe from its folder. Steam stays signed in and the game
// finds it through steam_appid.txt and the SteamAppId variable, the same way Steam starts it.
func startGameDirect(game string) (*gameProcess, error) {
	exe := filepath.Join(game, "ProjectZomboid64.exe")
	if !exists(exe) {
		return nil, errors.New("ProjectZomboid64.exe not found in " + game)
	}
	if !exists(filepath.Join(game, "steam_appid.txt")) {
		_ = os.WriteFile(filepath.Join(game, "steam_appid.txt"), []byte("108600"), 0644)
	}
	cmd := exec.Command(exe)
	cmd.Dir = game
	cmd.Env = append(os.Environ(), "SteamAppId=108600", "SteamGameId=108600", "SteamOverlayGameId=108600")
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &gameProcess{cmd: cmd}, nil
}

// startGameViaSteam asks Steam to start Project Zomboid (the fallback).
func startGameViaSteam() error { return shellOpen("steam://rungameid/108600") }

// gameRunning looks for ProjectZomboid64.exe in the process list.
func gameRunning() bool {
	found := false
	eachProcess(func(e *windows.ProcessEntry32) bool {
		if strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), "ProjectZomboid64.exe") {
			found = true
			return false
		}
		return true
	})
	return found
}

func eachProcess(f func(*windows.ProcessEntry32) bool) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if !f(&e) {
			return
		}
	}
}

// shellOpen opens a link or folder the way Explorer does.
func shellOpen(target string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	file, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}

func openURL(url string) error { return shellOpen(url) }

func devServerCommand() *exec.Cmd { return nil } // only used off Windows

var serverJob struct {
	once sync.Once
	h    windows.Handle
}

// tieToLauncher puts a child process in a job that Windows closes when the launcher exits, so a
// server can't outlive the launcher (an orphan keeps the network port and locks mod jars).
// It also notes the pid and start time in pidFile, so a later launcher can stop it if the job failed.
func tieToLauncher(p *os.Process, pidFile string) {
	serverJob.once.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			_ = windows.CloseHandle(h)
			return
		}
		serverJob.h = h
	})
	if p == nil {
		return
	}
	ph, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(p.Pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(ph)
	if serverJob.h != 0 {
		_ = windows.AssignProcessToJobObject(serverJob.h, ph)
	}
	if t, ok := startTime(ph); ok {
		_ = os.WriteFile(pidFile, []byte(strconv.Itoa(p.Pid)+" "+strconv.FormatInt(t, 10)), 0644)
	}
}

func startTime(h windows.Handle) (int64, bool) {
	var c, e, k, u windows.Filetime
	if err := windows.GetProcessTimes(h, &c, &e, &k, &u); err != nil {
		return 0, false
	}
	return c.Nanoseconds(), true
}

// killOrphanServers stops the server an earlier launcher started (and noted in pidFile) if it is
// still running. The start time check makes sure the pid wasn't reused by another program.
func killOrphanServers(pidFile string) int {
	b, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}
	_ = os.Remove(pidFile)
	f := strings.Fields(string(b))
	if len(f) != 2 {
		return 0
	}
	pid, _ := strconv.Atoi(f[0])
	want, _ := strconv.ParseInt(f[1], 10, 64)
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0
	}
	defer windows.CloseHandle(h)
	if t, ok := startTime(h); !ok || t != want {
		return 0
	}
	var code uint32
	if windows.GetExitCodeProcess(h, &code) != nil || code != 259 { // 259 = still running
		return 0
	}
	if windows.TerminateProcess(h, 1) != nil {
		return 0
	}
	return 1
}
