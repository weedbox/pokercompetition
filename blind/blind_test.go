package pokerblind

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/weedbox/pokerface"
)

func Test_Blind_Start(t *testing.T) {
	// create blind
	blind := NewBlind()

	// apply options
	options := &BlindOptions{
		ID:              uuid.New().String(),
		InitialLevel:    1,
		FinalBuyInLevel: 3,
		Levels: []BlindLevel{
			{
				Level: 1,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     10,
					BB:     20,
				},
				Duration: 2,
			},
			{
				Level: 2,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     20,
					BB:     30,
				},
				Duration: 2,
			},
			{
				Level: 3,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     30,
					BB:     40,
				},
				Duration: 2,
			},
			{
				Level: 4,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     40,
					BB:     50,
				},
				Duration: 1,
			},
			{
				Level: 5,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     50,
					BB:     60,
				},
				Duration: 1,
			},
		},
	}
	bs := blind.ApplyOptions(options)
	assert.NotNil(t, bs, "blind state should not be nil")

	// starting blind
	bs, err := blind.Start()
	assert.NoError(t, err, "starting blind failed")
	assert.Equal(t, 1, bs.CurrentLevel().Level, "current level is wrong")
	assert.NotEqual(t, -1, bs.StartedAt, "start at should not be negative")
}

func Test_Blind_BeforeFinalBuyIn(t *testing.T) {
	// create blind
	blind := NewBlind()

	// apply options
	options := &BlindOptions{
		ID:              uuid.New().String(),
		InitialLevel:    1,
		FinalBuyInLevel: 3,
		Levels: []BlindLevel{
			{
				Level: 1,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     10,
					BB:     20,
				},
				Duration: 2,
			},
			{
				Level: 2,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     20,
					BB:     30,
				},
				Duration: 2,
			},
			{
				Level: 3,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     30,
					BB:     40,
				},
				Duration: 2,
			},
			{
				Level: 4,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     40,
					BB:     50,
				},
				Duration: 1,
			},
			{
				Level: 5,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     50,
					BB:     60,
				},
				Duration: 1,
			},
		},
	}

	bs := blind.ApplyOptions(options)
	assert.NotNil(t, bs, "blind state should not be nil")

	// starting blind
	_, err := blind.Start()
	assert.NoError(t, err, "starting blind failed")

	time.Sleep(time.Second * 1)

	bs = blind.GetState()
	assert.Equal(t, 1, bs.CurrentLevel().Level, "current level is wrong")
	assert.False(t, bs.IsStoppedBuyIn(), "final buy in should be false")
}

func Test_Blind_FinalBuyIn(t *testing.T) {
	// create blind
	blind := NewBlind()

	// apply options
	options := &BlindOptions{
		ID:              uuid.New().String(),
		InitialLevel:    1,
		FinalBuyInLevel: 3,
		Levels: []BlindLevel{
			{
				Level: 1,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     10,
					BB:     20,
				},
				Duration: 2,
			},
			{
				Level: 2,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     20,
					BB:     30,
				},
				Duration: 2,
			},
			{
				Level: 3,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     30,
					BB:     40,
				},
				Duration: 2,
			},
			{
				Level: 4,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     40,
					BB:     50,
				},
				Duration: 1,
			},
			{
				Level: 5,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     50,
					BB:     60,
				},
				Duration: 1,
			},
		},
	}
	bs := blind.ApplyOptions(options)
	assert.NotNil(t, bs, "blind state should not be nil")

	// starting blind
	_, err := blind.Start()
	assert.NoError(t, err, "starting blind failed")

	time.Sleep(time.Second * 5)

	bs = blind.GetState()
	assert.Equal(t, 3, bs.CurrentLevel().Level, "current level is wrong")
	assert.True(t, bs.IsStoppedBuyIn(), "final buy in should be true")
}

func Test_Blind_LevelDuration(t *testing.T) {
	// create blind
	blind := NewBlind()

	// apply options
	options := &BlindOptions{
		ID:              uuid.New().String(),
		InitialLevel:    1,
		FinalBuyInLevel: 3,
		Levels: []BlindLevel{
			{
				Level: 1,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     10,
					BB:     20,
				},
				Duration: 1,
			},
			{
				Level: 2,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     20,
					BB:     30,
				},
				Duration: 1,
			},
		},
	}
	bs := blind.ApplyOptions(options)
	assert.NotNil(t, bs, "blind state should not be nil")

	// starting blind
	_, err := blind.Start()
	assert.NoError(t, err, "starting blind failed")

	time.Sleep(time.Second * 3)

	bs = blind.GetState()
	assert.Equal(t, 2, bs.CurrentLevel().Level, "current level is wrong")
}

func Test_Blind_BreakingLevel(t *testing.T) {
	// create blind
	blind := NewBlind()

	// apply options
	options := &BlindOptions{
		ID:              uuid.New().String(),
		InitialLevel:    1,
		FinalBuyInLevel: 3,
		Levels: []BlindLevel{
			{
				Level: 1,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     10,
					BB:     20,
				},
				Duration: 2,
			},
			{
				Level: 2,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     20,
					BB:     30,
				},
				Duration: 2,
			},
			{
				Level: -1,
				Ante:  0,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     0,
					BB:     0,
				},
				Duration: 2,
			},
			{
				Level: 4,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     40,
					BB:     50,
				},
				Duration: 1,
			},
			{
				Level: 5,
				Ante:  10,
				Blind: pokerface.BlindSetting{
					Dealer: 0,
					SB:     50,
					BB:     60,
				},
				Duration: 1,
			},
		},
	}
	bs := blind.ApplyOptions(options)
	assert.NotNil(t, bs, "blind state should not be nil")

	// starting blind
	_, err := blind.Start()
	assert.NoError(t, err, "starting blind failed")

	time.Sleep(time.Second * 5)

	bs = blind.GetState()
	assert.Equal(t, -1, bs.CurrentLevel().Level, "current level is wrong")
	assert.True(t, bs.IsBreaking(), "should be breaking level")
}
