package game

import (
	"os"
	"path/filepath"

	"github.com/olivierh59500/kickoff2/internal/engine"
)

func (g *Game) beginCompetition(cup bool) {
	g.finishCompetitionName()
	g.finishCompetitionEditor(true)
	g.restoreCompetitionConfig()
	if !cup {
		g.competitionSquadImportLocked = false // Original 5D92; cup 6602 retains it.
	}
	previous := g.tournament
	g.tournament = engine.NewTournament(cup, engine.InternationalCountries, g.config.Seed)
	// A new draw continues the current shared generator, when one exists.
	if previous != nil {
		g.tournament = engine.NewTournamentWithOriginalState(cup, engine.InternationalCountries, previous.OriginalRandomState)
	} else if g.prematch != nil {
		g.tournament = engine.NewTournamentWithOriginalState(cup, engine.InternationalCountries, g.prematch.RandomState())
	} else if g.Match != nil {
		g.tournament = engine.NewTournamentWithOriginalState(cup, engine.InternationalCountries, g.Match.OriginalRandomState())
	}
	g.tournament.Configure(g.config)
	for country := range g.tournament.Kits {
		design := g.kits[country+2]
		g.tournament.Kits[country] = engine.TournamentKit{Style: design.Style, Shirt: design.Shirt, Shorts: design.Shorts}
	}
	g.tournament.HumanTeams = g.competitionHumans
	g.prematch = nil
	g.openCompetitionMenu()
}

func (g *Game) openCompetitionMenu() {
	g.periodMenuActive = false
	g.competitionResult = nil
	g.openFeatureMenu(competitionScreen, 1)
}

func (g *Game) saveTournament() {
	g.captureCompetitionSetup()
	g.saveSettings()
	if g.tournament != nil {
		if err := g.tournament.Save(filepath.Join(g.dataDir, "tournament.json")); err != nil {
			g.notice = err.Error()
		} else {
			g.notice = "TOURNAMENT SAVED"
			if data, err := g.tournament.OriginalFileBytes(); err == nil {
				directory := filepath.Join(g.dataDir, "exports")
				if err = os.MkdirAll(directory, 0700); err == nil {
					extension := ".lge"
					if g.tournament.Cup {
						extension = ".cup"
					}
					path := filepath.Join(directory, "tournament"+extension)
					if err = os.WriteFile(path+".tmp", data, 0600); err == nil {
						err = os.Rename(path+".tmp", path)
					}
				}
				if err != nil {
					g.notice = err.Error()
				}
			} else {
				g.notice = "PORTABLE SAVE COMPLETE; " + err.Error()
			}
		}
	}
}

func (g *Game) loadTournament() {
	if g.tournament != nil && !g.tournament.Cup {
		g.competitionSquadImportLocked = true
	}
	t, err := engine.LoadTournamentFile(filepath.Join(g.dataDir, "tournament.json"))
	if err != nil {
		g.notice = err.Error()
		return
	}
	g.installTournament(t)
}

func (g *Game) installTournament(t *engine.Tournament) {
	g.restoreCompetitionConfig()
	g.tournament, g.competitionHumans = t, t.HumanTeams
	if !t.Cup {
		g.competitionSquadImportLocked = true // Original 63D6 also locks an unplayed loaded league.
	}
	// The original 6262/63E0 save region includes all these global options.
	g.config.Pitch, g.config.Wind, g.config.Referee = t.Pitch, t.Wind, t.Referee
	g.config.LeagueSkill = t.LeagueSkill
	g.config.HalfSeconds, g.config.Speed, g.config.Aftertouch = t.HalfSeconds, t.Speed, t.Aftertouch
	g.config.Skill, g.config.ExtraTime = t.MenuSkills, t.MenuExtraTime
	g.openCompetitionMenu()
	g.notice = "TOURNAMENT LOADED"
}

// Continue advances exactly one source fixture. CPU results use the existing
// original off-pitch path; selected human countries visit their squad menus.
func (g *Game) continueTournament() {
	if g.tournament == nil {
		return
	}
	if !g.tournament.Cup {
		g.competitionSquadImportLocked = true // Original 6170 precedes squad setup.
	}
	fixture, ok := g.tournament.Next()
	if !ok {
		g.notice = "TOURNAMENT COMPLETE"
		return
	}
	if !g.tournament.HumanTeams[fixture.Home] && !g.tournament.HumanTeams[fixture.Away] {
		next := *g.tournament
		next.Fixtures = append([]engine.Fixture(nil), g.tournament.Fixtures...)
		index := 0
		for next.Fixtures[index].Played {
			index++
		}
		if err := next.SimulateNext(); err != nil {
			g.notice = err.Error()
			return
		}
		result := next.Fixtures[index]
		g.competitionResult = &competitionResult{next: &next,
			names:  [2]string{next.Names[fixture.Home], next.Names[fixture.Away]},
			kits:   [2]engine.TournamentKit{next.Kits[fixture.Home], next.Kits[fixture.Away]},
			scores: [2]int{result.HomeGoals, result.AwayGoals}}
		g.openFeatureMenu(competitionScreen, 7)
		return
	}
	cfg, err := g.tournament.FixtureConfig(g.config)
	if err != nil {
		g.notice = err.Error()
		return
	}
	saved := g.config
	g.competitionSavedConfig = &saved
	g.config = cfg
	g.practiceMode = -1
	g.competitionCupRepeated, g.competitionSquadController = false, 0
	g.setupPlayers = 1
	if cfg.Human[0] && cfg.Human[1] {
		g.setupPlayers = 2
	}
	team := 0
	if !cfg.Human[0] {
		team = 1
	}
	g.openSquad(team, preMatchScreen)
}

func (g *Game) captureCompetitionSetup() {
	if g.competitionSavedConfig != nil && g.tournament != nil {
		g.tournament.StoreFixtureSetup(g.config)
		if g.prematch != nil {
			g.tournament.SetOriginalState(g.prematch.RandomState(), g.tournament.OriginalPaceScale)
		}
	}
}

func (g *Game) restoreCompetitionConfig() {
	if g.competitionSavedConfig != nil {
		g.config = *g.competitionSavedConfig
		g.competitionSavedConfig = nil
	}
	g.competitionSquadController, g.competitionCupRepeated = 0, false
}

func (g *Game) finishTournamentMatch() {
	if g.screen != matchScreen || g.tournament == nil || g.Match == nil || !g.Match.Config.Competition || g.Match.Phase != engine.FullTime {
		return
	}
	g.tournament.StoreFixtureSetup(g.Match.Config)
	if err := g.tournament.CompleteMatch(g.Match); err != nil {
		g.notice = err.Error()
		return
	}
	g.saveTournament()
	g.Match = nil
	g.paused = false
	g.openCompetitionMenu()
}
