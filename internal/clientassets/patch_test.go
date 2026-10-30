package clientassets

import (
	"bytes"
	"encoding/xml"
	"fmt"
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

	files, notes := s.PatchFiles(false)
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
	if err := s.RemoveClientFile("RESOURCES/GLOBALLOAD.TXT"); err != nil || len(s.Index().Files) != 3 {
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
	// stock, or a server's own with more frames: the next two are added
	n := strings.Count(dragItemBlock.FindString(string(b)), "<Frames>")
	out, added, warn := AddDragItemFrames(string(b), []int{12, n + 1, n + 2})
	if len(added) != 2 || warn != "" {
		t.Fatalf("%d frames: added %v warn %q", n, added, warn)
	}
	if strings.Count(dragItemBlock.FindString(out), "<Frames>") != n+2 {
		t.Errorf("want %d frames", n+2)
	}
	d := xml.NewDecoder(strings.NewReader(out))
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil } // us-ascii
	if err := d.Decode(new(struct{})); err != nil {
		t.Errorf("not XML any more: %v", err)
	}
	if len(out)-len(b) > 2000 || !strings.Contains(out, fmt.Sprintf("<Texture>dragitem%d.dds</Texture>", n+2)) {
		t.Errorf("grew by %d", len(out)-len(b))
	}
}

func TestPatchWarnsAboutSheetsTheClientCannotShow(t *testing.T) {
	s := testStore(t)
	if _, err := s.Import("dragitem179.png", testSheet(t)); err != nil {
		t.Fatal(err)
	}
	_, notes := s.PatchFiles(false)
	if len(notes) != 1 || !strings.Contains(notes[0], "EQUI_Animations.xml") {
		t.Fatalf("notes %v", notes)
	}
	if _, err := s.Import("EQUI_Animations.xml", []byte(animations)); err != nil {
		t.Fatal(err)
	}
	files, notes := s.PatchFiles(false)
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

func TestClientFilesInAnyFolder(t *testing.T) {
	s := testStore(t)
	for _, f := range []struct{ folder, name string }{
		{"", "dinput8.dll"},
		{"", "eqhost.txt"},
		{"", "EQUI.xml"},
		{"resources", "BaseData_custom.txt"},
		{"uifiles\\default", "starfall_window.tga"},
		{"uifiles/default/", "dragitem7.png"}, // still an icon sheet
	} {
		data := []byte("x")
		if strings.HasPrefix(f.name, "dragitem") {
			data = testSheet(t)
		}
		if _, err := s.ImportAt(f.folder, f.name, data); err != nil {
			t.Fatalf("%v/%v: %v", f.folder, f.name, err)
		}
	}
	files, _ := s.PatchFiles(false)
	var paths []string
	for _, f := range files {
		paths = append(paths, f.Path)
	}
	if got := strings.Join(paths, ","); got != "dinput8.dll,eqhost.txt,resources/BaseData_custom.txt,uifiles/default/dragitem7.dds,uifiles/default/EQUI.xml,uifiles/default/starfall_window.tga" {
		t.Errorf("paths %v", got)
	}

	for _, bad := range []struct{ folder, name, want string }{
		{"", "spells_us.txt", "database"},
		{"resources", "SkillCaps.txt", "database"},
		{"", "eqgame.exe", "programs"},
		{"", "patch.bat", "programs"},
		{"../..", "x.txt", "not a folder"},
		{"uifiles/../..", "x.txt", "not a folder"},
		{"", "notes.txt", "choose the folder"},
	} {
		if _, err := s.ImportAt(bad.folder, bad.name, []byte("x")); err == nil || !strings.Contains(err.Error(), bad.want) {
			t.Errorf("%v/%v: %v", bad.folder, bad.name, err)
		}
	}
	if err := s.RemoveClientFile("../index.json"); err == nil {
		t.Error("a path outside the client folder was accepted")
	}
}

// client files used to be kept by name; they move to their path
func TestOldClientFilesAreMoved(t *testing.T) {
	s := testStore(t)
	_ = os.MkdirAll(filepath.Join(s.dir, "files"), 0755)
	_ = os.WriteFile(filepath.Join(s.dir, "files", "GlobalLoad.txt"), []byte("old"), 0644)
	_ = os.WriteFile(filepath.Join(s.dir, "index.json"), []byte(`{"files":{"globalload.txt":{"file":"GlobalLoad.txt","path":"Resources/GlobalLoad.txt","size":3}}}`), 0644)

	files, _ := s.PatchFiles(false)
	if len(files) != 1 || files[0].Path != "Resources/GlobalLoad.txt" {
		t.Fatalf("%+v", files)
	}
	if b, err := files[0].Read(); err != nil || string(b) != "old" {
		t.Fatalf("%q %v", b, err)
	}
	if _, ok := s.Index().Files["resources/globalload.txt"]; !ok {
		t.Error("not keyed by path")
	}
}

// a beta build: kept and used in Spire, neither sent nor deleted, and still
// kept out when a newer build of it is uploaded
func TestIgnoredFilesStayOutOfThePatch(t *testing.T) {
	s := testStore(t)
	if _, err := s.ImportReaderAt("", "IT9001.eqg", bytes.NewReader(buildPFS(t, []string{"it9001.mod"})), ModeIgnore); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportReaderAt("", "dragitem181.png", bytes.NewReader(testSheet(t)), ModeIgnore); err != nil {
		t.Fatal(err)
	}
	_, _ = s.ImportAt("uifiles/default", "EQUI_Beta.xml", []byte("<x/>"))
	if err := s.SetMode("uifiles/default/EQUI_Beta.xml", ModeIgnore); err != nil {
		t.Fatal(err)
	}
	if s.Index().Models[9001] == nil {
		t.Error("an ignored archive's models are still item models in Spire")
	}

	mode := func() map[string]string {
		files, _ := s.PatchFiles(false)
		out := map[string]string{}
		for _, f := range files {
			switch {
			case f.Ignore:
				out[f.Path] = ModeIgnore
			case f.Remove:
				out[f.Path] = ModeRemove
			default:
				out[f.Path] = ModeAdd
			}
		}
		return out
	}
	want := map[string]string{"IT9001.eqg": "ignore", "uifiles/default/dragitem181.dds": "ignore", "uifiles/default/EQUI_Beta.xml": "ignore"}
	for p, m := range want {
		if mode()[p] != m {
			t.Errorf("%v: %v", p, mode()[p])
		}
	}

	// a newer beta build stays out
	if _, err := s.Import("IT9001.eqg", buildPFS(t, []string{"it9001.mod", "new.dds"})); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportAt("uifiles/default", "EQUI_Beta.xml", []byte("<y/>")); err != nil {
		t.Fatal(err)
	}
	if m := mode(); m["IT9001.eqg"] != "ignore" || m["uifiles/default/EQUI_Beta.xml"] != "ignore" {
		t.Errorf("re-uploads: %v", m)
	}

	// released: sent from now on
	if err := s.SetMode("IT9001.eqg", ModeAdd); err != nil || mode()["IT9001.eqg"] != "add" {
		t.Errorf("add: %v %v", err, mode())
	}
	if err := s.SetMode("IT9001.eqg", "maybe"); err == nil {
		t.Error("a made-up mode was taken")
	}
	// a removal with no copy cannot be ignored or added
	_, _ = s.AddRemoval("old.eqg")
	if err := s.SetMode("old.eqg", ModeIgnore); err == nil {
		t.Error("ignored a removal Spire has no copy of")
	}
}

// beta only: sent to the beta channel, not live; its icon sheet gets its
// A_DragItem frame in beta's EQUI_Animations.xml only, and it stays beta only
// when a newer build is uploaded
func TestBetaFilesGoToBetaOnly(t *testing.T) {
	s := testStore(t)
	if _, err := s.Import("EQUI_Animations.xml", []byte(animations)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ImportReaderAt("", "dragitem3.png", bytes.NewReader(testSheet(t)), ModeBeta); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import("IT9001.eqg", buildPFS(t, []string{"it9001.mod"})); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMode("IT9001.eqg", ModeBeta); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import("IT9001.eqg", buildPFS(t, []string{"it9001.mod", "new.dds"})); err != nil {
		t.Fatal(err)
	}

	for _, beta := range []bool{false, true} {
		files, _ := s.PatchFiles(beta)
		got := map[string]PatchFile{}
		for _, f := range files {
			got[f.Path] = f
		}
		if !got["uifiles/default/dragitem3.dds"].Beta || !got["IT9001.eqg"].Beta {
			t.Errorf("beta %v: not beta only: %+v", beta, got)
		}
		anim := got["uifiles/default/EQUI_Animations.xml"]
		b, _ := anim.Read()
		if has := strings.Contains(string(b), "dragitem3.dds"); has != beta || anim.Beta {
			t.Errorf("beta %v: frame for sheet 3 %v, animations %+v", beta, has, anim)
		}
		// beta's carries live's, for promoting
		live, ok := anim.LiveVariant()
		if ok != beta {
			t.Fatalf("beta %v: live variant %v", beta, ok)
		}
		if ok {
			if b, _ := live(); strings.Contains(string(b), "dragitem3.dds") {
				t.Error("live's variant has the beta-only sheet's frame")
			}
		}
	}
}

// a newer version with no folder chosen goes where the earlier one is kept
func TestUploadGoesWhereTheEarlierCopyIs(t *testing.T) {
	s := testStore(t)
	for _, f := range []struct{ folder, name string }{
		{"uifiles/starfall", "EQUI_Inventory.xml"}, // a skin folder, not the usual uifiles/default
		{"Resources", "custom_data.txt"},           // a file with no usual folder
		{"uifiles/default", "EQUI_Twice.xml"},
		{"uifiles/starfall", "EQUI_Twice.xml"},
	} {
		if _, err := s.ImportAt(f.folder, f.name, []byte("v1")); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetMode("uifiles/starfall/EQUI_Inventory.xml", ModeBeta); err != nil {
		t.Fatal(err)
	}

	msg, err := s.ImportAt("", "equi_inventory.xml", []byte("v2"))
	if err != nil || !strings.Contains(msg, "uifiles/starfall/equi_inventory.xml (where the earlier copy is)") {
		t.Fatalf("%q %v", msg, err)
	}
	f := s.Index().Files["uifiles/starfall/equi_inventory.xml"]
	if f == nil || !f.Beta || s.Index().Files["uifiles/default/equi_inventory.xml"] != nil {
		t.Errorf("%+v", s.Index().Files)
	}
	if b, ok := s.ClientFileData("uifiles/starfall/EQUI_Inventory.xml"); !ok || string(b) != "v2" {
		t.Errorf("%q", b)
	}
	// with no usual folder, the earlier copy's folder is used rather than asking
	if _, err := s.ImportAt("", "custom_data.txt", []byte("v2")); err != nil {
		t.Errorf("custom_data.txt: %v", err)
	}
	// kept in two folders: Spire asks
	if _, err := s.ImportAt("", "EQUI_Twice.xml", []byte("v2")); err == nil || !strings.Contains(err.Error(), "uifiles/default and uifiles/starfall") {
		t.Errorf("twice: %v", err)
	}
	// a folder chosen wins
	if _, err := s.ImportAt("uifiles/default", "EQUI_Inventory.xml", []byte("v3")); err != nil || s.Index().Files["uifiles/default/equi_inventory.xml"] == nil {
		t.Errorf("chosen: %v", err)
	}

	// a zip with the file at its top level suggests the same folder
	z := testZip(t, map[string][]byte{"custom_data.txt": []byte("v3"), "EQUI_Twice.xml": []byte("v3")})
	l, err := s.StageZip("update.zip", bytes.NewReader(z))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range l.Entries {
		switch e.Path {
		case "custom_data.txt":
			if e.Folder != "Resources" || e.NeedsFolder {
				t.Errorf("%+v", e)
			}
		case "EQUI_Twice.xml":
			if !e.NeedsFolder {
				t.Errorf("%+v", e)
			}
		}
	}
}
