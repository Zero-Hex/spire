package questapi

import (
	"os"
	"testing"
)

func TestParseChatTypes(t *testing.T) {
	constants := "namespace Zones {\n const uint16 X = 9;\n};\nnamespace Chat {\n\tconst uint16 White = 0;\n\tconst uint16 DimGray = 1;\n\tconst uint16 Default = 1;\n\tconst uint16 Yellow = 15;\n};\n"
	lua := "luabind::scope lua_register_message_types() {\n\treturn luabind::class_<MessageTypes>(\"MT\")\n\t[(\n\t\tluabind::value(\"White\", Chat::White),\n\t\tluabind::value(\"Yellow\", Chat::Yellow)\n\t)];\n}\n"
	got := ParseChatTypes(constants, lua)
	if len(got) != 3 || got[1].Value != 1 || len(got[1].Names) != 2 || got[1].Names[1] != "Default" || got[2].Lua[0] != "MT.Yellow" || len(got[1].Lua) != 0 {
		t.Fatalf("%+v", got)
	}
}

// the real source, when it is checked out next to this one
func TestParseChatTypesFromStarfall(t *testing.T) {
	c, err1 := os.ReadFile("../../../starfall-3/common/eq_constants.h")
	l, err2 := os.ReadFile("../../../starfall-3/zone/lua_general.cpp")
	if err1 != nil || err2 != nil {
		t.Skip("no server source")
	}
	got := ParseChatTypes(string(c), string(l))
	lua := 0
	for _, ct := range got {
		lua += len(ct.Lua)
	}
	if len(got) < 90 || got[0].Names[0] != "White" || lua < 90 {
		t.Fatalf("%d types, %d Lua names: %+v", len(got), lua, got[:3])
	}
	t.Logf("%d types, %d Lua names; last %+v", len(got), lua, got[len(got)-1])
}
