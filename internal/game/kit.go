package game

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/kickoff2/assets"
	"github.com/olivierh59500/kickoff2/internal/engine"
	"github.com/olivierh59500/kickoff2/internal/originalgraphics"
)

// kitColors is the complete original 20-entry color table at address 0x17264.
// The kit editor routine at 0x5b66 wraps both shirt and shorts choices at 20.
var kitColors = [...]uint16{0x00a, 0x00f, 0x08f, 0x0ff, 0xfff, 0xaaa, 0x888, 0x000, 0x600, 0xa00, 0xe00, 0xf60, 0xff0, 0xf8f, 0xf0f, 0x60f, 0x060, 0x0a0, 0x48a, 0x086}

type kitDesign struct {
	Style, Shirt, Shorts int
}

func validKitDesigns(designs [10]kitDesign) bool {
	for _, design := range designs {
		if design.Style < 0 || design.Style >= 6 || design.Shirt < 0 || design.Shirt >= len(kitColors) || design.Shorts < 0 || design.Shorts >= len(kitColors) {
			return false
		}
	}
	return true
}

// originalKitDefaults preserves the eight country defaults at address 0x170ea.
func originalKitDefaults() [10]kitDesign {
	// Boot53D2..53F0 initializes ANCO/Computer to(1,12,7)/(3,10,4).
	return [10]kitDesign{{1, 12, 7}, {3, 10, 4}, {0, 4, 1}, {0, 9, 1}, {0, 2, 4}, {0, 5, 7}, {0, 1, 4}, {0, 12, 1}, {0, 11, 4}, {1, 1, 4}}
}

func kitColor(index int) color.NRGBA {
	w := kitColors[(index+len(kitColors))%len(kitColors)]
	return color.NRGBA{uint8(w>>8&15) * 17, uint8(w>>4&15) * 17, uint8(w&15) * 17, 255}
}

func (g *Game) kitIndex(team int) int {
	name := g.config.TeamNames[team]
	if g.screen == preMatchScreen && g.prematch != nil {
		name = g.prematch.Config().TeamNames[team]
	}
	if g.screen == replayScreen && g.replaySource != nil {
		name = g.replaySource.Teams[team].Name
	} else if g.Match != nil && g.screen == matchScreen {
		name = g.Match.Teams[team].Name
	}
	for i, country := range engine.InternationalCountries {
		if strings.EqualFold(name, country) {
			return i + 2
		}
	}
	return team
}

func (g *Game) teamKitDesign(team int) kitDesign {
	config := g.config
	if g.screen == preMatchScreen && g.prematch != nil {
		config = g.prematch.Config()
	} else if g.screen == replayScreen && g.replaySource != nil {
		config = g.replaySource.Config
	} else if g.screen == matchScreen && g.Match != nil {
		config = g.Match.Config
	}
	if config.HasTeamKits[team] && config.TeamKits[team].Valid() {
		kit := config.TeamKits[team]
		return kitDesign{kit.Style, kit.Shirt, kit.Shorts}
	}
	if g.screen == replayScreen {
		return g.replaySourceKits[g.kitIndex(team)]
	}
	return g.kits[g.kitIndex(team)]
}

func kitTeamName(index int) string {
	if index < 2 {
		return [...]string{"Team A", "Team B"}[index]
	}
	return engine.InternationalCountries[index-2]
}

// indexedOriginal preserves planar palette indices. An RGB replacement would
// merge distinct shirt/shorts indices whenever their original colors coincide.
type indexedOriginal struct {
	width, height int
	indices       []byte
	palette       []color.NRGBA
	transparent   int
}

func decodeIndexedOriginal(data []byte) (indexedOriginal, error) {
	decoded, err := originalgraphics.DecodeIFF(data)
	return indexedOriginal{width: decoded.Width, height: decoded.Height,
		indices: decoded.Pixels, palette: decoded.Palette, transparent: decoded.Transparent}, err
}

func (source indexedOriginal) recolor(replacements map[byte]color.NRGBA) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, source.width, source.height))
	for i, index := range source.indices {
		if int(index) == source.transparent {
			continue
		}
		c := source.palette[index]
		if replacement, ok := replacements[index]; ok {
			c = replacement
		}
		dst.SetNRGBA(i%source.width, i/source.width, c)
	}
	return dst
}

func (g *Game) coloredKitAsset(filename string, design kitDesign, shirt, shorts byte) *ebiten.Image {
	neutral := g.kitTeam < 0 && strings.HasPrefix(filename, "kit")
	key := fmt.Sprintf("kit:%s:%d:%d:%t", filename, design.Shirt, design.Shorts, neutral)
	if im := g.images[key]; im != nil {
		return im
	}
	data, err := assets.Files.ReadFile("original/raw/" + filename)
	if err != nil {
		return nil
	}
	source, err := decodeIndexedOriginal(data)
	if err != nil {
		return nil
	}
	palette := map[byte]color.NRGBA{}
	if strings.HasSuffix(filename, ".bl1") {
		menu := "ko11.men"
		if strings.HasPrefix(filename, "skit") {
			menu = "ko6.men"
		}
		for index, word := range g.menus[menu].Palette {
			c := amigaMenuColor(word)
			palette[byte(index)] = color.NRGBA{c.R, c.G, c.B, 255}
		}
	}
	if !neutral {
		palette[shirt], palette[shorts] = kitColor(design.Shirt), kitColor(design.Shorts)
	}
	im := ebiten.NewImageFromImage(source.recolor(palette))
	g.images[key] = im
	return im
}

func (g *Game) playerKitSheet(team int) *ebiten.Image {
	design := g.teamKitDesign(team)
	// The original A/B sheets encode different palette indices so both teams
	// share one Amiga palette: A uses4/6; B uses2/5 (0xf6f2..0xf704).
	filename := fmt.Sprintf("style%da.iff", design.Style+1)
	shirt, shorts := byte(4), byte(6)
	if team == 1 {
		filename = fmt.Sprintf("style%db.iff", design.Style+1)
		shirt, shorts = 2, 5
	}
	return g.coloredKitAsset(filename, design, shirt, shorts)
}

func (g *Game) activateKit(code, delta int) {
	if code == 20 {
		g.Back()
		g.saveSettings()
		return
	}
	if code >= 10 && code <= 19 {
		g.kitTeam = code - 10
		g.saveSettings()
		return
	}
	if g.kitTeam < 0 {
		g.kitTeam = 0
	}
	design := &g.kits[g.kitTeam]
	before := *design
	switch {
	case code >= 1 && code <= 6:
		design.Style = code - 1
	case code == 7:
		design.Shirt = (design.Shirt + delta + len(kitColors)) % len(kitColors)
	case code == 8:
		design.Shorts = (design.Shorts + delta + len(kitColors)) % len(kitColors)
	}
	if *design != before {
		g.clearColoredKitCache()
	}
	g.saveSettings()
}

func (g *Game) clearColoredKitCache() {
	for key, im := range g.images {
		if strings.HasPrefix(key, "kit:") {
			im.Deallocate()
			delete(g.images, key)
		}
	}
}

func kitLabelInk(c color.NRGBA) color.Color {
	if int(c.R)*3+int(c.G)*6+int(c.B) > 1280 {
		return color.Black
	}
	return color.White
}

func (g *Game) drawKit(dst *ebiten.Image) {
	design := kitDesign{0, 10, 4}
	labels := map[int]string{9: ""}
	if g.kitTeam >= 0 {
		design = g.kits[g.kitTeam]
		labels[9] = kitTeamName(g.kitTeam)
	}
	for i := range g.kits {
		labels[i+10] = kitTeamName(i)
	}
	g.drawOriginalMenu(dst, "ko11.men", func(code int) bool {
		return code == g.selection || g.kitTeam >= 0 && code >= 1 && code <= 6 && code == design.Style+1
	}, labels)
	for _, record := range g.menus["ko11.men"].Records {
		if record.Kind == "image" {
			if im := g.coloredKitAsset(record.Asset, design, 11, 12); im != nil {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(float64(record.X), float64(record.Y+24))
				dst.DrawImage(im, op)
			}
		}
	}
	rect(dst, 242, 88, 65, 17, kitColor(design.Shirt))
	rect(dst, 242, 106, 65, 17, kitColor(design.Shorts))
	if g.kitTeam >= 0 {
		g.text(dst, "SHIRT", 248, 92, kitLabelInk(kitColor(design.Shirt)))
		g.text(dst, "SHORTS", 248, 110, kitLabelInk(kitColor(design.Shorts)))
	}
	for i := range kitColors {
		rect(dst, float64(14+i*14), 217, 13, 10, kitColor(i))
	}
	g.centered(dst, "SELECT TEAM / STYLE / SHIRT / SHORTS", 240, color.White)
}
