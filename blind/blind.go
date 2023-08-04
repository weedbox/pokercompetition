package pokerblind

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/weedbox/timebank"
)

var (
	ErrBlindNoOptions = errors.New("blind: options not available")
)

type Blind interface {
	// event
	OnBlindStateUpdated(func(*BlindState))
	OnErrorUpdated(func(*BlindState, error))

	// actions
	GetState() *BlindState
	PrintState() error
	ApplyOptions(options *BlindOptions) *BlindState
	Start() (*BlindState, error)
}

type blind struct {
	options      *BlindOptions
	bs           *BlindState
	stateUpdater func(*BlindState)
	errorUpdater func(*BlindState, error)
}

func NewBlind() Blind {
	return &blind{
		stateUpdater: func(bs *BlindState) {},
		errorUpdater: func(*BlindState, error) {},
	}
}

func (b *blind) OnBlindStateUpdated(fn func(*BlindState)) {
	b.stateUpdater = fn
}

func (b *blind) OnErrorUpdated(fn func(*BlindState, error)) {
	b.errorUpdater = fn
}

func (b *blind) GetState() *BlindState {
	return b.bs
}

func (b *blind) PrintState() error {
	data, err := json.Marshal(b.bs)
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func (b *blind) ApplyOptions(options *BlindOptions) *BlindState {
	b.options = options
	nowUnix := time.Now().Unix()
	levelEndAts := make([]int64, 0)
	for i := 0; i < len(options.Levels); i++ {
		levelEndAts = append(levelEndAts, UnsetValue)
	}

	b.bs = &BlindState{
		BlindID: options.ID,
		Meta: Meta{
			InitialLevel:    options.InitialLevel,
			FinalBuyInLevel: options.FinalBuyInLevel,
			Levels:          options.Levels,
		},
		Status: Status{
			FinalBuyInLevelIndex: UnsetValue,
			CurrentLevelIndex:    UnsetValue,
			LevelEndAts:          levelEndAts,
		},
		CreatedAt: nowUnix,
		StartedAt: UnsetValue,
		UpdatedAt: nowUnix,
	}
	return b.bs
}

func (b *blind) Start() (*BlindState, error) {
	if b.options == nil {
		return nil, ErrBlindNoOptions
	}

	startAt := time.Now()
	b.bs.StartedAt = startAt.Unix()

	for idx, bl := range b.bs.Meta.Levels {
		if bl.Level == b.bs.Meta.InitialLevel {
			b.bs.Status.CurrentLevelIndex = idx
		}
		if bl.Level == b.bs.Meta.FinalBuyInLevel {
			b.bs.Status.FinalBuyInLevelIndex = idx
		}
	}

	for i := (b.bs.Status.CurrentLevelIndex); i < len(b.bs.Meta.Levels); i++ {
		if i == b.bs.Status.CurrentLevelIndex {
			b.bs.Status.LevelEndAts[i] = startAt.Unix()
		} else {
			b.bs.Status.LevelEndAts[i] = b.bs.Status.LevelEndAts[i-1]
		}
		blindPassedSeconds := int64(b.bs.Meta.Levels[i].Duration)
		b.bs.Status.LevelEndAts[i] += blindPassedSeconds

		// update blind to all tables
		go b.updateLevel(b.bs.Status.LevelEndAts[i])
	}

	return b.bs, nil
}

func (b *blind) updateLevel(endAt int64) {
	levelEndTime := time.Unix(endAt, 0)
	if err := timebank.NewTimeBank().NewTaskWithDeadline(levelEndTime, func(isCancelled bool) {
		if isCancelled {
			return
		}

		if b.bs.Status.CurrentLevelIndex+1 < len(b.bs.Meta.Levels) {
			b.bs.Status.CurrentLevelIndex++
			b.emitState()
		}
	}); err != nil {
		b.errorUpdater(b.bs, err)
	}
}

func (b *blind) emitState() {
	b.bs.UpdatedAt = time.Now().Unix()
	b.stateUpdater(b.bs)
}
