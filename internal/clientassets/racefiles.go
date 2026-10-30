package clientassets

import (
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The client loads race models from two kinds of file:
//   - <zone>_chr.txt, next to eqgame.exe: a count line, then one
//     "CODE,archive" line per race model the zone loads (the archive without
//     its extension: rtn is rtn.eqg, tnm_chr is tnm_chr.s3d)
//   - Resources/GlobalLoad.txt: archives every zone loads, one per line as
//     "phase,flag,flags,archive,message" (an .eqg keeps its extension, an
//     .s3d does not)
// Both are edited here and kept for the client patch like any upload.

const globalLoadPath = "Resources/GlobalLoad.txt"

// RaceLine is a line of a race file. Raw is the line as it was; a line Spire
// adds or changes has no Raw and is written from its fields.
type RaceLine struct {
	Raw     string `json:"raw,omitempty"`
	Code    string `json:"code,omitempty"`    // _chr.txt: the race model code
	Archive string `json:"archive,omitempty"` // the archive, as the file names it
	Phase   string `json:"phase,omitempty"`   // GlobalLoad.txt fields
	Flag    string `json:"flag,omitempty"`
	Flags   string `json:"flags,omitempty"`
	Message string `json:"message,omitempty"`
}

type RaceFile struct {
	Path  string     `json:"path"`
	Kind  string     `json:"kind"` // "chr" or "globalload"
	Lines []RaceLine `json:"lines"`
	CRLF  bool       `json:"crlf"`
	// _chr.txt: its count line as read, and how many lines were read. Some
	// stock files' counts disagree with their lines; the count is written
	// again only when the lines change.
	Header string `json:"header,omitempty"`
	Read   int    `json:"read"`
}

//go:embed rof2_chr_zones.txt
var rof2ChrZonesFile string

// Rof2ChrZones are the zones the stock client ships a _chr.txt for
var Rof2ChrZones = func() map[string]bool {
	out := map[string]bool{}
	for i, l := range strings.Split(rof2ChrZonesFile, "\n") {
		if l = strings.TrimSpace(l); i >= 2 && l != "" {
			out[l] = true
		}
	}
	return out
}()

var chrName = regexp.MustCompile(`(?i)^([a-z0-9_]+)_chr\.txt$`)

// RaceFileKind says which race file a path in the client folder is, or ""
func RaceFileKind(path string) string {
	switch {
	case strings.EqualFold(path, globalLoadPath):
		return "globalload"
	case !strings.Contains(path, "/") && chrName.MatchString(path):
		return "chr"
	}
	return ""
}

// ParseRaceFile reads a _chr.txt or GlobalLoad.txt
func ParseRaceFile(path string, data []byte) (RaceFile, error) {
	kind := RaceFileKind(path)
	if kind == "" {
		return RaceFile{}, fmt.Errorf("%v is not a <zone>_chr.txt or %v", path, globalLoadPath)
	}
	text := string(data)
	f := RaceFile{Path: path, Kind: kind, Lines: []RaceLine{}, CRLF: strings.Contains(text, "\r\n")}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, raw := range lines {
		l := strings.TrimSpace(raw)
		if kind == "chr" && i == 0 {
			f.Header = raw
			continue
		}
		if raw == "" {
			continue
		}
		cols := strings.Split(l, ",")
		line := RaceLine{Raw: raw}
		switch {
		case kind == "chr" && len(cols) >= 2:
			line.Code, line.Archive = strings.TrimSpace(cols[0]), strings.TrimSpace(cols[1])
		case kind == "globalload" && len(cols) >= 5:
			line.Phase, line.Flag, line.Flags = strings.TrimSpace(cols[0]), strings.TrimSpace(cols[1]), strings.TrimSpace(cols[2])
			line.Archive, line.Message = strings.TrimSpace(cols[3]), strings.Join(cols[4:], ",")
		}
		f.Lines = append(f.Lines, line)
	}
	f.Read = len(f.Lines)
	return f, nil
}

var (
	raceCode    = regexp.MustCompile(`^[A-Za-z0-9]{3}$`)
	raceArchive = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

// Format writes the file back: lines read keep their text, new ones are
// written from their fields, and a _chr.txt's count is the lines it has
func (f RaceFile) Format() ([]byte, error) {
	var out []string
	for _, l := range f.Lines {
		if l.Raw != "" {
			out = append(out, l.Raw)
			continue
		}
		if !raceArchive.MatchString(l.Archive) {
			return nil, fmt.Errorf("%q is not an archive name", l.Archive)
		}
		switch f.Kind {
		case "chr":
			if !raceCode.MatchString(l.Code) {
				return nil, fmt.Errorf("%q is not a race model code (three letters or digits, like HUM)", l.Code)
			}
			out = append(out, strings.ToUpper(l.Code)+","+l.Archive)
		case "globalload":
			phase, flag, flags, msg := l.Phase, l.Flag, l.Flags, l.Message
			if phase == "" {
				phase, flag, flags, msg = "3", "0", "TFFFC", "Loading Characters"
			}
			if strings.ContainsAny(phase+flag+flags+msg, ",\r\n") {
				return nil, errors.New("GlobalLoad.txt fields hold no commas or line breaks")
			}
			out = append(out, strings.Join([]string{phase, flag, flags, l.Archive, msg}, ","))
		}
	}
	if f.Kind == "chr" {
		edited := len(f.Lines) != f.Read || f.Header == ""
		count := 0
		for _, l := range f.Lines {
			edited = edited || l.Raw == ""
			if l.Raw == "" || strings.Contains(l.Raw, ",") {
				count++
			}
		}
		header := f.Header
		if edited {
			header = strconv.Itoa(count)
		}
		out = append([]string{header}, out...)
	}
	eol := "\n"
	if f.CRLF {
		eol = "\r\n"
	}
	return []byte(strings.Join(out, eol) + eol), nil
}

// RaceFiles lists the race files Spire holds for the patch
func (s *Store) RaceFiles() []ClientFile {
	out := []ClientFile{}
	for _, f := range s.Index().Files {
		if f.Size > 0 && RaceFileKind(f.Path) != "" {
			out = append(out, *f)
		}
	}
	return out
}

// ReadRaceFile reads a race file Spire holds
func (s *Store) ReadRaceFile(path string) (RaceFile, error) {
	data, ok := s.ClientFileData(path)
	if !ok {
		return RaceFile{}, fmt.Errorf("Spire holds no %v: upload the client's copy first", path)
	}
	f, err := ParseRaceFile(s.Index().Files[strings.ToLower(path)].Path, data)
	return f, err
}

// SaveRaceFile writes a race file for the patch, keeping its Add / Beta /
// Ignore mode. A new _chr.txt starts empty; GlobalLoad.txt is never made
// from nothing: the client cannot load without its stock lines.
func (s *Store) SaveRaceFile(f RaceFile) (string, error) {
	path, err := cleanClientPath(f.Path)
	if err != nil {
		return "", err
	}
	f.Kind = RaceFileKind(path)
	if f.Kind == "" {
		return "", fmt.Errorf("%v is not a <zone>_chr.txt or %v", path, globalLoadPath)
	}
	if _, held := s.ClientFileData(path); !held && f.Kind == "globalload" {
		return "", errors.New("upload the client's Resources/GlobalLoad.txt first: one made from nothing would stop every zone loading its global models")
	}
	if old := s.Index().Files[strings.ToLower(path)]; old != nil {
		path = old.Path // the spelling it is kept under
	}
	data, err := f.Format()
	if err != nil {
		return "", err
	}
	folder := filepath.ToSlash(filepath.Dir(path))
	if folder == "." {
		folder = ""
	}
	return s.importClientFile(folder, filepath.Base(path), data, true, "")
}
