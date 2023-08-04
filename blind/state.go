package pokerblind

import "github.com/weedbox/pokerface"

const (
	UnsetValue = -1
)

type BlindState struct {
	BlindID   string `json:"blind_id"`
	Meta      Meta   `json:"meta"`
	Status    Status `json:"status"`
	CreatedAt int64  `json:"created_at"`
	StartedAt int64  `json:"started_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type Meta struct {
	InitialLevel    int          `json:"initial_level"`
	FinalBuyInLevel int          `json:"final_buy_in_level"`
	Levels          []BlindLevel `json:"levels"`
}

type Status struct {
	FinalBuyInLevelIndex int     `json:"final_buy_in_level_idx"`
	CurrentLevelIndex    int     `json:"current_level_index"`
	LevelEndAts          []int64 `json:"level_end_ats"`
}

type BlindLevel struct {
	Level    int                    `json:"level"` // 盲注等級(-1 表示中場休息)
	Ante     int64                  `json:"ante"`
	Blind    pokerface.BlindSetting `json:"blind"`
	Duration int                    `json:"duration"` // 等級持續時間 (Seconds)
}

func (bs *BlindState) CurrentLevel() BlindLevel {
	return bs.Meta.Levels[bs.Status.CurrentLevelIndex]
}

func (bs *BlindState) IsActive() bool {
	return bs.StartedAt != UnsetValue
}

func (bs *BlindState) IsBreaking() bool {
	return bs.CurrentLevel().Level == -1
}

func (bs *BlindState) IsStoppedBuyIn() bool {
	// 沒有預設 FinalBuyInLevelIndex 代表不能補碼，永遠都是停止買入階段
	if bs.Status.FinalBuyInLevelIndex == UnsetValue {
		return true
	}
	return bs.Status.CurrentLevelIndex >= bs.Status.FinalBuyInLevelIndex
}
