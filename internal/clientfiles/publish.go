package clientfiles

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
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
//
// There are two channels, each its own published patch: live, which every
// player's patcher reads at the patch address, and beta, for testers, at
// <patch address>beta/. Beta is published from what Spire holds; live is
// promoted from beta, so players get the files testers tried.

const (
	ChannelLive = "live"
	ChannelBeta = "beta"
)

func checkChannel(ch string) error {
	if ch != ChannelLive && ch != ChannelBeta {
		return fmt.Errorf("%q is not a channel: live or beta", ch)
	}
	return nil
}

type ManifestFile struct {
	Path   string `json:"path"`
	MD5    string `json:"md5"` // upper-case hex, as the patcher writes its own
	Size   int64  `json:"size"`
	Date   string `json:"date"`
	Source string `json:"source"`
	Beta   bool   `json:"beta,omitempty"` // beta only: promoting to live leaves it out
	// beta: the file as live gets it, when that differs (EQUI_Animations.xml
	// without beta-only sheets' frames); promoting sends this
	LiveMD5  string `json:"live_md5,omitempty"`
	LiveSize int64  `json:"live_size,omitempty"`
}

type Manifest struct {
	Version   string         `json:"version"`
	Published int64          `json:"published"`
	Files     []ManifestFile `json:"files"`
	Deletes   []string       `json:"deletes"` // deleted from players' clients if found
	Notes     []string       `json:"notes"`
	Promoted  string         `json:"promoted,omitempty"` // live: the beta version it was promoted from
}

type PublishDiff struct {
	Added     []string `json:"added"`
	Changed   []string `json:"changed"`
	Removed   []string `json:"removed"`   // no longer sent (players keep theirs)
	Deleted   []string `json:"deleted"`   // newly deleted from players' clients
	Undeleted []string `json:"undeleted"` // no longer deleted
	BetaOnly  []string `json:"beta_only"` // sent as before, now (or no longer) beta only
	Unchanged int      `json:"unchanged"`
}

// a file to publish; read once, when it is stored
type publishSource struct {
	Path   string
	Source string
	Beta   bool
	read   func() ([]byte, error)
	live   func() ([]byte, error) // the file as live gets it, if it differs
}

type Publisher struct {
	assets  *clientassets.Store
	mu      sync.Mutex
	current map[string]*Manifest
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

// channelDir holds a channel's manifests and patcher; live keeps the folder
// the patch was published to before there were channels. Stored files are
// shared.
func (p *Publisher) channelDir(ch string) string {
	if ch == ChannelBeta {
		return filepath.Join(p.root(), "beta")
	}
	return p.root()
}

func (p *Publisher) blob(md5 string) string {
	return filepath.Join(p.root(), "blobs", md5)
}

// Current is a channel's published manifest, or nil before its first publish
func (p *Publisher) Current(ch string) *Manifest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.currentLocked(ch)
}

func (p *Publisher) currentLocked(ch string) *Manifest {
	if m := p.current[ch]; m != nil {
		return m
	}
	b, err := os.ReadFile(filepath.Join(p.channelDir(ch), "current.json"))
	if err != nil {
		return nil
	}
	var m Manifest
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	if p.current == nil {
		p.current = map[string]*Manifest{}
	}
	p.current[ch] = &m
	return &m
}

// Publish stores the files and makes them the channel's published patch.
// Publishing the same files again changes nothing, so patchers see no new
// patch.
func (p *Publisher) Publish(ch string, sources []publishSource, deletes []string, notes []string) (Manifest, PublishDiff, error) {
	if err := checkChannel(ch); err != nil {
		return Manifest{}, PublishDiff{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := os.MkdirAll(filepath.Join(p.root(), "blobs"), 0755); err != nil {
		return Manifest{}, PublishDiff{}, err
	}

	today := time.Now().Format("20060102")
	var files []ManifestFile
	for _, s := range sources {
		if s.Beta && ch != ChannelBeta {
			continue
		}
		read := s.read
		if ch == ChannelLive && s.live != nil {
			read = s.live
		}
		data, err := read()
		if err != nil {
			notes = append(notes, fmt.Sprintf("%v left out: %v", s.Path, err))
			continue
		}
		hash, err := p.storeLocked(data)
		if err != nil {
			return Manifest{}, PublishDiff{}, err
		}
		f := ManifestFile{Path: s.Path, MD5: hash, Size: int64(len(data)), Date: today, Source: s.Source, Beta: s.Beta}
		if ch == ChannelBeta && s.live != nil {
			live, err := s.live()
			if err != nil {
				notes = append(notes, fmt.Sprintf("%v left out: %v", s.Path, err))
				continue
			}
			if f.LiveMD5, err = p.storeLocked(live); err != nil {
				return Manifest{}, PublishDiff{}, err
			}
			f.LiveSize = int64(len(live))
		}
		files = append(files, f)
	}
	m := newManifest(files, deletes, notes)
	diff, err := p.commitLocked(ch, &m)
	return m, diff, err
}

// storeLocked keeps a file by its MD5, which it returns
func (p *Publisher) storeLocked(data []byte) (string, error) {
	sum := md5.Sum(data)
	hash := strings.ToUpper(hex.EncodeToString(sum[:]))
	if _, err := os.Stat(p.blob(hash)); err != nil {
		if err := writeAtomic(p.blob(hash), data); err != nil {
			return "", err
		}
	}
	return hash, nil
}

// Promote makes beta's published patch live, leaving out its beta-only files
func (p *Publisher) Promote() (Manifest, PublishDiff, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	m, _, err := p.promotedLocked()
	if err != nil {
		return Manifest{}, PublishDiff{}, err
	}
	diff, err := p.commitLocked(ChannelLive, &m)
	return m, diff, err
}

// PromotePreview is what Promote would change on live, and the beta-only
// files it leaves out
func (p *Publisher) PromotePreview() (PublishDiff, []string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	m, left, err := p.promotedLocked()
	if err != nil {
		return PublishDiff{}, nil, err
	}
	return diffManifests(p.currentLocked(ChannelLive), &m), left, nil
}

func (p *Publisher) promotedLocked() (Manifest, []string, error) {
	beta := p.currentLocked(ChannelBeta)
	if beta == nil {
		return Manifest{}, nil, errors.New("nothing is published to beta: publish there first, then promote it")
	}
	var files []ManifestFile
	left := []string{}
	for _, f := range beta.Files {
		if f.Beta {
			left = append(left, f.Path)
			continue
		}
		if f.LiveMD5 != "" {
			f.MD5, f.Size = f.LiveMD5, f.LiveSize
		}
		f.LiveMD5, f.LiveSize = "", 0
		files = append(files, f)
	}
	notes := append([]string{}, beta.Notes...)
	m := newManifest(files, beta.Deletes, notes)
	m.Promoted = beta.Version
	return m, left, nil
}

// newManifest lists the files (one per path: the client folder is not
// case-sensitive) and the deletes, and numbers the version by its content
func newManifest(files []ManifestFile, deletes, notes []string) Manifest {
	byPath := map[string]ManifestFile{}
	for _, f := range files {
		byPath[strings.ToLower(f.Path)] = f
	}
	if notes == nil {
		notes = []string{}
	}
	m := Manifest{Published: time.Now().Unix(), Notes: notes, Deletes: []string{}, Files: []ManifestFile{}}
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
		// promoting reads these, so a change to them is a new version
		switch {
		case f.Beta:
			fmt.Fprintf(h, "%s\t%s\tbeta\n", strings.ToLower(f.Path), f.MD5)
		case f.LiveMD5 != "":
			fmt.Fprintf(h, "%s\t%s\tlive %s\n", strings.ToLower(f.Path), f.MD5, f.LiveMD5)
		default:
			fmt.Fprintf(h, "%s\t%s\n", strings.ToLower(f.Path), f.MD5)
		}
	}
	for _, d := range m.Deletes {
		fmt.Fprintf(h, "delete\t%s\n", strings.ToLower(d))
	}
	m.Version = hex.EncodeToString(h.Sum(nil))[:12]
	return m
}

// commitLocked makes m the channel's published patch, unless it already is
func (p *Publisher) commitLocked(ch string, m *Manifest) (PublishDiff, error) {
	old := p.currentLocked(ch)
	diff := diffManifests(old, m)
	if old != nil && old.Version == m.Version {
		*m = *old
		return diff, nil
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

	dir := p.channelDir(ch)
	b, _ := json.MarshalIndent(m, "", "  ")
	_ = os.MkdirAll(filepath.Join(dir, "history"), 0755)
	if err := writeAtomic(filepath.Join(dir, "history", strconv.FormatInt(m.Published, 10)+"-"+m.Version+".json"), b); err != nil {
		return PublishDiff{}, err
	}
	if err := writeAtomic(filepath.Join(dir, "current.json"), b); err != nil {
		return PublishDiff{}, err
	}
	if p.current == nil {
		p.current = map[string]*Manifest{}
	}
	cur := *m
	p.current[ch] = &cur
	p.pruneLocked(ch)
	return diff, nil
}

// pruneLocked drops stored files neither channel's published patch lists,
// and all but the channel's last 20 manifests
func (p *Publisher) pruneLocked(ch string) {
	keep := map[string]bool{}
	for _, c := range []string{ChannelLive, ChannelBeta} {
		if m := p.currentLocked(c); m != nil {
			for _, f := range m.Files {
				keep[f.MD5] = true
				if f.LiveMD5 != "" {
					keep[f.LiveMD5] = true
				}
			}
		}
	}
	entries, _ := os.ReadDir(filepath.Join(p.root(), "blobs"))
	for _, e := range entries {
		if !keep[e.Name()] {
			_ = os.Remove(filepath.Join(p.root(), "blobs", e.Name()))
		}
	}
	history, _ := filepath.Glob(filepath.Join(p.channelDir(ch), "history", "*.json"))
	sort.Strings(history)
	for i := 0; i+20 < len(history); i++ {
		_ = os.Remove(history[i])
	}
}

func diffManifests(old, cur *Manifest) PublishDiff {
	d := PublishDiff{Added: []string{}, Changed: []string{}, Removed: []string{}, Deleted: []string{}, Undeleted: []string{}, BetaOnly: []string{}}
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
		case o.Beta != f.Beta:
			d.BetaOnly = append(d.BetaOnly, f.Path)
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

// Open finds a channel's published file by its path in the client folder
func (p *Publisher) Open(ch, path string) (ManifestFile, string, bool) {
	m := p.Current(ch)
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
