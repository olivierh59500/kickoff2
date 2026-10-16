package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
)

// Fixture stores one scheduled league or cup match. Winner is an absolute team
// index, or -1 for a drawn league fixture or an unplayed fixture.
type Fixture struct {
	Home, Away, Round    int
	Played               bool
	HomeGoals, AwayGoals int
	Winner               int
	// ScoresUnknown marks history absent from an original aggregate-only save.
	ScoresUnknown bool `json:",omitempty"`
}

// Standing stores cumulative league results. Source 6508..6530 awards three
// points for a win and one point for a draw.
type Standing struct {
	Team, Played, Won, Drawn, Lost, For, Against, Points int
}

// Tournament represents either a fourteen-round, eight-team league or an
// eight-team knockout cup. Its JSON includes the complete fixture history.
type Tournament struct {
	OriginalFile  *OriginalTournamentFileState `json:",omitempty"`
	Kits          [8]TournamentKit
	MenuSkills    [2]int
	MenuExtraTime bool
	Version       int
	Cup           bool
	Names         [8]string
	Fixtures      []Fixture
	Table         [8]Standing
	Round         int
	Champion      int
	Seed          uint64
	RandomState   uint64
	// OriginalRandomState is the shared three-word Amiga generator. RandomState
	// remains a packed mirror for compatibility with earlier portable saves.
	OriginalRandomState [3]uint16
	OriginalPaceScale   uint16
	LeagueSkill         int
	DrawPositions       [8]int
	SquadFiles          [8]string
	UserSquads          [8][]byte
	SelectedSquadIDs    [8][13]uint32
	// SelectedSquadRecords identifies selections by one-based source-file
	// references. Zero remains available for an incomplete editor selection.
	SelectedSquadRecords    [8][13]uint8
	HasSelectedSquadRecords [8]bool
	Referee                 int
	Pitch                   Pitch
	Wind                    int
	HalfSeconds             int
	Speed                   float64
	Aftertouch              bool
	HumanTeams              [8]bool
	TeamSetups              [8]TournamentTeamSetup
	LastSimulation          *OriginalFixtureState
}

// NewTournament creates a schedule. Empty names receive unique defaults.
func NewTournament(cup bool, names [8]string, seed uint64) *Tournament {
	if seed == 0 {
		seed = 1990
	}
	return newOriginalTournament(cup, names, seed, [3]uint16{uint16(seed >> 16), uint16(seed), 0})
}

// NewTournamentWithOriginalState starts the draw from observed menu RNG words.
// It permits an exact original draw comparison without mapping wall-clock menu
// timing onto a modern configuration seed.
func NewTournamentWithOriginalState(cup bool, names [8]string, random [3]uint16) *Tournament {
	return newOriginalTournament(cup, names, 0, random)
}

func newOriginalTournament(cup bool, names [8]string, seed uint64, random [3]uint16) *Tournament {
	t := &Tournament{Version: 5, Cup: cup, Names: names, Round: 1, Champion: -1, Seed: seed, Referee: -1, HalfSeconds: 300, Speed: 1, Kits: OriginalTournamentKits}
	// Seed initialization follows 1694A. Menu timing is deliberately not invented;
	// callers comparing an original draw can restore the observed three words.
	t.OriginalRandomState = random
	for i := range t.Names {
		if t.Names[i] == "" {
			t.Names[i] = InternationalCountries[i]
		}
		t.SquadFiles[i] = fmt.Sprintf("int%d.sqd", i+1)
		t.Table[i].Team = i
		t.DrawPositions[i] = i
		if !cup {
			t.DrawPositions[i] = 7 - i
		}
		if squad, err := OriginalSquad(t.SquadFiles[i]); err == nil {
			if records, valid := CanonicalSquadSelection(squad.Players, [13]uint8{}, false, [13]uint32{}); valid && completeTournamentSelection(squad, records) {
				t.SelectedSquadRecords[i], t.HasSelectedSquadRecords[i] = records, true
				t.SelectedSquadIDs[i] = SquadSelectionIDs(squad.Players, records)
			}
		}
	}
	// 5D9A/6642 use DBF from 200, hence 201 swaps and 402 RNG calls.
	for i := 0; i <= 200; i++ {
		a := int(OriginalRandomWord(&t.OriginalRandomState) % 8)
		b := int(OriginalRandomWord(&t.OriginalRandomState) % 8)
		t.DrawPositions[a], t.DrawPositions[b] = t.DrawPositions[b], t.DrawPositions[a]
	}
	var order [8]int
	for team, position := range t.DrawPositions {
		order[position] = team
	}
	if cup {
		for i := 0; i < 8; i += 2 {
			t.Fixtures = append(t.Fixtures, Fixture{Home: order[i], Away: order[i+1], Round: 1, Winner: -1})
		}
	} else {
		for i, pair := range originalLeaguePairs {
			t.Fixtures = append(t.Fixtures, Fixture{Home: order[pair[0]], Away: order[pair[1]], Round: i/4 + 1, Winner: -1})
		}
	}
	t.rebuildTable()
	t.syncLegacyRandomState()
	return t
}

// Next returns the next unplayed fixture, or false when the tournament is over.
func (t *Tournament) Next() (Fixture, bool) {
	for _, f := range t.Fixtures {
		if !f.Played {
			return f, true
		}
	}
	return Fixture{}, false
}

// Record stores the next result. For a drawn cup game, winner is the match-side
// index (0 for home, 1 for away) decided by extra time or penalties.
func (t *Tournament) Record(homeGoals, awayGoals, winner int) error {
	if homeGoals < 0 || awayGoals < 0 {
		return errors.New("goals cannot be negative")
	}
	index := -1
	for i, f := range t.Fixtures {
		if !f.Played {
			index = i
			break
		}
	}
	if index < 0 {
		return errors.New("tournament is already complete")
	}
	f := &t.Fixtures[index]
	if t.Cup && homeGoals == awayGoals && (winner < 0 || winner > 1) {
		return errors.New("a drawn cup fixture requires a penalty winner")
	}
	f.Played = true
	f.HomeGoals = homeGoals
	f.AwayGoals = awayGoals
	f.Winner = -1
	if homeGoals > awayGoals {
		f.Winner = f.Home
	} else if awayGoals > homeGoals {
		f.Winner = f.Away
	} else if t.Cup {
		if winner == 0 {
			f.Winner = f.Home
		} else {
			f.Winner = f.Away
		}
	}
	t.rebuildTable()
	if t.Cup {
		complete := true
		winners := []int{}
		for _, result := range t.Fixtures {
			if result.Round == t.Round {
				if !result.Played {
					complete = false
				}
				winners = append(winners, result.Winner)
			}
		}
		if complete {
			if len(winners) == 1 {
				t.Champion = winners[0]
				return nil
			}
			t.Round++
			for i := 0; i < len(winners); i += 2 {
				t.Fixtures = append(t.Fixtures, Fixture{Home: winners[i], Away: winners[i+1], Round: t.Round, Winner: -1})
			}
		}
	} else if next, ok := t.Next(); ok {
		t.Round = next.Round
	} else {
		t.Round = 14
		t.Champion = t.Table[0].Team
	}
	return nil
}

func (t *Tournament) rebuildTable() {
	var table [8]Standing
	if t.OriginalFile != nil && !t.Cup {
		table = t.OriginalFile.BaseTable
	}
	for i := range table {
		table[i].Team = i
	}
	for _, f := range t.Fixtures {
		if !f.Played || f.ScoresUnknown {
			continue
		}
		h, a := &table[f.Home], &table[f.Away]
		h.Played++
		a.Played++
		h.For += f.HomeGoals
		h.Against += f.AwayGoals
		a.For += f.AwayGoals
		a.Against += f.HomeGoals
		if f.HomeGoals > f.AwayGoals {
			h.Won++
			h.Points += 3
			a.Lost++
		} else if f.AwayGoals > f.HomeGoals {
			a.Won++
			a.Points += 3
			h.Lost++
		} else {
			h.Drawn++
			a.Drawn++
			h.Points++
			a.Points++
		}
	}
	sort.SliceStable(table[:], func(i, j int) bool {
		a, b := table[i], table[j]
		if a.Points != b.Points {
			return a.Points > b.Points
		}
		if a.For-a.Against != b.For-b.Against {
			return a.For-a.Against > b.For-b.Against
		}
		if a.For != b.For {
			return a.For > b.For
		}
		return t.DrawPositions[a.Team] < t.DrawPositions[b.Team]
	})
	t.Table = table
}

// SimulateNext runs the original CPU-only match path, including its preceding
// wind, tactic and referee RNG calls. It commits result and RNG together so a
// rejected imported squad cannot partially advance a tournament save.
func (t *Tournament) SimulateNext() error {
	f, ok := t.Next()
	if !ok {
		return errors.New("tournament is already complete")
	}
	s := OriginalFixtureState{RandomState: t.OriginalRandomState, Cup: t.Cup}
	if t.LastSimulation != nil {
		s.Teams = t.LastSimulation.Teams
	}
	for side, team := range [2]int{f.Home, f.Away} {
		players, err := t.originalFixturePlayers(team, side == 0)
		if err != nil {
			return err
		}
		s.Players[side] = players
	}
	// 78D8 optionally chooses a random pitch, then 6CDA always draws three
	// wind words. 763A draws one tactic per CPU side before 6D52's referee.
	if t.Pitch > Artificial {
		OriginalRandomWord(&s.RandomState)
	}
	OriginalWind(t.Wind, &s.RandomState)
	for i := 0; i < 2; i++ {
		OriginalRandomWord(&s.RandomState)
	}
	referee := t.Referee
	if referee < 0 {
		referee = int(OriginalRandomWord(&s.RandomState) % uint16(len(Referees)))
	}
	ref := Referees[clampInt(referee, 0, len(Referees)-1)].Attributes
	s.Referee = [3]uint16{uint16(ref[0]), uint16(ref[1]), uint16(ref[2])}
	if err := SimulateOriginalFixture(&s); err != nil {
		return err
	}
	if err := t.Record(int(s.Goals[0]), int(s.Goals[1]), -1); err != nil {
		return err
	}
	t.OriginalRandomState, t.LastSimulation = s.RandomState, &s
	// A15E enters the ordinary selector at 6B4E after the result. Released
	// Fire consumes one RNG word before returning; held Fire adds more polls.
	t.AdvanceResultMenuRandom(1)
	t.syncLegacyRandomState()
	return nil
}

// Marshal encodes portable tournament data for desktop or mobile storage.
func (t *Tournament) Marshal() ([]byte, error) { return json.MarshalIndent(t, "", "  ") }

// LoadTournament validates saved data before it can be used by the application.
func LoadTournament(data []byte) (*Tournament, error) {
	var t Tournament
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, err
	}
	if (t.Version < 1 || t.Version > 5) || len(t.Fixtures) == 0 || len(t.Fixtures) > 56 || t.RandomState == 0 {
		return nil, errors.New("unsupported or invalid tournament save")
	}
	if t.Round < 1 || t.Round > 14 || t.Champion < -1 || t.Champion > 7 {
		return nil, errors.New("invalid tournament progress")
	}
	if t.Cup && t.Round > 3 {
		return nil, errors.New("invalid cup round")
	}
	for _, f := range t.Fixtures {
		if t.Cup && f.Round > 3 {
			return nil, errors.New("invalid saved cup round")
		}
		if f.Home < 0 || f.Home > 7 || f.Away < 0 || f.Away > 7 || f.Home == f.Away || f.HomeGoals < 0 || f.AwayGoals < 0 || f.Round < 1 || f.Round > 14 {
			return nil, errors.New("invalid saved fixture")
		}
		if f.Played && t.Cup && (f.Winner != f.Home && f.Winner != f.Away) {
			return nil, errors.New("cup fixture has no valid winner")
		}
	}
	if t.Version == 1 {
		// Keep saved fixture history while upgrading future CPU results to the
		// original algorithm. A xorshift stream cannot be continued as Amiga RNG.
		t.Version = 2
		t.OriginalRandomState = [3]uint16{uint16(t.RandomState >> 32), uint16(t.RandomState >> 16), uint16(t.RandomState)}
		t.Referee = -1
		for team := range t.DrawPositions {
			t.DrawPositions[team] = team
			t.SquadFiles[team] = fmt.Sprintf("int%d.sqd", team+1)
		}
	}
	if t.Version < 3 {
		// Older native versions always made country zero the human team.
		// Their missing duration/speed/aftertouch fields cannot be recovered.
		t.Version = 3
		t.HumanTeams[0] = true
		t.HalfSeconds, t.Speed = 300, 1
	}
	if t.Version < 4 {
		t.Version, t.Kits = 4, OriginalTournamentKits
	}
	if t.Version < 5 {
		// Versions one through four addressed selected players by their four-byte
		// name seed. Convert valid custom selections before duplicate seeds can
		// become ambiguous in the version-five record-based representation.
		for team := range t.Names {
			squad, err := OriginalSquad(t.SquadFiles[team])
			if len(t.UserSquads[team]) != 0 {
				squad, err = DecodeSquad(t.UserSquads[team])
			}
			if err != nil {
				return nil, fmt.Errorf("invalid saved competition squad: %w", err)
			}
			records, valid := CanonicalSquadSelection(squad.Players, [13]uint8{}, false, t.SelectedSquadIDs[team])
			if !valid || !completeTournamentSelection(squad, records) {
				return nil, errors.New("invalid saved competition selection")
			}
			t.SelectedSquadRecords[team], t.HasSelectedSquadRecords[team] = records, true
			t.SelectedSquadIDs[team] = SquadSelectionIDs(squad.Players, records)
		}
		t.Version = 5
	}
	if t.OriginalFile != nil {
		if len(t.OriginalFile.Header) != originalTournamentHeaderSize || len(t.OriginalFile.Teams) != originalTournamentTeamsSize || t.OriginalFile.Completed < 0 || t.OriginalFile.Completed > len(t.Fixtures) {
			return nil, errors.New("invalid original competition save metadata")
		}
	}
	for index, fixture := range t.Fixtures {
		if fixture.ScoresUnknown && (!fixture.Played || t.OriginalFile == nil || index >= t.OriginalFile.Completed) {
			return nil, errors.New("invalid unknown competition result")
		}
		if t.OriginalFile != nil && index < t.OriginalFile.Completed && !fixture.ScoresUnknown {
			return nil, errors.New("original aggregate history must not contain invented detailed scores")
		}
	}
	for _, kit := range t.Kits {
		if !kit.Valid() {
			return nil, errors.New("invalid competition kit")
		}
	}
	for _, skill := range t.MenuSkills {
		if skill < 0 || skill > 4 {
			return nil, errors.New("invalid competition menu skill")
		}
	}
	if t.HalfSeconds <= 0 || t.Speed < .25 || t.Speed > 1.5 || t.Pitch < Normal || t.Pitch > RandomPitch || t.Wind < 0 || t.Wind > 3 {
		return nil, errors.New("invalid saved competition options")
	}
	for _, setup := range t.TeamSetups {
		for _, controller := range setup.UnassignedControllers {
			if controller > 15 {
				return nil, errors.New("invalid saved unassigned controller nibble")
			}
		}
		for _, binding := range setup.Bindings {
			if binding > 2 {
				return nil, errors.New("invalid saved competition controller")
			}
		}
	}
	for team := range t.Names {
		squad, err := OriginalSquad(t.SquadFiles[team])
		if len(t.UserSquads[team]) != 0 {
			squad, err = DecodeSquad(t.UserSquads[team])
		}
		if err != nil {
			return nil, fmt.Errorf("invalid saved competition squad: %w", err)
		}
		records, valid := CanonicalSquadSelection(squad.Players, t.SelectedSquadRecords[team], t.HasSelectedSquadRecords[team], t.SelectedSquadIDs[team])
		if !valid || !completeTournamentSelection(squad, records) {
			return nil, errors.New("invalid saved competition selection")
		}
		t.SelectedSquadRecords[team], t.HasSelectedSquadRecords[team] = records, true
		t.SelectedSquadIDs[team] = SquadSelectionIDs(squad.Players, records)
	}
	if t.OriginalRandomState == [3]uint16{} || t.Referee < -1 || t.Referee >= len(Referees) {
		return nil, errors.New("invalid original tournament state")
	}
	var positions [8]bool
	for _, rank := range t.DrawPositions {
		if rank < 0 || rank >= 8 || positions[rank] {
			return nil, errors.New("invalid original tournament draw")
		}
		positions[rank] = true
	}
	t.syncLegacyRandomState()
	t.rebuildTable()
	return &t, nil
}

// completeTournamentSelection rejects editor work-in-progress. Tournament
// files always carry eleven starters and both reserves as distinct records.
func completeTournamentSelection(squad Squad, records [13]uint8) bool {
	used := make([]bool, len(squad.Players)+1)
	for _, reference := range records {
		if reference == 0 || int(reference) > len(squad.Players) || used[reference] {
			return false
		}
		used[reference] = true
	}
	return true
}

// Save writes an atomic desktop save. Mobile callers may instead use Marshal.
func (t *Tournament) Save(path string) error {
	data, err := t.Marshal()
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err = os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err = os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

// LoadTournamentFile reads a desktop tournament save.
func LoadTournamentFile(path string) (*Tournament, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadTournament(data)
}
