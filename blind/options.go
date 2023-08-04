package pokerblind

type BlindOptions struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	InitialLevel    int          `json:"initial_level"`
	FinalBuyInLevel int          `json:"final_buy_in_level"`
	Levels          []BlindLevel `json:"levels"`
}
