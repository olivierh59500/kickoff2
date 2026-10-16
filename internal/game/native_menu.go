package game

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/kickoff2/assets"
	"github.com/olivierh59500/kickoff2/internal/engine"
	"image"
	"image/color"
	"strings"
	"unicode/utf8"
)

// originalMenu preserves the menu records, text, and palette recovered by the
// extractor. Their coordinates are relative to the original 24-line top margin.
type originalMenu struct {
	Source  string       `json:"source"`
	Palette []uint16     `json:"amiga_palette"`
	Records []menuRecord `json:"records"`
}
type menuRecord struct {
	Kind                string `json:"kind"`
	Label               string `json:"label"`
	Asset               string `json:"asset"`
	X, Y, Width, Height int
	Selection           int    `json:"selection"`
	Flags               string `json:"flags"`
}

func loadOriginalMenus() map[string]originalMenu {
	data, err := assets.Files.ReadFile("original/menus.json")
	if err != nil {
		return nil
	}
	var menus []originalMenu
	if json.Unmarshal(data, &menus) != nil {
		return nil
	}
	result := map[string]originalMenu{}
	for _, menu := range menus {
		result[menu.Source] = menu
	}
	return result
}
func amigaMenuColor(word uint16) color.RGBA {
	return color.RGBA{uint8(min(255, int(word>>8&15)*34)), uint8(min(255, int(word>>4&15)*34)), uint8(min(255, int(word&15)*34)), 255}
}
func (g *Game) drawOriginalMenu(dst *ebiten.Image, source string, selected func(int) bool, labels map[int]string) {
	menu, ok := g.menus[source]
	if !ok {
		return
	}
	g.drawOriginalMenuRecords(dst, menu, selected, labels)
}

func (g *Game) drawOriginalMenuRecords(dst *ebiten.Image, menu originalMenu, selected func(int) bool, labels map[int]string) {
	source := menu.Source
	palette := make([]color.Color, len(menu.Palette))
	for i, v := range menu.Palette {
		palette[i] = amigaMenuColor(v)
	}
	dst.Fill(palette[0])
	for i := len(menu.Records) - 1; i >= 0; i-- {
		record := menu.Records[i]
		if record.Kind == "image" {
			if source == "ko6.men" || source == "ko15.men" {
				// These twelve records are alternative team/style icons at two
				// identical positions; the live choices are drawn after the menu.
				continue
			}
			im := g.asset(record.Asset)
			if record.Asset == "logo.bl1" {
				im = g.originalMenuLogo(source)
			}
			if im != nil {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(float64(record.X), float64(record.Y+24))
				dst.DrawImage(im, op)
			}
			continue
		}
		flags, err := hex.DecodeString(record.Flags)
		if err != nil || len(flags) < 10 {
			continue
		}
		background, border := int(flags[1])&15, int(flags[2])&15
		x, y := record.X, record.Y+24
		foreground := int(flags[6]) & 15
		fill := palette[background]
		fg := palette[foreground]
		if selected != nil && selected(record.Selection) {
			// The original highlight routine at0x820c uses complement drawing
			// across all four bitplanes, so palette indices are XORed with15.
			fill = palette[background^15]
			fg = palette[foreground^15]
			border ^= 15
		}
		if source == "ko4.men" && (record.Selection == 10+g.practicePlayers || record.Selection == 21 && !g.practiceDown || record.Selection == 22 && g.practiceDown) {
			border ^= 15
		}
		if flags[1] != 255 {
			// 8568..8598 paints the offset shadow before the button's fill.
			// Shadow colour and signed offset are record+1E and record+20.
			if len(flags) > 10 && int8(flags[8]) >= 0 {
				offset := int(int8(flags[10]))
				rect(dst, float64(x+offset), float64(y+offset), float64(record.Width), float64(record.Height), palette[int(flags[8])&15])
			}
			rect(dst, float64(x), float64(y), float64(record.Width), float64(record.Height), fill)
		}
		if flags[2] != 255 {
			// 8256 draws the complete one-pixel outline, with inclusive
			// endpoints at width-1/height-1 on all four sides.
			rect(dst, float64(x), float64(y), float64(record.Width), 1, palette[border])
			rect(dst, float64(x), float64(y+record.Height-1), float64(record.Width), 1, palette[border])
			rect(dst, float64(x), float64(y), 1, float64(record.Height), palette[border])
			rect(dst, float64(x+record.Width-1), float64(y), 1, float64(record.Height), palette[border])
		}
		label := record.Label
		if custom, ok := labels[record.Selection]; ok {
			label = custom
		}
		if source == "ko5.men" && record.Selection == 32 && record.Y >= 160 {
			label = ""
		}
		fontID := int(flags[5])
		advance, height := 6, 6
		if fontID > 0 {
			advance = 8
			height = 8
		}
		if fontID >= 2 {
			height = 16
		}
		lines := strings.Split(label, "\n")
		if len(lines) == 1 && utf8.RuneCountInString(label)*advance > record.Width && strings.Contains(label, " ") {
			lines = strings.SplitN(label, " ", 2)
		}
		for row, text := range lines {
			// 8156 uses inclusive centering (+1) and the signed alignment
			// byte at record+21: negative left, zero center, positive right.
			width := utf8.RuneCountInString(text) * advance
			tx, ty := x+(record.Width-width)/2+1, y+(record.Height-height*len(lines))/2+1+row*height
			if len(flags) > 11 {
				if int8(flags[11]) < 0 {
					tx = x + advance/2
				} else if flags[11] > 0 {
					tx = x + record.Width - width
				}
			}
			if int8(flags[7]) >= 0 {
				shade := int(flags[7]) & 15
				if selected != nil && selected(record.Selection) {
					shade ^= 15
				}
				// 80F4 draws the shade at the logical position, then80FC
				// subtracts the signed offset before drawing the foreground.
				depth := int(int8(flags[9]))
				g.nativeText(dst, text, tx, ty, palette[shade], fontID, advance, height)
				tx, ty = tx-depth, ty-depth
			}
			g.nativeText(dst, text, tx, ty, fg, fontID, advance, height)
		}
	}
}

func (g *Game) originalMenuLogo(source string) *ebiten.Image {
	key := "menu-logo:" + source
	if im := g.images[key]; im != nil {
		return im
	}
	data, err := assets.Files.ReadFile("original/raw/logo.bl1")
	if err != nil {
		return nil
	}
	bitmap, err := decodeIndexedOriginal(data)
	if err != nil {
		return nil
	}
	palette := map[byte]color.NRGBA{}
	for index, word := range g.menus[source].Palette {
		c := amigaMenuColor(word)
		palette[byte(index)] = color.NRGBA{c.R, c.G, c.B, 255}
	}
	im := ebiten.NewImageFromImage(bitmap.recolor(palette))
	g.images[key] = im
	return im
}
func (g *Game) originalHit(source string, x, y int) int {
	menu := g.menus[source]
	for _, r := range menu.Records {
		if r.Kind == "button" && r.Selection != 0 && x >= r.X && x < r.X+r.Width && y >= r.Y+24 && y < r.Y+24+r.Height {
			return r.Selection
		}
	}
	return 0
}
func (g *Game) selectedOption(code int) bool {
	switch code {
	case 11:
		return g.config.Pitch == engine.Normal
	case 12:
		return g.config.Pitch == engine.Wet
	case 13:
		return g.config.Pitch == engine.Soggy
	case 14:
		return g.config.Pitch == engine.Artificial
	case 15:
		return g.config.Pitch == engine.RandomPitch
	case 21:
		return g.config.HalfSeconds == 180
	case 22:
		return g.config.HalfSeconds == 300
	case 23:
		return g.config.HalfSeconds == 600
	case 24:
		return g.config.HalfSeconds == 1200
	case 31, 32, 33, 34:
		return g.config.Wind == code-31
	case 41:
		return !g.config.ExtraTime
	case 42:
		return g.config.ExtraTime
	case 51:
		return !g.config.Aftertouch
	case 52:
		return g.config.Aftertouch
	case 61:
		return g.config.LeagueSkill == 0
	case 62:
		return g.config.LeagueSkill == 1
	case 63:
		return g.config.LeagueSkill == 2
	case 101:
		return g.config.Referee < 0
	case 102:
		return g.config.Referee >= 0
	case 71:
		return g.config.Speed == 1
	case 72:
		return g.config.Speed == 0.5
	case 73:
		return g.config.Speed == 0.25
	}
	return false
}
func (g *Game) activateOriginalOption(code int) {
	switch code {
	case 11:
		g.config.Pitch = engine.Normal
	case 12:
		g.config.Pitch = engine.Wet
	case 13:
		g.config.Pitch = engine.Soggy
	case 14:
		g.config.Pitch = engine.Artificial
	case 15:
		g.config.Pitch = engine.RandomPitch
	case 21:
		g.config.HalfSeconds = 180
	case 22:
		g.config.HalfSeconds = 300
	case 23:
		g.config.HalfSeconds = 600
	case 24:
		g.config.HalfSeconds = 1200
	case 31, 32, 33, 34:
		g.config.Wind = code - 31
	case 41:
		g.config.ExtraTime = false
	case 42:
		g.config.ExtraTime = true
	case 51:
		g.config.Aftertouch = false
	case 52:
		g.config.Aftertouch = true
	case 61:
		g.config.LeagueSkill = 0
	case 62:
		g.config.LeagueSkill = 1
	case 63:
		g.config.LeagueSkill = 2
	case 71:
		g.config.Speed = 1
	case 72:
		g.config.Speed = 0.5
	case 73:
		g.config.Speed = 0.25
	case 81:
		g.optionTeam = 0
		g.skillReturn = optionsScreen
		g.screen = skillScreen
		g.selection = 6 - g.config.Skill[0]
	case 82:
		g.optionTeam = 1
		g.skillReturn = optionsScreen
		g.screen = skillScreen
		g.selection = 6 - g.config.Skill[1]
	case 91:
		g.openTactics(0)
	case 92:
		g.openTactics(1)
	case 101:
		g.config.Referee = -1
	case 102:
		g.config.Referee = (g.config.Referee + 1) % len(engine.Referees)
	case 110:
		g.Back()
	}
	g.saveSettings()
}

func (g *Game) nativeText(dst *ebiten.Image, text string, x, y int, c color.Color, fontID, advance, height int) {
	fontID = max(0, min(3, fontID))
	font := g.asset(fmt.Sprintf("font-bank%d", fontID))
	if font == nil {
		g.text(dst, text, x, y, c)
		return
	}
	for _, r := range text {
		code := int(r)
		limit := 256
		atlasHeight := 8
		if fontID >= 2 {
			limit = 128
			atlasHeight = 16
		}
		if code >= 0 && code < limit && r != ' ' {
			sx, sy := (code%16)*8, (code/16)*atlasHeight
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x), float64(y))
			op.ColorScale.ScaleWithColor(c)
			dst.DrawImage(font.SubImage(image.Rect(sx, sy, sx+8, sy+atlasHeight)).(*ebiten.Image), op)
		}
		x += advance
	}
}
func (g *Game) refereeLabel() string {
	if g.config.Referee < 0 || g.config.Referee >= len(engine.Referees) {
		return engine.Referees[0].Name
	}
	return engine.Referees[g.config.Referee].Name
}
