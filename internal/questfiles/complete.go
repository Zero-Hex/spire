package questfiles

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/EQEmu/spire/internal/questapi"
)

// Completion is one name the editor offers after a trigger such as quest::
type Completion struct {
	Name       string   `json:"name"`
	Params     []string `json:"params"`     // the first signature's, for the inserted call
	Signatures []string `json:"signatures"` // every overload, eg Say(string message)
	Returns    string   `json:"returns,omitempty"`
	Class      string   `json:"class"` // where it comes from: Client, quest, a plugin file
}

// Completions are the names offered in one language, keyed by what precedes
// them: "quest::", "$client->", "eq.", "e.self:", ... and the events, offered
// after "sub " or "function "
type Completions struct {
	Triggers map[string][]Completion `json:"triggers"`
	Events   []Completion            `json:"events"`
	Notes    []string                `json:"notes"`
}

// the Lua event table fields that hold a mob
var luaMobVars = []string{"self", "other"}

func (k *Checker) Completions(lang string) Completions {
	out := Completions{Triggers: map[string][]Completion{}, Events: []Completion{}, Notes: []string{}}
	if k.api == nil {
		return out
	}
	if !k.api.FromLocalSource() {
		out.Notes = append(out.Notes, "The Quest API is upstream EQEmu's: Spire did not find this server's source, so its own methods are not offered.")
	}

	k.api.Parse(false)
	k.api.Read(func(api questapi.Response) {
		if lang == "lua" {
			classes := map[string][]Completion{}
			for class, list := range api.LuaApi.LuaMethods {
				for _, m := range list {
					classes[class] = append(classes[class], Completion{Name: m.Method, Params: m.Params, Returns: m.ReturnType, Class: class})
				}
			}
			out.Triggers["eq."] = merge(classes, []string{"eq"})
			for _, v := range luaMobVars {
				out.Triggers["e."+v+":"] = merge(classes, []string{"Mob", "Client", "NPC", "Bot", "Merc"})
			}
			for _, e := range api.LuaApi.LuaEvents {
				out.Events = append(out.Events, event(e.EventIdentifier, e.EntityType, e.EventVars, "e."))
			}
			return
		}

		classes := map[string][]Completion{}
		for class, list := range api.PerlApi.PerlMethods {
			for _, m := range list {
				classes[class] = append(classes[class], Completion{Name: m.Method, Params: m.Params, Returns: m.ReturnType, Class: class})
			}
		}
		out.Triggers["quest::"] = merge(classes, []string{"quest"})
		for v, cls := range perlVarClasses {
			out.Triggers["$"+v+"->"] = merge(classes, cls)
		}
		for _, e := range api.PerlApi.PerlEvents {
			out.Events = append(out.Events, event(e.EventIdentifier, e.EntityType, e.EventVars, "$"))
		}
	})
	out.Events = mergeEvents(out.Events)

	if lang != "lua" {
		out.Triggers["plugin::"] = k.pluginCompletions()
	}
	return out
}

// merge lists the methods of classes, the first class's first; overloads
// become one entry with several signatures
func merge(classes map[string][]Completion, order []string) []Completion {
	var out []Completion
	at := map[string]int{}
	for _, class := range order {
		for _, c := range classes[class] {
			sig := c.Name + "(" + strings.Join(c.Params, ", ") + ")"
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
