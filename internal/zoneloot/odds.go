package zoneloot

import "math"

// Expected drops per kill, as the server rolls loot (zone/loot.cpp):
//   - each loot table row is rolled multiplier × Zone:GlobalLootMultiplier
//     times, and succeeds with its probability (100 always; 0 never; above
//     100 always)
//   - a drop group with no drop limit and no minimum rolls every item
//     against its chance, multiplier times
//   - otherwise it makes mindrop picks, then droplimit − mindrop more, each
//     happening with probability 1 − Π(1 − chance) over its items (always, if
//     any item's chance is 100 or more); a pick chooses an item weighted by
//     chance, and its multiplier − 1 extra copies each drop at its chance.
//     A group of more than 100 items with no drop limit gets a limit of 10,
//     and a limit below the minimum is raised to it.
// Items with NPC level limits count only for NPCs in range.

type TableEntry struct {
	LootdropID  int     `json:"lootdrop_id"`
	Multiplier  int     `json:"multiplier"`
	Droplimit   int     `json:"droplimit"`
	Mindrop     int     `json:"mindrop"`
	Probability float64 `json:"probability"`
}

type DropEntry struct {
	ItemID      int     `json:"item_id"`
	Chance      float64 `json:"chance"`
	Multiplier  int     `json:"multiplier"`
	Charges     int     `json:"item_charges"`
	EquipItem   int     `json:"equip_item"`
	NpcMinLevel int     `json:"npc_min_level"`
	NpcMaxLevel int     `json:"npc_max_level"`
}

func levelOK(e DropEntry, level int) bool {
	return !(e.NpcMinLevel > 0 && level < e.NpcMinLevel) && !(e.NpcMaxLevel > 0 && level > e.NpcMaxLevel)
}

// dropGroupExpected is the expected copies of each item one call to a drop
// group gives an NPC of this level
func dropGroupExpected(entries []DropEntry, droplimit, mindrop, level int) map[int]float64 {
	out := map[int]float64{}
	if droplimit == 0 && mindrop == 0 {
		for _, e := range entries {
			if levelOK(e, level) {
				out[e.ItemID] += float64(max(e.Multiplier, 0)) * clampChance(e.Chance)
			}
		}
		return out
	}
	if len(entries) > 100 && droplimit == 0 {
		droplimit = 10
	}
	if droplimit < mindrop {
		droplimit = mindrop
	}
	total, noLoot, bypass := 0.0, 1.0, false
	var active []DropEntry
	for _, e := range entries {
		if !levelOK(e, level) {
			continue
		}
		total += e.Chance
		if e.Chance >= 100 {
			bypass = true
		} else {
			noLoot *= (100 - e.Chance) / 100
		}
		active = append(active, e)
	}
	if len(active) == 0 || total <= 0 {
		return out
	}
	q := 1 - noLoot
	if bypass {
		q = 1
	}
	picks := float64(mindrop) + float64(droplimit-mindrop)*q
	for _, e := range active {
		copies := 1.0
		if e.Multiplier > 1 {
			copies += float64(e.Multiplier-1) * clampChance(e.Chance)
		}
		out[e.ItemID] += picks * (e.Chance / total) * copies
	}
	return out
}

// TableExpected is the expected copies of each item per kill of an NPC of
// this level with this loot table
func TableExpected(table []TableEntry, drops map[int][]DropEntry, level int, globalMultiplier int) map[int]float64 {
	out := map[int]float64{}
	for _, te := range table {
		rolls := float64(max(te.Multiplier, 0) * max(globalMultiplier, 0))
		p := tableChance(te.Probability)
		if rolls == 0 || p == 0 {
			continue
		}
		for item, n := range dropGroupExpected(drops[te.LootdropID], te.Droplimit, te.Mindrop, level) {
			out[item] += rolls * p * n
		}
	}
	return out
}

func tableChance(p float64) float64 {
	switch {
	case p == 0:
		return 0
	case p >= 100:
		return 1
	case p < 0:
		return 1 // outside 0-100 the server's roll is 0, which passes
	}
	return p / 100
}

func clampChance(c float64) float64 {
	return math.Max(0, math.Min(c, 100)) / 100
}
