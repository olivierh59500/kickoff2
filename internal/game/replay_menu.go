package game

import (
	"fmt"
	"image"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/exp/textinput"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type replayTextEditor struct {
	field  textinput.Field
	active bool
}

// cleanReplayText preserves the original printable font and bounded field width.
func cleanReplayText(value string) string {
	var result strings.Builder
	for _, r := range value {
		if r >= 32 && r <= 126 && result.Len() < 28 {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func (g *Game) beginReplayTyping(code int) {
	g.finishReplayTyping()
	index := code - 10
	if index < 0 || index >= len(g.replayStoreFields) {
		return
	}
	g.selection, g.replayTyping = code, true
	if g.touch {
		value := g.cleanReplayField(index, g.replayStoreFields[index])
		g.replayText.field.SetTextAndSelection(value, len(value), len(value))
		g.replayText.field.Focus()
		g.replayText.active = true
	}
}

// finishReplayTyping also runs before lifecycle suspension or contextual Back.
func (g *Game) finishReplayTyping() {
	wasTyping := g.replayTyping
	if g.replayText.active {
		index := g.selection - 10
		value := g.replayText.field.TextForRendering()
		committed := g.replayText.field.Text()
		g.replayText.field.Blur()
		// Blur drains a final queued commit. Keep it when it arrived after the
		// last Update; otherwise retain the visible composition being accepted.
		if latest := g.replayText.field.Text(); latest != committed {
			value = latest
		}
		if index >= 0 && index < len(g.replayStoreFields) {
			g.replayStoreFields[index] = g.cleanReplayField(index, value)
		}
		g.replayText.active = false
	}
	g.replayTyping = false
	if wasTyping && g.replayStoreReturn == preMatchScreen && g.prematch != nil {
		g.prematch.AdvanceMenuRandom()
	}
}

func (g *Game) replayTextBounds() image.Rectangle {
	for _, record := range g.menus["ko20.men"].Records {
		if record.Kind == "button" && record.Selection == g.selection {
			x := record.X + g.fieldOffset() + 2
			y := record.Y + 24 + (record.Height-8)/2
			return image.Rect(x, y, x+max(1, record.Width-4), y+8)
		}
	}
	return image.Rect(g.fieldOffset()+20, 50, g.fieldOffset()+300, 58)
}

func (g *Game) replayMenuSource() string {
	switch g.screen {
	case replayHomeScreen:
		return "ko19.men"
	case replayStoreScreen:
		return "ko20.men"
	case replayEditScreen:
		return "ko21.men"
	case replayBrowserScreen:
		if g.replayBrowserGolden {
			return "ko24.men"
		}
		return "ko22.men"
	}
	return ""
}

func (g *Game) openReplayHome() {
	g.finishReplayTyping()
	if g.replayLength() == 0 {
		g.loadReplay()
	}
	g.openFeatureMenu(replayHomeScreen, 1)
	if err := g.loadReplayCatalog(); err != nil {
		g.notice = err.Error()
	}
}

func (g *Game) openReplayStore(clipID string) {
	g.replayStoreReturn = replayHomeScreen
	g.finishReplayTyping()
	g.replayEditClip = clipID
	g.replayStoreOriginal = clipID == "" && (g.originalReplay != nil || g.recordedMovie != nil)
	g.replayStoreFields = [6]string{fmt.Sprintf("REPLAY %03d", len(g.replayCatalog.Clips)+1), time.Now().Format("2006-01-02"), "TEAM A", "TEAM B", "PLAYER1", "PLAYER2"}
	for _, clip := range g.replayCatalog.Clips {
		if clip.ID == clipID {
			g.replayStoreOriginal = clip.Original
			g.replayStoreFields = [6]string{clip.Title, clip.Date, clip.Teams[0], clip.Teams[1], clip.Players[0], clip.Players[1]}
		}
	}
	if clipID == "" && g.replaySource != nil {
		g.replayStoreFields[2], g.replayStoreFields[3] = g.replaySource.Teams[0].Name, g.replaySource.Teams[1].Name
	}
	if clipID == "" && g.originalReplay != nil {
		g.replayStoreFields = g.originalReplay.Metadata().Fields
	}
	if clipID == "" && g.originalReplay == nil && g.recordedMovie != nil {
		g.replayStoreFields = g.recordedMovie.Metadata().Fields
	}
	g.replayTyping = false
	g.openFeatureMenu(replayStoreScreen, 10)
}

func (g *Game) selectedLibraryClip() (replayClip, bool) {
	if len(g.replayCatalog.Clips) == 0 {
		return replayClip{}, false
	}
	g.replayLibraryIndex = max(0, min(g.replayLibraryIndex, len(g.replayCatalog.Clips)-1))
	return g.replayCatalog.Clips[g.replayLibraryIndex], true
}

func (g *Game) toggleGoldenClip(id string) {
	for i, selected := range g.goldenSelection {
		if selected == id {
			g.goldenSelection = append(g.goldenSelection[:i], g.goldenSelection[i+1:]...)
			return
		}
	}
	if len(g.goldenSelection) < 64 {
		g.goldenSelection = append(g.goldenSelection, id)
	}
}

func (g *Game) moveGoldenClip(id string, delta int) {
	for i, selected := range g.goldenSelection {
		if selected == id {
			j := max(0, min(len(g.goldenSelection)-1, i+delta))
			g.goldenSelection[i], g.goldenSelection[j] = g.goldenSelection[j], g.goldenSelection[i]
			return
		}
	}
}

func (g *Game) updateReplayMenu(x, y int, clicked bool) bool {
	source := g.replayMenuSource()
	if source == "" {
		return false
	}
	if g.screen != replayStoreScreen && (inpututil.IsKeyJustPressed(ebiten.KeyI) || clicked && y >= 238 && y < 254 && x >= 4 && x < 160 && g.screen != replayEditScreen) {
		g.openImport("replay", 0, replayHomeScreen)
		return true
	}
	if g.screen == replayBrowserScreen && !g.replayBrowserGolden && (inpututil.IsKeyJustPressed(ebiten.KeyE) || clicked && y >= 238 && y < 254 && x >= 164 && x < 316) {
		if clip, ok := g.selectedLibraryClip(); ok && (clip.Original || clip.Recorded) {
			if err := g.exportOriginalLibraryClip(clip.ID); err != nil {
				g.notice = err.Error()
			}
		}
		return true
	}
	if g.screen == replayStoreScreen && g.replayTyping {
		index := g.selection - 10
		if index >= 0 && index < 6 {
			if g.replayText.active {
				if _, err := g.replayText.field.HandleInputWithBounds(g.replayTextBounds()); err != nil {
					g.notice = err.Error()
					g.finishReplayTyping()
					return true
				}
				g.replayStoreFields[index] = g.cleanReplayField(index, g.replayText.field.TextForRendering())
			} else {
				for _, r := range ebiten.AppendInputChars(nil) {
					if r >= 32 && (r <= 126 || g.replayStoreOriginal && r <= 255) {
						g.replayStoreFields[index] = g.cleanReplayField(index, g.replayStoreFields[index]+string(r))
					}
				}
				if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(g.replayStoreFields[index]) > 0 {
					value := g.replayStoreFields[index]
					_, size := utf8.DecodeLastRuneInString(value)
					g.replayStoreFields[index] = value[:len(value)-size]
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || g.menuActivate || clicked || g.replayText.active && !g.replayText.field.IsFocused() {
			g.finishReplayTyping()
			g.menuActivate = false
			if clicked {
				code := g.originalHit(source, x, y)
				if code >= 10 && code <= 15 {
					g.beginReplayTyping(code)
				} else if code == 9 || code == 16 {
					g.activateReplayMenu(code)
				}
			}
		}
		return true
	}
	codes := []int{1, 2, 3, 4, 5}
	switch g.screen {
	case replayStoreScreen:
		codes = []int{10, 11, 12, 13, 14, 15, 9, 16}
	case replayEditScreen:
		codes = []int{1, 2, 3, 4, 5, 6, 8}
	case replayBrowserScreen:
		codes = []int{1, 2, 3, 5}
		if g.replayBrowserGolden {
			codes = []int{1, 2, 3, 10}
		}
	}
	dx, dy, fire := g.menuPadInput()
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		dx = -1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		dx = 1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		dy = -1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		dy = 1
	}
	if g.screen == replayEditScreen {
		if clip, ok := g.selectedLibraryClip(); ok {
			if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
				g.toggleGoldenClip(clip.ID)
			}
			if dy != 0 {
				g.moveGoldenClip(clip.ID, dy)
				dy = 0
			}
			if inpututil.IsKeyJustPressed(ebiten.KeyE) {
				g.openReplayStore(clip.ID)
				return true
			}
		}
	}
	if g.screen == replayBrowserScreen && g.replayBrowserGolden && len(g.replayCatalog.Golden) > 0 {
		if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) {
			g.replayLibraryIndex = (g.replayLibraryIndex + 1) % len(g.replayCatalog.Golden)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
			g.replayLibraryIndex = (g.replayLibraryIndex + len(g.replayCatalog.Golden) - 1) % len(g.replayCatalog.Golden)
		}
	}
	if dx != 0 || dy != 0 {
		g.navigateOriginal(source, codes, dx, dy)
	}
	if clicked {
		code := g.originalHit(source, x, y)
		for _, allowed := range codes {
			if code == allowed {
				g.selection, fire = code, true
			}
		}
		if g.screen == replayEditScreen && x >= 4 && x < 316 && y >= 224 && y < 237 {
			if clip, ok := g.selectedLibraryClip(); ok {
				g.toggleGoldenClip(clip.ID)
			}
		}
		if g.screen == replayEditScreen && y >= 237 && y < 250 && x >= 176 && x < 240 {
			if clip, ok := g.selectedLibraryClip(); ok {
				g.openReplayStore(clip.ID)
				return true
			}
		}
		if g.screen == replayEditScreen && y >= 237 && y < 250 && x >= 244 && x < 316 {
			if clip, ok := g.selectedLibraryClip(); ok {
				delta := -1
				if x >= 280 {
					delta = 1
				}
				g.moveGoldenClip(clip.ID, delta)
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) || inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && g.screen == replayBrowserScreen {
		if clip, ok := g.selectedLibraryClip(); ok && !g.replayBrowserGolden {
			if err := g.deleteLibraryClip(clip.ID); err != nil {
				g.notice = err.Error()
			} else {
				g.notice = "CLIP DELETED"
			}
		}
	}
	if fire || g.menuActivate || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyZ) {
		g.activateReplayMenu(g.selection)
	}
	return true
}

func (g *Game) activateReplayMenu(code int) {
	switch g.screen {
	case replayHomeScreen:
		switch code {
		case 1:
			if g.replayLength() == 0 {
				g.notice = "NO REPLAY RECORDED"
			} else {
				g.openReplayStore("")
			}
		case 2, 3:
			g.replayBrowserGolden = code == 3
			g.replayLibraryIndex = 0
			g.openFeatureMenu(replayBrowserScreen, 2)
		case 4:
			g.goldenSelection = nil
			g.replayBrowserGolden = false
			g.replayLibraryIndex = 0
			g.openFeatureMenu(replayEditScreen, 2)
		case 5:
			g.Back()
		}
	case replayStoreScreen:
		switch {
		case code >= 10 && code <= 15:
			g.beginReplayTyping(code)
		case code == 9:
			if err := g.deleteLibraryClip(g.replayEditClip); err != nil {
				g.notice = err.Error()
			} else {
				g.openReplayHome()
			}
		case code == 16:
			if g.replayStoreReturn == preMatchScreen {
				g.finishReplayTyping()
				g.returnToPrematch()
				return
			}
			if g.replayEditClip == "" {
				if _, err := g.storeReplayClip(g.replayStoreFields); err != nil {
					g.notice = err.Error()
					return
				}
			} else {
				if g.replayStoreOriginal {
					if err := g.editOriginalReplayMetadata(g.replayEditClip, g.replayStoreFields); err != nil {
						g.notice = err.Error()
						return
					}
				}
				for i := range g.replayCatalog.Clips {
					clip := &g.replayCatalog.Clips[i]
					if clip.ID == g.replayEditClip {
						clip.Title, clip.Date = g.replayStoreFields[0], g.replayStoreFields[1]
						clip.Teams, clip.Players = [2]string{g.replayStoreFields[2], g.replayStoreFields[3]}, [2]string{g.replayStoreFields[4], g.replayStoreFields[5]}
					}
				}
				if err := g.saveReplayCatalog(); err != nil {
					g.notice = err.Error()
					return
				}
			}
			g.openReplayHome()
			g.notice = "REPLAY STORED"
		}
	case replayEditScreen:
		switch code {
		case 1:
			g.replayLibraryIndex = max(0, g.replayLibraryIndex-1)
		case 3:
			g.replayLibraryIndex = min(max(0, len(g.replayCatalog.Clips)-1), g.replayLibraryIndex+1)
		case 2:
			if clip, ok := g.selectedLibraryClip(); ok {
				if err := g.startReplayPlaylist([]string{clip.ID}, replayEditScreen); err != nil {
					g.notice = err.Error()
				}
			}
		case 4:
			if clip, ok := g.selectedLibraryClip(); ok {
				g.toggleGoldenClip(clip.ID)
			}
		case 5:
			if err := g.saveGoldenGoals("", g.goldenSelection); err != nil {
				g.notice = err.Error()
			} else {
				g.notice = "GOLDEN GOALS SAVED"
			}
		case 6:
			g.replayBrowserGolden = true
			g.replayLibraryIndex = 0
			g.openFeatureMenu(replayBrowserScreen, 2)
		case 8:
			g.openReplayHome()
		}
	case replayBrowserScreen:
		if g.replayBrowserGolden {
			if len(g.replayCatalog.Golden) == 0 {
				g.notice = "NO GOLDEN GOALS SAVED"
				return
			}
			g.replayLibraryIndex = max(0, min(g.replayLibraryIndex, len(g.replayCatalog.Golden)-1))
			golden := g.replayCatalog.Golden[g.replayLibraryIndex]
			switch code {
			case 10:
				g.replayLibraryIndex = (g.replayLibraryIndex + 1) % len(g.replayCatalog.Golden)
			case 1:
				if err := g.startReplayPlaylist(golden.Clips, replayBrowserScreen); err != nil {
					g.notice = err.Error()
				}
			case 2:
				g.goldenSelection = append([]string(nil), golden.Clips...)
				g.replayLibraryIndex = 0
				g.openFeatureMenu(replayEditScreen, 2)
			case 3:
				g.openReplayHome()
			}
			return
		}
		switch code {
		case 1:
			g.replayLibraryIndex = max(0, g.replayLibraryIndex-1)
		case 3:
			g.replayLibraryIndex = min(max(0, len(g.replayCatalog.Clips)-1), g.replayLibraryIndex+1)
		case 2:
			if clip, ok := g.selectedLibraryClip(); ok {
				if err := g.startReplayPlaylist([]string{clip.ID}, replayBrowserScreen); err != nil {
					g.notice = err.Error()
				}
			} else {
				g.notice = "NO MATCH REPLAYS SAVED"
			}
		case 5:
			g.openReplayHome()
		}
	}
}

func (g *Game) drawReplayMenu(dst *ebiten.Image) {
	labels := map[int]string{}
	if g.screen == replayStoreScreen {
		for i, field := range g.replayStoreFields {
			labels[10+i] = field
		}
		labels[16] = "STORE AND RETURN"
		if g.replayStoreReturn == preMatchScreen {
			labels[16] = "QUIT"
		}
		if g.replayTyping {
			labels[g.selection] += "_"
		}
	} else if g.screen == replayBrowserScreen || g.screen == replayEditScreen {
		if g.replayBrowserGolden && g.screen == replayBrowserScreen {
			labels[5] = "QUIT"
			if len(g.replayCatalog.Golden) > 0 {
				index := max(0, min(g.replayLibraryIndex, len(g.replayCatalog.Golden)-1))
				golden := g.replayCatalog.Golden[index]
				labels[10], labels[16] = golden.Title, fmt.Sprintf("%d CLIPS", len(golden.Clips))
			}
		} else if clip, ok := g.selectedLibraryClip(); ok {
			labels[10], labels[11] = clip.Title, clip.Date
			labels[12], labels[13], labels[14], labels[15] = clip.Teams[0], clip.Teams[1], clip.Players[0], clip.Players[1]
			labels[16] = fmt.Sprintf("%d / %d", g.replayLibraryIndex+1, len(g.replayCatalog.Clips))
		}
	}
	if g.screen == replayEditScreen {
		labels[4], labels[5], labels[6] = "ADD/REMOVE\nCLIP", "SAVE\nGOLDEN GOALS", "LOAD\nGOLDEN GOALS"
	}
	g.drawOriginalMenu(dst, g.replayMenuSource(), func(code int) bool { return code == g.selection }, labels)
	if g.screen == replayEditScreen {
		position := 0
		if clip, ok := g.selectedLibraryClip(); ok {
			for i, id := range g.goldenSelection {
				if id == clip.ID {
					position = i + 1
				}
			}
		}
		g.centered(dst, fmt.Sprintf("CLIP %d OF %d : TAP HERE TO ADD / REMOVE", position, len(g.goldenSelection)), 226, gold)
		g.text(dst, "SPACE SELECT", 7, 240, ink)
		rect(dst, 176, 237, 64, 13, gold)
		g.text(dst, "DETAILS", 187, 240, panel)
		rect(dst, 244, 237, 34, 13, gold)
		rect(dst, 280, 237, 34, 13, gold)
		g.text(dst, "UP", 254, 240, panel)
		g.text(dst, "DOWN", 285, 240, panel)
	} else if g.notice != "" {
		g.centered(dst, g.notice, 238, gold)
	} else if g.screen == replayBrowserScreen && g.replayBrowserGolden {
		g.centered(dst, "CLICK FILE NAME / PAGE UP-DOWN TO SELECT", 238, ink)
	}
	if g.screen == replayHomeScreen || g.screen == replayBrowserScreen {
		rect(dst, 4, 238, 156, 16, panel)
		g.text(dst, "IMPORT ORIGINAL [I]", 9, 242, ink)
		if g.screen == replayBrowserScreen && !g.replayBrowserGolden {
			if clip, ok := g.selectedLibraryClip(); ok && (clip.Original || clip.Recorded) {
				rect(dst, 164, 238, 152, 16, panel)
				g.text(dst, "EXPORT ORIGINAL [E]", 168, 242, ink)
			}
		}
	}
}

func (g *Game) replayFieldLimit(index int) int {
	if g.replayStoreOriginal {
		if index == 0 {
			return 8
		}
		return 9
	}
	return 28
}

func (g *Game) cleanReplayField(index int, value string) string {
	if !g.replayStoreOriginal {
		return cleanReplayText(value)
	}
	var result strings.Builder
	count := 0
	for _, character := range value {
		if character >= 32 && character <= 255 && count < g.replayFieldLimit(index) {
			result.WriteRune(character)
			count++
		}
	}
	return result.String()
}
