package game

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/kickoff2/internal/engine"
)

// Menus and played matches share one generator. Cancelling a toss or starting
// another match must not reset it to the same configuration seed.
func (g *Game) prepareMenuMatch(cfg engine.Config) *engine.MatchPreparation {
	if g.prematch != nil {
		return engine.PrepareMatchWithOriginalRandomState(cfg, g.prematch.RandomState())
	}
	if g.Match != nil {
		return engine.PrepareMatchWithOriginalRandomState(cfg, g.Match.OriginalRandomState())
	}
	return engine.PrepareMatch(cfg)
}

func (g *Game) openPrematch(prepared *engine.MatchPreparation) {
	if prepared == nil {
		cfg := g.config
		cfg.Practice = false
		prepared = g.prepareMenuMatch(cfg)
	}
	g.prematch, g.toss = prepared, prepared.Toss()
	if prepared.Config().Human == ([2]bool{}) {
		g.startMatch(prepared.Start(false, 0))
		return
	}
	prepared.AdvanceMenuRandom()
	g.prematchInputReady = false
	g.prematchPadDirection = -1
	g.prematchStoreFields = nil
	g.openFeatureMenu(preMatchScreen, g.prematchSelection())
}

func (g *Game) prematchSelection() int {
	if g.toss.HumanChoice {
		return 30
	}
	return 34
}

func (g *Game) prematchCodes() []int {
	if g.toss.HumanChoice {
		return []int{30, 31, 32}
	}
	return []int{33, 34}
}

// A human winner chooses ends. A CPU winner is acknowledged by the primary
// human controller, matching7724 and the6B36 first-joystick wrapper.
func (g *Game) prematchController() int {
	if g.toss.HumanChoice {
		return g.toss.Winner
	}
	if g.prematch != nil && !g.prematch.Config().Human[0] {
		return 1
	}
	return 0
}

// The source compares the complete direction with its preceding poll at 6B66.
// Adding a second held axis creates an ignored diagonal; releasing it can
// produce a fresh cardinal choice even without a new key press.
func (g *Game) prematchKeyMove(x, y int) (dx, dy int) {
	direction := -1
	if x != 0 || y != 0 {
		direction = engine.OriginalDirectionIndex(engine.Vec2{X: float64(x), Y: float64(y)})
	}
	previous := g.prematchKeyDirection
	g.prematchKeyDirection = direction
	if !g.prematchInputReady || direction == previous || x != 0 && y != 0 {
		return 0, 0
	}
	return x, y
}

func (g *Game) updatePrematch(x, y int, clicked bool) {
	if g.prematch == nil {
		g.openPrematch(nil)
	}
	team := g.prematchController()
	cfg := g.prematch.Config()
	dx, dy, fire := g.menuPadInputForSlot(cfg, team)
	secondary := team == 1 && cfg.Human[0]
	left, right, up, down, button := ebiten.KeyArrowLeft, ebiten.KeyArrowRight, ebiten.KeyArrowUp, ebiten.KeyArrowDown, ebiten.KeyControlLeft
	if secondary {
		left, right, up, down, button = ebiten.KeyJ, ebiten.KeyL, ebiten.KeyI, ebiten.KeyK, ebiten.KeyControlRight
	}
	kx, ky := 0, 0
	aliases := !secondary && !cfg.Cooperative[team]
	if pressed(left) || aliases && pressed(ebiten.KeyA) {
		kx--
	}
	if pressed(right) || aliases && pressed(ebiten.KeyD) {
		kx++
	}
	if pressed(up) || aliases && pressed(ebiten.KeyW) {
		ky--
	}
	if pressed(down) || aliases && pressed(ebiten.KeyS) {
		ky++
	}
	if kdx, kdy := g.prematchKeyMove(kx, ky); kdx != 0 || kdy != 0 {
		dx, dy = kdx, kdy
	}
	fire = fire || g.menuActivate || inpututil.IsKeyJustPressed(button)
	if !secondary {
		fire = fire || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyZ)
	} else {
		fire = fire || inpututil.IsKeyJustPressed(ebiten.KeyO)
	}
	if !g.prematchInputReady {
		held := pressed(button) || g.prematchPadFireHeld || g.menuActivate
		if secondary {
			held = held || pressed(ebiten.KeyO)
		} else {
			held = held || pressed(ebiten.KeyEnter, ebiten.KeyZ)
		}
		if !held {
			g.prematchInputReady = true
		}
		return
	}
	if !fire && (dx != 0 || dy != 0) && (dx == 0 || dy == 0) {
		// 6B42 uses one linear selection value; a new cardinal direction
		// moves by one and stops at the declared bounds without wrapping.
		delta := dx + dy
		if delta < 0 {
			g.selection = max(g.prematchCodes()[0], g.selection-1)
		} else if delta > 0 {
			codes := g.prematchCodes()
			g.selection = min(codes[len(codes)-1], g.selection+1)
		}
	}
	if clicked {
		for _, r := range g.prematchMenu().Records {
			if r.Kind != "button" || x < r.X || x >= r.X+r.Width || y < r.Y+24 || y >= r.Y+24+r.Height {
				continue
			}
			for _, code := range g.prematchCodes() {
				if r.Selection == code {
					g.selection = code
					fire = true
				}
			}
		}
	}
	if fire {
		g.activatePrematch(g.selection)
	}
}

func (g *Game) activatePrematch(code int) {
	if g.prematch == nil {
		g.openPrematch(nil)
	}
	if g.toss.HumanChoice && (code == 30 || code == 31) {
		g.toss.ChooseEnd(code - 30)
		g.startConfiguredGame()
	} else if !g.toss.HumanChoice && code == 34 {
		g.startConfiguredGame()
	} else if g.toss.HumanChoice && code == 32 || !g.toss.HumanChoice && code == 33 {
		fields := g.prematchStoreFields
		g.prematch.AdvanceMenuRandom()
		g.openReplayStore("")
		if fields != nil {
			g.replayStoreFields = *fields
		}
		g.replayStoreReturn = preMatchScreen
	}
}

func (g *Game) returnToPrematch() {
	fields := g.replayStoreFields
	g.prematchStoreFields = &fields
	g.replayStoreReturn = replayHomeScreen
	if g.prematch != nil {
		g.prematch.AdvanceMenuRandom()
	}
	g.prematchInputReady = false
	g.prematchPadDirection = -1
	g.openFeatureMenu(preMatchScreen, g.prematchSelection())
}

func (g *Game) prematchMenu() originalMenu {
	menu := g.menus["ko6.men"]
	filtered := make([]menuRecord, 0, len(menu.Records))
	for _, r := range menu.Records {
		if r.Selection == 32 && !g.toss.HumanChoice || (r.Selection == 33 || r.Selection == 34) && g.toss.HumanChoice {
			continue
		}
		filtered = append(filtered, r)
	}
	menu.Records = filtered
	return menu
}

// The source formats each wind component as an arrow, integer speed and m\s.
func prematchWind(value int32, horizontal bool) string {
	direction := 2
	if value < 0 {
		direction = 1
		value = -value
	}
	if horizontal {
		direction = (direction ^ 3) + 2
	}
	speed := int32(int16(value>>6)) * 50 / 0x3000
	return fmt.Sprintf("%c%dm\\s", direction, speed)
}

func (g *Game) drawPrematch(dst *ebiten.Image) {
	if g.prematch == nil {
		return
	}
	cfg := g.prematch.Config()
	x, y := g.prematch.WindVector()
	winner := g.toss.Winner
	caption := ""
	if winner >= 0 {
		caption = strings.ToUpper(cfg.TeamNames[winner]) + " WINS THE TOSS"
	}
	labels := map[int]string{1: strings.ToUpper(cfg.TeamNames[0]), 2: strings.ToUpper(cfg.TeamNames[1]), 3: g.prematch.Referee().Name, 4: strings.ToUpper(cfg.Pitch.String()), 5: prematchWind(y, false), 6: prematchWind(x, true), 7: caption}
	g.drawOriginalMenuRecords(dst, g.prematchMenu(), func(code int) bool { return code == g.selection || !g.toss.HumanChoice && code == 30+g.toss.EndChoice }, labels)
	for team := 0; team < 2; team++ {
		design := g.teamKitDesign(team)
		if im := g.coloredKitAsset(fmt.Sprintf("skit%d.bl1", design.Style+1), design, 11, 12); im != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(18, float64(24+team*18))
			dst.DrawImage(im, op)
		}
	}
	if g.notice != "" {
		g.centered(dst, g.notice, 236, ink)
	}
}
