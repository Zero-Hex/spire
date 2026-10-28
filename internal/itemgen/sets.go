package itemgen

import (
	"bufio"
	"bytes"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// A Heroes' Forge piece is shown by the item's herosforgemodel: the set
// number × 100 + the slot (0 helm, 1 chest, 2 arms, 3 wrists, 4 hands, 5 legs,
// 6 feet, 7 robe). The set's models are not named by it: the client's
// Resources/NewArmorTagData.txt lists, per set×100+slot, the IT models used,
// so the set an archive is for is read from there.

const (
	SlotRobe = 7
	// values under 1000 are shorthand to the server (×100 + slot), so a set
	// number under 10 cannot be given in full
	minSet = 10
)

// SetMatch is a Heroes' Forge set an archive's models belong to
type SetMatch struct {
	Set    int   `json:"set"`
	Slots  []int `json:"slots"`  // the slots with models in this archive
	Models int   `json:"models"` // how many of the archive's models it names
}

// ArmorTags is NewArmorTagData.txt: the set×100+slot ids naming each IT model
type ArmorTags map[int][]int

// ParseArmorTags reads NewArmorTagData.txt: tab separated rows of
// ID, race tag, IT number, IT number pair, ...; # starts a comment
func ParseArmorTags(data []byte) ArmorTags {
	tags := ArmorTags{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		id, err := strconv.Atoi(strings.TrimSpace(cols[0]))
		if err != nil || len(cols) < 3 {
			continue
		}
		for _, c := range cols[2:min(4, len(cols))] {
			if it, err := strconv.Atoi(strings.TrimSpace(c)); err == nil && it > 0 {
				tags[it] = append(tags[it], id)
			}
		}
	}
	return tags
}

// FindSets lists the sets the models belong to, the one naming most of them
// first
func FindSets(tags ArmorTags, models []int) []SetMatch {
	bySet := map[int]*SetMatch{}
	slots := map[int]map[int]bool{}
	for _, it := range models {
		counted := map[int]bool{}
		for _, id := range tags[it] {
			set, slot := id/100, id%100
			if set < minSet || slot > SlotRobe {
				continue
			}
			m := bySet[set]
			if m == nil {
				m = &SetMatch{Set: set}
				bySet[set] = m
				slots[set] = map[int]bool{}
			}
			if !counted[set] {
				counted[set] = true
				m.Models++
			}
			slots[set][slot] = true
		}
	}
	var out []SetMatch
	for set, m := range bySet {
		for s := range slots[set] {
			m.Slots = append(m.Slots, s)
		}
		sort.Ints(m.Slots)
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Models != out[j].Models {
			return out[i].Models > out[j].Models
		}
		return out[i].Set < out[j].Set
	})
	return out
}

var hfTexture = regexp.MustCompile(`(?i)^hfm0*(\d+)_`)

// SetFromTexture guesses the set from an archive's Heroes' Forge texture
// (hfm253_c.dds is set 253), or 0
func SetFromTexture(texture string) int {
	if m := hfTexture.FindStringSubmatch(texture); m != nil {
		n, _ := strconv.Atoi(m[1])
		if n >= minSet {
			return n
		}
	}
	return 0
}

// armor slots: the item's slots bits, the Heroes' Forge slot, a name
var armorSlots = []struct {
	bits  int
	slot  int
	label string
}{
	{1 << 2, 0, "Helm"},
	{1 << 17, 1, "Chestpiece"},
	{1 << 7, 2, "Sleeves"},
	{1<<9 | 1<<10, 3, "Bracer"},
	{1 << 12, 4, "Gloves"},
	{1 << 18, 5, "Leggings"},
	{1 << 19, 6, "Boots"},
}

// slotOf is the Heroes' Forge slot an item worn in these slots shows on, or -1
func slotOf(slots int) int {
	for _, s := range armorSlots {
		if slots&s.bits != 0 {
			return s.slot
		}
	}
	return -1
}

func slotLabel(slot int) string {
	if slot == SlotRobe {
		return "Robe"
	}
	for _, s := range armorSlots {
		if s.slot == slot {
			return s.label
		}
	}
	return ""
}
