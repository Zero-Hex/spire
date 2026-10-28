package clientassets

import (
	"bytes"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// the Heroes' Forge set the server owner uploaded, when it is here
const armorSample = "/root/.claude/uploads/0955f81f-fb37-5531-a979-93bd33251add/d778df89-it63400.eqg"

func TestArmorSetIsOneCardNotItemModels(t *testing.T) {
	data, err := os.ReadFile(armorSample)
	if err != nil {
		t.Skip("no armor sample")
	}
	s := testStore(t)
	if _, err := s.Import("it63400.eqg", data); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import("IT9001.eqg", buildPFS(t, []string{"it9001.mod", "sword_c.dds"})); err != nil {
		t.Fatal(err)
	}
	idx := s.Index()
	a := idx.Archives["it63400.eqg"]
	if a.Kind != ArchiveArmor || len(a.Models) != 171 || a.Texture != "hfm253_c.dds" || !a.Swatch {
		t.Fatalf("%+v", a)
	}
	if idx.Models[63400] != nil || idx.Models[9001] == nil || idx.Archives["it9001.eqg"].Kind != ArchiveItems {
		t.Errorf("armor pieces must not be item models; items must be")
	}
	img, err := png.Decode(bytes.NewReader(mustRead(t, filepath.Join(s.dir, "models", "set_it63400.png"))))
	if err != nil || img.Bounds().Dx() != previewSize {
		t.Fatalf("swatch: %v", err)
	}

	// told otherwise, it becomes item models, and back
	if err := s.SetArchiveKind("it63400.eqg", ArchiveItems); err != nil || s.Index().Models[63725] == nil {
		t.Fatalf("to items: %v", err)
	}
	if err := s.SetArchiveKind("IT63400.EQG", ArchiveArmor); err != nil || s.Index().Models[63725] != nil {
		t.Fatalf("to armor: %v", err)
	}
}

// archives imported before sets were told apart are sorted on load
func TestOldArchivesAreClassified(t *testing.T) {
	data, err := os.ReadFile(armorSample)
	if err != nil {
		t.Skip("no armor sample")
	}
	s := testStore(t)
	_ = os.WriteFile(filepath.Join(s.dir, "archives", "it63400.eqg"), data, 0644)
	old := Index{Archives: map[string]*Archive{"it63400.eqg": {File: "it63400.eqg", Models: []int{63400, 63401}}},
		Models: map[int]*ModelInfo{63400: {Id: 63400, Archive: "it63400.eqg"}, 63401: {Id: 63401, Archive: "it63400.eqg", Preview: true}}}
	b, _ := json.Marshal(old)
	_ = os.WriteFile(filepath.Join(s.dir, "index.json"), b, 0644)
	_ = os.WriteFile(filepath.Join(s.dir, "models", "CTN_63401.png"), []byte("blank"), 0644)

	idx := s.Index()
	if idx.Archives["it63400.eqg"].Kind != ArchiveArmor || len(idx.Models) != 0 {
		t.Fatalf("%+v %v", idx.Archives["it63400.eqg"], idx.Models)
	}
	if _, err := os.Stat(filepath.Join(s.dir, "models", "CTN_63401.png")); !os.IsNotExist(err) {
		t.Error("the blank piece preview was kept")
	}
}

func mustRead(t *testing.T, p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
