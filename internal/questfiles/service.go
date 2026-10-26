package questfiles

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/EQEmu/spire/internal/eqemuserver"
	"github.com/EQEmu/spire/internal/pathmgmt"
)

// Service reads and writes quest scripts in the server's quests folder for
// the Quest Scripts page (the Window Forge and a text editor). Saves keep a
// backup of the old file outside the quests folder, so nothing in it is
// loaded as a script, and refuse to overwrite a file changed on disk since
// it was opened.
type Service struct {
	pathmgmt *pathmgmt.PathManagement
	updater  *eqemuserver.Updater

	// overridden by tests
	questsDir  string
	backupsDir string
}

func NewService(pathmgmt *pathmgmt.PathManagement, updater *eqemuserver.Updater) *Service {
	return &Service{pathmgmt: pathmgmt, updater: updater}
}

var (
	ErrConflict = errors.New("the file changed on disk since it was opened")
	ErrNotFound = errors.New("not found")
)

const (
	maxScriptBytes = 2 << 20
	keepBackups    = 20
	forgePath      = "tools/window_forge/window_forge.html"
)

func (s *Service) QuestsDir() string {
	if s.questsDir != "" {
		return s.questsDir
	}
	return s.pathmgmt.GetQuestsDir()
}

// BackupsDir sits next to the quests folder, not in it
func (s *Service) BackupsDir() string {
	if s.backupsDir != "" {
		return s.backupsDir
	}
	return filepath.Join(filepath.Dir(s.QuestsDir()), "spire-quest-backups")
}

// resolve turns a path relative to the quests folder into an absolute one,
// refusing anything that leaves the folder. Scripts must be .pl or .lua.
func (s *Service) resolve(rel string, script bool) (string, error) {
	root, err := filepath.Abs(s.QuestsDir())
	if err != nil {
		return "", err
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == ".." {
			return "", fmt.Errorf("%v: paths may not contain ..", rel)
		}
	}
	clean := filepath.Clean("/" + filepath.FromSlash(rel))
	abs := filepath.Join(root, clean)
	if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", fmt.Errorf("%v is outside the quests folder", rel)
	}
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		if strings.HasPrefix(part, ".") {
			return "", fmt.Errorf("%v: hidden files and folders are not shown", rel)
		}
	}
	if script {
		ext := strings.ToLower(filepath.Ext(abs))
		if ext != ".pl" && ext != ".lua" {
			return "", fmt.Errorf("%v: only .pl and .lua scripts can be edited", rel)
		}
	}
	return abs, nil
}

func relPath(root, abs string) string {
	r, _ := filepath.Rel(root, abs)
	if r == "." {
		return ""
	}
	return filepath.ToSlash(r)
}

type Entry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Modified int64  `json:"modified"`
	// the zone tries a .lua before a .pl of the same name, so a .pl next
	// to a same-named .lua never loads
	SupersededBy string `json:"superseded_by,omitempty"`
	Supersedes   string `json:"supersedes,omitempty"`
}

type Listing struct {
	Dir   string  `json:"dir"`
	Dirs  []Entry `json:"dirs"`
	Files []Entry `json:"files"`
}

// List shows the folders and scripts in one folder of the quests folder
func (s *Service) List(dir string) (Listing, error) {
	abs, err := s.resolve(dir, false)
	if err != nil {
		return Listing{}, err
	}
	root, _ := filepath.Abs(s.QuestsDir())

	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return Listing{}, ErrNotFound
		}
		return Listing{}, err
	}

	out := Listing{Dir: relPath(root, abs), Dirs: []Entry{}, Files: []Entry{}}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		entry := Entry{Name: e.Name(), Path: relPath(root, filepath.Join(abs, e.Name())), Size: info.Size(), Modified: info.ModTime().UnixNano()}
		if e.IsDir() {
			out.Dirs = append(out.Dirs, entry)
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext == ".pl" || ext == ".lua" {
			out.Files = append(out.Files, entry)
		}
	}

	names := map[string]bool{}
	for _, f := range out.Files {
		names[f.Name] = true
	}
	for i, f := range out.Files {
		base := strings.TrimSuffix(f.Name, filepath.Ext(f.Name))
		switch filepath.Ext(f.Name) {
		case ".pl":
			if names[base+".lua"] {
				out.Files[i].SupersededBy = base + ".lua"
			}
		case ".lua":
			if names[base+".pl"] {
				out.Files[i].Supersedes = base + ".pl"
			}
		}
	}
	return out, nil
}

// supersededBy names the same-named .lua that loads instead of a .pl
func supersededBy(abs string) string {
	if filepath.Ext(abs) != ".pl" {
		return ""
	}
	lua := strings.TrimSuffix(abs, ".pl") + ".lua"
	if _, err := os.Stat(lua); err == nil {
		return filepath.Base(lua)
	}
	return ""
}

// Modified is in nanoseconds, more digits than a JavaScript number holds
// exactly, so it travels as a string
type Script struct {
	Path         string `json:"path"`
	Content      string `json:"content"`
	Modified     int64  `json:"modified,string"`
	SupersededBy string `json:"superseded_by,omitempty"`
}

func (s *Service) Read(rel string) (Script, error) {
	abs, err := s.resolve(rel, true)
	if err != nil {
		return Script{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return Script{}, ErrNotFound
		}
		return Script{}, err
	}
	if info.Size() > maxScriptBytes {
		return Script{}, fmt.Errorf("%v is larger than %v MB", rel, maxScriptBytes>>20)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return Script{}, err
	}
	root, _ := filepath.Abs(s.QuestsDir())
	return Script{Path: relPath(root, abs), Content: string(data), Modified: info.ModTime().UnixNano(), SupersededBy: supersededBy(abs)}, nil
}

// Save writes a script. modified is the time the file had when it was
// opened (0 for a new file); a different time on disk means someone else
// changed it, and the save is refused. retire names a script in the same
// folder that is backed up and removed after the save: a Lua script
// rewritten as Perl, since the zone loads .lua ahead of .pl.
func (s *Service) Save(rel, content string, modified int64, retire string) (Script, error) {
	abs, err := s.resolve(rel, true)
	if err != nil {
		return Script{}, err
	}
	if len(content) > maxScriptBytes {
		return Script{}, fmt.Errorf("the script is larger than %v MB", maxScriptBytes>>20)
	}

	var retireAbs string
	if retire != "" {
		retireAbs, err = s.resolve(retire, true)
		if err != nil {
			return Script{}, err
		}
		if filepath.Dir(retireAbs) != filepath.Dir(abs) || retireAbs == abs {
			return Script{}, errors.New("only a script in the same folder can be replaced")
		}
	}

	info, err := os.Stat(abs)
	switch {
	case err == nil:
		if modified == 0 || info.ModTime().UnixNano() != modified {
			return Script{}, ErrConflict
		}
		if err := s.backup(abs); err != nil {
			return Script{}, fmt.Errorf("backing up the old file: %v", err)
		}
	case os.IsNotExist(err):
		if modified != 0 {
			return Script{}, ErrConflict // it was deleted since it was opened
		}
		if _, err := os.Stat(filepath.Dir(abs)); err != nil {
			return Script{}, fmt.Errorf("the folder %v does not exist", filepath.Dir(rel))
		}
	default:
		return Script{}, err
	}

	// write next to it and rename, so a zone never reads half a file
	tmp := filepath.Join(filepath.Dir(abs), "."+filepath.Base(abs)+".spire-tmp")
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return Script{}, err
	}
	if err := os.Rename(tmp, abs); err != nil {
		_ = os.Remove(tmp)
		return Script{}, err
	}

	if retireAbs != "" {
		if _, err := os.Stat(retireAbs); err == nil {
			if err := s.backup(retireAbs); err != nil {
				return Script{}, fmt.Errorf("saved, but backing up %v failed so it was kept: %v", retire, err)
			}
			if err := os.Remove(retireAbs); err != nil {
				return Script{}, fmt.Errorf("saved, but %v could not be removed: %v", retire, err)
			}
		}
	}

	return s.Read(rel)
}

// backup copies a script to <backups>/<path>/<time>.<ext>, keeping the
// newest keepBackups copies
func (s *Service) backup(abs string) error {
	root, _ := filepath.Abs(s.QuestsDir())
	dir := filepath.Join(s.BackupsDir(), filepath.FromSlash(relPath(root, abs)))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	src, err := os.Open(abs)
	if err != nil {
		return err
	}
	defer src.Close()

	name := time.Now().Format("2006-01-02_15-04-05.000000") + filepath.Ext(abs)
	dst, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}

	backups, _ := s.backupNames(abs)
	for i := keepBackups; i < len(backups); i++ {
		_ = os.Remove(filepath.Join(dir, backups[i]))
	}
	return nil
}

// backupNames lists a script's backups, newest first
func (s *Service) backupNames(abs string) ([]string, error) {
	root, _ := filepath.Abs(s.QuestsDir())
	entries, err := os.ReadDir(filepath.Join(s.BackupsDir(), filepath.FromSlash(relPath(root, abs))))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names, nil
}

func (s *Service) Backups(rel string) ([]string, error) {
	abs, err := s.resolve(rel, true)
	if err != nil {
		return nil, err
	}
	return s.backupNames(abs)
}

func (s *Service) ReadBackup(rel, name string) (string, error) {
	abs, err := s.resolve(rel, true)
	if err != nil {
		return "", err
	}
	names, err := s.backupNames(abs)
	if err != nil {
		return "", err
	}
	for _, n := range names {
		if n == name {
			root, _ := filepath.Abs(s.QuestsDir())
			data, err := os.ReadFile(filepath.Join(s.BackupsDir(), filepath.FromSlash(relPath(root, abs)), n))
			return string(data), err
		}
	}
	return "", ErrNotFound
}

// ForgeHTML is the Window Forge from the server's own source checkout, so
// it is always the version that matches the server
func (s *Service) ForgeHTML() ([]byte, error) {
	root := s.updater.GetSourceRoot()
	if root == "" {
		return nil, errors.New("Spire cannot find the server source, so it cannot load " + forgePath)
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(forgePath)))
	if err != nil {
		return nil, fmt.Errorf("%v not found in the server source (%v)", forgePath, root)
	}
	return data, nil
}
