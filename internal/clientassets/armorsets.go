package clientassets

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Armor is shown by Heroes' Forge set, not by archive: a set may be spread
// over archives (the weapon toolkit ships one per slot group), and one
// archive may hold several sets. NewArmorTagData.txt says which set each IT
// model is in; each model's own material list says which texture it wears.

// ArmorSet is a Heroes' Forge set as Spire holds it
type ArmorSet struct {
	Set      int          `json:"set"`     // 0: no NewArmorTagData.txt row names its models
	Guess    int          `json:"guess"`   // untagged: the set its texture name suggests
	Texture  string       `json:"texture"` // the colour texture most of its models wear
	Swatch   string       `json:"swatch"`  // that texture drawn small (a file under models/), or ""
	Slots    []int        `json:"slots"`   // slots it has models for; untagged: not known
	Models   []int        `json:"models"`
	Archives []SetArchive `json:"archives"`
	Missing  []int        `json:"missing"` // models its tag rows name that no archive holds
}

type SetArchive struct {
	File   string `json:"file"`
	Models int    `json:"models"`
	Beta   bool   `json:"beta"`
	Ignore bool   `json:"ignore"`
	Remove bool   `json:"remove"`
}

// ArmorSets lists the Heroes' Forge sets in the archives Spire holds: those
// NewArmorTagData.txt names, then models in armor archives it does not, by
// the texture they wear
func (s *Store) ArmorSets() []ArmorSet {
	idx := s.Index()
	var tags ArmorTags
	if data, ok := s.ClientFileData("Resources/NewArmorTagData.txt"); ok {
		tags = ParseArmorTags(data)
	}

	type group struct {
		set      *ArmorSet
		slots    map[int]bool
		textures map[string]int
		archives map[string]int
		models   map[int]bool
	}
	newGroup := func(set ArmorSet) *group {
		return &group{set: &set, slots: map[int]bool{}, textures: map[string]int{}, archives: map[string]int{}, models: map[int]bool{}}
	}
	sets := map[int]*group{}
	untagged := map[string]*group{} // archive + texture

	var files []string
	for key := range idx.Archives {
		files = append(files, key)
	}
	sort.Strings(files)
	for _, key := range files {
		a := idx.Archives[key]
		for _, it := range a.Models {
			texture := a.ModelTextures[it]
			tagged := false
			for _, id := range tags.models[it] {
				set, slot := id/100, id%100
				if set < MinSet || slot > SlotRobe {
					continue
				}
				tagged = true
				g := sets[set]
				if g == nil {
					g = newGroup(ArmorSet{Set: set})
					sets[set] = g
				}
				g.slots[slot] = true
				if !g.models[it] {
					g.models[it] = true
					g.archives[key]++
					if texture != "" {
						g.textures[texture]++
					}
				}
			}
			// an armor archive's model no tag row names
			if !tagged && a.Kind == ArchiveArmor {
				k := key + "\t" + strings.ToLower(texture)
				g := untagged[k]
				if g == nil {
					g = newGroup(ArmorSet{Guess: SetFromTexture(texture)})
					untagged[k] = g
				}
				g.models[it] = true
				g.archives[key]++
				if texture != "" {
					g.textures[texture]++
				}
			}
		}
	}

	finish := func(g *group) ArmorSet {
		set := *g.set
		set.Slots, set.Models, set.Archives, set.Missing = []int{}, []int{}, []SetArchive{}, []int{}
		if set.Set > 0 {
			for slot := range g.slots {
				set.Slots = append(set.Slots, slot)
			}
			sort.Ints(set.Slots)
			for it := range tags.sets[set.Set] {
				if !g.models[it] && !heldModel(idx, it) {
					set.Missing = append(set.Missing, it)
				}
			}
			sort.Ints(set.Missing)
		}
		for it := range g.models {
			set.Models = append(set.Models, it)
		}
		sort.Ints(set.Models)
		best := 0
		for t, n := range g.textures {
			if n > best || (n == best && t < set.Texture) {
				set.Texture, best = t, n
			}
		}
		if set.Texture != "" {
			if _, err := os.Stat(filepath.Join(s.Dir(), "models", textureSwatchFile(set.Texture))); err == nil {
				set.Swatch = "models/" + textureSwatchFile(set.Texture)
			}
		}
		for key, n := range g.archives {
			a := idx.Archives[key]
			set.Archives = append(set.Archives, SetArchive{File: a.File, Models: n, Beta: a.Beta, Ignore: a.Ignore, Remove: a.Remove})
		}
		sort.Slice(set.Archives, func(i, j int) bool { return set.Archives[i].File < set.Archives[j].File })
		return set
	}

	out := []ArmorSet{}
	var numbers []int
	for n := range sets {
		numbers = append(numbers, n)
	}
	sort.Ints(numbers)
	for _, n := range numbers {
		out = append(out, finish(sets[n]))
	}
	var keys []string
	for k := range untagged {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, finish(untagged[k]))
	}
	return out
}

// heldModel says whether any archive holds the model
func heldModel(idx Index, it int) bool {
	for _, a := range idx.Archives {
		for _, m := range a.Models {
			if m == it {
				return true
			}
		}
	}
	return false
}

var unsafeFileChars = regexp.MustCompile(`[^a-z0-9_.-]+`)

// textureSwatchFile is where a texture's swatch is kept, under models/
func textureSwatchFile(texture string) string {
	name := strings.ToLower(strings.TrimSuffix(texture, filepath.Ext(texture)))
	return "tex_" + unsafeFileChars.ReplaceAllString(name, "_") + ".png"
}

// modelTextures reads which colour texture each model in an archive wears:
// the first *_c.dds its materials name that the archive holds, else the
// first texture it names that the archive holds
func modelTextures(data []byte, models []int) map[int]string {
	out := map[int]string{}
	names, err := pfsFileNames(data)
	if err != nil {
		return out
	}
	inArchive := map[string]string{}
	for _, n := range names {
		inArchive[strings.ToLower(n)] = n
	}
	files, err := pfsFiles(data)
	if err != nil {
		return out
	}
	for _, it := range models {
		mod := files[fmt.Sprintf("it%d.mod", it)]
		if mod == nil {
			continue
		}
		first := ""
		for _, t := range eqgmStrings(mod) {
			held, ok := inArchive[strings.ToLower(t)]
			if !ok || !strings.EqualFold(filepath.Ext(t), ".dds") {
				continue
			}
			if strings.HasSuffix(strings.ToLower(t), "_c.dds") {
				first = held
				break
			}
			if first == "" {
				first = held
			}
		}
		if first != "" {
			out[it] = first
		}
	}
	return out
}

// eqgmStrings is the string table of an EQGM model: after the 28-byte header
// (magic, version, string table size, materials, vertices, triangles, bones)
func eqgmStrings(mod []byte) []string {
	if len(mod) < 28 || string(mod[:4]) != "EQGM" {
		return nil
	}
	size := int(binary.LittleEndian.Uint32(mod[8:]))
	if size <= 0 || 28+size > len(mod) {
		return nil
	}
	var out []string
	for _, b := range bytes.Split(mod[28:28+size], []byte{0}) {
		if len(b) > 0 {
			out = append(out, string(b))
		}
	}
	return out
}

// pfsFiles unpacks every file of a PFS archive, by lower-case name
func pfsFiles(data []byte) (map[string][]byte, error) {
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
	out := map[string][]byte{}
	for i, n := range names {
		if i >= len(entries) {
			break
		}
		b, err := pfsUnpack(data, entries[i].off, entries[i].size)
		if err != nil {
			return nil, fmt.Errorf("%v: %v", n, err)
		}
		out[strings.ToLower(n)] = b
	}
	return out, nil
}

// readArmorTexturesLocked reads which texture each of an armor archive's
// models wears, and draws a swatch of each
func (s *Store) readArmorTexturesLocked(a *Archive, data []byte) {
	a.ModelTextures = modelTextures(data, a.Models)
	drawn := map[string]bool{}
	models := filepath.Join(s.dirLocked(), "models")
	_ = os.MkdirAll(models, 0755)
	for _, t := range a.ModelTextures {
		if drawn[t] {
			continue
		}
		drawn[t] = true
		if png, err := textureSwatch(data, t); err == nil {
			_ = os.WriteFile(filepath.Join(models, textureSwatchFile(t)), png, 0644)
		}
	}
}
