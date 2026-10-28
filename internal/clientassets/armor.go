package clientassets

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Heroes' Forge armor comes as archives of models for each slot, race and
// gender (it63400.mod ... it63725.mod) wearing a set's texture (hfm253_c.dds).
// A piece renders as nothing on its own, so armor archives are not shown as
// item models to preview and pick, but by set (armorsets.go).

const (
	ArchiveItems = "items"
	ArchiveArmor = "armor"

	// more models than this in one archive is a set, not a pack of items
	armorPieces = 40
)

var heroesForgeTexture = regexp.MustCompile(`(?i)^hf[mf]?\d+_`)

// classifyArchive says whether an archive is an armor set, and the texture
// to show for it
func classifyArchive(names []string, models []int) (string, string) {
	var textures []string
	hf := false
	for _, n := range names {
		if strings.EqualFold(filepath.Ext(n), ".dds") {
			textures = append(textures, n)
			if heroesForgeTexture.MatchString(n) {
				hf = true
			}
		}
	}
	sort.Strings(textures)
	texture := ""
	for _, t := range textures {
		if strings.HasSuffix(strings.ToLower(t), "_c.dds") {
			texture = t
			break
		}
	}
	if texture == "" && len(textures) > 0 {
		texture = textures[0]
	}
	if hf || len(models) > armorPieces {
		return ArchiveArmor, texture
	}
	return ArchiveItems, texture
}

func swatchFile(archive string) string {
	return "set_" + strings.ToLower(strings.TrimSuffix(archive, filepath.Ext(archive))) + ".png"
}

// applyKindLocked makes an archive's models item models, or not (armor),
// and draws an armor set's texture. data is the archive, or nil to read it.
func (s *Store) applyKindLocked(idx *Index, a *Archive, data []byte) {
	models := filepath.Join(s.dirLocked(), "models")
	if a.Kind == ArchiveArmor {
		for _, id := range a.Models {
			if m := idx.Models[id]; m != nil && strings.EqualFold(m.Archive, a.File) {
				delete(idx.Models, id)
				_ = os.Remove(filepath.Join(models, fmt.Sprintf("CTN_%d.png", id)))
			}
		}
		a.Swatch = false
		if data == nil {
			b, err := os.ReadFile(filepath.Join(s.dirLocked(), "archives", a.File))
			if err != nil {
				return
			}
			data = b
		}
		s.readArmorTexturesLocked(a, data)
		if a.Texture == "" {
			return
		}
		if png, err := textureSwatch(data, a.Texture); err == nil {
			_ = os.MkdirAll(models, 0755)
			a.Swatch = os.WriteFile(filepath.Join(models, swatchFile(a.File)), png, 0644) == nil
		}
		return
	}
	for _, id := range a.Models {
		if idx.Models[id] == nil {
			idx.Models[id] = &ModelInfo{Id: id, Archive: a.File, Updated: a.Updated}
		}
	}
}

// classifyArchivesLocked sorts archives imported before armor sets were told
// apart
func (s *Store) classifyArchivesLocked(idx *Index) {
	for _, a := range idx.Archives {
		// armor read before textures were read per model
		if a.Kind == ArchiveArmor && a.ModelTextures == nil {
			if data, err := os.ReadFile(filepath.Join(s.dirLocked(), "archives", a.File)); err == nil {
				s.readArmorTexturesLocked(a, data)
			} else {
				a.ModelTextures = map[int]string{}
			}
			continue
		}
		if a.Kind != "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dirLocked(), "archives", a.File))
		if err != nil {
			a.Kind = ArchiveItems
			continue
		}
		names, _ := pfsFileNames(data)
		a.Kind, a.Texture = classifyArchive(names, a.Models)
		if a.Kind == ArchiveArmor {
			s.applyKindLocked(idx, a, data)
		}
	}
}

// SetArchiveKind corrects how an archive is shown: "armor" or "items"
func (s *Store) SetArchiveKind(name, kind string) error {
	if kind != ArchiveArmor && kind != ArchiveItems {
		return errors.New("an archive is armor or items")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.loadLocked()
	a := idx.Archives[strings.ToLower(filepath.Base(name))]
	if a == nil {
		return fmt.Errorf("%v is not imported", name)
	}
	a.Kind = kind
	s.applyKindLocked(idx, a, nil)
	return s.saveLocked()
}

// pfsFile reads one file of a PFS archive. The name table lists the files in
// the order of their offsets.
func pfsFile(data []byte, name string) ([]byte, error) {
	names, err := pfsFileNames(data)
	if err != nil {
		return nil, err
	}
	dirOff := int(binary.LittleEndian.Uint32(data[0:4]))
	count := int(binary.LittleEndian.Uint32(data[dirOff:]))
	type entry struct{ off, size int }
	var entries []entry
	for i := 0; i < count; i++ {
		p := dirOff + 4 + i*12
		if binary.LittleEndian.Uint32(data[p:]) == 0x61580AC9 {
			continue
		}
		entries = append(entries, entry{int(binary.LittleEndian.Uint32(data[p+4:])), int(binary.LittleEndian.Uint32(data[p+8:]))})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].off < entries[j].off })
	for i, n := range names {
		if strings.EqualFold(n, name) && i < len(entries) {
			return pfsUnpack(data, entries[i].off, entries[i].size)
		}
	}
	return nil, fmt.Errorf("%v is not in the archive", name)
}

// textureSwatch is a texture of an archive, scaled to a preview's size
func textureSwatch(archive []byte, texture string) ([]byte, error) {
	dds, err := pfsFile(archive, texture)
	if err != nil {
		return nil, err
	}
	img, err := decodeDDS(dds)
	if err != nil {
		return nil, err
	}
	small := image.NewNRGBA(image.Rect(0, 0, previewSize, previewSize))
	b := img.Bounds()
	for y := 0; y < previewSize; y++ {
		for x := 0; x < previewSize; x++ {
			c := img.NRGBAAt(b.Min.X+x*b.Dx()/previewSize, b.Min.Y+y*b.Dy()/previewSize)
			c.A = 255 // armor textures keep a mask in the alpha channel
			small.SetNRGBA(x, y, c)
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, small); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// modelsLabel says in a few words what an archive holds: "armor set (171
// pieces)", "item model IT23651" or "3 item models"
func modelsLabel(kind string, ids []int) string {
	switch {
	case kind == ArchiveArmor:
		return fmt.Sprintf("armor set (%d pieces)", len(ids))
	case len(ids) == 1:
		return fmt.Sprintf("item model IT%d", ids[0])
	default:
		return fmt.Sprintf("%d item models", len(ids))
	}
}
