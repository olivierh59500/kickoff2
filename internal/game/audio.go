package game

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/olivierh59500/kickoff2/assets"
	"github.com/olivierh59500/kickoff2/internal/engine"
)

const audioRate = 44100

type soundCue string

const (
	shortWhistleCue soundCue = "short whistle"
	longWhistleCue  soundCue = "long whistle"
	crowdCue        soundCue = "crowd"
	penaltyCrowdCue soundCue = "penalty contest crowd"
	goalFinishCue   soundCue = "goal celebration finish"
	yellowCardCue   soundCue = "yellow card"
	redCardCue      soundCue = "red card"
	setPieceCue     soundCue = "set piece preparation"
	ballKickCue     soundCue = "ball kick"
	ballTouchCue    soundCue = "aerial kick"
	contactCue      soundCue = "player contact"
	foulCrowdCue    soundCue = "foul crowd"
	foulWhistleCue  soundCue = "foul whistle and impact"
	postCue         soundCue = "goal frame"
	bounce1Cue      soundCue = "bounce1"
	bounce2Cue      soundCue = "bounce2"
	bounce3Cue      soundCue = "bounce3"
	bounce4Cue      soundCue = "bounce4"
)

// originalSoundCommands preserves the shipped signed-word sequences and source
// locations. The startup sound before the game menu remains outside the port.
var originalSoundCommands = map[soundCue][]int16{
	shortWhistleCue: {16, 16, 0},                      // 0x 19b 7c
	longWhistleCue:  {17, 17, 0},                      // 0x 19b 82
	crowdCue:        {15, -5, 13, 14, 0},              // 0x 19b 88
	penaltyCrowdCue: {658, 115, -4, 113, 114, 0},      // 0x19BC2, caller0x12FF4
	setPieceCue:     {12, 0},                          // 0x19B9E, callers1312A/12EDA
	goalFinishCue:   {150, -2, 11, 0},                 // 0x19BAC, caller122FE
	yellowCardCue:   {17, 17, 1116, 113, 114, 0},      // 0x19BDA, caller12D24
	redCardCue:      {17, 17, 1345, 113, 114, 0},      // 0x19BCE, caller12C6E
	ballKickCue:     {8, 0},                           // 0x187DC, 0x194BE and 0x19DC8
	ballTouchCue:    {10, 0},                          // 0x 187d 8 and 0x 19dcc
	contactCue:      {194, 7, 0},                      // 0x 19b 4e
	foulCrowdCue:    {17, 17, 1116, 115, 113, 114, 0}, // 0x19BB4, caller 0x121D8
	foulWhistleCue:  {17, 17, 7, 0},                   // 0x19BE6, caller 0x121E0
	postCue:         {9, 0},                           // 0x 1844a
	bounce1Cue:      {1, 0},
	bounce2Cue:      {2, 0},
	bounce3Cue:      {3, 0},
	bounce4Cue:      {4, 0},
}

type soundCacheKey struct {
	cue   soundCue
	right bool
}

// soundBank provides the original four-channel first-free allocation policy.
// Each voice plays its whole decoded command sequence without host-side gaps
// between samples. Stereo sides follow Paula channels 0/3 and 1/2 respectively.
type soundBank struct {
	context *audio.Context
	samples []paulaSample
	voices  [4]*audio.Player
	pcm     map[soundCacheKey][]byte
	muted   bool
}

func newSoundBank() (*soundBank, error) {
	data, err := assets.Files.ReadFile("original/raw/expsound")
	if err != nil {
		return nil, err
	}
	samples, err := decodePaulaSamples(data)
	if err != nil {
		return nil, err
	}
	// Prepare both hardware sides before match playback so a long crowd cue
	// never spends a simulation update allocating/resampling its PCM stream.
	pcm := map[soundCacheKey][]byte{}
	for cue, commands := range originalSoundCommands {
		segments, err := compilePaulaSequence(commands, samples)
		if err != nil {
			return nil, fmt.Errorf("invalid source cue %s: %w", cue, err)
		}
		pcm[soundCacheKey{cue: cue}] = renderPaulaSequence(segments, samples, 0)
		pcm[soundCacheKey{cue: cue, right: true}] = renderPaulaSequence(segments, samples, 1)
	}
	ctx := audio.CurrentContext()
	if ctx == nil {
		ctx = audio.NewContext(audioRate)
	}
	return &soundBank{context: ctx, samples: samples, pcm: pcm}, nil
}

// decodeSamples retains a centered single-sample view for resource validation;
// live playback uses complete command sequences and original DMA truncation.
func decodeSamples(data []byte) ([][]byte, error) {
	samples, err := decodePaulaSamples(data)
	if err != nil {
		return nil, err
	}
	pcm := make([][]byte, len(samples))
	for i, sample := range samples {
		segment := paulaSegment{sample: i, period: sample.period, volume: 40, endClock: uint64(len(sample.data)) * uint64(sample.period)}
		pcm[i] = renderPaulaSequence([]paulaSegment{segment}, samples, -1)
	}
	return pcm, nil
}

// firstFreeSoundChannel follows 0xd 208. A fifth simultaneous cue is dropped;
// existing whistles/crowd voices are never stolen to create an invented channel.
func firstFreeSoundChannel(active [4]bool) int {
	for channel, busy := range active {
		if !busy {
			return channel
		}
	}
	return -1
}

func (s *soundBank) playCue(cue soundCue) bool {
	if s.muted {
		return false
	}
	var active [4]bool
	for channel, player := range s.voices {
		if player == nil {
			continue
		}
		if player.IsPlaying() {
			active[channel] = true
		} else {
			_ = player.Close()
			s.voices[channel] = nil
		}
	}
	channel := firstFreeSoundChannel(active)
	if channel < 0 {
		return false
	}
	commands, ok := originalSoundCommands[cue]
	if !ok {
		return false
	}
	key := soundCacheKey{cue: cue, right: channel == 1 || channel == 2}
	pcm := s.pcm[key]
	if pcm == nil {
		segments, err := compilePaulaSequence(commands, s.samples)
		if err != nil {
			return false
		}
		pcm = renderPaulaSequence(segments, s.samples, channel)
		s.pcm[key] = pcm
	}
	player := s.context.NewPlayerFromBytes(pcm)
	player.SetVolume(1)
	player.Play()
	s.voices[channel] = player
	return true
}

func (s *soundBank) setMuted(muted bool) {
	s.muted = muted
	if muted {
		s.pause()
	}
}

func (s *soundBank) pause() {
	for channel, player := range s.voices {
		if player != nil {
			player.Pause()
			_ = player.Close()
			s.voices[channel] = nil
		}
	}
}

func (s *soundBank) close() { s.pause() }

func soundPreferencesForEvent(event engine.Event, match *engine.Match) (practice, crowdMuted bool) {
	if event.HasSoundPreferences {
		return event.SoundPractice, event.SoundCrowdMuted
	}
	return match.Config.Practice, match.Config.CrowdMuted
}

func soundCueForEvent(event engine.Event, match *engine.Match) (soundCue, bool) {
	practice, crowdMuted := soundPreferencesForEvent(event, match)
	switch event.Kind {
	case "match_start":
		// 11E24 invokes the same137DA crowd sequence at the field entrance.
		return crowdCue, !practice && !crowdMuted
	case "shot", "penalty", "pass", "corner", "free_kick", "goal_kick", "throw_in", "keeper_kick", "tackle_ball":
		return ballKickCue, true
	case "save":
		// Original 13C42 and successful keeper callers request no sound.
		return "", false
	case "keeper_parry":
		switch event.SoundSample {
		case 8:
			return ballKickCue, true
		case 10:
			return ballTouchCue, true
		}
		return "", false
	case "set_piece_ready":
		return setPieceCue, !practice && !crowdMuted
	case "header", "chip", "scissor":
		return ballTouchCue, true
	case "whistle":
		if practice {
			return "", false
		}
		if match.Phase == engine.HalfTime || match.Phase == engine.FullTime {
			return longWhistleCue, true
		}
		return shortWhistleCue, true
	case "goal":
		if practice || !match.OriginalPenaltyContestActive() && event.Player < 0 {
			return "", false
		}
		if match.OriginalPenaltyContestActive() {
			return penaltyCrowdCue, !crowdMuted
		}
		if crowdMuted {
			return longWhistleCue, true
		}
		return penaltyCrowdCue, true
	case "period_end":
		return longWhistleCue, !practice
	case "post":
		return postCue, true
	case "tackle_hit", "player_contact":
		return contactCue, true
	case "foul":
		return foulWhistleCue, !practice
	case "yellow_card", "red_card":
		if practice {
			return "", false
		}
		if crowdMuted {
			return longWhistleCue, true
		}
		if event.Kind == "yellow_card" {
			return yellowCardCue, true
		}
		return redCardCue, true
	case "bounce":
		sample := event.SoundSample
		if sample == 0 {
			sample = 1
		}
		if sample >= 1 && sample <= 4 {
			return [...]soundCue{bounce1Cue, bounce2Cue, bounce3Cue, bounce4Cue}[sample-1], true
		}
	}
	return "", false
}

// soundCuesForEvent retains distinct ordered requests and independent source
// Practice/crowd gates. Contact sounds belong to the earlier actor event.
func soundCuesForEvent(event engine.Event, match *engine.Match) [2]soundCue {
	practice, crowdMuted := soundPreferencesForEvent(event, match)
	if event.Kind == "foul" && !practice && !crowdMuted {
		return [2]soundCue{foulCrowdCue, foulWhistleCue}
	}
	if cue, ok := soundCueForEvent(event, match); ok {
		if event.Kind == "goal" && !match.OriginalPenaltyContestActive() {
			return [2]soundCue{cue, goalFinishCue}
		}
		return [2]soundCue{cue}
	}
	return [2]soundCue{}
}

func (g *Game) matchAudio(previousPhase engine.Phase) {
	whistle := false
	for _, event := range g.Match.Events {
		if event.Kind == "whistle" {
			whistle = true
		}
		for _, cue := range soundCuesForEvent(event, g.Match) {
			if cue != "" {
				g.audio.playCue(cue)
			}
		}
	}
	if !g.Match.Config.Practice && !whistle && g.Match.Tick == 1 && previousPhase == engine.Kickoff {
		g.audio.playCue(shortWhistleCue)
	}
}
