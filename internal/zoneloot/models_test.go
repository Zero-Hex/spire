package zoneloot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/EQEmu/spire/internal/clientassets"
	"github.com/EQEmu/spire/internal/logger"
	"github.com/EQEmu/spire/internal/pathmgmt"
)

func testStore(t *testing.T) *clientassets.Store {
	pm := pathmgmt.NewPathManagement(logger.ProvideAppLogger())
	pm.SetServerPath(t.TempDir())
	return clientassets.NewStore(logger.ProvideAppLogger(), pm)
}

func TestModelCodes(t *testing.T) {
	loadRaceData()
	for _, c := range []struct {
		race, gender int
		want         string
	}{{1, 0, "HUM"}, {1, 1, "HUF"}, {36, 2, "RAT"}, {415, 2, "RTN"}, {23, 1, "CPF"}, {99999, 0, ""}} {
		if got := modelCode(c.race, c.gender); got != c.want {
			t.Errorf("race %d gender %d: %q, want %q", c.race, c.gender, got, c.want)
		}
	}
}

func TestPlanModels(t *testing.T) {
	db := testDB(t)
	store := testStore(t)

	plan, err := PlanModels(db, store, "", "warrens")
	if err != nil {
		t.Fatal(err)
	}
	// every model the Warrens NPCs show is the zone's own or global
	if !plan.NeedsUpload || plan.Held || len(plan.Adds) != 0 || len(plan.Needs) < 8 {
		t.Fatalf("plan: upload %v held %v adds %+v needs %d", plan.NeedsUpload, plan.Held, plan.Adds, len(plan.Needs))
	}
	if _, err := SaveModels(store, "warrens", ModelSave{Mode: "add"}); err == nil || !strings.Contains(err.Error(), "upload it first") {
		t.Errorf("saved over the stock file without it: %v", err)
	}

	// the client's file, with lines added by hand
	file := clientassets.RaceFile{Path: "warrens_chr.txt", Kind: "chr", CRLF: true, Lines: []clientassets.RaceLine{
		{Code: "rtn", Archive: "rtn"}, {Code: "btn", Archive: "btn"}, {Code: "dke", Archive: "dke"},
		{Code: "T05", Archive: "T05"}, {Code: "xyz", Archive: "starfall_custom"}, {Code: "kob", Archive: "warrens_chr"},
		{Code: "dcm", Archive: "dcm"},
	}}
	if _, err := store.SaveRaceFile(file); err != nil {
		t.Fatal(err)
	}
	// the zone's quests spawn a drakeling (DKE) and a sea serpent (SNK, needing a line)
	quests := t.TempDir()
	os.MkdirAll(filepath.Join(quests, "warrens"), 0755)
	os.WriteFile(filepath.Join(quests, "warrens", "event.pl"), []byte("sub EVENT_SAY { quest::spawn2(25312, 0, 0, $x, $y, $z, $h); }"), 0644)
	os.WriteFile(filepath.Join(quests, "warrens", "x.lua"), []byte("function event_say(e) eq.unique_spawn(719, 0, 0, 1, 2, 3, 0) end"), 0644)

	plan, err = PlanModels(db, store, quests, "warrens")
	if err != nil {
		t.Fatal(err)
	}
	if plan.NeedsUpload || !plan.Held || len(plan.QuestNPCs) != 2 {
		t.Fatalf("plan: upload %v held %v quest NPCs %+v", plan.NeedsUpload, plan.Held, plan.QuestNPCs)
	}
	status := map[string]ModelLine{}
	for _, l := range plan.Lines {
		status[strings.ToUpper(l.Code)] = l
	}
	for code, want := range map[string]string{"RTN": "variant", "BTN": "variant", "DKE": "needed", "DCM": "unused", "T05": "keep", "XYZ": "keep", "KOB": "zone"} {
		if status[code].Status != want {
			t.Errorf("%v: %v (%v), want %v", code, status[code].Status, status[code].Why, want)
		}
	}
	if !status["DCM"].Remove || status["T05"].Remove || status["RTN"].Remove || status["KOB"].Remove {
		t.Errorf("suggested removals: %+v", plan.Lines)
	}
	if len(plan.Adds) != 1 || plan.Adds[0].Code != "SNK" || plan.Adds[0].Archive != "snk" {
		t.Fatalf("adds: %+v", plan.Adds)
	}

	// save: drop the drachnid, add the snake, beta only
	path, err := SaveModels(store, "warrens", ModelSave{Remove: []int{status["DCM"].Index}, Add: []clientassets.RaceLine{{Code: "SNK", Archive: "snk"}},
		Mode: "beta", Updated: plan.Updated})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := store.ClientFileData(path)
	if string(b) != "7\r\nRTN,rtn\r\nBTN,btn\r\nDKE,dke\r\nT05,T05\r\nXYZ,starfall_custom\r\nKOB,warrens_chr\r\nSNK,snk\r\n" {
		t.Errorf("file: %q", b)
	}
	if f := store.Index().Files["warrens_chr.txt"]; f == nil || !f.Beta {
		t.Errorf("not beta only: %+v", f)
	}
	after, _ := PlanModels(db, store, quests, "warrens")
	if len(after.Adds) != 0 {
		t.Errorf("adds after saving: %+v", after.Adds)
	}
	if _, err := SaveModels(store, "warrens", ModelSave{Mode: "add", Updated: plan.Updated - 100}); err == nil {
		t.Error("saved over a file changed since")
	}
}
