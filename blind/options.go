package pokerblind

type BlindOptions struct {
	ID                   string       `json:"id"`
	InitialLevel         int          `json:"initial_level"`
	FinalBuyInLevelIndex int          `json:"final_buy_in_level_index"`
	Levels               []BlindLevel `json:"levels"`
}
