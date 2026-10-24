package questfiles

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testService(t *testing.T) *Service {
	base := t.TempDir()
	s := &Service{questsDir: filepath.Join(base, "quests"), backupsDir: filepath.Join(base, "spire-quest-backups")}
	write := func(rel, content string) {
		p := filepath.Join(s.questsDir, filepath.FromSlash(rel))
		_ = os.MkdirAll(filepath.Dir(p), 0755)
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("qeynos/Guard_Phil.pl", "sub EVENT_SAY {}\n")
	write("qeynos/Fippy.lua", "function event_say(e) end\n")
	write("qeynos/notes.txt", "not a script")
	write("qeynos/.hidden.pl", "")
	write("global/global_player.pl", "")
	_ = os.WriteFile(filepath.Join(base, "secret.pl"), []byte("outside"), 0644)
	return s
}

func TestPathsStayInsideQuests(t *testing.T) {
	s := testService(t)
	for _, bad := range []string{"../secret.pl", "qeynos/../../secret.pl", "qeynos/.hidden.pl", "qeynos/notes.txt"} {
		if _, err := s.Read(bad); err == nil {
			t.Errorf("%v was readable", bad)
		}
		if _, err := s.Save(bad, "x", 0, ""); err == nil {
			t.Errorf("%v was writable", bad)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(filepath.Dir(s.questsDir), "secret.pl")); string(b) != "outside" {
		t.Error("the file outside the quests folder was changed")
	}
	if _, err := os.Stat(filepath.Join(s.questsDir, "secret.pl")); !os.IsNotExist(err) {
		t.Error("a .. path was quietly written inside the quests folder")
	}
	if _, err := s.Read("/qeynos/Guard_Phil.pl"); err != nil {
		t.Errorf("a leading slash should still be inside quests: %v", err)
	}
}

func TestListShowsScriptsAndFolders(t *testing.T) {
	s := testService(t)
	root, err := s.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Dirs) != 2 || len(root.Files) != 0 {
		t.Fatalf("root: %+v", root)
	}
	q, _ := s.List("qeynos")
	var names []string
	for _, f := range q.Files {
		names = append(names, f.Name)
	}
	if strings.Join(names, ",") != "Fippy.lua,Guard_Phil.pl" {
		t.Errorf("qeynos lists %v; only visible .pl and .lua should show", names)
	}
}

func TestSaveBacksUpAndRefusesStaleWrites(t *testing.T) {
	s := testService(t)
	opened, _ := s.Read("qeynos/Guard_Phil.pl")

	saved, err := s.Save("qeynos/Guard_Phil.pl", "sub EVENT_SAY { quest::say('hi'); }\n", opened.Modified, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(saved.Content, "quest::say") {
		t.Fatal("new content was not written")
	}
	backups, _ := s.Backups("qeynos/Guard_Phil.pl")
	if len(backups) != 1 {
		t.Fatalf("want 1 backup, got %v", backups)
	}
	old, _ := s.ReadBackup("qeynos/Guard_Phil.pl", backups[0])
	if old != "sub EVENT_SAY {}\n" {
		t.Errorf("backup holds %q", old)
	}

	// a second editor still holding the first version is refused
	if _, err := s.Save("qeynos/Guard_Phil.pl", "stale", opened.Modified, ""); !errors.Is(err, ErrConflict) {
		t.Errorf("stale save: want a conflict, got %v", err)
	}
	// a new file needs modified 0, and its folder must exist
	if _, err := s.Save("qeynos/New_NPC.pl", "sub EVENT_SAY {}", 0, ""); err != nil {
		t.Errorf("new file: %v", err)
	}
	if _, err := s.Save("nowhere/New_NPC.pl", "x", 0, ""); err == nil {
		t.Error("a file in a missing folder was created")
	}
	if _, err := s.Save("qeynos/Guard_Phil.pl", "x", 0, ""); !errors.Is(err, ErrConflict) {
		t.Errorf("overwriting with modified 0: want a conflict, got %v", err)
	}
}

func TestSaveAsPerlRetiresTheLuaScript(t *testing.T) {
	s := testService(t)
	if _, err := s.Save("qeynos/Fippy.pl", "sub EVENT_SAY {}\n", 0, "qeynos/Fippy.lua"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.questsDir, "qeynos", "Fippy.lua")); !os.IsNotExist(err) {
		t.Error("the .lua is still there, and the zone would load it ahead of the .pl")
	}
	if b, _ := s.Backups("qeynos/Fippy.lua"); len(b) != 1 {
		t.Errorf("the .lua was not backed up: %v", b)
	}
	if _, err := s.Save("qeynos/Fippy.pl", "x", 0, "global/global_player.pl"); err == nil {
		t.Error("a script in another folder was retired")
	}
}

func TestBackupsArePruned(t *testing.T) {
	s := testService(t)
	for i := 0; i < keepBackups+5; i++ {
		cur, _ := s.Read("qeynos/Guard_Phil.pl")
		if _, err := s.Save("qeynos/Guard_Phil.pl", strings.Repeat("x", i+1), cur.Modified, ""); err != nil {
			t.Fatal(err)
		}
	}
	if b, _ := s.Backups("qeynos/Guard_Phil.pl"); len(b) != keepBackups {
		t.Errorf("want %d backups kept, got %d", keepBackups, len(b))
	}
}
