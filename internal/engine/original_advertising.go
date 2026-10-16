package engine

// OriginalAdvertisingState stores the signed contact latches at1D184/1D182.
// They belong to the original advertising/outer-board callback at A916.
type OriginalAdvertisingState struct {
	BoardContact, OuterContact int16
}

// OriginalAdvertisingStep translates A916..AAB4. The source runs this callback
// after ball/frame integration when drawing is enabled. Its shallow board bands,
// central left-side opening, lift impulses and outer limits use integer words.
func OriginalAdvertisingStep(ball OriginalBallState, contact OriginalAdvertisingState) (OriginalBallState, OriginalAdvertisingState) {
	x, y, height := int16(ball.X>>16), int16(ball.Y>>16), int16(ball.Height>>16)
	outer := func(checkX bool) (OriginalBallState, OriginalAdvertisingState) {
		if checkX && (x < -108 || x > 1036) {
			if contact.OuterContact == 0 {
				contact.OuterContact++
				ball.VelocityX = -(ball.VelocityX >> 2)
			}
			return ball, contact
		}
		if y < -58 || y > 1470 {
			if contact.OuterContact == 0 {
				contact.OuterContact++
				ball.VelocityX, ball.VelocityY = 0, 0
			}
		} else {
			contact.OuterContact = 0
		}
		return ball, contact
	}
	if height > 8 {
		return outer(true)
	}
	axis, impulse := 0, int32(0)
	switch {
	case x <= -52:
		if x < -68 {
			contact.BoardContact = -1
			return outer(true)
		}
		if y < -14 || y > 1408 || y >= 676 && y <= 716 {
			contact.BoardContact = 0
			return ball, contact
		}
		axis, impulse = 1, 4000
	case x >= 976:
		if x > 992 {
			contact.BoardContact = -1
			return outer(true)
		}
		if y < -14 || y > 1408 {
			contact.BoardContact = 0
			return ball, contact
		}
		axis, impulse = 1, -4000
	case y <= -46:
		if y < -62 {
			contact.BoardContact = 0
			return outer(false)
		}
		if x < -20 || x > 944 {
			contact.BoardContact = 0
			return ball, contact
		}
		axis, impulse = 2, 4000
	case y >= 1440:
		if y > 1456 {
			contact.BoardContact = 0
			return outer(false)
		}
		if x < -20 || x > 944 {
			contact.BoardContact = 0
			return ball, contact
		}
		axis, impulse = 2, -4000
	default:
		contact.BoardContact = 0
		return ball, contact
	}
	velocity := &ball.VelocityY
	if axis == 1 {
		velocity = &ball.VelocityX
	}
	*velocity += impulse
	if axis == 1 && contact.BoardContact < 0 {
		ball.VelocityX, ball.VelocityY = 0, 0
		return ball, contact
	}
	if contact.BoardContact == 0 {
		contact.BoardContact++
		half := *velocity >> 1
		*velocity -= half
		// BPL tests the remaining velocity after SUB, rather than the
		// half word itself. Retaining that detail also preserves tiny values.
		if *velocity < 0 {
			half = -half
		}
		ball.Lift += half
	}
	return ball, contact
}
