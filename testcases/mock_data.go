package testcases

import (
	"time"

	"github.com/google/uuid"
	"github.com/weedbox/pokercompetition"
	"github.com/weedbox/pokercompetition/model"
	pokertablemodel "github.com/weedbox/pokertable/model"
)

type TableSettingPayload struct {
	ShortID        string
	Code           string
	Name           string
	InvitationCode string
	JoinPlayers    []pokercompetition.JoinPlayer
}

type CompetitionSettingPayload struct {
	// Competition
	Meta          model.CompetitionMeta
	StartAt   int64
	DisableAt int64

	// Tables
	TableSettings []TableSettingPayload
}

func NewCTCompetitionSettingPayload(tableSettings ...TableSettingPayload) CompetitionSettingPayload {
	return CompetitionSettingPayload{
		Meta: model.CompetitionMeta{
			Blind: model.Blind{
				ID:               uuid.New().String(),
				Name:             "20 min FAST",
				InitialLevel:     1,
				FinalBuyInLevel:  2,
				DealerBlindTimes: 1,
				Levels: []model.BlindLevel{
					{
						Level:        1,
						SBChips:      10,
						BBChips:      20,
						AnteChips:    0,
						DurationMins: 10,
					},
					{
						Level:        2,
						SBChips:      20,
						BBChips:      30,
						AnteChips:    0,
						DurationMins: 10,
					},
					{
						Level:        3,
						SBChips:      30,
						BBChips:      40,
						AnteChips:    0,
						DurationMins: 10,
					},
				},
			},
			Ticket: model.Ticket{
				ID:   uuid.New().String(),
				Name: "CT 3300 20 min",
			},
			Scene:           "Scene 1",
			MaxDurationMins: 60,
			MinPlayerCount:  3,
			MaxPlayerCount:  9,
			Rule:            model.CompetitionRule_Default,
			Mode:            model.CompetitionMode_CT,
			BuyInSetting: model.BuyInSetting{
				IsFree:     false,
				MinTickets: 1,
				MaxTickets: 2,
			},
			ReBuySetting: model.ReBuySetting{
				MinTicket:        1,
				MaxTicket:        2,
				MaxTimes:         6,
				WaitingTimeInSec: 15,
			},
			AddonSetting: model.AddonSetting{
				IsBreakOnly: true,
				RedeemChips: []int64{},
				MaxTimes:    0,
			},
			ActionTimeSecs:       10,
			TableMaxSeatCount:    9,
			TableMinPlayingCount: 2,
			MinChipsUnit:         10,
		},
		StartAt:   -1,
		DisableAt: time.Now().Add(time.Hour * 24).Unix(),
		TableSettings: tableSettings,
	}
}

func NewTableSettingPayload() TableSettingPayload {
	return TableSettingPayload{
		ShortID:        "ABC123",
		Code:           "01",
		Name:           "table name",
		InvitationCode: "come to play",
		JoinPlayers:    []pokercompetition.JoinPlayer{},
	}
}

func NewDefaultCompetitionSetting(competitionSetting CompetitionSettingPayload) pokercompetition.CompetitionSetting {
	return pokercompetition.CompetitionSetting{
		Meta:          competitionSetting.Meta,
		StartAt:   competitionSetting.StartAt,
		DisableAt: competitionSetting.DisableAt,
	}
}

func NewDefaultTableSetting(competitionMeta model.CompetitionMeta, tableSetting TableSettingPayload) pokertablemodel.TableSetting {
	blindLevels := []pokertablemodel.BlindLevel{}
	for _, bl := range competitionMeta.Blind.Levels {
		blindLevels = append(blindLevels, pokertablemodel.BlindLevel{
			Level:        bl.Level,
			SBChips:      bl.SBChips,
			BBChips:      bl.BBChips,
			AnteChips:    bl.AnteChips,
			DurationMins: bl.DurationMins,
		})
	}

	joinPlayers := []pokertablemodel.JoinPlayer{}
	for _, player := range tableSetting.JoinPlayers {
		joinPlayers = append(joinPlayers, pokertablemodel.JoinPlayer{
			PlayerID:    player.PlayerID,
			RedeemChips: player.RedeemChips,
		})
	}

	return pokertablemodel.TableSetting{
		ShortID:        tableSetting.ShortID,
		Code:           tableSetting.Code,
		Name:           tableSetting.Name,
		InvitationCode: tableSetting.InvitationCode,
		CompetitionMeta: pokertablemodel.CompetitionMeta{
			Blind: pokertablemodel.Blind{
				ID:              competitionMeta.Blind.ID,
				Name:            competitionMeta.Blind.Name,
				FinalBuyInLevel: competitionMeta.Blind.FinalBuyInLevel,
				InitialLevel:    competitionMeta.Blind.InitialLevel,
				Levels:          blindLevels,
			},
			MaxDurationMins:      competitionMeta.MaxDurationMins,
			Rule:                 string(competitionMeta.Rule),
			Mode:                 string(competitionMeta.Mode),
			TableMaxSeatCount:    competitionMeta.TableMaxSeatCount,
			TableMinPlayingCount: competitionMeta.TableMinPlayingCount,
			MinChipsUnit:         competitionMeta.MinChipsUnit,
		},
		JoinPlayers: joinPlayers,
	}
}
