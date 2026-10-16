package engine

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

const (
	originalTournamentHeaderSize = 610
	originalTournamentTeamSize   = 0x41e
	originalTournamentTeamsSize  = 8 * originalTournamentTeamSize
	originalTournamentPoolBase   = 0xc6d390
)

// TournamentKit stores the original style and two palette-choice indexes.
type TournamentKit struct{ Style, Shirt, Shorts int }

func (kit TournamentKit) Valid() bool {
	return kit.Style >= 0 && kit.Style < 6 && kit.Shirt >= 0 && kit.Shirt < 20 && kit.Shorts >= 0 && kit.Shorts < 20
}

// OriginalTournamentKits follows the eight country defaults at source170EA.
var OriginalTournamentKits = [8]TournamentKit{{0, 4, 1}, {0, 9, 1}, {0, 2, 4}, {0, 5, 7}, {0, 1, 4}, {0, 12, 1}, {0, 11, 4}, {1, 1, 4}}

// OriginalTournamentFileState preserves file-only bytes and aggregate history.
// Original files omit individual old scores and the session RNG. Native saves
// retain the standings baseline so future results do not invent that history.
type OriginalTournamentFileState struct {
	Header, Teams, Trailing []byte
	Custom                  [8][4][]byte
	BaseTable               [8]Standing
	Completed               int
}

func originalLatin1(bytes []byte) string {
	result := make([]rune, 0, len(bytes))
	for _, value := range bytes {
		if value == 0 {
			break
		}
		result = append(result, rune(value))
	}
	return string(result)
}

func putOriginalText(destination []byte, value string) error {
	if utf8.RuneCountInString(value) > len(destination) {
		return fmt.Errorf("original text exceeds %d characters", len(destination))
	}
	clear(destination)
	index := 0
	for _, r := range value {
		if r < 32 || r > 255 || r == 127 {
			return fmt.Errorf("original text contains an unsupported character")
		}
		destination[index], index = byte(r), index+1
	}
	return nil
}

// DecodeOriginalTournament reads the byte stream written by6262. File type is
// supplied by its .lge/.cup extension, as in the original menus. Session RNG is
// explicitly supplied because the original file does not contain those words.
func DecodeOriginalTournament(data []byte, cup bool, random [3]uint16) (*Tournament, error) {
	if len(data) < originalTournamentHeaderSize+originalTournamentTeamsSize || len(data) > 65536 {
		return nil, fmt.Errorf("original competition file has an invalid length")
	}
	header := append([]byte(nil), data[:originalTournamentHeaderSize]...)
	body := append([]byte(nil), data[originalTournamentHeaderSize:originalTournamentHeaderSize+originalTournamentTeamsSize]...)
	word := func(offset int) int { return int(binary.BigEndian.Uint16(header[offset:])) }
	completed := word(0)
	if completed > 56 || cup && completed > 3 {
		return nil, fmt.Errorf("invalid original competition progress")
	}
	t := &Tournament{Version: 5, Cup: cup, Champion: -1, Round: 1, Seed: 1990,
		OriginalRandomState: random, Kits: OriginalTournamentKits,
		OriginalFile: &OriginalTournamentFileState{Header: header, Teams: body}}
	if random == [3]uint16{} {
		t.OriginalRandomState = [3]uint16{0, 1990, 0}
	}
	t.syncLegacyRandomState()
	var seen [8]bool
	base := binary.BigEndian.Uint32(header[6:])
	if base == 0 || base > ^uint32(0)-7*originalTournamentTeamSize {
		return nil, fmt.Errorf("invalid original country identity base")
	}
	for country := 0; country < 8; country++ {
		record := header[6+country*24 : 6+(country+1)*24]
		if binary.BigEndian.Uint32(record) != base+uint32(country*originalTournamentTeamSize) {
			return nil, fmt.Errorf("invalid original country record identity")
		}
		rank := int(binary.BigEndian.Uint16(record[22:]))
		if cup {
			if rank != country || binary.BigEndian.Uint16(record[6:]) != uint16('A'+binary.BigEndian.Uint16(record[20:])/2) {
				return nil, fmt.Errorf("invalid original cup country identity or quarter-final group")
			}
			rank = int(binary.BigEndian.Uint16(record[20:]))
		}
		if rank >= 8 || seen[rank] || binary.BigEndian.Uint16(record[4:]) > 3 {
			return nil, fmt.Errorf("invalid original country ranks or controls")
		}
		seen[rank], t.DrawPositions[country], t.HumanTeams[country] = true, rank, binary.BigEndian.Uint16(record[4:]) != 0
		team := body[country*originalTournamentTeamSize : (country+1)*originalTournamentTeamSize]
		count := int(binary.BigEndian.Uint16(team[20:]))
		if count < 13 || count > 24 {
			return nil, fmt.Errorf("unsupported original competition squad count %d", count)
		}
		squadData := append([]byte(nil), team[:22+count*42]...)
		squad, err := DecodeSquad(squadData)
		if err != nil {
			return nil, err
		}
		t.Names[country], t.UserSquads[country] = originalLatin1(team[:20]), squadData
		var assigned [13]bool
		for _, player := range squad.Players {
			shirt, binding := player.Number, player.Record[21]>>4
			if shirt == 0 {
				t.TeamSetups[country].UnassignedControllers[player.RecordIndex] = binding
				continue
			}
			slot := shirt - 1
			if shirt == 14 {
				slot = 12
			}
			if shirt == 13 || slot < 0 || slot >= 13 || assigned[slot] || binding > 2 {
				return nil, fmt.Errorf("invalid original competition shirt or controller assignment")
			}
			assigned[slot], t.SelectedSquadIDs[country][slot] = true, player.ID
			t.SelectedSquadRecords[country][slot] = uint8(player.RecordIndex + 1)
			t.TeamSetups[country].Bindings[slot] = binding
		}
		for _, present := range assigned {
			if !present {
				return nil, fmt.Errorf("original competition squad is incomplete")
			}
		}
		t.TeamSetups[country].HasBindings = true
		t.HasSelectedSquadRecords[country] = true
		t.TeamSetups[country].HasUnassignedControllers = true
		t.TeamSetups[country].Tactics = OriginalTacticState{Count: 4, Active: int(binary.BigEndian.Uint16(team[0x416:]))}
		if t.TeamSetups[country].Tactics.Active >= 4 {
			return nil, fmt.Errorf("invalid original active tactic")
		}
		t.Kits[country] = TournamentKit{int(binary.BigEndian.Uint16(team[0x418:])), int(binary.BigEndian.Uint16(team[0x41a:])), int(binary.BigEndian.Uint16(team[0x41c:]))}
		if !t.Kits[country].Valid() {
			return nil, fmt.Errorf("invalid original competition kit")
		}
	}
	cursor := originalTournamentHeaderSize + originalTournamentTeamsSize
	for country := 0; country < 8; country++ {
		team := body[country*originalTournamentTeamSize:]
		for slot := 0; slot < 4; slot++ {
			id := binary.BigEndian.Uint32(team[0x406+slot*4:])
			if id < 20 {
				if id >= uint32(FormationCount) {
					return nil, fmt.Errorf("unsupported original tactic identity %d", id)
				}
				t.TeamSetups[country].Tactics.Slots[slot] = int(id)
				continue
			}
			if cursor+810 > len(data) {
				return nil, fmt.Errorf("original custom tactic is truncated")
			}
			block := data[cursor : cursor+810]
			if _, err := DecodeTactic(block[10:]); err != nil {
				return nil, err
			}
			t.TeamSetups[country].Tactics.Slots[slot] = OriginalCustomTacticID
			t.TeamSetups[country].TacticNames[slot] = originalLatin1(block[:10])
			t.TeamSetups[country].TacticData[slot] = append([]byte(nil), block[10:]...)
			t.OriginalFile.Custom[country][slot] = append([]byte(nil), block...)
			cursor += 810
		}
	}
	t.OriginalFile.Trailing = append([]byte(nil), data[cursor:]...)
	if err := t.decodeOriginalOptions(header); err != nil {
		return nil, err
	}
	if cup {
		if err := t.decodeOriginalCup(header, completed); err != nil {
			return nil, err
		}
	} else {
		var order [8]int
		for country, rank := range t.DrawPositions {
			order[rank] = country
		}
		for index, pair := range originalLeaguePairs {
			t.Fixtures = append(t.Fixtures, Fixture{Home: order[pair[0]], Away: order[pair[1]], Round: index/4 + 1, Played: index < completed, ScoresUnknown: index < completed, Winner: -1})
		}
		var total int
		for country := 0; country < 8; country++ {
			r := header[6+country*24:]
			v := make([]int, 7)
			for i := range v {
				v[i] = int(binary.BigEndian.Uint16(r[6+2*i:]))
			}
			if v[0] > 14 || v[0] != v[1]+v[2]+v[3] || v[6] != 3*v[1]+v[2] {
				return nil, fmt.Errorf("invalid original league totals")
			}
			t.OriginalFile.BaseTable[country] = Standing{country, v[0], v[1], v[2], v[3], v[4], v[5], v[6]}
			total += v[0]
		}
		if total != completed*2 {
			return nil, fmt.Errorf("original league count disagrees with its table")
		}
		t.Round = 14
		if completed < 56 {
			t.Round = completed/4 + 1
		}
	}
	t.OriginalFile.Completed = 0
	gap := false
	for _, fixture := range t.Fixtures {
		if fixture.Played {
			if gap {
				return nil, fmt.Errorf("original competition results are not a completed fixture prefix")
			}
			t.OriginalFile.Completed++
		} else {
			gap = true
		}
	}
	t.rebuildTable()
	if !cup && completed == 56 {
		t.Champion = t.Table[0].Team
	}
	return t, nil
}

func (t *Tournament) decodeOriginalOptions(header []byte) error {
	v := make([]int, 13)
	for i := range v {
		v[i] = int(binary.BigEndian.Uint16(header[584+2*i:]))
	}
	if v[0] > 4 || v[1] > 3 || v[2] > 3 || v[3] > 1 || v[4] > 1 || v[5] > 2 || v[6] > 2 || v[7] > 4 || v[8] > 4 || v[9] > 1 || v[12] >= len(Referees) {
		return fmt.Errorf("invalid original competition options")
	}
	t.Pitch = PitchFromOriginalIndex(uint16(v[0]))
	t.HalfSeconds = [4]int{180, 300, 600, 1200}[v[1]]
	t.Wind, t.MenuExtraTime, t.Aftertouch, t.LeagueSkill = v[2], v[3] != 0, v[4] != 0, v[5]
	t.Speed = [3]float64{1, .5, .25}[v[6]]
	t.MenuSkills = [2]int{v[7], v[8]}
	t.Referee = -1
	if v[9] != 0 {
		t.Referee = v[12]
	}
	return nil
}

func (t *Tournament) decodeOriginalCup(header []byte, stage int) error {
	var order [8]int
	for country, rank := range t.DrawPositions {
		order[rank] = country
	}
	participants := order[:]
	for round := 1; round <= 3; round++ {
		winners := []int{}
		allPlayed := true
		for index := 0; index < len(participants); index += 2 {
			home, away := participants[index], participants[index+1]
			a := binary.BigEndian.Uint16(header[6+home*24+6+round*2:])
			b := binary.BigEndian.Uint16(header[6+away*24+6+round*2:])
			if a != 0 && b != 0 {
				return fmt.Errorf("two original cup winners share one fixture")
			}
			winner := -1
			if a != 0 {
				winner = home
			} else if b != 0 {
				winner = away
			}
			played := winner >= 0
			t.Fixtures = append(t.Fixtures, Fixture{Home: home, Away: away, Round: round, Played: played, ScoresUnknown: played, Winner: winner})
			allPlayed = allPlayed && played
			if played {
				winners = append(winners, winner)
			}
		}
		if !allPlayed {
			t.Round = round
			break
		}
		if round == 3 {
			t.Round, t.Champion = 3, winners[0]
			break
		}
		participants = winners
	}
	expected := t.Round - 1
	if t.Champion >= 0 {
		expected = 3
	}
	if expected != stage {
		return fmt.Errorf("original cup round disagrees with its bracket")
	}
	return nil
}

// OriginalFileBytes writes a loadable .lge/.cup stream without inventing old
// match scores. Legacy address identifiers are confined to this file boundary.
func (t *Tournament) OriginalFileBytes() ([]byte, error) {
	if t == nil || t.Pitch < Normal || t.Pitch > RandomPitch || t.Wind < 0 || t.Wind > 3 || t.LeagueSkill < 0 || t.LeagueSkill > 2 || t.Referee < -1 || t.Referee >= len(Referees) {
		return nil, fmt.Errorf("invalid competition options")
	}
	for _, skill := range t.MenuSkills {
		if skill < 0 || skill > 4 {
			return nil, fmt.Errorf("invalid competition menu skill")
		}
	}
	var seenRanks, seenTeams [8]bool
	for country, rank := range t.DrawPositions {
		if rank < 0 || rank >= 8 || seenRanks[rank] || t.Table[country].Team < 0 || t.Table[country].Team >= 8 || seenTeams[t.Table[country].Team] {
			return nil, fmt.Errorf("invalid competition ranks")
		}
		seenRanks[rank], seenTeams[t.Table[country].Team] = true, true
	}
	header := make([]byte, originalTournamentHeaderSize)
	body := make([]byte, originalTournamentTeamsSize)
	if t.OriginalFile != nil {
		copy(header, t.OriginalFile.Header)
		copy(body, t.OriginalFile.Teams)
	} else {
		binary.BigEndian.PutUint16(header[606:], 1)
	}
	put := func(offset, value int) error {
		if value < 0 || value > 65535 {
			return fmt.Errorf("competition value does not fit the original word")
		}
		binary.BigEndian.PutUint16(header[offset:], uint16(value))
		return nil
	}
	completed := 0
	gap := false
	for _, fixture := range t.Fixtures {
		if fixture.Played {
			if gap {
				return nil, fmt.Errorf("original saves require a completed fixture prefix")
			}
			completed++
		} else {
			gap = true
		}
	}
	stage := completed
	if t.Cup {
		stage = t.Round - 1
		if t.Champion >= 0 {
			stage = 3
		}
	}
	if err := put(0, stage); err != nil {
		return nil, err
	}
	controls := t.CountryControls()
	var ranks [8]int
	for rank, row := range t.Table {
		ranks[row.Team] = rank
	}
	base := uint32(originalTournamentPoolBase)
	if t.OriginalFile != nil {
		base = binary.BigEndian.Uint32(header[6:])
	}
	for country := 0; country < 8; country++ {
		at := 6 + country*24
		binary.BigEndian.PutUint32(header[at:], base+uint32(country*originalTournamentTeamSize))
		_ = put(at+4, int(controls[country]))
		for off := 6; off <= 18; off += 2 {
			_ = put(at+off, 0)
		}
		if t.Cup {
			_ = put(at+20, t.DrawPositions[country])
			_ = put(at+22, country)
		} else {
			row := t.Table[ranks[country]]
			for index, value := range []int{row.Played, row.Won, row.Drawn, row.Lost, row.For, row.Against, row.Points} {
				if err := put(at+6+2*index, value); err != nil {
					return nil, err
				}
			}
			_ = put(at+20, ranks[country])
			_ = put(at+22, t.DrawPositions[country])
		}
	}
	if t.Cup {
		t.encodeOriginalCup(header, stage)
	}
	lengthIndex, speedIndex := -1, -1
	for i, value := range []int{180, 300, 600, 1200} {
		if t.HalfSeconds == value {
			lengthIndex = i
		}
	}
	for i, value := range []float64{1, .5, .25} {
		if t.Speed == value {
			speedIndex = i
		}
	}
	if lengthIndex < 0 || speedIndex < 0 {
		return nil, fmt.Errorf("duration or speed has no original option")
	}
	for i, value := range []int{int(t.Pitch.OriginalIndex()), lengthIndex, t.Wind, boolIndex(t.MenuExtraTime), boolIndex(t.Aftertouch), t.LeagueSkill, speedIndex, t.MenuSkills[0], t.MenuSkills[1], boolIndex(t.Referee >= 0)} {
		if err := put(584+2*i, value); err != nil {
			return nil, err
		}
	}
	if t.Referee >= 0 {
		_ = put(608, t.Referee)
	}
	custom := []byte{}
	for country := 0; country < 8; country++ {
		team := body[country*originalTournamentTeamSize : (country+1)*originalTournamentTeamSize]
		squad, err := OriginalSquad(t.SquadFiles[country])
		if len(t.UserSquads[country]) != 0 {
			squad, err = DecodeSquad(t.UserSquads[country])
		}
		if err != nil {
			return nil, err
		}
		if len(squad.Players) > 24 || len(squad.Players) < 13 {
			return nil, fmt.Errorf("original competition squads require13..24 players")
		}
		if originalLatin1(team[:20]) != t.Names[country] {
			if err := putOriginalText(team[:20], t.Names[country]); err != nil {
				return nil, err
			}
		}
		binary.BigEndian.PutUint16(team[20:], uint16(len(squad.Players)))
		records, valid := CanonicalSquadSelection(squad.Players, t.SelectedSquadRecords[country], t.HasSelectedSquadRecords[country], t.SelectedSquadIDs[country])
		if !valid || !completeTournamentSelection(squad, records) {
			return nil, fmt.Errorf("invalid original competition record selection")
		}
		var found [13]bool
		for index, player := range squad.Players {
			r := player.Record
			slot := SquadSelectionSlot(records, player.RecordIndex)
			shirt, binding := 0, uint8(0)
			if t.TeamSetups[country].HasUnassignedControllers {
				binding = t.TeamSetups[country].UnassignedControllers[player.RecordIndex]
			} else {
				binding = player.Record[21] >> 4
			}
			if slot >= 0 {
				binding = 0
				if found[slot] {
					return nil, fmt.Errorf("duplicate original shirt")
				}
				found[slot] = true
				shirt = slot + 1
				if slot == 12 {
					shirt = 14
				}
				if t.TeamSetups[country].HasBindings {
					binding = t.TeamSetups[country].Bindings[slot]
				}
				if binding > 2 {
					return nil, fmt.Errorf("unsupported original tournament controller")
				}
			}
			r[21] = uint8(shirt) | binding<<4
			copy(team[22+index*42:], r[:])
		}
		for _, ok := range found {
			if !ok {
				return nil, fmt.Errorf("original competition squad is incomplete")
			}
		}
		config, err := t.CountryConfig(DefaultConfig(), country)
		if err != nil {
			return nil, err
		}
		bank := OriginalAssignedTactics(config, 0)
		for slot, id := range bank.Slots {
			value := uint32(id)
			if id == OriginalCustomTacticID {
				data := originalAssignedTacticData(config, 0, slot)
				if _, err := DecodeTactic(data); err != nil {
					return nil, err
				}
				value = binary.BigEndian.Uint32(team[0x406+slot*4:])
				if value < 20 {
					value = uint32(0x50000 + country*0x1000 + slot*0x400)
				}
				block := make([]byte, 810)
				if t.OriginalFile != nil {
					copy(block, t.OriginalFile.Custom[country][slot])
				}
				name := config.AssignedTacticNames[0][slot]
				if t.OriginalFile == nil || len(t.OriginalFile.Custom[country][slot]) != 810 || originalLatin1(block[:10]) != name {
					name = originalTacticName(name)
					if err := putOriginalText(block[:10], name); err != nil {
						return nil, err
					}
				}
				copy(block[10:], data[:800])
				custom = append(custom, block...)
			}
			binary.BigEndian.PutUint32(team[0x406+slot*4:], value)
		}
		binary.BigEndian.PutUint16(team[0x416:], uint16(bank.Active))
		kit := t.Kits[country]
		if !kit.Valid() {
			return nil, fmt.Errorf("invalid competition kit")
		}
		for i, value := range []int{kit.Style, kit.Shirt, kit.Shorts} {
			binary.BigEndian.PutUint16(team[0x418+2*i:], uint16(value))
		}
	}
	result := append(header, body...)
	result = append(result, custom...)
	if t.OriginalFile != nil {
		result = append(result, t.OriginalFile.Trailing...)
	}
	if len(result) > 65536 {
		return nil, fmt.Errorf("original competition export exceeds its bounded file size")
	}
	return result, nil
}

func boolIndex(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (t *Tournament) encodeOriginalCup(header []byte, stage int) {
	put := func(country, offset, value int) {
		binary.BigEndian.PutUint16(header[6+country*24+offset:], uint16(value))
	}
	current := stage + 1
	if current > 3 {
		current = 3
	}
	for _, fixture := range t.Fixtures {
		// Each reached round assigns fixture letters before any result.
		letter := int('A')
		for _, prior := range t.Fixtures {
			if prior.Round == fixture.Round {
				if prior.Home == fixture.Home && prior.Away == fixture.Away {
					break
				}
				letter++
			}
		}
		for _, country := range []int{fixture.Home, fixture.Away} {
			put(country, 4+fixture.Round*2, letter)
		}
		if fixture.Played {
			loser := fixture.Home
			if loser == fixture.Winner {
				loser = fixture.Away
			}
			put(loser, 14, 1)
			put(fixture.Winner, 6+fixture.Round*2, int('*'))
			if fixture.Round == current {
				put(fixture.Home, 16, 1)
				put(fixture.Away, 16, 1)
			}
		}
	}
}
