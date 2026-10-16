package game

import (
	"image"
	"strings"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/exp/textinput"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/kickoff2/internal/engine"
)

// competitionTeamEditor keeps a country edit separate from played-fixture setup.
type competitionTeamEditor struct {
	country, selection int
	saved              engine.Config
}

type competitionNameEditor struct {
	active, ime bool
	country     int
	field       textinput.Field
}

func competitionEditTouchKey(x, y int) (ebiten.Key, bool) {
	if y >= 225 && y < 239 {
		for index, key := range []ebiten.Key{ebiten.KeyF3, ebiten.KeyF5} {
			if x >= 10+index*160 && x < 150+index*160 {
				return key, true
			}
		}
	}
	return 0, false
}

func (g *Game) selectedCompetitionCountry() (int, bool) {
	if g.tournament == nil || g.selection < 10 || g.selection > 66 || (g.selection-10)%8 != 0 {
		return 0, false
	}
	return g.competitionRows()[(g.selection-10)/8], true
}

// competitionRowAction follows 6056/607A/6148 without changing the row order.
func (g *Game) competitionRowAction(key ebiten.Key) {
	country, ok := g.selectedCompetitionCountry()
	if !ok || g.competitionResult != nil || g.competitionEditor != nil || g.competitionName.active {
		return
	}
	if key == ebiten.KeyEnter {
		g.competitionName = competitionNameEditor{active: true, country: country}
		if g.touch {
			value := g.tournament.Names[country]
			g.competitionName.field.SetTextAndSelection(value, len(value), len(value))
			g.competitionName.field.Focus()
			g.competitionName.ime = true
		}
		g.CancelInput()
		return
	}
	if key != ebiten.KeyF3 && key != ebiten.KeyF5 {
		return
	}
	if key == ebiten.KeyF3 {
		if g.competitionSquadImportLocked {
			return // Source 605E tests the shared import-lock word.
		}
	}
	config, err := g.tournament.CountryConfig(g.config, country)
	if err != nil {
		g.notice = err.Error()
		return
	}
	g.competitionEditor = &competitionTeamEditor{country: country, selection: g.selection, saved: g.config}
	g.config = config
	if key == ebiten.KeyF3 {
		g.openImport("sqd", 0, competitionScreen)
	} else {
		g.openTactics(0)
	}
}

func (g *Game) captureCompetitionEditor() {
	if g.competitionEditor != nil && g.tournament != nil {
		g.tournament.StoreCountrySetup(g.competitionEditor.country, g.config)
	}
}

func (g *Game) finishCompetitionEditor(commit bool) {
	if g.competitionEditor == nil {
		return
	}
	if commit {
		g.captureCompetitionEditor()
	}
	editor := g.competitionEditor
	g.config, g.competitionEditor = editor.saved, nil
	g.openFeatureMenu(competitionScreen, editor.selection)
}

// competitionNameKey implements the original 872A..875A byte editor. Native
// strings represent each source Latin-1 byte as one rune for font and JSON use.
func competitionNameKey(value string, key uint16) (string, bool) {
	if key == 13 {
		return value, true
	}
	if key == 8 || key == 127 {
		if len(value) != 0 {
			_, size := utf8.DecodeLastRuneInString(value)
			value = value[:len(value)-size]
		}
		return value, false
	}
	if key >= 32 && key <= 255 && utf8.RuneCountInString(value) < 20 {
		value += string(rune(key))
	}
	return value, false
}

func cleanCompetitionName(value string) string {
	var result strings.Builder
	count := 0
	for _, character := range value {
		if character >= 32 && character <= 255 && character != 127 && count < 20 {
			result.WriteRune(character)
			count++
		}
	}
	return result.String()
}

func (g *Game) finishCompetitionName() {
	if !g.competitionName.active {
		return
	}
	if g.competitionName.ime {
		value := g.competitionName.field.TextForRendering()
		committed := g.competitionName.field.Text()
		g.competitionName.field.Blur()
		if latest := g.competitionName.field.Text(); latest != committed {
			value = latest
		}
		g.tournament.Names[g.competitionName.country] = cleanCompetitionName(value)
	}
	g.competitionName.active, g.competitionName.ime = false, false
	g.CancelInput()
}

func (g *Game) competitionNameBounds() image.Rectangle {
	for _, record := range g.menus[g.competitionMenuSource()].Records {
		if record.Kind == "button" && record.Selection == g.selection {
			x, y := record.X+g.fieldOffset()+2, record.Y+24+2
			return image.Rect(x, y, x+record.Width-4, y+record.Height-4)
		}
	}
	return image.Rect(g.fieldOffset()+2, 55, g.fieldOffset()+131, 68)
}

func (g *Game) updateCompetitionName(clicked bool) {
	name := &g.tournament.Names[g.competitionName.country]
	if g.competitionName.ime {
		if _, err := g.competitionName.field.HandleInputWithBounds(g.competitionNameBounds()); err != nil {
			g.notice = err.Error()
			g.finishCompetitionName()
			return
		}
		value := g.competitionName.field.TextForRendering()
		*name = cleanCompetitionName(value)
		if value == g.competitionName.field.Text() && *name != value {
			// Bound committed IME text too, so Backspace never edits an invisible tail.
			g.competitionName.field.SetTextAndSelection(*name, len(*name), len(*name))
		}
	} else {
		for _, character := range ebiten.AppendInputChars(nil) {
			if character <= 255 {
				*name, _ = competitionNameKey(*name, uint16(character))
			}
		}
		for _, key := range []ebiten.Key{ebiten.KeyBackspace, ebiten.KeyDelete} {
			if inpututil.IsKeyJustPressed(key) {
				*name, _ = competitionNameKey(*name, 8)
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) || g.menuActivate || clicked || g.competitionName.ime && !g.competitionName.field.IsFocused() {
		g.finishCompetitionName()
	}
}

func (g *Game) drawCompetitionNameCursor(dst *ebiten.Image) {
	for _, record := range g.menus[g.competitionMenuSource()].Records {
		if record.Kind != "button" || record.Selection != g.selection {
			continue
		}
		width := utf8.RuneCountInString(g.tournament.Names[g.competitionName.country]) * 6
		x := record.X + (record.Width-width)/2 + 1 + width
		y := record.Y + 24 + (record.Height-6)/2 + 2
		palette := g.menus[g.competitionMenuSource()].Palette
		// Original 86FE draws below the text pen through the last font row.
		rect(dst, float64(x), float64(y), 1, 5, amigaMenuColor(palette[15]))
		return
	}
}

func (g *Game) textInputActive() bool { return g.replayTyping || g.competitionName.active }

func (g *Game) activeTextField() *textinput.Field {
	if g.competitionName.active && g.competitionName.ime {
		return &g.competitionName.field
	}
	if g.replayTyping && g.replayText.active {
		return &g.replayText.field
	}
	return nil
}
