package pokercompetition

import (
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
	ShortID        string       `json:"short_id"`
	Code           string       `json:"code"`
	Name           string       `json:"name"`
	InvitationCode string       `json:"invitation_code"`
	JoinPlayers    []JoinPlayer `json:"join_players"`
}

type JoinPlayer struct {
	PlayerID    string `json:"player_id"`
	RedeemChips int64  `json:"redeem_chips"`
}

func NewPokerTableSetting(competitionID string, competitionMeta CompetitionMeta, tableSetting TableSetting) pokertable.TableSetting {
	return pokertable.TableSetting{
		ShortID:        tableSetting.ShortID,
		Code:           tableSetting.Code,
		Name:           tableSetting.Name,
		InvitationCode: tableSetting.InvitationCode,
		CompetitionMeta: pokertable.CompetitionMeta{
			ID:                  competitionID,
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
