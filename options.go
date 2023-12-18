package pokercompetition

type CompetitionEngineOptions struct {
	OnCompetitionUpdated                func(competition *Competition)
	OnCompetitionErrorUpdated           func(competition *Competition, err error)
	OnCompetitionPlayerUpdated          func(competitionID string, competitionPlayer *CompetitionPlayer)
	OnCompetitionFinalPlayerRankUpdated func(competitionID, playerID string, rank int)
	OnCompetitionStateUpdated           func(competitionID string, competition *Competition)
	OnAdvancePlayerCountUpdated         func(competitionID string, totalBuyInCount int) int
	OnCompetitionPlayerCashOut          func(competitionID string, competitionPlayer *CompetitionPlayer)
}

func NewDefaultCompetitionEngineOptions() *CompetitionEngineOptions {
	return &CompetitionEngineOptions{
		OnCompetitionUpdated:                func(competition *Competition) {},
		OnCompetitionErrorUpdated:           func(competition *Competition, err error) {},
		OnCompetitionPlayerUpdated:          func(competitionID string, competitionPlayer *CompetitionPlayer) {},
		OnCompetitionFinalPlayerRankUpdated: func(competitionID, playerID string, rank int) {},
		OnCompetitionStateUpdated:           func(competitionID string, competition *Competition) {},
		OnAdvancePlayerCountUpdated:         func(competitionID string, totalBuyInCount int) int { return 0 },
		OnCompetitionPlayerCashOut:          func(competitionID string, competitionPlayer *CompetitionPlayer) {},
	}
}
