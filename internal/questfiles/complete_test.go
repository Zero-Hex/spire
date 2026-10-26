package questfiles

import (
	"fmt"
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

func find(list []Completion, name string) Completion {
	for _, c := range list {
		if c.Name == name {
			return c
		}
	}
	return Completion{}
}

func TestPerlCompletions(t *testing.T) {
	k := testChecker(t)
	_ = os.WriteFile(filepath.Join(k.service.QuestsDir(), "plugins", "money.pl"), []byte(
		"sub GiveCoin {\n  my ($client, $amount) = @_;\n}\n\nsub Shifty {\n  my $npc = shift;\n  my $text = shift;\n}\n"), 0644)

	c := k.Completions("perl")
	if got := names(c.Namespaces["quest::"]); got != "SetTint,gmsay,popupcolormessage,say,settimer" {
		t.Errorf("quest:: %v", got)
	}
	if got := names(c.Classes["Client"]); got != "GetLevel,WindowBegin" {
		t.Errorf("Client %v", got)
	}
	if strings.Join(c.Vars["$client"], ",") != "Client,Mob" || strings.Join(c.Parents["Client"], ",") != "Mob" {
		t.Errorf("vars %v parents %v", c.Vars["$client"], c.Parents)
	}
	if c.Types["Mob*"] != "Mob" || c.Types["Client*"] != "Client" {
		t.Errorf("types %v", c.Types)
	}
	plugins := map[string]string{}
	for _, p := range c.Namespaces["plugin::"] {
		plugins[p.Name] = strings.Join(p.Params, " ") + " @" + p.Class
	}
	if plugins["GiveCoin"] != "$client $amount @money.pl" || plugins["Shifty"] != "$npc $text @money.pl" || plugins["ShowScriptWindow"] != " @script_window.pl" {
		t.Errorf("plugin:: %v", plugins)
	}
	if got := names(c.Events); got != "EVENT_SAY,EVENT_SPAWN" {
		t.Errorf("events %v", got)
	}
	if len(c.ChatTypes) != 1 {
		t.Errorf("chat types %v", c.ChatTypes)
	}
}

func TestChatArgs(t *testing.T) {
	c := testChecker(t).Completions("perl")
	got := fmt.Sprint(
		find(c.Classes["Mob"], "Message").ChatArgs,
		find(c.Namespaces["quest::"], "gmsay").ChatArgs,
		find(c.Namespaces["quest::"], "popupcolormessage").ChatArgs, // a string colour
		find(c.Namespaces["quest::"], "SetTint").ChatArgs,           // an armour tint
		len(find(c.Namespaces["quest::"], "gmsay").Signatures),
	)
	if got != "[0] [1] [] [] 2" {
		t.Errorf("got %v", got)
	}
}

func TestLuaCompletions(t *testing.T) {
	c := testChecker(t).Completions("lua")
	if got := names(c.Namespaces["eq."]); got != "set_timer" {
		t.Errorf("eq. %v", got)
	}
	if got := names(c.Classes["Mob"]); got != "GetName,Say" {
		t.Errorf("Mob %v", got)
	}
	if strings.Join(c.Vars["e.self"], ",") != "Mob,Client,NPC,Bot,Merc" {
		t.Errorf("e.self %v", c.Vars)
	}
	if strings.Join(c.Parents["Mob"], ",") != "Entity" {
		t.Errorf("Lua parents %v", c.Parents)
	}
	if _, ok := c.Namespaces["plugin::"]; ok {
		t.Error("Lua has no plugin::")
	}
	if got := names(c.Events); got != "event_say" {
		t.Errorf("events %v", got)
	}
}
