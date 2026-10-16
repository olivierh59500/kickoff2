package engine

// Referee preserves the original name and four temperament bytes consumed by
// the source foul/booking routines. Isolated decision parity and natural card
// presentation are validated separately.
type Referee struct {
	Name       string
	Attributes [4]byte
}

// Referees is the original twenty-four-entry table at executable address 0x172ec.
var Referees = []Referee{
	{"R.J. FERNANDEZ", [4]byte{15, 6, 4, 80}},
	{"F. ZAPPA", [4]byte{20, 200, 20, 85}},
	{"J. DANIELS", [4]byte{5, 3, 0, 30}},
	{"S. SCREECH", [4]byte{4, 200, 0, 80}},
	{"J. BRAWN", [4]byte{5, 1, 1, 70}},
	{"A.N. INDI", [4]byte{10, 200, 10, 25}},
	{"D. MORGAN", [4]byte{10, 5, 2, 70}},
	{"T. BOYLAN", [4]byte{20, 2, 2, 40}},
	{"N. RAJAH", [4]byte{15, 4, 4, 60}},
	{"B. BROWN", [4]byte{10, 3, 2, 80}},
	{"J. MARTYN", [4]byte{30, 10, 5, 100}},
	{"M. TAYLOR", [4]byte{17, 2, 1, 75}},
	{"R. HARPER", [4]byte{20, 20, 7, 60}},
	{"F. WILLIS", [4]byte{1, 0, 0, 75}},
	{"G. PENN", [4]byte{2, 0, 0, 100}},
	{"A. DENT", [4]byte{25, 7, 7, 90}},
	{"I. BALL", [4]byte{16, 1, 1, 100}},
	{"V. JONES", [4]byte{1, 0, 0, 40}},
	{"D. WOOLEY", [4]byte{20, 6, 6, 30}},
	{"D. DUFUS", [4]byte{16, 15, 2, 50}},
	{"G. ELLIS", [4]byte{25, 6, 1, 75}},
	{"S. REDPATH", [4]byte{20, 10, 5, 90}},
	{"J. CUPITT", [4]byte{5, 5, 1, 100}},
	{"M. SPURWAY", [4]byte{20, 10, 5, 90}},
}
