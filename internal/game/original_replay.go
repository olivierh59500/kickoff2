package game

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/kickoff2/internal/importdata"
	"github.com/olivierh59500/kickoff2/internal/replaydata"
	"github.com/olivierh59500/kickoff2/internal/replayrender"
)

func (g *Game) replayLength() int {
	if g.originalReplay != nil {
		return len(g.originalReplayFrames)
	}
	return len(g.replay)
}

func (g *Game) clearOriginalReplay() {
	g.originalReplay, g.originalReplayRenderer = nil, nil
	g.originalReplayFrames, g.originalReplayCameras, g.originalReplayCaptions = nil, nil, nil
	if g.originalReplayImage != nil {
		g.originalReplayImage.Dispose()
		g.originalReplayImage = nil
	}
}

func (g *Game) currentReplayFile() (replayFile, error) {
	if g.originalReplay != nil {
		data, err := g.originalReplay.MarshalBinary()
		return replayFile{Version: 1, Original: data}, err
	}
	if len(g.replay) == 0 || g.replaySource == nil {
		return replayFile{}, fmt.Errorf("no replay has been recorded")
	}
	frames := make([]visualFrame, len(g.replay))
	for index := range frames {
		frames[index] = portableFrame(g.replayAt(index))
	}
	saved := replayFile{Version: 1, Metadata: g.replaySource, Frames: frames, Kits: &g.replaySourceKits}
	if g.recordedMovie != nil {
		var err error
		saved.Recorded, err = g.recordedMovie.MarshalBinary()
		if err != nil {
			return replayFile{}, err
		}
	}
	return saved, nil
}

// installReplayFile creates detached playback data. It never changes Match,
// saved match options, live RNG, controls, clock or the user's kit designs.
func (g *Game) installReplayFile(saved replayFile) error {
	if saved.Version != 1 {
		return fmt.Errorf("invalid replay version")
	}
	if len(saved.Original) != 0 {
		if saved.Metadata != nil || len(saved.Frames) != 0 || saved.Kits != nil || len(saved.Recorded) != 0 {
			return fmt.Errorf("ambiguous replay representation")
		}
		clip, err := replaydata.Parse(saved.Original)
		if err != nil {
			return err
		}
		frames, err := clip.Frames()
		if err != nil || len(frames) == 0 {
			return fmt.Errorf("original replay has no playable frames")
		}
		renderer, err := replayrender.New(clip.Metadata())
		if err != nil {
			return err
		}
		cameras, captions := replayrender.Timeline(frames)
		g.clearOriginalReplay()
		g.originalReplay, g.originalReplayFrames = clip, frames
		g.originalReplayCameras, g.originalReplayCaptions = cameras, captions
		g.originalReplayRenderer = renderer
		g.replay, g.replaySource = nil, nil
		g.replayStart, g.replayIndex = 0, 0
		return nil
	}
	if saved.Metadata == nil || !validReplayFrames(saved.Frames) || saved.Metadata.Config.Pitch < 0 || saved.Metadata.Config.Pitch > 3 {
		return fmt.Errorf("invalid portable replay clip")
	}
	if saved.Kits != nil && !validKitDesigns(*saved.Kits) {
		return fmt.Errorf("invalid replay kit data")
	}
	var recording *replaydata.Clip
	if len(saved.Recorded) != 0 {
		var err error
		recording, err = replaydata.Parse(saved.Recorded)
		if err != nil {
			return err
		}
		// Portable files identify their native surface explicitly. Earlier
		// recordings wrote that enum directly into the original header; repair
		// only this known native projection, preserving all other bytes.
		binary.BigEndian.PutUint16(recording.Header[60:], saved.Metadata.Config.Pitch.OriginalIndex())
	}
	g.clearOriginalReplay()
	g.replay = make([]snapshot, len(saved.Frames))
	for index, frame := range saved.Frames {
		g.replay[index] = frame.snapshot()
	}
	g.replaySource, g.replayStart, g.replayIndex = saved.Metadata, 0, 0
	g.recordedMovie = recording
	g.replaySourceKits = g.kits
	if saved.Kits != nil {
		g.replaySourceKits = *saved.Kits
	}
	g.clearColoredKitCache()
	return nil
}

func (g *Game) drawOriginalReplay(dst *ebiten.Image) {
	if len(g.originalReplayFrames) == 0 || g.originalReplayRenderer == nil {
		return
	}
	index := min(g.replayIndex, len(g.originalReplayFrames)-1)
	frame, err := g.originalReplayRenderer.Render(g.originalReplayFrames[index], g.originalReplayCameras[index], g.originalReplayCaptions[index])
	if err != nil {
		g.notice = err.Error()
		return
	}
	if g.originalReplayImage == nil {
		g.originalReplayImage = ebiten.NewImage(320, 256)
	}
	g.originalReplayImage.WritePixels(frame.Pix)
	dst.DrawImage(g.originalReplayImage, nil)
}

func (g *Game) importOriginalReplays(file importdata.File) ([]string, error) {
	clips, err := importdata.ReadReplays(file)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, clip := range clips {
		data, err := clip.MarshalBinary()
		if err != nil {
			return nil, err
		}
		if err := g.installReplayFile(replayFile{Version: 1, Original: data}); err != nil {
			return nil, err
		}
		metadata := clip.Metadata()
		stored, err := g.storeReplayClip(metadata.Fields)
		if err != nil {
			return nil, err
		}
		ids = append(ids, stored.ID)
	}
	return ids, nil
}

// OpenOriginalReplay uses the same validated import/storage/playback path as
// the desktop/touch browser, including complete .rep and custom data ADFs.
func (g *Game) OpenOriginalReplay(filename, entry string) error {
	ids, err := g.importOriginalReplays(importdata.File{Path: filename, DiskEntry: entry, Kind: "replay"})
	if err != nil {
		return err
	}
	return g.startReplayPlaylist(ids, replayHomeScreen)
}

func (g *Game) exportOriginalLibraryClip(id string) error {
	if err := g.loadLibraryClip(id); err != nil {
		return err
	}
	clip := g.originalReplay
	if clip == nil {
		clip = g.recordedMovie
	}
	if clip == nil {
		return fmt.Errorf("select an original movie to export")
	}
	data, err := clip.MarshalBinary()
	if err != nil {
		return err
	}
	name := clip.Metadata().Fields[0]
	name = strings.Map(func(value rune) rune {
		if value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z' || value >= '0' && value <= '9' || value == '_' {
			return value
		}
		return '_'
	}, name)
	if name == "" {
		name = "REPLAY"
	}
	if len(name) > 8 {
		name = name[:8]
	}
	directory := filepath.Join(g.dataDir, "exports")
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	var file *os.File
	for index := 0; index < 1000; index++ {
		base := name
		if index > 0 {
			base = name[:min(5, len(name))] + fmt.Sprintf("%03d", index)
		}
		file, err = os.OpenFile(filepath.Join(directory, base+".grp"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return err
		}
	}
	if file == nil {
		return fmt.Errorf("original replay export folder is full")
	}
	complete := false
	defer func() {
		if !complete {
			os.Remove(file.Name())
		}
	}()
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	complete = true
	if g.importPickerEnabled {
		g.requestDocumentExport(file.Name())
	}
	g.notice = "ORIGINAL REPLAY EXPORTED"
	return nil
}

func (g *Game) requestDocumentExport(filename string) {
	g.replayExportRequest = filename
	g.documentExportOrigin, g.documentExportPending = g.screen, true
}

// TakeReplayExportRequest transfers an explicit replay/competition Export action to the Android
// document host; desktop exports stay in the selected data directory.
func (g *Game) TakeReplayExportRequest() string {
	filename := g.replayExportRequest
	g.replayExportRequest = ""
	return filename
}

// ReplayExportCompleted reports only to a replay screen; a late document result
// must not replace a notice on another screen after the user navigates away.
func (g *Game) ReplayExportCompleted(message string) {
	if g.documentExportPending {
		g.documentExportPending = false
		if g.documentExportOrigin == competitionScreen {
			if g.screen == competitionScreen {
				g.notice = message
			}
			return
		}
	}
	if g.screen == replayHomeScreen || g.screen == replayBrowserScreen || g.screen == replayEditScreen || g.screen == replayScreen {
		g.notice = message
	}
}

func (g *Game) replayTouchAction(x, y int) bool {
	if x < 4 || x >= 76 || g.replayLength() == 0 {
		return false
	}
	switch {
	case y >= 64 && y < 92:
		g.replayIndex = 0
	case y >= 100 && y < 128:
		g.replayIndex = max(0, g.replayIndex-1)
	case y >= 136 && y < 164:
		g.replayIndex = min(g.replayLength()-1, g.replayIndex+1)
	default:
		return false
	}
	g.replayPaused = true
	return true
}

func (g *Game) editOriginalReplayMetadata(id string, fields [6]string) error {
	if !validReplayID(id) {
		return fmt.Errorf("invalid original replay identity")
	}
	filename := filepath.Join(g.libraryDirectory(), id+".json")
	data, err := os.ReadFile(filename)
	if err != nil || len(data) > 8*1024*1024 {
		return fmt.Errorf("missing original replay file")
	}
	var saved replayFile
	if err := json.Unmarshal(data, &saved); err != nil || len(saved.Original) == 0 {
		return fmt.Errorf("invalid original replay file")
	}
	clip, err := replaydata.Parse(saved.Original)
	if err != nil {
		return err
	}
	metadata := clip.Metadata()
	metadata.Fields = fields
	if err := clip.SetMetadata(metadata); err != nil {
		return err
	}
	saved.Original, err = clip.MarshalBinary()
	if err != nil {
		return err
	}
	return writeReplayJSON(filename, saved)
}
