package game

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/kickoff2/internal/engine"
	"github.com/olivierh59500/kickoff2/internal/replaydata"
	"github.com/olivierh59500/kickoff2/internal/replayrender"
)

// liveVideoFrame preserves source screen commands separately from match state.
// Native replay playback must not recompute visibility or controller selection.
type liveVideoFrame struct {
	Frame   replaydata.Frame
	Camera  replayrender.Camera
	Caption *replaydata.Command `json:",omitempty"`
	HUD     replayrender.HUD
}

func captureLiveVideo(match *engine.Match) *liveVideoFrame {
	frame, ready := match.OriginalLiveVideoFrame()
	cameraX, cameraY := match.OriginalVideoCamera()
	if !ready || match.Phase != engine.Playing {
		frame = match.OriginalVideoPreview()
		state := match.OriginalSnapshot()
		cameraX, cameraY = state.ActorCameraX, state.ActorCameraY
	}
	state := match.OriginalSnapshot()
	if marker, cell, visible := match.OriginalPenaltyMarkerVisual(); visible {
		code := cell + match.Restart.Team*100
		if cell >= 100 {
			code = 120 + match.Restart.Team*100 + cell
		}
		frame.Commands = append(frame.Commands, replaydata.Command{X: int16(math.Floor(marker.X)) - int16(cameraX>>16) - 8,
			Y: int16(math.Floor(marker.Y)) - int16(cameraY>>16) - 8, Code: int16(code)})
	}
	result := &liveVideoFrame{Frame: frame, Camera: replayrender.Camera{X: cameraX, Y: cameraY},
		HUD: replayrender.HUD{Scores: state.Scores, RemainingSeconds: match.RemainingSeconds(), Message: match.PublishedMessage(),
			Practice: match.Config.Practice, Scanner: state.Scanner}}
	if panel, visible := match.OriginalCornerPanelVisual(); visible {
		result.HUD.CornerPanel = &panel
	}
	if caption, present := match.OriginalVideoCaption(); present {
		result.Caption = &caption
	}
	return result
}

func (g *Game) indexedMatchSetup(match *engine.Match) replaydata.Metadata {
	metadata := replaydata.Metadata{Pitch: match.Config.Pitch.OriginalIndex()}
	for team := range metadata.Kits {
		design := g.teamKitDesign(team)
		metadata.Kits[team] = replaydata.Kit{Style: uint16(design.Style), Shirt: kitColors[design.Shirt], Shorts: kitColors[design.Shorts]}
	}
	return metadata
}

func (g *Game) drawIndexedMatch(dst *ebiten.Image, match *engine.Match) bool {
	var video *liveVideoFrame
	if g.screen == replayScreen && len(g.replay) != 0 {
		video = g.replayAt(min(g.replayIndex, len(g.replay)-1)).Video
		if video == nil {
			return false // Older portable files retain their recorded interpretation.
		}
	} else {
		video = captureLiveVideo(match)
	}
	metadata := g.indexedMatchSetup(match)
	if g.indexedMatchRenderer == nil || metadata != g.indexedMatchMetadata {
		renderer, err := replayrender.New(metadata)
		if err != nil {
			g.notice = err.Error()
			return true
		}
		g.indexedMatchRenderer, g.indexedMatchMetadata = renderer, metadata
	}
	hud := video.HUD
	if !g.scanner {
		hud.Scanner = engine.OriginalScannerState{}
	}
	frame, err := g.indexedMatchRenderer.RenderLive(video.Frame, video.Camera, video.Caption, hud)
	if err != nil {
		g.notice = err.Error()
		return true
	}
	if g.images == nil {
		g.images = map[string]*ebiten.Image{}
	}
	image := g.images["indexed-match"]
	if image == nil {
		image = ebiten.NewImage(320, 256)
		g.images["indexed-match"] = image
	}
	image.WritePixels(frame.Pix)
	dst.DrawImage(image, nil)
	if g.screen == matchScreen {
		g.drawOriginalPlayerView(dst)
	}
	return true
}
