package engine

// OriginalInitialPacePercent follows the90-percent division-four branches at
// 5912 and5A02. Explicit competition percentages supersede single-game setup.
// Imported/custom squads use the source Player Manager choice, not its DIV.4
// menu index, even when their file happens to contain fourth-division players.
func OriginalInitialPacePercent(config Config, team int) int {
	if config.PacePercent[team] > 0 {
		return config.PacePercent[team]
	}
	if config.Skill[team] == 0 && len(config.UserSquads[team]) == 0 && config.SquadFiles[team] == "" && InternationalSquad(config.TeamNames[team]) == "" && (!config.Practice || team == 0) {
		return 90
	}
	return 100
}

// OriginalLeaguePacePercent retains the signed arithmetic at6208..621A.
// Source choices0/1/2 yield110/100/90 for human league fixtures.
func OriginalLeaguePacePercent(skill int) int {
	return 110 - clampInt(skill, 0, 2)*10
}
