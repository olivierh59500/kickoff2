package engine

import (
	"encoding/json"
	"strings"

	"github.com/olivierh59500/kickoff2/assets"
)

var originalSurnames = func() []string {
	data, err := assets.Files.ReadFile("original/player-surnames.json")
	if err != nil {
		panic(err)
	}
	var surnames []string
	if err := json.Unmarshal(data, &surnames); err != nil || len(surnames) != 183 {
		panic("invalid original player surname bank")
	}
	return surnames
}()

// OriginalPlayerName reproduces the naming routine at0x8d42 for any imported
// squad seed. The48-bit register at0x16922 uses three sequential ROXL.W steps.
func OriginalPlayerName(seed uint32) string {
	if seed == 0 {
		// 8D46 skips the generator and copies the current Player Manager name.
		// The ordinary squad-loading path initializes that buffer to this literal.
		return "PLAYER-MANAGER"
	}
	a, b, c := uint16(seed>>16), uint16(seed), uint16(0)
	random := func() uint16 {
		feedback := (byte(b)>>3 ^ byte(c)) & 1
		oldA, oldB := a, b
		a = a<<1 | uint16(feedback)
		b = b<<1 | oldA>>15
		c = c<<1 | oldB>>15
		return a
	}
	for i := 0; i < 20; i++ {
		random()
	}
	value, count := random()%100, 1
	if value >= 98 {
		count = 3
	} else if value >= 75 {
		count = 2
	}
	var name strings.Builder
	for i := 0; i < count; i++ {
		value = random() % 100
		group := "O"
		if value < 44 {
			group = "ADJR"
		} else if value < 76 {
			group = "CEGMS"
		} else if value < 99 {
			group = "BFHILNTW"
		}
		name.WriteByte(group[int(random())%len(group)])
		name.WriteByte('.')
	}
	name.WriteString(originalSurnames[int(random())%len(originalSurnames)])
	return name.String()
}
