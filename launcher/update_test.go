package main

import "testing"

func TestNewerVersion(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"2.6", "2.5", true}, {"2.10", "2.9", true}, {"2.5", "2.5", false}, {"2.5.1", "2.5", true}, {"2.4", "2.5", false}, {"3", "2.9", true}} {
		if got := newerVersion(c.a, c.b); got != c.want {
			t.Errorf("newerVersion(%q, %q) = %v", c.a, c.b, got)
		}
	}
}
