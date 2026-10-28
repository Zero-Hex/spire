package clientfiles

import (
	"archive/zip"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/EQEmu/spire/internal/clientassets"
	"github.com/EQEmu/spire/internal/models"
	"github.com/labstack/echo/v4"
)

// the client patch: the files the RoF2 client needs from Spire, in one zip
// laid out as the client folder. The database files are made as it is built;
// the rest is what the client assets store holds.

type patchContents struct {
	Files []clientassets.PatchFile `json:"files"`
	Notes []string                 `json:"notes"`
}

func (f *Controller) databaseFiles(c echo.Context) []clientassets.PatchFile {
	count := func(model models.Modelable, table, what string) string {
		var n int64
		if err := f.db.Get(model, c).Model(model).Count(&n).Error; err != nil {
			return fmt.Sprintf("database: %v cannot be read (%v)", table, err)
		}
		return fmt.Sprintf("database: %d %s (%s)", n, what, table)
	}
	now := time.Now().Unix()
	return []clientassets.PatchFile{
		{Path: "dbstr_us.txt", Source: count(&models.DbStr{}, "db_str", "strings"), Updated: now},
		{Path: "spells_us.txt", Source: count(&models.SpellsNew{}, "spells_new", "spells"), Updated: now},
	}
}

func (f *Controller) patchContents(c echo.Context) error {
	files, notes := f.assets.PatchFiles()
	files = append(f.databaseFiles(c), files...)
	if notes == nil {
		notes = []string{}
	}
	return c.JSON(http.StatusOK, patchContents{Files: files, Notes: notes})
}

// patch streams the zip
func (f *Controller) patch(c echo.Context) error {
	// the database files first: a failure there can still be an error page
	spells := f.exporter.ExportSpells(f.db.Get(&models.SpellsNew{}, c))
	dbstr := f.exporter.ExportDbStr(f.db.Get(&models.DbStr{}, c))
	if spells == "" {
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "spells_new exported nothing"})
	}

	assets, notes := f.assets.PatchFiles()
	built := time.Now()
	name := "client-patch-" + built.Format("2006-01-02-1504") + ".zip"

	res := c.Response()
	res.Header().Set(echo.HeaderContentType, "application/zip")
	res.Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", name))
	res.WriteHeader(http.StatusOK)

	zw := zip.NewWriter(res)
	var listing []string
	add := func(path, source string, modified time.Time, data []byte) {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: path, Method: zip.Deflate, Modified: modified})
		if err == nil {
			_, err = w.Write(data)
		}
		if err != nil {
			notes = append(notes, fmt.Sprintf("%v could not be written: %v", path, err))
			return
		}
		listing = append(listing, fmt.Sprintf("  %-44s %s", path, source))
	}

	add("spells_us.txt", "database (spells_new)", built, []byte(spells))
	add("dbstr_us.txt", "database (db_str)", built, []byte(dbstr))
	for _, a := range assets {
		data, err := a.Read()
		if err != nil {
			notes = append(notes, fmt.Sprintf("%v left out: %v", a.Path, err))
			continue
		}
		add(a.Path, a.Source, time.Unix(a.Updated, 0), data)
	}

	readme := []string{
		"Client patch built by Spire " + built.Format("2006-01-02 15:04 MST"),
		"",
		"Unzip into the RoF2 client folder (the one with eqgame.exe), replacing files.",
		"",
		"Files:",
	}
	readme = append(readme, listing...)
	if len(notes) > 0 {
		readme = append(readme, "", "Notes:")
		for _, n := range notes {
			readme = append(readme, "  - "+n)
		}
	}
	add("README-client-patch.txt", "this file", built, []byte(strings.Join(readme, "\r\n")+"\r\n"))

	return zw.Close()
}
