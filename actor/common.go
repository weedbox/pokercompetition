package actor

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weedbox/pokercompetition"
)

func NewCTCompetitionSetting() pokercompetition.CompetitionSetting {
	return pokercompetition.CompetitionSetting{
		Meta: pokercompetition.CompetitionMeta{
			Blind: pokercompetition.Blind{
				ID:              uuid.New().String(),
				Name:            "20 min FAST",
				InitialLevel:    1,
				FinalBuyInLevel: 2,
				DealerBlindTime: 1,
				Levels: []pokercompetition.BlindLevel{
					{
						Level:    1,
						SB:       10,
						BB:       20,
						Ante:     0,
						Duration: 3,
					},
					{
						Level:    2,
						SB:       20,
						BB:       30,
						Ante:     0,
						Duration: 3,
					},
					{
						Level:    3,
						SB:       30,
						BB:       40,
						Ante:     0,
						Duration: 3,
					},
				},
			},
			Ticket: pokercompetition.Ticket{
				ID:   uuid.New().String(),
				Name: "CT 3300 20 min",
			},
			Scene:          "Scene 1",
			MaxDuration:    15,
			MinPlayerCount: 3,
			MaxPlayerCount: 9,
			Rule:           pokercompetition.CompetitionRule_Default,
			Mode:           pokercompetition.CompetitionMode_CT,
			BuyInSetting: pokercompetition.BuyInSetting{
				IsFree:    false,
				MinTicket: 1,
				MaxTicket: 2,
			},
			ReBuySetting: pokercompetition.ReBuySetting{
				MinTicket:   1,
				MaxTicket:   2,
				MaxTime:     6,
				WaitingTime: 3,
			},
			AddonSetting: pokercompetition.AddonSetting{
				IsBreakOnly: true,
				RedeemChips: []int64{},
				MaxTime:     0,
			},
			ActionTime:          10,
			TableMaxSeatCount:   9,
			TableMinPlayerCount: 2,
			MinChipUnit:         10,
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

func NewMTTCompetitionSetting() pokercompetition.CompetitionSetting {
	return pokercompetition.CompetitionSetting{
		Meta: pokercompetition.CompetitionMeta{
			Blind: pokercompetition.Blind{
				ID:              uuid.New().String(),
				Name:            "30 min MTT",
				InitialLevel:    1,
				FinalBuyInLevel: 2,
				DealerBlindTime: 1,
				Levels: []pokercompetition.BlindLevel{
					{
						Level:    1,
						SB:       10,
						BB:       20,
						Ante:     0,
						Duration: 3,
					},
					{
						Level:    2,
						SB:       20,
						BB:       30,
						Ante:     0,
						Duration: 3,
					},
					{
						Level:    3,
						SB:       30,
						BB:       40,
						Ante:     0,
						Duration: 3,
					},
				},
			},
			Ticket: pokercompetition.Ticket{
				ID:   uuid.New().String(),
				Name: "MTT 3300 30 min",
			},
			Scene:          "Scene 1",
			MaxDuration:    999999,
			MinPlayerCount: 2,
			MaxPlayerCount: 9,
			Rule:           pokercompetition.CompetitionRule_Default,
			Mode:           pokercompetition.CompetitionMode_MTT,
			BuyInSetting: pokercompetition.BuyInSetting{
				IsFree:    false,
				MinTicket: 1,
				MaxTicket: 1,
			},
			ReBuySetting: pokercompetition.ReBuySetting{
				MinTicket:   1,
				MaxTicket:   1,
				MaxTime:     6,
				WaitingTime: 1,
			},
			AddonSetting: pokercompetition.AddonSetting{
				IsBreakOnly: true,
				RedeemChips: []int64{1000, 1200, 1500},
				MaxTime:     3,
			},
			ActionTime:          10,
			TableMaxSeatCount:   9,
			TableMinPlayerCount: 2,
			MinChipUnit:         10,
		},
		StartAt:       -1,
		DisableAt:     time.Now().Add(time.Hour * 24).Unix(),
		TableSettings: []pokercompetition.TableSetting{},
	}
}

func LogJSON(t *testing.T, msg string, jsonPrinter func() (string, error)) {
	json, _ := jsonPrinter()
	fmt.Printf("\n===== [%s] =====\n%s\n", msg, json)
}
