package game

import (
	"os"
	"path/filepath"

	"github.com/olivierh59500/kickoff2/internal/engine"
	"github.com/olivierh59500/kickoff2/internal/importdata"
)

func (g *Game) openCompetitionLoad() {
	if g.tournament == nil {
		return
	}
	// Original63D6 locks a league even when its filename chooser is cancelled.
	if !g.tournament.Cup {
		g.competitionSquadImportLocked = true
	}
	g.competitionLoad = true
	kind := "lge"
	if g.tournament.Cup {
		kind = "cup"
	}
	g.openImport(kind, 0, competitionScreen)
}

func (g *Game) listImportFiles(directory string) ([]importdata.File, error) {
	files, err := importdata.List(directory, g.importKind)
	if err != nil || !g.competitionLoad {
		return files, err
	}
	exports := filepath.Join(g.dataDir, "exports")
	if directory != exports {
		saved, err := importdata.List(exports, g.importKind)
		if err != nil {
			return nil, err
		}
		files = append(saved, files...)
	}
	path := filepath.Join(g.dataDir, "tournament.json")
	if stat, err := os.Lstat(path); err == nil && stat.Mode().IsRegular() && stat.Size() <= 2*1024*1024 {
		files = append([]importdata.File{{Name: "PORTABLE SAVE - MATCH HISTORY", Path: path, Kind: "tournament", Size: stat.Size()}}, files...)
	}
	return files, nil
}

func (g *Game) exportCompetitionDocument() {
	if !g.importPickerEnabled || g.tournament == nil || g.notice != "TOURNAMENT SAVED" {
		return
	}
	extension := ".lge"
	if g.tournament.Cup {
		extension = ".cup"
	}
	g.requestDocumentExport(filepath.Join(g.dataDir, "exports", "tournament"+extension))
}

func (g *Game) importCompetitionFile(file importdata.File, data []byte) error {
	var tournament *engine.Tournament
	var err error
	if file.Kind == "tournament" {
		tournament, err = engine.LoadTournament(data)
	} else {
		random := [3]uint16{0, uint16(g.config.Seed), uint16(g.config.Seed >> 16)}
		if g.tournament != nil {
			random = g.tournament.OriginalRandomState
		}
		tournament, err = engine.DecodeOriginalTournament(data, file.Kind == "cup", random)
		if err == nil && g.tournament != nil {
			// Like the original RNG, the initializer's pace-scale word is
			// session state outside the competition file's saved regions.
			tournament.OriginalPaceScale = g.tournament.OriginalPaceScale
		}
	}
	if err != nil {
		return err
	}
	g.competitionLoad = false
	g.installTournament(tournament)
	return nil
}
