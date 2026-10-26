package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// The 3D character preview draws the game's own models (DirectX .x text files) and textures.

type Mesh struct {
	Pos []float32 `json:"p"`
	UV  []float32 `json:"uv"`
	Idx []uint32  `json:"i"`
	// skinning, when the file has SkinWeights: 4 bone slots per vertex, the bone's offset matrix
	// (bind pose to bone space, 16 floats each) and the file's skeleton
	Bones   []string  `json:"b,omitempty"`
	Joints  []uint16  `json:"j,omitempty"`
	Weights []float32 `json:"w,omitempty"`
	Offsets []float32 `json:"o,omitempty"`
	Frames  []XFrame  `json:"f,omitempty"`
}

// XFrame is one bone of a skeleton: its parent (-1 for a root) and its bind pose relative to it.
type XFrame struct {
	Name   string      `json:"n"`
	Parent int         `json:"p"`
	M      [16]float32 `json:"m"`
}

type xNode struct {
	typ      string
	name     string
	data     []string
	children []*xNode
}

func xTokens(src string) []string {
	var toks []string
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == '/' && i+1 < len(src) && src[i+1] == '/', c == '#':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ',' || c == ';':
			i++
		case c == '{' || c == '}':
			toks = append(toks, string(c))
			i++
		case c == '<':
			for i < len(src) && src[i] != '>' {
				i++
			}
			i++
		case c == '"':
			j := i + 1
			for j < len(src) && src[j] != '"' {
				j++
			}
			toks = append(toks, src[i:min(j+1, len(src))])
			i = j + 1
		default:
			j := i
			for j < len(src) && !strings.ContainsRune(" \t\r\n,;{}\"<", rune(src[j])) {
				j++
			}
			toks = append(toks, src[i:j])
			i = j
		}
	}
	return toks
}

func parseX(src string) (*xNode, error) {
	if !strings.HasPrefix(src, "xof ") {
		return nil, errors.New("not a .x file")
	}
	if !strings.Contains(src[:16], "txt") {
		return nil, errors.New("binary .x files aren't supported")
	}
	toks := xTokens(src[16:])
	root := &xNode{typ: "root"}
	stack := []*xNode{root}
	i := 0
	for i < len(toks) {
		t := toks[i]
		cur := stack[len(stack)-1]
		switch {
		case t == "{": // a reference like { Material0 }: skip it, but keep the bone an Animation is for
			if cur.typ == "Animation" && i+2 < len(toks) && toks[i+2] == "}" {
				cur.name = toks[i+1]
			}
			for i < len(toks) && toks[i] != "}" {
				i++
			}
			i++
		case t == "}":
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			i++
		case i+1 < len(toks) && toks[i+1] == "{" && !isNum(t):
			n := &xNode{typ: t}
			cur.children = append(cur.children, n)
			stack = append(stack, n)
			i += 2
		case i+2 < len(toks) && toks[i+2] == "{" && !isNum(t) && !isNum(toks[i+1]):
			n := &xNode{typ: t, name: toks[i+1]}
			cur.children = append(cur.children, n)
			stack = append(stack, n)
			i += 3
		default:
			cur.data = append(cur.data, t)
			i++
		}
	}
	return root, nil
}

func isNum(t string) bool {
	if t == "" {
		return false
	}
	c := t[0]
	return c == '-' || c == '+' || c == '.' || (c >= '0' && c <= '9')
}

type mat4 [16]float64

var identity = mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}

// row-vector convention, as DirectX writes them: v' = v * M
func (a mat4) mul(b mat4) mat4 {
	var r mat4
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			var s float64
			for k := 0; k < 4; k++ {
				s += a[i*4+k] * b[k*4+j]
			}
			r[i*4+j] = s
		}
	}
	return r
}

func nums(data []string) []float64 {
	out := make([]float64, 0, len(data))
	for _, d := range data {
		if f, err := strconv.ParseFloat(d, 64); err == nil {
			out = append(out, f)
		}
	}
	return out
}

// meshFromX merges every mesh in the file, each moved by the frames around it.
func meshFromX(src string) (*Mesh, error) {
	root, err := parseX(src)
	if err != nil {
		return nil, err
	}
	out := &Mesh{}
	var walk func(n *xNode, m mat4, parent int)
	walk = func(n *xNode, m mat4, parent int) {
		switch n.typ {
		case "template":
			return
		case "Frame":
			local := frameMatrix(n)
			m = local.mul(m)
			f := XFrame{Name: n.name, Parent: parent}
			for i := range local {
				f.M[i] = float32(local[i])
			}
			out.Frames = append(out.Frames, f)
			parent = len(out.Frames) - 1
		case "Mesh":
			addMesh(out, n, m)
			return
		}
		for _, c := range n.children {
			walk(c, m, parent)
		}
	}
	walk(root, identity, -1)
	if len(out.Bones) == 0 {
		out.Frames = nil
	}
	if len(out.Pos) == 0 {
		return nil, errors.New("no mesh in file")
	}
	return out, nil
}

func addMesh(out *Mesh, n *xNode, m mat4) {
	v := nums(n.data)
	if len(v) < 1 {
		return
	}
	nv := int(v[0])
	if len(v) < 1+nv*3+1 {
		return
	}
	base := uint32(len(out.Pos) / 3)
	for i := 0; i < nv; i++ {
		x, y, z := v[1+i*3], v[2+i*3], v[3+i*3]
		tx := x*m[0] + y*m[4] + z*m[8] + m[12]
		ty := x*m[1] + y*m[5] + z*m[9] + m[13]
		tz := x*m[2] + y*m[6] + z*m[10] + m[14]
		out.Pos = append(out.Pos, float32(tx), float32(ty), float32(tz))
	}
	p := 1 + nv*3
	nf := int(v[p])
	p++
	for f := 0; f < nf && p < len(v); f++ {
		k := int(v[p])
		p++
		if p+k > len(v) {
			break
		}
		for j := 1; j+1 < k; j++ { // triangle fan
			out.Idx = append(out.Idx, base+uint32(v[p]), base+uint32(v[p+j]), base+uint32(v[p+j+1]))
		}
		p += k
	}
	uv := make([]float32, nv*2)
	for _, c := range n.children {
		if c.typ == "MeshTextureCoords" {
			t := nums(c.data)
			if len(t) >= 1 && int(t[0]) == nv && len(t) >= 1+nv*2 {
				for i := 0; i < nv*2; i++ {
					uv[i] = float32(t[1+i])
				}
			}
		}
	}
	out.UV = append(out.UV, uv...)
	addSkin(out, n, m, int(base), nv)
}

func frameMatrix(n *xNode) mat4 {
	local := identity
	for _, c := range n.children {
		if c.typ == "FrameTransformMatrix" {
			if v := nums(c.data); len(v) >= 16 {
				copy(local[:], v[:16])
			}
		}
	}
	return local
}

// addSkin reads a mesh's SkinWeights. Vertices were moved by the mesh's frame (m), so each offset
// matrix gets m's inverse in front. Each vertex keeps its 4 heaviest bones.
func addSkin(out *Mesh, n *xNode, m mat4, base, nv int) {
	type slot struct {
		bone int
		w    float64
	}
	var per [][]slot
	inv := m.inverse()
	for _, c := range n.children {
		if c.typ != "SkinWeights" || len(c.data) < 1 {
			continue
		}
		if per == nil {
			per = make([][]slot, nv)
		}
		name := strings.Trim(c.data[0], `"`)
		v := nums(c.data[1:])
		if len(v) < 1 {
			continue
		}
		k := int(v[0])
		if len(v) < 1+2*k+16 {
			continue
		}
		bone := -1
		for i, b := range out.Bones {
			if b == name {
				bone = i
			}
		}
		if bone < 0 {
			var off mat4
			copy(off[:], v[1+2*k:1+2*k+16])
			off = inv.mul(off)
			for _, f := range off {
				out.Offsets = append(out.Offsets, float32(f))
			}
			out.Bones = append(out.Bones, name)
			bone = len(out.Bones) - 1
		}
		for i := 0; i < k; i++ {
			vi := int(v[1+i])
			if vi >= 0 && vi < nv && v[1+k+i] > 0 {
				per[vi] = append(per[vi], slot{bone, v[1+k+i]})
			}
		}
	}
	if per == nil {
		if len(out.Joints) > 0 { // an unskinned mesh in a skinned file: pad so the arrays line up
			out.Joints = append(out.Joints, make([]uint16, nv*4)...)
			out.Weights = append(out.Weights, make([]float32, nv*4)...)
		}
		return
	}
	if pad := base*4 - len(out.Joints); pad > 0 {
		out.Joints = append(out.Joints, make([]uint16, pad)...)
		out.Weights = append(out.Weights, make([]float32, pad)...)
	}
	for _, s := range per {
		sort.Slice(s, func(i, j int) bool { return s[i].w > s[j].w })
		if len(s) > 4 {
			s = s[:4]
		}
		var sum float64
		for _, x := range s {
			sum += x.w
		}
		for i := 0; i < 4; i++ {
			if i < len(s) && sum > 0 {
				out.Joints = append(out.Joints, uint16(s[i].bone))
				out.Weights = append(out.Weights, float32(s[i].w/sum))
			} else {
				out.Joints = append(out.Joints, 0)
				out.Weights = append(out.Weights, 0)
			}
		}
	}
}

// inverse of an affine matrix (rotation/scale plus translation), row-vector convention.
func (a mat4) inverse() mat4 {
	// 3x3 part
	m := [9]float64{a[0], a[1], a[2], a[4], a[5], a[6], a[8], a[9], a[10]}
	det := m[0]*(m[4]*m[8]-m[5]*m[7]) - m[1]*(m[3]*m[8]-m[5]*m[6]) + m[2]*(m[3]*m[7]-m[4]*m[6])
	if math.Abs(det) < 1e-12 {
		return identity
	}
	id := 1 / det
	r := [9]float64{
		(m[4]*m[8] - m[5]*m[7]) * id, (m[2]*m[7] - m[1]*m[8]) * id, (m[1]*m[5] - m[2]*m[4]) * id,
		(m[5]*m[6] - m[3]*m[8]) * id, (m[0]*m[8] - m[2]*m[6]) * id, (m[2]*m[3] - m[0]*m[5]) * id,
		(m[3]*m[7] - m[4]*m[6]) * id, (m[1]*m[6] - m[0]*m[7]) * id, (m[0]*m[4] - m[1]*m[3]) * id,
	}
	tx, ty, tz := a[12], a[13], a[14]
	return mat4{
		r[0], r[1], r[2], 0,
		r[3], r[4], r[5], 0,
		r[6], r[7], r[8], 0,
		-(tx*r[0] + ty*r[3] + tz*r[6]), -(tx*r[1] + ty*r[4] + tz*r[7]), -(tx*r[2] + ty*r[5] + tz*r[8]), 1,
	}
}

// Animations ----------------------------------------------------------------------------------------

// Anim is one AnimationSet: per bone, key times in seconds and values (rotation as x,y,z,w in
// three.js's order, position x,y,z, scale x,y,z).
type Anim struct {
	Name   string      `json:"name"`
	Length float64     `json:"length"`
	Tracks []AnimTrack `json:"tracks"`
}

type AnimTrack struct {
	Bone string    `json:"bone"`
	RT   []float32 `json:"rt,omitempty"`
	R    []float32 `json:"r,omitempty"`
	PT   []float32 `json:"pt,omitempty"`
	P    []float32 `json:"p,omitempty"`
	ST   []float32 `json:"st,omitempty"`
	S    []float32 `json:"s,omitempty"`
}

func animFromX(src string) (*Anim, error) {
	root, err := parseX(src)
	if err != nil {
		return nil, err
	}
	tps := 4800.0
	var set *xNode
	for _, c := range root.children {
		if c.typ == "AnimTicksPerSecond" {
			if v := nums(c.data); len(v) > 0 && v[0] > 0 {
				tps = v[0]
			}
		}
		if c.typ == "AnimationSet" && set == nil {
			set = c
		}
	}
	if set == nil {
		return nil, errors.New("no animation in file")
	}
	out := &Anim{Name: set.name}
	for _, a := range set.children {
		if a.typ != "Animation" || a.name == "" {
			continue
		}
		tr := AnimTrack{Bone: a.name}
		for _, k := range a.children {
			if k.typ != "AnimationKey" {
				continue
			}
			v := nums(k.data)
			if len(v) < 2 {
				continue
			}
			typ, n, p := int(v[0]), int(v[1]), 2
			for i := 0; i < n && p+1 < len(v); i++ {
				t, cnt := v[p]/tps, int(v[p+1])
				p += 2
				if p+cnt > len(v) {
					break
				}
				x := v[p : p+cnt]
				p += cnt
				out.Length = math.Max(out.Length, t)
				switch {
				case typ == 0 && cnt == 4: // DirectX keeps w first and the conjugate rotation
					tr.RT = append(tr.RT, float32(t))
					tr.R = append(tr.R, float32(-x[1]), float32(-x[2]), float32(-x[3]), float32(x[0]))
				case typ == 1 && cnt == 3:
					tr.ST = append(tr.ST, float32(t))
					tr.S = append(tr.S, float32(x[0]), float32(x[1]), float32(x[2]))
				case typ == 2 && cnt == 3:
					tr.PT = append(tr.PT, float32(t))
					tr.P = append(tr.P, float32(x[0]), float32(x[1]), float32(x[2]))
				}
			}
		}
		out.Tracks = append(out.Tracks, tr)
	}
	return out, nil
}

func (ix *AssetIndex) Anim(medias []string, name string) (*Anim, error) {
	if name == "" || strings.ContainsAny(name, `/\.`) {
		return nil, os.ErrNotExist
	}
	path := ix.find(medias, "anims_x/bob/"+name)
	if path == "" || !strings.HasSuffix(strings.ToLower(path), ".x") {
		return nil, os.ErrNotExist
	}
	ix.mu.Lock()
	if a, ok := ix.anim[path]; ok {
		ix.mu.Unlock()
		return a, nil
	}
	ix.mu.Unlock()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	a, err := animFromX(string(b))
	if err != nil {
		return nil, err
	}
	ix.mu.Lock()
	ix.anim[path] = a
	ix.mu.Unlock()
	return a, nil
}

// Asset lookup: the game asks for models and textures by name without case or extension.

type AssetIndex struct {
	mu    sync.Mutex
	roots map[string]map[string]string // media root -> lower relative path (no ext) -> real path
	mesh  map[string]*Mesh
	anim  map[string]*Anim
}

func newAssetIndex() *AssetIndex {
	return &AssetIndex{roots: map[string]map[string]string{}, mesh: map[string]*Mesh{}, anim: map[string]*Anim{}}
}

func (ix *AssetIndex) index(media string) map[string]string {
	if m, ok := ix.roots[media]; ok {
		return m
	}
	m := map[string]string{}
	for _, sub := range []string{"textures", "models_X", "anims_X"} {
		base := filepath.Join(media, sub)
		_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(media, path)
			rel = strings.ToLower(filepath.ToSlash(rel))
			ext := filepath.Ext(rel)
			if ext != ".png" && ext != ".x" {
				return nil
			}
			m[strings.TrimSuffix(rel, ext)] = path
			return nil
		})
	}
	ix.roots[media] = m
	return m
}

// find looks a name up in the game's media folder and every mod's (mods win, like in game).
func (ix *AssetIndex) find(medias []string, rel string) string {
	rel = strings.ToLower(strings.ReplaceAll(rel, "\\", "/"))
	rel = strings.TrimPrefix(rel, "media/")
	rel = strings.TrimSuffix(strings.TrimSuffix(rel, ".png"), ".x")
	ix.mu.Lock()
	defer ix.mu.Unlock()
	for i := len(medias) - 1; i >= 0; i-- {
		m := ix.index(medias[i])
		for _, cand := range []string{rel, "textures/" + rel, "models_x/" + rel} {
			if p, ok := m[cand]; ok {
				return p
			}
		}
	}
	return ""
}

func (ix *AssetIndex) Mesh(medias []string, rel string) (*Mesh, error) {
	path := ix.find(medias, rel)
	if path == "" || !strings.HasSuffix(strings.ToLower(path), ".x") {
		return nil, os.ErrNotExist
	}
	ix.mu.Lock()
	if m, ok := ix.mesh[path]; ok {
		ix.mu.Unlock()
		return m, nil
	}
	ix.mu.Unlock()
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := meshFromX(string(b))
	if err != nil {
		return nil, err
	}
	for i := range m.Pos {
		if math.IsNaN(float64(m.Pos[i])) {
			m.Pos[i] = 0
		}
	}
	ix.mu.Lock()
	ix.mesh[path] = m
	ix.mu.Unlock()
	return m, nil
}

func (ix *AssetIndex) Texture(medias []string, rel string) string {
	p := ix.find(medias, rel)
	if strings.HasSuffix(strings.ToLower(p), ".png") {
		return p
	}
	return ""
}

// BodyTexture paints clothes that are drawn onto the skin (t-shirts, socks, shoes) over a body
// texture. The body's alpha is kept: the game cuts out the faces that map to transparent pixels.
type BodyLayer struct {
	Path string
	Tint *[3]float64
}

// The game's CharacterMask parts, in m_Masks order. Torso and Pelvis are drawn as their halves.
var maskParts = []string{"Head", "Torso", "Pelvis", "LeftArm", "LeftHand", "RightArm", "RightHand", "LeftLeg", "LeftFoot", "RightLeg", "RightFoot", "Dress", "Chest", "Waist", "Belt", "Crotch"}

// BodyMask hides one part of the body under a piece of clothing with a 3D model, so the skin
// doesn't poke through it. Folder is the item's m_MasksFolder ("" for the game's body masks).
type BodyMask struct {
	Folder string
	Part   int
}

func (ix *AssetIndex) BodyTexture(medias []string, base string, layers []BodyLayer, masks []BodyMask) ([]byte, error) {
	bp := ix.Texture(medias, base)
	if bp == "" {
		return nil, os.ErrNotExist
	}
	img, err := readPNG(bp)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	out := image.NewNRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.SetNRGBA(x, y, color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA))
		}
	}
	for _, l := range layers {
		lp := ix.Texture(medias, l.Path)
		if lp == "" {
			continue
		}
		li, err := readPNG(lp)
		if err != nil {
			continue
		}
		lb := li.Bounds()
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				c := color.NRGBAModel.Convert(li.At(lb.Min.X+x*lb.Dx()/b.Dx(), lb.Min.Y+y*lb.Dy()/b.Dy())).(color.NRGBA)
				if c.A == 0 {
					continue
				}
				r, g, bl := float64(c.R), float64(c.G), float64(c.B)
				if l.Tint != nil {
					r, g, bl = r*l.Tint[0], g*l.Tint[1], bl*l.Tint[2]
				}
				a := float64(c.A) / 255
				d := out.NRGBAAt(x, y)
				d.R = uint8(float64(d.R)*(1-a) + r*a)
				d.G = uint8(float64(d.G)*(1-a) + g*a)
				d.B = uint8(float64(d.B)*(1-a) + bl*a)
				out.SetNRGBA(x, y, d)
			}
		}
	}
	for _, m := range masks {
		if m.Part < 0 || m.Part >= len(maskParts) {
			continue
		}
		names := []string{maskParts[m.Part]}
		switch names[0] {
		case "Torso":
			names = []string{"Chest", "Waist"}
		case "Pelvis":
			names = []string{"Belt", "Crotch"}
		}
		for _, name := range names {
			mp := ""
			if m.Folder != "" {
				mp = ix.Texture(medias, m.Folder+"/"+name)
			}
			if mp == "" {
				mp = ix.Texture(medias, "Body/Masks/"+name)
			}
			if mp == "" {
				continue
			}
			mi, err := readPNG(mp)
			if err != nil {
				continue
			}
			mb := mi.Bounds()
			for y := 0; y < b.Dy(); y++ {
				for x := 0; x < b.Dx(); x++ {
					if _, _, _, a := mi.At(mb.Min.X+x*mb.Dx()/b.Dx(), mb.Min.Y+y*mb.Dy()/b.Dy()).RGBA(); a > 0x8000 {
						d := out.NRGBAAt(x, y)
						d.A = 0
						out.SetNRGBA(x, y, d)
					}
				}
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func readPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}
