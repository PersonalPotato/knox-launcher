package main

import (
	"os"
	"testing"
)

func TestXModel(t *testing.T) {
	base := "/home/claude/pzdev/Steam/steamapps/common/ProjectZomboid/media/models_X/Skinned/"
	for _, f := range []string{"MaleBody.x", "FemaleBody.x", "Clothes/Bob_JacketLeather.x", "Hair/Bob_Hair_CrewCut.x"} {
		b, err := os.ReadFile(base + f)
		if err != nil {
			t.Skip(err)
		}
		m, err := meshFromX(string(b))
		if err != nil {
			t.Fatal(f, err)
		}
		var mn, mx [3]float32
		for i := 0; i < 3; i++ {
			mn[i], mx[i] = 1e9, -1e9
		}
		for i := 0; i < len(m.Pos); i += 3 {
			for k := 0; k < 3; k++ {
				mn[k] = min(mn[k], m.Pos[i+k])
				mx[k] = max(mx[k], m.Pos[i+k])
			}
		}
		t.Logf("%s: %d verts %d tris uv %d bounds %v %v", f, len(m.Pos)/3, len(m.Idx)/3, len(m.UV)/2, mn, mx)
	}
}
