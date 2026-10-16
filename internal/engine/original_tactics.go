package engine

import (
	"fmt"
	"path"
	"strings"
)

// OriginalCustomTacticID canonically represents a source RAM pointer to a
// ten-byte name header followed by800 coordinate bytes. The native bank stores
// the file bytes separately, without exposing a host or guest memory address.
const OriginalCustomTacticID = 10

// OriginalAssignedTactics resolves a saved four-entry human bank. Older
// configurations assign Formation to entry zero and retain the other defaults.
func OriginalAssignedTactics(config Config, team int) OriginalTacticState {
	if team < 0 || team > 1 {
		return OriginalTacticState{}
	}
	state := config.OriginalTactics[team]
	legacy := state.Count == 0
	if legacy {
		formation := Formation(clampInt(int(config.Formation[team]), 0, int(FormationCount)-1))
		state = OriginalTacticState{Slots: [4]int{originalFormationIndex(formation), 1, 2, 3}}
	}
	state.Count, state.Active = 4, clampInt(state.Active, 0, 3)
	for slot := range state.Slots {
		data := config.AssignedTacticData[team][slot]
		if slot == state.Active && len(data) == 0 && (legacy || state.Slots[slot] == OriginalCustomTacticID) {
			data = config.UserTactics[team]
		}
		if len(data) != 0 {
			if _, err := DecodeTactic(data); err == nil {
				state.Slots[slot] = OriginalCustomTacticID
				continue
			}
		}
		state.Slots[slot] = clampInt(state.Slots[slot], 0, int(FormationCount)-1)
	}
	return state
}

func originalAssignedTacticData(config Config, team, slot int) []byte {
	data := config.AssignedTacticData[team][slot]
	state := config.OriginalTactics[team]
	if len(data) == 0 && slot == state.Active && (state.Count == 0 || state.Slots[slot] == OriginalCustomTacticID) {
		data = config.UserTactics[team]
	}
	return data
}

func originalTacticName(name string) string {
	name = path.Base(name)
	name = strings.TrimSuffix(name, path.Ext(name))
	if name == "" || name == "." {
		name = "CUSTOM"
	}
	if runes := []rune(name); len(runes) > 9 {
		name = string(runes[:9])
	}
	return name
}

// OriginalAssignedTacticName exposes1493C's extension-free caption for menus.
func OriginalAssignedTacticName(config Config, team, slot int) string {
	if team < 0 || team > 1 || slot < 0 || slot > 3 {
		return ""
	}
	state := OriginalAssignedTactics(config, team)
	if state.Slots[slot] == OriginalCustomTacticID {
		return originalTacticName(config.AssignedTacticNames[team][slot])
	}
	return [...]string{"4-2-4", "4-3-3", "4-4-2", "5-3-2", "balanced", "blitz", "falcon", "lockout", "zbreak", "zxcross"}[state.Slots[slot]]
}

func materializeOriginalTacticBank(config *Config, team int) OriginalTacticState {
	state := OriginalAssignedTactics(*config, team)
	if state.Slots[state.Active] == OriginalCustomTacticID && len(config.AssignedTacticData[team][state.Active]) == 0 {
		config.AssignedTacticData[team][state.Active] = append([]byte(nil), config.UserTactics[team]...)
	}
	config.OriginalTactics[team] = state
	return state
}

func updateOriginalActiveTacticConfig(config *Config, team int, state OriginalTacticState) {
	config.OriginalTactics[team] = state
	config.UserTactics[team] = append([]byte(nil), config.AssignedTacticData[team][state.Active]...)
	if id := state.Slots[state.Active]; id < OriginalCustomTacticID {
		config.Formation[team] = originalFormationFromIndex(id)
	}
}

// AssignOriginalTactic follows79B2..79D4/7AB4..7AEA: replace one assigned entry
// while retaining the selected active entry. Custom files retain their own bytes.
func AssignOriginalTactic(config *Config, team, slot, id int, data []byte, name string) error {
	if config == nil || team < 0 || team > 1 || slot < 0 || slot > 3 || id < 0 || id > OriginalCustomTacticID {
		return fmt.Errorf("invalid original tactic assignment")
	}
	if id == OriginalCustomTacticID {
		if _, err := DecodeTactic(data); err != nil {
			return err
		}
	}
	state := materializeOriginalTacticBank(config, team)
	state.Slots[slot] = id
	config.AssignedTacticData[team][slot], config.AssignedTacticNames[team][slot] = nil, ""
	if id == OriginalCustomTacticID {
		// The source uses the first800 bytes; retain the complete imported file
		// for portable settings, including any unused trailing bytes.
		config.AssignedTacticData[team][slot] = append([]byte(nil), data...)
		config.AssignedTacticNames[team][slot] = originalTacticName(name)
	}
	updateOriginalActiveTacticConfig(config, team, state)
	return nil
}

// SelectOriginalAssignedTactic follows7314's squad-menu selection without
// replacing the entry or any other assigned file.
func SelectOriginalAssignedTactic(config *Config, team, slot int) bool {
	if config == nil || team < 0 || team > 1 || slot < 0 || slot > 3 {
		return false
	}
	state := materializeOriginalTacticBank(config, team)
	state.Active = slot
	updateOriginalActiveTacticConfig(config, team, state)
	return true
}

// OriginalTacticBank returns the currently loaded runtime descriptor.
func (m *Match) OriginalTacticBank(team int) OriginalTacticState {
	if team < 0 || team > 1 {
		return OriginalTacticState{}
	}
	side := &m.Teams[team]
	return OriginalTacticState{Slots: side.originalTacticSlots, Active: side.originalTacticSlot, Count: side.originalTacticCount}
}

// reloadOriginalTactic follows1497A..14A4C, including every keeper target,
// reflection of all Y words and reflection of only the first16 X words.
func (m *Match) reloadOriginalTactic(team int) {
	side := &m.Teams[team]
	id := side.originalTacticSlots[side.originalTacticSlot]
	var tactic Tactic
	custom := id == OriginalCustomTacticID
	if custom {
		var err error
		tactic, err = DecodeTactic(originalAssignedTacticData(m.Config, team, side.originalTacticSlot))
		if err != nil {
			return
		}
		m.Config.UserTactics[team] = append([]byte(nil), originalAssignedTacticData(m.Config, team, side.originalTacticSlot)...)
	} else {
		side.Formation = originalFormationFromIndex(id)
		side.queuedFormation = side.Formation
		m.Config.UserTactics[team] = nil
	}
	for index := range side.Players {
		p := &side.Players[index]
		for target := range p.originalTargets {
			v := Vec2{456, 18}
			if index != 0 {
				if custom {
					v = tactic.Targets[index-1][target]
				} else {
					v = originalTactics[side.Formation].Targets[index-1][target]
				}
			}
			if m.AttackDirection(team) < 0 {
				v.Y = PitchHeight - v.Y
				if target < 16 {
					v.X = PitchWidth - v.X
				}
			}
			p.originalTargets[target] = v
		}
		p.originalHasTargets = true
	}
}

// QueueOriginalTacticSlot supplies the actual decoded number-key word. Keyboard
// commands are independent of joystick ownership and are polled after the world.
func (m *Match) QueueOriginalTacticSlot(team, slot int) bool {
	if team < 0 || team > 1 || slot < 0 || slot > 3 {
		return false
	}
	scan := [2][4]uint32{{2, 3, 4, 5}, {0x6D, 0x6E, 0x6F, 0x6A}}[team][slot]
	return m.QueueOriginalKeyboardEvent(scan<<16 | uint32(0x31+slot))
}

func (m *Match) originalApplyTacticKey(key uint32) bool {
	character := int(key & 65535)
	if character < 0x31 || character > 0x34 {
		return false
	}
	team := 0
	if int16(key>>16) > 0x60 {
		team = 1
	}
	if !m.Config.Human[team] {
		return true
	}
	m.Teams[team].originalTacticSlot = character - 0x31
	m.reloadOriginalTactic(team)
	name := OriginalAssignedTacticName(m.Config, team, character-0x31)
	m.notice(name+": "+m.Teams[team].Name, 150)
	return true
}
