package pokercompetition

import (
	"github.com/weedbox/pokertable"
)

type CompetitionSetting struct {
	CompetitionID string          `json:"competition_id"`
	Meta          CompetitionMeta `json:"meta"`
	StartAt       int64           `json:"start_game_at"`
	DisableAt     int64           `json:"disable_game_at"`
	TableSettings []TableSetting  `json:"table_settings"`
}

type TableSetting struct {
	TableID     string                  `json:"table_id"`
	JoinPlayers []pokertable.JoinPlayer `json:"join_players"`
}

type JoinPlayer struct {
	PlayerID    string `json:"player_id"`
	RedeemChips int64  `json:"redeem_chips"`
}

func NewPokerTableSetting(competitionID string, competitionMeta CompetitionMeta, tableSetting TableSetting) pokertable.TableSetting {
	return pokertable.TableSetting{
		TableID: tableSetting.TableID,
		Meta: pokertable.TableMeta{
			CompetitionID:       competitionID,
			Rule:                string(competitionMeta.Rule),
			Mode:                string(competitionMeta.Mode),
			MaxDuration:         competitionMeta.MaxDuration,
			TableMaxSeatCount:   competitionMeta.TableMaxSeatCount,
			TableMinPlayerCount: competitionMeta.TableMinPlayerCount,
			MinChipUnit:         competitionMeta.MinChipUnit,
			ActionTime:          competitionMeta.ActionTime,
		},
		JoinPlayers: tableSetting.JoinPlayers,
	}
}
