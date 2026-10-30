package clientassets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// every race file of the stock client, when the client repository is next to
// this one, reads and writes back as it was (but for a final line break)
func TestRaceFilesRoundTrip(t *testing.T) {
	dir := "../../../RoF2-Fresh"
	names, _ := filepath.Glob(filepath.Join(dir, "*_chr.txt"))
	if len(names) == 0 {
		t.Skip("no client")
	}
	names = append(names, filepath.Join(dir, "Resources", "GlobalLoad.txt"), filepath.Join(dir, "Resources", "GlobalLoad_chr.txt"))
	changed := 0
	for _, n := range names {
		b, _ := os.ReadFile(n)
		rel := filepath.Base(n)
		switch {
		case strings.HasSuffix(n, "GlobalLoad.txt"):
			rel = globalLoadPath
		case strings.HasSuffix(n, "GlobalLoad_chr.txt"):
			rel = globalLoadChrPath
		}
		f, err := ParseRaceFile(rel, b)
		if err != nil {
			t.Fatalf("%v: %v", n, err)
		}
		out, err := f.Format()
		if err != nil {
			t.Fatalf("%v: %v", n, err)
		}
		norm := func(s string) string { return strings.TrimRight(s, "\r\n") }
		if norm(string(out)) != norm(string(b)) {
			changed++
			if changed < 4 {
				t.Errorf("%v changed:\n%q\n%q", n, b, out)
			}
		}
	}
	if changed > 0 {
		t.Errorf("%d of %d changed", changed, len(names))
	}
}

func TestRaceFileEdits(t *testing.T) {
	f, err := ParseRaceFile("abysmal_chr.txt", []byte("2\nsdf,sdf_chr\nsdm,sdm_chr\n"))
	if err != nil || len(f.Lines) != 2 || f.Lines[1].Code != "sdm" || f.Lines[1].Archive != "sdm_chr" || f.CRLF {
		t.Fatalf("%+v %v", f, err)
	}
	// a count that disagrees with the lines (as some stock files' do) is kept
	// until the lines change
	odd, _ := ParseRaceFile("hole_chr.txt", []byte("6\ncst,cst\nmud,mud\n"))
	if out, _ := odd.Format(); string(out) != "6\ncst,cst\nmud,mud\n" {
		t.Errorf("unchanged: %q", out)
	}
	odd.Lines = odd.Lines[1:]
	if out, _ := odd.Format(); string(out) != "1\nmud,mud\n" {
		t.Errorf("a line removed: %q", out)
	}

	f.Lines = append(f.Lines[1:], RaceLine{Code: "ele", Archive: "global_chr"}, RaceLine{Code: "RPT", Archive: "rpt"})
	out, _ := f.Format()
	if string(out) != "3\nsdm,sdm_chr\nELE,global_chr\nRPT,rpt\n" {
		t.Errorf("%q", out)
	}
	for _, bad := range []RaceLine{{Code: "HUMAN", Archive: "x"}, {Code: "HUM", Archive: "../x"}, {Code: "HU,", Archive: "x"}} {
		if _, err := (RaceFile{Kind: "chr", Lines: []RaceLine{bad}}).Format(); err == nil {
			t.Errorf("took %+v", bad)
		}
	}

	g, _ := ParseRaceFile("Resources/globalload.txt", []byte("1,1,TFFFE,Equipment-01.eqg,Loading Character Equipment Files\r\n3,0,TFFFC,rap_chr,Loading Characters\r\n"))
	if !g.CRLF || g.Lines[1].Archive != "rap_chr" || g.Kind != "globalload" {
		t.Fatalf("%+v", g)
	}
	g.Lines = append(g.Lines, RaceLine{Archive: "starfall_chr"})
	out, _ = g.Format()
	if !strings.HasSuffix(string(out), "3,0,TFFFC,rap_chr,Loading Characters\r\n3,0,TFFFC,starfall_chr,Loading Characters\r\n") {
		t.Errorf("%q", out)
	}
	for path, want := range map[string]string{"abysmal_chr.txt": "chr", "Resources/GlobalLoad.txt": "globalload", "resources/globalload_chr.txt": "chr", "uifiles/a_chr.txt": "", "eqhost.txt": ""} {
		if RaceFileKind(path) != want {
			t.Errorf("%v", path)
		}
	}
	if !Rof2ChrZones["abysmal"] || len(Rof2ChrZones) < 400 {
		t.Errorf("%d stock zones", len(Rof2ChrZones))
	}
}

func TestRaceFilesAreSavedForThePatch(t *testing.T) {
	s := testStore(t)
	if _, err := s.SaveRaceFile(RaceFile{Path: "Resources/GlobalLoad.txt", Lines: []RaceLine{{Archive: "x_chr"}}}); err == nil {
		t.Error("made a GlobalLoad.txt from nothing")
	}
	if _, err := s.SaveRaceFile(RaceFile{Path: "Resources/GlobalLoad_chr.txt", Lines: []RaceLine{{Code: "TAC", Archive: "tac_chr"}}}); err == nil {
		t.Error("made a GlobalLoad_chr.txt from nothing")
	}
	if _, err := s.SaveRaceFile(RaceFile{Path: "eqhost.txt"}); err == nil {
		t.Error("saved another file")
	}
	if _, err := s.ImportAt("", "Abysmal_chr.txt", []byte("1\r\nsdf,sdf_chr\r\n")); err != nil {
		t.Fatal(err)
	}
	_ = s.SetMode("Abysmal_chr.txt", ModeBeta)
	f, err := s.ReadRaceFile("abysmal_chr.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.Lines = append(f.Lines, RaceLine{Code: "ELE", Archive: "global_chr"})
	if _, err := s.SaveRaceFile(f); err != nil {
		t.Fatal(err)
	}
	b, _ := s.ClientFileData("abysmal_chr.txt")
	kept := s.Index().Files["abysmal_chr.txt"]
	if string(b) != "2\r\nsdf,sdf_chr\r\nELE,global_chr\r\n" || kept.Path != "Abysmal_chr.txt" || !kept.Beta {
		t.Errorf("%q %+v", b, kept)
	}
	// a new zone file starts empty
	if _, err := s.SaveRaceFile(RaceFile{Path: "starfallhub_chr.txt", Lines: []RaceLine{{Code: "HUM", Archive: "globalhum_chr"}}}); err != nil {
		t.Fatal(err)
	}
	if len(s.RaceFiles()) != 2 {
		t.Errorf("%+v", s.RaceFiles())
	}
}
