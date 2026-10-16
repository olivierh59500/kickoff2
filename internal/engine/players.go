package engine

import "math"

func (m *Match) homePosition(team, index int) Vec2 {
	if index == 0 {
		y := PitchHeight - 6
		if m.AttackDirection(team) == 1 {
			y = 6
		}
		return Vec2{PitchWidth / 2, y}
	}
	return TacticPosition(m.Teams[team].Formation, index, Vec2{PitchWidth / 2, PitchHeight / 2}, m.AttackDirection(team))
}

func (m *Match) nearest(team int, exclude int) int {
	best := -1
	distance := math.MaxFloat64
	target := m.Ball.Pos.Add(m.Ball.Vel.Mul(0.08))
	for i := 1; i < 11; i++ {
		p := &m.Teams[team].Players[i]
		if p.SentOff || i == exclude {
			continue
		}
		d := p.Pos.Sub(target).Len()
		if d < distance {
			best = i
			distance = d
		}
	}
	return best
}

func (m *Match) selectPlayers(input [2]Input) {
	m.selectPlayersForTeam(input, -1)
}

func (m *Match) selectPlayersForTeam(input [2]Input, selectedTeam int) {
	for t := range m.Teams {
		if selectedTeam >= 0 && selectedTeam != t {
			continue
		}
		if m.Config.Practice && t == 1 {
			continue
		}
		team := &m.Teams[t]
		if !m.Config.Human[t] {
			team.selectOwnPlayer(-1)
			continue
		}
		// An opposing reference must reach C7D4 as recorded. A preferred own
		// shirt or the separate possession pointer must not overwrite it first.
		if !team.ControlledOpposing {
			// Keep the pre-binding native configuration API usable: callers can
			// still change FixedPlayer directly. Explicit source rosters use B4.
			if !m.Config.HasSquadBindings[t] {
				if fixed := m.boundPrimaryPlayer(t); fixed >= 0 && !m.secondaryBoundPlayer(t, fixed) {
					team.selectOwnPlayer(fixed)
					continue
				}
			}
			if m.Ball.TrappedBy >= 0 && m.Ball.TrappedBy/11 == t && m.Ball.TrappedBy%11 != 0 && !m.secondaryBoundPlayer(t, m.Ball.TrappedBy%11) {
				team.selectOwnPlayer(m.Ball.TrappedBy % 11)
				continue
			}
		}
		m.selectOriginalHuman(t, input[t])
	}
}

func (m *Match) updatePlayers(input [2]Input) bool {
	dt := 1.0 / TPS
	firstTeam, firstActor, closestTeam, second := 0, 0, -1, -1
	if pending := m.originalCornerContinuation; pending.active {
		// D69A returns to the suspended actor, rather than dispatching it again.
		m.originalCornerContinuation.active = false
		m.finishOriginalPlayer(pending.team, pending.actor, pending.step)
		firstTeam, firstActor = pending.team, pending.actor+1
		closestTeam, second = pending.closest, pending.second
	} else if m.penaltyFlightTicks > 0 {
		m.penaltyFlightTicks--
	}
	for t := firstTeam; t < len(m.Teams); t++ {
		start := 0
		if t == firstTeam && firstActor > 0 {
			start = firstActor
		} else {
			if m.penaltyFlightTicks > 0 && t != m.penaltyTeam && m.Config.Human[t] {
				m.Teams[t].selectOwnPlayer(0)
			}
			closestTeam = m.originalNearest(t, -1)
			second = m.originalNearest(t, closestTeam)
		}
		for i := start; i < len(m.Teams[t].Players); i++ {
			p := &m.Teams[t].Players[i]
			previouslyVisible := p.originalVisible
			entryState := p.originalState
			entrySubstate := p.originalSubstate
			ballWasVisible := m.originalBallDrawEnabled
			if p.shotPauseTicks > 0 {
				p.shotPauseTicks--
			}
			if p.SentOff || p.originalState == 0 {
				p.Vel = Vec2{}
				m.updateOriginalScannerActor(t, i)
				continue
			}
			if p.ActionTicks > 0 {
				p.ActionTicks--
			}
			if p.tackleTicks > 0 {
				p.tackleTicks--
			}
			if p.jumpTicks > 0 {
				p.jumpTicks--
				p.JumpHeight = OriginalJumpHeight(p.jumpTicks)
			} else {
				p.JumpHeight = 0
			}
			if m.Config.Practice && t == 1 && i != 0 {
				p.Vel = Vec2{}
				continue
			}
			m.originalFatigue(t, i, p)
			if i == 0 {
				if m.penaltyFlightTicks > 0 && t != m.penaltyTeam && m.Config.Human[t] {
					m.updatePenaltyKeeper(t, p, input[t])
					m.recordOriginalPlayerVideo(t, i, previouslyVisible, entryState, entrySubstate)
					m.updateOriginalScannerActor(t, i)
					if p.touchCooldown > 0 {
						p.touchCooldown--
					}
					continue
				}
				m.updateKeeper(t, p, dt)
				if !ballWasVisible && m.originalBallDrawEnabled {
					m.recordOriginalBallVideo()
				}
				m.recordOriginalPlayerVideo(t, i, previouslyVisible, entryState, entrySubstate)
				m.updateOriginalScannerActor(t, i)
				if p.touchCooldown > 0 {
					p.touchCooldown--
				}
				continue
			}
			move := Vec2{}
			originalStep := -1
			tacticDistance := math.Inf(1)
			in, previous, isControlled := m.playerInput(t, i)
			if p.originalState == 44 && !isControlled {
				move, originalStep = m.originalShotActor(t, i)
			} else if p.originalState == 4 {
				move, originalStep = m.updateOriginalDepartingPlayer(t, p)
			} else if p.originalState == 54 || p.originalState == 34 {
				originalStep = 0
				if p.originalState == 34 && m.originalOfficialsEnabled {
					p.Facing = OriginalDirection(0)
				}
			} else if p.originalState == 55 || p.originalState == 59 {
				var arrived bool
				move, originalStep, arrived = m.originalRunToTarget(p, p.originalRestartTarget)
				if p.originalState == 59 && arrived {
					p.originalState, p.originalDirectionHistory = 1, -1
				}
			} else if p.originalState >= 56 && p.originalState <= 58 {
				move, originalStep = m.updateOriginalCloseFreeKickTaker(t, i, in, isControlled)
			} else if p.originalState == 60 {
				move, originalStep = m.updateOriginalCloseFreeKickParticipant(t, i)
			} else if p.originalState == 41 || p.originalState == 42 {
				move, originalStep = m.updateOriginalFoulPenaltyTaker(t, i, in)
			} else if p.originalState == 43 {
				move, originalStep = m.updateOriginalPenaltyParticipant(t, i)
			} else if p.originalState == 26 || p.originalState == 27 {
				fixed := isControlled && m.originalRecordHasController(t, i)
				move, originalStep = m.updateOriginalFreeKickActor(t, i, closestTeam, second, in, fixed)
			} else if p.originalState == 40 {
				move, originalStep = m.updateOriginalFreeKickRetreat(p)
			} else if p.originalState == 28 {
				var arrived bool
				move, originalStep, arrived = m.originalRunToTarget(p, m.originalFoul.Pos)
				if arrived {
					p.originalState, p.originalDirectionHistory = 31, -1
				}
			} else if p.originalState == 33 {
				move, originalStep = m.updateOriginalEnteringPlayer(t, p)
			} else if p.originalState == 35 || p.originalState == 36 {
				move, originalStep = m.updateOriginalCelebratingPlayer(t, i)
			} else if p.originalState == 31 || p.originalState == 32 {
				move, originalStep = m.updateOriginalKickoffPlayer(t, i)
			} else if p.originalState >= 21 && p.originalState <= 25 || p.originalState == 61 || p.originalState == 62 {
				move, originalStep = m.updateOriginalCornerActor(t, i, in)
			} else if p.originalState >= 5 && p.originalState <= 10 || p.originalState == 29 {
				move, originalStep = m.updateOriginalThrowActor(t, i, in, isControlled && m.Config.Human[t])
			} else if p.originalState == 13 {
				move, originalStep = m.updateOriginalJump(t, i, in, isControlled && m.Config.Human[t])
			} else if p.originalState == 12 {
				move, originalStep = m.updateOriginalTackle(t, i)
			} else if p.originalState == 63 || p.originalState == 14 || p.originalState == 15 || p.originalState == 30 {
				move, originalStep = m.updateOriginalSpecialPlayer(t, i)
			} else if (p.originalState == 16 || p.originalState == 17) && m.originalRecordHasController(t, i) {
				move, originalStep = m.updateOriginalFixedGoalKickActor(t, i, in)
			} else if p.originalState == 2 || p.originalState == 3 || p.originalState == 16 || p.originalState == 17 {
				// DE3E/DE4E stage every kickoff participant, including fixed
				// controllers. E2D4/E2F2 permit fixed goal-kick movement.
				kind, taking := GoalKick, p.originalState == 16
				if p.originalState == 2 || p.originalState == 3 {
					kind, taking = KickOff, p.originalState == 2
				}
				target, _ := m.restartTacticPosition(t, i, kind, taking, m.Ball.Pos, m.AttackDirection(t))
				if originalDistanceSquared(p.Pos, target) > 16 {
					move = originalTargetHeading(p, target)
					p.Facing = move
					originalStep = originalPace(p)
				} else {
					originalStep = 0
					p.Facing = OriginalDirection(OriginalDirectionIndex(Vec2{math.Floor(m.Ball.Pos.X) - math.Floor(p.Pos.X), math.Floor(m.Ball.Pos.Y) - math.Floor(p.Pos.Y)}))
					p.originalDirectionHistory = -1
				}
			} else if isControlled && m.Config.Human[t] {
				move, originalStep = m.updateOriginalHuman(t, i, in, previous)
			} else if m.originalBallOwner == t*11+i {
				move, originalStep = m.updateOriginalAIOwner(t, i)
			} else if m.originalAIJump(t, i) {
				move, originalStep = p.Facing, originalPace(p)
			} else if contactMove, contactStep, contact := m.originalAIContact(t, i, closestTeam, second); contact {
				move, originalStep = contactMove, contactStep
			} else if m.originalAINearClaim(t, i) {
				// DBB8's aggression draw precedes the C7BE/119A8
				// possession claim, even when no tackle is selected.
				move, originalStep = p.Facing, p.originalStep
			} else {
				target := m.tacticPosition(t, i, m.Ball.Pos, m.AttackDirection(t))
				tacticHold := false
				chase := false
				owner := m.originalBallOwner
				ownOwner := owner >= 0 && owner/11 == t
				closest := closestTeam
				// 0xdb5e admits only nearby actors or the two nearest actors
				// to the possession-support routine; distant teammates retain
				// their formation run even beside the intended pass receiver.
				ballDecision, secondGate := m.originalAIBallDecision(t, i, closest, second)
				if secondGate {
					// 0xdb90..0xdbaa stops close support before the possession
					// routine, retaining both heading and direction history.
					target = p.Pos
					tacticHold = true
					originalStep = 0
				} else if owner < 0 && p.originalBallDistance <= 64 {
					// 0x119cc stops below a loose ball before testing height.
					// A missed aerial attempt still tracks its incoming heading.
					target = p.Pos
					tacticHold = true
					p.Facing = m.originalChaseHeading(p)
					originalStep = 0
				} else if !ownOwner && i == second && closest >= 0 && originalDistanceSquared(p.Pos, m.Teams[t].Players[closest].Pos) <= 10000 {
					target = p.Pos
					tacticHold = true
					// 0x11a3a clears movement but still enters the ball-heading
					// routine at0x11a3e; stopped support keeps anticipating.
					p.Facing = m.originalChaseHeading(p)
					originalStep = 0
				} else if ownOwner && ballDecision && originalDistanceSquared(p.Pos, m.Teams[t].Players[owner%11].Pos) <= 2500 {
					target = p.Pos
					tacticHold = true
				} else if ballDecision {
					target = m.Ball.Pos
					originalStep = originalPace(p)
					chase = true
				}
				tacticDistance = math.Sqrt(float64(originalDistanceSquared(p.Pos, target)))
				// 119A8 chases using the cached ball distance; a ball that
				// moved close during this update does not use C736's arrival
				// radius. Only a tactical destination stops within four units.
				if chase || tacticDistance > 4 {
					if chase {
						move = m.originalChaseHeading(p)
					} else {
						move = originalTargetHeading(p, target)
					}
				} else if !tacticHold {
					p.Facing = OriginalDirection(OriginalDirectionIndex(Vec2{math.Floor(m.Ball.Pos.X) - math.Floor(p.Pos.X), math.Floor(m.Ball.Pos.Y) - math.Floor(p.Pos.Y)}))
					p.originalDirectionHistory = -1
				}
				if move.Len() > 0.1 {
					p.Facing = move
				}
			}
			step := originalPlayerStep{
				move: move, originalStep: originalStep, tacticDistance: tacticDistance,
				controlled: isControlled, previouslyVisible: previouslyVisible,
				entryState: entryState, ballWasVisible: ballWasVisible,
			}
			if m.originalCornerSelecting {
				m.originalCornerContinuation = originalCornerContinuation{
					active: true, team: t, actor: i, closest: closestTeam, second: second, step: step,
				}
				return false
			}
			m.finishOriginalPlayer(t, i, step)
		}
		// 14C42 selects at each team's end, before the other team's
		// holder can replace the ball position used by directional groups.
		if m.Config.Human[t] {
			m.recordOriginalControllerVideo(t)
			m.selectPlayersForTeam(m.controls, t)
		}
	}
	return true
}

// finishOriginalPlayer is the common movement/draw tail after actor dispatch.
// A corner selector suspends before this tail and runs it once on confirmation.
func (m *Match) finishOriginalPlayer(team, actor int, step originalPlayerStep) {
	p := &m.Teams[team].Players[actor]
	// At 0xab10 the pace word is multiplied by 1000 and added to a
	// 16.16 position. The selected human receives +20 at 0x107b0.
	paceWord := originalPace(p)
	if step.controlled && m.Config.Human[team] {
		paceWord += 20
	} else if !step.controlled {
		// Normal tactic movement halves its pace within 150 units,
		// then halves it again within 100 units (0xc784..0xc798).
		if step.tacticDistance <= 150 {
			paceWord >>= 1
		}
		if step.tacticDistance <= 100 {
			paceWord >>= 1
		}
	}
	pace := float64(paceWord) * 1000 * TPS / originalFixedScale
	if step.originalStep >= 0 {
		paceWord = step.originalStep
		pace = float64(paceWord) * 1000 * TPS / originalFixedScale
	}
	if p.shotPauseTicks > 0 {
		p.Vel = Vec2{}
		paceWord = 0
	} else if p.tackleTicks > 0 {
		p.Vel = p.Vel.Mul(0.98)
	} else {
		p.Vel = step.move.Mul(pace)
	}
	p.Pos = p.Pos.Add(p.Vel.Mul(1.0 / TPS))
	// AAB6 adds the complete step without clamping actor coordinates.
	// Throwers can remain beyond the line while returning to play.
	p.originalStep = paceWord
	if step.move.Len() < .05 && p.originalState != 10 && p.originalState != 29 {
		p.originalStep = 0
	}
	advanceOriginalAnimation(p)
	p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
	p.originalDistanceReady = true
	if !step.ballWasVisible && m.originalBallDrawEnabled {
		m.recordOriginalBallVideo()
	}
	m.recordOriginalPlayerVideo(team, actor, step.previouslyVisible, step.entryState, 0)
	m.updateOriginalScannerActor(team, actor)
	// Both native wrappers, 0xd84a and 0x138e8, decrement after
	// the actor action so a fresh fifteen-tick touch exports fourteen.
	if p.touchCooldown > 0 {
		p.touchCooldown--
	}
}

func (m *Match) updatePenaltyKeeper(team int, p *Player, in Input) {
	if in.Fire {
		p.Pos.X = clamp(p.Pos.X+in.Move.X*5, PitchWidth/2-GoalWidth/2, PitchWidth/2+GoalWidth/2)
		p.JumpHeight = math.Max(0, -in.Move.Y*20)
		p.ActionTicks = 12
	}
	if m.Ball.TrappedBy == team*11 {
		m.updateKeeper(team, p, 1.0/TPS)
		m.penaltyFlightTicks = 0
	}
}

func (m *Match) passTarget(team, index int) int {
	p := &m.Teams[team].Players[index]
	best := index
	score := -math.MaxFloat64
	for i := 1; i < 11; i++ {
		q := &m.Teams[team].Players[i]
		if i == index || q.SentOff {
			continue
		}
		distance := q.Pos.Sub(p.Pos).Len()
		if distance < 35 || distance > 350 {
			continue
		}
		progress := (q.Pos.Y - p.Pos.Y) * float64(m.AttackDirection(team))
		value := progress - distance*0.25
		for j := 1; j < 11; j++ {
			if q.Pos.Sub(m.Teams[1-team].Players[j].Pos).Len() < 55 {
				value -= 75
			}
		}
		if value > score {
			best = i
			score = value
		}
	}
	return best
}

func (m *Match) updateKeeper(team int, p *Player, dt float64) {
	m.updateOriginalKeeper(team, p, dt)
}

// updateLegacyKeeper retains the previous model for reference while the source
// dive and release states are translated; ordinary play uses updateOriginalKeeper.
func (m *Match) updateLegacyKeeper(team int, p *Player, dt float64) {
	direction := m.AttackDirection(team)
	goalY := 35.0
	if direction < 0 {
		goalY = PitchHeight - 35
	}
	if m.Ball.TrappedBy == team*11 {
		p.Vel = Vec2{}
		m.keeperHold--
		if m.keeperHold <= 0 || m.controls[team].Fire && !m.previous[team].Fire {
			aim := m.controls[team].Move
			if aim.Len() < 0.1 {
				aim = Vec2{(m.random() - 0.5) * 0.5, float64(direction)}
			}
			if aim.Y*float64(direction) < 0.1 {
				aim.Y = float64(direction)
			}
			m.kick(team, 0, aim, 380, 115, "keeper_kick")
		}
		return
	}
	target := Vec2{clamp(m.Ball.Pos.X, PitchWidth/2-GoalWidth*0.66, PitchWidth/2+GoalWidth*0.66), goalY}
	if m.Ball.Pos.Y < PenaltyAreaDepth && direction == 1 || m.Ball.Pos.Y > PitchHeight-PenaltyAreaDepth && direction == -1 {
		if math.Abs(m.Ball.Pos.X-PitchWidth/2) < PenaltyAreaWidth/2 {
			target = m.Ball.Pos.Add(m.Ball.Vel.Mul(0.08))
			target.Y = clamp(target.Y, 15, PitchHeight-15)
		}
	}
	// Anticipating the intersection makes the keeper save fast shots rather than
	// merely follow the current horizontal ball position.
	if math.Abs(m.Ball.Vel.Y) > 40 && m.Ball.Vel.Y*float64(direction) < 0 {
		flight := (goalY - m.Ball.Pos.Y) / m.Ball.Vel.Y
		if flight > 0 && flight < 0.8 {
			target.X = clamp(m.Ball.Pos.X+m.Ball.Vel.X*flight, PitchWidth/2-GoalWidth*0.8, PitchWidth/2+GoalWidth*0.8)
		}
	}
	delta := target.Sub(p.Pos)
	if delta.Len() > 3 {
		p.Vel = delta.Unit().Mul(190 + 55*p.Attributes.Pace)
		p.Facing = delta.Unit()
	} else {
		p.Vel = Vec2{}
	}
	p.Pos = p.Pos.Add(p.Vel.Mul(dt))
}

func (m *Match) resolvePlayerCollisions() {
	for t := range m.Teams {
		for i := range m.Teams[t].Players {
			p := &m.Teams[t].Players[i]
			if p.SentOff || m.Config.Practice && t == 1 && i != 0 {
				continue
			}
			// Source contact routines compare opposing players. Teammates can
			// overlap; repelling them changes the observed dribble trajectory.
			for u := t + 1; u < 2; u++ {
				for j := range m.Teams[u].Players {
					if u == t && j <= i {
						continue
					}
					q := &m.Teams[u].Players[j]
					if q.SentOff || m.Config.Practice && u == 1 && j != 0 {
						continue
					}
					delta := p.Pos.Sub(q.Pos)
					distance := delta.Len()
					if distance >= PlayerRadius*1.8 {
						continue
					}
					if u != t && m.Phase == Playing {
						if p.tackleTicks > 0 {
							m.checkFoul(t, i, u, j)
						} else if q.tackleTicks > 0 {
							m.checkFoul(u, j, t, i)
						}
					}
					// Original 0x11a90 dispatches changes to heading, possession
					// and player actions; it does not separate overlapping bodies.
				}
			}
		}
	}
}

func (m *Match) checkFoul(team, index, otherTeam, otherIndex int) {
	p := &m.Teams[team].Players[index]
	q := &m.Teams[otherTeam].Players[otherIndex]
	if q.Keeper {
		return
	}
	ballFirst := m.Ball.LastTeam == team && m.Ball.LastPlayer == index && m.Tick-p.lastTouch < 6
	behind := p.Pos.Sub(q.Pos).Dot(q.Facing) < -4
	if ballFirst && !behind {
		return
	}
	p.tackleTicks = 0
	p.ActionTicks = 35
	// Contact without the ball and contact from behind are always penalized.
	// Original temperament bytes supply distinct native discipline profiles. The
	// exact original adjudication routine remains outside this reconstruction.
	strictness := clamp(1-float64(m.Referee.Attributes[1])/200, 0.05, 1)
	redChance := 0.06 + 0.18*clamp(1-float64(m.Referee.Attributes[2])/20, 0, 1)
	severe := behind && p.Attributes.Aggression > 0.6 && m.random() < redChance
	previousCards := p.YellowCards
	previousSentOff := p.SentOff
	if severe {
		p.SentOff = true
		m.notice("RED CARD", TPS*3)
		m.emit("red_card", team, index, p.Pos)
	} else if behind && m.random() < 0.7+0.3*strictness || m.random() > p.Attributes.Tackling+0.3*(1-strictness) {
		p.YellowCards++
		if p.YellowCards >= 2 {
			p.SentOff = true
			m.notice("SECOND YELLOW - RED CARD", TPS*3)
			m.emit("red_card", team, index, p.Pos)
		} else {
			m.notice("YELLOW CARD", TPS*3)
			m.emit("yellow_card", team, index, p.Pos)
		}
	}
	cardMessage := ""
	if p.YellowCards != previousCards || p.SentOff != previousSentOff {
		cardMessage = m.Message
	}
	restart := FreeKick
	defendsTop := m.AttackDirection(team) == 1
	inArea := math.Abs(q.Pos.X-PitchWidth/2) < PenaltyAreaWidth/2 && (defendsTop && q.Pos.Y < PenaltyAreaDepth || !defendsTop && q.Pos.Y > PitchHeight-PenaltyAreaDepth)
	pos := q.Pos
	if inArea {
		restart = Penalty
		pos = Vec2{PitchWidth / 2, PenaltySpotDepth}
		if !defendsTop {
			pos.Y = PitchHeight - PenaltySpotDepth
		}
	}
	m.beginRestart(restart, otherTeam, pos, TPS*2)
	if cardMessage != "" {
		m.notice(restart.String()+" - "+cardMessage, TPS*3)
	}
	m.emit("foul", team, index, q.Pos)
}
