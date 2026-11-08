package zoneloot

import (
	"fmt"
	"strconv"
	"strings"

	"gorm.io/gorm"
)

// The Starfall zone scaling plugin (starfall-3 plugins/npc_challenge_scaling.pl),
// worked out for an entered player level, so a zone can be checked before
// the plugin runs on it. Change the math in both places; the tests here and
// tools/plugin_tests/npc_challenge_scaling_test.pl there check the same cases.
//
// An NPC's stats as it spawns are its npc_types row, with the zeros filled in
// from npc_scale_global_base the way the engine's NpcScaleManager does. Level
// scaling moves it to the players' level plus its type's offset, scaling HP,
// AC and damage by the ratio of that curve between the two levels and attack
// and accuracy by its difference; challenge scaling multiplies on top.

type ScaleRules struct {
	LevelEnabled bool `json:"level_enabled"`
	TrashOffset  int  `json:"trash_offset"`
	NamedOffset  int  `json:"named_offset"`
	RaidOffset   int  `json:"raid_offset"`
	RaidTargets  bool `json:"raid_targets"`
	MaxRaise     int  `json:"max_raise"`
	Lower        bool `json:"lower"`

	ChallengeEnabled bool    `json:"challenge_enabled"`
	TrashHP          float64 `json:"trash_hp"`
	TrashDamage      float64 `json:"trash_damage"`
	TrashAC          float64 `json:"trash_ac"`
	BossHP           float64 `json:"boss_hp"`
	BossDamage       float64 `json:"boss_damage"`
	BossAC           float64 `json:"boss_ac"`

	ClassLevelDamageMods bool `json:"class_level_damage_mods"` // Combat:UseNPCDamageClassLevelMods
}

// the server's rule names, and the defaults its ruletypes.h gives them
var scaleRuleNames = map[string]string{
	"Custom:NPC_Level_Scaling_Enabled":                     "false",
	"Custom:NPC_Level_Scaling_Trash_Offset":                "0",
	"Custom:NPC_Level_Scaling_Named_Offset":                "2",
	"Custom:NPC_Level_Scaling_Raid_Offset":                 "3",
	"Custom:NPC_Level_Scaling_Raid_Targets":                "false",
	"Custom:NPC_Level_Scaling_Max_Raise":                   "10",
	"Custom:NPC_Level_Scaling_Lower":                       "false",
	"Custom:NPC_Challenge_Scaling_Enabled":                 "true",
	"Custom:NPC_Challenge_Scaling_Trash_HP_Multiplier":     "1.25",
	"Custom:NPC_Challenge_Scaling_Trash_Damage_Multiplier": "1.15",
	"Custom:NPC_Challenge_Scaling_Trash_AC_Multiplier":     "1.10",
	"Custom:NPC_Challenge_Scaling_Boss_HP_Multiplier":      "1.75",
	"Custom:NPC_Challenge_Scaling_Boss_Damage_Multiplier":  "1.40",
	"Custom:NPC_Challenge_Scaling_Boss_AC_Multiplier":      "1.25",
	"Combat:UseNPCDamageClassLevelMods":                    "true",
}

// ServerScaleRules reads the rules from rule_values (ruleset 1), and says
// which were there; the rest are their defaults
func ServerScaleRules(db *gorm.DB) (ScaleRules, []string) {
	vals := map[string]string{}
	for k, v := range scaleRuleNames {
		vals[k] = v
	}
	set := []string{}
	if hasTable(db, "rule_values") {
		var rows []struct{ RuleName, RuleValue string }
		names := make([]string, 0, len(scaleRuleNames))
		for k := range scaleRuleNames {
			names = append(names, k)
		}
		db.Raw("SELECT rule_name, rule_value FROM rule_values WHERE ruleset_id = 1 AND rule_name IN ?", names).Scan(&rows)
		for _, r := range rows {
			vals[r.RuleName] = r.RuleValue
			set = append(set, r.RuleName)
		}
	}
	b := func(k string) bool { return strings.EqualFold(strings.TrimSpace(vals[k]), "true") }
	i := func(k string) int { n, _ := strconv.Atoi(strings.TrimSpace(vals[k])); return n }
	f := func(k string) float64 { n, _ := strconv.ParseFloat(strings.TrimSpace(vals[k]), 64); return n }
	return ScaleRules{
		LevelEnabled: b("Custom:NPC_Level_Scaling_Enabled"), TrashOffset: i("Custom:NPC_Level_Scaling_Trash_Offset"),
		NamedOffset: i("Custom:NPC_Level_Scaling_Named_Offset"), RaidOffset: i("Custom:NPC_Level_Scaling_Raid_Offset"),
		RaidTargets: b("Custom:NPC_Level_Scaling_Raid_Targets"), MaxRaise: i("Custom:NPC_Level_Scaling_Max_Raise"),
		Lower: b("Custom:NPC_Level_Scaling_Lower"), ChallengeEnabled: b("Custom:NPC_Challenge_Scaling_Enabled"),
		TrashHP: f("Custom:NPC_Challenge_Scaling_Trash_HP_Multiplier"), TrashDamage: f("Custom:NPC_Challenge_Scaling_Trash_Damage_Multiplier"),
		TrashAC: f("Custom:NPC_Challenge_Scaling_Trash_AC_Multiplier"), BossHP: f("Custom:NPC_Challenge_Scaling_Boss_HP_Multiplier"),
		BossDamage: f("Custom:NPC_Challenge_Scaling_Boss_Damage_Multiplier"), BossAC: f("Custom:NPC_Challenge_Scaling_Boss_AC_Multiplier"),
		ClassLevelDamageMods: b("Combat:UseNPCDamageClassLevelMods"),
	}, set
}

type ScaleStats struct {
	Level    int `json:"level"`
	HP       int `json:"hp"`
	AC       int `json:"ac"`
	MinDmg   int `json:"min_dmg"`
	MaxDmg   int `json:"max_dmg"`
	Attack   int `json:"attack"`
	Accuracy int `json:"accuracy"`
}

type curveRow struct {
	HP, AC, MinDmg, MaxDmg, Attack, Accuracy int
}

// curve is npc_scale_global_base, looked up as the engine does: the zone and
// version's row, else the zone's version 0, else the global one
type curve map[string]curveRow

func loadCurve(db *gorm.DB) (curve, error) {
	var rows []struct {
		Type, Level                              int
		HP, AC, MinDmg, MaxDmg, Attack, Accuracy int
		ZoneIDList, InstanceVersionList          string
	}
	if !hasTable(db, "npc_scale_global_base") {
		return curve{}, nil
	}
	if err := db.Raw(`SELECT type, level, COALESCE(hp, 0) AS hp, COALESCE(ac, 0) AS ac, COALESCE(min_dmg, 0) AS min_dmg, COALESCE(max_dmg, 0) AS max_dmg,
		COALESCE(attack, 0) AS attack, COALESCE(accuracy, 0) AS accuracy, COALESCE(zone_id_list, '') AS zone_id_list,
		COALESCE(instance_version_list, '') AS instance_version_list FROM npc_scale_global_base`).Scan(&rows).Error; err != nil {
		return nil, err
	}
	c := curve{}
	split := func(s string) []string {
		var out []string
		for _, p := range strings.Split(s, "|") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		if len(out) == 0 {
			out = []string{"0"}
		}
		return out
	}
	for _, r := range rows {
		for _, z := range split(r.ZoneIDList) {
			for _, v := range split(r.InstanceVersionList) {
				key := fmt.Sprintf("%d:%d:%s:%s", r.Type, r.Level, z, v)
				if _, ok := c[key]; !ok {
					c[key] = curveRow{r.HP, r.AC, r.MinDmg, r.MaxDmg, r.Attack, r.Accuracy}
				}
			}
		}
	}
	return c, nil
}

func (c curve) at(typ, level, zoneID, version int) (curveRow, bool) {
	for _, k := range []string{fmt.Sprintf("%d:%d:%d:%d", typ, level, zoneID, version), fmt.Sprintf("%d:%d:%d:0", typ, level, zoneID), fmt.Sprintf("%d:%d:0:0", typ, level)} {
		if r, ok := c[k]; ok {
			return r, true
		}
	}
	return curveRow{}, false
}

// scaleType is the engine's: 2 raid, 1 named, 0 trash
func scaleType(name string, rare, raid bool) int {
	switch {
	case raid:
		return 2
	case rare || strings.Contains(name, "#") || (name != "" && name[0] >= 'A' && name[0] <= 'Z'):
		return 1
	}
	return 0
}

// classLevelDamageMod is NpcScaleManager::GetClassLevelDamageMod
func classLevelDamageMod(level, class int) int {
	steps := func(levels []int, mods []int) int {
		for i, l := range levels {
			if level < l {
				return mods[i]
			}
		}
		return mods[len(mods)-1]
	}
	switch class {
	case 1: // warrior
		return steps([]int{20, 30, 40, 53, 57, 60, 70}, []int{220, 230, 250, 270, 280, 290, 300, 311})
	case 6, 2, 10: // druid, cleric, shaman
		return steps([]int{70}, []int{150, 157})
	case 16, 3, 5: // berserker, paladin, shadow knight
		return steps([]int{35, 45, 51, 56, 60, 68}, []int{210, 220, 230, 240, 250, 260, 270})
	case 7, 8, 9, 15: // monk, bard, rogue, beastlord
		return steps([]int{51, 58, 70}, []int{180, 190, 200, 210})
	case 4: // ranger
		return steps([]int{58, 70}, []int{200, 210, 220})
	case 13, 12, 11, 14: // magician, wizard, necromancer, enchanter
		return steps([]int{70}, []int{120, 127})
	}
	return steps([]int{35, 45, 51, 56, 60}, []int{210, 220, 230, 240, 250, 260})
}

// targetLevel is sf_scale_target_level
func (r ScaleRules) targetLevel(typ, level, reference int) int {
	if reference <= 0 || (typ == 2 && !r.RaidTargets) {
		return level
	}
	target := reference + []int{r.TrashOffset, r.NamedOffset, r.RaidOffset}[typ]
	if target < level && !r.Lower {
		target = level
	}
	if r.MaxRaise > 0 && target > level+r.MaxRaise {
		target = level + r.MaxRaise
	}
	return min(max(target, 1), 255)
}

// scale is npc_zone_scale on an NPC's spawn stats
func (r ScaleRules) scale(o ScaleStats, typ int, boss bool, cv func(level int) (curveRow, bool), reference int) ScaleStats {
	s := o
	if r.LevelEnabled {
		if target := r.targetLevel(typ, o.Level, reference); target != o.Level {
			from, ok1 := cv(o.Level)
			to, ok2 := cv(target)
			if ok1 && ok2 {
				ratio := func(orig, f, t int) int {
					if f > 0 {
						return int(float64(orig) * float64(t) / float64(f))
					}
					return max(orig, t)
				}
				s.HP = ratio(o.HP, from.HP, to.HP)
				s.AC = ratio(o.AC, from.AC, to.AC)
				s.MinDmg = ratio(o.MinDmg, from.MinDmg, to.MinDmg)
				s.MaxDmg = ratio(o.MaxDmg, from.MaxDmg, to.MaxDmg)
				s.Attack = max(o.Attack+to.Attack-from.Attack, 0)
				s.Accuracy = max(o.Accuracy+to.Accuracy-from.Accuracy, 0)
				s.Level = target
			}
		}
	}
	if r.ChallengeEnabled {
		hp, dmg, ac := r.TrashHP, r.TrashDamage, r.TrashAC
		if boss {
			hp, dmg, ac = r.BossHP, r.BossDamage, r.BossAC
		}
		s.HP = int(float64(s.HP) * hp)
		s.AC = int(float64(s.AC) * ac)
		s.MinDmg = int(float64(s.MinDmg) * dmg)
		s.MaxDmg = int(float64(s.MaxDmg) * dmg)
	}
	return s
}

type NPCScale struct {
	ID     int        `json:"id"`
	Type   int        `json:"type"` // 0 trash, 1 named, 2 raid
	Boss   bool       `json:"boss"` // '#': challenge scaling's boss multipliers
	Spawn  ScaleStats `json:"spawn"`
	Scaled ScaleStats `json:"scaled"`
	Drops  []Drop     `json:"drops"` // per kill at the scaled level, when it changes
}

type ScalePreview struct {
	PlayerLevel int              `json:"player_level"`
	Rules       ScaleRules       `json:"rules"`
	ServerRules ScaleRules       `json:"server_rules"`
	RulesSet    []string         `json:"rules_set"` // rules rule_values has; the rest are defaults
	Curve       bool             `json:"curve"`     // npc_scale_global_base has rows
	NPCs        map[int]NPCScale `json:"npcs"`
}

// PreviewScaling works the plugin out for a zone version's NPCs with a
// player of a level in the zone: with the server's rules, or others
func PreviewScaling(db *gorm.DB, zone string, version, playerLevel int, rules *ScaleRules) (*ScalePreview, error) {
	if playerLevel < 0 || playerLevel > 255 {
		return nil, fmt.Errorf("player level must be from 1 to 255")
	}
	z, err := Load(db, zone, version)
	if err != nil {
		return nil, err
	}
	server, set := ServerScaleRules(db)
	out := &ScalePreview{PlayerLevel: playerLevel, Rules: server, ServerRules: server, RulesSet: set, NPCs: map[int]NPCScale{}}
	if rules != nil {
		out.Rules = *rules
	}
	cv, err := loadCurve(db)
	if err != nil {
		return nil, err
	}
	out.Curve = len(cv) > 0
	var zoneID int
	db.Raw("SELECT zoneidnumber FROM zone WHERE short_name = ? LIMIT 1", zone).Scan(&zoneID)

	var ids []int
	for _, n := range z.NPCs {
		ids = append(ids, n.ID)
	}
	skipCol := ""
	if hasColumn(db, "npc_types", "skip_auto_scale") {
		skipCol = ", skip_auto_scale"
	}
	var rows []map[string]any
	if err := db.Raw(`SELECT id, name, level, class, hp, AC AS ac, mindmg, maxdmg, ATK AS atk, Accuracy AS accuracy, rare_spawn, raid_target,
		(STR = 0 AND STA = 0 AND DEX = 0 AND AGI = 0 AND _INT = 0 AND WIS = 0 AND CHA = 0 AND MR = 0 AND FR = 0 AND CR = 0 AND PR = 0 AND DR = 0) AS no_stats`+skipCol+`
		FROM npc_types WHERE id IN ?`, nonEmpty(ids)).Scan(&rows).Error; err != nil {
		return nil, err
	}
	text := func(v any) string {
		if b, ok := v.([]byte); ok {
			return string(b)
		}
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
	num := func(v any) int {
		n, _ := strconv.ParseFloat(text(v), 64)
		return int(n)
	}
	byID := map[int]*NPC{}
	for i := range z.NPCs {
		byID[z.NPCs[i].ID] = &z.NPCs[i]
	}
	for _, row := range rows {
		id, level, class := num(row["id"]), num(row["level"]), num(row["class"])
		name := text(row["name"])
		typ := scaleType(name, num(row["rare_spawn"]) != 0, num(row["raid_target"]) != 0)
		at := func(l int) (curveRow, bool) { return cv.at(typ, l, zoneID, version) }
		o := ScaleStats{Level: level, HP: num(row["hp"]), AC: num(row["ac"]), MinDmg: num(row["mindmg"]), MaxDmg: num(row["maxdmg"]),
			Attack: num(row["atk"]), Accuracy: num(row["accuracy"])}
		// the zeros the engine fills in at spawn (NpcScaleManager::ScaleNPC, always_scale off)
		if base, ok := at(level); ok && num(row["skip_auto_scale"]) == 0 {
			autoScaled := o.HP == 0 && o.MinDmg == 0 && o.MaxDmg == 0 && num(row["no_stats"]) != 0
			if o.AC == 0 && autoScaled {
				o.AC = base.AC
			}
			if o.HP == 0 {
				o.HP = base.HP
			}
			if o.Accuracy == 0 {
				o.Accuracy = base.Accuracy
			}
			if o.Attack == 0 {
				o.Attack = base.Attack
			}
			if o.MinDmg == 0 || o.MaxDmg == 0 {
				o.MinDmg, o.MaxDmg = base.MinDmg, base.MaxDmg
				if out.Rules.ClassLevelDamageMods {
					mod := classLevelDamageMod(level, class)
					o.MinDmg, o.MaxDmg = o.MinDmg*mod/220, o.MaxDmg*mod/220
				}
			}
		}
		ns := NPCScale{ID: id, Type: typ, Boss: strings.Contains(name, "#"), Spawn: o}
		ns.Scaled = out.Rules.scale(o, typ, ns.Boss, at, playerLevel)
		if n := byID[id]; n != nil && ns.Scaled.Level != o.Level {
			ns.Drops = []Drop{}
			if t := z.Tables[n.LoottableID]; t != nil {
				drops := map[int][]DropEntry{}
				for gid, g := range z.Dropgroups {
					drops[gid] = g.Entries
				}
				for item, e := range TableExpected(t.Entries, drops, ns.Scaled.Level, z.GlobalMultiplier) {
					if e > 0 {
						ns.Drops = append(ns.Drops, Drop{ItemID: item, Expected: e})
					}
				}
				sortDrops(ns.Drops)
			}
		}
		out.NPCs[id] = ns
	}
	return out, nil
}

func sortDrops(d []Drop) {
	for i := 1; i < len(d); i++ {
		for j := i; j > 0 && d[j].Expected > d[j-1].Expected; j-- {
			d[j], d[j-1] = d[j-1], d[j]
		}
	}
}

func hasColumn(db *gorm.DB, table, column string) bool {
	var n int64
	db.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?", table, column).Scan(&n)
	return n > 0
}
