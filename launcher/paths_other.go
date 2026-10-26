//go:build !windows

package main

import (
	"os"
	"path/filepath"
)

// On a dev machine everything lives under ./devdata (or the EZ_* variables).
func devRoot() string {
	if v := os.Getenv("EZ_DEV"); v != "" {
		return v
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, "devdata")
}

func detectSteamPath() string {
	if v := os.Getenv("EZ_STEAM"); v != "" {
		return v
	}
	return filepath.Join(devRoot(), "Steam")
}

func detectZomboidDir() string {
	if v := os.Getenv("EZ_ZOMBOID"); v != "" {
		return v
	}
	return filepath.Join(devRoot(), "Zomboid")
}

func detectDataDir() string { return filepath.Join(devRoot(), "KnoxLauncher") }

func samePath(a, b string) bool { return filepath.Clean(a) == filepath.Clean(b) }
