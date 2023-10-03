package actor

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weedbox/pokercompetition"
)

func NewCTCompetitionSetting_Breaking() pokercompetition.CompetitionSetting {
	blindLevels := []pokercompetition.BlindLevel{
		{
			Level:      1,
			SB:         10,
			BB:         20,
			Ante:       0,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      2,
			SB:         10,
			BB:         20,
			Ante:       0,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      3,
			SB:         20,
			BB:         40,
			Ante:       4,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      4,
			SB:         30,
			BB:         60,
			Ante:       6,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      5,
			SB:         40,
			BB:         80,
			Ante:       8,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      -1,
			SB:         0,
			BB:         0,
			Ante:       0,
			Duration:   10,
			AllowAddon: false,
		},
		{
			Level:      6,
			SB:         50,
			BB:         100,
			Ante:       10,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      7,
			SB:         80,
			BB:         160,
			Ante:       16,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      8,
			SB:         100,
			BB:         200,
			Ante:       20,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      9,
			SB:         150,
			BB:         300,
			Ante:       30,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      10,
			SB:         200,
			BB:         400,
			Ante:       40,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      -1,
			SB:         0,
			BB:         0,
			Ante:       0,
			Duration:   10,
			AllowAddon: true,
		},
		{
			Level:      11,
			SB:         300,
			BB:         600,
			Ante:       60,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      12,
			SB:         400,
			BB:         800,
			Ante:       80,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      13,
			SB:         500,
			BB:         1000,
			Ante:       100,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      14,
			SB:         800,
			BB:         1600,
			Ante:       160,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      15,
			SB:         1000,
			BB:         2000,
			Ante:       200,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      -1,
			SB:         0,
			BB:         0,
			Ante:       0,
			Duration:   10,
			AllowAddon: false,
		},
		{
			Level:      16,
			SB:         1500,
			BB:         3000,
			Ante:       300,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      17,
			SB:         2000,
			BB:         4000,
			Ante:       400,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      18,
			SB:         3000,
			BB:         6000,
			Ante:       600,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      19,
			SB:         4000,
			BB:         8000,
			Ante:       800,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      20,
			SB:         5000,
			BB:         10000,
			Ante:       1000,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      -1,
			SB:         0,
			BB:         0,
			Ante:       0,
			Duration:   10,
			AllowAddon: false,
		},
		{
			Level:      21,
			SB:         6000,
			BB:         12000,
			Ante:       1200,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      22,
			SB:         8000,
			BB:         16000,
			Ante:       1000,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      23,
			SB:         10000,
			BB:         20000,
			Ante:       2000,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      24,
			SB:         15000,
			BB:         30000,
			Ante:       3000,
			Duration:   2,
			AllowAddon: false,
		},
		{
			Level:      25,
			SB:         20000,
			BB:         40000,
			Ante:       4000,
			Duration:   2,
			AllowAddon: false,
		},
	}
	maxDuration := 0
	for _, bl := range blindLevels {
		maxDuration += bl.Duration
	}

	return pokercompetition.CompetitionSetting{
		CompetitionID: uuid.New().String(),
		Meta: pokercompetition.CompetitionMeta{
			Blind: pokercompetition.Blind{
				ID:                   uuid.New().String(),
				InitialLevel:         1,
				FinalBuyInLevelIndex: 3,
				DealerBlindTime:      1,
				Levels:               blindLevels,
			},
			MaxDuration:    maxDuration,
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

func NewCTCompetitionSetting_Normal() pokercompetition.CompetitionSetting {
	return pokercompetition.CompetitionSetting{
		CompetitionID: uuid.New().String(),
		Meta: pokercompetition.CompetitionMeta{
			Blind: pokercompetition.Blind{
				ID:                   uuid.New().String(),
				InitialLevel:         1,
				FinalBuyInLevelIndex: 1,
				DealerBlindTime:      1,
				Levels: []pokercompetition.BlindLevel{
					{
						Level:    1,
						SB:       10,
						BB:       20,
						Ante:     0,
						Duration: 5,
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
				ID:                   uuid.New().String(),
				InitialLevel:         1,
				FinalBuyInLevelIndex: 1,
				DealerBlindTime:      1,
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
