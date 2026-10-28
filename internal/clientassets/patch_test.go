package clientassets

import (
	"bytes"
	"encoding/xml"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodeDDSRoundTrips(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 4))
	img.Set(3, 2, color.NRGBA{R: 200, G: 100, B: 50, A: 128})
	back, err := decodeDDS(encodeDDS(img))
	if err != nil {
		t.Fatal(err)
	}
	if got := back.NRGBAAt(3, 2); got != (color.NRGBA{R: 200, G: 100, B: 50, A: 128}) || back.Bounds().Dx() != 8 {
		t.Fatalf("got %v, %v", got, back.Bounds())
	}
}

func TestSheetGoesToThePatchAsUploaded(t *testing.T) {
	s := testStore(t)
	dds := encodeDDS(mustDecodePNG(t, testSheet(t)))
	dds = append(dds, []byte("trailing bytes only the original has")...)
	if _, err := s.Import("dragitem5.dds", dds); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import("dragitem180.png", testSheet(t)); err != nil {
		t.Fatal(err)
	}

	if got, _ := s.SheetDDS(5); !bytes.Equal(got, dds) {
		t.Error("an uploaded .dds is not sent byte for byte")
	}
	png180, err := s.SheetDDS(180)
	if err != nil {
		t.Fatal(err)
	}
	img, err := decodeDDS(png180)
	if err != nil {
		t.Fatal(err)
	}
	// cell column 2, row 3 is coloured (80, 120, 0)
	if c := img.NRGBAAt(2*40+5, 3*40+5); c.R != 80 || c.G != 120 {
		t.Errorf("a .png sheet became %v at column 2, row 3", c)
	}

	// a sheet imported before uploads were kept is rebuilt from its icons
	_ = os.Remove(filepath.Join(s.dir, "sheets", "dragitem180.dds"))
	rebuilt, err := s.SheetDDS(180)
	if err != nil {
		t.Fatal(err)
	}
	if img, _ := decodeDDS(rebuilt); img.NRGBAAt(2*40+5, 3*40+5) != (color.NRGBA{R: 80, G: 120, A: 255}) {
		t.Errorf("rebuilt sheet has %v", img.NRGBAAt(2*40+5, 3*40+5))
	}

	_ = s.RemoveSheet(5)
	if _, err := os.Stat(filepath.Join(s.dir, "sheets", "dragitem5.dds")); !os.IsNotExist(err) {
		t.Error("a removed sheet's upload was kept")
	}
}

func mustDecodePNG(t *testing.T, b []byte) image.Image {
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestClientFilesGoWhereTheClientReadsThem(t *testing.T) {
	s := testStore(t)
	for _, f := range []struct{ name, data string }{
		{"GlobalLoad.txt", "1,1,globalpcfroglok_chr,..."},
		{"qeynos2_chr.txt", "2\nglobalfroglok_chr\n"},
		{"EQUI_Inventory.xml", "<x/>"},
		{"race_dragon.eqg", string(buildPFS(t, []string{"dragon.mod"}))},
	} {
		if _, err := s.Import(f.name, []byte(f.data)); err != nil {
			t.Fatalf("%v: %v", f.name, err)
		}
	}
	if _, err := s.Import("readme.txt", []byte("x")); err == nil {
		t.Error("a file the client patch does not take was kept")
	}
	// the client folder is not case-sensitive: this replaces qeynos2_chr.txt
	if _, err := s.Import("Qeynos2_CHR.txt", []byte("3\n")); err != nil {
		t.Fatal(err)
	}

	files, notes := s.PatchFiles()
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	if got := strings.Join(paths, ","); got != "Qeynos2_CHR.txt,race_dragon.eqg,Resources/GlobalLoad.txt,uifiles/default/EQUI_Inventory.xml" {
		t.Errorf("paths %v", got)
	}
	if len(notes) != 0 {
		t.Errorf("notes %v", notes)
	}
	if b, _ := files[0].Read(); string(b) != "3\n" {
		t.Errorf("chr: %q", b)
	}
	if _, err := os.Stat(filepath.Join(s.dir, "files", "qeynos2_chr.txt")); !os.IsNotExist(err) {
		t.Error("the replaced spelling was left behind")
	}
	if len(s.Index().Archives) != 0 {
		t.Error("an archive without item models was listed as one")
	}
	if err := s.RemoveClientFile("GLOBALLOAD.TXT"); err != nil || len(s.Index().Files) != 3 {
		t.Errorf("remove: %v %v", err, s.Index().Files)
	}
}

const animations = "<XML>\r\n\t<Ui2DAnimation item=\"A_Other\">\r\n\t\t<Frames>\r\n\t\t</Frames>\r\n\t</Ui2DAnimation>\r\n" +
	"\t<Ui2DAnimation item=\"A_DragItem\">\r\n\t\t<Cycle>false</Cycle>\r\n" +
	"\t\t<Frames>\r\n\t\t\t<Texture>dragitem1.dds</Texture>\r\n\t\t</Frames>\r\n" +
	"\t\t<Frames>\r\n\t\t\t<Texture>dragitem2.dds</Texture>\r\n\t\t</Frames>\r\n" +
	"\t</Ui2DAnimation>\r\n</XML>\r\n"

func TestDragItemFramesAreAddedInOrder(t *testing.T) {
	out, added, warn := AddDragItemFrames(animations, []int{1, 3, 4, 6})
	if len(added) != 2 || added[0] != 3 || added[1] != 4 || !strings.Contains(warn, "dragitem5.dds") {
		t.Fatalf("added %v, warn %q", added, warn)
	}
	block := dragItemBlock.FindString(out)
	if strings.Count(block, "<Frames>") != 4 || !strings.Contains(block, "<Texture>dragitem4.dds</Texture>\r\n") ||
		strings.Index(block, "dragitem3") > strings.Index(block, "dragitem4") {
		t.Fatalf("block:\n%v", block)
	}
	if !strings.HasSuffix(block, "\t\t</Frames>\r\n\t</Ui2DAnimation>") || strings.Count(out, "A_Other") != 1 {
		t.Fatalf("out:\n%v", out)
	}
	if _, added, _ := AddDragItemFrames(animations, []int{1, 2}); len(added) != 0 {
		t.Error("frames were added for sheets the file has")
	}
}

// the client's own file, when the client repository is next to this one
func TestDragItemFramesInTheRof2File(t *testing.T) {
	b, err := os.ReadFile("../../../RoF2-Fresh/uifiles/default/EQUI_Animations.xml")
	if err != nil {
		t.Skip("no client")
	}
	out, added, warn := AddDragItemFrames(string(b), []int{12, 179, 180})
	if len(added) != 2 || warn != "" {
		t.Fatalf("added %v warn %q", added, warn)
	}
	if strings.Count(dragItemBlock.FindString(out), "<Frames>") != 180 {
		t.Error("want 180 frames")
	}
	d := xml.NewDecoder(strings.NewReader(out))
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil } // us-ascii
	if err := d.Decode(new(struct{})); err != nil {
		t.Errorf("not XML any more: %v", err)
	}
	if len(out)-len(b) > 2000 || !strings.Contains(out, "<Texture>dragitem180.dds</Texture>") {
		t.Errorf("grew by %d", len(out)-len(b))
	}
}

func TestPatchWarnsAboutSheetsTheClientCannotShow(t *testing.T) {
	s := testStore(t)
	if _, err := s.Import("dragitem179.png", testSheet(t)); err != nil {
		t.Fatal(err)
	}
	_, notes := s.PatchFiles()
	if len(notes) != 1 || !strings.Contains(notes[0], "EQUI_Animations.xml") {
		t.Fatalf("notes %v", notes)
	}
	if _, err := s.Import("EQUI_Animations.xml", []byte(animations)); err != nil {
		t.Fatal(err)
	}
	files, notes := s.PatchFiles()
	var anim PatchFile
	for _, f := range files {
		if strings.HasSuffix(f.Path, "EQUI_Animations.xml") {
			anim = f
		}
	}
	// the test file declares 2 sheets, so 3..178 are missing: nothing is added
	if anim.Path != "uifiles/default/EQUI_Animations.xml" || len(notes) != 1 || !strings.Contains(notes[0], "dragitem3.dds") {
		t.Fatalf("%+v %v", anim, notes)
	}
}
