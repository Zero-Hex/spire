package clientassets

import (
	"bytes"
	"encoding/binary"
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
// files (spells_us.txt, dbstr_us.txt) are added by the client files package.

// ClientFile is a file kept only to go into the client patch
type ClientFile struct {
	File    string `json:"file"`
	Path    string `json:"path"` // where it goes in the client folder
	Size    int64  `json:"size"`
	Updated int64  `json:"updated"`
}

// PatchFile is one file of the client patch
type PatchFile struct {
	Path    string `json:"path"`   // in the client folder, / separated
	Source  string `json:"source"` // what it comes from, for people
	Size    int64  `json:"size"`
	Updated int64  `json:"updated"`

	read func() ([]byte, error)
}

func (p PatchFile) Read() ([]byte, error) { return p.read() }

// Rof2DragItemSheets is how many A_DragItem frames (icon sheets) the stock
// RoF2 EQUI_Animations.xml declares
const Rof2DragItemSheets = 178

// clientPath is where a client file goes in the client folder, or "" when it
// is not one Spire keeps
func clientPath(name string) string {
	lower := strings.ToLower(name)
	switch {
	case lower == "globalload.txt" || lower == "globalload_chr.txt":
		return "Resources/" + name
	case strings.HasSuffix(lower, "_chr.txt"):
		return name
	case strings.HasPrefix(lower, "equi_") && strings.HasSuffix(lower, ".xml"):
		return "uifiles/default/" + name
	case strings.HasSuffix(lower, ".eqg") || strings.HasSuffix(lower, ".s3d"):
		return name
	}
	return ""
}

// importClientFile keeps a file for the patch as it is
func (s *Store) importClientFile(name string, data []byte) (string, error) {
	path := clientPath(name)
	if path == "" {
		return "", fmt.Errorf("%v is not a file the client patch takes", name)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.loadLocked()
	key := strings.ToLower(name)
	// the client folder is not case-sensitive: a new upload replaces any spelling
	if old := idx.Files[key]; old != nil {
		_ = os.Remove(filepath.Join(s.dirLocked(), "files", old.File))
	}
	_ = os.MkdirAll(filepath.Join(s.dirLocked(), "files"), 0755)
	if err := os.WriteFile(filepath.Join(s.dirLocked(), "files", name), data, 0644); err != nil {
		return "", err
	}
	idx.Files[key] = &ClientFile{File: name, Path: path, Size: int64(len(data)), Updated: time.Now().Unix()}
	if err := s.saveLocked(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%v: kept for the client patch as %v", name, path), nil
}

// RemoveClientFile drops a file kept for the patch
func (s *Store) RemoveClientFile(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.loadLocked()
	f := idx.Files[strings.ToLower(filepath.Base(name))]
	if f == nil {
		return fmt.Errorf("%v is not kept", name)
	}
	_ = os.Remove(filepath.Join(s.dirLocked(), "files", f.File))
	delete(idx.Files, strings.ToLower(f.File))
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
// the client would not show as expected
func (s *Store) PatchFiles() ([]PatchFile, []string) {
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
			Path: "uifiles/default/" + sheetFile(n), Source: source, Size: size, Updated: idx.Sheets[n].Updated,
			read: func() ([]byte, error) { return s.SheetDDS(n) },
		})
	}

	keep := func(path, source, file string, updated int64) {
		size := int64(0)
		if fi, err := os.Stat(file); err == nil {
			size = fi.Size()
		}
		files = append(files, PatchFile{Path: path, Source: source, Size: size, Updated: updated,
			read: func() ([]byte, error) { return os.ReadFile(file) }})
	}
	for _, a := range idx.Archives {
		labels := make([]string, len(a.Models))
		for i, id := range a.Models {
			labels[i] = "IT" + strconv.Itoa(id)
		}
		keep(a.File, "item models "+strings.Join(labels, ", "), filepath.Join(dir, "archives", a.File), a.Updated)
	}

	var animations *ClientFile
	for _, f := range idx.Files {
		if strings.EqualFold(f.File, "EQUI_Animations.xml") {
			animations = f
			continue
		}
		keep(f.Path, "uploaded", filepath.Join(dir, "files", f.File), f.Updated)
	}

	past := 0
	for _, n := range sheets {
		if n > Rof2DragItemSheets {
			past++
		}
	}
	switch {
	case animations != nil:
		file := filepath.Join(dir, "files", animations.File)
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
		files = append(files, PatchFile{Path: animations.Path, Source: source, Size: int64(len(xml)), Updated: animations.Updated,
			read: func() ([]byte, error) { return []byte(xml), nil }})
	case past > 0:
		notes = append(notes, fmt.Sprintf(
			"%d icon sheet(s) are past the %d the stock client declares, and the client shows no icon from them until "+
				"EQUI_Animations.xml lists them. Upload the client's uifiles/default/EQUI_Animations.xml to Spire and "+
				"the patch adds their A_DragItem frames.", past, Rof2DragItemSheets))
	}

	sort.Slice(files, func(i, j int) bool { return strings.ToLower(files[i].Path) < strings.ToLower(files[j].Path) })
	return files, notes
}
