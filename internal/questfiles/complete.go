package questfiles

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/EQEmu/spire/internal/questapi"
)

// Completion is one name the editor offers, such as a method after $client->
type Completion struct {
	Name       string   `json:"name"`
	Params     []string `json:"params"`     // the first signature's, for the inserted call
	Signatures []string `json:"signatures"` // every overload, eg Say(string message)
	Returns    string   `json:"returns,omitempty"`
	Class      string   `json:"class"`               // where it comes from: Client, quest, a plugin file
	ChatArgs   []int    `json:"chat_args,omitempty"` // the arguments that take a chat type
}

// Completions are what the editor offers in one language. A variable's
// methods are its classes' and their parents'; a call's result is the class
// its return type names (types), so the editor can follow
// my $t = $client->GetTarget(); to $t->
type Completions struct {
	Namespaces map[string][]Completion `json:"namespaces"` // "quest::", "plugin::", "eq."
	Classes    map[string][]Completion `json:"classes"`    // each class's own methods
	Parents    map[string][]string     `json:"parents"`    // Client: Mob (direct or all)
	Vars       map[string][]string     `json:"vars"`       // "$client": Client, Mob; "e.self": ...
	Types      map[string]string       `json:"types"`      // a return type as written: its class
	Events     []Completion            `json:"events"`     // offered after "sub " or "function "
	ChatTypes  []questapi.ChatType     `json:"chat_types"`
	Notes      []string                `json:"notes"`
}

// the classes each Perl class inherits from
var classParents = map[string][]string{
	"Client": {"Mob"},
	"NPC":    {"Mob"},
	"Bot":    {"NPC", "Mob"},
	"Merc":   {"NPC", "Mob"},
	"Corpse": {"Mob"},
}

// return types that name a class by another name
var typeAliases = map[string]string{
	"ItemInstance": "QuestItem", // Perl's EQ::ItemInstance*
}

var mobClasses = []string{"Mob", "Client", "NPC", "Bot", "Merc"}

func (k *Checker) Completions(lang string) Completions {
	out := Completions{
		Namespaces: map[string][]Completion{},
		Classes:    map[string][]Completion{},
		Parents:    classParents,
		Vars:       map[string][]string{},
		Types:      map[string]string{},
		Events:     []Completion{},
		ChatTypes:  []questapi.ChatType{},
		Notes:      []string{},
	}
	if k.api == nil {
		return out
	}
	if !k.api.FromLocalSource() {
		out.Notes = append(out.Notes, "The Quest API is upstream EQEmu's: Spire did not find this server's source, so its own methods are not offered.")
	}

	k.api.Parse(false)
	k.api.Read(func(api questapi.Response) {
		classes := map[string][]Completion{}
		add := func(class, name string, params []string, returns string) {
			classes[class] = append(classes[class], Completion{Name: name, Params: params, Returns: returns, Class: class})
		}
		if lang == "lua" {
			for class, list := range api.LuaApi.LuaMethods {
				for _, m := range list {
					add(class, m.Method, m.Params, m.ReturnType)
				}
			}
			for _, e := range api.LuaApi.LuaEvents {
				out.Events = append(out.Events, event(e.EventIdentifier, e.EntityType, e.EventVars, "e."))
			}
			out.Vars["e.self"] = mobClasses
			out.Vars["e.other"] = mobClasses
		} else {
			for class, list := range api.PerlApi.PerlMethods {
				for _, m := range list {
					add(class, m.Method, m.Params, m.ReturnType)
				}
			}
			for _, e := range api.PerlApi.PerlEvents {
				out.Events = append(out.Events, event(e.EventIdentifier, e.EntityType, e.EventVars, "$"))
			}
			for v, cls := range perlVarClasses {
				out.Vars["$"+v] = cls
			}
		}

		for class := range classes {
			list := merge(classes, []string{class})
			switch class {
			case "quest":
				out.Namespaces["quest::"] = list
			case "eq":
				out.Namespaces["eq."] = list
			default:
				out.Classes[class] = list
			}
		}
		// the class each return type names: Mob*, Lua_Client, EQ::ItemInstance*
		for _, list := range classes {
			for _, m := range list {
				if m.Returns == "" {
					continue
				}
				if class := typeClass(m.Returns, out.Classes); class != "" {
					out.Types[m.Returns] = class
				}
			}
		}
	})
	out.Events = mergeEvents(out.Events)
	out.ChatTypes = k.api.ChatTypes()
	// the Lua bindings have their own: Lua_Bot is a Lua_Mob, and Lua_Mob a Lua_Entity
	if lua := k.api.LuaParents(); lang == "lua" && len(lua) > 0 {
		out.Parents = lua
	}

	if lang != "lua" {
		out.Namespaces["plugin::"] = k.pluginCompletions()
	}
	return out
}

func typeClass(returns string, classes map[string][]Completion) string {
	t := strings.TrimSpace(returns)
	for _, cut := range []string{"const ", "EQ::", "Lua_", "*", "&"} {
		t = strings.ReplaceAll(t, cut, "")
	}
	t = strings.TrimSpace(t)
	if a, ok := typeAliases[t]; ok {
		t = a
	}
	if _, ok := classes[t]; ok {
		return t
	}
	return ""
}

var (
	chatParamNames   = map[string]bool{"emote_color_id": true, "color_id": true, "message_type": true, "chat_type": true}
	chatGenericNames = map[string]bool{"type": true, "color": true}
	chatMethodWords  = []string{"message", "marquee", "coloredtext"}
)

// chatArg reports whether a parameter takes a chat type: named for one, or a
// plain type / color of a call that shows a message
func chatArg(method, param string) bool {
	f := strings.Fields(param)
	if len(f) == 0 {
		return false
	}
	name := strings.TrimLeft(f[len(f)-1], "*&$")
	if len(f) > 1 && !strings.Contains(strings.ToLower(strings.Join(f[:len(f)-1], " ")), "int") {
		return false // a string color, say
	}
	if chatParamNames[name] {
		return true
	}
	if !chatGenericNames[name] {
		return false
	}
	lower := strings.ToLower(method)
	if strings.Contains(lower, "popup") {
		return false // popups take an HTML colour
	}
	for _, w := range chatMethodWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// merge lists the methods of classes, the first class's first; overloads
// become one entry with several signatures
func merge(classes map[string][]Completion, order []string) []Completion {
	var out []Completion
	at := map[string]int{}
	for _, class := range order {
		for _, c := range classes[class] {
			sig := c.Name + "(" + strings.Join(c.Params, ", ") + ")"
			for i, p := range c.Params {
				if chatArg(c.Name, p) {
					if j, ok := at[c.Name]; ok {
						if !containsInt(out[j].ChatArgs, i) {
							out[j].ChatArgs = append(out[j].ChatArgs, i)
						}
					} else if !containsInt(c.ChatArgs, i) {
						c.ChatArgs = append(c.ChatArgs, i)
					}
				}
			}
			if i, ok := at[c.Name]; ok {
				if !contains(out[i].Signatures, sig) {
					out[i].Signatures = append(out[i].Signatures, sig)
				}
				continue
			}
			c.Signatures = []string{sig}
			if c.Params == nil {
				c.Params = []string{}
			}
			at[c.Name] = len(out)
			out = append(out, c)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if out == nil {
		out = []Completion{}
	}
	return out
}

func event(name, entity string, vars []string, varPrefix string) Completion {
	shown := make([]string, 0, len(vars))
	for _, v := range vars {
		shown = append(shown, varPrefix+v)
	}
	return Completion{Name: name, Params: []string{}, Signatures: []string{strings.Join(shown, ", ")}, Class: entity}
}

// mergeEvents keeps one entry per event, naming every entity type it runs on
func mergeEvents(events []Completion) []Completion {
	var out []Completion
	at := map[string]int{}
	for _, e := range events {
		if e.Name == "" || strings.HasSuffix(e.Name, "_") {
			continue // an event the parser could not name
		}
		if i, ok := at[e.Name]; ok {
			if e.Class != "" && !strings.Contains(out[i].Class, e.Class) {
				out[i].Class += ", " + e.Class
			}
			continue
		}
		at[e.Name] = len(out)
		out = append(out, e)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if out == nil {
		out = []Completion{}
	}
	return out
}

func containsInt(list []int, n int) bool {
	for _, v := range list {
		if v == n {
			return true
		}
	}
	return false
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

var (
	pluginArgList  = regexp.MustCompile(`my\s*\(([^)]*)\)\s*=\s*@_`)
	pluginArgShift = regexp.MustCompile(`my\s+([$@%]\w+)\s*=\s*shift\b`)
)

// pluginCompletions lists the subs in the plugins folder, with the arguments
// each one unpacks at its top: my ($client, $npc) = @_; or my $x = shift;
func (k *Checker) pluginCompletions() []Completion {
	out := []Completion{}
	seen := map[string]bool{}
	questsDir := k.service.QuestsDir()
	for _, dir := range []string{filepath.Join(questsDir, "plugins"), filepath.Join(filepath.Dir(questsDir), "plugins")} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.pl"))
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			lines := strings.Split(string(data), "\n")
			for i, l := range lines {
				m := perlSubDef.FindStringSubmatch(l)
				if m == nil || seen[m[1]] {
					continue
				}
				seen[m[1]] = true
				params := pluginParams(lines[i:min(i+8, len(lines))])
				out = append(out, Completion{
					Name:       m[1],
					Params:     params,
					Signatures: []string{m[1] + "(" + strings.Join(params, ", ") + ")"},
					Class:      filepath.Base(f),
				})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func pluginParams(body []string) []string {
	params := []string{}
	for i, l := range body {
		if i > 0 && perlSubDef.MatchString(l) {
			break // the next sub
		}
		if m := pluginArgList.FindStringSubmatch(l); m != nil {
			for _, p := range strings.Split(m[1], ",") {
				if p = strings.TrimSpace(p); p != "" {
					params = append(params, p)
				}
			}
			return params
		}
		if m := pluginArgShift.FindStringSubmatch(l); m != nil {
			params = append(params, m[1])
		}
	}
	return params
}
