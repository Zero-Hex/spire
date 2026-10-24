package questapi

import (
	"os"
	"path/filepath"
	"testing"

	gocache "github.com/patrickmn/go-cache"
)

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestReadServerSource(t *testing.T) {
	if readServerSource(t.TempDir()) != nil {
		t.Fatal("accepted a directory that is not a server checkout")
	}

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "zone", "embparser.cpp"), "")
	writeFile(t, filepath.Join(root, "zone", "perl_client.cpp"), "x")
	writeFile(t, filepath.Join(root, "common", "repositories", "a.h"), "y")
	writeFile(t, filepath.Join(root, "zone", "notes.txt"), "skip")
	writeFile(t, filepath.Join(root, "world", "world.cpp"), "skip")

	files := readServerSource(root)
	for _, want := range []string{"zone/embparser.cpp", "zone/perl_client.cpp", "common/repositories/a.h"} {
		if _, ok := files[want]; !ok {
			t.Errorf("missing %s", want)
		}
	}
	if len(files) != 3 {
		t.Errorf("read %d files, want 3: %v", len(files), files)
	}
}

// a parse->Event mention with no call after it, as in a comment, used to
// index past the end of the split and panic
func TestParseEventsIgnoresCommentMentions(t *testing.T) {
	files := map[string]string{
		"zone/gm_commands/x.cpp": "// routes through parse->EventNPC, the same dispatcher\n" +
			"\tparse->EventPlayer(EVENT_LEVEL_UP, this, \"\", 0);\n",
	}
	ps := &ParseService{cache: gocache.New(0, 0)}
	ps.parsePerlEvents(files)
	ps.parseLuaEvents(files)
}
