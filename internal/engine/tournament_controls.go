package engine

// TournamentTeamSetup retains each country's squad-menu assignments. Bindings
// are physical joystick numbers 1/2, as in source squad records, rather than
// permanent home/away roles. A different active joystick clears an old binding.
type TournamentTeamSetup struct {
	UnassignedControllers    [64]uint8
	HasUnassignedControllers bool
	Bindings                 [13]uint8
	HasBindings              bool
	Tactics                  OriginalTacticState
	TacticData               [4][]byte
	TacticNames              [4]string
}

// CountryControls follows 5DF4: selected countries show J, with the current
// home/away humans assigned J1/J2. An away-only human receives J1.
func (t *Tournament) CountryControls() [8]uint16 {
	var controls [8]uint16
	for team, human := range t.HumanTeams {
		if human {
			controls[team] = 1
		}
	}
	if fixture, ok := t.Next(); ok {
		if t.HumanTeams[fixture.Home] {
			controls[fixture.Home] = 2
		}
		if t.HumanTeams[fixture.Away] {
			controls[fixture.Away] = 2
			if t.HumanTeams[fixture.Home] {
				controls[fixture.Away] = 3
			}
		}
	}
	return controls
}

// StoreFixtureSetup carries human squad/tactic edits back to their countries.
// CPU tactic draws are temporary and do not replace the assigned human bank.
func (t *Tournament) StoreFixtureSetup(config Config) {
	fixture, ok := t.Next()
	if !ok {
		return
	}
	for side, team := range [2]int{fixture.Home, fixture.Away} {
		if !config.Human[side] {
			continue
		}
		t.storeCountrySetup(team, config, side)
	}
}

// StoreCountrySetup commits a table editor without using the next fixture's ID.
func (t *Tournament) StoreCountrySetup(country int, config Config) {
	if country < 0 || country >= len(t.Names) {
		return
	}
	t.Names[country], t.SquadFiles[country] = config.TeamNames[0], config.SquadFiles[0]
	t.storeCountrySetup(country, config, 0)
}

func (t *Tournament) storeCountrySetup(team int, config Config, side int) {
	t.UserSquads[team] = append([]byte(nil), config.UserSquads[side]...)
	t.SelectedSquadIDs[team] = config.SelectedSquadIDs[side]
	t.SelectedSquadRecords[team] = config.SelectedSquadRecords[side]
	t.HasSelectedSquadRecords[team] = config.HasSelectedSquadRecords[side]
	setup := &t.TeamSetups[team]
	setup.UnassignedControllers, setup.HasUnassignedControllers = config.UnassignedSquadControllers[side], config.HasUnassignedSquadControllers[side]
	setup.HasBindings, setup.Bindings = config.HasSquadBindings[side], [13]uint8{}
	controller := uint8(1)
	if side == 1 && config.Human[0] {
		controller = 2
	}
	for slot, role := range config.SquadBindings[side] {
		if role == 1 {
			setup.Bindings[slot] = controller
		} else if role == 3 {
			setup.Bindings[slot] = 3 - controller
		}
	}
	setup.Tactics = OriginalAssignedTactics(config, side)
	for slot := 0; slot < 4; slot++ {
		setup.TacticData[slot] = append([]byte(nil), originalAssignedTacticData(config, side, slot)...)
		setup.TacticNames[slot] = config.AssignedTacticNames[side][slot]
	}
}
