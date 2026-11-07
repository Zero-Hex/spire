package zoneloot

import (
	"math"
	"math/rand"
	"testing"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// the expected counts match a simulation of the server's own rolls
func TestExpectedMatchesSimulation(t *testing.T) {
	drops := map[int][]DropEntry{
		1: {{ItemID: 10, Chance: 30, Multiplier: 1}, {ItemID: 11, Chance: 10, Multiplier: 2}},                                         // rolled freely
		2: {{ItemID: 20, Chance: 25, Multiplier: 1}, {ItemID: 21, Chance: 5, Multiplier: 1}, {ItemID: 22, Chance: 70, Multiplier: 3}}, // limited
		3: {{ItemID: 30, Chance: 100, Multiplier: 1}, {ItemID: 31, Chance: 50, Multiplier: 1, NpcMinLevel: 30}},
	}
	table := []TableEntry{
		{LootdropID: 1, Multiplier: 2, Probability: 60},
		{LootdropID: 2, Multiplier: 1, Droplimit: 2, Mindrop: 1, Probability: 100},
		{LootdropID: 3, Multiplier: 1, Droplimit: 1, Probability: 40},
	}
	got := TableExpected(table, drops, 20, 1)
	sim := simulate(table, drops, 20, 400000)
	for item := range sim {
		if !near(got[item], sim[item], 0.01) {
			t.Errorf("item %d: expected %.4f, simulated %.4f", item, got[item], sim[item])
		}
	}
	if got[31] != 0 {
		t.Errorf("an item for level 30+ dropped from a level 20 NPC: %v", got[31])
	}
}

// a port of NPC::AddLootTable / AddLootDropTable's rolls
func simulate(table []TableEntry, drops map[int][]DropEntry, level, kills int) map[int]float64 {
	r := rand.New(rand.NewSource(1))
	count := map[int]float64{}
	for k := 0; k < kills; k++ {
		for _, te := range table {
			for i := 0; i < te.Multiplier; i++ {
				roll := 0.0
				if te.Probability >= 0 && te.Probability <= 100 {
					roll = r.Float64() * 100
				}
				if te.Probability == 0 || !(te.Probability == 100 || roll <= te.Probability) {
					continue
				}
				le := drops[te.LootdropID]
				dl, md := te.Droplimit, te.Mindrop
				if dl == 0 && md == 0 {
					for _, e := range le {
						for j := 0; j < e.Multiplier; j++ {
							if r.Float64()*100 <= e.Chance && levelOK(e, level) {
								count[e.ItemID]++
							}
						}
					}
					continue
				}
				if dl < md {
					dl = md
				}
				total, noLoot, bypass := 0.0, 1.0, false
				for _, e := range le {
					if levelOK(e, level) {
						total += e.Chance
						if e.Chance >= 100 {
							bypass = true
						} else {
							noLoot *= (100 - e.Chance) / 100
						}
					}
				}
				got := 0
				for d := 0; d < dl; d++ {
					if got < md || bypass || r.Float64() >= noLoot {
						roll := r.Float64() * total
						for _, e := range le {
							if !levelOK(e, level) {
								continue
							}
							if roll < e.Chance {
								count[e.ItemID]++
								got++
								for c := 1; c < max(e.Multiplier, 1); c++ {
									if r.Float64()*100 <= e.Chance {
										count[e.ItemID]++
									}
								}
								break
							}
							roll -= e.Chance
						}
					}
				}
			}
		}
	}
	for k := range count {
		count[k] /= float64(kills)
	}
	return count
}
