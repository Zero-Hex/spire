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
	if p.Current(ChannelLive) != nil {
		t.Fatal("published before publishing")
	}

	m1, d1, err := p.Publish(ChannelLive, []publishSource{
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
	m2, d2, _ := p.Publish(ChannelLive, []publishSource{
		src("spells_us.txt", "spells v1"),
		src("resources/globalload.txt", "global"),
		src("uifiles/default/EQUI.xml", "global"),
	}, nil, nil)
	if m2.Version != m1.Version || m2.Published != m1.Published || d2.Unchanged != 3 {
		t.Fatalf("republish: %v vs %v, %+v", m2.Version, m1.Version, d2)
	}

	m3, d3, _ := p.Publish(ChannelLive, []publishSource{
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
	f, blob, ok := p.Open(ChannelLive, "SPELLS_US.TXT")
	if b, _ := os.ReadFile(blob); !ok || string(b) != "spells v2" || f.Size != 9 {
		t.Fatalf("open: %v %q %+v", ok, b, f)
	}
	if _, _, ok := p.Open(ChannelLive, "uifiles/default/EQUI.xml"); ok {
		t.Error("a file left out of the publish is still served")
	}

	// a new publisher (Spire restarted) reads the published patch back
	again := &Publisher{dir: p.dir}
	if c := again.Current(ChannelLive); c == nil || c.Version != m3.Version {
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
	m1, _, _ := p.Publish(ChannelLive, []publishSource{src("spells_us.txt", "x")}, nil, nil)

	m2, d2, _ := p.Publish(ChannelLive, []publishSource{src("spells_us.txt", "x")}, []string{"IT9.eqg", "spells_us.txt", "it9.eqg"}, nil)
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

	_, d3, _ := p.Publish(ChannelLive, []publishSource{src("spells_us.txt", "x")}, nil, nil)
	if strings.Join(d3.Undeleted, ",") != "IT9.eqg" {
		t.Errorf("%+v", d3)
	}
}

// beta is published from Spire, beta-only files included; promoting makes
// exactly that live, without them
func TestPromoteMakesBetaLive(t *testing.T) {
	p := &Publisher{dir: t.TempDir()}
	if _, _, err := p.Promote(); err == nil {
		t.Fatal("promoted with nothing on beta")
	}
	live1, _, _ := p.Publish(ChannelLive, []publishSource{src("spells_us.txt", "spells v1")}, nil, nil)

	betaOnly := src("IT9001.eqg", "beta model")
	betaOnly.Beta = true
	sources := []publishSource{src("spells_us.txt", "spells v2"), betaOnly}
	beta, _, err := p.Publish(ChannelBeta, sources, []string{"IT1.eqg"}, nil)
	if err != nil || len(beta.Files) != 2 || !beta.Files[0].Beta {
		t.Fatalf("%+v %v", beta, err)
	}
	// publishing beta leaves live as it was
	if p.Current(ChannelLive).Version != live1.Version {
		t.Fatal("live changed")
	}
	// a beta-only file is never published to live directly
	if m, _, _ := p.Publish(ChannelLive, sources, nil, nil); len(m.Files) != 1 {
		t.Errorf("live got %+v", m.Files)
	}

	d, left, err := p.PromotePreview()
	if err != nil || strings.Join(left, ",") != "IT9001.eqg" || strings.Join(d.Deleted, ",") != "IT1.eqg" || d.Unchanged != 1 {
		t.Fatalf("preview %+v %v %v", d, left, err)
	}
	live, d, err := p.Promote()
	if err != nil || len(live.Files) != 1 || live.Files[0].Path != "spells_us.txt" || live.Promoted != beta.Version ||
		strings.Join(live.Deletes, ",") != "IT1.eqg" {
		t.Fatalf("%+v %+v %v", live, d, err)
	}
	if _, _, ok := p.Open(ChannelLive, "IT9001.eqg"); ok {
		t.Error("live serves a beta-only file")
	}
	if _, blob, ok := p.Open(ChannelBeta, "IT9001.eqg"); !ok {
		t.Error("beta lost its file")
	} else if b, _ := os.ReadFile(blob); string(b) != "beta model" {
		t.Errorf("beta file %q", b)
	}
	// stored files either channel lists are kept: spells v1 is gone, the rest stay
	if blobs(t, p) != 2 {
		t.Errorf("blobs %d", blobs(t, p))
	}
	// promoting again changes nothing
	if again, _, _ := p.Promote(); again.Version != live.Version || again.Published != live.Published {
		t.Error("a second promote made a new version")
	}

	// a file built differently for live (EQUI_Animations.xml) is promoted as
	// live's
	anim := src("uifiles/default/EQUI_Animations.xml", "frames 1-179")
	anim.live = func() ([]byte, error) { return []byte("frames 1-178"), nil }
	if _, _, err := p.Publish(ChannelBeta, []publishSource{anim}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, blob, _ := p.Open(ChannelBeta, "uifiles/default/EQUI_Animations.xml"); true {
		if b, _ := os.ReadFile(blob); string(b) != "frames 1-179" {
			t.Errorf("beta's %q", b)
		}
	}
	if _, _, err := p.Promote(); err != nil {
		t.Fatal(err)
	}
	f, blob, _ := p.Open(ChannelLive, "uifiles/default/EQUI_Animations.xml")
	if b, _ := os.ReadFile(blob); string(b) != "frames 1-178" || f.Size != 12 || f.LiveMD5 != "" {
		t.Errorf("live's %q %+v", b, f)
	}
	// published straight to live, it is live's too
	m, _, _ := p.Publish(ChannelLive, []publishSource{anim}, nil, nil)
	if _, blob, _ := p.Open(ChannelLive, "uifiles/default/EQUI_Animations.xml"); m.Files[0].LiveMD5 != "" {
		t.Error("live has a live variant")
	} else if b, _ := os.ReadFile(blob); string(b) != "frames 1-178" {
		t.Errorf("straight to live: %q", b)
	}

	// making a file beta only, with no other change, is a new beta version, so
	// a promote after it leaves the file out
	plain := src("IT9001.eqg", "beta model")
	b1, _, _ := p.Publish(ChannelBeta, []publishSource{src("spells_us.txt", "spells v2"), plain}, nil, nil)
	b2, d2, _ := p.Publish(ChannelBeta, []publishSource{src("spells_us.txt", "spells v2"), betaOnly}, nil, nil)
	if b2.Version == b1.Version || strings.Join(d2.BetaOnly, ",") != "IT9001.eqg" || d2.Unchanged != 1 {
		t.Errorf("%v %+v", b2.Version, d2)
	}
}
