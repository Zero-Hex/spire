package clientfiles

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/EQEmu/spire/internal/pathmgmt"
)

// The patcher program (eqemupatcher) is built here from its source on this
// server (the starfall-patcher repository's build.sh, with the .NET SDK),
// and served to players' patchers, which update themselves to it:
// <patch address><file name>-hash.txt and <patch address><file name>.exe.
//
// A build is staged first, so it can be tried before players get it; Release
// makes it the one players' patchers update to.

type PatcherSettings struct {
	SourceDir  string `json:"source_dir"`  // the starfall-patcher checkout, with build.sh
	PublicURL  string `json:"public_url"`  // Spire's patch address as players reach it
	ServerName string `json:"server_name"` // the patcher's window title
	FileName   string `json:"file_name"`   // players run <file name>.exe
	GitPull    bool   `json:"git_pull"`    // update the checkout before building
}

type PatcherBuild struct {
	Version    string `json:"version"`
	MD5        string `json:"md5"`
	Size       int64  `json:"size"`
	Built      int64  `json:"built"`
	PublicURL  string `json:"public_url"` // baked in: where it reads files and updates from
	ServerName string `json:"server_name"`
	FileName   string `json:"file_name"`
	Source     string `json:"source"` // "built" or "uploaded"
}

type PatcherState struct {
	Settings PatcherSettings `json:"settings"`
	Builds   int             `json:"builds"` // numbers the versions
	Staged   *PatcherBuild   `json:"staged"`
	Released *PatcherBuild   `json:"released"`
}

type PatcherStatus struct {
	PatcherState
	Running   bool   `json:"running"`
	Log       string `json:"log"`
	LastError string `json:"last_error"`
}

type Patcher struct {
	publisher *Publisher
	pathmgmt  *pathmgmt.PathManagement
	mu        sync.Mutex
	running   bool
	log       bytes.Buffer
	lastError string
}

func NewPatcher(publisher *Publisher, pathmgmt *pathmgmt.PathManagement) *Patcher {
	return &Patcher{publisher: publisher, pathmgmt: pathmgmt}
}

// DotnetInstallDir is where the page suggests installing the .NET SDK: in the
// server folder, which in akk-stack is kept outside the container, unlike the
// rest of the container's home folder
func (p *Patcher) DotnetInstallDir() string {
	if p.pathmgmt == nil {
		return ""
	}
	if dir := p.pathmgmt.GetEQEmuServerPath(); dir != "" {
		return filepath.Join(dir, ".dotnet")
	}
	return ""
}

// Dotnet is the dotnet builds use, or ""
func (p *Patcher) Dotnet() string {
	if dir := p.DotnetInstallDir(); dir != "" {
		return FindDotnet(filepath.Join(dir, "dotnet"))
	}
	return FindDotnet()
}

func (p *Patcher) dir() string {
	return filepath.Join(p.publisher.root(), "patcher")
}

func (p *Patcher) stagedExe() string   { return filepath.Join(p.dir(), "staged.exe") }
func (p *Patcher) releasedExe() string { return filepath.Join(p.dir(), "released.exe") }

func (p *Patcher) loadLocked() PatcherState {
	var s PatcherState
	if b, err := os.ReadFile(filepath.Join(p.dir(), "state.json")); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	if s.Settings.FileName == "" {
		s.Settings.FileName = "eqemupatcher"
	}
	if s.Settings.ServerName == "" {
		s.Settings.ServerName = "EQEmu Patcher"
	}
	return s
}

func (p *Patcher) saveLocked(s PatcherState) error {
	if err := os.MkdirAll(p.dir(), 0755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return writeAtomic(filepath.Join(p.dir(), "state.json"), b)
}

func (p *Patcher) Status() PatcherStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return PatcherStatus{PatcherState: p.loadLocked(), Running: p.running, Log: p.log.String(), LastError: p.lastError}
}

// Released is the patcher players update to, or nil
func (p *Patcher) Released() (*PatcherBuild, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.loadLocked().Released, p.releasedExe()
}

var (
	patcherFileName  = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	publicURLPattern = regexp.MustCompile(`^https?://[^\s/]+(/[^\s]*)?/$`)
)

func (p *Patcher) SaveSettings(s PatcherSettings) error {
	s.FileName = strings.TrimSuffix(strings.TrimSpace(s.FileName), ".exe")
	s.PublicURL = strings.TrimSpace(s.PublicURL)
	s.ServerName = strings.TrimSpace(s.ServerName)
	s.SourceDir = strings.TrimSpace(s.SourceDir)
	if !patcherFileName.MatchString(s.FileName) {
		return errors.New("the file name may hold letters, digits, _ . and - only")
	}
	if s.PublicURL != "" && !strings.HasSuffix(s.PublicURL, "/") {
		s.PublicURL += "/"
	}
	if s.PublicURL != "" && !publicURLPattern.MatchString(s.PublicURL) {
		return errors.New("the public address must be like http://host:3000/api/v1/patch/")
	}
	if s.ServerName == "" || strings.ContainsAny(s.ServerName, "\"\r\n") {
		return errors.New("the server name must be one line, with no quotes")
	}
	if s.SourceDir != "" {
		if !filepath.IsAbs(s.SourceDir) {
			return errors.New("the patcher source folder must be a full path")
		}
		if _, err := os.Stat(filepath.Join(s.SourceDir, "build.sh")); err != nil {
			return fmt.Errorf("%v has no build.sh: it should be a checkout of the patcher repository", s.SourceDir)
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	st := p.loadLocked()
	st.Settings = s
	return p.saveLocked(st)
}

// Build starts a build in the background; Status follows it
func (p *Patcher) Build() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return errors.New("a build is already running")
	}
	st := p.loadLocked()
	set := st.Settings
	switch {
	case set.SourceDir == "":
		return errors.New("set the patcher source folder first")
	case set.PublicURL == "":
		return errors.New("set the public address first")
	}
	if p.Dotnet() == "" {
		return errors.New("the .NET SDK (dotnet) is not installed where Spire runs")
	}

	p.running = true
	p.lastError = ""
	p.log.Reset()
	version := fmt.Sprintf("1.1.%d", st.Builds+1)
	go p.run(set, version)
	return nil
}

func (p *Patcher) logf(format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.appendLocked(fmt.Sprintf(format, args...))
}

// the log keeps its last 64 KB
func (p *Patcher) appendLocked(s string) {
	p.log.WriteString(s)
	if n := p.log.Len(); n > 64<<10 {
		rest := append([]byte(nil), p.log.Bytes()[n-(64<<10):]...)
		p.log.Reset()
		p.log.Write(rest)
	}
}

type logWriter struct{ p *Patcher }

func (w logWriter) Write(b []byte) (int, error) {
	w.p.mu.Lock()
	defer w.p.mu.Unlock()
	w.p.appendLocked(string(b))
	return len(b), nil
}

func (p *Patcher) run(set PatcherSettings, version string) {
	err := p.build(set, version)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.running = false
	if err != nil {
		p.lastError = err.Error()
		p.appendLocked("\nFailed: " + err.Error() + "\n")
	} else {
		p.appendLocked("\nBuilt " + version + ". Try it, then Release it to players.\n")
	}
}

func (p *Patcher) build(set PatcherSettings, version string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	if set.GitPull {
		if _, err := os.Stat(filepath.Join(set.SourceDir, ".git")); err == nil {
			p.logf("$ git pull --ff-only\n")
			cmd := exec.CommandContext(ctx, "git", "-C", set.SourceDir, "pull", "--ff-only")
			cmd.Stdout, cmd.Stderr = logWriter{p}, logWriter{p}
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("git pull: %v", err)
			}
		}
	}

	out, err := os.MkdirTemp("", "spire-patcher-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(out)

	dotnet := p.Dotnet()
	p.logf("$ build.sh (version %v, %v, with %v)\n", version, set.PublicURL, dotnet)
	cmd := exec.CommandContext(ctx, "sh", filepath.Join(set.SourceDir, "build.sh"), out)
	cmd.Dir = set.SourceDir
	cmd.Env = append(os.Environ(),
		// dotnet-install.sh does not put dotnet on the PATH
		"PATH="+filepath.Dir(dotnet)+string(os.PathListSeparator)+os.Getenv("PATH"),
		"DOTNET_ROOT="+filepath.Dir(dotnet),
		// building needs no ICU, which a server container may not have
		"DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=1",
		"VERSION="+version,
		"SERVER_NAME="+set.ServerName,
		"FILE_NAME="+set.FileName,
		"FILELIST_URL="+set.PublicURL,
		"PATCHER_URL="+set.PublicURL,
	)
	if os.Getenv("HOME") == "" {
		// dotnet keeps its package cache under HOME
		cmd.Env = append(cmd.Env, "HOME="+filepath.Join(p.dir(), "home"), "DOTNET_CLI_HOME="+filepath.Join(p.dir(), "home"))
	}
	cmd.Stdout, cmd.Stderr = logWriter{p}, logWriter{p}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build.sh: %v", err)
	}

	exe, err := os.ReadFile(filepath.Join(out, "eqemupatcher.exe"))
	if err != nil {
		return errors.New("build.sh made no eqemupatcher.exe")
	}
	return p.stage(exe, PatcherBuild{
		Version: version, PublicURL: set.PublicURL, ServerName: set.ServerName, FileName: set.FileName, Source: "built",
	}, true)
}

// Upload stages a patcher built elsewhere. Its own settings are not known:
// it is served under the current file name.
func (p *Patcher) Upload(exe []byte) error {
	if len(exe) < 2 || string(exe[:2]) != "MZ" {
		return errors.New("that is not a Windows program")
	}
	p.mu.Lock()
	set := p.loadLocked().Settings
	p.mu.Unlock()
	return p.stage(exe, PatcherBuild{Version: "uploaded", PublicURL: set.PublicURL, ServerName: set.ServerName, FileName: set.FileName, Source: "uploaded"}, false)
}

func (p *Patcher) stage(exe []byte, b PatcherBuild, numbered bool) error {
	sum := md5.Sum(exe)
	b.MD5 = strings.ToUpper(hex.EncodeToString(sum[:]))
	b.Size = int64(len(exe))
	b.Built = time.Now().Unix()

	p.mu.Lock()
	defer p.mu.Unlock()
	if err := os.MkdirAll(p.dir(), 0755); err != nil {
		return err
	}
	if err := writeAtomic(p.stagedExe(), exe); err != nil {
		return err
	}
	st := p.loadLocked()
	if numbered {
		st.Builds++
	}
	st.Staged = &b
	return p.saveLocked(st)
}

// Release makes the staged patcher the one players' patchers update to
func (p *Patcher) Release() (*PatcherBuild, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	st := p.loadLocked()
	if st.Staged == nil {
		return nil, errors.New("nothing is staged: build or upload a patcher first")
	}
	exe, err := os.ReadFile(p.stagedExe())
	if err != nil {
		return nil, err
	}
	if err := writeAtomic(p.releasedExe(), exe); err != nil {
		return nil, err
	}
	st.Released = st.Staged
	if err := p.saveLocked(st); err != nil {
		return nil, err
	}
	return st.Released, nil
}

// FindDotnet is the .NET SDK's dotnet: on the PATH, in DOTNET_ROOT, in one
// of the given places, or where Microsoft's dotnet-install.sh and packages
// put it. "" when there is none.
func FindDotnet(also ...string) string {
	if p, err := exec.LookPath("dotnet"); err == nil {
		return p
	}
	var places []string
	if root := os.Getenv("DOTNET_ROOT"); root != "" {
		places = append(places, filepath.Join(root, "dotnet"))
	}
	places = append(places, also...)
	if home, err := os.UserHomeDir(); err == nil {
		places = append(places, filepath.Join(home, ".dotnet", "dotnet"))
	}
	places = append(places, "/usr/share/dotnet/dotnet", "/usr/lib/dotnet/dotnet", "/opt/dotnet/dotnet")
	for _, p := range places {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() && fi.Mode()&0111 != 0 {
			return p
		}
	}
	return ""
}
