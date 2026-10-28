package clientassets

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"compress/zlib"

	"github.com/EQEmu/spire/internal/logger"
	"github.com/EQEmu/spire/internal/pathmgmt"
)

// Store keeps the client's own icons and item models, so Spire shows what the
// RoF2 client shows instead of only the stock asset pack:
//
//   - icon sheets (dragitem<N>.dds/.png) are cut into one PNG per icon;
//   - item model archives (.eqg) are kept, their IT models listed, and a
//     preview per model is stored once the Icons & Models page renders it.
//
// Files go in through the page or by copying them into the drop folder.
type Store struct {
	logger   *logger.AppLogger
	pathmgmt *pathmgmt.PathManagement
	mu       sync.Mutex
	dir      string
	index    *Index
	changed  []func()
}

// Index is what the store holds; saved as index.json
type Index struct {
	Version  int64                  `json:"version"`
	Sheets   map[int]*SheetInfo     `json:"sheets"`
	Models   map[int]*ModelInfo     `json:"models"`
	Archives map[string]*Archive    `json:"archives"`
	Files    map[string]*ClientFile `json:"files"` // kept for the client patch, by lower-case name
}

type SheetInfo struct {
	Sheet     int    `json:"sheet"`
	File      string `json:"file"`
	FirstIcon int    `json:"first_icon"`
	LastIcon  int    `json:"last_icon"`
	Updated   int64  `json:"updated"`
	Remove    bool   `json:"remove,omitempty"` // deleted from players' clients, not sent
	Ignore    bool   `json:"ignore,omitempty"` // kept in Spire only: neither sent nor deleted
	Beta      bool   `json:"beta,omitempty"`   // sent to the beta channel only
}

type ModelInfo struct {
	Id      int    `json:"id"`
	Archive string `json:"archive"`
	Preview bool   `json:"preview"`
	Updated int64  `json:"updated"`
}

type Archive struct {
	File    string `json:"file"`
	Models  []int  `json:"models"`
	Updated int64  `json:"updated"`
	// "armor" for a Heroes' Forge armor set (a piece per slot and race, none
	// of which renders alone), "items" for item models; "" not yet looked at
	Kind    string `json:"kind,omitempty"`
	Texture string `json:"texture,omitempty"` // armor: the colour texture shown for the set
	Swatch  bool   `json:"swatch,omitempty"`  // armor: models/set_<file>.png holds that texture
	Ignore  bool   `json:"ignore,omitempty"`  // kept in Spire only: neither sent nor deleted
	Remove  bool   `json:"remove,omitempty"`  // deleted from players' clients, not sent
	Beta    bool   `json:"beta,omitempty"`    // sent to the beta channel only
}

const (
	// RoF2 ships 178 icon sheets of 36 icons: icon ids 500 to 6907. Spire's
	// stock asset pack comes from a newer client and runs to 10760; icons
	// above this are not in the RoF2 client and are not offered.
	Rof2LastStockIcon = 6907

	iconSize      = 40
	iconsPerAxis  = 6
	iconsPerSheet = iconsPerAxis * iconsPerAxis
	firstIcon     = 500

	previewSize = 125 // the stock item model previews are 125px

	maxUploadBytes = 64 << 20
)

var (
	sheetName = regexp.MustCompile(`(?i)^dragitem(\d+)\.(dds|png)$`)
	modelName = regexp.MustCompile(`(?i)^it(\d+)\.mod$`)
)

func NewStore(logger *logger.AppLogger, pathmgmt *pathmgmt.PathManagement) *Store {
	return &Store{logger: logger, pathmgmt: pathmgmt}
}

// Dir is where everything is kept: next to the server when Spire knows where
// that is, otherwise the user cache folder
func (s *Store) Dir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dirLocked()
}

func (s *Store) dirLocked() string {
	if s.dir != "" {
		return s.dir
	}
	base := s.pathmgmt.GetEQEmuServerPath()
	if base == "" {
		cache, err := os.UserCacheDir()
		if err != nil {
			cache = os.TempDir()
		}
		base = filepath.Join(cache, "spire")
	}
	s.dir = filepath.Join(base, "spire-client-assets")
	for _, d := range []string{"drop", "icons", "models", "archives", "sheets", "files"} {
		_ = os.MkdirAll(filepath.Join(s.dir, d), 0755)
	}
	return s.dir
}

// DropDir is the folder the watcher imports from
func (s *Store) DropDir() string {
	return filepath.Join(s.Dir(), "drop")
}

// OnChange registers a callback run after anything is imported or removed
func (s *Store) OnChange(f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.changed = append(s.changed, f)
}

// Index returns a copy of the index
func (s *Store) Index() Index {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.loadLocked()
	b, _ := json.Marshal(idx)
	var out Index
	_ = json.Unmarshal(b, &out)
	return out
}

func (s *Store) loadLocked() *Index {
	if s.index != nil {
		return s.index
	}
	idx := &Index{Sheets: map[int]*SheetInfo{}, Models: map[int]*ModelInfo{}, Archives: map[string]*Archive{}}
	if b, err := os.ReadFile(filepath.Join(s.dirLocked(), "index.json")); err == nil {
		_ = json.Unmarshal(b, idx)
	}
	if idx.Sheets == nil {
		idx.Sheets = map[int]*SheetInfo{}
	}
	if idx.Models == nil {
		idx.Models = map[int]*ModelInfo{}
	}
	if idx.Archives == nil {
		idx.Archives = map[string]*Archive{}
	}
	if idx.Files == nil {
		idx.Files = map[string]*ClientFile{}
	}
	s.migrateFilesLocked(idx)
	s.classifyArchivesLocked(idx)
	s.index = idx
	return idx
}

// saveLocked writes the index and the stylesheet, then runs the change hooks
func (s *Store) saveLocked() error {
	idx := s.loadLocked()
	idx.Version = time.Now().UnixNano()
	b, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.dirLocked(), "index.json"), b, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.dirLocked(), "client-assets.css"), []byte(s.cssLocked()), 0644); err != nil {
		return err
	}
	hooks := append([]func(){}, s.changed...)
	go func() {
		for _, f := range hooks {
			f()
		}
	}()
	return nil
}

// Import takes one dropped or uploaded file. It returns what was done.
func (s *Store) Import(name string, data []byte) (string, error) {
	return s.ImportAt("", name, data)
}

// ImportReader is Import for request bodies and files, with a size cap
func (s *Store) ImportReader(name string, r io.Reader) (string, error) {
	return s.ImportReaderAt("", name, r, "")
}

// ImportReaderAt is ImportAt for request bodies and files, with a size cap;
// hold keeps the file from players: ModeBeta sends it to the beta channel
// only, ModeIgnore to neither ("" sends it)
func (s *Store) ImportReaderAt(folder, name string, r io.Reader, hold string) (string, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxUploadBytes+1))
	if err != nil {
		return "", err
	}
	if len(data) > maxUploadBytes {
		return "", fmt.Errorf("%v is larger than %v MB", name, maxUploadBytes>>20)
	}
	return s.importInto(folder, name, data, false, hold)
}

// importSheet cuts a 256x256 icon sheet into its 36 icons. Sheet N is the
// N-th <Frames> entry of A_DragItem in EQUI_Animations.xml; the client fills
// a sheet column by column, so cell c is column c/6, row c%6.
func (s *Store) importSheet(name string, data []byte, hold string) (string, error) {
	n, _ := strconv.Atoi(sheetName.FindStringSubmatch(name)[1])
	if n < 1 {
		return "", fmt.Errorf("%v: sheet numbers start at 1", name)
	}

	var img image.Image
	var err error
	if strings.EqualFold(filepath.Ext(name), ".dds") {
		img, err = decodeDDS(data)
	} else {
		img, err = png.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return "", fmt.Errorf("%v: %v", name, err)
	}
	if b := img.Bounds(); b.Dx() != 256 || b.Dy() != 256 {
		return "", fmt.Errorf("%v is %vx%v; icon sheets must be 256x256 like the stock ones", name, b.Dx(), b.Dy())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	first := firstIcon + iconsPerSheet*(n-1)
	for c := 0; c < iconsPerSheet; c++ {
		cell := image.NewNRGBA(image.Rect(0, 0, iconSize, iconSize))
		at := image.Pt((c/iconsPerAxis)*iconSize, (c%iconsPerAxis)*iconSize)
		draw.Draw(cell, cell.Bounds(), img, img.Bounds().Min.Add(at), draw.Src)

		var out bytes.Buffer
		if err := png.Encode(&out, cell); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(s.dirLocked(), "icons", fmt.Sprintf("item_%d.png", first+c)), out.Bytes(), 0644); err != nil {
			return "", err
		}
	}

	// the client patch sends the sheet as it was uploaded; a .png goes as a DDS
	sheet := data
	if !strings.EqualFold(filepath.Ext(name), ".dds") {
		sheet = encodeDDS(img)
	}
	_ = os.MkdirAll(filepath.Join(s.dirLocked(), "sheets"), 0755)
	if err := os.WriteFile(filepath.Join(s.dirLocked(), "sheets", sheetFile(n)), sheet, 0644); err != nil {
		return "", err
	}

	idx := s.loadLocked()
	// a sheet held back from players stays held back when it is uploaded again
	if hold == "" && idx.Sheets[n] != nil {
		hold = holdMode(idx.Sheets[n].Ignore, idx.Sheets[n].Beta)
	}
	if hold == "" {
		hold = s.heldFileLocked(idx, "uifiles/default/"+sheetFile(n))
	}
	s.replacesFileLocked(idx, "uifiles/default/"+sheetFile(n))
	idx.Sheets[n] = &SheetInfo{Ignore: hold == ModeIgnore, Beta: hold == ModeBeta,
		Sheet: n, File: name, FirstIcon: first, LastIcon: first + iconsPerSheet - 1, Updated: time.Now().Unix()}
	if err := s.saveLocked(); err != nil {
		return "", err
	}

	msg := fmt.Sprintf("%v: icons %v-%v", name, first, first+iconsPerSheet-1)
	if n <= 178 {
		msg += " (replaces a stock sheet)"
	}
	return msg, nil
}

// importArchive keeps an .eqg and lists the IT models in it
func (s *Store) importArchive(name string, data []byte, hold string) (string, error) {
	names, err := pfsFileNames(data)
	if err != nil {
		return "", fmt.Errorf("%v: %v", name, err)
	}

	var ids []int
	for _, f := range names {
		if m := modelName.FindStringSubmatch(f); m != nil {
			id, _ := strconv.Atoi(m[1])
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	if len(ids) == 0 {
		// a race or zone archive: not an item model, but the client still needs it
		return s.importClientFile("", name, data, true, hold)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.WriteFile(filepath.Join(s.dirLocked(), "archives", name), data, 0644); err != nil {
		return "", err
	}

	idx := s.loadLocked()
	// an archive held back from players stays held back when it is uploaded again
	if old := idx.Archives[strings.ToLower(name)]; old != nil && hold == "" {
		hold = holdMode(old.Ignore, old.Beta)
	}
	if hold == "" {
		hold = s.heldFileLocked(idx, name)
	}
	s.replacesFileLocked(idx, name)
	now := time.Now().Unix()

	// a re-imported archive replaces its old model list
	if old := idx.Archives[strings.ToLower(name)]; old != nil {
		for _, id := range old.Models {
			if m := idx.Models[id]; m != nil && strings.EqualFold(m.Archive, name) {
				delete(idx.Models, id)
			}
		}
	}
	a := &Archive{File: name, Models: ids, Updated: now, Ignore: hold == ModeIgnore, Beta: hold == ModeBeta}
	a.Kind, a.Texture = classifyArchive(names, ids)
	idx.Archives[strings.ToLower(name)] = a
	for _, id := range ids {
		// the archive changed, so its previews are redrawn
		_ = os.Remove(filepath.Join(s.dirLocked(), "models", fmt.Sprintf("CTN_%d.png", id)))
	}
	s.applyKindLocked(idx, a, data)
	if err := s.saveLocked(); err != nil {
		return "", err
	}

	// the model numbers, for items' model fields; an armor set says what it is
	if a.Kind == ArchiveArmor || len(ids) > 8 {
		return fmt.Sprintf("%v: %v", name, modelsLabel(a.Kind, ids)), nil
	}
	labels := make([]string, len(ids))
	for i, id := range ids {
		labels[i] = fmt.Sprintf("IT%d", id)
	}
	return fmt.Sprintf("%v: %v", name, strings.Join(labels, ", ")), nil
}

// SavePreview stores the rendered preview of a model in the store
func (s *Store) SavePreview(id int, data []byte) error {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("preview is not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() > 512 || b.Dy() > 512 {
		return errors.New("preview is larger than 512px")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.loadLocked()
	m := idx.Models[id]
	if m == nil {
		return fmt.Errorf("IT%d is not in any imported archive", id)
	}
	if err := os.WriteFile(filepath.Join(s.dirLocked(), "models", fmt.Sprintf("CTN_%d.png", id)), data, 0644); err != nil {
		return err
	}
	m.Preview = true
	m.Updated = time.Now().Unix()
	return s.saveLocked()
}

// RemoveSheet drops an imported sheet; Spire shows the stock icons again
func (s *Store) RemoveSheet(n int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.loadLocked()
	sh := idx.Sheets[n]
	if sh == nil {
		return fmt.Errorf("sheet %d is not imported", n)
	}
	for id := sh.FirstIcon; id <= sh.LastIcon; id++ {
		_ = os.Remove(filepath.Join(s.dirLocked(), "icons", fmt.Sprintf("item_%d.png", id)))
	}
	_ = os.Remove(filepath.Join(s.dirLocked(), "sheets", sheetFile(n)))
	delete(idx.Sheets, n)
	return s.saveLocked()
}

// RemoveArchive drops an imported archive and its models
func (s *Store) RemoveArchive(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.loadLocked()
	a := idx.Archives[strings.ToLower(name)]
	if a == nil {
		return fmt.Errorf("%v is not imported", name)
	}
	for _, id := range a.Models {
		if m := idx.Models[id]; m != nil && strings.EqualFold(m.Archive, a.File) {
			delete(idx.Models, id)
			_ = os.Remove(filepath.Join(s.dirLocked(), "models", fmt.Sprintf("CTN_%d.png", id)))
		}
	}
	_ = os.Remove(filepath.Join(s.dirLocked(), "archives", a.File))
	delete(idx.Archives, strings.ToLower(name))
	return s.saveLocked()
}

// CustomIconIds are the icon ids imported sheets provide
func (s *Store) CustomIconIds() []int {
	idx := s.Index()
	var ids []int
	for _, sh := range idx.Sheets {
		for id := sh.FirstIcon; id <= sh.LastIcon; id++ {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids
}

// PreviewModelIds are the imported models that have a preview
func (s *Store) PreviewModelIds() []int {
	idx := s.Index()
	var ids []int
	for id, m := range idx.Models {
		if m.Preview {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids
}

// cssLocked overrides the stock sprite classes for every imported icon and
// model. URLs are relative to the stylesheet, so it works wherever it is
// served from.
func (s *Store) cssLocked() string {
	idx := s.loadLocked()
	var b strings.Builder
	b.WriteString("/* generated by Spire from imported client assets */\n")

	var sheets []int
	for n := range idx.Sheets {
		sheets = append(sheets, n)
	}
	sort.Ints(sheets)
	for _, n := range sheets {
		sh := idx.Sheets[n]
		for id := sh.FirstIcon; id <= sh.LastIcon; id++ {
			url := fmt.Sprintf("icons/item_%d.png?v=%d", id, sh.Updated)
			fmt.Fprintf(&b, ".item-%d { background: url('%s') 0 0 / 40px 40px no-repeat; height: 40px; width: 40px; display: inline-block; }\n", id, url)
			fmt.Fprintf(&b, ".item-%d-sm { background: url('%s') 0 0 / 13px 13px no-repeat; height: 13px; width: 13px; display: inline-block; }\n", id, url)
		}
	}

	var models []int
	for id, m := range idx.Models {
		if m.Preview {
			models = append(models, id)
		}
	}
	sort.Ints(models)
	for _, id := range models {
		fmt.Fprintf(&b, ".object-ctn-%d { background: url('models/CTN_%d.png?v=%d') center / contain no-repeat; height: %dpx; width: %dpx; display: inline-block; }\n",
			id, id, idx.Models[id].Updated, previewSize, previewSize)
	}
	return b.String()
}

// pfsFileNames lists the files in a PFS archive (.eqg/.s3d) from its name
// table, without unpacking the files themselves
func pfsFileNames(data []byte) ([]string, error) {
	if len(data) < 12 || string(data[4:8]) != "PFS " {
		return nil, errors.New("not an .eqg archive (no PFS header)")
	}
	dirOff := int(binary.LittleEndian.Uint32(data[0:4]))
	if dirOff+4 > len(data) {
		return nil, errors.New("bad directory offset")
	}
	count := int(binary.LittleEndian.Uint32(data[dirOff:]))
	if dirOff+4+count*12 > len(data) {
		return nil, errors.New("truncated directory")
	}

	for i := 0; i < count; i++ {
		p := dirOff + 4 + i*12
		crc := binary.LittleEndian.Uint32(data[p:])
		if crc != 0x61580AC9 {
			continue
		}
		off := int(binary.LittleEndian.Uint32(data[p+4:]))
		size := int(binary.LittleEndian.Uint32(data[p+8:]))
		table, err := pfsUnpack(data, off, size)
		if err != nil {
			return nil, err
		}

		if len(table) < 4 {
			return nil, errors.New("empty name table")
		}
		n := int(binary.LittleEndian.Uint32(table))
		names := make([]string, 0, n)
		q := 4
		for j := 0; j < n && q+4 <= len(table); j++ {
			l := int(binary.LittleEndian.Uint32(table[q:]))
			if q+4+l > len(table) {
				break
			}
			names = append(names, strings.TrimRight(string(table[q+4:q+4+l]), "\x00"))
			q += 4 + l
		}
		return names, nil
	}
	return nil, errors.New("archive has no file-name table")
}

// pfsUnpack inflates one file: a run of {compressed size, size, zlib data}
// blocks
func pfsUnpack(data []byte, off, size int) ([]byte, error) {
	if size < 0 || size > maxUploadBytes {
		return nil, errors.New("bad file size in archive")
	}
	out := make([]byte, 0, size)
	for len(out) < size {
		if off+8 > len(data) {
			return nil, errors.New("truncated file block")
		}
		plen := int(binary.LittleEndian.Uint32(data[off:]))
		if off+8+plen > len(data) {
			return nil, errors.New("truncated file block")
		}
		zr, err := zlib.NewReader(bytes.NewReader(data[off+8 : off+8+plen]))
		if err != nil {
			return nil, err
		}
		chunk, err := io.ReadAll(zr)
		_ = zr.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
		off += 8 + plen
	}
	return out[:size], nil
}
