package clientassets

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The client patch is everything Spire holds that the RoF2 client needs, laid
// out as the client folder: icon sheets in uifiles/default, item models and
// the other files uploaded for it where the client reads them. The database
// files (spells_us.txt, dbstr_us.txt, SkillCaps.txt, BaseData.txt) are added
// by the client files package.

// ClientFile is a file kept only to go into the client patch
type ClientFile struct {
	File    string `json:"file"`
	Path    string `json:"path"` // where it goes in the client folder
	Size    int64  `json:"size"`
	Updated int64  `json:"updated"`
	// deleted from players' clients if they have it, not sent; a removal
	// with no copy kept has Size 0
	Remove bool `json:"remove,omitempty"`
	Ignore bool `json:"ignore,omitempty"` // kept in Spire only: neither sent nor deleted
	Beta   bool `json:"beta,omitempty"`   // sent to the beta channel only
}

// PatchFile is one file of the client patch
type PatchFile struct {
	Path    string `json:"path"`   // in the client folder, / separated
	Source  string `json:"source"` // what it comes from, for people
	Kind    string `json:"kind"`   // sheet, models, file, removal (no copy kept), database
	Size    int64  `json:"size"`
	Updated int64  `json:"updated"`
	Remove  bool   `json:"remove"` // deleted from players' clients if found, not sent
	Ignore  bool   `json:"ignore"` // kept in Spire only: neither sent nor deleted
	Beta    bool   `json:"beta"`   // sent to the beta channel only

	read func() ([]byte, error)
	live func() ([]byte, error)
}

func (p PatchFile) Read() ([]byte, error) { return p.read() }

// LiveVariant reads the file as live gets it, when that differs from the
// beta listing's: EQUI_Animations.xml without the frames of beta-only sheets
func (p PatchFile) LiveVariant() (func() ([]byte, error), bool) { return p.live, p.live != nil }

// Rof2DragItemSheets is how many A_DragItem frames (icon sheets) the stock
// RoF2 EQUI_Animations.xml declares
const Rof2DragItemSheets = 178

// the files the patch builds from the database: an upload would be replaced
var generatedFiles = map[string]string{
	"spells_us.txt": "spells_new",
	"dbstr_us.txt":  "db_str",
	"skillcaps.txt": "skill_caps",
	"basedata.txt":  "base_data",
}

// never kept: the patch goes to players' machines, and these run on their own
var refusedExtensions = map[string]bool{
	".exe": true, ".bat": true, ".cmd": true, ".com": true, ".msi": true,
	".ps1": true, ".vbs": true, ".js": true, ".scr": true, ".lnk": true,
}

var clientFolderPart = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_ .()-]*$`)

// defaultFolder is where a client file goes when no folder is given
func defaultFolder(name string) (string, bool) {
	lower := strings.ToLower(name)
	switch {
	case lower == "globalload.txt" || lower == "globalload_chr.txt":
		return "Resources", true
	case lower == "equi.xml" || (strings.HasPrefix(lower, "equi_") && strings.HasSuffix(lower, ".xml")):
		return "uifiles/default", true
	case strings.HasSuffix(lower, "_chr.txt"), lower == "eqhost.txt", lower == "dinput8.dll",
		strings.HasSuffix(lower, ".eqg"), strings.HasSuffix(lower, ".s3d"):
		return "", true
	}
	return "", false
}

// cleanClientFolder is a folder inside the client folder, / separated, or an
// error for anything that could point outside it
func cleanClientFolder(folder string) (string, error) {
	f := strings.Trim(strings.ReplaceAll(folder, "\\", "/"), "/ ")
	if f == "" {
		return "", nil
	}
	parts := strings.Split(f, "/")
	for _, p := range parts {
		if p == "." || p == ".." || !clientFolderPart.MatchString(p) {
			return "", fmt.Errorf("%q is not a folder inside the client folder", folder)
		}
	}
	return strings.Join(parts, "/"), nil
}

// ImportAt imports a file into a folder of the client folder ("" for the one
// with eqgame.exe). Icon sheets and item model archives are imported as
// such; anything else is kept for the client patch as it is. With no folder,
// the usual one for the file is used.
func (s *Store) ImportAt(folder, name string, data []byte) (string, error) {
	return s.importInto(folder, name, data, false, "")
}

// importInto is ImportAt; placed says the folder was chosen for this file,
// so "" puts it next to eqgame.exe rather than in its usual folder
func (s *Store) importInto(folder, name string, data []byte, placed bool, hold string) (string, error) {
	base := filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	folder, err := cleanClientFolder(folder)
	if err != nil {
		return "", err
	}
	switch {
	case sheetName.MatchString(base) && (folder == "" || strings.EqualFold(folder, "uifiles/default")):
		return s.importSheet(base, data, hold)
	case strings.EqualFold(filepath.Ext(base), ".eqg") && folder == "":
		return s.importArchive(base, data, hold)
	}
	return s.importClientFile(folder, base, data, placed || folder != "", hold)
}

// importClientFile keeps a file for the patch as it is, at folder/name
func (s *Store) importClientFile(folder, name string, data []byte, explicit bool, hold string) (string, error) {
	lower := strings.ToLower(name)
	if table, ok := generatedFiles[lower]; ok {
		return "", fmt.Errorf("%v is built from the database (%v) when the patch is: change it there", name, table)
	}
	if refusedExtensions[strings.ToLower(filepath.Ext(lower))] {
		return "", fmt.Errorf("%v: programs and scripts are not sent to players", name)
	}
	if !explicit {
		f, ok := defaultFolder(name)
		if !ok {
			return "", fmt.Errorf("%v: choose the folder it goes in (or upload the client folder it is in)", name)
		}
		folder = f
	}
	path := name
	if folder != "" {
		path = folder + "/" + name
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.loadLocked()
	key := strings.ToLower(path)
	files := filepath.Join(s.dirLocked(), "files")
	// the client folder is not case-sensitive: a new upload replaces any spelling
	if old := idx.Files[key]; old != nil {
		_ = os.Remove(filepath.Join(files, filepath.FromSlash(old.Path)))
		// held back from players, it stays held back when it is uploaded again
		if hold == "" {
			hold = holdMode(old.Ignore, old.Beta)
		}
	}
	disk := filepath.Join(files, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(disk), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(disk, data, 0644); err != nil {
		return "", err
	}
	idx.Files[key] = &ClientFile{File: name, Path: path, Size: int64(len(data)), Updated: time.Now().Unix(), Ignore: hold == ModeIgnore, Beta: hold == ModeBeta}
	if err := s.saveLocked(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%v: kept for the client patch as %v", name, path), nil
}

// migrateFilesLocked moves client files kept by name (before they were kept
// by path) to their path
func (s *Store) migrateFilesLocked(idx *Index) {
	files := filepath.Join(s.dirLocked(), "files")
	for key, f := range idx.Files {
		want := strings.ToLower(f.Path)
		if key != want {
			delete(idx.Files, key)
			idx.Files[want] = f
		}
		disk := filepath.Join(files, filepath.FromSlash(f.Path))
		if _, err := os.Stat(disk); err == nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(files, f.File)); err == nil {
			_ = os.MkdirAll(filepath.Dir(disk), 0755)
			_ = os.Rename(filepath.Join(files, f.File), disk)
		}
	}
}

// ClientFileData reads a client file kept for the patch, by its path (any
// case); false when Spire keeps no copy of it
func (s *Store) ClientFileData(path string) ([]byte, bool) {
	f := s.Index().Files[strings.ToLower(path)]
	if f == nil || f.Size == 0 {
		return nil, false
	}
	b, err := os.ReadFile(filepath.Join(s.Dir(), "files", filepath.FromSlash(f.Path)))
	return b, err == nil
}

// SheetOfIcon is the icon sheet an icon id is on
func SheetOfIcon(icon int) int {
	if icon < firstIcon {
		return 0
	}
	return (icon-firstIcon)/iconsPerSheet + 1
}

// RemoveClientFile drops a file kept for the patch, by its path
func (s *Store) RemoveClientFile(path string) error {
	clean, err := cleanClientFolder(path)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.loadLocked()
	f := idx.Files[strings.ToLower(clean)]
	if f == nil {
		return fmt.Errorf("%v is not kept", path)
	}
	_ = os.Remove(filepath.Join(s.dirLocked(), "files", filepath.FromSlash(f.Path)))
	delete(idx.Files, strings.ToLower(f.Path))
	return s.saveLocked()
}

func sheetFile(n int) string {
	return fmt.Sprintf("dragitem%d.dds", n)
}

// SheetDDS is an imported sheet as the client reads it: the uploaded .dds,
// or, for a .png sheet (or one imported before uploads were kept), an
// uncompressed DDS rebuilt from its icons
func (s *Store) SheetDDS(n int) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sh := s.loadLocked().Sheets[n]
	if sh == nil {
		return nil, fmt.Errorf("sheet %d is not imported", n)
	}
	if b, err := os.ReadFile(filepath.Join(s.dirLocked(), "sheets", sheetFile(n))); err == nil {
		return b, nil
	}

	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	for c := 0; c < iconsPerSheet; c++ {
		b, err := os.ReadFile(filepath.Join(s.dirLocked(), "icons", fmt.Sprintf("item_%d.png", sh.FirstIcon+c)))
		if err != nil {
			return nil, err
		}
		icon, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		at := image.Pt((c/iconsPerAxis)*iconSize, (c%iconsPerAxis)*iconSize)
		draw.Draw(img, image.Rectangle{Min: at, Max: at.Add(image.Pt(iconSize, iconSize))}, icon, icon.Bounds().Min, draw.Src)
	}
	return encodeDDS(img), nil
}

// encodeDDS writes an uncompressed 32-bit (A8R8G8B8) DDS, one mip level
func encodeDDS(img image.Image) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]byte, 128+w*h*4)
	le := binary.LittleEndian
	copy(out[0:4], "DDS ")
	le.PutUint32(out[4:], 124)
	le.PutUint32(out[8:], 0x1|0x2|0x4|0x8|0x1000) // caps, height, width, pitch, pixel format
	le.PutUint32(out[12:], uint32(h))
	le.PutUint32(out[16:], uint32(w))
	le.PutUint32(out[20:], uint32(w*4))
	le.PutUint32(out[76:], 32)         // pixel format size
	le.PutUint32(out[80:], 0x40|0x1)   // RGB, with alpha
	le.PutUint32(out[88:], 32)         // bits per pixel
	le.PutUint32(out[92:], 0x00FF0000) // R
	le.PutUint32(out[96:], 0x0000FF00) // G
	le.PutUint32(out[100:], 0x000000FF)
	le.PutUint32(out[104:], 0xFF000000)
	le.PutUint32(out[108:], 0x1000) // a texture

	nrgba := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(nrgba, nrgba.Bounds(), img, b.Min, draw.Src)
	p := out[128:]
	for i := 0; i < w*h; i++ {
		r, g, bl, a := nrgba.Pix[i*4], nrgba.Pix[i*4+1], nrgba.Pix[i*4+2], nrgba.Pix[i*4+3]
		p[i*4], p[i*4+1], p[i*4+2], p[i*4+3] = bl, g, r, a
	}
	return out
}

var (
	dragItemBlock   = regexp.MustCompile(`(?s)<Ui2DAnimation item="A_DragItem">.*?</Ui2DAnimation>`)
	dragItemTexture = regexp.MustCompile(`(?i)<Texture>\s*dragitem(\d+)\.dds\s*</Texture>`)
)

// AddDragItemFrames adds an A_DragItem frame to EQUI_Animations.xml for each
// imported sheet past the ones it declares. Sheet N is the N-th frame, so
// frames are only added while the sheets run on without a gap.
func AddDragItemFrames(xml string, sheets []int) (string, []int, string) {
	loc := dragItemBlock.FindStringIndex(xml)
	if loc == nil {
		return xml, nil, "it has no A_DragItem animation"
	}
	block := xml[loc[0]:loc[1]]
	frames := strings.Count(block, "<Frames>")

	have := map[int]bool{}
	for _, n := range sheets {
		have[n] = true
	}
	sort.Ints(sheets)
	var added []int
	warn := ""
	for n := frames + 1; len(sheets) > 0 && n <= sheets[len(sheets)-1]; n++ {
		if !have[n] {
			warn = fmt.Sprintf("there is no dragitem%d.dds, so the sheets after it would land in the wrong frames and were not added", n)
			break
		}
		added = append(added, n)
	}
	if len(added) == 0 {
		return xml, nil, warn
	}

	nl := "\n"
	if strings.Contains(block, "\r\n") {
		nl = "\r\n"
	}
	var b strings.Builder
	for _, n := range added {
		for _, l := range []string{
			"\t\t<Frames>",
			"\t\t\t<Texture>" + sheetFile(n) + "</Texture>",
			"\t\t\t<Location>", "\t\t\t\t<X>0</X>", "\t\t\t\t<Y>0</Y>", "\t\t\t</Location>",
			"\t\t\t<Size>", "\t\t\t\t<CX>256</CX>", "\t\t\t\t<CY>256</CY>", "\t\t\t</Size>",
			"\t\t\t<Hotspot>", "\t\t\t\t<X>0</X>", "\t\t\t\t<Y>0</Y>", "\t\t\t</Hotspot>",
			"\t\t\t<Duration>1000</Duration>",
			"\t\t</Frames>",
		} {
			b.WriteString(l + nl)
		}
	}
	end := strings.LastIndex(block, "</Ui2DAnimation>")
	lineStart := strings.LastIndex(block[:end], "\n") + 1
	block = block[:lineStart] + b.String() + block[lineStart:]
	return xml[:loc[0]] + block + xml[loc[1]:], added, warn
}

// PatchFiles lists what the store puts in the client patch, and anything
// the client would not show as expected. beta builds EQUI_Animations.xml as
// the beta channel gets it, with frames for its beta-only sheets too.
func (s *Store) PatchFiles(beta bool) ([]PatchFile, []string) {
	idx := s.Index()
	dir := s.Dir()
	var files []PatchFile
	var notes []string

	var sheets []int
	for n := range idx.Sheets {
		sheets = append(sheets, n)
	}
	sort.Ints(sheets)
	for _, n := range sheets {
		n := n
		source := "icon sheet (as uploaded)"
		if !strings.EqualFold(filepath.Ext(idx.Sheets[n].File), ".dds") {
			source = "icon sheet (" + idx.Sheets[n].File + ", as an uncompressed DDS)"
		}
		size := int64(0)
		if fi, err := os.Stat(filepath.Join(dir, "sheets", sheetFile(n))); err == nil {
			size = fi.Size()
		} else {
			source = "icon sheet (rebuilt from its icons)"
			size = 128 + 256*256*4
		}
		files = append(files, PatchFile{
			Path: "uifiles/default/" + sheetFile(n), Source: source, Kind: "sheet", Size: size, Updated: idx.Sheets[n].Updated,
			Remove: idx.Sheets[n].Remove, Ignore: idx.Sheets[n].Ignore, Beta: idx.Sheets[n].Beta,
			read: func() ([]byte, error) { return s.SheetDDS(n) },
		})
	}

	keep := func(path, source, kind, file string, updated int64, remove, ignore, beta bool) {
		size := int64(0)
		if fi, err := os.Stat(file); err == nil {
			size = fi.Size()
		}
		files = append(files, PatchFile{Path: path, Source: source, Kind: kind, Size: size, Updated: updated, Remove: remove, Ignore: ignore, Beta: beta,
			read: func() ([]byte, error) { return os.ReadFile(file) }})
	}
	for _, a := range idx.Archives {
		keep(a.File, modelsLabel(a.Kind, a.Models), "models", filepath.Join(dir, "archives", a.File), a.Updated, a.Remove, a.Ignore, a.Beta)
	}

	var animations *ClientFile
	for _, f := range idx.Files {
		if f.Remove && f.Size == 0 {
			files = append(files, PatchFile{Path: f.Path, Source: "removed from players (Spire keeps no copy)", Kind: "removal",
				Updated: f.Updated, Remove: true})
			continue
		}
		if strings.EqualFold(f.Path, "uifiles/default/EQUI_Animations.xml") && !f.Remove && !f.Ignore && (beta || !f.Beta) {
			animations = f
			continue
		}
		keep(f.Path, "uploaded", "file", filepath.Join(dir, "files", filepath.FromSlash(f.Path)), f.Updated, f.Remove, f.Ignore, f.Beta)
	}

	// frames are added for the sheets the channel's players get
	sent, liveSent := sheets[:0:0], sheets[:0:0]
	for _, n := range sheets {
		if sh := idx.Sheets[n]; !sh.Remove && !sh.Ignore {
			if beta || !sh.Beta {
				sent = append(sent, n)
			}
			if !sh.Beta {
				liveSent = append(liveSent, n)
			}
		}
	}
	sheets = sent

	past := 0
	for _, n := range sheets {
		if n > Rof2DragItemSheets {
			past++
		}
	}
	switch {
	case animations != nil:
		file := filepath.Join(dir, "files", filepath.FromSlash(animations.Path))
		b, err := os.ReadFile(file)
		if err != nil {
			notes = append(notes, "EQUI_Animations.xml could not be read: "+err.Error())
			break
		}
		xml, added, warn := AddDragItemFrames(string(b), sheets)
		source := "uploaded"
		if len(added) == 1 {
			source = fmt.Sprintf("uploaded, with an A_DragItem frame added for sheet %d", added[0])
		} else if len(added) > 1 {
			source = fmt.Sprintf("uploaded, with A_DragItem frames added for sheets %d-%d", added[0], added[len(added)-1])
		}
		if warn != "" {
			notes = append(notes, "EQUI_Animations.xml: "+warn+".")
		}
		anim := PatchFile{Path: animations.Path, Source: source, Kind: "file", Size: int64(len(xml)), Updated: animations.Updated,
			Beta: animations.Beta,
			read: func() ([]byte, error) { return []byte(xml), nil }}
		// promoting beta to live sends live's, without beta-only sheets' frames
		if liveXML, _, _ := AddDragItemFrames(string(b), liveSent); beta && !animations.Beta && liveXML != xml {
			anim.live = func() ([]byte, error) { return []byte(liveXML), nil }
		}
		files = append(files, anim)
	case past > 0:
		notes = append(notes, fmt.Sprintf(
			"%d icon sheet(s) are past the %d the stock client declares, and the client shows no icon from them until "+
				"EQUI_Animations.xml lists them. Upload the client's uifiles/default/EQUI_Animations.xml to Spire and "+
				"the patch adds their A_DragItem frames.", past, Rof2DragItemSheets))
	}

	sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].Path) < strings.ToLower(files[j].Path) })
	return files, notes
}

// files never deleted from players' clients: Spire builds them, or the client
// cannot run without them
var keptOnClients = map[string]string{
	"uifiles/default/equi.xml":            "the client's UI cannot load without it",
	"uifiles/default/equi_animations.xml": "the client's UI cannot load without it",
}

// cleanClientPath is a path inside the client folder, / separated
func cleanClientPath(p string) (string, error) {
	clean, err := cleanClientFolder(p)
	if err == nil && clean == "" {
		err = errors.New("no file named")
	}
	return clean, err
}

// removable says why a file may not be deleted from players' clients, or nil
func removable(path string) error {
	lower := strings.ToLower(path)
	base := strings.ToLower(filepath.Base(path))
	if table, ok := generatedFiles[base]; ok {
		return fmt.Errorf("%v is built from the database (%v): players need it", path, table)
	}
	if refusedExtensions[filepath.Ext(base)] {
		return fmt.Errorf("%v: programs are not deleted from players", path)
	}
	if why, ok := keptOnClients[lower]; ok {
		return fmt.Errorf("%v: %v", path, why)
	}
	if m := sheetName.FindStringSubmatch(base); m != nil && strings.HasPrefix(lower, "uifiles/default/") {
		if n, _ := strconv.Atoi(m[1]); n <= Rof2DragItemSheets {
			return fmt.Errorf("%v is a stock icon sheet: deleting it blanks those icons for every player", path)
		}
	}
	return nil
}

// Modes of a file in the client patch
const (
	ModeAdd    = "add"    // sent to players, live and beta
	ModeBeta   = "beta"   // sent to the beta channel only
	ModeIgnore = "ignore" // kept in Spire only: neither sent nor deleted
	ModeRemove = "remove" // deleted from players' clients if found, live and beta
)

// holdMode is how a file is held back from players: ModeIgnore, ModeBeta,
// or "" when it is sent
func holdMode(ignore, beta bool) string {
	switch {
	case ignore:
		return ModeIgnore
	case beta:
		return ModeBeta
	}
	return ""
}

// SetMode sets what the patch does with a file: send it, send it to beta
// only, leave it out (Spire keeps using it), or delete it from players'
// clients. Spire keeps its copy either way, so the mode can be changed back.
func (s *Store) SetMode(path, mode string) error {
	if mode != ModeAdd && mode != ModeBeta && mode != ModeIgnore && mode != ModeRemove {
		return fmt.Errorf("%q is not add, beta, ignore or remove", mode)
	}
	path, err := cleanClientPath(path)
	if err != nil {
		return err
	}
	if mode == ModeRemove {
		if err := removable(path); err != nil {
			return err
		}
	}
	remove, ignore, beta := mode == ModeRemove, mode == ModeIgnore, mode == ModeBeta
	lower := strings.ToLower(path)

	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.loadLocked()
	switch {
	case idx.Files[lower] != nil:
		f := idx.Files[lower]
		if mode != ModeRemove && f.Size == 0 {
			return fmt.Errorf("Spire keeps no copy of %v: upload it to send or keep it", path)
		}
		f.Remove, f.Ignore, f.Beta = remove, ignore, beta
	case !strings.Contains(path, "/") && idx.Archives[lower] != nil:
		idx.Archives[lower].Remove, idx.Archives[lower].Ignore, idx.Archives[lower].Beta = remove, ignore, beta
	default:
		m := sheetName.FindStringSubmatch(filepath.Base(path))
		n := 0
		if m != nil && strings.HasPrefix(lower, "uifiles/default/") {
			n, _ = strconv.Atoi(m[1])
		}
		if idx.Sheets[n] == nil {
			return fmt.Errorf("%v is not in the patch", path)
		}
		idx.Sheets[n].Remove, idx.Sheets[n].Ignore, idx.Sheets[n].Beta = remove, ignore, beta
	}
	return s.saveLocked()
}

// SetRemove is SetMode remove (true) or add (false)
func (s *Store) SetRemove(path string, remove bool) error {
	if remove {
		return s.SetMode(path, ModeRemove)
	}
	return s.SetMode(path, ModeAdd)
}

// heldFileLocked is how a client file kept at path is held back from
// players (holdMode)
func (s *Store) heldFileLocked(idx *Index, path string) string {
	if f := idx.Files[strings.ToLower(path)]; f != nil {
		return holdMode(f.Ignore, f.Beta)
	}
	return ""
}

// AddRemoval deletes a file from players' clients if they have it: one that
// went out by mistake, or that Spire never held
func (s *Store) AddRemoval(path string) (string, error) {
	path, err := cleanClientPath(path)
	if err != nil {
		return "", err
	}
	if err := removable(path); err != nil {
		return "", err
	}
	// a file the patch holds is switched to Remove
	if s.SetRemove(path, true) == nil {
		return path + ": removed from players (Spire keeps its copy)", nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.loadLocked()
	idx.Files[strings.ToLower(path)] = &ClientFile{File: filepath.Base(path), Path: path, Updated: time.Now().Unix(), Remove: true}
	if err := s.saveLocked(); err != nil {
		return "", err
	}
	return path + ": deleted from players' clients if found", nil
}

// replacesFileLocked drops a client file kept at a path an icon sheet or item
// model archive now takes, a removal included: the new one is sent
func (s *Store) replacesFileLocked(idx *Index, path string) {
	key := strings.ToLower(path)
	if f := idx.Files[key]; f != nil {
		_ = os.Remove(filepath.Join(s.dirLocked(), "files", filepath.FromSlash(f.Path)))
		delete(idx.Files, key)
	}
}
