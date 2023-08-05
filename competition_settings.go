package pokercompetition

import (
	"github.com/google/uuid"
	"github.com/thoas/go-funk"
	"github.com/weedbox/pokertable"
)

type CompetitionSetting struct {
	Meta          CompetitionMeta `json:"meta"`
	StartAt       int64           `json:"start_game_at"`
	DisableAt     int64           `json:"disable_game_at"`
	TableSettings []TableSetting  `json:"table_settings"`
}

type TableSetting struct {
	TableID     string       `json:"table_id"`
	JoinPlayers []JoinPlayer `json:"join_players"`
}

type JoinPlayer struct {
	PlayerID    string `json:"player_id"`
	RedeemChips int64  `json:"redeem_chips"`
}

func NewPokerTableSetting(competitionID string, competitionMeta CompetitionMeta, tableSetting TableSetting) pokertable.TableSetting {
	return pokertable.TableSetting{
		TableID: uuid.New().String(),
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
		JoinPlayers: funk.Map(tableSetting.JoinPlayers, func(joinPlayer JoinPlayer) pokertable.JoinPlayer {
			return pokertable.JoinPlayer{
				PlayerID:    joinPlayer.PlayerID,
				RedeemChips: joinPlayer.RedeemChips,
			}
		}).([]pokertable.JoinPlayer),
	}
}
