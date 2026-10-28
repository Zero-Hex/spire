package clientassets

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

func testZip(t *testing.T, files map[string][]byte) []byte {
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, data := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write(data)
	}
	_ = w.Close()
	return b.Bytes()
}

func TestZipIsListedThenPicked(t *testing.T) {
	s := testStore(t)
	z := testZip(t, map[string][]byte{
		"MyWeapon/IT9001.eqg":                     buildPFS(t, []string{"it9001.mod", "sword.dds"}),
		"MyWeapon/dragitem180.png":                testSheet(t),
		"MyWeapon/uifiles/default/EQUI_Sword.xml": []byte("<x/>"),
		"MyWeapon/notes/readme.txt":               []byte("hi"),
		"MyWeapon/odd.bin":                        []byte("?"),
		"MyWeapon/spells_us.txt":                  []byte("x"),
		"MyWeapon/setup.exe":                      []byte("MZ"),
		"__MACOSX/MyWeapon/._IT9001.eqg":          []byte("junk"),
	})
	l, err := s.StageZip("MyWeapon.zip", bytes.NewReader(z))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ZipEntry{}
	for _, e := range l.Entries {
		got[e.Path] = e
	}
	check := func(path, kind, folder string, needsFolder bool) {
		e, ok := got[path]
		if !ok || e.Kind != kind || e.Folder != folder || e.NeedsFolder != needsFolder {
			t.Errorf("%v: %+v", path, e)
		}
	}
	check("IT9001.eqg", "models", "", false)
	check("dragitem180.png", "sheet", "uifiles/default", false)
	check("uifiles/default/EQUI_Sword.xml", "file", "uifiles/default", false)
	check("notes/readme.txt", "file", "notes", false)
	check("odd.bin", "file", "", true)
	check("spells_us.txt", "refused", "", false)
	check("setup.exe", "refused", "", false)
	if len(l.Entries) != 7 || !strings.Contains(got["IT9001.eqg"].Note, "IT9001") {
		t.Errorf("%d entries: %+v", len(l.Entries), got["IT9001.eqg"])
	}

	results, err := s.ImportZip(l.ID, []ZipPick{
		{Entry: got["IT9001.eqg"].Entry},
		{Entry: got["dragitem180.png"].Entry, Folder: "uifiles/default"},
		{Entry: got["odd.bin"].Entry, Folder: ""}, // chosen: next to eqgame.exe
		{Entry: got["uifiles/default/EQUI_Sword.xml"].Entry, Folder: "uifiles/default", Remove: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Error != "" {
			t.Errorf("%v: %v", r.File, r.Error)
		}
	}
	idx := s.Index()
	if idx.Archives["it9001.eqg"] == nil || idx.Sheets[180] == nil || idx.Files["odd.bin"] == nil {
		t.Fatalf("not imported: %+v", idx)
	}
	if f := idx.Files["uifiles/default/equi_sword.xml"]; f == nil || !f.Remove || f.Size != 0 {
		t.Errorf("the removal: %+v", f)
	}
	if _, err := s.ImportZip("../../index", nil); err == nil {
		t.Error("a path was taken for an upload id")
	}
}

func TestRemovalRules(t *testing.T) {
	s := testStore(t)
	_, _ = s.Import("dragitem12.png", testSheet(t))
	_, _ = s.Import("dragitem180.png", testSheet(t))
	_, _ = s.ImportAt("uifiles/default", "EQUI_Sword.xml", []byte("<x/>"))

	for path, want := range map[string]string{
		"uifiles/default/dragitem12.dds": "stock icon sheet",
		"spells_us.txt":                  "database",
		"Resources/SkillCaps.txt":        "database",
		"uifiles/default/EQUI.xml":       "cannot load",
		"eqgame.exe":                     "programs",
		"../../etc/passwd":               "not a folder",
		"":                               "no file",
	} {
		if _, err := s.AddRemoval(path); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: %v", path, err)
		}
	}

	// a held file switches to Remove and back, keeping its copy
	if err := s.SetRemove("uifiles/default/equi_sword.xml", true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRemove("uifiles/default/dragitem180.dds", true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddRemoval("IT1234.eqg"); err != nil {
		t.Fatal(err)
	}
	files, _ := s.PatchFiles()
	state := map[string]string{}
	for _, f := range files {
		state[f.Path] = f.Kind
		if f.Remove {
			state[f.Path] += " remove"
		}
	}
	if state["uifiles/default/EQUI_Sword.xml"] != "file remove" || state["uifiles/default/dragitem180.dds"] != "sheet remove" ||
		state["IT1234.eqg"] != "removal remove" || state["uifiles/default/dragitem12.dds"] != "sheet" {
		t.Fatalf("%v", state)
	}
	if err := s.SetRemove("uifiles/default/EQUI_Sword.xml", false); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRemove("IT1234.eqg", false); err == nil || !strings.Contains(err.Error(), "no copy") {
		t.Errorf("sent a file Spire has no copy of: %v", err)
	}
	// uploading it again sends it
	if _, err := s.ImportAt("", "IT1234.eqg", buildPFS(t, []string{"it1234.mod"})); err != nil {
		t.Fatal(err)
	}
	files, _ = s.PatchFiles()
	for _, f := range files {
		if f.Remove && f.Path != "uifiles/default/dragitem180.dds" {
			t.Errorf("still removed: %+v", f)
		}
	}
}
