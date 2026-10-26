//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func detectSteamPath() string {
	if v := os.Getenv("EZ_STEAM"); v != "" {
		return v
	}
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		for _, path := range []string{`Software\Valve\Steam`, `SOFTWARE\WOW6432Node\Valve\Steam`} {
			k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			for _, name := range []string{"SteamPath", "InstallPath"} {
				if v, _, err := k.GetStringValue(name); err == nil && v != "" {
					k.Close()
					return filepath.Clean(v)
				}
			}
			k.Close()
		}
	}
	return `C:\Program Files (x86)\Steam`
}

func detectZomboidDir() string {
	if v := os.Getenv("EZ_ZOMBOID"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Zomboid")
}

func detectDataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "KnoxLauncher")
}

func samePath(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }
