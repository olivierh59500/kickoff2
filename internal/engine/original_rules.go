package engine

// updateOriginalRules retains125FA's single manager dispatch. Boundary rule0
// may stage a restart immediately; returning from substitution42 may not also
// execute the restored throw/corner/kickoff manager in that same update.
func (m *Match) updateOriginalRules() {
	entry := m.originalRuleState
	if entry == 0 {
		m.checkBoundaries()
		if m.originalRuleState != 0 {
			// These native markers account for the source setup returning
			// without decrementing its newly established goal wait.
			m.originalPracticeGoalPending, m.originalMatchGoalPending = false, false
			return
		}
		if m.originalFoul.Active {
			m.updateOriginalFoulRule()
			return
		}
		m.updateOriginalThrowRule()
		return
	}
	switch entry {
	case 12, 13, 14, 35, 45:
		m.updateOriginalPeriodRule()
	case 3:
		m.updateOriginalThrowRule()
	case 21:
		m.beginOriginalThrowIn()
	case 24:
		m.beginOriginalGoalKick(m.DefendingTeam(m.originalGoalFrame.OutY < 0))
	case 25:
		team := 1 - m.DefendingTeam(m.originalGoalFrame.OutY < 0)
		if m.Config.Practice {
			team = 0
		}
		m.beginOriginalCorner(team)
	case 4, 37:
		m.updateOriginalCornerRule()
	case 5:
		m.updateOriginalGoalKickRule()
	case 1, 2, 9, 10, 11, 30, 36:
		m.updateOriginalPracticeGoalRule()
		m.updateOriginalMatchGoalRule()
	default:
		m.updateOriginalFoulRule()
	}
}
