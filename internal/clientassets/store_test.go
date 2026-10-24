package clientassets

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EQEmu/spire/internal/logger"
)

func testStore(t *testing.T) *Store {
	s := NewStore(logger.ProvideAppLogger(), nil)
	s.dir = t.TempDir()
	for _, d := range []string{"drop", "icons", "models", "archives"} {
		_ = os.MkdirAll(filepath.Join(s.dir, d), 0755)
	}
	return s
}

// a 256x256 sheet whose cell at column x, row y is coloured (x*40, y*40, 0)
func testSheet(t *testing.T) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x / 40 * 40), G: uint8(y / 40 * 40), A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func iconColor(t *testing.T, s *Store, id int) color.NRGBA {
	f, err := os.Open(filepath.Join(s.dir, "icons", "item_"+itoa(id)+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return color.NRGBAModel.Convert(img.At(20, 20)).(color.NRGBA)
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func TestImportSheetIsColumnFirst(t *testing.T) {
	s := testStore(t)

	msg, err := s.Import("dragitem179.png", testSheet(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "icons 6908-6943") {
		t.Fatalf("unexpected result %q", msg)
	}

	// cell 1 is column 0 row 1; cell 6 is column 1 row 0
	if c := iconColor(t, s, 6909); c.R != 0 || c.G != 40 {
		t.Errorf("icon 6909 should be column 0 row 1, got %+v", c)
	}
	if c := iconColor(t, s, 6914); c.R != 40 || c.G != 0 {
		t.Errorf("icon 6914 should be column 1 row 0, got %+v", c)
	}

	css, _ := os.ReadFile(filepath.Join(s.dir, "client-assets.css"))
	for _, want := range []string{".item-6908 {", ".item-6943-sm {", "icons/item_6908.png"} {
		if !strings.Contains(string(css), want) {
			t.Errorf("css has no %q", want)
		}
	}
}

func TestImportSheetRejectsOtherSizes(t *testing.T) {
	s := testStore(t)
	img := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	if _, err := s.Import("dragitem179.png", b.Bytes()); err == nil {
		t.Fatal("a 512x512 sheet was accepted")
	}
	if _, err := s.Import("icons.png", b.Bytes()); err == nil {
		t.Fatal("a file not named dragitem<N> was accepted")
	}
}

func TestItemIconsMapIsRof2PlusImported(t *testing.T) {
	s := testStore(t)
	if _, err := s.Import("dragitem179.png", testSheet(t)); err != nil {
		t.Fatal(err)
	}

	stock := `[{"type":"directory","name":"./assets/item_icons","contents":[
		{"name":"./assets/item_icons/item_500.png"},
		{"name":"./assets/item_icons/item_6907.png"},
		{"name":"./assets/item_icons/item_6908.png"},
		{"name":"./assets/item_icons/item_10760.png"}]}]`

	var dirs []staticMapDir
	if err := json.Unmarshal(s.ItemIconsMap([]byte(stock)), &dirs); err != nil {
		t.Fatal(err)
	}
	have := map[string]int{}
	for _, f := range dirs[0].Contents {
		have[f.Name]++
	}
	for _, want := range []string{"item_500", "item_6907", "item_6908", "item_6943"} {
		if have["./assets/item_icons/"+want+".png"] != 1 {
			t.Errorf("%v should be listed once, got %v", want, have["./assets/item_icons/"+want+".png"])
		}
	}
	if have["./assets/item_icons/item_10760.png"] != 0 {
		t.Error("icon 10760 is not in RoF2 and should be dropped")
	}
}

// buildPFS writes a minimal PFS archive holding the named files
func buildPFS(t *testing.T, names []string) []byte {
	pack := func(data []byte) []byte {
		var z bytes.Buffer
		w := zlib.NewWriter(&z)
		_, _ = w.Write(data)
		_ = w.Close()
		out := make([]byte, 8)
		binary.LittleEndian.PutUint32(out[0:], uint32(z.Len()))
		binary.LittleEndian.PutUint32(out[4:], uint32(len(data)))
		return append(out, z.Bytes()...)
	}

	var body bytes.Buffer
	body.Write(make([]byte, 12))
	type entry struct{ crc, off, size uint32 }
	var entries []entry
	for i, n := range names {
		data := []byte("file " + n)
		entries = append(entries, entry{uint32(i + 1), uint32(body.Len()), uint32(len(data))})
		body.Write(pack(data))
	}

	var table bytes.Buffer
	_ = binary.Write(&table, binary.LittleEndian, uint32(len(names)))
	for _, n := range names {
		_ = binary.Write(&table, binary.LittleEndian, uint32(len(n)+1))
		table.WriteString(n)
		table.WriteByte(0)
	}
	entries = append(entries, entry{0x61580AC9, uint32(body.Len()), uint32(table.Len())})
	body.Write(pack(table.Bytes()))

	dirOff := body.Len()
	_ = binary.Write(&body, binary.LittleEndian, uint32(len(entries)))
	for _, e := range entries {
		_ = binary.Write(&body, binary.LittleEndian, e)
	}

	out := body.Bytes()
	binary.LittleEndian.PutUint32(out[0:], uint32(dirOff))
	copy(out[4:8], "PFS ")
	binary.LittleEndian.PutUint32(out[8:], 0x20000)
	return out
}

func TestImportArchiveListsModels(t *testing.T) {
	s := testStore(t)
	eqg := buildPFS(t, []string{"it62141.mod", "it62142.mod", "blade.dds", "other.mds"})

	msg, err := s.Import("custom_blades.eqg", eqg)
	if err != nil {
		t.Fatal(err)
	}
	if msg != "custom_blades.eqg: IT62141, IT62142" {
		t.Fatalf("unexpected result %q", msg)
	}

	// listed in the picker only once it has a preview
	var dirs []staticMapDir
	stock := `[{"type":"directory","name":"./assets/objects","contents":[{"name":"./assets/objects/CTN_1.png"}]}]`
	_ = json.Unmarshal(s.ObjectsMap([]byte(stock)), &dirs)
	if len(dirs[0].Contents) != 1 {
		t.Fatalf("models without previews were listed: %+v", dirs[0].Contents)
	}

	var b bytes.Buffer
	_ = png.Encode(&b, image.NewNRGBA(image.Rect(0, 0, 125, 125)))
	if err := s.SavePreview(62141, b.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := s.SavePreview(99999, b.Bytes()); err == nil {
		t.Error("a preview for a model in no archive was accepted")
	}
	_ = json.Unmarshal(s.ObjectsMap([]byte(stock)), &dirs)
	if len(dirs[0].Contents) != 2 || dirs[0].Contents[1].Name != "./assets/objects/CTN_62141.png" {
		t.Fatalf("model with a preview not listed: %+v", dirs[0].Contents)
	}

	// removing the archive removes its models
	if err := s.RemoveArchive("custom_blades.eqg"); err != nil {
		t.Fatal(err)
	}
	if len(s.Index().Models) != 0 {
		t.Error("models remain after the archive was removed")
	}
	if _, err := s.Import("broken.eqg", []byte("not an archive")); err == nil {
		t.Error("a broken archive was accepted")
	}
}

func TestDropFolderImportsAndCleansUp(t *testing.T) {
	s := testStore(t)
	_ = os.WriteFile(filepath.Join(s.dir, "drop", "dragitem180.png"), testSheet(t), 0644)
	_ = os.WriteFile(filepath.Join(s.dir, "drop", "notes.txt"), []byte("x"), 0644)

	s.importDropFolder()

	if _, err := os.Stat(filepath.Join(s.dir, "icons", "item_6944.png")); err != nil {
		t.Error("sheet 180 was not imported")
	}
	if _, err := os.Stat(filepath.Join(s.dir, "drop", "dragitem180.png")); !os.IsNotExist(err) {
		t.Error("imported file was left in the drop folder")
	}
	if _, err := os.Stat(filepath.Join(s.dir, "drop", "notes.txt.failed")); err != nil {
		t.Error("an unimportable file was not set aside as .failed")
	}
}
