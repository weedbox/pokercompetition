package testcases

import (
	"time"

	"github.com/google/uuid"
	"github.com/weedbox/pokercompetition"
)

func NewCTCompetitionSetting() pokercompetition.CompetitionSetting {
	return pokercompetition.CompetitionSetting{
		Meta: pokercompetition.CompetitionMeta{
			Blind: pokercompetition.Blind{
				ID:               uuid.New().String(),
				Name:             "20 min FAST",
				InitialLevel:     1,
				FinalBuyInLevel:  -1,
				DealerBlindTimes: 1,
				Levels: []pokercompetition.BlindLevel{
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
			Ticket: pokercompetition.Ticket{
				ID:   uuid.New().String(),
				Name: "CT 3300 20 min",
			},
			Scene:           "Scene 1",
			MaxDurationMins: 1,
			MinPlayerCount:  3,
			MaxPlayerCount:  9,
			Rule:            pokercompetition.CompetitionRule_Default,
			Mode:            pokercompetition.CompetitionMode_CT,
			BuyInSetting: pokercompetition.BuyInSetting{
				IsFree:     false,
				MinTickets: 1,
				MaxTickets: 2,
			},
			ReBuySetting: pokercompetition.ReBuySetting{
				MinTicket:        1,
				MaxTicket:        2,
				MaxTimes:         6,
				WaitingTimeInSec: 15,
			},
			AddonSetting: pokercompetition.AddonSetting{
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
		TableSettings: []pokercompetition.TableSetting{
			{
				ShortID:        "ABC123",
				Code:           "0001",
				Name:           "20 min - 0001",
				InvitationCode: "welcome to play",
				JoinPlayers:    []pokercompetition.JoinPlayer{},
			},
		},
	}
}
