package zoneloot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/EQEmu/spire/internal/clientassets"
	"github.com/EQEmu/spire/internal/http/staticmaps"
	"gorm.io/gorm"
)

// A zone's <zone>_chr.txt lists the race models it loads beyond what every
// zone loads (GlobalLoad.txt, GlobalLoad_chr.txt) and what the zone's own
// <zone>_chr.s3d holds (the client loads that by itself: no stock file lists
// it). The plan works out the models the zone's NPCs need, in every version
// (one _chr.txt serves them all) and including NPCs its quests spawn, and
// suggests lines to add and lines nothing uses. Lines it can't place (an
// animation set like T05, a code no race has) are kept.

type raceInfo struct {
	RaceID      int    `json:"race_id"`
	Description string `json:"description"`
	Male        string `json:"male_gender_model_code"`
	Female      string `json:"female_gender_model_code"`
	Neutral     string `json:"neutral_gender_model_code"`
	Sources     []struct {
		File     string `json:"source_file"`
		IsGlobal bool   `json:"is_global"`
		Models   []struct {
			Code   string `json:"code"`
			Gender int    `json:"gender"`
		} `json:"models"`
	} `json:"sources"`
}

type rof2Scan struct {
	Codes    map[string][]string `json:"codes"`
	Archives map[string]struct {
		Models     []string `json:"models"`
		Animations []string `json:"animations"`
	} `json:"archives"`
	Files []string `json:"files"`
}

var (
	raceDataOnce sync.Once
	races        map[int]*raceInfo
	raceCodes    map[string]bool  // every code a race model has
	codeRaces    map[string][]int // the races a model code is for
	rof2         rof2Scan
	rof2Files    map[string]bool
)

func loadRaceData() {
	raceDataOnce.Do(func() {
		var rm struct {
			Races []*raceInfo `json:"races"`
		}
		_ = json.Unmarshal(staticmaps.RaceInventoryMap(), &rm)
		races, raceCodes, codeRaces = map[int]*raceInfo{}, map[string]bool{}, map[string][]int{}
		add := func(code string, race int) {
			c := strings.ToUpper(code)
			raceCodes[c] = true
			if !containsInt(codeRaces[c], race) {
				codeRaces[c] = append(codeRaces[c], race)
			}
		}
		for _, r := range rm.Races {
			races[r.RaceID] = r
			for _, c := range []string{r.Male, r.Female, r.Neutral} {
				if c != "" {
					add(c, r.RaceID)
				}
			}
			for _, s := range r.Sources {
				for _, m := range s.Models {
					add(m.Code, r.RaceID)
				}
			}
		}
		_ = json.Unmarshal(staticmaps.Rof2RaceArchives(), &rof2)
		rof2Files = map[string]bool{}
		for _, f := range rof2.Files {
			rof2Files[strings.ToLower(f)] = true
		}
	})
}

// modelCode is the race model an NPC of a race and gender shows
func modelCode(race, gender int) string {
	r := races[race]
	if r == nil {
		return ""
	}
	order := map[int][]string{0: {r.Male, r.Neutral, r.Female}, 1: {r.Female, r.Male, r.Neutral}, 2: {r.Neutral, r.Male, r.Female}}[gender]
	if order == nil {
		order = []string{r.Male, r.Female, r.Neutral}
	}
	for _, c := range order {
		if c != "" {
			return strings.ToUpper(c)
		}
	}
	for _, s := range r.Sources {
		for _, m := range s.Models {
			if m.Gender == gender {
				return strings.ToUpper(m.Code)
			}
		}
	}
	return ""
}

// archiveModels is the models in a client archive, by its name with or
// without extension
func archiveModels(name string) []string {
	n := strings.ToLower(name)
	for _, f := range []string{n, n + ".s3d", n + ".eqg"} {
		if a, ok := rof2.Archives[f]; ok {
			return a.Models
		}
	}
	return nil
}

func baseName(file string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.ToLower(file), ".s3d"), ".eqg")
}

type NPCRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type ModelNeed struct {
	Code     string   `json:"code"`
	Race     int      `json:"race"`
	RaceName string   `json:"race_name"`
	NPCs     []NPCRef `json:"npcs"`
	// where the zone gets it: global, zone (its own _chr.s3d), line (the
	// file has a line for it), add (a line is suggested), missing (no
	// archive Spire knows holds it)
	From    string `json:"from"`
	Archive string `json:"archive,omitempty"` // the line's, or the suggested one
}

type ModelLine struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Archive string `json:"archive"`
	// needed, keep (an animation set, or a code no race has), variant (a
	// model of a race by the same name as one NPCs here are: the client shows
	// newer models in place of older ones where they're loaded), unused (no
	// NPC here uses it), global (every zone loads it already), zone (the
	// zone's own archive holds it)
	Status string `json:"status"`
	Remove bool   `json:"remove"` // suggested
	Why    string `json:"why"`
}

type ModelPlan struct {
	Path        string         `json:"path"`
	Held        bool           `json:"held"`         // Spire keeps a copy for the patch
	Mode        string         `json:"mode"`         // its patch mode: "" (sent), beta, ignore
	Stock       bool           `json:"stock"`        // the stock client ships one
	NeedsUpload bool           `json:"needs_upload"` // stock, not held: upload the client's copy first
	Lines       []ModelLine    `json:"lines"`
	Needs       []ModelNeed    `json:"needs"`
	Adds        []ModelNeed    `json:"adds"`
	NoModel     []NPCRef       `json:"no_model"`   // races without a known model
	QuestNPCs   []NPCRef       `json:"quest_npcs"` // spawned by the zone's quests
	Versions    []int          `json:"versions"`
	Global      int            `json:"global"` // models every zone loads, by Spire's reckoning
	Updated     int64          `json:"updated"`
	Archives    map[string]int `json:"archives"` // models in each archive a line or suggestion names
}

var questSpawn = regexp.MustCompile(`(?:quest::|eq\.)(?:spawn2|spawn|unique_spawn|spawn_from_spawn2|spawn_condition)\s*\(\s*(\d{2,})`)

// questNPCs is the NPC ids a zone's quest scripts spawn
func questNPCs(questsDir, zone string) []int {
	if questsDir == "" {
		return nil
	}
	seen := map[int]bool{}
	var out []int
	_ = filepath.WalkDir(filepath.Join(questsDir, zone), func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if ext := strings.ToLower(filepath.Ext(p)); ext != ".pl" && ext != ".lua" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for _, m := range questSpawn.FindAllStringSubmatch(string(b), -1) {
			if id, err := strconv.Atoi(m[1]); err == nil && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
		return nil
	})
	return out
}

func chrPath(zone string) string { return zone + "_chr.txt" }

// globalModels is the models every zone loads: from Spire's GlobalLoad.txt
// and GlobalLoad_chr.txt where it keeps them, else the stock ones' archives
// as the race list marks them
func globalModels(store *clientassets.Store) map[string]bool {
	out := map[string]bool{}
	gl, errGL := store.ReadRaceFile("Resources/GlobalLoad.txt")
	glc, errGLC := store.ReadRaceFile("Resources/GlobalLoad_chr.txt")
	if errGL == nil {
		for _, l := range gl.Lines {
			if l.Archive != "" && !strings.HasPrefix(l.Archive, "_") {
				for _, m := range archiveModels(l.Archive) {
					out[strings.ToUpper(m)] = true
				}
			}
		}
	}
	if errGLC == nil {
		for _, l := range glc.Lines {
			if l.Code != "" && !strings.HasPrefix(l.Archive, "_") {
				out[strings.ToUpper(l.Code)] = true
			}
		}
	}
	if errGL != nil || errGLC != nil {
		for _, r := range races {
			for _, s := range r.Sources {
				if !s.IsGlobal {
					continue
				}
				for _, m := range archiveModels(s.File) {
					out[strings.ToUpper(m)] = true
				}
				for _, m := range s.Models {
					out[strings.ToUpper(m.Code)] = true
				}
			}
		}
	}
	return out
}

// bestArchive is the archive a line for a code should name: Spire's own
// upload of <code>.eqg, the client's <code>.eqg or <code>_chr.s3d, else the
// client archive holding it with the fewest other models
func bestArchive(store *clientassets.Store, code string) string {
	lc := strings.ToLower(code)
	if store != nil {
		if _, ok := store.Index().Archives[lc+".eqg"]; ok {
			return lc
		}
	}
	files := rof2.Codes[strings.ToUpper(code)]
	for _, want := range []string{lc + ".eqg", lc + "_chr.s3d"} {
		for _, f := range files {
			if strings.ToLower(f) == want {
				return baseName(f)
			}
		}
	}
	best, size := "", 0
	for _, f := range files {
		n := len(rof2.Archives[strings.ToLower(f)].Models)
		if best == "" || n < size || (n == size && f < best) {
			best, size = f, n
		}
	}
	if best == "" {
		return ""
	}
	return baseName(best)
}

// PlanModels works out a zone's _chr.txt
func PlanModels(db *gorm.DB, store *clientassets.Store, questsDir, zone string) (*ModelPlan, error) {
	loadRaceData()
	if !zoneName.MatchString(zone) {
		return nil, fmt.Errorf("%q is not a zone short name", zone)
	}
	plan := &ModelPlan{Path: chrPath(zone), Stock: clientassets.Rof2ChrZones[zone], Lines: []ModelLine{}, Needs: []ModelNeed{},
		Adds: []ModelNeed{}, NoModel: []NPCRef{}, QuestNPCs: []NPCRef{}, Archives: map[string]int{}}
	versions, err := Versions(db, zone)
	if err != nil {
		return nil, err
	}
	plan.Versions = versions

	// the file as Spire keeps it
	var file clientassets.RaceFile
	if f, ok := store.Index().Files[strings.ToLower(plan.Path)]; ok && f.Size > 0 {
		plan.Held, plan.Path, plan.Updated = true, f.Path, f.Updated
		switch {
		case f.Ignore:
			plan.Mode = clientassets.ModeIgnore
		case f.Beta:
			plan.Mode = clientassets.ModeBeta
		}
		if file, err = store.ReadRaceFile(f.Path); err != nil {
			return nil, err
		}
	}
	plan.NeedsUpload = plan.Stock && !plan.Held

	// the NPCs: every version's spawns, and the quests'
	sc, err := zoneScope(db, zone, nil)
	if err != nil {
		return nil, err
	}
	ids := append([]int{}, sc.npcs...)
	fromQuests := map[int]bool{}
	for _, id := range questNPCs(questsDir, zone) {
		if !set(sc.npcs)[id] {
			fromQuests[id] = true
			ids = append(ids, id)
		}
	}
	var npcs []struct {
		ID, Race, Gender int
		Name             string
	}
	if err := db.Raw("SELECT id, name, race, gender FROM npc_types WHERE id IN ? ORDER BY id", nonEmpty(ids)).Scan(&npcs).Error; err != nil {
		return nil, err
	}

	global := globalModels(store)
	plan.Global = len(global)
	own := map[string]bool{}
	for _, a := range []string{zone + "_chr", zone + "_chr2"} {
		for _, m := range archiveModels(a) {
			own[strings.ToUpper(m)] = true
		}
	}
	lineFor := map[string]clientassets.RaceLine{}
	for _, l := range file.Lines {
		if l.Code != "" && !strings.HasPrefix(l.Archive, "_") {
			if _, ok := lineFor[strings.ToUpper(l.Code)]; !ok {
				lineFor[strings.ToUpper(l.Code)] = l
			}
		}
	}

	needs := map[string]*ModelNeed{}
	for _, n := range npcs {
		ref := NPCRef{ID: n.ID, Name: n.Name}
		if fromQuests[n.ID] {
			plan.QuestNPCs = append(plan.QuestNPCs, ref)
		}
		code := modelCode(n.Race, n.Gender)
		if code == "" {
			plan.NoModel = append(plan.NoModel, ref)
			continue
		}
		need := needs[code]
		if need == nil {
			need = &ModelNeed{Code: code, Race: n.Race, NPCs: []NPCRef{}}
			if r := races[n.Race]; r != nil {
				need.RaceName = r.Description
			}
			needs[code] = need
		}
		if len(need.NPCs) < 12 {
			need.NPCs = append(need.NPCs, ref)
		}
	}

	// races by name: a newer model of the same creature (Rat 415 for Rat 36)
	neededNames := map[string]string{}
	for _, need := range needs {
		if need.RaceName != "" {
			neededNames[strings.ToLower(need.RaceName)] = need.RaceName
		}
	}
	variantOf := func(code string) string {
		for _, r := range codeRaces[code] {
			if ri := races[r]; ri != nil {
				if name, ok := neededNames[strings.ToLower(ri.Description)]; ok {
					return name
				}
			}
		}
		return ""
	}

	// a needed model's archives' animation sets are needed too
	animations := map[string]bool{}
	for code := range needs {
		for _, f := range rof2.Codes[code] {
			for _, a := range rof2.Archives[strings.ToLower(f)].Animations {
				animations[strings.ToUpper(a)] = true
			}
		}
	}

	for code, need := range needs {
		switch l, ok := lineFor[code]; {
		case ok:
			need.From, need.Archive = "line", l.Archive
		case global[code]:
			need.From = "global"
		case own[code]:
			need.From = "zone"
		default:
			if a := bestArchive(store, code); a != "" {
				need.From, need.Archive = "add", a
				plan.Adds = append(plan.Adds, *need)
			} else {
				need.From = "missing"
			}
		}
		plan.Needs = append(plan.Needs, *need)
	}
	sort.Slice(plan.Needs, func(i, j int) bool { return plan.Needs[i].Code < plan.Needs[j].Code })
	sort.Slice(plan.Adds, func(i, j int) bool { return plan.Adds[i].Code < plan.Adds[j].Code })

	for i, l := range file.Lines {
		if l.Archive == "" {
			continue
		}
		code := strings.ToUpper(l.Code)
		ml := ModelLine{Index: i, Code: l.Code, Archive: l.Archive}
		switch {
		case strings.HasPrefix(l.Archive, "_"):
			ml.Status, ml.Why = "keep", "disabled (its archive starts with _)"
		case needs[code] != nil:
			ml.Status, ml.Why = "needed", fmt.Sprintf("%v: %v", needs[code].RaceName, npcNames(needs[code].NPCs))
		case animations[code]:
			ml.Status, ml.Why = "keep", "animations a needed model uses"
		case !raceCodes[code]:
			ml.Status, ml.Why = "keep", "no race has this model: an animation set or a custom model"
		case variantOf(code) != "":
			ml.Status, ml.Why = "variant", fmt.Sprintf("another %v model, and NPCs here are %v: the client may show this one in its place", variantOf(code), variantOf(code))
		default:
			ml.Status, ml.Remove, ml.Why = "unused", true, "no NPC here uses it"
		}
		// needed, but loaded anyway: a line may still pick its version
		if ml.Status == "needed" && (global[code] || own[code]) {
			where := "every zone loads it"
			if own[code] {
				where = "the zone's own " + zone + "_chr.s3d holds it"
			}
			ml.Why += "; " + where + " (a line can still pick which version: only remove it if that's not why it's here)"
			if global[code] {
				ml.Status = "global"
			} else {
				ml.Status = "zone"
			}
		}
		plan.Lines = append(plan.Lines, ml)
		plan.Archives[l.Archive] = len(archiveModels(l.Archive))
	}
	for _, a := range plan.Adds {
		plan.Archives[a.Archive] = len(archiveModels(a.Archive))
	}
	return plan, nil
}

func npcNames(refs []NPCRef) string {
	var names []string
	for i, r := range refs {
		if i == 4 {
			names = append(names, "…")
			break
		}
		names = append(names, strings.ReplaceAll(strings.TrimPrefix(r.Name, "#"), "_", " "))
	}
	return strings.Join(names, ", ")
}

type ModelSave struct {
	Remove  []int                   `json:"remove"` // line indexes
	Add     []clientassets.RaceLine `json:"add"`    // code and archive
	Mode    string                  `json:"mode"`   // add or beta
	Updated int64                   `json:"updated"`
}

// SaveModels writes the zone's _chr.txt to Spire's client patch
func SaveModels(store *clientassets.Store, zone string, req ModelSave) (string, error) {
	if !zoneName.MatchString(zone) {
		return "", fmt.Errorf("%q is not a zone short name", zone)
	}
	if req.Mode != clientassets.ModeAdd && req.Mode != clientassets.ModeBeta {
		return "", errors.New("mode is add or beta")
	}
	path := chrPath(zone)
	file := clientassets.RaceFile{Path: path, Kind: "chr", Lines: []clientassets.RaceLine{}, CRLF: true}
	if f, ok := store.Index().Files[strings.ToLower(path)]; ok && f.Size > 0 {
		if req.Updated != f.Updated {
			return "", fmt.Errorf("%v changed since this was worked out: look again", f.Path)
		}
		var err error
		if file, err = store.ReadRaceFile(f.Path); err != nil {
			return "", err
		}
	} else if clientassets.Rof2ChrZones[zone] {
		return "", fmt.Errorf("the client ships its own %v: upload it first, so its lines are kept", path)
	}
	drop := set(req.Remove)
	var lines []clientassets.RaceLine
	for i, l := range file.Lines {
		if !drop[i] {
			lines = append(lines, l)
		}
	}
	have := map[string]bool{}
	for _, l := range lines {
		have[strings.ToUpper(l.Code)+","+strings.ToLower(l.Archive)] = true
	}
	for _, a := range req.Add {
		code, archive := strings.ToUpper(strings.TrimSpace(a.Code)), strings.TrimSpace(a.Archive)
		if !regexp.MustCompile(`^[A-Za-z0-9]{3}$`).MatchString(code) || !regexp.MustCompile(`^[A-Za-z0-9_]+$`).MatchString(archive) {
			return "", fmt.Errorf("%q,%q is not a model line", a.Code, a.Archive)
		}
		if !have[code+","+strings.ToLower(archive)] {
			have[code+","+strings.ToLower(archive)] = true
			lines = append(lines, clientassets.RaceLine{Code: code, Archive: archive})
		}
	}
	file.Lines = lines
	if _, err := store.SaveRaceFile(file); err != nil {
		return "", err
	}
	saved := path
	if f := store.Index().Files[strings.ToLower(path)]; f != nil {
		saved = f.Path
	}
	if err := store.SetMode(saved, req.Mode); err != nil {
		return "", err
	}
	return saved, nil
}
