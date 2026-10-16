package game

import (
	"bytes"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/olivierh59500/kickoff2/assets"
	"github.com/olivierh59500/kickoff2/internal/engine"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
)

var ink = color.RGBA{224, 224, 224, 255}
var gold = color.RGBA{240, 208, 32, 255}
var panel = color.RGBA{24, 40, 24, 255}
var shadow = color.RGBA{0, 0, 0, 160}
var originalHUDColors = [2]color.RGBA{{255, 255, 0, 255}, {238, 0, 0, 255}}

// amigaDACChannel expands the high CMAP nibble to the original12-bit DAC.
// Extracted IFF colors use levels00,10,..F0; actual Amiga output uses00,11,..FF.
func amigaDACChannel(value uint8) uint8 { return (value >> 4) * 17 }

func (g *Game) asset(name string) *ebiten.Image {
	if im := g.images[name]; im != nil {
		return im
	}
	data, err := assets.Files.ReadFile("original/images/" + name + ".png")
	if err != nil {
		return nil
	}
	im, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	// Expand each stored four-bit color component to its display intensity.
	dac := image.NewNRGBA(im.Bounds())
	for y := im.Bounds().Min.Y; y < im.Bounds().Max.Y; y++ {
		for x := im.Bounds().Min.X; x < im.Bounds().Max.X; x++ {
			pixel := color.NRGBAModel.Convert(im.At(x, y)).(color.NRGBA)
			pixel.R, pixel.G, pixel.B = amigaDACChannel(pixel.R), amigaDACChannel(pixel.G), amigaDACChannel(pixel.B)
			dac.SetNRGBA(x, y, pixel)
		}
	}
	im = dac
	if name == "fonts" {
		// The disk font uses black ink on grey. Convert only its ink to opaque white
		// so the original glyph shape can be tinted against any menu background.
		dest := image.NewNRGBA(im.Bounds())
		for y := im.Bounds().Min.Y; y < im.Bounds().Max.Y; y++ {
			for x := im.Bounds().Min.X; x < im.Bounds().Max.X; x++ {
				r, _, _, _ := im.At(x, y).RGBA()
				if r < 0x4000 {
					dest.SetNRGBA(x, y, color.NRGBA{255, 255, 255, 255})
				}
			}
		}
		im = dest
	}
	result := ebiten.NewImageFromImage(im)
	g.images[name] = result
	return result
}

func glyph(c rune) (int, int, bool) {
	rows := []string{"ABCDEFGHIJKLMNOPQRST", "UVWXYZ0123456789()->", "<^v{}:`'/\\=?_.*!| #"}
	for y, row := range rows {
		for x, r := range row {
			if r == c {
				return x * 16, y * 8, true
			}
		}
	}
	return 0, 0, false
}

// text draws the original disk font with its original seven-pixel glyph height.
func (g *Game) text(dst *ebiten.Image, s string, x, y int, c color.Color) {
	font := g.asset("fonts")
	s = strings.ToUpper(s)
	if font == nil {
		ebitenutil.DebugPrintAt(dst, s, x, y)
		return
	}
	for _, r := range s {
		if r != ' ' {
			sx, sy, ok := glyph(r)
			if ok {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(float64(x), float64(y))
				op.ColorScale.ScaleWithColor(c)
				dst.DrawImage(font.SubImage(image.Rect(sx, sy, sx+8, sy+8)).(*ebiten.Image), op)
			}
		}
		x += 6
	}
}
func (g *Game) centered(dst *ebiten.Image, s string, y int, c color.Color) {
	g.text(dst, s, (dst.Bounds().Dx()-len(s)*6)/2, y, c)
}
func rect(dst *ebiten.Image, x, y, w, h float64, c color.Color) {
	vector.DrawFilledRect(dst, float32(x), float32(y), float32(w), float32(h), c, false)
}
func line(dst *ebiten.Image, x1, y1, x2, y2 float64, c color.Color) {
	vector.StrokeLine(dst, float32(x1), float32(y1), float32(x2), float32(y2), 1, c, false)
}

// Draw presents the exact disk title/artwork and the native match state.
func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{10, 16, 12, 255})
	if g.idleDemo != nil {
		screen.Fill(color.Black)
	}
	offset := g.fieldOffset()

	// Drawing into a subimage uses the parent coordinate origin. Render the native
	// frame separately to keep all menu, field, and touch hit regions identical.
	frame := g.images["frame"]
	if frame == nil {
		frame = ebiten.NewImage(320, 256)
		g.images["frame"] = frame
	}
	frame.Fill(panel)
	g.drawScreen(frame)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(offset), 0)
	screen.DrawImage(frame, op)
	if g.touch && g.idleDemo == nil {
		g.drawTouch(screen)
	}
	if g.smoke > 0 && g.ticks >= g.smoke && g.capture != "" && !g.captured {
		pixels := make([]byte, screen.Bounds().Dx()*screen.Bounds().Dy()*4)
		screen.ReadPixels(pixels)
		output := image.NewRGBA(screen.Bounds())
		copy(output.Pix, pixels)
		err := os.MkdirAll(filepath.Dir(g.capture), 0755)
		if err == nil {
			var file *os.File
			file, err = os.Create(g.capture)
			if err == nil {
				err = png.Encode(file, output)
				closeErr := file.Close()
				if err == nil {
					err = closeErr
				}
			}
		}
		g.captureErr = err
		g.captured = true
	}
}

func (g *Game) drawScreen(frame *ebiten.Image) {
	if g.idleDemo != nil {
		g.drawIdleDemo(frame)
		return
	}
	switch g.screen {
	case matchScreen:
		if g.Match.Phase == engine.HalfTime || g.Match.Phase == engine.FullTime {
			g.drawPeriodMenu(frame)
		} else {
			g.drawMatch(frame, g.Match.Teams, g.Match.Ball, g.camera)
		}
		if g.paused {
			g.drawPause(frame)
		}
	case replayScreen:
		if g.originalReplay != nil {
			g.drawOriginalReplay(frame)
		} else if len(g.replay) > 0 {
			i := min(g.replayIndex, len(g.replay)-1)
			s := g.replayAt(i)
			g.replaySource.ClockTicks = s.ClockTicks
			g.drawMatch(frame, s.Teams, s.Ball, s.Camera)
			rect(frame, 90, 6, 140, 15, shadow)
			g.centered(frame, "ACTION REPLAY", 10, gold)
			g.centered(frame, "F1 / FIRE TO SAVE", 239, ink)
		}
	case helpScreen:
		g.drawHelp(frame)
	case optionsScreen:
		g.drawOriginalMenu(frame, "ko12.men", g.selectedOption, map[int]string{102: g.refereeLabel()})
		g.text(frame, ">", 312, 55+g.selection*16, gold)
		g.centered(frame, "UP / DOWN SELECT : LEFT / RIGHT CHANGE", 244, ink)
	case kitScreen:
		g.drawKit(frame)
	case practiceScreen:
		g.drawFeatureMenu(frame)
	case controlsScreen:
		g.drawFeatureMenu(frame)
	case practiceSelectionScreen, preMatchScreen:
		g.drawFeatureMenu(frame)
	case squadScreen:
		g.drawSquad(frame)
	case tacticsScreen:
		g.drawOriginalMenu(frame, "ko13.men", func(code int) bool { return code == g.selection }, g.tacticMenuLabels())
	case skillScreen:
		g.drawFeatureMenu(frame)
	case internationalScreen:
		g.drawFeatureMenu(frame)
	case competitionScreen:
		g.drawCompetition(frame)
	case importScreen:
		g.drawImportMenu(frame)
	case replayHomeScreen, replayStoreScreen, replayBrowserScreen, replayEditScreen:
		g.drawReplayMenu(frame)
	default:
		g.drawTitle(frame)
	}

}

func (g *Game) drawTitle(dst *ebiten.Image) {
	g.drawOriginalMenu(dst, "ko1.men", func(code int) bool { return code == g.selection+1 }, map[int]string{5: "INT.\nFRIENDLY", 6: "ACTION\nREPLAY", 9: "SPECIAL\nEVENTS"})
	if g.notice != "" {
		g.centered(dst, g.notice, 235, gold)
	} else {
		g.centered(dst, "H FOR CONTROLS", 237, ink)
	}
}

func (g *Game) drawMenu(dst *ebiten.Image, title string, items []string, top, step int) {
	g.centered(dst, title, 20, gold)
	for i, item := range items {
		y := top + i*step
		if i == g.selection {
			rect(dst, 22, float64(y-3), 276, float64(step-1), color.RGBA{56, 88, 48, 255})
			g.text(dst, ">", 26, y, gold)
		}
		g.text(dst, item, 40, y, ink)
	}
}

func (g *Game) updateCamera() {
	if g.Match == nil {
		return
	}
	topLeft := g.Match.OriginalCameraOffset()
	g.camera = engine.Vec2{X: math.Floor(topLeft.X) + 160, Y: math.Floor(topLeft.Y) + 108}
}

func screenPoint(pos, camera engine.Vec2) (float64, float64) {
	return pos.X - camera.X + 160, pos.Y - camera.Y + 132
}

// animationTicks preserves the captured sprite/ball phase during rewind or slow
// replay. Older portable files without timestamps retain their prior behavior.
func (g *Game) animationTicks() int {
	if g.screen == replayScreen && len(g.replay) > 0 {
		frame := g.replayAt(min(g.replayIndex, len(g.replay)-1))
		if frame.RenderTicks != 0 {
			return frame.RenderTicks
		}
	}
	return g.ticks
}

func (g *Game) drawMatch(dst *ebiten.Image, teams [2]engine.Team, ball engine.Ball, camera engine.Vec2) {
	match := g.Match
	if g.screen == replayScreen {
		match = g.replaySource
	}
	if match == nil {
		return
	}
	if g.drawIndexedMatch(dst, match) {
		return
	}
	g.drawPitch(dst, camera)
	// FCCE integrates/draws the ball before FCE2 starts the actor passes.
	ballVisible := match.OriginalBallVisible()
	if g.screen == replayScreen && len(g.replay) > 0 {
		frame := g.replayAt(min(g.replayIndex, len(g.replay)-1))
		ballVisible = !frame.HasBallVisibility || frame.BallVisible
	}
	g.drawOriginalBall(dst, ball, camera, ballVisible)
	// FCDE draws officials after the ball and before either footballer pass.
	official, hasOfficial := match.OriginalOfficialVisual()
	if g.screen == replayScreen && len(g.replay) > 0 {
		frame := g.replayAt(min(g.replayIndex, len(g.replay)-1))
		official, hasOfficial = frame.Official, frame.HasOfficial
	}
	if hasOfficial {
		g.drawOfficial(dst, official, camera)
	}
	// 14A84 processes each eleven-player record in source team/shirt order.
	// Overlapping sprites retain that order instead of being sorted by field Y.
	for t := range teams {
		for _, p := range teams[t].Players {
			if p.SentOff || match.Config.Practice && t == 1 && !p.Keeper {
				continue
			}
			x, y := screenPoint(p.Pos, camera)
			if x < -20 || x > 340 || y < -20 || y > 265 {
				continue
			}
			g.drawPlayer(dst, p, t, x, y)
		}
	}
	// Source14B24/14BBC place one-pixel controller masks eight pixels below
	// the actor: FFFF for the primary and FC3F for its fixed partner.
	for team := range teams {
		if !match.Config.Human[team] {
			continue
		}
		for marker, index := range []int{teams[team].Controlled, match.PartnerPlayer(team)} {
			actorTeam := team
			if marker == 0 && teams[team].ControlledOpposing {
				actorTeam = 1 - team
			}
			if index < 0 || index >= len(teams[actorTeam].Players) || teams[actorTeam].Players[index].SentOff {
				continue
			}
			x, y := screenPoint(teams[actorTeam].Players[index].Pos, camera)
			x, y = math.Floor(x)-8, math.Floor(y)+8
			if y < 24 || y >= 248 {
				continue
			}
			if marker == 0 {
				rect(dst, x, y, 16, 1, color.Black)
			} else {
				rect(dst, x, y, 6, 1, color.Black)
				rect(dst, x+10, y, 6, 1, color.Black)
			}
		}
	}
	if marker, index, visible := match.OriginalPenaltyMarkerVisual(); visible {
		mx, my := screenPoint(marker, camera)
		if sheet := g.playerKitSheet(match.Restart.Team); sheet != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(math.Floor(mx)-8, math.Floor(my)-8)
			sx, sy := index%20*16, index/20*16
			dst.DrawImage(sheet.SubImage(image.Rect(sx, sy, sx+16, sy+16)).(*ebiten.Image), op)
		}
	}
	rect(dst, 0, 0, 320, 23, color.Black)
	if g.screen == matchScreen {
		g.drawOriginalPlayerView(dst)
	}
	message := ""
	if visible := match.PublishedMessage(); g.screen != replayScreen && visible != "" && !(match.Config.Practice && visible == "PRACTICE") {
		message = visible
	}
	if _, prompt := match.OriginalInjurySubstitutePrompt(); prompt && g.screen == matchScreen {
		message = "SELECT SUBSTITUTE"
	}
	if message != "" {
		// F7D2 replaces the clock at sourceY216 (+24 raster rows),
		// centered on character-column26 with integer half-length.
		g.text(dst, message, (26-len(message)/2)*6, 240, color.Black)
	} else if !match.Config.Practice {
		remaining := match.RemainingSeconds()
		minutes := fmt.Sprint(remaining / 60)
		g.text(dst, minutes, 123+(5-len(minutes))*6, 240, color.Black)
		g.text(dst, ":", 153, 240, color.Black)
		g.text(dst, fmt.Sprintf("%02d", remaining%60), 159, 240, color.Black)
	}
	scoreA := fmt.Sprint(teams[0].Score)
	g.text(dst, scoreA, 278-len(scoreA)*6, 232, originalHUDColors[0])
	scoreB := fmt.Sprint(teams[1].Score)
	g.text(dst, scoreB, 308-len(scoreB)*6, 232, originalHUDColors[1])

	if g.scanner {
		g.drawScanner(dst, teams, ball)
	}
	nameTeam, namePlayer := ball.LastTeam, ball.LastPlayer
	if nameTeam < 0 || nameTeam > 1 || namePlayer < 0 || namePlayer > 10 {
		nameTeam = 0
		namePlayer = teams[0].Controlled
		if match.Config.Human[1] && !match.Config.Human[0] {
			nameTeam = 1
			namePlayer = teams[1].Controlled
		}
		if teams[nameTeam].ControlledOpposing {
			nameTeam = 1 - nameTeam
		}
	}
	if namePlayer >= 0 && namePlayer < 11 {
		player := teams[nameTeam].Players[namePlayer]
		nameColor := originalHUDColors[nameTeam]
		g.text(dst, fmt.Sprintf("%d %s", player.Number, player.Name), 6, 232, nameColor)
	}

	if match.Config.Practice {
		g.centered(dst, "PRACTICE", 240, color.Black)
	}
	if g.screen != replayScreen && (match.Phase == engine.Kickoff || match.Phase == engine.SetPiece || match.Phase == engine.Shootout) {
		rect(dst, 44, 228, 232, 18, shadow)
		g.centered(dst, "AIM / HOLD FIRE / RELEASE", 233, ink)
	}
	if g.screen != replayScreen && match.Phase == engine.FullTime {
		rect(dst, 72, 115, 176, 47, panel)
		g.centered(dst, "FULL TIME", 124, gold)
		g.centered(dst, "ENTER / TAP TO CONTINUE", 146, ink)
	}
}

func (g *Game) drawOriginalBall(dst *ebiten.Image, ball engine.Ball, camera engine.Vec2, visible bool) {
	if !visible {
		return
	}
	im := g.asset("ball")
	if im == nil {
		return
	}
	bx, by := screenPoint(ball.Pos, camera)
	index, elevation, hasShadow := engine.OriginalBallSpriteLayout(ball.Height)
	draw := func(index int, x, y float64) {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(math.Floor(x)-8, math.Floor(y)-8)
		dst.DrawImage(im.SubImage(image.Rect(index*16, 0, index*16+16, 16)).(*ebiten.Image), op)
	}
	if hasShadow {
		draw(8, bx+float64(elevation), by-float64(elevation))
	}
	draw(index, bx, by-float64(elevation))
}

func (g *Game) drawPitch(dst *ebiten.Image, camera engine.Vec2) {
	dst.Fill(color.Black)
	rect(dst, 0, 24, 320, 224, color.RGBA{0, 160, 0, 255})
	match := g.Match
	if g.screen == replayScreen {
		match = g.replaySource
	}
	surface := []string{"nor", "sog", "wet", "art"}[int(match.Config.Pitch)]
	if pitch := g.asset("pitch." + surface); pitch != nil {
		// The reconstructed 80 by 96 tile ground includes its original sidelines,
		// goal nets, corner flags, stands, and advertisements. Engine coordinates
		// start at the upper-left field marking, 176 by 64 pixels into this image.
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(160-camera.X-176, 132-camera.Y-64)
		view := dst.SubImage(image.Rect(0, 24, 320, 248)).(*ebiten.Image)
		view.DrawImage(pitch, op)
	}
}

func (g *Game) drawPlayer(dst *ebiten.Image, p engine.Player, team int, x, y float64) {
	sheet := g.playerKitSheet(team)
	if p.Keeper {
		sheet = g.asset("keeper")
	}
	if sheet == nil {
		return
	}
	direction := int(math.Round((math.Atan2(p.Facing.Y, p.Facing.X)+math.Pi/2)/(math.Pi/4))+8) % 8
	animation := engine.OriginalRunningFrame(p.AnimationPhase, p.Keeper)
	index := direction*8 + animation
	if p.ActionTicks > 0 {
		index = 88 + direction
	}
	if phase := p.OriginalJumpFrame(); phase >= 0 {
		index = 100 + direction*4 + phase
	}
	if p.Keeper {
		index = direction*7 + animation%7
	}
	offsetY := -p.JumpHeight
	if special, offset, present := p.OriginalSpecialSprite(); present {
		index, offsetY = special, float64(offset)
	}
	columns := 20
	sx, sy := (index%columns)*16, (index/columns)*16
	if sy+16 > sheet.Bounds().Dy() {
		sy = 0
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(math.Floor(x)-8, math.Floor(y)-8+offsetY)
	dst.DrawImage(sheet.SubImage(image.Rect(sx, sy, sx+16, sy+16)).(*ebiten.Image), op)
}

func (g *Game) drawOfficial(dst *ebiten.Image, official engine.OriginalOfficialState, camera engine.Vec2) {
	// B5E4's global bank720 begins referee.png; its ordinary actions use
	// the same eight-direction, eight-pose runner layout as outfield actors.
	if official.State < 0 || official.State > 6 {
		return
	}
	sheet := g.asset("referee")
	if sheet == nil {
		return
	}
	x, y := screenPoint(engine.Vec2{X: float64(official.X) / 65536, Y: float64(official.Y) / 65536}, camera)
	if x < -20 || x > 340 || y < -20 || y > 265 {
		return
	}
	index := engine.OriginalOfficialSprite(official)
	sx, sy := index%20*16, index/20*16
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(math.Floor(x)-8, math.Floor(y)-8)
	dst.DrawImage(sheet.SubImage(image.Rect(sx, sy, sx+16, sy+16)).(*ebiten.Image), op)
}

func (g *Game) drawScanner(dst *ebiten.Image, teams [2]engine.Team, ball engine.Ball) {
	rect(dst, 8, 36, 56, 86, color.RGBA{0, 96, 0, 210})
	line(dst, 8, 36, 64, 36, color.Black)
	line(dst, 64, 36, 64, 122, color.Black)
	line(dst, 64, 122, 8, 122, color.Black)
	line(dst, 8, 122, 8, 36, color.Black)
	line(dst, 8, 79, 64, 79, color.Black)
	for t := range teams {
		c := color.Color(originalHUDColors[0])
		if t == 1 {
			c = originalHUDColors[1]
		}
		for _, p := range teams[t].Players {
			if !p.SentOff && (!func() bool {
				if g.screen == replayScreen {
					return g.replaySource.Config.Practice
				}
				return g.Match.Config.Practice
			}() || t == 0 || p.Keeper) {
				rect(dst, 9+p.Pos.X/engine.PitchWidth*54, 37+p.Pos.Y/engine.PitchHeight*84, 1, 1, c)
			}
		}
	}
	rect(dst, 9+ball.Pos.X/engine.PitchWidth*54, 37+ball.Pos.Y/engine.PitchHeight*84, 1, 1, color.White)
}

func (g *Game) drawPause(dst *ebiten.Image) {
	rect(dst, 52, 81, 216, 149, color.RGBA{0, 24, 0, 240})
	g.centered(dst, "PAUSED", 92, gold)
	g.centered(dst, "RESUME", 116, ink)
	g.centered(dst, "MAIN MENU", 149, ink)
	g.centered(dst, "SUB 12       SUB 14", 179, ink)
	if len(g.replay) > 0 {
		g.centered(dst, "ACTION REPLAY", 207, ink)
	}
}
func (g *Game) drawHelp(dst *ebiten.Image) {
	g.centered(dst, "CONTROLS", 18, gold)
	items := []string{"ARROWS / WASD : MOVE TEAM A", "Z / LEFT CTRL : FIRE", "I J K L / O : PLAYER 2", "CO-OP : P1 ARROWS / P3 WASD Q", "P4 : U B N COMMA / Y", "HOLD FIRE BEFORE BALL : TRAP", "AIM + RELEASE FIRE : PASS", "FIRE AFTER TOUCHING : SHOOT", "REVERSE DIRECTION : CHIP", "FIRE AWAY FROM BALL : TACKLE", "FIRE WITH HIGH BALL : HEADER", "DIRECTION AFTER SHOT : SWERVE", "1 - 4 : ASSIGNED TACTIC SLOT", "P / SPACE : PAUSE    X : SCANNER", "R / S AFTER GOAL : REPLAY", "C : CROWD    M : SOUND    F11 : FULL", "ESC : PAUSE THEN MAIN MENU", "TOUCH : STICK / FIRE / T1-T4"}
	for i, s := range items {
		g.text(dst, s, 12, 40+i*11, ink)
	}
	g.centered(dst, "ENTER OR TAP TO RETURN", 241, gold)
}
func (g *Game) drawTouch(dst *ebiten.Image) {
	rect(dst, 0, 0, 80, 256, panel)
	rect(dst, 400, 0, 80, 256, panel)
	selecting, injured, substituteTeam := false, false, 0
	if g.screen == matchScreen && g.Match != nil {
		_, _, selecting = g.Match.OriginalSubstitutionSelection()
		substituteTeam, injured = g.Match.OriginalInjurySubstitutePrompt()
	}
	if g.screen == matchScreen && g.Match != nil && g.Match.Phase != engine.HalfTime && g.Match.Phase != engine.FullTime {
		team := touchPrimaryTeam(g.Match.Config)
		if g.Match.Config.Human[team] {
			active := g.Match.OriginalTacticBank(team).Active
			for slot := 0; slot < 4; slot++ {
				x, y := 4+slot%2*36, 4+slot/2*30
				c := color.Color(color.RGBA{56, 88, 48, 255})
				if active == slot {
					c = gold
				}
				rect(dst, float64(x), float64(y), 34, 28, c)
				g.text(dst, tacticTouchLabel(slot), x+11, y+10, ink)
			}
		}
	}
	if g.crowdControlAvailable() {
		label := "CROWD ON"
		if g.Match.Config.CrowdMuted {
			label = "CROWD OFF"
		}
		rect(dst, 404, 126, 72, 19, color.RGBA{56, 88, 48, 255})
		g.text(dst, label, 405, 132, ink)
	}
	if g.screen == replayScreen {
		for _, button := range []struct {
			text string
			y    int
		}{{"REWIND", 64}, {"BACK", 100}, {"STEP", 136}} {
			rect(dst, 4, float64(button.y), 72, 28, color.RGBA{56, 88, 48, 255})
			g.text(dst, button.text, 10, button.y+10, ink)
		}
		label := "SLOW"
		if g.replaySlow {
			label = "NORMAL"
		}
		rect(dst, 404, 126, 72, 28, color.RGBA{56, 88, 48, 255})
		g.text(dst, label, 417, 136, ink)
	} else {
		g.text(dst, "MOVE", 25, 79, gold)
		cx, cy := 40.0, 162.0
		if g.stickActive {
			cx, cy = g.stickX, g.stickY
		}
		vector.StrokeCircle(dst, float32(cx), float32(cy), 28, 1, ink, false)
		vector.DrawFilledCircle(dst, float32(cx+g.touchMove.X*22), float32(cy+g.touchMove.Y*22), 9, gold, false)
	}
	for _, button := range []struct {
		s string
		y int
	}{{"PAUSE", 34}, {"SOUND", 70}, {"MENU", 106}, {"FIRE", 171}} {
		if g.screen == replayScreen && button.s == "FIRE" {
			button.s = "SAVE"
		}
		if selecting && button.s == "FIRE" {
			for i, label := range []string{"NEXT", "PREV"} {
				rect(dst, float64(404+i*36), 146, 35, 34, color.RGBA{56, 88, 48, 255})
				g.text(dst, label, 406+i*36, 158, ink)
			}
			rect(dst, 404, 192, 72, 34, color.RGBA{56, 88, 48, 255})
			g.text(dst, "SELECT", 411, 204, ink)
			continue
		}
		if injured && button.s == "FIRE" {
			for slot, y := range []int{146, 192} {
				c := color.Color(color.RGBA{56, 88, 48, 255})
				label := fmt.Sprintf("SUB %d", 12+2*slot)
				if g.Match.Teams[substituteTeam].BenchUsed[slot] {
					c, label = panel, "USED"
				}
				rect(dst, 404, float64(y), 72, 34, c)
				g.text(dst, label, 413, y+10, ink)
			}
			continue
		}
		h := 27.0
		if button.s == "FIRE" || button.s == "SAVE" {
			h = 80
		}
		c := color.Color(color.RGBA{56, 88, 48, 255})
		if button.s == "FIRE" && g.touchFire {
			c = gold
		}
		rect(dst, 404, float64(button.y-12), 72, h, c)
		g.text(dst, button.s, 425, button.y, ink)
	}
}

// runningFrames reproduces the original walking sequence at code address 0xAAF0.
var runningFrames = []int{0, 0, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 1, 1, 1, 2, 2, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 6, 6, 6, 6, 5, 5, 5, 6, 6, 7, 7}

func positionLabel(position int, team string) string {
	if position == 0 {
		return team + " : PLAY AS TEAM"
	}
	return fmt.Sprintf("%s : POSITION %d", team, position+1)
}
