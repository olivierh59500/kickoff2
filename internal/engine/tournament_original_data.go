package engine

import (
	"fmt"
)

// originalLeaguePairs is the 56-pair fixture table at 1711A. Its entries refer to
// shuffled draw positions, not squad indices. It is not the circle algorithm.
var originalLeaguePairs = [56][2]int{
	{0, 1}, {2, 3}, {4, 5}, {6, 7},
	{0, 2}, {1, 3}, {4, 6}, {5, 7},
	{0, 3}, {1, 2}, {4, 7}, {5, 6},
	{0, 4}, {1, 5}, {2, 6}, {3, 7},
	{0, 5}, {1, 6}, {2, 7}, {3, 4},
	{0, 6}, {1, 7}, {2, 4}, {3, 5},
	{0, 7}, {1, 4}, {2, 5}, {3, 6},
	{7, 6}, {5, 4}, {3, 2}, {1, 0},
	{7, 5}, {6, 4}, {3, 1}, {2, 0},
	{6, 5}, {7, 4}, {2, 1}, {3, 0},
	{7, 3}, {6, 2}, {5, 1}, {4, 0},
	{4, 3}, {7, 2}, {6, 1}, {5, 0},
	{5, 3}, {4, 2}, {7, 1}, {6, 0},
	{6, 3}, {5, 2}, {4, 1}, {7, 0},
}

// Configure copies the competition-wide options. The original tournament starts
// with int1.sqd..int8.sqd regardless of the single-game display names or squads.
func (t *Tournament) Configure(config Config) {
	t.Referee = clampInt(config.Referee, -1, len(Referees)-1)
	t.Pitch = Pitch(clampInt(int(config.Pitch), 0, 4))
	t.Wind = clampInt(config.Wind, 0, 3)
	t.LeagueSkill = clampInt(config.LeagueSkill, 0, 2)
	t.HalfSeconds, t.Speed, t.Aftertouch = config.HalfSeconds, config.Speed, config.Aftertouch
	t.MenuSkills, t.MenuExtraTime = config.Skill, config.ExtraTime
	if t.HalfSeconds <= 0 {
		t.HalfSeconds = 300
	}
	if t.Speed <= 0 {
		t.Speed = 1
	}
	t.Speed = clamp(t.Speed, .25, 1.5)
}

// FixtureConfig maps the next fixture's persisted original squads into a match.
// Country control flags and saved options are shared with CPU-only fixtures.
func (t *Tournament) FixtureConfig(base Config) (Config, error) {
	f, ok := t.Next()
	if !ok {
		return Config{}, fmt.Errorf("tournament is already complete")
	}
	for side, team := range [2]int{f.Home, f.Away} {
		controller := 1
		if side == 1 && t.HumanTeams[f.Home] {
			controller = 2
		}
		t.applyCountryConfig(&base, side, team, controller)
	}
	base.ExtraTime, base.Practice = t.Cup, false
	base.Competition = true
	base.LeagueSkill = t.LeagueSkill
	base.Pitch, base.Wind, base.Referee = t.Pitch, t.Wind, t.Referee
	base.HalfSeconds, base.Speed, base.Aftertouch = t.HalfSeconds, t.Speed, t.Aftertouch
	base.Skill = t.MenuSkills
	base.TeamKits, base.HasTeamKits = [2]TournamentKit{t.Kits[f.Home], t.Kits[f.Away]}, [2]bool{true, true}
	pace := 100
	if !t.Cup {
		pace = OriginalLeaguePacePercent(t.LeagueSkill)
	}
	base.PacePercent = [2]int{pace, pace}
	return base, nil
}

// CountryConfig opens a table country's data independently of the next fixture.
// The presentation editor uses side zero; permanent storage keeps country IDs.
func (t *Tournament) CountryConfig(base Config, country int) (Config, error) {
	if country < 0 || country >= len(t.Names) {
		return Config{}, fmt.Errorf("invalid competition country")
	}
	t.applyCountryConfig(&base, 0, country, 1)
	// A table editor does not enter 70CE or clear a different physical binding.
	for slot, binding := range t.TeamSetups[country].Bindings {
		if binding == 2 {
			base.SquadBindings[0][slot] = 3
		}
	}
	return base, nil
}

func (t *Tournament) applyCountryConfig(base *Config, side, country, controller int) {
	base.TeamNames[side] = t.Names[country]
	base.SquadFiles[side] = t.SquadFiles[country]
	base.UserSquads[side] = append([]byte(nil), t.UserSquads[country]...)
	base.SelectedSquadIDs[side] = t.SelectedSquadIDs[country]
	base.SelectedSquadRecords[side] = t.SelectedSquadRecords[country]
	base.HasSelectedSquadRecords[side] = t.HasSelectedSquadRecords[country]
	base.Human[side] = t.HumanTeams[country]
	base.Cooperative[side] = false
	base.FixedPlayer[side], base.PartnerPlayer[side] = 0, 0
	base.SquadBindings[side], base.HasSquadBindings[side] = [13]uint8{}, t.TeamSetups[country].HasBindings
	base.SquadBindingPorts[side] = [3]uint8{uint8(controller), 0, uint8(3 - controller)}
	base.UnassignedSquadControllers[side], base.HasUnassignedSquadControllers[side] = t.TeamSetups[country].UnassignedControllers, t.TeamSetups[country].HasUnassignedControllers
	for slot, binding := range t.TeamSetups[country].Bindings {
		if int(binding) == controller {
			base.SquadBindings[side][slot] = 1
		}
	}
	base.Formation[side] = Formation424
	base.OriginalTactics[side] = t.TeamSetups[country].Tactics
	base.UserTactics[side] = nil
	for slot := 0; slot < 4; slot++ {
		base.AssignedTacticData[side][slot] = append([]byte(nil), t.TeamSetups[country].TacticData[slot]...)
		base.AssignedTacticNames[side][slot] = t.TeamSetups[country].TacticNames[slot]
	}
	updateOriginalActiveTacticConfig(base, side, OriginalAssignedTactics(*base, side))
}

// SetOriginalState imports the shared menu/match RNG and the retained pace-scale
// word at 1D13C. Before any human match that word is zero in the original image.
func (t *Tournament) SetOriginalState(random [3]uint16, paceScale uint16) {
	t.OriginalRandomState, t.OriginalPaceScale = random, paceScale
	t.syncLegacyRandomState()
}

// CompleteMatch records a played fixture and carries the on-pitch RNG forward.
// 9FD8 leaves the away side's setup scale at1D13C. Fatigue changes actor
// fixed pace, not this shared initializer word used by later off-pitch games.
func (t *Tournament) CompleteMatch(match *Match) error {
	if match == nil || match.Phase != FullTime {
		return fmt.Errorf("tournament match has not finished")
	}
	if err := t.Record(match.Teams[0].Score, match.Teams[1].Score, match.Winner); err != nil {
		return err
	}
	t.SetOriginalState(match.OriginalRandomState(), uint16(OriginalInitialPacePercent(match.Config, 1)))
	t.AdvanceResultMenuRandom(1)
	return nil
}

// AdvanceResultMenuRandom preserves additional 6B4E polls while the original
// result menu waits for Fire release. One released-entry poll is already part
// of SimulateNext/CompleteMatch. Exact held-button CPU timing is external to the
// off-pitch routine, so original-state imports may supply measured poll counts.
func (t *Tournament) AdvanceResultMenuRandom(polls int) {
	for i := 0; i < polls; i++ {
		OriginalRandomWord(&t.OriginalRandomState)
	}
	t.syncLegacyRandomState()
}

func (t *Tournament) syncLegacyRandomState() {
	t.RandomState = uint64(t.OriginalRandomState[0])<<32 | uint64(t.OriginalRandomState[1])<<16 | uint64(t.OriginalRandomState[2])
}

func (t *Tournament) originalFixturePlayers(team int, home bool) ([11]OriginalFixturePlayer, error) {
	var players [11]OriginalFixturePlayer
	squad, err := OriginalSquad(t.SquadFiles[team])
	if len(t.UserSquads[team]) != 0 {
		squad, err = DecodeSquad(t.UserSquads[team])
	}
	if err != nil {
		return players, err
	}
	records, valid := CanonicalSquadSelection(squad.Players, t.SelectedSquadRecords[team], t.HasSelectedSquadRecords[team], t.SelectedSquadIDs[team])
	if !valid || !completeTournamentSelection(squad, records) {
		return players, fmt.Errorf("original tournament squad has an invalid record selection")
	}
	var found [11]bool
	for _, source := range squad.Players {
		number := 0
		if slot := SquadSelectionSlot(records, source.RecordIndex); slot >= 0 {
			number = slot + 1
		}
		if number < 1 || number > 11 {
			continue
		}
		r := source.Record
		technique := func(at int) uint16 {
			v := originalSquareRoot(int(r[at])*int(r[13])) + (int(int8(r[24])) >> 2)
			if home {
				v += 10
			}
			return uint16(clampInt(v, 0, 200) >> 1)
		}
		p := OriginalFixturePlayer{Pace: uint16((int(r[12]>>2) + 180) * int(t.OriginalPaceScale) / 100),
			Shooting: technique(17), Passing: technique(16), Tackling: technique(18),
			Stamina: uint16(r[10] >> 1), Discipline: uint16(byte(200-r[14]) >> 1), Resilience: uint16(r[11] >> 1), Role: uint16(r[5])}
		if number == 1 {
			p.Shooting, p.Passing, p.Tackling, p.Role = technique(19), technique(19), 0, 1
		}
		players[number-1], found[number-1] = p, true
	}
	for i, present := range found {
		if !present {
			return players, fmt.Errorf("original tournament squad is missing selection %d", i+1)
		}
	}
	return players, nil
}
