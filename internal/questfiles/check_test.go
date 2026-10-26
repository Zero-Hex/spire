package questfiles

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/EQEmu/spire/internal/questapi"
)

type fakeAPI struct{}

func (fakeAPI) FromLocalSource() bool { return true }

func (fakeAPI) Parse(bool) questapi.Response {
	pm := func(names ...string) []questapi.PerlMethod {
		var out []questapi.PerlMethod
		for _, n := range names {
			out = append(out, questapi.PerlMethod{Method: n})
		}
		return out
	}
	lm := func(names ...string) []questapi.LuaMethod {
		var out []questapi.LuaMethod
		for _, n := range names {
			out = append(out, questapi.LuaMethod{Method: n})
		}
		return out
	}
	var r questapi.Response
	r.PerlApi.PerlMethods = map[string][]questapi.PerlMethod{
		"Client": pm("GetLevel", "WindowBegin"),
		"Mob":    pm("GetName", "Say"),
		"NPC":    pm("GetNPCTypeID"),
		"quest":  pm("say", "settimer"),
	}
	r.PerlApi.PerlEvents = []questapi.PerlEvent{{EventName: "EVENT_SAY", EventIdentifier: "EVENT_SAY"}, {EventName: "EVENT_SPAWN", EventIdentifier: "EVENT_SPAWN"}}
	r.LuaApi.LuaMethods = map[string][]questapi.LuaMethod{
		"Mob": lm("Say", "GetName"),
		"eq":  lm("set_timer"),
	}
	r.LuaApi.LuaEvents = []questapi.LuaEvent{{EventName: "EVENT_SAY", EventIdentifier: "event_say"}}
	return r
}

func testChecker(t *testing.T) *Checker {
	s := testService(t)
	plugins := filepath.Join(s.questsDir, "plugins")
	_ = os.MkdirAll(plugins, 0755)
	_ = os.WriteFile(filepath.Join(plugins, "script_window.pl"), []byte("sub ShowScriptWindow {\n}\nsub check_handin { }\n"), 0644)
	return &Checker{service: s, api: fakeAPI{}}
}

func problems(r CheckResult, source string) []Problem {
	var out []Problem
	for _, p := range r.Problems {
		if p.Source == source {
			out = append(out, p)
		}
	}
	return out
}

func TestPerlSyntaxErrorsHaveLines(t *testing.T) {
	if _, err := exec.LookPath("perl"); err != nil {
		t.Skip("no perl")
	}
	k := testChecker(t)

	r := k.Check("a.pl", "sub EVENT_SAY {\n  if ($text =~ /hail/i) {\n    quest::say(\"Hail!\")\n    quest::say(\"x\");\n  }\n}\n")
	errs := problems(r, "perl")
	if len(errs) == 0 || errs[0].Line != 4 || !strings.Contains(errs[0].Message, "near") {
		t.Fatalf("missing semicolon: %+v", errs)
	}

	r = k.Check("b.pl", "sub EVENT_SAY {\n  if ($text =~ /hail/i) {\n    quest::say(\"Hail!\");\n}\n")
	if errs = problems(r, "perl"); len(errs) == 0 || !strings.Contains(errs[0].Message, "Missing right curly") {
		t.Fatalf("missing brace: %+v", errs)
	}

	r = k.Check("c.pl", "sub EVENT_SAY {\n  quest::say(\"ok\");\n}\n")
	if len(r.Problems) != 0 {
		t.Fatalf("a valid script has problems: %+v", r.Problems)
	}
}

func TestLuaSyntaxErrors(t *testing.T) {
	k := testChecker(t)
	r := k.Check("a.lua", "local a = 1\nlocal x = = 2\n")
	if errs := problems(r, "lua"); len(errs) != 1 || errs[0].Line != 2 {
		t.Fatalf("mid-file error: %+v", errs)
	}
	r = k.Check("b.lua", "function event_say(e)\n  if true then\n    e.self:Say('hi')\nend\n")
	if errs := problems(r, "lua"); len(errs) != 1 || errs[0].Line != 4 || !strings.Contains(errs[0].Message, "'end'") {
		t.Fatalf("missing end: %+v", errs)
	}
	r = k.Check("c.lua", "for i = 1, 3 do\n  if i == 2 then goto continue end\n  ::continue::\nend\n")
	if len(problems(r, "lua")) != 0 {
		t.Fatalf("LuaJIT goto was rejected: %+v", r.Problems)
	}
}

func TestQuestAPITypos(t *testing.T) {
	k := testChecker(t)
	r := k.Check("a.pl", strings.Join([]string{
		"sub EVENT_SAY {",                         // 1
		"  my $l = $client->GetLevle();",          // 2 typo
		"  $client->GetName();",                   // 3 Mob method on a client: fine
		"  $npc->getnpctypeid();",                 // 4 wrong case
		"  quest::say('x'); quest::sya('y');",     // 5 typo
		"  plugin::ShowScriptWindow($client);",    // 6 defined in plugins
		"  plugin::ShowScriptWindo($client);",     // 7 typo
		"  # $client->NotAMethod() in a comment",  // 8 ignored
		"  my $db = $dbh->prepare('x');",          // 9 not a quest variable
		"  $client->Message(15, 'it\\'s # fine');", // 10 Message unknown in the fake API
		"}",
		"sub EVENT_SAYY { }", // 12 not an event
	}, "\n"))

	var got []string
	for _, p := range problems(r, "quest-api") {
		got = append(got, itoaLine(p.Line)+": "+p.Message)
	}
	want := []string{
		"2: $client has no method GetLevle (Client, Mob); did you mean GetLevel?",
		"4: $npc has no method getnpctypeid (NPC, Mob); did you mean GetNPCTypeID? (names are case-sensitive)",
		"5: quest::sya is not in the Quest API; did you mean say?",
		"7: plugin::ShowScriptWindo is not defined in the plugins folder; did you mean ShowScriptWindow?",
		"10: $client has no method Message (Client, Mob)",
		"12: EVENT_SAYY is not a Perl event, so this sub never runs; did you mean EVENT_SAY?",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%v\nwant:\n%v", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	r = k.Check("b.lua", "function event_sya(e)\n  e.self:Sya('x') -- e.self:Bogus()\n  eq.set_timer('a', 1)\n  eq.set_timr('a', 1)\nend\n")
	got = nil
	for _, p := range problems(r, "quest-api") {
		got = append(got, itoaLine(p.Line)+": "+p.Message)
	}
	want = []string{
		"1: event_sya is not a Lua event, so this function never runs; did you mean event_say?",
		"2: e.self has no method Sya; did you mean Say?",
		"4: eq.set_timr is not in the Quest API; did you mean set_timer?",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("lua got:\n%v\nwant:\n%v", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func itoaLine(n int) string {
	return strconv.Itoa(n)
}

func TestLuaSupersedesSameNamedPerl(t *testing.T) {
	s := testService(t)
	_ = os.WriteFile(filepath.Join(s.questsDir, "qeynos", "Fippy.pl"), []byte("sub EVENT_SAY {}"), 0644)

	l, _ := s.List("qeynos")
	flags := map[string]string{}
	for _, f := range l.Files {
		flags[f.Name] = f.SupersededBy + "|" + f.Supersedes
	}
	if flags["Fippy.pl"] != "Fippy.lua|" || flags["Fippy.lua"] != "|Fippy.pl" || flags["Guard_Phil.pl"] != "|" {
		t.Fatalf("flags: %v", flags)
	}
	sc, _ := s.Read("qeynos/Fippy.pl")
	if sc.SupersededBy != "Fippy.lua" {
		t.Errorf("read: %+v", sc.SupersededBy)
	}
}
