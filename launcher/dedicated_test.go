package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSandboxRoundTrip(t *testing.T) {
	in := map[string]string{"Zombies": "2", "ZombieLore.Speed": "3", "ZombieLore.Active": "true", "Map.MapNeedsLight": "false", "LootName": "abc def"}
	f := filepath.Join(t.TempDir(), "x_SandboxVars.lua")
	if err := os.WriteFile(f, []byte(sandboxLua(in)), 0644); err != nil {
		t.Fatal(err)
	}
	out := readSandboxFile(f)
	for k, v := range in {
		if out[k] != v {
			t.Errorf("%s: got %q want %q\n%s", k, out[k], v, sandboxLua(in))
		}
	}
}

func TestPatchIni(t *testing.T) {
	f := filepath.Join(t.TempDir(), "w.ini")
	_ = os.WriteFile(f, []byte("# c\r\nPVP=true\r\nMods=a\r\n"), 0644)
	if err := patchIni(f, map[string]string{"Mods": "b;c", "Public": "false"}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(f)
	if string(b) != "# c\r\nPVP=true\r\nMods=b;c\r\nPublic=false\r\n" {
		t.Errorf("got %q", b)
	}
}
