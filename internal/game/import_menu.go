package game

import (
	"fmt"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/kickoff2/internal/engine"
	"github.com/olivierh59500/kickoff2/internal/importdata"
)

// SetImportDir selects an existing desktop folder containing original SQD/TAC
// files or conventional AmigaDOS data-disk ADFs. The application only reads that folder; settings retain validated bytes
// in its private data directory so Android/desktop saves remain portable.
func (g *Game) SetImportDir(directory string) { g.importDir = directory }

// SetImportPickerEnabled exposes the host's native document chooser in the browser.
func (g *Game) SetImportPickerEnabled(enabled bool) { g.importPickerEnabled = enabled }

// TakeImportPickerRequest transfers a user activation to the Android host.
// The host calls it on the game thread after Update.
func (g *Game) TakeImportPickerRequest() bool {
	requested := g.importPickerRequested
	g.importPickerRequested = false
	return requested
}

// RefreshImports receives the result of a native document selection on the game thread.
func (g *Game) RefreshImports(message string) {
	if g.screen != importScreen {
		return
	}
	g.notice = message
	dir := g.importDir
	if dir == "" {
		dir = filepath.Join(g.dataDir, "imports")
	}
	files, err := g.listImportFiles(dir)
	if err != nil {
		g.notice = err.Error()
		return
	}
	g.importFiles = files
	g.selection = min(g.selection, len(files))
	g.notice = message
	g.CancelInput()
}

func (g *Game) importPage(delta int) {
	lastPage := len(g.importFiles) / 10
	page := max(0, min(lastPage, g.selection/10+delta))
	g.selection = min(len(g.importFiles), page*10)
}

func (g *Game) openImport(kind string, team int, returnTo screen) {
	dir := g.importDir
	if dir == "" {
		dir = filepath.Join(g.dataDir, "imports")
	}
	g.importKind, g.importTeam, g.importReturn = kind, team, returnTo
	files, err := g.listImportFiles(dir)
	g.importFiles = files
	g.openFeatureMenu(importScreen, 0)
	if err != nil {
		g.notice = err.Error()
	} else if len(files) == 0 {
		g.notice = "COPY SQD / TAC / ADF TO THE IMPORT FOLDER"
		if g.competitionLoad {
			g.notice = "SELECT AN ORIGINAL LEAGUE OR CUP SAVE"
		}
		if kind == "replay" {
			g.notice = "SELECT ORIGINAL MOVIES OR THEIR DATA DISK"
		}
	}
}

func (g *Game) importSelected() error {
	if g.selection < 0 || g.selection >= len(g.importFiles) {
		return fmt.Errorf("no original data file selected")
	}
	file := g.importFiles[g.selection]
	if g.importKind == "replay" {
		ids, err := g.importOriginalReplays(file)
		if err != nil {
			return err
		}
		g.replayBrowserGolden, g.replayLibraryIndex = false, len(g.replayCatalog.Clips)-len(ids)
		g.openFeatureMenu(replayBrowserScreen, 2)
		g.notice = "ORIGINAL MOVIES IMPORTED"
		return nil
	}
	data, checksum, err := importdata.Read(file)
	if err != nil {
		return err
	}
	team := g.importTeam
	if g.competitionLoad {
		return g.importCompetitionFile(file, data)
	}
	if file.Kind == "sqd" {
		squad, _ := engine.DecodeSquad(data)
		g.config.UserSquads[team] = append([]byte(nil), data...)
		g.config.SquadFiles[team] = ""
		g.resetSelectedSquad(team)
		g.resetSquadBindings(team)
		if squad.Name != "" {
			g.config.TeamNames[team] = squad.Name
		}
	} else {
		if g.importReturn == tacticsScreen {
			g.tacticPending = engine.OriginalCustomTacticID
			g.tacticPendingData = append([]byte(nil), data...)
			g.tacticPendingName = file.Name
		} else {
			state := engine.OriginalAssignedTactics(g.config, team)
			if err := engine.AssignOriginalTactic(&g.config, team, state.Active, engine.OriginalCustomTacticID, data, file.Name); err != nil {
				return err
			}
		}
	}
	g.saveSettings()
	if file.Kind == "sqd" && g.importReturn == competitionScreen && g.competitionEditor != nil {
		g.finishCompetitionEditor(true)
	} else if file.Kind == "sqd" && g.importReturn == squadScreen {
		g.openSquad(team, g.squadReturn)
	} else if file.Kind == "tac" && g.importReturn == tacticsScreen {
		g.screen, g.selection = tacticsScreen, 20
		g.CancelInput()
	} else {
		g.openFeatureMenu(g.importReturn, 0)
	}
	g.notice = fmt.Sprintf("IMPORTED %s %s", file.Name, checksum[:8])
	return nil
}

func (g *Game) updateImportMenu(x, y int, clicked bool) {
	_, dy, fire := g.menuPadInput()
	count := len(g.importFiles) + 1
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || dy < 0 {
		g.selection = (g.selection + count - 1) % count
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || dy > 0 {
		g.selection = (g.selection + 1) % count
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
		g.importPage(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) {
		g.importPage(1)
	}
	if clicked && x >= 20 && x < 300 && y >= 199 && y < 217 {
		switch {
		case x < 88:
			g.importPage(-1)
		case x >= 232:
			g.importPage(1)
		case g.importPickerEnabled:
			g.importPickerRequested = true
			g.CancelInput()
		}
		return
	}
	if clicked && x >= 20 && x < 300 && y >= 237 && y < 252 {
		g.Back()
		return
	}
	if clicked && x >= 20 && x < 300 && y >= 53 && y < 193 {
		row := (y - 53) / 14
		page := g.selection / 10
		index := page*10 + row
		if index > len(g.importFiles) {
			return
		}
		g.selection = index
		fire = true
	}
	if fire || g.menuActivate || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyZ) {
		if g.selection == len(g.importFiles) {
			g.Back()
		} else if err := g.importSelected(); err != nil {
			g.notice = err.Error()
		}
	}
}

func (g *Game) drawImportMenu(dst *ebiten.Image) {
	label := "LOAD PLAYER MANAGER SQUAD"
	if g.importKind == "tac" {
		label = "LOAD ORIGINAL TACTICS"
	}
	if g.importKind == "replay" {
		label = "LOAD ORIGINAL REPLAYS"
	}
	if g.competitionLoad {
		label = "LOAD LEAGUE"
		if g.importKind == "cup" {
			label = "LOAD CUP"
		}
	}
	g.centered(dst, label, 20, gold)
	if g.competitionLoad {
		g.centered(dst, "AMIGA DATA FILE / PORTABLE SAVE", 37, ink)
	} else if g.importKind == "replay" {
		g.centered(dst, "MOVIES / MATCH INDEX / DATA DISK", 37, ink)
	} else {
		g.centered(dst, fmt.Sprintf("TEAM %c : VALIDATED ORIGINAL %s", 'A'+g.importTeam, g.importKind), 37, ink)
	}
	page := g.selection / 10
	for row := 0; row < 10; row++ {
		index := page*10 + row
		if index > len(g.importFiles) {
			break
		}
		name := "CANCEL"
		if index < len(g.importFiles) {
			name = g.importFiles[index].Name
			if len(name) > 44 {
				name = "..." + name[len(name)-41:]
			}
		}
		if index == g.selection {
			rect(dst, 20, float64(53+row*14), 280, 14, gold)
			g.text(dst, name, 28, 55+row*14, panel)
		} else {
			g.text(dst, name, 28, 55+row*14, ink)
		}
	}
	for _, button := range []struct {
		x, width int
		label    string
	}{{20, 66, "PREV"}, {90, 138, "OPEN FILE"}, {232, 68, "NEXT"}} {
		if button.label == "OPEN FILE" && !g.importPickerEnabled {
			continue
		}
		rect(dst, float64(button.x), 199, float64(button.width), 18, panel)
		g.text(dst, button.label, button.x+(button.width-len(button.label)*6)/2, 204, gold)
	}
	g.centered(dst, "CANCEL", 241, ink)
	if g.notice != "" {
		g.centered(dst, g.notice, 225, gold)
	}
}
