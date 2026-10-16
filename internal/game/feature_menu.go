package game

import (
	"fmt"
	"image/color"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/kickoff2/internal/engine"
)

// featureMenuSource associates each menu screen with its layout record
// file. Selection numbers remain the original values used by the 68000 menus.
func (g *Game) featureMenuSource() string {
	switch g.screen {
	case kitScreen:
		return "ko11.men"
	case practiceScreen:
		return "ko2.men"
	case practiceSelectionScreen:
		return "ko4.men"
	case controlsScreen:
		return "ko3.men"
	case internationalScreen:
		return "ko10.men"
	case skillScreen:
		return "ko8.men"
	case preMatchScreen:
		return "ko6.men"
	case tacticsScreen:
		return "ko13.men"
	}
	return ""
}

func (g *Game) featureMenuCodes() []int {
	switch g.screen {
	case kitScreen:
		return []int{1, 2, 3, 4, 5, 6, 7, 8, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	case practiceScreen:
		return []int{1, 2}
	case practiceSelectionScreen:
		return []int{11, 12, 21, 22}
	case controlsScreen:
		return []int{1, 2, 3, 4}
	case internationalScreen:
		return []int{10, 11, 12, 13, 14, 15, 16, 17, 1, 2, 18}
	case skillScreen:
		return []int{2, 3, 4, 5, 6, 7, 8}
	case preMatchScreen:
		return g.prematchCodes()
	case tacticsScreen:
		return g.tacticMenuCodes()
	}
	return nil
}

// navigateOriginal finds the closest selectable rectangle in the requested
// direction, allowing keyboards and controllers to follow the original layout.
func (g *Game) navigateOriginal(source string, codes []int, dx, dy int) {
	allowed := map[int]bool{}
	for _, code := range codes {
		allowed[code] = true
	}
	var current menuRecord
	found := false
	for _, r := range g.menus[source].Records {
		if r.Kind == "button" && r.Selection == g.selection {
			current, found = r, true
			break
		}
	}
	if !found {
		g.selection = codes[0]
		return
	}
	cx, cy := current.X+current.Width/2, current.Y+current.Height/2
	best, score := g.selection, int(^uint(0)>>1)
	for _, r := range g.menus[source].Records {
		if r.Kind != "button" || !allowed[r.Selection] || r.Selection == g.selection {
			continue
		}
		x, y := r.X+r.Width/2-cx, r.Y+r.Height/2-cy
		along, across := x*dx+y*dy, x*dy-y*dx
		if along <= 0 {
			continue
		}
		candidate := along*along + 4*across*across
		if candidate < score {
			best, score = r.Selection, candidate
		}
	}
	g.selection = best
}

func (g *Game) openFeatureMenu(next screen, selection int) {
	g.screen, g.selection = next, selection
	g.notice = ""
	g.CancelInput()
}

// openOriginalPractice follows the live main-menu dispatch at0x587e. The disk
// also contains ko2.men, but that unused drills menu is not on this route.
func (g *Game) openOriginalPractice() {
	g.tournament = nil
	g.practiceMode, g.practicePlayers = 0, 1
	g.openFeatureMenu(practiceSelectionScreen, 11)
}

func (g *Game) updateFeatureMenu(x, y int, clicked bool) bool {
	if g.screen == preMatchScreen {
		g.updatePrematch(x, y, clicked)
		return true
	}
	if g.screen == squadScreen {
		g.updateSquad(x, y, clicked)
		return true
	}
	source := g.featureMenuSource()
	if source == "" {
		return false
	}
	padX, padY, padFire := g.menuPadInput()
	dx, dy := padX, padY
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || inpututil.IsKeyJustPressed(ebiten.KeyA) {
		dx = -1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || inpututil.IsKeyJustPressed(ebiten.KeyD) {
		dx = 1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) {
		dy = -1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) {
		dy = 1
	}
	activate := padFire || g.menuActivate || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyControlLeft)
	delta := 1
	if dx != 0 || dy != 0 {
		if g.screen == kitScreen && (g.selection == 7 || g.selection == 8) && dx != 0 {
			activate, delta = true, dx
		} else {
			g.navigateOriginal(source, g.featureMenuCodes(), dx, dy)
		}
	}
	if clicked {
		if g.screen == kitScreen && x >= 14 && x < 294 && y >= 217 && y < 227 {
			if g.kitTeam < 0 {
				g.kitTeam = 0
			}
			index := (x - 14) / 14
			if g.selection == 8 {
				g.kits[g.kitTeam].Shorts = index
			} else {
				g.kits[g.kitTeam].Shirt = index
				g.selection = 7
			}
			g.clearColoredKitCache()
			g.saveSettings()
			return true
		}
		code := g.originalHit(source, x, y)
		for _, allowed := range g.featureMenuCodes() {
			if code == allowed {
				g.selection, activate = code, true
				break
			}
		}
	}
	if g.screen == practiceScreen && inpututil.IsKeyJustPressed(ebiten.KeyC) {
		g.practiceMode = 2
		g.practicePlayers = 1
		g.openFeatureMenu(practiceSelectionScreen, 11)
		return true
	}
	if activate {
		g.activateFeature(g.selection, delta)
	}
	return true
}

func (g *Game) activateFeature(code, delta int) {
	switch g.screen {
	case tacticsScreen:
		g.activateTactic(code)
	case kitScreen:
		g.activateKit(code, delta)
	case practiceScreen:
		g.practiceMode = code - 1
		g.practicePlayers = 1
		g.openFeatureMenu(practiceSelectionScreen, 11)
	case practiceSelectionScreen:
		switch code {
		case 11:
			g.practicePlayers = 1
			g.selection = 21
		case 12:
			g.practicePlayers = 2
			g.selection = 21
		case 21, 22:
			g.rememberSquadBindingPorts()
			g.practiceDown = code == 22
			g.setupPlayers = g.practicePlayers
			g.config.Human = [2]bool{true, false}
			g.config.Cooperative = [2]bool{g.practicePlayers == 2, false}
			g.initializePartners()
			g.openSquad(0, preMatchScreen)
		}
	case controlsScreen:
		g.rememberSquadBindingPorts()
		g.setupPlayers = code
		g.config.Human = [2]bool{true, code >= 2}
		g.config.Cooperative = [2]bool{code >= 3, code == 4}
		g.initializePartners()
		g.openSquad(0, preMatchScreen)
	case internationalScreen:
		switch {
		case code == 1 || code == 2:
			g.internationalTeam = code - 1
		case code >= 10 && code <= 17:
			team := g.internationalTeam
			g.config.TeamNames[team] = engine.InternationalCountries[code-10]
			g.config.SquadFiles[team] = engine.InternationalSquad(g.config.TeamNames[team])
			g.config.UserSquads[team] = nil
			g.resetSelectedSquad(team)
			g.resetSquadBindings(team)
			g.internationalTeam = 1 - team
		case code == 18:
			g.practiceMode = -1
			g.openFeatureMenu(controlsScreen, 1)
		}
	case skillScreen:
		if code == 7 {
			g.openImport("sqd", g.optionTeam, g.skillReturn)
			return
		}
		if code >= 2 && code <= 6 {
			g.config.Skill[g.optionTeam] = 6 - code
			g.config.SquadFiles[g.optionTeam] = ""
			g.config.UserSquads[g.optionTeam] = nil
			g.resetSelectedSquad(g.optionTeam)
			g.resetSquadBindings(g.optionTeam)
		}
		g.saveSettings()
		if g.skillReturn == squadScreen {
			g.openSquad(g.optionTeam, g.squadReturn)
		} else {
			g.openFeatureMenu(optionsScreen, 0)
		}
	case preMatchScreen:
		g.activatePrematch(code)
	}
}

func (g *Game) startConfiguredGame() {
	practice := g.practiceMode >= 0
	if !practice {
		if g.prematch == nil {
			g.openPrematch(nil)
			if g.prematch == nil {
				return
			}
		}
		g.startMatch(g.prematch.Start(g.toss.TeamADown, g.toss.InitialKickoff))
	} else {
		g.config.StartDownfield = g.practiceDown
		cfg := g.config
		cfg.Practice = g.practiceMode != 1
		cfg.Human[0] = true
		prepared := g.prepareMenuMatch(cfg)
		g.startMatch(prepared.Start(g.practiceDown, 0))
		if g.practiceMode == 1 {
			g.Match.StartShootout()
		}
		if g.practiceMode == 2 {
			g.Match.PracticeRestart(engine.Corner)
		}
	}
	g.saveSettings()
}

func (g *Game) initializePartners() {
	for team := range g.config.Cooperative {
		if g.config.HasSquadBindings[team] {
			// Source 70CE filters a country only when its squad is opened.
			// A skipped computer team's assignments survive for a later visit.
			g.syncSquadBindingSlotsForTeam(team)
			continue
		}
		if g.config.PartnerPlayer[team] < 1 || g.config.PartnerPlayer[team] > 10 {
			g.config.PartnerPlayer[team] = 0
		}
		if g.config.Cooperative[team] && g.config.PartnerPlayer[team] == g.config.FixedPlayer[team] {
			g.config.FixedPlayer[team] = 0
		}
	}
}

func (g *Game) drawFeatureMenu(dst *ebiten.Image) {
	if g.screen == preMatchScreen {
		g.drawPrematch(dst)
		return
	}
	labels := map[int]string{}
	switch g.screen {
	case internationalScreen:
		for i, country := range engine.InternationalCountries {
			labels[10+i] = country
		}
		labels[1], labels[2] = g.config.TeamNames[0], g.config.TeamNames[1]
	case skillScreen:
		labels[8] = [...]string{"TEAM A", "TEAM B"}[g.optionTeam]
	}
	g.drawOriginalMenu(dst, g.featureMenuSource(), func(code int) bool {
		if g.screen == practiceSelectionScreen {
			// 06BF6 complements the narrow row cursor, while 06BC8/824A
			// complement only the outline of each selected row value.
			return code == 10 && g.selection < 20 || code == 20 && g.selection >= 20
		}
		if g.screen == internationalScreen {
			return code == g.selection || code == g.internationalTeam+1
		}
		return code == g.selection
	}, labels)
	if g.notice != "" {
		g.centered(dst, g.notice, 236, color.White)
	}
}

func (g *Game) squadFile(team int) string {
	if g.config.SquadFiles[team] != "" {
		return g.config.SquadFiles[team]
	}
	if name := engine.InternationalSquad(g.config.TeamNames[team]); name != "" {
		return name
	}
	return [...]string{"div4.sqd", "div3.sqd", "div2.sqd", "div1.sqd", "intern.sqd"}[max(0, min(4, g.config.Skill[team]))]
}

func (g *Game) squadPool() []engine.SquadPlayer {
	squad, err := engine.OriginalSquad(g.squadFile(g.squadTeam))
	if len(g.config.UserSquads[g.squadTeam]) != 0 {
		squad, err = engine.DecodeSquad(g.config.UserSquads[g.squadTeam])
	}
	if err != nil {
		g.notice = err.Error()
	}
	if g.squadSort != 0 {
		shirt := func(recordIndex int) int {
			if slot := g.selectedSquadRecordSlot(g.squadTeam, recordIndex); slot >= 0 {
				return slot
			}
			return 99
		}
		sort.SliceStable(squad.Players, func(i, j int) bool {
			left, right := squad.Players[i], squad.Players[j]
			switch g.squadSort {
			case 1:
				return left.Name < right.Name
			case 2:
				return left.Role < right.Role
			case 3:
				return shirt(left.RecordIndex) < shirt(right.RecordIndex)
			}
			return false
		})
	}
	return squad.Players
}

func (g *Game) openSquad(team int, returnTo screen) {
	g.squadTeam, g.squadReturn = team, returnTo
	g.squadCursor, g.squadSlot, g.squadPending = -1, 0, -1
	g.squadSort = 0
	g.openFeatureMenu(squadScreen, 30)
	pool := g.squadPool()
	records, valid := engine.CanonicalSquadSelection(pool, g.config.SelectedSquadRecords[team], g.config.HasSelectedSquadRecords[team], g.config.SelectedSquadIDs[team])
	if !valid {
		g.resetSelectedSquad(team)
		g.resetSquadBindings(team)
		records, _ = engine.CanonicalSquadSelection(pool, [13]uint8{}, false, [13]uint32{})
	}
	g.config.SelectedSquadRecords[team] = records
	g.config.HasSelectedSquadRecords[team] = true
	g.syncSelectedSquadIDs(pool)
	g.initializeSquadBindings(pool)
	// Source 7246..727A rebuilds the cached controller word every time 70CE
	// opens the squad screen, after filtering the complete player pool.
	g.rebuildSquadControllerMask()
}

// assignSquadPlayer follows744A: clear the target occupant, then move the
// selected record and its retained binding. Its former shirt becomes vacant.
func (g *Game) assignSquadPlayer(poolIndex, slot int) {
	pool := g.squadPool()
	if poolIndex < 0 || poolIndex >= len(pool) || slot < 0 || slot >= 13 {
		return
	}
	selected := &g.config.SelectedSquadRecords[g.squadTeam]
	bindings := &g.config.SquadBindings[g.squadTeam]
	record := uint8(pool[poolIndex].RecordIndex + 1)
	if occupant := selected[slot]; occupant > 0 && int(occupant) <= len(g.config.UnassignedSquadControllers[g.squadTeam]) {
		g.config.UnassignedSquadControllers[g.squadTeam][occupant-1] = 0
	}
	bindings[slot] = 0 // Reassigning the same record also clears its binding.
	binding := g.squadRoleForController(g.config.UnassignedSquadControllers[g.squadTeam][pool[poolIndex].RecordIndex])
	g.config.UnassignedSquadControllers[g.squadTeam][pool[poolIndex].RecordIndex] = 0
	for i, existing := range selected {
		if existing == record && i != slot {
			binding, bindings[i] = bindings[i], 0
			selected[i] = 0
			break
		}
	}
	selected[slot] = record
	bindings[slot] = binding
	g.syncSelectedSquadIDs(pool)
	g.syncSquadBindingSlots()
	g.squadPending = -1
	g.notice = ""
	g.saveSettings()
}

func (g *Game) updateSquad(x, y int, clicked bool) {
	for slot, key := range []ebiten.Key{ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4} {
		if inpututil.IsKeyJustPressed(key) {
			engine.SelectOriginalAssignedTactic(&g.config, g.squadTeam, slot)
		}
	}
	pool := g.squadPool()
	if len(pool) == 0 {
		return
	}
	padX, padY, padFire := g.squadPadInput()
	dx, dy := padX, padY
	left, right, up, down, button, alias := ebiten.KeyArrowLeft, ebiten.KeyArrowRight, ebiten.KeyArrowUp, ebiten.KeyArrowDown, ebiten.KeyControlLeft, ebiten.KeyZ
	secondary := g.squadControllerNumber(false) == 2
	if secondary {
		left, right, up, down, button, alias = ebiten.KeyJ, ebiten.KeyL, ebiten.KeyI, ebiten.KeyK, ebiten.KeyControlRight, ebiten.KeyO
	}
	leftHeld, rightHeld, upHeld, downHeld := pressed(left), pressed(right), pressed(up), pressed(down)
	if !secondary && !g.config.Cooperative[g.squadTeam] {
		leftHeld, rightHeld = leftHeld || pressed(ebiten.KeyA), rightHeld || pressed(ebiten.KeyD)
		upHeld, downHeld = upHeld || pressed(ebiten.KeyW), downHeld || pressed(ebiten.KeyS)
	}
	if upHeld || downHeld {
		dy = int(axis(upHeld, downHeld))
	}
	if leftHeld || rightHeld {
		dx = int(axis(leftHeld, rightHeld))
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		g.optionTeam, g.skillReturn = g.squadTeam, squadScreen
		g.openFeatureMenu(skillScreen, 6-g.config.Skill[g.squadTeam])
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(alias) || inpututil.IsKeyJustPressed(button) || g.menuActivate || padFire {
		g.activateSquadCursor()
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) || g.padPausePressed() {
		g.finishSquad()
		return
	}
	g.stepSquadDirection(dx, dy, len(pool))
	if !clicked {
		return
	}
	if x >= 4 && x < 207 && y >= 57 && y < 208 {
		row := (y - 59) / 6
		if row >= 0 && row < len(pool) {
			g.squadCursor, g.squadPending = row, row
		}
		return
	}
	code := g.originalHit("ko5.men", x, y)
	if code == 2 || code == 4 || code == 5 {
		g.squadSort = map[int]int{2: 1, 4: 2, 5: 3}[code]
		g.squadCursor, g.squadPending = -1, -1
	}
	if code >= 12 && code <= 25 && code != 24 {
		slot := code - 12
		if code == 25 {
			slot = 12
		}
		g.squadSlot = slot
		g.selection = code
		if g.squadPending >= 0 {
			g.assignSquadPlayer(g.squadPending, slot)
		}
	}
	if code == 1 {
		g.finishSquad()
	}
	if code >= 8 && code <= 11 {
		engine.SelectOriginalAssignedTactic(&g.config, g.squadTeam, code-8)
	}
	if code == 30 {
		g.selection = 30
		if g.squadCursor >= 0 {
			g.toggleSquadController(false)
		}
	}
	if code == 31 {
		g.selection = 31
		if g.config.Cooperative[g.squadTeam] {
			if g.squadCursor >= 0 {
				g.toggleSquadController(true)
			}
		} else {
			g.optionTeam, g.skillReturn = g.squadTeam, squadScreen
			g.openFeatureMenu(skillScreen, 6-g.config.Skill[g.squadTeam])
		}
	}
}

func (g *Game) finishSquad() {
	available, seen := map[uint8]bool{}, map[uint8]bool{}
	for _, player := range g.squadPool() {
		available[uint8(player.RecordIndex+1)] = true
	}
	for _, record := range g.config.SelectedSquadRecords[g.squadTeam] {
		if record == 0 || !available[record] || seen[record] {
			g.notice = "INCOMPLETE"
			return
		}
		seen[record] = true
	}
	if !originalSquadControllersValid(g.originalSquadMode(), g.squadControllerMask()) {
		g.notice = "YOU MUST PICK A PLAYER FOR BOTH JOYSTICKS"
		return
	}
	g.captureCompetitionSetup()
	g.saveSettings()
	if g.competitionSavedConfig != nil && g.tournament != nil && g.tournament.Cup && g.config.Human == ([2]bool{true, true}) && g.squadTeam == 1 && !g.competitionCupRepeated {
		// Original 6854 falls through 6858: the away squad is revisited
		// with joystick one after its normal joystick-two visit.
		g.competitionCupRepeated, g.competitionSquadController = true, 1
		g.config.SquadBindings[1] = [13]uint8{}
		g.config.HasSquadBindings[1] = true
		g.syncSquadBindingSlotsForTeam(1)
		g.openSquad(1, preMatchScreen)
		return
	}
	if g.squadTeam == 0 && g.setupPlayers >= 2 && g.squadReturn == preMatchScreen && g.practiceMode < 0 {
		g.openSquad(1, preMatchScreen)
		return
	}
	if g.squadReturn == preMatchScreen {
		if g.practiceMode >= 0 {
			g.startConfiguredGame()
		} else if g.competitionSavedConfig != nil && g.tournament != nil {
			g.openPrematch(engine.PrepareMatchWithOriginalRandomState(g.config, g.tournament.OriginalRandomState))
		} else {
			g.openPrematch(nil)
		}
	} else {
		g.openFeatureMenu(g.squadReturn, 34)
	}
}

func (g *Game) drawSquad(dst *ebiten.Image) {
	labels := g.squadMenuLabels()
	tactics := engine.OriginalAssignedTactics(g.config, g.squadTeam)
	for i := 0; i < 4; i++ {
		labels[8+i] = engine.OriginalAssignedTacticName(g.config, g.squadTeam, i)
	}
	for i := 0; i < 13; i++ {
		code, number := 12+i, i+1
		if i == 12 {
			code, number = 25, 14
		}
		labels[code] = fmt.Sprint(number)
	}
	g.drawOriginalMenu(dst, "ko5.men", func(code int) bool {
		// Routine0x72c8 complements Done while the squad has no pending
		// assignment; its separate controller/tactic selections remain active.
		return code == 1 && g.squadCursor < 0 || code >= 8 && code <= 11 && code-8 == tactics.Active || code == g.selection
	}, labels)
	pool := g.squadPool()
	selected := g.config.SelectedSquadRecords[g.squadTeam]
	for i, player := range pool {
		y := 59 + i*6
		if y >= 209 {
			break
		}
		c := color.Color(amigaMenuColor(g.menus["ko5.men"].Palette[11]))
		if i == g.squadCursor || i == g.squadPending {
			rect(dst, 5, float64(y-1), 201, 6, color.RGBA{96, 0, 0, 255})
		}
		shirt := ""
		for slot, record := range selected {
			if record == uint8(player.RecordIndex+1) {
				number := slot + 1
				if slot == 12 {
					number = 14
				}
				shirt = fmt.Sprint(number)
			}
		}
		if controller := g.squadPoolController(player); controller != 0 {
			g.nativeText(dst, fmt.Sprintf("J%d", controller), 191, y, c, 0, 6, 8)
		}
		role := [...]string{"?", "KEEPER", "DEFENDER", "MIDFIELD", "FORWARD"}[max(0, min(4, player.Role))]
		g.nativeText(dst, player.Name, 9, y, c, 0, 6, 8)
		g.nativeText(dst, role, 125, y, c, 0, 6, 8)
		g.nativeText(dst, shirt, 177, y, c, 0, 6, 8)
	}
	for player := 1; player <= 10; player++ {
		pos := engine.UserTacticPreviewPosition(g.config, g.squadTeam, player)
		x, y := 230+int(pos.X)/12-2, 67+int(pos.Y)/12-2
		advance := 6
		if player+1 >= 10 {
			x -= 3
			advance = 4
		}
		g.nativeText(dst, fmt.Sprint(player+1), x, y, amigaMenuColor(g.menus["ko5.men"].Palette[11]), 0, advance, 8)
	}
	if g.notice != "" {
		g.centered(dst, g.notice, 236, color.White)
	} else {
		g.centered(dst, "FIRE SELECT : SPACE DONE : F1 SKILL", 236, color.White)
	}
}
