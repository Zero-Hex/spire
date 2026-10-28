package clientfiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func src(path, data string) publishSource {
	return publishSource{Path: path, Source: "test", read: func() ([]byte, error) { return []byte(data), nil }}
}

func blobs(t *testing.T, p *Publisher) int {
	e, _ := os.ReadDir(filepath.Join(p.root(), "blobs"))
	return len(e)
}

func TestPublishOnlyChangesWhatChanged(t *testing.T) {
	p := &Publisher{dir: t.TempDir()}
	if p.Current() != nil {
		t.Fatal("published before publishing")
	}

	m1, d1, err := p.Publish([]publishSource{
		src("spells_us.txt", "spells v1"),
		src("Resources/GlobalLoad.txt", "global"),
		src("uifiles/default/EQUI.xml", "global"), // same content: stored once
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(m1.Files) != 3 || len(d1.Added) != 3 || blobs(t, p) != 2 {
		t.Fatalf("%+v %+v blobs %d", m1, d1, blobs(t, p))
	}
	if m1.Files[0].Path != "Resources/GlobalLoad.txt" || m1.Files[0].MD5 != strings.ToUpper(m1.Files[0].MD5) {
		t.Errorf("first file %+v", m1.Files[0])
	}

	// the same files again: no new version, so patchers see no new patch
	m2, d2, _ := p.Publish([]publishSource{
		src("spells_us.txt", "spells v1"),
		src("resources/globalload.txt", "global"),
		src("uifiles/default/EQUI.xml", "global"),
	}, nil, nil)
	if m2.Version != m1.Version || m2.Published != m1.Published || d2.Unchanged != 3 {
		t.Fatalf("republish: %v vs %v, %+v", m2.Version, m1.Version, d2)
	}

	m3, d3, _ := p.Publish([]publishSource{
		src("spells_us.txt", "spells v2"),
		src("Resources/GlobalLoad.txt", "global"),
		src("dinput8.dll", "dll"),
	}, nil, nil)
	if m3.Version == m1.Version || strings.Join(d3.Changed, ",") != "spells_us.txt" ||
		strings.Join(d3.Added, ",") != "dinput8.dll" || strings.Join(d3.Removed, ",") != "uifiles/default/EQUI.xml" || d3.Unchanged != 1 {
		t.Fatalf("%+v", d3)
	}
	// spells v1 is no longer published: its stored copy is dropped
	if blobs(t, p) != 3 {
		t.Errorf("blobs %d", blobs(t, p))
	}
	f, blob, ok := p.Open("SPELLS_US.TXT")
	if b, _ := os.ReadFile(blob); !ok || string(b) != "spells v2" || f.Size != 9 {
		t.Fatalf("open: %v %q %+v", ok, b, f)
	}
	if _, _, ok := p.Open("uifiles/default/EQUI.xml"); ok {
		t.Error("a file left out of the publish is still served")
	}

	// a new publisher (Spire restarted) reads the published patch back
	again := &Publisher{dir: p.dir}
	if c := again.Current(); c == nil || c.Version != m3.Version {
		t.Fatal("not read back")
	}
}

func TestFilelistIsWhatThePatcherReads(t *testing.T) {
	m := &Manifest{Version: "abc123", Files: []ManifestFile{
		{Path: "spells_us.txt", MD5: "0CC175B9C0F1B6A831C399E269772661", Size: 12, Date: "20261028"},
		{Path: "uifiles/default/EQUI: \"odd\" #name.xml", MD5: "X", Size: 1, Date: "20261028"},
	}}
	var got struct {
		Version        string `yaml:"version"`
		Deletes        []any  `yaml:"deletes"`
		DownloadPrefix string `yaml:"downloadprefix"`
		Downloads      []struct {
			Name string `yaml:"name"`
			MD5  string `yaml:"md5"`
			Date string `yaml:"date"`
			Size int    `yaml:"size"`
		} `yaml:"downloads"`
	}
	out := Filelist(m, "http://15.204.93.56:3000/api/v1/patch/rof/")
	if err := yaml.Unmarshal(out, &got); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if got.Version != "abc123" || got.DownloadPrefix != "http://15.204.93.56:3000/api/v1/patch/rof/" || len(got.Deletes) != 0 ||
		len(got.Downloads) != 2 || got.Downloads[0].Size != 12 || got.Downloads[1].Name != "uifiles/default/EQUI: \"odd\" #name.xml" {
		t.Fatalf("%+v\n%s", got, out)
	}
}

func TestPublishDeletesFromClients(t *testing.T) {
	p := &Publisher{dir: t.TempDir()}
	m1, _, _ := p.Publish([]publishSource{src("spells_us.txt", "x")}, nil, nil)

	m2, d2, _ := p.Publish([]publishSource{src("spells_us.txt", "x")}, []string{"IT9.eqg", "spells_us.txt", "it9.eqg"}, nil)
	if m2.Version == m1.Version {
		t.Error("a new delete is not a new patch, so patchers would not run it")
	}
	if strings.Join(m2.Deletes, ",") != "IT9.eqg" || strings.Join(d2.Deleted, ",") != "IT9.eqg" {
		t.Errorf("deletes %v, diff %+v (a file sent is not also deleted)", m2.Deletes, d2)
	}

	var got struct {
		Deletes []struct {
			Name string `yaml:"name"`
		} `yaml:"deletes"`
	}
	if err := yaml.Unmarshal(Filelist(&m2, "http://h/rof/"), &got); err != nil || len(got.Deletes) != 1 || got.Deletes[0].Name != "IT9.eqg" {
		t.Fatalf("%+v %v", got, err)
	}

	_, d3, _ := p.Publish([]publishSource{src("spells_us.txt", "x")}, nil, nil)
	if strings.Join(d3.Undeleted, ",") != "IT9.eqg" {
		t.Errorf("%+v", d3)
	}
}
