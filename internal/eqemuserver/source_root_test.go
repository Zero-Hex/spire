package eqemuserver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceRootFromCMakeCache(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "CMakeCache.txt")
	if err := os.WriteFile(cache, []byte("CMAKE_MAKE_PROGRAM:FILEPATH=/usr/bin/ninja\nCMAKE_HOME_DIRECTORY:INTERNAL=/home/eqemu/code\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if root := SourceRootFromCMakeCache(cache); root != "/home/eqemu/code" {
		t.Fatalf("got %q", root)
	}
	if root := SourceRootFromCMakeCache(filepath.Join(t.TempDir(), "missing")); root != "" {
		t.Fatalf("missing cache gave %q", root)
	}
}
