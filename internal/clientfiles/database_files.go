package clientfiles

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/EQEmu/spire/internal/models"
	"github.com/labstack/echo/v4"
)

// the client files the server's export_client_files writes from the
// database, and where each goes in the client folder
var serverExports = []struct {
	file  string // as export_client_files names it, in the server's export folder
	path  string // in the client folder
	table string
}{
	{"spells_us.txt", "spells_us.txt", "spells_new"},
	{"dbstr_us.txt", "dbstr_us.txt", "db_str"},
	{"SkillCaps.txt", "Resources/SkillCaps.txt", "skill_caps"},
	{"BaseData.txt", "Resources/BaseData.txt", "base_data"},
}

// one export at a time: they share the export folder
var exportMu sync.Mutex

type builtFile struct {
	Path   string
	Source string
	Data   []byte
}

// databaseFiles builds the client files that come from the database. The
// server's own export_client_files makes them when Spire can find it (it is
// what the server's tooling ships); otherwise Spire's exporter makes
// spells_us.txt and dbstr_us.txt, and SkillCaps.txt and BaseData.txt are left
// out with a note.
func (f *Controller) databaseFiles(c echo.Context) ([]builtFile, []string, error) {
	exportMu.Lock()
	defer exportMu.Unlock()

	var notes []string
	bin := f.pathmgmt.GetExportClientFilesBinPath()
	if _, err := os.Stat(bin); err == nil {
		files, err := f.runServerExport(bin)
		if err == nil {
			return files, nil, nil
		}
		notes = append(notes, fmt.Sprintf("export_client_files failed (%v); spells_us.txt and dbstr_us.txt came from Spire's own export instead, and SkillCaps.txt and BaseData.txt were left out.", err))
	} else {
		notes = append(notes, fmt.Sprintf("Spire did not find export_client_files (%v), so spells_us.txt and dbstr_us.txt came from its own export and SkillCaps.txt and BaseData.txt were left out.", bin))
	}

	spells := f.exporter.ExportSpells(f.db.Get(&models.SpellsNew{}, c))
	if spells == "" {
		return nil, notes, errors.New("spells_new exported nothing")
	}
	dbstr := f.exporter.ExportDbStr(f.db.Get(&models.DbStr{}, c))
	return []builtFile{
		{Path: "spells_us.txt", Source: "database (spells_new, Spire's export)", Data: []byte(spells)},
		{Path: "dbstr_us.txt", Source: "database (db_str, Spire's export)", Data: []byte(dbstr)},
	}, notes, nil
}

func (f *Controller) runServerExport(bin string) ([]builtFile, error) {
	if err := f.pathmgmt.MakeExportDirIfNotExists(); err != nil {
		return nil, err
	}
	dir := f.pathmgmt.GetExportDir()
	// a file left from an earlier export must not pass for a new one
	for _, e := range serverExports {
		_ = os.Remove(filepath.Join(dir, e.file))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin) // no argument: every file
	cmd.Dir = f.pathmgmt.GetEQEmuServerPath()
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%v: %s", err, lastLine(out))
	}

	var files []builtFile
	for _, e := range serverExports {
		data, err := os.ReadFile(filepath.Join(dir, e.file))
		if err != nil {
			return nil, fmt.Errorf("it wrote no %v", e.file)
		}
		files = append(files, builtFile{Path: e.path, Source: "database (" + e.table + ")", Data: data})
	}
	return files, nil
}

func lastLine(out []byte) string {
	s := string(out)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\n' {
			return s[i+1:]
		}
	}
	return s
}
