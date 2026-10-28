package clientfiles

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/EQEmu/spire/internal/clientassets"
)

// Publishing freezes the client patch for the patcher (eqemupatcher): each
// file is stored once by its MD5, and a manifest lists them. Players' patchers
// read the manifest as filelist_rof.yml and download only the files whose MD5
// differs from theirs. Edits in Spire reach players only when published.

type ManifestFile struct {
	Path   string `json:"path"`
	MD5    string `json:"md5"` // upper-case hex, as the patcher writes its own
	Size   int64  `json:"size"`
	Date   string `json:"date"`
	Source string `json:"source"`
}

type Manifest struct {
	Version   string         `json:"version"`
	Published int64          `json:"published"`
	Files     []ManifestFile `json:"files"`
	Deletes   []string       `json:"deletes"` // deleted from players' clients if found
	Notes     []string       `json:"notes"`
}

type PublishDiff struct {
	Added     []string `json:"added"`
	Changed   []string `json:"changed"`
	Removed   []string `json:"removed"`   // no longer sent (players keep theirs)
	Deleted   []string `json:"deleted"`   // newly deleted from players' clients
	Undeleted []string `json:"undeleted"` // no longer deleted
	Unchanged int      `json:"unchanged"`
}

// a file to publish; read once, when it is stored
type publishSource struct {
	Path   string
	Source string
	read   func() ([]byte, error)
}

type Publisher struct {
	assets  *clientassets.Store
	mu      sync.Mutex
	current *Manifest
	dir     string // tests set it
}

func NewPublisher(assets *clientassets.Store) *Publisher {
	return &Publisher{assets: assets}
}

func (p *Publisher) root() string {
	if p.dir == "" {
		p.dir = filepath.Join(p.assets.Dir(), "published")
	}
	return p.dir
}

func (p *Publisher) blob(md5 string) string {
	return filepath.Join(p.root(), "blobs", md5)
}

// Current is the published manifest, or nil before the first publish
func (p *Publisher) Current() *Manifest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.currentLocked()
}

func (p *Publisher) currentLocked() *Manifest {
	if p.current != nil {
		return p.current
	}
	b, err := os.ReadFile(filepath.Join(p.root(), "current.json"))
	if err != nil {
		return nil
	}
	var m Manifest
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	p.current = &m
	return p.current
}

// Publish stores the files and makes them the published patch. Publishing
// the same files again changes nothing, so patchers see no new patch.
func (p *Publisher) Publish(sources []publishSource, deletes []string, notes []string) (Manifest, PublishDiff, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := os.MkdirAll(filepath.Join(p.root(), "blobs"), 0755); err != nil {
		return Manifest{}, PublishDiff{}, err
	}

	today := time.Now().Format("20060102")
	byPath := map[string]ManifestFile{}
	for _, s := range sources {
		data, err := s.read()
		if err != nil {
			notes = append(notes, fmt.Sprintf("%v left out: %v", s.Path, err))
			continue
		}
		sum := md5.Sum(data)
		hash := strings.ToUpper(hex.EncodeToString(sum[:]))
		if _, err := os.Stat(p.blob(hash)); err != nil {
			if err := writeAtomic(p.blob(hash), data); err != nil {
				return Manifest{}, PublishDiff{}, err
			}
		}
		// the client folder is not case-sensitive: one file per path
		byPath[strings.ToLower(s.Path)] = ManifestFile{Path: s.Path, MD5: hash, Size: int64(len(data)), Date: today, Source: s.Source}
	}

	m := Manifest{Published: time.Now().Unix(), Notes: notes, Deletes: []string{}}
	for _, f := range byPath {
		m.Files = append(m.Files, f)
	}
	sort.Slice(m.Files, func(i, j int) bool { return strings.ToLower(m.Files[i].Path) < strings.ToLower(m.Files[j].Path) })
	// a file sent is not also deleted
	seen := map[string]bool{}
	for _, d := range deletes {
		key := strings.ToLower(d)
		if _, sent := byPath[key]; !sent && !seen[key] {
			seen[key] = true
			m.Deletes = append(m.Deletes, d)
		}
	}
	sort.Slice(m.Deletes, func(i, j int) bool { return strings.ToLower(m.Deletes[i]) < strings.ToLower(m.Deletes[j]) })

	// the version is the content: the same files publish as the same version
	h := md5.New()
	for _, f := range m.Files {
		fmt.Fprintf(h, "%s\t%s\n", strings.ToLower(f.Path), f.MD5)
	}
	for _, d := range m.Deletes {
		fmt.Fprintf(h, "delete\t%s\n", strings.ToLower(d))
	}
	m.Version = hex.EncodeToString(h.Sum(nil))[:12]

	old := p.currentLocked()
	diff := diffManifests(old, &m)
	if old != nil && old.Version == m.Version {
		return *old, diff, nil
	}
	if old != nil {
		// a file whose content did not change keeps the date it was published
		dates := map[string]string{}
		for _, f := range old.Files {
			dates[strings.ToLower(f.Path)+"\t"+f.MD5] = f.Date
		}
		for i, f := range m.Files {
			if d, ok := dates[strings.ToLower(f.Path)+"\t"+f.MD5]; ok {
				m.Files[i].Date = d
			}
		}
	}

	b, _ := json.MarshalIndent(m, "", "  ")
	_ = os.MkdirAll(filepath.Join(p.root(), "history"), 0755)
	if err := writeAtomic(filepath.Join(p.root(), "history", strconv.FormatInt(m.Published, 10)+"-"+m.Version+".json"), b); err != nil {
		return Manifest{}, PublishDiff{}, err
	}
	if err := writeAtomic(filepath.Join(p.root(), "current.json"), b); err != nil {
		return Manifest{}, PublishDiff{}, err
	}
	p.current = &m
	p.pruneLocked()
	return m, diff, nil
}

// pruneLocked drops stored files the published patch no longer lists, and
// all but the last 20 manifests
func (p *Publisher) pruneLocked() {
	keep := map[string]bool{}
	for _, f := range p.current.Files {
		keep[f.MD5] = true
	}
	entries, _ := os.ReadDir(filepath.Join(p.root(), "blobs"))
	for _, e := range entries {
		if !keep[e.Name()] {
			_ = os.Remove(filepath.Join(p.root(), "blobs", e.Name()))
		}
	}
	history, _ := filepath.Glob(filepath.Join(p.root(), "history", "*.json"))
	sort.Strings(history)
	for i := 0; i+20 < len(history); i++ {
		_ = os.Remove(history[i])
	}
}

func diffManifests(old, cur *Manifest) PublishDiff {
	d := PublishDiff{Added: []string{}, Changed: []string{}, Removed: []string{}, Deleted: []string{}, Undeleted: []string{}}
	before := map[string]ManifestFile{}
	if old != nil {
		for _, f := range old.Files {
			before[strings.ToLower(f.Path)] = f
		}
	}
	for _, f := range cur.Files {
		key := strings.ToLower(f.Path)
		o, ok := before[key]
		switch {
		case !ok:
			d.Added = append(d.Added, f.Path)
		case o.MD5 != f.MD5:
			d.Changed = append(d.Changed, f.Path)
		default:
			d.Unchanged++
		}
		delete(before, key)
	}
	for _, f := range before {
		d.Removed = append(d.Removed, f.Path)
	}
	sort.Strings(d.Removed)

	was := map[string]bool{}
	if old != nil {
		for _, p := range old.Deletes {
			was[strings.ToLower(p)] = true
		}
	}
	for _, p := range cur.Deletes {
		if !was[strings.ToLower(p)] {
			d.Deleted = append(d.Deleted, p)
		}
		delete(was, strings.ToLower(p))
	}
	if old != nil {
		for _, p := range old.Deletes {
			if was[strings.ToLower(p)] {
				d.Undeleted = append(d.Undeleted, p)
			}
		}
	}
	return d
}

// Open finds a published file by its path in the client folder
func (p *Publisher) Open(path string) (ManifestFile, string, bool) {
	m := p.Current()
	if m == nil {
		return ManifestFile{}, "", false
	}
	for _, f := range m.Files {
		if strings.EqualFold(f.Path, path) {
			return f, p.blob(f.MD5), true
		}
	}
	return ManifestFile{}, "", false
}

// Filelist is the published patch as eqemupatcher's filelist_rof.yml. A file
// left out of a later publish is only deleted from players' clients when it
// is marked Remove: it may have replaced one the client ships with.
func Filelist(m *Manifest, downloadPrefix string) []byte {
	var b strings.Builder
	b.WriteString("version: " + yamlString(m.Version) + "\n")
	if len(m.Deletes) == 0 {
		b.WriteString("deletes: []\n")
	} else {
		b.WriteString("deletes:\n")
		for _, d := range m.Deletes {
			b.WriteString("- name: " + yamlString(d) + "\n")
		}
	}
	b.WriteString("downloadprefix: " + yamlString(downloadPrefix) + "\n")
	b.WriteString("downloads:\n")
	for _, f := range m.Files {
		b.WriteString("- name: " + yamlString(f.Path) + "\n")
		b.WriteString("  md5: " + yamlString(f.MD5) + "\n")
		b.WriteString("  date: " + yamlString(f.Date) + "\n")
		b.WriteString("  size: " + strconv.FormatInt(f.Size, 10) + "\n")
	}
	return []byte(b.String())
}

// yamlString is a double-quoted YAML scalar
func yamlString(s string) string {
	b, _ := json.Marshal(s) // JSON strings are valid YAML double-quoted scalars
	return string(b)
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
