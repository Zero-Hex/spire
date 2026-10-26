package questfiles

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/EQEmu/spire/internal/questapi"
	"github.com/yuin/gopher-lua/parse"
)

// Problem is one thing the check found in a script
type Problem struct {
	Line     int    `json:"line"` // 1-based; 0 = the whole file
	Column   int    `json:"column"`
	Severity string `json:"severity"` // error, warning
	Source   string `json:"source"`   // perl, lua, quest-api
	Message  string `json:"message"`
}

type CheckResult struct {
	Problems []Problem `json:"problems"`
	Checks   []string  `json:"checks"` // what ran
	Notes    []string  `json:"notes"`  // what could not run, and why
}

// Checker finds syntax errors (perl -c, a Lua 5.1 parser) and calls the
// Quest API does not have: a misspelt method only fails when its line runs
type Checker struct {
	service *Service
	api     questAPI
}

// questAPI is what the checks need of questapi.ParseService
type questAPI interface {
	FromLocalSource() bool
	Parse(forceRefresh bool) questapi.Response
}

func NewChecker(service *Service, api *questapi.ParseService) *Checker {
	return &Checker{service: service, api: api}
}

const perlCheckTimeout = 10 * time.Second

func (k *Checker) Check(path, content string) CheckResult {
	r := CheckResult{Problems: []Problem{}, Checks: []string{}, Notes: []string{}}
	if strings.EqualFold(filepath.Ext(path), ".lua") {
		k.checkLuaSyntax(content, &r)
		k.lintLua(content, &r)
	} else {
		k.checkPerlSyntax(content, &r)
		k.lintPerl(content, &r)
	}
	sort.SliceStable(r.Problems, func(i, j int) bool { return r.Problems[i].Line < r.Problems[j].Line })
	return r
}

// ------------------------------------------------------------------ Perl

// "<message> at script.pl line N, near "<text that may span lines>"" or
// "<message> at script.pl line N, at end of line"
var perlMessage = regexp.MustCompile(`(?s)(.+?) at script\.pl line (\d+)(?:, near "(.*?)"\n|[^\n]*\n)`)

// checkPerlSyntax compiles the script with the server's perl. -c runs BEGIN
// blocks and use statements, so it gets a time limit; anyone who can edit
// quests can already run code through the zone.
func (k *Checker) checkPerlSyntax(content string, r *CheckResult) {
	perl, err := exec.LookPath("perl")
	if err != nil {
		r.Notes = append(r.Notes, "No perl on this server, so Perl syntax is not checked.")
		return
	}

	dir, err := os.MkdirTemp("", "spire-perl-check-")
	if err != nil {
		r.Notes = append(r.Notes, "Perl check failed to start: "+err.Error())
		return
	}
	defer os.RemoveAll(dir)
	if err := os.WriteFile(filepath.Join(dir, "script.pl"), []byte(content), 0600); err != nil {
		r.Notes = append(r.Notes, "Perl check failed to start: "+err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), perlCheckTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, perl, "-c", "script.pl")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		r.Notes = append(r.Notes, fmt.Sprintf("The Perl check gave up after %v (a BEGIN block or use statement that does not return).", perlCheckTimeout))
		return
	}
	r.Checks = append(r.Checks, "perl -c")
	if err == nil {
		return
	}

	text := strings.ReplaceAll(string(out), "\r\n", "\n")
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	found := false
	for _, m := range perlMessage.FindAllStringSubmatch(text, -1) {
		line, _ := strconv.Atoi(m[2])
		msg := strings.TrimSpace(m[1])
		if near := strings.Join(strings.Fields(m[3]), " "); near != "" {
			msg += ` near "` + near + `"`
		}
		r.Problems = append(r.Problems, Problem{Line: line, Severity: "error", Source: "perl", Message: msg})
		found = true
	}
	if !found {
		msg := strings.TrimSpace(strings.ReplaceAll(text, "script.pl had compilation errors.", ""))
		if msg == "" {
			msg = "Perl could not compile the script."
		}
		r.Problems = append(r.Problems, Problem{Severity: "error", Source: "perl", Message: msg})
	}
}

// pod (=head1 ... =cut) and comments are blanked, keeping line numbers
func stripPerlComments(content string) []string {
	lines := strings.Split(content, "\n")
	inPod := false
	for i, l := range lines {
		if strings.HasPrefix(l, "=") && len(l) > 1 && l[1] >= 'a' && l[1] <= 'z' {
			inPod = !strings.HasPrefix(l, "=cut")
			lines[i] = ""
			continue
		}
		if inPod {
			lines[i] = ""
			continue
		}
		lines[i] = cutComment(l, '#', "")
	}
	return lines
}

// cutComment drops a comment from a line, ignoring the marker inside quotes
// (a rough cut: good enough for warnings)
func cutComment(l string, marker byte, marker2 string) string {
	var quote byte
	for i := 0; i < len(l); i++ {
		c := l[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case marker2 != "" && strings.HasPrefix(l[i:], marker2):
			return l[:i]
		case marker2 == "" && c == marker:
			// $# (last index) and s#..# style delimiters are not comments
			if i > 0 && (l[i-1] == '$' || l[i-1] == 's' || l[i-1] == 'q' || l[i-1] == 'm') {
				continue
			}
			return l[:i]
		}
	}
	return l
}

// the quest variables the zone exports, and the API classes they can hold
var perlVarClasses = map[string][]string{
	"client":      {"Client", "Mob"},
	"npc":         {"NPC", "Mob"},
	"bot":         {"Bot", "NPC", "Mob"},
	"merc":        {"Merc", "NPC", "Mob"},
	"mob":         {"Mob", "Client", "NPC", "Bot", "Merc"},
	"other":       {"Mob", "Client", "NPC", "Bot", "Merc"},
	"target":      {"Mob", "Client", "NPC", "Bot", "Merc"},
	"corpse":      {"Corpse", "Mob"},
	"entity_list": {"EntityList"},
	"zone":        {"Zone"},
	"questitem":   {"QuestItem"},
	"spell":       {"Spell"},
	"group":       {"Group"},
	"raid":        {"Raid"},
	"object":      {"Object"},
	"door":        {"Doors"},
	"inventory":   {"Inventory"},
}

var (
	perlVarCall   = regexp.MustCompile(`\$([a-z_]+)\s*->\s*([A-Za-z_]\w*)\s*\(`)
	perlQuestCall = regexp.MustCompile(`\bquest::([A-Za-z_]\w*)`)
	perlPluginRef = regexp.MustCompile(`\bplugin::([A-Za-z_]\w*)`)
	perlEventSub  = regexp.MustCompile(`^\s*sub\s+(EVENT_\w+)`)
	perlSubDef    = regexp.MustCompile(`(?m)^\s*sub\s+([A-Za-z_]\w*)`)
)

func (k *Checker) lintPerl(content string, r *CheckResult) {
	if k.api == nil || !k.api.FromLocalSource() {
		r.Notes = append(r.Notes, "Quest API checks are off: Spire has not parsed the API from this server's source, so it would not know the server's own methods.")
		return
	}
	api := k.api.Parse(false)

	methods := map[string]map[string]bool{}
	for class, list := range api.PerlApi.PerlMethods {
		methods[class] = map[string]bool{}
		for _, m := range list {
			methods[class][m.Method] = true
		}
	}
	events := map[string]bool{}
	for _, e := range api.PerlApi.PerlEvents {
		events[e.EventIdentifier] = true
		events[e.EventName] = true
	}
	plugins, pluginsFound := k.pluginSubs()
	r.Checks = append(r.Checks, "Quest API")

	warn := func(line int, col int, msg string) {
		r.Problems = append(r.Problems, Problem{Line: line, Column: col, Severity: "warning", Source: "quest-api", Message: msg})
	}

	for i, l := range stripPerlComments(content) {
		line := i + 1
		for _, m := range perlVarCall.FindAllStringSubmatchIndex(l, -1) {
			v, name := l[m[2]:m[3]], l[m[4]:m[5]]
			classes, ok := perlVarClasses[v]
			if !ok {
				continue
			}
			if !known(name, classes, methods) {
				warn(line, m[4]+1, fmt.Sprintf("$%s has no method %s (%s)%s", v, name, strings.Join(classes, ", "), suggest(name, union(classes, methods))))
			}
		}
		for _, m := range perlQuestCall.FindAllStringSubmatchIndex(l, -1) {
			name := l[m[2]:m[3]]
			if !methods["quest"][name] {
				warn(line, m[2]+1, "quest::"+name+" is not in the Quest API"+suggest(name, methods["quest"]))
			}
		}
		if pluginsFound {
			for _, m := range perlPluginRef.FindAllStringSubmatchIndex(l, -1) {
				name := l[m[2]:m[3]]
				if !plugins[name] {
					warn(line, m[2]+1, "plugin::"+name+" is not defined in the plugins folder"+suggest(name, plugins))
				}
			}
		}
		if m := perlEventSub.FindStringSubmatchIndex(l); m != nil {
			name := l[m[2]:m[3]]
			if !events[name] {
				warn(line, m[2]+1, name+" is not a Perl event, so this sub never runs"+suggest(name, events))
			}
		}
	}
}

// pluginSubs are the subs defined in the server's Perl plugins
func (k *Checker) pluginSubs() (map[string]bool, bool) {
	subs := map[string]bool{}
	found := false
	questsDir := k.service.QuestsDir()
	for _, dir := range []string{filepath.Join(questsDir, "plugins"), filepath.Join(filepath.Dir(questsDir), "plugins")} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.pl"))
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			found = true
			for _, m := range perlSubDef.FindAllStringSubmatch(string(data), -1) {
				subs[m[1]] = true
			}
		}
	}
	return subs, found
}

// ------------------------------------------------------------------ Lua

var luaPos = regexp.MustCompile(`line:(\d+)\(column:(\d+)\)\s*(.*)`)

// checkLuaSyntax parses with a Lua 5.1 parser, the language LuaJIT runs
func (k *Checker) checkLuaSyntax(content string, r *CheckResult) {
	r.Checks = append(r.Checks, "Lua syntax")
	_, err := parse.Parse(strings.NewReader(content), "script.lua")
	if err == nil {
		return
	}

	text := strings.TrimSpace(strings.TrimPrefix(err.Error(), "script.lua"))
	text = strings.Join(strings.Fields(text), " ")
	if m := luaPos.FindStringSubmatch(text); m != nil {
		line, _ := strconv.Atoi(m[1])
		col, _ := strconv.Atoi(m[2])
		r.Problems = append(r.Problems, Problem{Line: line, Column: col + 1, Severity: "error", Source: "lua", Message: strings.TrimSpace(strings.TrimPrefix(m[3], ":"))})
		return
	}
	// "at EOF": the file ended inside a block
	lines := strings.Count(strings.TrimRight(content, "\n"), "\n") + 1
	r.Problems = append(r.Problems, Problem{Line: lines, Severity: "error", Source: "lua", Message: "The file ends inside a block: an 'end' (or ')' / '}') is missing"})
}

// comments (-- and --[[ ]]) are blanked, keeping line numbers
func stripLuaComments(content string) []string {
	lines := strings.Split(content, "\n")
	inBlock := false
	for i, l := range lines {
		if inBlock {
			if j := strings.Index(l, "]]"); j >= 0 {
				inBlock = false
				l = strings.Repeat(" ", j+2) + l[j+2:]
			} else {
				lines[i] = ""
				continue
			}
		}
		cut := cutComment(l, 0, "--")
		if strings.HasPrefix(l[len(cut):], "--[[") && !strings.Contains(l[len(cut)+4:], "]]") {
			inBlock = true
		}
		lines[i] = cut
	}
	return lines
}

var (
	luaEqCall    = regexp.MustCompile(`\beq\.([A-Za-z_]\w*)\s*\(`)
	luaEventFunc = regexp.MustCompile(`^\s*function\s+(event_\w+)\s*\(`)
	luaSelfCall  = regexp.MustCompile(`\be\.(self|other)\s*:\s*([A-Za-z_]\w*)\s*\(`)
)

func (k *Checker) lintLua(content string, r *CheckResult) {
	if k.api == nil || !k.api.FromLocalSource() {
		r.Notes = append(r.Notes, "Quest API checks are off: Spire has not parsed the API from this server's source, so it would not know the server's own methods.")
		return
	}
	api := k.api.Parse(false)

	methods := map[string]map[string]bool{}
	for class, list := range api.LuaApi.LuaMethods {
		methods[class] = map[string]bool{}
		for _, m := range list {
			methods[class][m.Method] = true
		}
	}
	events := map[string]bool{}
	for _, e := range api.LuaApi.LuaEvents {
		events[e.EventIdentifier] = true
	}
	mobClasses := []string{"Mob", "Client", "NPC", "Bot", "Merc"}
	r.Checks = append(r.Checks, "Quest API")

	warn := func(line int, col int, msg string) {
		r.Problems = append(r.Problems, Problem{Line: line, Column: col, Severity: "warning", Source: "quest-api", Message: msg})
	}
	for i, l := range stripLuaComments(content) {
		line := i + 1
		for _, m := range luaEqCall.FindAllStringSubmatchIndex(l, -1) {
			name := l[m[2]:m[3]]
			if !methods["eq"][name] {
				warn(line, m[2]+1, "eq."+name+" is not in the Quest API"+suggest(name, methods["eq"]))
			}
		}
		for _, m := range luaSelfCall.FindAllStringSubmatchIndex(l, -1) {
			who, name := l[m[2]:m[3]], l[m[4]:m[5]]
			if !known(name, mobClasses, methods) {
				warn(line, m[4]+1, fmt.Sprintf("e.%s has no method %s%s", who, name, suggest(name, union(mobClasses, methods))))
			}
		}
		if m := luaEventFunc.FindStringSubmatchIndex(l); m != nil {
			name := l[m[2]:m[3]]
			if !events[name] {
				warn(line, m[2]+1, name+" is not a Lua event, so this function never runs"+suggest(name, events))
			}
		}
	}
}

// ------------------------------------------------------------------ helpers

func known(name string, classes []string, methods map[string]map[string]bool) bool {
	for _, c := range classes {
		if methods[c][name] {
			return true
		}
	}
	return false
}

func union(classes []string, methods map[string]map[string]bool) map[string]bool {
	out := map[string]bool{}
	for _, c := range classes {
		for m := range methods[c] {
			out[m] = true
		}
	}
	return out
}

// suggest names the closest known name, if one is close: a typo
func suggest(name string, names map[string]bool) string {
	best, bestDist := "", 3
	lower := strings.ToLower(name)
	for n := range names {
		if strings.ToLower(n) == lower {
			return "; did you mean " + n + "? (names are case-sensitive)"
		}
		if d := distance(lower, strings.ToLower(n)); d < bestDist || (d == bestDist && best != "" && n < best) {
			best, bestDist = n, d
		}
	}
	if best == "" {
		return ""
	}
	return "; did you mean " + best + "?"
}

// distance is the Levenshtein edit distance
func distance(a, b string) int {
	if a == b {
		return 0
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
