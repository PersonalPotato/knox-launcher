package main

import (
	"os"
	"path/filepath"
	"strings"
)

// readServerIni reads Zomboid/Server/<world>.ini as key -> value.
func (a *App) readServerIni(world string) map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(filepath.Join(a.paths.serverDir(), world+".ini"))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, "="); i > 0 {
			out[line[:i]] = line[i+1:]
		}
	}
	return out
}

// readSandbox flattens Zomboid/Server/<world>_SandboxVars.lua into "ZombieLore.Speed" -> "2".
func (a *App) readSandbox(world string) map[string]string {
	return readSandboxFile(filepath.Join(a.paths.serverDir(), world+"_SandboxVars.lua"))
}

func readSandboxFile(file string) map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(file)
	if err != nil {
		return out
	}
	toks := luaTokens(string(b))
	var path []string
	key := ""
	expectValue := false
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch {
		case t == "{":
			if key != "" {
				path = append(path, key)
			} else {
				path = append(path, "")
			}
			key, expectValue = "", false
		case t == "}":
			if len(path) > 0 {
				path = path[:len(path)-1]
			}
			key, expectValue = "", false
		case t == "=":
			expectValue = true
		case t == ",":
			key, expectValue = "", false
		default:
			if expectValue {
				full := key
				// path[0] is "SandboxVars"
				if len(path) > 1 {
					full = strings.Join(append(append([]string{}, path[1:]...), key), ".")
				}
				if len(path) >= 1 && key != "VERSION" {
					out[strings.TrimPrefix(full, ".")] = strings.Trim(t, "\"")
				}
				expectValue = false
			} else {
				key = t
			}
		}
	}
	return out
}

func luaTokens(src string) []string {
	var toks []string
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == '-' && i+1 < len(src) && src[i+1] == '-':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
		case c == '{' || c == '}' || c == '=' || c == ',':
			toks = append(toks, string(c))
			i++
		case c == '"':
			j := i + 1
			for j < len(src) && src[j] != '"' {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			if j >= len(src) {
				j = len(src) - 1
			}
			toks = append(toks, src[i:j+1])
			i = j + 1
		default:
			j := i
			for j < len(src) && !strings.ContainsRune(" \t\r\n{}=,", rune(src[j])) {
				j++
			}
			toks = append(toks, src[i:j])
			i = j
		}
	}
	return toks
}
