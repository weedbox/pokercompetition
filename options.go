package pokercompetition

type CompetitionEngineOptions struct {
	OnCompetitionUpdated                func(*Competition)
	OnCompetitionErrorUpdated           func(*Competition, error)
	OnCompetitionPlayerUpdated          func(string, *CompetitionPlayer)
	OnCompetitionFinalPlayerRankUpdated func(string, string, int)
	OnCompetitionStateUpdated           func(string, *Competition)
}

func NewDefaultCompetitionEngineOptions() *CompetitionEngineOptions {
	return &CompetitionEngineOptions{
		OnCompetitionUpdated:                func(*Competition) {},
		OnCompetitionErrorUpdated:           func(*Competition, error) {},
		OnCompetitionPlayerUpdated:          func(string, *CompetitionPlayer) {},
		OnCompetitionFinalPlayerRankUpdated: func(string, string, int) {},
		OnCompetitionStateUpdated:           func(string, *Competition) {},
	}
}
