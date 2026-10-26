//go:build !windows

package main

import "strconv"

// The dev build signs in with a test ticket that a community server run with TEST_MODE=1 accepts.
func (m *mockSteam) WebTicket(identity string) (string, string, error) {
	return "test:" + strconv.FormatUint(mockMe, 10) + ":" + mockNames[mockMe], identity, nil
}
