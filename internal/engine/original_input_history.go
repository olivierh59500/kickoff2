package engine

func initialOriginalInputHistory(config Config) [4]int16 {
	history := [4]int16{-1, -1, 0, 0}
	if config.Practice {
		// Two-player Practice joins J2 as the home partner, rather than J3.
		history[2] = -1
	}
	return history
}

// updateOriginalInputHistory translates the retained direction words updated
// at1660E/16666 before the next hardware sample. The high byte ages while the
// stick is neutral; an active direction resets it to four. Parallel polling at
// 166D0..16746 does not write the retained words for J3/J4.
func (m *Match) updateOriginalInputHistory() {
	inputs := [4]Input{m.controls[0], m.controls[1], m.cooperativeInputs[0], m.cooperativeInputs[1]}
	for controller, in := range inputs {
		if controller >= 2 && !(controller == 2 && m.Config.Practice) {
			continue
		}
		if in.Move.Len() > .05 {
			m.originalDirectionInputHistory[controller] = int16(0x0400 | OriginalDirectionIndex(in.Move))
			continue
		}
		word := m.originalDirectionInputHistory[controller]
		age := int8(uint16(word)>>8) - 1
		if age < 0 {
			m.originalDirectionInputHistory[controller] = -1
		} else {
			m.originalDirectionInputHistory[controller] = int16(uint16(uint8(age))<<8 | uint16(uint8(word)))
		}
	}
}

func (m *Match) originalRetainedDirection(team, player int) int {
	controller := m.originalPlayerController(team, player)
	word := m.originalDirectionInputHistory[controller]
	if word < 0 {
		return -1
	}
	return int(word) & 7
}
