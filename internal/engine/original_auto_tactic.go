package engine

// OriginalTacticState preserves a human's four assigned entries or a CPU's
// single entry and the active index. The automatic decision modifies entry
// zero, then reloads the active entry; those entries need not be the same.
type OriginalTacticState struct {
	Slots  [4]int
	Active int
	Count  int
}

// OriginalInitialTactics follows763A..76AE. A human side retains its squad's
// four assigned entries and active slot. A side without an input pointer gets
// one random standard entry and active slot zero, in team order.
func OriginalInitialTactics(human [2]bool, squads [2]OriginalTacticState, random *[3]uint16) (states [2]OriginalTacticState) {
	for team := range states {
		if human[team] {
			states[team] = squads[team]
			states[team].Count = 4
		} else {
			states[team] = OriginalTacticState{Slots: [4]int{int(OriginalRandomWord(random) % 4)}, Count: 1}
		}
	}
	return states
}

// Original tactic IDs put4-4-2 before5-3-2. Keep this explicit boundary
// mapping rather than changing the native configuration enum and saved values.
func originalFormationIndex(formation Formation) int {
	switch formation {
	case Formation442:
		return 2
	case Formation532:
		return 3
	default:
		return int(formation)
	}
}

func originalFormationFromIndex(index int) Formation {
	switch index {
	case 2:
		return Formation442
	case 3:
		return Formation532
	default:
		return Formation(index)
	}
}

// OriginalAutoTacticChoice translates the score comparisons and the ten
// winning/losing pairs at1378A and19BF2. Equal scores retain the current tactic.
func OriginalAutoTacticChoice(current, ownScore, opponentScore int) int {
	if ownScore == opponentScore || current < 0 || current >= int(FormationCount) {
		return current
	}
	choices := [FormationCount][2]int{
		{8, 5}, {2, 6}, {3, 0}, {7, 4}, {3, 9},
		{4, 5}, {4, 5}, {7, 1}, {2, 9}, {8, 6},
	}
	column := 0
	if ownScore < opponentScore {
		column = 1
	}
	return choices[current][column]
}

func (m *Match) originalAutomaticTactic(team int) {
	side := &m.Teams[team]
	if side.Score == m.Teams[1-team].Score {
		return
	}
	side.originalTacticSlots[0] = OriginalAutoTacticChoice(side.originalTacticSlots[0], side.Score, m.Teams[1-team].Score)
	// 1497A reloads the standard twenty-target records immediately. Imported
	// runtime targets must change too, rather than hiding the new formation.
	m.reloadOriginalTactic(team)
}
