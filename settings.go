package pokercompetition

import (
	"github.com/weedbox/pokercompetition/model"
)

type CompetitionSetting struct {
	Meta          model.CompetitionMeta `json:"meta"`
	StartAt   int64                 `json:"start_game_at"`
	DisableAt int64                 `json:"disable_game_at"`
	// Tables        []*pokertablemodel.Table `json:"tables"`
}

type JoinPlayer struct {
	TableID     string `json:"table_id"`
	PlayerID    string `json:"player_id"`
	RedeemChips int64  `json:"redeem_chips"`
}
