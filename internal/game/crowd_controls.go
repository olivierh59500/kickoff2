package game

import "github.com/olivierh59500/kickoff2/internal/engine"

// SetCrowdMuted changes the original C preference without stopping effects or
// already playing voices. The original keyboard command only changes a flag.
func (g *Game) SetCrowdMuted(muted bool) {
	g.config.CrowdMuted = muted
	if g.Match != nil {
		g.Match.Config.CrowdMuted = muted
	}
}

func (g *Game) crowdControlAvailable() bool {
	if g.screen != matchScreen || g.Match == nil || g.paused || g.Match.Phase != engine.Playing || g.Match.OriginalCornerSelectorActive() {
		return false
	}
	_, _, selecting := g.Match.OriginalSubstitutionSelection()
	_, injured := g.Match.OriginalInjurySubstitutePrompt()
	return !selecting && !injured
}

func (g *Game) queueCrowdToggle() {
	if g.crowdControlAvailable() {
		g.crowdTogglePending = !g.crowdTogglePending
	}
}

func (g *Game) queueCrowdKey(decoded rune) {
	if decoded == 'c' {
		g.queueCrowdToggle()
	}
}

// applyCrowdToggle follows FD9E after the current world's sound requests.
// Slow video waits retain a pending key until an actual world update finishes.
func (g *Game) applyCrowdToggle() {
	if g.crowdTogglePending {
		g.SetCrowdMuted(!g.Match.Config.CrowdMuted)
		g.crowdTogglePending = false
	}
}
