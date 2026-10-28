package clientassets

import (
	"encoding/binary"
	"strings"
	"testing"
)

// an EQGM model whose materials name these textures
func eqgm(textures ...string) []byte {
	table := "Material#1\x00Opaque_MaxCB1.fx\x00e_TextureDiffuse0\x00" + strings.Join(textures, "\x00") + "\x00"
	out := make([]byte, 28)
	copy(out, "EQGM")
	binary.LittleEndian.PutUint32(out[4:], 3)
	binary.LittleEndian.PutUint32(out[8:], uint32(len(table)))
	return append(append(out, table...), make([]byte, 16)...)
}

const setTags = "#ID\tTag\tIT Number\tIT Number Pair\r\n" +
	"2000\tL\t14100\t\t\t\t# Helm\r\n" +
	"26000\tL\t70000\t\t\t\t\r\n" +
	"26001\tL\t70050\t70051\t\t\t\r\n" +
	"26005\tL\t70099\t\t\t\t\r\n" +
	"26000\tHUM\t70000\t\t\t\t\r\n"

func TestTagFileRows(t *testing.T) {
	tags := ParseArmorTags([]byte(setTags + "29400-29449\tL\t1\t\t\r\n"))
	if got := tags.models[70051]; len(got) != 1 || got[0] != 26001 {
		t.Errorf("pair: %v", got)
	}
	if len(tags.sets[260]) != 4 || tags.models[1] != nil {
		t.Errorf("sets %v", tags.sets)
	}
	sets := FindSets(tags, []int{70000, 70050, 99999})
	if len(sets) != 1 || sets[0].Set != 260 || sets[0].Models != 2 || len(sets[0].Slots) != 2 {
		t.Fatalf("%+v", sets)
	}
	if SetFromTexture("hfm253_c.dds") != 253 || SetFromTexture("HFM040_C.DDS") != 40 || SetFromTexture("hfm5_c.dds") != 0 || SetFromTexture("it1.dds") != 0 {
		t.Error("texture")
	}
	for bits, want := range map[int]int{4: 0, 131072: 1, 128: 2, 1536: 3, 512: 3, 4096: 4, 262144: 5, 524288: 6, 8192: -1, 0: -1} {
		if got := SlotOf(bits); got != want {
			t.Errorf("%d: %d, want %d", bits, got, want)
		}
	}
}

// set 260 is spread over two archives; the first also holds models no tag
// row names, wearing another texture: shown apart, as set 261 by its name
func TestArmorSetsAcrossArchives(t *testing.T) {
	s := testStore(t)
	a := buildPFSWith(t, []string{"it70000.mod", "it70001.mod", "hfm260_c.dds", "hfm261_c.dds"}, map[string][]byte{
		"it70000.mod": eqgm("cloth_armor.dds", "hfm260_c.dds", "hfm260_n.dds"),
		"it70001.mod": eqgm("hfm261_c.dds"),
	})
	b := buildPFSWith(t, []string{"it70050.mod", "it70051.mod", "hfm260_c.dds"}, map[string][]byte{
		"it70050.mod": eqgm("hfm260_c.dds"),
		"it70051.mod": eqgm("hfm260_c.dds"),
	})
	for name, data := range map[string][]byte{"ArmorA.eqg": a, "ArmorB.eqg": b} {
		if _, err := s.Import(name, data); err != nil {
			t.Fatal(err)
		}
	}
	if got := s.Index().Archives["armora.eqg"].ModelTextures; got[70000] != "hfm260_c.dds" || got[70001] != "hfm261_c.dds" {
		t.Fatalf("textures %v", got)
	}
	_ = s.SetMode("ArmorB.eqg", ModeBeta)

	// with no tag file, each archive's models are grouped by texture
	if sets := s.ArmorSets(); len(sets) != 3 || sets[0].Set != 0 || sets[0].Guess != 260 {
		t.Fatalf("untagged %+v", sets)
	}

	if _, err := s.ImportAt("Resources", "NewArmorTagData.txt", []byte(setTags)); err != nil {
		t.Fatal(err)
	}
	sets := s.ArmorSets()
	if len(sets) != 2 {
		t.Fatalf("%+v", sets)
	}
	set := sets[0]
	if set.Set != 260 || strings.Join(itoas(set.Models), ",") != "70000,70050,70051" || strings.Join(itoas(set.Slots), ",") != "0,1" ||
		set.Texture != "hfm260_c.dds" || len(set.Archives) != 2 || !set.Archives[1].Beta || strings.Join(itoas(set.Missing), ",") != "70099" {
		t.Errorf("%+v", set)
	}
	other := sets[1]
	if other.Set != 0 || other.Guess != 261 || len(other.Models) != 1 || other.Archives[0].File != "ArmorA.eqg" || len(other.Slots) != 0 {
		t.Errorf("%+v", other)
	}
}

func itoas(n []int) []string {
	var out []string
	for _, i := range n {
		out = append(out, itoa(i))
	}
	return out
}
