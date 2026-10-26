package questapi

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ChatType is a chat channel type: the number Message() and the like take
// for where (and in what colour) a line shows in the client
type ChatType struct {
	Value int      `json:"value"`
	Names []string `json:"names"` // Chat:: names in the server source, eg Yellow
	Lua   []string `json:"lua"`   // the Lua MT constants, eg MT.Yellow
}

var (
	chatConst = regexp.MustCompile(`const\s+\w+\s+(\w+)\s*=\s*(\d+)\s*;`)
	chatLua   = regexp.MustCompile(`luabind::value\("(\w+)",\s*Chat::(\w+)\)`)
)

// ChatTypes reads namespace Chat (common/eq_constants.h) and the Lua MT
// constants (zone/lua_general.cpp) from the parsed server source
func (c *ParseService) ChatTypes() []ChatType {
	return ParseChatTypes(sourceFile(c.files, "common", "eq_constants.h"), sourceFile(c.files, "zone", "lua_general.cpp"))
}

func sourceFile(files map[string]string, dir, name string) string {
	if f, ok := files[dir+"/"+name]; ok {
		return f
	}
	return files[filepath.Join(dir, name)]
}

func ParseChatTypes(constants, lua string) []ChatType {
	block := between(constants, "namespace Chat {", "};")
	if block == "" {
		return []ChatType{}
	}

	byValue := map[int]*ChatType{}
	byName := map[string]int{}
	var order []int
	for _, m := range chatConst.FindAllStringSubmatch(block, -1) {
		v, _ := strconv.Atoi(m[2])
		byName[m[1]] = v
		if byValue[v] == nil {
			byValue[v] = &ChatType{Value: v, Names: []string{}, Lua: []string{}}
			order = append(order, v)
		}
		byValue[v].Names = append(byValue[v].Names, m[1])
	}

	for _, m := range chatLua.FindAllStringSubmatch(between(lua, "scope lua_register_message_types()", "\n}"), -1) {
		if v, ok := byName[m[2]]; ok {
			byValue[v].Lua = append(byValue[v].Lua, "MT."+m[1])
		}
	}

	sort.Ints(order)
	out := make([]ChatType, 0, len(order))
	for _, v := range order {
		out = append(out, *byValue[v])
	}
	return out
}

func between(s, start, end string) string {
	i := strings.Index(s, start)
	if i < 0 {
		return ""
	}
	s = s[i+len(start):]
	if j := strings.Index(s, end); j >= 0 {
		return s[:j]
	}
	return s
}

var luaClassParent = regexp.MustCompile(`class Lua_(\w+)\s*:\s*public\s+Lua_(\w+)`)

// LuaParents reads which Lua class each one inherits from (Lua_Client :
// public Lua_Mob) from zone/lua_*.h; the bindings' own base, Lua_Ptr, is left out
func (c *ParseService) LuaParents() map[string][]string {
	out := map[string][]string{}
	for name, content := range c.files {
		base := filepath.Base(filepath.ToSlash(name))
		if !strings.HasPrefix(base, "lua_") || !strings.HasSuffix(base, ".h") {
			continue
		}
		for _, m := range luaClassParent.FindAllStringSubmatch(content, -1) {
			if m[2] != "Ptr" {
				out[m[1]] = []string{m[2]}
			}
		}
	}
	return out
}
