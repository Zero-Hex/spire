package clientfiles

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

// a source folder whose build.sh writes a fake patcher and shows what it got
func fakeSource(t *testing.T) string {
	dir := t.TempDir()
	script := `#!/bin/sh
set -e
echo "building $VERSION for $FILELIST_URL as $FILE_NAME ($SERVER_NAME)"
printf 'MZ fake %s %s' "$VERSION" "$PATCHER_URL" > "$1/eqemupatcher.exe"
`
	if err := os.WriteFile(filepath.Join(dir, "build.sh"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	// Build looks for the .NET SDK first
	bin := t.TempDir()
	_ = os.WriteFile(filepath.Join(bin, "dotnet"), []byte("#!/bin/sh\n"), 0755)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return dir
}

func waitBuilt(t *testing.T, p *Patcher) PatcherStatus {
	for i := 0; i < 200; i++ {
		if s := p.Status(); !s.Running {
			return s
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the build did not finish")
	return PatcherStatus{}
}

func TestPatcherSettingsAreChecked(t *testing.T) {
	p := NewPatcher(&Publisher{dir: t.TempDir()}, nil)
	src := fakeSource(t)
	for _, bad := range []PatcherSettings{
		{FileName: "../evil", ServerName: "x"},
		{FileName: "ok", ServerName: "x", PublicURL: "ftp://host/"},
		{FileName: "ok", ServerName: "a \"quoted\" name"},
		{FileName: "ok", ServerName: "x", SourceDir: "relative/path"},
		{FileName: "ok", ServerName: "x", SourceDir: t.TempDir()}, // no build.sh
	} {
		if err := p.SaveSettings(ChannelLive, bad); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	if err := p.SaveSettings(ChannelLive, PatcherSettings{FileName: "starfall.exe", ServerName: "Starfall", PublicURL: "http://15.204.93.56:3000/api/v1/patch", SourceDir: src}); err != nil {
		t.Fatal(err)
	}
	s := p.Status().Channels[ChannelLive].Settings
	if s.FileName != "starfall" || s.PublicURL != "http://15.204.93.56:3000/api/v1/patch/" {
		t.Errorf("%+v", s)
	}
}

func TestPatcherIsBuiltStagedAndReleased(t *testing.T) {
	pub := &Publisher{dir: t.TempDir()}
	p := NewPatcher(pub, nil)
	if err := p.Build(ChannelLive); err == nil {
		t.Fatal("built with no settings")
	}
	_ = p.SaveSettings(ChannelLive, PatcherSettings{FileName: "starfall", ServerName: "Starfall", PublicURL: "http://h:3000/api/v1/patch/", SourceDir: fakeSource(t)})

	if err := p.Build(ChannelLive); err != nil {
		t.Fatal(err)
	}
	st := waitBuilt(t, p)
	s := st.Channels[ChannelLive]
	if st.LastError != "" || s.Staged == nil || s.Staged.Version != "1.1.1" || !strings.Contains(st.Log, "building 1.1.1 for http://h:3000/api/v1/patch/ as starfall (Starfall)") {
		t.Fatalf("%+v", st)
	}
	exe, _ := os.ReadFile(p.stagedExe(ChannelLive))
	sum := md5.Sum(exe)
	if s.Staged.MD5 != strings.ToUpper(hex.EncodeToString(sum[:])) {
		t.Error("md5")
	}
	if s.Released != nil {
		t.Fatal("released before Release")
	}

	// players get it only once it is released
	pc := NewPublicController(pub, p)
	get := func(path string) (int, string) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/patch/"+path, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("*")
		c.SetParamValues(path)
		_ = pc.serve(c)
		return rec.Code, rec.Body.String()
	}
	if code, _ := get("starfall-hash.txt"); code != http.StatusNotFound {
		t.Errorf("unreleased hash: %v", code)
	}

	// players' patchers stop without a file list: no release before a publish
	if _, err := p.Release(ChannelLive); err == nil || !strings.Contains(err.Error(), "publish the client patch to live first") {
		t.Fatalf("released with nothing published: %v", err)
	}
	if _, _, err := pub.Publish(ChannelLive, []publishSource{src("spells_us.txt", "x")}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Release(ChannelLive); err != nil {
		t.Fatal(err)
	}
	if code, body := get("starfall-hash.txt"); code != 200 || body != s.Staged.MD5 {
		t.Errorf("hash: %v %q", code, body)
	}
	// the patcher asks for <url>/<name>.exe: a double slash
	if code, body := get("/starfall.exe"); code != 200 || body != string(exe) {
		t.Errorf("exe: %v %q", code, body)
	}
	if code, _ := get("eqemupatcher.exe"); code != http.StatusNotFound {
		t.Errorf("another name is served: %v", code)
	}

	// the next build is numbered on, and does not touch the released one
	_ = p.Build(ChannelLive)
	s = waitBuilt(t, p).Channels[ChannelLive]
	if s.Staged.Version != "1.1.2" || s.Released.Version != "1.1.1" {
		t.Errorf("%+v %+v", s.Staged, s.Released)
	}
}

func TestPatcherBuildFailureIsReported(t *testing.T) {
	p := NewPatcher(&Publisher{dir: t.TempDir()}, nil)
	src := fakeSource(t)
	_ = os.WriteFile(filepath.Join(src, "build.sh"), []byte("#!/bin/sh\necho 'error CS1002: ; expected'\nexit 1\n"), 0755)
	_ = p.SaveSettings(ChannelLive, PatcherSettings{FileName: "starfall", ServerName: "S", PublicURL: "http://h/api/v1/patch/", SourceDir: src})
	_ = p.Build(ChannelLive)
	s := waitBuilt(t, p)
	if s.LastError == "" || s.Channels[ChannelLive].Staged != nil || !strings.Contains(s.Log, "CS1002") {
		t.Fatalf("%+v", s)
	}
	if err := p.Upload(ChannelLive, []byte("not a program")); err == nil {
		t.Error("a non-program was staged")
	}
	if err := p.Upload(ChannelLive, []byte("MZ built on Windows")); err != nil || p.Status().Channels[ChannelLive].Staged.Source != "uploaded" {
		t.Errorf("upload: %v", err)
	}
}

// dotnet-install.sh puts the SDK in ~/.dotnet and not on the PATH
func TestDotnetIsFoundInHome(t *testing.T) {
	home := t.TempDir()
	_ = os.MkdirAll(filepath.Join(home, ".dotnet"), 0755)
	_ = os.WriteFile(filepath.Join(home, ".dotnet", "dotnet"), []byte("#!/bin/sh\n"), 0755)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("DOTNET_ROOT", "")
	t.Setenv("HOME", home)
	if got := FindDotnet(); got != filepath.Join(home, ".dotnet", "dotnet") {
		t.Errorf("found %q", got)
	}
}

// the server folder is checked too: in akk-stack it outlives the container
func TestDotnetIsFoundInTheGivenPlace(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "dotnet"), []byte("#!/bin/sh\n"), 0755)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("DOTNET_ROOT", "")
	t.Setenv("HOME", t.TempDir())
	if got := FindDotnet(filepath.Join(dir, "dotnet")); got != filepath.Join(dir, "dotnet") {
		t.Errorf("found %q", got)
	}
}

func publicGet(pc *PublicController, path string) (int, string) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "http://h:3000/api/v1/patch/"+path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("*")
	c.SetParamValues(path)
	_ = pc.serve(c)
	return rec.Code, rec.Body.String()
}

// the beta patcher reads the beta channel, under its own name, and updates
// itself from there; live's is untouched
func TestBetaPatcherIsItsOwn(t *testing.T) {
	pub := &Publisher{dir: t.TempDir()}
	p := NewPatcher(pub, nil)
	source := fakeSource(t)
	if err := p.SaveSettings(ChannelLive, PatcherSettings{FileName: "starfall", ServerName: "Starfall", PublicURL: "http://h:3000/api/v1/patch/", SourceDir: source}); err != nil {
		t.Fatal(err)
	}
	// beta starts from live's settings
	b := p.Status().Channels[ChannelBeta].Settings
	if b.FileName != "starfall-beta" || b.PublicURL != "http://h:3000/api/v1/patch/beta/" || b.SourceDir != source || b.ServerName != "Starfall Beta" {
		t.Fatalf("beta defaults %+v", b)
	}
	for _, bad := range []PatcherSettings{
		{FileName: "starfall-beta", ServerName: "x", PublicURL: "http://h:3000/api/v1/patch/"}, // live's address
		{FileName: "Starfall", ServerName: "x", PublicURL: "http://h:3000/api/v1/patch/beta/"}, // live's name
	} {
		if err := p.SaveSettings(ChannelBeta, bad); err == nil {
			t.Errorf("beta accepted %+v", bad)
		}
	}
	if err := p.SaveSettings(ChannelLive, PatcherSettings{FileName: "starfall", ServerName: "x", PublicURL: "http://h:3000/api/v1/patch/beta/"}); err == nil {
		t.Error("live took beta's address")
	}
	if err := p.SaveSettings(ChannelBeta, b); err != nil {
		t.Fatal(err)
	}

	if err := p.Build(ChannelBeta); err != nil {
		t.Fatal(err)
	}
	st := waitBuilt(t, p)
	if st.Building != ChannelBeta || st.Channels[ChannelBeta].Staged == nil || st.Channels[ChannelLive].Staged != nil ||
		!strings.Contains(st.Log, "for http://h:3000/api/v1/patch/beta/ as starfall-beta") {
		t.Fatalf("%+v", st)
	}
	// published to live only: beta's patcher would find no file list
	_, _, _ = pub.Publish(ChannelLive, []publishSource{src("spells_us.txt", "live")}, nil, nil)
	if _, err := p.Release(ChannelBeta); err == nil {
		t.Fatal("released the beta patcher with nothing on beta")
	}
	_, _, _ = pub.Publish(ChannelBeta, []publishSource{src("spells_us.txt", "beta")}, nil, nil)
	if _, err := p.Release(ChannelBeta); err != nil {
		t.Fatal(err)
	}

	pc := NewPublicController(pub, p)
	staged := st.Channels[ChannelBeta].Staged
	if code, body := publicGet(pc, "beta/starfall-beta-hash.txt"); code != 200 || body != staged.MD5 {
		t.Errorf("beta hash: %v %q", code, body)
	}
	if code, _ := publicGet(pc, "starfall-beta-hash.txt"); code != 404 {
		t.Errorf("live serves beta's patcher: %v", code)
	}
	if code, body := publicGet(pc, "beta//starfall-beta.exe"); code != 200 || !strings.Contains(body, "/patch/beta/") {
		t.Errorf("beta exe: %v %q", code, body)
	}
	if code, body := publicGet(pc, "beta/rof/filelist_rof.yml"); code != 200 || !strings.Contains(body, `downloadprefix: "http://h:3000/api/v1/patch/beta/rof/"`) {
		t.Errorf("beta filelist: %v %s", code, body)
	}
	if code, body := publicGet(pc, "beta/rof/spells_us.txt"); code != 200 || body != "beta" {
		t.Errorf("beta file: %v %q", code, body)
	}
	if code, body := publicGet(pc, "rof/spells_us.txt"); code != 200 || body != "live" {
		t.Errorf("live file: %v %q", code, body)
	}
	if code, body := publicGet(pc, "rof/filelist_rof.yml"); code != 200 || !strings.Contains(body, `downloadprefix: "http://h:3000/api/v1/patch/rof/"`) {
		t.Errorf("live filelist: %v %s", code, body)
	}
}
