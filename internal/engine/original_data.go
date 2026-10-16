package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/olivierh59500/kickoff2/assets"
	"math"
	"path"
	"strings"
)

// Tactic stores the twenty original positional targets for each outfield player.
// The first twelve correspond to three columns and four rows of ball territory;
// the remaining eight cover kickoffs, goal kicks and both sides of a corner.
type Tactic struct{ Targets [10][20]Vec2 }

// DecodeTactic reads the first 800 bytes as ten consecutive 80-byte records,
// matching the original file-loading routine at address 0x149de.
func DecodeTactic(data []byte) (Tactic, error) {
	var tactic Tactic
	if len(data) < 800 {
		return tactic, errors.New("original tactic file is truncated")
	}
	for i := range tactic.Targets {
		for j := range tactic.Targets[i] {
			offset := i*80 + j*4
			v := Vec2{float64(binary.BigEndian.Uint16(data[offset:])), float64(binary.BigEndian.Uint16(data[offset+2:]))}
			if v.X > PitchWidth || v.Y > PitchHeight {
				return Tactic{}, errors.New("tactic target lies outside the original pitch")
			}
			tactic.Targets[i][j] = v
		}
	}
	return tactic, nil
}

var originalTactics = func() [FormationCount]Tactic {
	var tactics [FormationCount]Tactic
	for i, name := range [...]string{"4-2-4.tac", "4-3-3.tac", "5-3-2.tac", "4-4-2.tac", "balanced.tac", "blitz.tac", "falcon.tac", "lockout.tac", "zbreak.tac", "zxcross.tac"} {
		data, err := assets.Files.ReadFile("original/raw/" + name)
		if err != nil {
			panic(err)
		}
		tactics[i], err = DecodeTactic(data)
		if err != nil {
			panic(err)
		}
	}
	return tactics
}()

// TacticPosition applies the original normal-play zone lookup. Raw tactic files
// face downwards. Reversing ends reflects the target coordinates and reverses the
// twelve-zone index, as in the original routines at 0xa8b2, 0xc712, and 0x14908.
func TacticPosition(formation Formation, player int, ball Vec2, attackDirection int) Vec2 {
	player = clampInt(player, 1, 10)
	column := clampInt(int(ball.X/304), 0, 2)
	row := clampInt(int(ball.Y/348), 0, 3)
	zone := column*4 + row
	if attackDirection < 0 {
		zone = 11 - zone
	}
	return originalTacticTarget(formation, player, zone, attackDirection)
}

func originalTacticTarget(formation Formation, player, targetIndex, attackDirection int) Vec2 {
	player = clampInt(player, 1, 10)
	target := originalTactics[clampInt(int(formation), 0, int(FormationCount)-1)].Targets[player-1][targetIndex]
	if attackDirection < 0 {
		target.Y = PitchHeight - target.Y
		// The corner targets retain their world-side X coordinates (0x14914).
		if targetIndex < 16 {
			target.X = PitchWidth - target.X
		}
	}
	return target
}

// RestartTacticPosition selects the eight original special targets. Kickoff
// states 2/3 use targets 12/13; goal-kick states 16/17 use 14/15. Corner states
// 62/61 use defending/attacking targets 16/17 on the left and 18/19 on the right.
// The caller's team direction determines end reflection, while corner.X chooses
// the absolute side of the ground, as in 0xea7c and 0xeaa8.
func RestartTacticPosition(formation Formation, player int, kind RestartKind, taking bool, corner Vec2, attackDirection int) (Vec2, bool) {
	targetIndex := 0
	switch kind {
	case KickOff:
		targetIndex = 12
		if !taking {
			targetIndex++
		}
	case GoalKick:
		targetIndex = 14
		if !taking {
			targetIndex++
		}
	case Corner:
		targetIndex = 16
		if taking {
			targetIndex++
		}
		if corner.X >= PitchWidth/2 {
			targetIndex += 2
		}
	default:
		return Vec2{}, false
	}
	return originalTacticTarget(formation, player, targetIndex, attackDirection), true
}

// TacticPreviewPosition exposes the menu preview target used at 0x6ffe. The
// source selects target fifteen and reflects only Y before its 1:12 drawing.
func TacticPreviewPosition(formation Formation, player int) Vec2 {
	target := originalTactics[clampInt(int(formation), 0, int(FormationCount)-1)].Targets[clampInt(player, 1, 10)-1][15]
	return Vec2{target.X, PitchHeight - target.Y}
}

// Squad preserves all original records, including selection and reserve numbers.
type Squad struct {
	Name    string
	Players []SquadPlayer
}

// SquadPlayer contains the decoded subset verified in routine 0x9e80, together
// with the entire raw record so undecoded fields are never discarded.
type SquadPlayer struct {
	// ID is the original name-generator seed. Different records may share it.
	ID uint32
	// RecordIndex stays in original file order when a menu sorts the view.
	RecordIndex                 int
	Name                        string
	Age, Role, Division, Number int
	Attributes                  Attributes
	Record                      [42]byte
}

// DecodeSquad validates the twenty-character header, player count, and 42-byte
// records. Player identities are name-generator seeds, not text pointers.
func DecodeSquad(data []byte) (Squad, error) {
	var squad Squad
	if len(data) < 22 {
		return squad, errors.New("original squad header is truncated")
	}
	count := int(binary.BigEndian.Uint16(data[20:22]))
	if count < 11 || count > 64 || len(data) != 22+count*42 {
		return squad, errors.New("original squad record count does not match its data")
	}
	squad.Name = string(bytes.TrimRight(bytes.SplitN(data[:20], []byte{0}, 2)[0], " "))
	for i := 0; i < count; i++ {
		r := data[22+i*42 : 22+(i+1)*42]
		p := SquadPlayer{ID: binary.BigEndian.Uint32(r), RecordIndex: i, Age: int(r[4]), Role: int(r[5]), Division: int(r[6]), Number: int(r[0x15] & 15)}
		p.Name = originalPlayerNames[p.ID]
		if p.Name == "" {
			p.Name = OriginalPlayerName(p.ID)
		}
		copy(p.Record[:], r)
		p.Attributes = OriginalAttributes(p.Record, false)
		squad.Players = append(squad.Players, p)
	}
	return squad, nil
}

// OriginalAttributes translates initialization at 0x9e4a and 0x9f16. Signed
// form is divided by four with an arithmetic shift; the home squad gains ten
// before clipping to 0..200 and dividing by two. The normalized output retains
// those integer results rather than recomputing them with floating-point roots.
func OriginalAttributes(record [42]byte, home bool) Attributes {
	return originalSelectedAttributes(record, home, record[5] == 1)
}

// The source chooses the keeper calculation by selected shirt zero, not by
// the raw positional role. Squad edits can place another record in that shirt.
func originalSelectedAttributes(record [42]byte, home, keeper bool) Attributes {
	technique := func(offset int) float64 {
		value := originalSquareRoot(int(record[offset]) * int(record[0xd]))
		value += int(int8(record[0x18])) >> 2
		if home {
			value += 10
		}
		return float64(clampInt(value, 0, 200)>>1) / 100
	}
	a := Attributes{Pace: clamp(float64(record[0xc])/200, 0, 1),
		Stamina:    clamp(float64(record[0xa]>>1)/100, 0, 1),
		Aggression: clamp(float64(record[0xe])/200, 0, 1),
		Resilience: clamp(float64(record[0xb]>>1)/100, 0, 1),
		Passing:    technique(0x10), Shooting: technique(0x11), Tackling: technique(0x12)}
	if keeper {
		a.Passing, a.Shooting = technique(0x13), technique(0x13)
		a.Tackling = 0
	}
	return a
}

// originalInitialAggressionThreshold retains9F76's byte subtraction before
// its word shift. SUB.B wraps at eight bits, including raw values above200.
func originalInitialAggressionThreshold(p *Player) int {
	if p.OriginalRecord != ([42]byte{}) {
		return int(byte(200-p.OriginalRecord[0xE]) >> 1)
	}
	return 100 - int(math.Round(p.Attributes.Aggression*100))
}

// This is BC40's literal DIVU.W/EXT.L iteration. The quotient is extended as a
// signed word, overflow leaves the dividend unchanged, and the stop comparison
// examines only the low word. Allocator/cached values with bit31 set must not
// enter a signed Go Newton loop that can oscillate forever.
func originalSquareRoot(value int) int {
	dividend, divisor := uint32(value), uint32(65535)
	for {
		quotient := dividend / uint32(uint16(divisor))
		word := uint16(dividend)
		if quotient <= 65535 {
			word = uint16(quotient)
		}
		next := (divisor + uint32(int32(int16(word)))) >> 1
		difference := int16(divisor - next)
		if difference >= -1 && difference <= 1 {
			return int(next)
		}
		divisor = next
	}
}

// OriginalSquad reads one embedded original squad by its disk filename.
func OriginalSquad(filename string) (Squad, error) {
	data, err := assets.Files.ReadFile("original/raw/" + path.Base(filename))
	if err != nil {
		return Squad{}, err
	}
	return DecodeSquad(data)
}

// originalRuntimeName follows9E14: copy at most15 bytes, stopping at NUL,
// and mask any byte with bit6 set. Menus retain the original header spelling.
func originalRuntimeName(name string) string {
	result := make([]byte, 0, min(15, len(name)))
	for index := 0; index < len(name) && index < 15 && name[index] != 0; index++ {
		value := name[index]
		if value&0x40 != 0 {
			value &= 0x5F
		}
		result = append(result, value)
	}
	return string(result)
}

// CanonicalSquadSelection resolves a squad selection to source-record
// references. References store RecordIndex plus one so zero remains a vacant
// shirt. A marked record selection is authoritative. Older seed selections are
// migrated by assigning distinct matching records, with the original shirt as
// the first tie breaker and file order as the second. With no old selection,
// the source records' shirt nibbles provide the default.
func CanonicalSquadSelection(players []SquadPlayer, records [13]uint8, hasRecords bool, legacyIDs [13]uint32) ([13]uint8, bool) {
	byRecord := make(map[int]SquadPlayer, len(players))
	for _, player := range players {
		if player.RecordIndex < 0 || player.RecordIndex >= 255 {
			return [13]uint8{}, false
		}
		if _, duplicate := byRecord[player.RecordIndex]; duplicate {
			return [13]uint8{}, false
		}
		byRecord[player.RecordIndex] = player
	}

	if hasRecords {
		used := make(map[uint8]bool, len(records))
		for _, record := range records {
			if record == 0 {
				continue
			}
			if _, present := byRecord[int(record)-1]; !present || used[record] {
				return [13]uint8{}, false
			}
			used[record] = true
		}
		return records, true
	}

	hasLegacy := false
	for _, seed := range legacyIDs {
		hasLegacy = hasLegacy || seed != 0
	}
	if hasLegacy {
		var migrated [13]uint8
		used := make(map[int]bool, len(migrated))
		// Resolve records already carrying the requested shirt first. Doing this
		// as a separate pass keeps an earlier vacant shirt from consuming a
		// duplicate seed that exactly belongs to a later shirt.
		for slot, seed := range legacyIDs {
			if seed == 0 {
				continue
			}
			index := selectionRecordCandidate(players, used, seed, selectionShirt(slot), true)
			if index >= 0 {
				migrated[slot], used[index] = uint8(index+1), true
			}
		}
		for slot, seed := range legacyIDs {
			if seed == 0 || migrated[slot] != 0 {
				continue
			}
			index := selectionRecordCandidate(players, used, seed, 0, false)
			if index < 0 {
				return [13]uint8{}, false
			}
			migrated[slot], used[index] = uint8(index+1), true
		}
		return migrated, true
	}

	var derived [13]uint8
	for _, player := range players {
		slot := selectionSlotForShirt(player.Number)
		if slot < 0 {
			continue
		}
		// 9E80 rescans from record zero for each shirt and stops at the first
		// match, independently of any sorted menu view.
		record := uint8(player.RecordIndex + 1)
		if derived[slot] == 0 || record < derived[slot] {
			derived[slot] = record
		}
	}
	return derived, true
}

func selectionRecordCandidate(players []SquadPlayer, used map[int]bool, seed uint32, shirt int, requireShirt bool) int {
	best := -1
	for _, player := range players {
		if used[player.RecordIndex] || player.ID != seed || requireShirt && player.Number != shirt {
			continue
		}
		if best < 0 || player.RecordIndex < best {
			best = player.RecordIndex
		}
	}
	return best
}

func selectionShirt(slot int) int {
	if slot == 12 {
		return 14
	}
	return slot + 1
}

func selectionSlotForShirt(shirt int) int {
	if shirt >= 1 && shirt <= 12 {
		return shirt - 1
	}
	if shirt == 14 {
		return 12
	}
	return -1
}

// SquadSelectionIDs returns the legacy seed mirror for record references.
// Zero-seed records intentionally remain zero; the record marker carries their
// identity for current settings.
func SquadSelectionIDs(players []SquadPlayer, records [13]uint8) [13]uint32 {
	byRecord := make(map[int]uint32, len(players))
	for _, player := range players {
		if player.RecordIndex >= 0 && player.RecordIndex < 255 {
			byRecord[player.RecordIndex] = player.ID
		}
	}
	var ids [13]uint32
	for slot, record := range records {
		if record != 0 {
			ids[slot] = byRecord[int(record)-1]
		}
	}
	return ids
}

// SquadSelectionSlot returns the shirt slot occupied by one source record.
func SquadSelectionSlot(records [13]uint8, recordIndex int) int {
	if recordIndex < 0 || recordIndex >= 255 {
		return -1
	}
	record := uint8(recordIndex + 1)
	for slot, selected := range records {
		if selected == record {
			return slot
		}
	}
	return -1
}

func (m *Match) loadOriginalRoster(team int) {
	filename := m.Config.SquadFiles[team]
	if filename == "" {
		filename = InternationalSquad(m.Config.TeamNames[team])
	}
	if filename == "" {
		filename = [...]string{"div4.sqd", "div3.sqd", "div2.sqd", "div1.sqd", "intern.sqd"}[clampInt(m.Config.Skill[team], 0, 4)]
	}
	squad, err := OriginalSquad(filename)
	if len(m.Config.UserSquads[team]) != 0 {
		squad, err = DecodeSquad(m.Config.UserSquads[team])
	}
	if err != nil {
		return
	}
	requested := m.Config.HasSelectedSquadRecords[team] || m.Config.SelectedSquadIDs[team] != ([13]uint32{})
	selected, valid := CanonicalSquadSelection(squad.Players, m.Config.SelectedSquadRecords[team], m.Config.HasSelectedSquadRecords[team], m.Config.SelectedSquadIDs[team])
	if !valid {
		m.Config.SelectedSquadRecords[team] = [13]uint8{}
		m.Config.HasSelectedSquadRecords[team] = false
		m.Config.SelectedSquadIDs[team] = [13]uint32{}
		selected, _ = CanonicalSquadSelection(squad.Players, [13]uint8{}, false, [13]uint32{})
		requested = false
	} else if requested {
		m.Config.SelectedSquadRecords[team] = selected
		m.Config.HasSelectedSquadRecords[team] = true
		m.Config.SelectedSquadIDs[team] = SquadSelectionIDs(squad.Players, selected)
	}
	for _, source := range squad.Players {
		n := 0
		if slot := SquadSelectionSlot(selected, source.RecordIndex); slot >= 0 {
			n = selectionShirt(slot)
		}
		if n < 1 || n > 14 || n == 13 {
			continue
		}
		name := source.Name
		if name == "" {
			name = fmt.Sprintf("PLAYER %02d", n)
		}
		p := Player{Number: n, Name: originalRuntimeName(name), Keeper: n == 1, Stamina: 1, Attributes: originalSelectedAttributes(source.Record, team == 0, n == 1),
			OriginalID: source.ID, OriginalRecord: source.Record, Role: source.Role, lastTouch: -1000, Facing: Vec2{0, float64(m.AttackDirection(team))}}
		if p.Keeper {
			p.Role = 1 // 9F66 assigns the keeper role to selected shirt zero.
		}
		if n <= 11 {
			m.Teams[team].Players[n-1] = p
		} else {
			m.Teams[team].Bench[(n-12)/2] = p
		}
	}
}

// InternationalCountries is the original country-label order (labels 84..91).
var InternationalCountries = [8]string{"ENGLAND", "SPAIN", "FRANCE", "GERMANY", "ITALY", "SWEDEN", "HOLLAND", "SCOTLAND"}

// InternationalSquad maps original country selections to their disk squad file.
func InternationalSquad(name string) string {
	for i, country := range InternationalCountries {
		if strings.EqualFold(name, country) {
			return fmt.Sprintf("int%d.sqd", i+1)
		}
	}
	return ""
}

var originalPlayerNames = func() map[uint32]string {
	names := map[uint32]string{}
	data, err := assets.Files.ReadFile("original/squads.json")
	if err != nil {
		return names
	}
	var squads []struct {
		Players []struct {
			NameSeed uint32 `json:"name_seed"`
			Name     string `json:"name"`
		} `json:"players"`
	}
	if json.Unmarshal(data, &squads) != nil {
		return names
	}
	for _, squad := range squads {
		for _, player := range squad.Players {
			names[player.NameSeed] = player.Name
		}
	}
	return names
}()
