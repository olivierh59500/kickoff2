package engine

// OriginalGoalFrameState preserves the persistent words surrounding the ball.
// ContactMask is the byte modified by BSET.B at1D144; the following counters
// are signed words at1D146/1D148. OutKind0/1/2 mean inside/side/end, and3/4
// identify goals after applying the source end-orientation switch.
type OriginalGoalFrameState struct {
	ContactMask                   byte
	ContactTicks, ContactCooldown int16
	OutKind                       int16
	OutX, OutY                    int16
	EndsSwapped                   bool
}

var originalGoalCollisionAngles = [9][2]int32{
	{-10000, 0}, {-1250, -9922}, {5000, -8660}, {8750, -4842},
	{10000, 0}, {8750, 4842}, {5000, 8660}, {-1250, 9922}, {-10000, 0},
}

// originalFrameReflection preserves the source's sequential signed MULS/DIVS
// operations. Its second component intentionally consumes the newly computed
// first component at A630/A6C8; an orthogonal vector reflection differs.
func originalFrameReflection(first, second int32, angle int) (int32, int32) {
	pair := originalGoalCollisionAngles[angle]
	a := int32(int16(first>>6)) * pair[0]
	b := int32(int16(second>>6)) * pair[1]
	first = ((a + b) / 10000) << 6
	a = int32(int16(second>>6)) * pair[0]
	b = int32(int16(first>>6)) * pair[1]
	second = ((b - a) / 10000) << 6
	return first, second
}

// OriginalGoalFrameStep translates A51A..A834 after OriginalBallStep has
// integrated its free-flight values. previousY is the entry Y, needed because
// the source records a touchline crossing before integrating its Y component.
// This helper retains source post-contact flags, cooldown, net containment and
// goal/end detection. The bool identifies an original goal-frame sound event.
func OriginalGoalFrameStep(ball OriginalBallState, previousY int32, bounds OriginalGoalFrameState) (OriginalBallState, OriginalGoalFrameState, bool) {
	x, y, height := int16(ball.X>>16), int16(ball.Y>>16), int16(ball.Height>>16)
	if uint16(x) >= 912 {
		if bounds.OutKind == 0 {
			bounds.OutKind, bounds.OutX, bounds.OutY = 1, x, int16(previousY>>16)
		}
	} else if bounds.OutKind == 1 {
		bounds.OutKind = 0
	}
	if uint16(y) < 1392 {
		bounds.ContactMask, bounds.ContactTicks, bounds.ContactCooldown = 0, 0, 0
		if bounds.OutKind == 2 {
			bounds.OutKind = 0
		}
		return ball, bounds, false
	}
	if y <= -64 || y >= 1456 {
		ball.VelocityX, ball.VelocityY = 0, 0
		return ball, bounds, false
	}
	if bounds.OutKind == 3 || bounds.OutKind == 4 {
		if y <= -19 || y >= 1410 {
			ball.VelocityX >>= 1
			ball.VelocityY = 0
		}
		if x < 412 || x >= 500 {
			ball.VelocityX = 0
			ball.VelocityY >>= 1
		}
		if height >= 28 {
			ball.Lift = -ball.Lift
		}
		return ball, bounds, false
	}
	if bounds.OutKind == 2 {
		if y < -24 || y > 1416 || x < 404 || x > 508 || height > 36 {
			return ball, bounds, false
		}
		if x >= 412 && x <= 500 {
			if height < 32 {
				ball.VelocityX, ball.VelocityY = 0, 0
			} else {
				ball.Height -= ball.Lift
				ball.Lift = (-ball.Lift) >> 2
				if y > 696 {
					ball.VelocityY += 20000
				} else {
					ball.VelocityY -= 20000
				}
			}
		} else {
			ball.X -= ball.VelocityX
			ball.VelocityX = (-ball.VelocityX) >> 2
		}
		return ball, bounds, false
	}
	if y <= 1402 && y >= -10 && bounds.ContactCooldown > 0 {
		bounds.ContactCooldown--
		return ball, bounds, false
	}
	endOut := func() (OriginalBallState, OriginalGoalFrameState, bool) {
		if bounds.OutKind == 0 {
			bounds.OutKind, bounds.OutX, bounds.OutY = 2, x, y
		}
		return ball, bounds, false
	}
	goal := func(sound bool) (OriginalBallState, OriginalGoalFrameState, bool) {
		bounds.OutKind = 4
		if y < 0 {
			bounds.OutKind = 3
		}
		if bounds.EndsSwapped {
			bounds.OutKind ^= 7
		}
		return ball, bounds, sound
	}
	if x < 404 || x > 508 || height > 36 {
		return endOut()
	}
	if x > 412 && x < 500 && height < 28 {
		return goal(false)
	}
	sound := false
	halfVelocities := func() {
		ball.VelocityX -= ball.VelocityX >> 1
		ball.VelocityY -= ball.VelocityY >> 1
		ball.Lift -= ball.Lift >> 1
		bounds.ContactCooldown = 4
		sound = true
	}
	if height >= 28 && bounds.ContactMask&4 == 0 {
		bounds.ContactMask |= 4
		ball.Lift, ball.VelocityY = originalFrameReflection(ball.Lift, ball.VelocityY, int(height-28))
		if y < 0 {
			ball.Lift = -ball.Lift
		}
		halfVelocities()
	}
	angle := -1
	if x <= 412 && bounds.ContactMask&2 == 0 {
		bounds.ContactMask |= 2
		angle = int(x - 404)
	} else if x >= 500 && bounds.ContactMask&1 == 0 {
		bounds.ContactMask |= 1
		angle = int(x - 500)
	}
	if angle >= 0 {
		ball.VelocityX, ball.VelocityY = originalFrameReflection(ball.VelocityX, ball.VelocityY, angle)
		if y < 0 {
			ball.VelocityX = -ball.VelocityX
		}
		halfVelocities()
		return ball, bounds, sound
	}
	bounds.ContactTicks++
	if bounds.ContactTicks >= 10 {
		if x < 410 || x > 502 || height > 30 {
			result, next, _ := endOut()
			return result, next, sound
		}
		return goal(sound)
	}
	return ball, bounds, sound
}
