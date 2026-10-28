package clientassets

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A zip (from the weapon toolkit, say) is staged first: Spire lists what is in
// it and where each file would go, and imports only the files picked.

// ZipEntry is one file of a staged zip
type ZipEntry struct {
	Entry       string `json:"entry"` // its name in the zip
	Path        string `json:"path"`  // shown: without a folder every file sits in
	Size        int64  `json:"size"`
	Kind        string `json:"kind"`   // sheet, models, file, refused
	Folder      string `json:"folder"` // where it goes in the client folder; "" next to eqgame.exe
	Note        string `json:"note"`
	NeedsFolder bool   `json:"needs_folder"` // no usual place: choose one
}

type ZipListing struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Entries []ZipEntry `json:"entries"`
}

// ZipPick is a file of a staged zip, the folder it goes in, and whether it
// is added to the patch or deleted from players' clients if found
type ZipPick struct {
	Entry  string `json:"entry"`
	Folder string `json:"folder"`
	Remove bool   `json:"remove"`
}

type ZipResult struct {
	File   string `json:"file"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

const (
	maxZipBytes = 1 << 30
	keepStaged  = 24 * time.Hour
)

var stagedID = regexp.MustCompile(`^[0-9a-f]{16}$`)

func (s *Store) stagingDir() string {
	return filepath.Join(s.Dir(), "staging")
}

// StageZip keeps an uploaded zip and lists what is in it
func (s *Store) StageZip(name string, r io.Reader) (ZipListing, error) {
	dir := s.stagingDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ZipListing{}, err
	}
	// zips staged a day ago are not coming back
	if old, _ := os.ReadDir(dir); old != nil {
		for _, e := range old {
			if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > keepStaged {
				_ = os.Remove(filepath.Join(dir, e.Name()))
			}
		}
	}

	b := make([]byte, 8)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	file := filepath.Join(dir, id+".zip")
	f, err := os.Create(file)
	if err != nil {
		return ZipListing{}, err
	}
	n, err := io.Copy(f, io.LimitReader(r, maxZipBytes+1))
	_ = f.Close()
	if err == nil && n > maxZipBytes {
		err = fmt.Errorf("%v is larger than %v MB", name, maxZipBytes>>20)
	}
	if err != nil {
		_ = os.Remove(file)
		return ZipListing{}, err
	}

	l, err := s.listZip(id)
	if err != nil {
		_ = os.Remove(file)
		return ZipListing{}, fmt.Errorf("%v: %v", name, err)
	}
	l.Name = name
	return l, nil
}

func (s *Store) openStaged(id string) (*zip.ReadCloser, error) {
	if !stagedID.MatchString(id) {
		return nil, errors.New("no such upload")
	}
	z, err := zip.OpenReader(filepath.Join(s.stagingDir(), id+".zip"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("that upload is gone: upload the zip again")
		}
		return nil, errors.New("not a zip file")
	}
	return z, nil
}

func (s *Store) listZip(id string) (ZipListing, error) {
	z, err := s.openStaged(id)
	if err != nil {
		return ZipListing{}, err
	}
	defer z.Close()

	var files []*zip.File
	for _, f := range z.File {
		name := strings.Trim(strings.ReplaceAll(f.Name, "\\", "/"), "/")
		base := path.Base(name)
		if f.FileInfo().IsDir() || strings.HasPrefix(name, "__MACOSX/") || strings.HasPrefix(base, ".") {
			continue
		}
		files = append(files, f)
	}
	if len(files) == 0 {
		return ZipListing{}, errors.New("there are no files in it")
	}

	// a zip of a folder: show paths inside that folder
	prefix := ""
	if first := strings.SplitN(strings.Trim(strings.ReplaceAll(files[0].Name, "\\", "/"), "/"), "/", 2); len(first) == 2 {
		prefix = first[0] + "/"
		for _, f := range files {
			if !strings.HasPrefix(strings.Trim(strings.ReplaceAll(f.Name, "\\", "/"), "/"), prefix) {
				prefix = ""
				break
			}
		}
	}

	l := ZipListing{ID: id, Entries: []ZipEntry{}}
	for _, f := range files {
		shown := strings.TrimPrefix(strings.Trim(strings.ReplaceAll(f.Name, "\\", "/"), "/"), prefix)
		l.Entries = append(l.Entries, s.describe(f, shown))
	}
	sort.Slice(l.Entries, func(i, j int) bool {
		if l.Entries[i].Kind != l.Entries[j].Kind {
			return kindOrder(l.Entries[i].Kind) < kindOrder(l.Entries[j].Kind)
		}
		return strings.ToLower(l.Entries[i].Path) < strings.ToLower(l.Entries[j].Path)
	})
	return l, nil
}

func kindOrder(kind string) int {
	return map[string]int{"models": 0, "sheet": 1, "file": 2, "refused": 3}[kind]
}

// describe says what a file in a zip is and where it would go
func (s *Store) describe(f *zip.File, shown string) ZipEntry {
	base := path.Base(shown)
	dir := path.Dir(shown)
	if dir == "." {
		dir = ""
	}
	lower := strings.ToLower(base)
	e := ZipEntry{Entry: f.Name, Path: shown, Size: int64(f.UncompressedSize64)}
	refuse := func(why string) ZipEntry {
		e.Kind, e.Note = "refused", why
		return e
	}

	if table, ok := generatedFiles[lower]; ok {
		return refuse("built from the database (" + table + ") when the patch is")
	}
	if refusedExtensions[strings.ToLower(path.Ext(lower))] {
		return refuse("programs and scripts are not sent to players")
	}
	if f.UncompressedSize64 > maxUploadBytes {
		return refuse(fmt.Sprintf("larger than %v MB", maxUploadBytes>>20))
	}

	if m := sheetName.FindStringSubmatch(base); m != nil {
		n, _ := strconv.Atoi(m[1])
		first := firstIcon + iconsPerSheet*(n-1)
		e.Kind, e.Folder = "sheet", "uifiles/default"
		e.Note = fmt.Sprintf("icon sheet %d: icons %d-%d", n, first, first+iconsPerSheet-1)
		if n <= Rof2DragItemSheets {
			e.Note += ", replacing a stock sheet"
		}
		return e
	}

	if strings.HasSuffix(lower, ".eqg") && dir == "" {
		if models := zipModels(f); len(models) > 0 {
			e.Kind, e.Note = "models", "item models "+strings.Join(models, ", ")
			return e
		}
	}

	e.Kind = "file"
	folder, err := cleanClientFolder(dir)
	switch {
	case dir != "" && err == nil:
		e.Folder = folder
	default:
		if f, ok := defaultFolder(base); ok {
			e.Folder = f
		} else {
			e.NeedsFolder = true
			e.Note = "choose the folder it goes in"
		}
	}
	if strings.HasSuffix(lower, ".eqg") && e.Note == "" {
		e.Note = "no item models in it: kept as it is"
	}
	return e
}

func zipModels(f *zip.File) []string {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, maxUploadBytes))
	if err != nil {
		return nil
	}
	names, err := pfsFileNames(data)
	if err != nil {
		return nil
	}
	var out []string
	for _, n := range names {
		if m := modelName.FindStringSubmatch(n); m != nil {
			out = append(out, "IT"+m[1])
		}
	}
	sort.Strings(out)
	return out
}

// ImportZip imports the picked files of a staged zip
func (s *Store) ImportZip(id string, picks []ZipPick) ([]ZipResult, error) {
	z, err := s.openStaged(id)
	if err != nil {
		return nil, err
	}
	defer z.Close()
	byName := map[string]*zip.File{}
	for _, f := range z.File {
		byName[f.Name] = f
	}

	results := []ZipResult{}
	for _, p := range picks {
		r := ZipResult{File: p.Entry}
		f := byName[p.Entry]
		if f != nil && p.Remove {
			r.Result, err = s.AddRemoval(path.Join(p.Folder, path.Base(strings.ReplaceAll(f.Name, "\\", "/"))))
			if err != nil {
				r.Error = err.Error()
			}
			results = append(results, r)
			continue
		}
		var data []byte
		switch {
		case f == nil:
			err = errors.New("not in the zip")
		case f.UncompressedSize64 > maxUploadBytes:
			err = fmt.Errorf("larger than %v MB", maxUploadBytes>>20)
		default:
			data, err = readZipFile(f)
		}
		if err == nil {
			r.Result, err = s.importInto(p.Folder, path.Base(strings.ReplaceAll(f.Name, "\\", "/")), data, true)
		}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	return results, nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	// the size in a zip's directory can lie: read no more than allowed
	data, err := io.ReadAll(io.LimitReader(rc, maxUploadBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxUploadBytes {
		return nil, fmt.Errorf("larger than %v MB", maxUploadBytes>>20)
	}
	return data, nil
}
