package game

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/olivierh59500/kickoff2/internal/engine"
	"github.com/olivierh59500/kickoff2/internal/replaydata"
)

// originalHeaderText projects native labels into the source's Latin-1 fields.
// The original Store editor exposes the same limits before an original save.
func originalHeaderText(text string, limit int) string {
	var value []rune
	for _, c := range text {
		if len(value) >= limit {
			break
		}
		if c >= 32 && c <= 255 {
			value = append(value, c)
		}
	}
	return string(value)
}

func (g *Game) recordingSetup() replaydata.Metadata {
	metadata := replaydata.Metadata{Pitch: g.Match.Config.Pitch.OriginalIndex()}
	for team := range metadata.Kits {
		index := team
		for country, name := range engine.InternationalCountries {
			if strings.EqualFold(g.Match.Teams[team].Name, name) {
				index = country + 2
				break
			}
		}
		design := g.kits[index]
		if g.Match.Config.HasTeamKits[team] && g.Match.Config.TeamKits[team].Valid() {
			kit := g.Match.Config.TeamKits[team]
			design = kitDesign{kit.Style, kit.Shirt, kit.Shorts}
		}
		metadata.Kits[team] = replaydata.Kit{Style: uint16(design.Style), Shirt: kitColors[design.Shirt], Shorts: kitColors[design.Shorts]}
	}
	metadata.Fields = [6]string{"REPLAY", time.Now().Format("02/01/06"), originalHeaderText(g.Match.Teams[0].Name, 9), originalHeaderText(g.Match.Teams[1].Name, 9), "PLAYER 1", "COMPUTER"}
	if g.Match.Config.Human[1] {
		metadata.Fields[5] = "PLAYER 2"
	}
	return metadata
}

func (g *Game) originalRecordedFile() (replayFile, error) {
	if g.recordedMovie == nil {
		return replayFile{}, fmt.Errorf("no original recording is available")
	}
	data, err := g.recordedMovie.MarshalBinary()
	if err != nil {
		return replayFile{}, err
	}
	return replayFile{Version: 1, Original: data}, nil
}

func (g *Game) prepareRecordedPlayback() error {
	if g.recordedMovie == nil {
		return nil
	}
	saved, err := g.originalRecordedFile()
	if err != nil {
		return err
	}
	return g.installReplayFile(saved)
}

// SaveOriginalRecording creates a new original .grp record from actual native
// gameplay commands. It never overwrites the destination or alters live state.
func (g *Game) SaveOriginalRecording(filename string) error {
	var clip *replaydata.Clip
	if g.originalReplay != nil {
		clip = g.originalReplay
	} else {
		clip = g.recordedMovie
	}
	if clip == nil {
		return fmt.Errorf("no original recording is available")
	}
	data, err := clip.MarshalBinary()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil {
		os.Remove(filename)
		if writeErr != nil {
			return writeErr
		}
		return closeErr
	}
	return nil
}
