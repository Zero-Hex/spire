package zoneloot

import "testing"

var defaultRules = ScaleRules{
	LevelEnabled: true, TrashOffset: 0, NamedOffset: 2, RaidOffset: 3, MaxRaise: 10,
	ChallengeEnabled: true, TrashHP: 1.25, TrashDamage: 1.15, TrashAC: 1.10, BossHP: 1.75, BossDamage: 1.40, BossAC: 1.25,
}

// the cases tools/plugin_tests/npc_challenge_scaling_test.pl in starfall-3
// runs the plugin through, with the same curve rows
func TestScaleMatchesPlugin(t *testing.T) {
	cv := curve{
		"0:24:0:0": {845, 103, 11, 55, 0, 0}, "0:30:0:0": {1350, 127, 12, 68, 0, 0}, "0:34:0:0": {1750, 158, 20, 90, 16, 16},
		"1:24:0:0": {1001, 124, 13, 66, 0, 0}, "1:34:0:0": {3264, 190, 24, 108, 19, 19},
	}
	at := func(typ int) func(int) (curveRow, bool) {
		return func(l int) (curveRow, bool) { return cv.at(typ, l, 101, 0) }
	}
	for _, c := range []struct {
		what      string
		o         ScaleStats
		typ       int
		boss      bool
		reference int
		want      ScaleStats
	}{
		{"kobold at 30", ScaleStats{24, 900, 110, 12, 60, 5, 0}, 0, false, 30, ScaleStats{30, 1796, 148, 14, 85, 5, 0}},
		{"old kobold stays 36", ScaleStats{36, 2000, 150, 20, 100, 0, 0}, 0, false, 30, ScaleStats{36, 2500, 165, 23, 114, 0, 0}},
		{"raid target: challenge only", ScaleStats{24, 5000, 200, 30, 120, 0, 0}, 2, true, 30, ScaleStats{24, 8750, 250, 42, 168, 0, 0}},
		{"kobold at 34", ScaleStats{24, 900, 110, 12, 60, 5, 0}, 0, false, 34, ScaleStats{34, 2328, 184, 24, 112, 21, 16}},
		{"king to 34", ScaleStats{24, 1000, 130, 15, 70, 0, 0}, 1, false, 34, ScaleStats{34, 4075, 218, 31, 131, 19, 19}},
	} {
		if got := defaultRules.scale(c.o, c.typ, c.boss, at(c.typ), c.reference); got != c.want {
			t.Errorf("%v: %+v, want %+v", c.what, got, c.want)
		}
	}
	// nobody in the zone, or level scaling off: challenge only
	off := defaultRules
	off.LevelEnabled = false
	if got := off.scale(ScaleStats{24, 900, 110, 12, 60, 5, 0}, 0, false, at(0), 30); got.Level != 24 || got.HP != 1125 {
		t.Errorf("level scaling off: %+v", got)
	}
	if got := defaultRules.scale(ScaleStats{24, 900, 110, 12, 60, 5, 0}, 0, false, at(0), 0); got.Level != 24 {
		t.Errorf("no players: %+v", got)
	}
	lower := defaultRules
	lower.Lower = true
	if got := lower.targetLevel(0, 36, 30); got != 30 {
		t.Errorf("lowering: %v", got)
	}
	if got := defaultRules.targetLevel(0, 10, 50); got != 20 {
		t.Errorf("max raise: %v", got)
	}
}

func TestClassLevelDamageMod(t *testing.T) {
	for _, c := range []struct{ level, class, want int }{{19, 1, 220}, {75, 1, 311}, {30, 2, 150}, {50, 5, 230}, {57, 9, 190}, {65, 4, 210}, {80, 12, 127}, {40, 99, 220}} {
		if got := classLevelDamageMod(c.level, c.class); got != c.want {
			t.Errorf("level %d class %d: %d, want %d", c.level, c.class, got, c.want)
		}
	}
}

func TestPreviewScaling(t *testing.T) {
	db := testDB(t)
	p, err := PreviewScaling(db, "warrens", 0, 30, &defaultRules)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Curve || len(p.NPCs) < 100 {
		t.Fatalf("curve %v, %d NPCs", p.Curve, len(p.NPCs))
	}
	raised, withDrops := 0, 0
	for _, n := range p.NPCs {
		if n.Spawn.HP <= 0 {
			t.Errorf("NPC %d spawns with no HP: %+v", n.ID, n.Spawn)
		}
		if n.Scaled.Level > n.Spawn.Level {
			raised++
			if n.Scaled.HP <= n.Spawn.HP || n.Scaled.Level > n.Spawn.Level+10 {
				t.Errorf("NPC %d: %+v -> %+v", n.ID, n.Spawn, n.Scaled)
			}
			if n.Drops != nil {
				withDrops++
			}
		}
	}
	king := p.NPCs[101109]
	t.Logf("King Gragnar %+v -> %+v; %d of %d NPCs raised", king.Spawn, king.Scaled, raised, len(p.NPCs))
	if king.Type != 1 || king.Scaled.Level != 32 {
		t.Errorf("King Gragnar: %+v", king)
	}
	if raised == 0 || withDrops == 0 {
		t.Errorf("raised %d, with drops %d", raised, withDrops)
	}
	srv, err := PreviewScaling(db, "warrens", 0, 30, nil)
	if err != nil {
		t.Fatal(err)
	}
	if srv.Rules.LevelEnabled || !srv.Rules.ChallengeEnabled || srv.Rules.TrashHP != 1.25 {
		t.Errorf("server rules (defaults here): %+v", srv.Rules)
	}
}
