package game

import (
	"encoding/json"
	"github.com/olivierh59500/kickoff2/internal/engine"
	"github.com/olivierh59500/kickoff2/internal/replaydata"
	"os"
	"path/filepath"
)

// snapshot records visual state rather than rerunning input against live state.
// A replay cannot alter the match RNG, clock, or score.
type snapshot struct {
	Video                          *liveVideoFrame
	Official                       engine.OriginalOfficialState
	HasOfficial                    bool
	BallVisible, HasBallVisibility bool
	Teams                          [2]engine.Team
	Ball                           engine.Ball
	Camera                         engine.Vec2
	ClockTicks                     int
	RenderTicks                    int
}

func (g *Game) recordFrame() {
	// The source blocks inside this update until the selector is confirmed.
	// Its cursor changes belong to one pending movie frame, not new markers.
	if g.Match.OriginalCornerSelectorActive() {
		return
	}
	g.clearOriginalReplay()
	frame := snapshot{Teams: g.Match.Teams, Ball: g.Match.Ball, Camera: g.camera, ClockTicks: g.Match.ClockTicks, RenderTicks: g.ticks, BallVisible: g.Match.OriginalBallVisible(), HasBallVisibility: true}
	frame.Official, frame.HasOfficial = g.Match.OriginalOfficialVisual()
	frame.Video = captureLiveVideo(g.Match)
	if g.liveRecorder == nil {
		g.liveRecorder = replaydata.NewRecorder()
	}
	recorded, ready := g.Match.OriginalVideoFrame()
	if !ready {
		recorded = frame.Video.Frame
	}
	if err := g.liveRecorder.AppendFrame(recorded); err != nil {
		g.notice = err.Error()
	} else {
		x, y := g.Match.OriginalVideoNextTarget()
		movie, err := g.liveRecorder.Snapshot(g.recordingSetup(), x, y)
		if err != nil {
			g.notice = err.Error()
		} else {
			g.recordedMovie = movie
		}
	}
	metadata := *g.Match
	g.replaySource = &metadata
	g.replaySourceKits = g.kits
	if len(g.replay) < 50*15 {
		g.replay = append(g.replay, frame)
	} else {
		g.replay[g.replayStart] = frame
		g.replayStart = (g.replayStart + 1) % len(g.replay)
	}
}
func (g *Game) replayAt(index int) snapshot { return g.replay[(g.replayStart+index)%len(g.replay)] }

type replayFile struct {
	Version  int
	Metadata *engine.Match
	Frames   []visualFrame
	Kits     *[10]kitDesign `json:"Kits,omitempty"`
	Original []byte         `json:"Original,omitempty"`
	Recorded []byte         `json:"Recorded,omitempty"`
}

func validReplayFrames(frames []visualFrame) bool {
	if len(frames) < 1 || len(frames) > 750 {
		return false
	}
	for _, frame := range frames {
		if frame.RenderTicks < 0 || frame.ClockTicks < 0 {
			return false
		}
		if frame.Video != nil && (len(frame.Video.Frame.Commands) > 64 || len([]rune(frame.Video.HUD.Message)) > 80) {
			return false
		}
	}
	return true
}

func (g *Game) saveReplay() {
	saved, err := g.currentReplayFile()
	if err != nil {
		g.notice = err.Error()
		return
	}
	data, err := json.Marshal(saved)
	if err == nil {
		err = os.MkdirAll(g.dataDir, 0755)
	}
	path := filepath.Join(g.dataDir, "replay.json")
	if err == nil {
		err = os.WriteFile(path+".tmp", data, 0600)
	}
	if err == nil {
		err = os.Rename(path+".tmp", path)
	}
	if err != nil {
		g.notice = err.Error()
	} else {
		g.notice = "REPLAY SAVED"
	}
}
func (g *Game) loadReplay() {
	path := filepath.Join(g.dataDir, "replay.json")
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Size() > 8*1024*1024 {
		g.notice = "INVALID REPLAY FILE"
		return
	}
	var saved replayFile
	if err = json.NewDecoder(file).Decode(&saved); err != nil {
		g.notice = "INVALID REPLAY FILE"
		return
	}
	if err := g.installReplayFile(saved); err != nil {
		g.notice = "INVALID REPLAY FILE"
	}

}

// visualFrame deliberately stores only the data used by Draw. Original roster
// bytes, attributes, and bench records belong to the engine, not every video frame.
type visualFrame struct {
	Video                          *liveVideoFrame               `json:"Video,omitempty"`
	Official                       *engine.OriginalOfficialState `json:"Official,omitempty"`
	BallVisible, HasBallVisibility bool                          `json:",omitempty"`
	Teams                          [2]visualTeam
	Ball                           engine.Ball
	Camera                         engine.Vec2
	ClockTicks                     int
	RenderTicks                    int `json:"RenderTicks,omitempty"`
}
type visualTeam struct {
	Name               string
	Score, Controlled  int
	ControlledOpposing bool `json:",omitempty"`
	Players            [11]visualPlayer
}
type visualPlayer struct {
	OriginalState, OriginalSubstate int `json:",omitempty"`
	JumpTicks                       int
	AnimationPhase                  uint16
	Name                            string
	Number                          int
	Keeper, SentOff                 bool
	Pos, Vel, Facing                engine.Vec2
	ActionTicks                     int
	JumpHeight                      float64
}

func portableFrame(frame snapshot) visualFrame {
	result := visualFrame{Ball: frame.Ball, Camera: frame.Camera, ClockTicks: frame.ClockTicks, RenderTicks: frame.RenderTicks, BallVisible: frame.BallVisible, HasBallVisibility: frame.HasBallVisibility}
	result.Video = frame.Video
	if frame.HasOfficial {
		official := frame.Official
		result.Official = &official
	}
	for t, team := range frame.Teams {
		result.Teams[t] = visualTeam{Name: team.Name, Score: team.Score, Controlled: team.Controlled, ControlledOpposing: team.ControlledOpposing}
		for i, p := range team.Players {
			result.Teams[t].Players[i] = visualPlayer{JumpTicks: p.VisualJumpTicks(), AnimationPhase: p.AnimationPhase, Name: p.Name, Number: p.Number, Keeper: p.Keeper, SentOff: p.SentOff, Pos: p.Pos, Vel: p.Vel, Facing: p.Facing, ActionTicks: p.ActionTicks, JumpHeight: p.JumpHeight}
			result.Teams[t].Players[i].OriginalState, result.Teams[t].Players[i].OriginalSubstate = p.VisualOriginalAction()
		}
	}
	return result
}
func (frame visualFrame) snapshot() snapshot {
	result := snapshot{Ball: frame.Ball, Camera: frame.Camera, ClockTicks: frame.ClockTicks, RenderTicks: frame.RenderTicks, BallVisible: frame.BallVisible, HasBallVisibility: frame.HasBallVisibility}
	result.Video = frame.Video
	if frame.Official != nil {
		result.Official, result.HasOfficial = *frame.Official, true
	}
	for t, team := range frame.Teams {
		result.Teams[t] = engine.Team{Name: team.Name, Score: team.Score, Controlled: team.Controlled, ControlledOpposing: team.ControlledOpposing}
		for i, p := range team.Players {
			result.Teams[t].Players[i] = engine.Player{Name: p.Name, Number: p.Number, Keeper: p.Keeper, SentOff: p.SentOff, Pos: p.Pos, Vel: p.Vel, Facing: p.Facing, ActionTicks: p.ActionTicks, JumpHeight: p.JumpHeight, AnimationPhase: p.AnimationPhase}
			result.Teams[t].Players[i].RestoreVisualJumpTicks(p.JumpTicks)
			result.Teams[t].Players[i].RestoreVisualOriginalAction(p.OriginalState, p.OriginalSubstate)
		}
	}
	return result
}
