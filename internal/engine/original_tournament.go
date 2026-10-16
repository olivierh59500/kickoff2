package engine

import "errors"

// OriginalFixturePlayer retains the runtime words read by 11518..119A6. These
// are source integers, not normalized skill estimates. Discipline is (200 minus
// the squad aggression byte)/2; lower values therefore mean greater aggression.
type OriginalFixturePlayer struct {
	Pace, Shooting, Stamina, Discipline, Passing, Tackling, Resilience uint16
	Role, FoulPoints, Bookings, Goals, Injuries                        uint16
}

// OriginalFixtureTeam stores the eight aggregate words at 1D8D2/1D922 and the
// three following counters. The counters survive successive CPU fixtures in
// the source; the eight aggregates are rebuilt at the start of each fixture.
type OriginalFixtureTeam struct {
	Pace, Shooting, Stamina, Discipline, Keeper, Passing, Tackling, Resilience uint16
	Dismissals, Bookings, Fouls                                                uint16
}

// OriginalFixtureState is the complete input/output of the CPU-only routine at
// 115C6. The three referee words are copied by 6D52 from its temperament bytes.
// Player statistics and RNG are returned for independent original comparisons.
type OriginalFixtureState struct {
	Players     [2][11]OriginalFixturePlayer
	Teams       [2]OriginalFixtureTeam
	RandomState [3]uint16
	Referee     [3]uint16
	Cup         bool
	Goals       [2]uint16
	Contests    int
	RandomCalls int
}

func (s *OriginalFixtureState) random(divisor uint16) uint16 {
	if divisor == 0 {
		divisor = 1
	}
	s.RandomCalls++
	return OriginalRandomWord(&s.RandomState) % divisor
}

// originalFixtureDivision retains a 68000 DIVU overflow: the destination is
// unchanged when the quotient does not fit a word. Subsequent MULU instructions
// consume only that destination's low word.
func originalFixtureDivision(value uint32, divisor uint16) uint32 {
	if value/uint32(divisor) > 65535 {
		return value
	}
	return value / uint32(divisor)
}

// originalFixtureAggregate translates 11518, including its role restrictions
// and final replacement of negative aggregate words with one.
func originalFixtureAggregate(players [11]OriginalFixturePlayer, previous OriginalFixtureTeam) OriginalFixtureTeam {
	t := OriginalFixtureTeam{Keeper: ((players[0].Shooting + players[0].Passing) >> 1) * 11,
		Dismissals: previous.Dismissals, Bookings: previous.Bookings, Fouls: previous.Fouls}
	for _, p := range players[1:] {
		t.Pace += p.Pace
		if p.Role == 4 {
			t.Shooting += p.Shooting
		} else if p.Role == 3 {
			t.Shooting += p.Shooting >> 1
		}
		t.Passing += p.Passing
		t.Stamina += p.Stamina
		t.Discipline += p.Discipline
		if p.Role == 2 {
			t.Tackling += p.Tackling
		} else if p.Role == 3 {
			t.Tackling += p.Tackling >> 1
		}
		t.Resilience += p.Resilience
	}
	for _, word := range []*uint16{&t.Pace, &t.Shooting, &t.Stamina, &t.Discipline, &t.Keeper, &t.Passing, &t.Tackling, &t.Resilience} {
		if int16(*word) < 0 {
			*word = 1
		}
	}
	return t
}

func originalFixtureStrength(t OriginalFixtureTeam) uint16 {
	v := originalFixtureDivision(uint32(t.Pace)*uint32(t.Pace), 550)
	for _, factor := range []uint16{t.Stamina, t.Passing, t.Tackling} {
		v = originalFixtureDivision(uint32(uint16(v))*uint32(factor), 550)
	}
	return uint16(v) + 1000
}

func (s *OriginalFixtureState) compareStrength() int16 {
	return int16(s.random(originalFixtureStrength(s.Teams[0])) - s.random(originalFixtureStrength(s.Teams[1])))
}

// contest is the signed branch table at 117C6. Zero comparisons are intentionally
// asymmetric; replacing this with a probability or a symmetric sign test changes
// both results and the number of RNG calls.
func (s *OriginalFixtureState) contest() int {
	first, second := s.compareStrength(), s.compareStrength()
	if second < 0 && first <= 0 {
		return -1
	}
	if second >= 0 && first > 0 {
		return 1
	}
	home := int16(s.random(4400) - s.Teams[0].Discipline)
	away := int16(s.random(4400) - s.Teams[1].Discipline)
	if home < 0 && away >= 0 {
		return 2
	}
	if home >= 0 && away < 0 {
		return -2
	}
	return 0
}

// selectPlayer retains the two-pass weighted subtraction at 117E8/11850/118BE.
// An all-zero eligible roster would loop forever in the original; report that
// condition to the caller instead of inventing a player or hanging the app.
func (s *OriginalFixtureState) selectPlayer(team, kind int) (int, error) {
	var weights [11]uint16
	var total uint16
	for i, p := range s.Players[team] {
		switch kind {
		case 0: // Goalscorer: attackers, midfielders, then quarter-weight defenders.
			switch p.Role {
			case 4:
				weights[i] = p.Shooting
			case 3:
				weights[i] = p.Shooting >> 1
			case 2:
				weights[i] = p.Shooting >> 2
			}
		case 1: // Injury: every player, with the goalkeeper quarter weighted.
			weights[i] = p.Discipline
			if p.Role == 1 {
				weights[i] >>= 2
			}
		case 2: // Foul: the goalkeeper is excluded.
			if p.Role != 1 {
				weights[i] = p.Discipline
			}
		}
		total += weights[i]
	}
	if total == 0 || int16(total) < 0 {
		return -1, errors.New("original CPU fixture has no selectable player")
	}
	remaining := s.random(total)
	for i, weight := range weights {
		remaining -= weight
		if int16(remaining) < 0 {
			return i, nil
		}
	}
	return -1, errors.New("original CPU fixture player weights overflowed")
}

func (s *OriginalFixtureState) dismiss(team, player int) {
	p, t := &s.Players[team][player], &s.Teams[team]
	t.Dismissals++
	t.Pace -= p.Pace
	if p.Role == 4 {
		t.Shooting -= p.Shooting
	} else if p.Role == 3 {
		t.Shooting -= p.Shooting >> 1
	}
	t.Passing -= p.Passing
	t.Stamina -= p.Stamina
	t.Discipline -= p.Discipline
	if p.Role == 2 {
		t.Tackling -= p.Tackling
	} else if p.Role == 3 {
		t.Tackling -= p.Tackling >> 1
	}
}

func (s *OriginalFixtureState) foul(team int) error {
	s.Teams[team].Fouls++
	i, err := s.selectPlayer(team, 2)
	if err != nil {
		return err
	}
	p := &s.Players[team][i]
	p.FoulPoints++
	if int16(s.random(s.Referee[0])) < int16(p.FoulPoints) {
		p.FoulPoints += s.Referee[1]
		p.Bookings++
		if int16(p.Bookings) >= 2 {
			s.dismiss(team, i)
		} else {
			s.Teams[team].Bookings++
		}
	}
	p.FoulPoints += s.Referee[2]
	other := 1 - team
	if int16(s.random(12000)) < int16(s.Teams[other].Resilience) {
		i, err = s.selectPlayer(other, 1)
		if err != nil {
			return err
		}
		s.Players[other][i].Injuries++
	}
	return nil
}

// SimulateOriginalFixture executes the original 100 contests, scorer selection,
// fouls, bookings, dismissals and injuries in source RNG order. A tied cup repeats
// 30 more contests until its score differs; it does not choose a random winner.
// This is the off-pitch routine, independent of the on-pitch match simulation.
func SimulateOriginalFixture(s *OriginalFixtureState) error {
	if s.RandomState == [3]uint16{} {
		return errors.New("original CPU fixture RNG is in its absorbing zero state")
	}
	s.Goals, s.Contests, s.RandomCalls = [2]uint16{}, 0, 0
	for team := range s.Teams {
		s.Teams[team] = originalFixtureAggregate(s.Players[team], s.Teams[team])
	}
	for contests := 100; ; contests = 30 {
		for n := 0; n < contests; n++ {
			s.Contests++
			switch event := s.contest(); event {
			case 1, -1:
				defense := func(t OriginalFixtureTeam) uint16 { return uint16(uint32(t.Keeper) * uint32(t.Tackling) >> 6) }
				team := 0
				first, second := s.Teams[0].Shooting, defense(s.Teams[1])
				if event < 0 {
					team = 1
					first, second = defense(s.Teams[0]), s.Teams[1].Shooting
				}
				a, b := int16(s.random(first)), int16(s.random(second))
				if event > 0 && a >= b || event < 0 && a <= b {
					s.Goals[team]++
					i, err := s.selectPlayer(team, 0)
					if err != nil {
						return err
					}
					s.Players[team][i].Goals++
				}
			case 2:
				if err := s.foul(0); err != nil {
					return err
				}
			case -2:
				if err := s.foul(1); err != nil {
					return err
				}
			}
		}
		if !s.Cup || s.Goals[0] != s.Goals[1] {
			return nil
		}
	}
}
