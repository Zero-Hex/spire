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
		if err := p.SaveSettings(bad); err == nil {
			t.Errorf("accepted %+v", bad)
		}
	}
	if err := p.SaveSettings(PatcherSettings{FileName: "starfall.exe", ServerName: "Starfall", PublicURL: "http://15.204.93.56:3000/api/v1/patch", SourceDir: src}); err != nil {
		t.Fatal(err)
	}
	s := p.Status().Settings
	if s.FileName != "starfall" || s.PublicURL != "http://15.204.93.56:3000/api/v1/patch/" {
		t.Errorf("%+v", s)
	}
}

func TestPatcherIsBuiltStagedAndReleased(t *testing.T) {
	pub := &Publisher{dir: t.TempDir()}
	p := NewPatcher(pub, nil)
	if err := p.Build(); err == nil {
		t.Fatal("built with no settings")
	}
	_ = p.SaveSettings(PatcherSettings{FileName: "starfall", ServerName: "Starfall", PublicURL: "http://h:3000/api/v1/patch/", SourceDir: fakeSource(t)})

	if err := p.Build(); err != nil {
		t.Fatal(err)
	}
	s := waitBuilt(t, p)
	if s.LastError != "" || s.Staged == nil || s.Staged.Version != "1.1.1" || !strings.Contains(s.Log, "building 1.1.1 for http://h:3000/api/v1/patch/ as starfall (Starfall)") {
		t.Fatalf("%+v", s)
	}
	exe, _ := os.ReadFile(p.stagedExe())
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
	if _, err := p.Release(); err == nil || !strings.Contains(err.Error(), "publish the client patch first") {
		t.Fatalf("released with nothing published: %v", err)
	}
	if _, _, err := pub.Publish([]publishSource{src("spells_us.txt", "x")}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Release(); err != nil {
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
	_ = p.Build()
	s = waitBuilt(t, p)
	if s.Staged.Version != "1.1.2" || s.Released.Version != "1.1.1" {
		t.Errorf("%+v %+v", s.Staged, s.Released)
	}
}

func TestPatcherBuildFailureIsReported(t *testing.T) {
	p := NewPatcher(&Publisher{dir: t.TempDir()}, nil)
	src := fakeSource(t)
	_ = os.WriteFile(filepath.Join(src, "build.sh"), []byte("#!/bin/sh\necho 'error CS1002: ; expected'\nexit 1\n"), 0755)
	_ = p.SaveSettings(PatcherSettings{FileName: "starfall", ServerName: "S", PublicURL: "http://h/api/v1/patch/", SourceDir: src})
	_ = p.Build()
	s := waitBuilt(t, p)
	if s.LastError == "" || s.Staged != nil || !strings.Contains(s.Log, "CS1002") {
		t.Fatalf("%+v", s)
	}
	if err := p.Upload([]byte("not a program")); err == nil {
		t.Error("a non-program was staged")
	}
	if err := p.Upload([]byte("MZ built on Windows")); err != nil || p.Status().Staged.Source != "uploaded" {
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
