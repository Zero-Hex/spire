package questfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func names(list []Completion) string {
	var out []string
	for _, c := range list {
		out = append(out, c.Name)
	}
	return strings.Join(out, ",")
}

func TestPerlCompletions(t *testing.T) {
	k := testChecker(t)
	_ = os.WriteFile(filepath.Join(k.service.QuestsDir(), "plugins", "money.pl"), []byte(
		"sub GiveCoin {\n  my ($client, $amount) = @_;\n}\n\nsub Shifty {\n  my $npc = shift;\n  my $text = shift;\n}\n"), 0644)

	c := k.Completions("perl")
	if got := names(c.Triggers["quest::"]); got != "say,settimer" {
		t.Errorf("quest:: %v", got)
	}
	// Client methods and the Mob methods a client also has
	if got := names(c.Triggers["$client->"]); got != "GetLevel,GetName,Say,WindowBegin" {
		t.Errorf("$client-> %v", got)
	}
	if got := names(c.Triggers["$npc->"]); got != "GetNPCTypeID,GetName,Say" {
		t.Errorf("$npc-> %v", got)
	}
	plugins := map[string]string{}
	for _, p := range c.Triggers["plugin::"] {
		plugins[p.Name] = strings.Join(p.Params, " ") + " @" + p.Class
	}
	if plugins["GiveCoin"] != "$client $amount @money.pl" || plugins["Shifty"] != "$npc $text @money.pl" || plugins["ShowScriptWindow"] != " @script_window.pl" {
		t.Errorf("plugin:: %v", plugins)
	}
	if got := names(c.Events); got != "EVENT_SAY,EVENT_SPAWN" {
		t.Errorf("events %v", got)
	}
}

func TestLuaCompletions(t *testing.T) {
	c := testChecker(t).Completions("lua")
	if got := names(c.Triggers["eq."]); got != "set_timer" {
		t.Errorf("eq. %v", got)
	}
	if got := names(c.Triggers["e.self:"]); got != "GetName,Say" {
		t.Errorf("e.self: %v", got)
	}
	if _, ok := c.Triggers["plugin::"]; ok {
		t.Error("Lua has no plugin::")
	}
	if got := names(c.Events); got != "event_say" {
		t.Errorf("events %v", got)
	}
}
