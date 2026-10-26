package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readZB/rewrite must keep ZombieBuddy's own fields, including 64-bit Steam ids, exactly.
func TestZBKeepsFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	p := zbApprovalsPath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	in := `{"formatVersion":2,"mods":[{"id":"X","workshop_id":3809306528,"jar_hash":"aa","decision":false,"time":"t","author_id":76561198012345678}]}`
	_ = os.WriteFile(p, []byte(in), 0644)
	f := readZB()
	if d, ok := f.decisions()["aa"]; !ok || d || len(f.Mods) != 1 {
		t.Fatal("not read")
	}
	f.Mods[0]["decision"] = true
	b, _ := jsonIndent(f)
	if !strings.Contains(string(b), "76561198012345678") || !strings.Contains(string(b), "3809306528") {
		t.Fatalf("ids changed: %s", b)
	}
}
