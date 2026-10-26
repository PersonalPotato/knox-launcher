//go:build !windows

package main

import "os"

// EZ_ZB_UPDATE=1 pretends the Workshop has a newer ZombieBuddy than the one downloaded.
func (m *mockSteam) ItemUpdated(id uint64) (uint32, error) {
	if os.Getenv("EZ_ZB_UPDATE") != "" {
		return 1761000000, nil
	}
	return 1760000000, nil
}
