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
		CompetitionID: uuid.New().String(),
		Meta: pokercompetition.CompetitionMeta{
			Blind: pokercompetition.Blind{
				ID:              uuid.New().String(),
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
			MaxDuration:    15,
			MinPlayerCount: 3,
			MaxPlayerCount: 9,
			Rule:           pokercompetition.CompetitionRule_Default,
			Mode:           pokercompetition.CompetitionMode_CT,
			ReBuySetting: pokercompetition.ReBuySetting{
				MaxTime:     6,
				WaitingTime: 1,
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
				TableID:     uuid.New().String(),
				JoinPlayers: []pokercompetition.JoinPlayer{},
			},
		},
	}
}

func NewMTTCompetitionSetting() pokercompetition.CompetitionSetting {
	return pokercompetition.CompetitionSetting{
		CompetitionID: uuid.New().String(),
		Meta: pokercompetition.CompetitionMeta{
			Blind: pokercompetition.Blind{
				ID:              uuid.New().String(),
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
			MaxDuration:    999999,
			MinPlayerCount: 2,
			MaxPlayerCount: 9,
			Rule:           pokercompetition.CompetitionRule_Default,
			Mode:           pokercompetition.CompetitionMode_MTT,
			ReBuySetting: pokercompetition.ReBuySetting{
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
		// StartAt: time.Now().Add(time.Second * 2).Unix(),
		StartAt:       -1,
		DisableAt:     time.Now().Add(time.Hour * 24).Unix(),
		TableSettings: []pokercompetition.TableSetting{},
	}
}

func LogJSON(t *testing.T, msg string, jsonPrinter func() (string, error)) {
	json, _ := jsonPrinter()
	fmt.Printf("\n===== [%s] =====\n%s\n", msg, json)
}
