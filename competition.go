package pokercompetition

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/thoas/go-funk"
	"github.com/weedbox/pokertable"
)

type CompetitionStateStatus string
type CompetitionPlayerStatus string
type CompetitionMode string
type CompetitionRule string

const (
	// CompetitionStateStatus
	CompetitionStateStatus_PreRegistering CompetitionStateStatus = "pre-registering" // 賽事已建立 (但不可報名)
	CompetitionStateStatus_Registering    CompetitionStateStatus = "registering"     // 賽事已建立 (可報名參賽)
	CompetitionStateStatus_DelayedBuyIn   CompetitionStateStatus = "delayed_buy_in"  // 賽事已建立 (延遲買入)
	CompetitionStateStatus_StoppedBuyIn   CompetitionStateStatus = "stopped_buy_in"  // 賽事已建立 (停止買入)
	CompetitionStateStatus_End            CompetitionStateStatus = "end"             // 賽事已結束 (正常結束)
	CompetitionStateStatus_AutoEnd        CompetitionStateStatus = "auto_end"        // 賽事已結束 (開賽未成功自動關閉)
	CompetitionStateStatus_ForceEnd       CompetitionStateStatus = "force_end"       // 賽事已結束 (其他原因強制關閉)
	CompetitionStateStatus_Restoring      CompetitionStateStatus = "restoring"       // 賽事資料轉移中 (Graceful Shutdown Use)

	// CompetitionPlayerStatus
	CompetitionPlayerStatus_WaitingTableBalancing CompetitionPlayerStatus = "waiting_table_balancing" // 等待拆併桌中
	CompetitionPlayerStatus_Playing               CompetitionPlayerStatus = "playing"                 // 比賽中
	CompetitionPlayerStatus_ReBuyWaiting          CompetitionPlayerStatus = "re_buy_waiting"          // 等待補碼中 (已不再桌次內)
	CompetitionPlayerStatus_Knockout              CompetitionPlayerStatus = "knockout"                // 已淘汰

	// CompetitionMode
	CompetitionMode_CT   CompetitionMode = "ct"   // 倒數錦標賽
	CompetitionMode_MTT  CompetitionMode = "mtt"  // 大型錦標賽
	CompetitionMode_Cash CompetitionMode = "cash" // 現金桌

	// CompetitionRule
	CompetitionRule_Default   CompetitionRule = "default"    // 常牌
	CompetitionRule_ShortDeck CompetitionRule = "short_deck" // 短牌
	CompetitionRule_Omaha     CompetitionRule = "omaha"      // 奧瑪哈
)

type Competition struct {
	UpdateSerial int64             `json:"update_serial" mapstructure:"update_serial"` // 更新序列號 (數字越大越晚發生)
	ID           string            `json:"id" mapstructure:"id"`                       // 賽事 Unique ID
	Meta         CompetitionMeta   `json:"meta" mapstructure:"meta"`                   // 賽事固定資料
	State        *CompetitionState `json:"state" mapstructure:"state"`                 // 賽事動態資料
	UpdateAt     int64             `json:"update_at" mapstructure:"update_at"`         // 更新時間 (Seconds)
}

type CompetitionMeta struct {
	Blind               Blind           `json:"blind" mapstructure:"blind"`                                   // 盲注資訊
	MaxDuration         int             `json:"max_duration" mapstructure:"max_duration"`                     // 比賽時間總長 (Seconds)
	MinPlayerCount      int             `json:"min_player_count" mapstructure:"min_player_count"`             // 最小參賽人數
	MaxPlayerCount      int             `json:"max_player_count" mapstructure:"max_player_count"`             // 最大參賽人數
	TableMaxSeatCount   int             `json:"table_max_seat_count" mapstructure:"table_max_seat_count"`     // 每桌人數上限
	TableMinPlayerCount int             `json:"table_min_player_count" mapstructure:"table_min_player_count"` // 每桌最小開打數
	Rule                CompetitionRule `json:"rule" mapstructure:"rule"`                                     // 德州撲克規則, 常牌(default), 短牌(short_deck), 奧瑪哈(omaha)
	Mode                CompetitionMode `json:"mode" mapstructure:"mode"`                                     // 賽事模式 (CT, MTT, Cash)
	ReBuySetting        ReBuySetting    `json:"re_buy_setting" mapstructure:"re_buy_setting"`                 // ReBuy 設定
	AddonSetting        AddonSetting    `json:"addon_setting" mapstructure:"addon_setting"`                   // Addon 設定
	ActionTime          int             `json:"action_time" mapstructure:"action_time"`                       // 思考時間 (Seconds)
	MinChipUnit         int64           `json:"min_chip_unit" mapstructure:"min_chip_unit"`                   // 最小單位籌碼量
}

type CompetitionState struct {
	OpenAt     int64                  `json:"open_at" mapstructure:"open_at"`         // 賽事建立時間 (可報名、尚未開打)
	DisableAt  int64                  `json:"disable_at" mapstructure:"disable_at"`   // 賽事未開打前，賽局可見時間 (Seconds)
	StartAt    int64                  `json:"start_at" mapstructure:"start_at"`       // 賽事開打時間 (可報名、開打) (Seconds)
	EndAt      int64                  `json:"end_at" mapstructure:"end_at"`           // 賽事結束時間 (Seconds)
	BlindState *BlindState            `json:"blind_state" mapstructure:"blind_state"` // 盲注狀態
	Players    []*CompetitionPlayer   `json:"players" mapstructure:"players"`         // 參與過賽事玩家陣列
	Status     CompetitionStateStatus `json:"status" mapstructure:"status"`           // 賽事狀態
	Tables     []*pokertable.Table    `json:"tables" mapstructure:"tables"`           // 多桌
	Rankings   []*CompetitionRank     `json:"rankings" mapstructure:"rankings"`       // 停止買入後玩家排名 (陣列 Index 即是排名 rank - 1, ex: index 0 -> 第一名, index 1 -> 第二名...)
}

type CompetitionRank struct {
	PlayerID   string `json:"player_id" mapstructure:"player_id"`     // 玩家 ID
	FinalChips int64  `json:"final_chips" mapstructure:"final_chips"` // 玩家最後籌碼數
}

type CompetitionPlayer struct {
	PlayerID       string `json:"player_id" mapstructure:"player_id"` // 玩家 ID
	CurrentTableID string `json:"table_id" mapstructure:"table_id"`   // 當前桌次 ID
	CurrentSeat    int    `json:"seat" mapstructure:"seat"`           // 當前座位
	JoinAt         int64  `json:"join_at" mapstructure:"join_at"`     // 加入時間 (Seconds)

	// current info
	Status     CompetitionPlayerStatus `json:"status" mapstructure:"status"`               // 參與玩家狀態
	Rank       int                     `json:"rank" mapstructure:"rank"`                   // 當前桌次排名
	Chips      int64                   `json:"chips" mapstructure:"chips"`                 // 當前籌碼
	IsReBuying bool                    `json:"is_re_buying" mapstructure:"is_re_buying"`   // 是否正在補碼
	ReBuyEndAt int64                   `json:"re_buy_end_at" mapstructure:"re_buy_end_at"` // 最後補碼時間 (Seconds)
	ReBuyTimes int                     `json:"re_buy_times" mapstructure:"re_buy_times"`   // 補碼次數
	AddonTimes int                     `json:"addon_times" mapstructure:"addon_times"`     // 增購次數

	// statistics info
	// best
	BestWinningPotChips int64    `json:"best_winning_pot_chips" mapstructure:"best_winning_pot_chips"` // 贏得最大底池籌碼數
	BestWinningCombo    []string `json:"best_winning_combo" mapstructure:"best_winning_combo"`         // 身為贏家時最大的牌型組合
	BestWinningType     string   `json:"best_winning_type" mapstructure:"best_winning_type"`           // 身為贏家時最大的牌型類型
	BestWinningPower    int      `json:"best_winning_power" mapstructure:"best_winning_power"`         // 身為贏家時最大的牌型牌力

	// accumulated info
	// competition/table
	TotalRedeemChips int64 `json:"total_redeem_chips" mapstructure:"total_redeem_chips"` // 累積兌換籌碼
	TotalGameCounts  int64 `json:"total_game_counts" mapstructure:"total_game_counts"`   // 總共玩幾手牌

	// game: round & actions
	TotalWalkTimes        int64 `json:"total_walk_times" mapstructure:"total_walk_times"`                 // Preflop 除了大盲以外的人全部 Fold，而贏得籌碼的次數
	TotalVPIPTimes        int   `json:"total_vpip_times" mapstructure:"total_vpip_times"`                 // 入池總次數
	TotalFoldTimes        int   `json:"total_fold_times" mapstructure:"total_fold_times"`                 // 棄牌總次數
	TotalPreflopFoldTimes int   `json:"total_preflop_fold_times" mapstructure:"total_preflop_fold_times"` // Preflop 棄牌總次數
	TotalFlopFoldTimes    int   `json:"total_flop_fold_times" mapstructure:"total_flop_fold_times"`       // Flop 棄牌總次數
	TotalTurnFoldTimes    int   `json:"total_turn_fold_times" mapstructure:"total_turn_fold_times"`       // Turn 棄牌總次數
	TotalRiverFoldTimes   int   `json:"total_river_fold_times" mapstructure:"total_river_fold_times"`     // River 棄牌總次數
	TotalActionTimes      int   `json:"total_action_times" mapstructure:"total_action_times"`             // 下注動作總次數
	TotalRaiseTimes       int   `json:"total_raise_times" mapstructure:"total_raise_times"`               // 加注/入池總次數(AllIn&Raise、Raise、Bet)
	TotalCallTimes        int   `json:"total_call_times" mapstructure:"total_call_times"`                 // 跟注總次數
	TotalCheckTimes       int   `json:"total_check_times" mapstructure:"total_check_times"`               // 過牌總次數
	TotalProfitTimes      int   `json:"total_profit_times" mapstructure:"total_profit_times"`             // 總共贏得籌碼次數
}

type Blind struct {
	ID              string       `json:"id" mapstructure:"id"`                                 // ID
	InitialLevel    int          `json:"initial_level" mapstructure:"initial_level"`           // 起始盲注級別
	FinalBuyInLevel int          `json:"final_buy_in_level" mapstructure:"final_buy_in_level"` // 最後買入盲注等級
	DealerBlindTime int          `json:"dealer_blind_time" mapstructure:"dealer_blind_time"`   // Dealer 位置要收取的前注倍數 (短牌用)
	Levels          []BlindLevel `json:"levels" mapstructure:"levels"`                         // 級別資訊列表
}

type BlindLevel struct {
	Level      int   `json:"level" mapstructure:"level"`             // 盲注等級(-1 表示中場休息)
	SB         int64 `json:"sb" mapstructure:"sb"`                   // 小盲籌碼量
	BB         int64 `json:"bb" mapstructure:"bb"`                   // 大盲籌碼量
	Ante       int64 `json:"ante" mapstructure:"ante"`               // 前注籌碼量
	Duration   int   `json:"duration" mapstructure:"duration"`       // 等級持續時間 (Seconds)
	AllowAddon bool  `json:"allow_addon" mapstructure:"allow_addon"` // 是否允許增購
}

type BlindState struct {
	FinalBuyInLevelIndex int     `json:"final_buy_in_level_idx" mapstructure:"final_buy_in_level_idx"` // 最後買入盲注等級索引值
	CurrentLevelIndex    int     `json:"current_level_index" mapstructure:"current_level_index"`       // 現在盲注等級級別索引值
	EndAts               []int64 `json:"end_ats" mapstructure:"end_ats"`                               // 每個等級結束時間 (Seconds)
}

type ReBuySetting struct {
	MaxTime     int `json:"max_time" mapstructure:"max_time"`         // 最大次數
	WaitingTime int `json:"waiting_time" mapstructure:"waiting_time"` // 玩家可補碼時間 (Seconds)
}

type AddonSetting struct {
	IsBreakOnly bool    `json:"is_break_only" mapstructure:"is_break_only"` // 是否中場休息限定
	RedeemChips []int64 `json:"redeem_chips" mapstructure:"redeem_chips"`   // 可兌換籌碼數
	MaxTime     int     `json:"max_time" mapstructure:"max_time"`           // 最大次數
}

// Competition Setters
func (c *Competition) AsPlayer() {
	c.State.Tables = nil
}

// Competition Getters
func (c Competition) GetJSON() (string, error) {
	encoded, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func (c Competition) CanStart() bool {
	if c.State.Status != CompetitionStateStatus_Registering {
		return false
	}

	currentPlayerCount := 0
	for _, table := range c.State.Tables {
		for _, player := range table.State.PlayerStates {
			if player.IsIn && player.Bankroll > 0 {
				currentPlayerCount++
			}
		}
	}

	if currentPlayerCount >= c.Meta.MinPlayerCount {
		// 開打條件一: 當賽局已經設定 StartAt (開打時間) & 現在時間已經大於等於開打時間且達到最小開桌人數
		if c.State.StartAt > 0 && time.Now().Unix() >= c.State.StartAt {
			return true
		}

		// 開打條件二: 賽局沒有設定 StartAt (開打時間) & 達到最小開桌人數
		if c.State.StartAt <= 0 {
			return true
		}
	}
	return false
}

func (c Competition) PlayingPlayerCount() int {
	return len(funk.Filter(c.State.Players, func(player *CompetitionPlayer) bool {
		return player.Chips > 0
	}).([]*CompetitionPlayer))
}

func (c Competition) FindTableIdx(predicate func(*pokertable.Table) bool) int {
	for idx, table := range c.State.Tables {
		if predicate(table) {
			return idx
		}
	}
	return UnsetValue
}

func (c Competition) FindPlayerIdx(predicate func(*CompetitionPlayer) bool) int {
	for idx, player := range c.State.Players {
		if predicate(player) {
			return idx
		}
	}
	return UnsetValue
}

func (c Competition) CurrentBlindLevel() BlindLevel {
	if c.State.BlindState.CurrentLevelIndex < 0 {
		fmt.Println("[DEBUG#ERROR] Invalid CurrentBlindLevel")
		return BlindLevel{}
	}
	return c.Meta.Blind.Levels[c.State.BlindState.CurrentLevelIndex]
}

func (c Competition) CurrentBlindData() (int, int64, int64, int64, int64) {
	bl := c.CurrentBlindLevel()
	dealer := int64(0)
	if c.Meta.Blind.DealerBlindTime > 0 {
		dealer = bl.Ante * (int64(c.Meta.Blind.DealerBlindTime) - 1)
	}
	return bl.Level, bl.Ante, dealer, bl.SB, bl.BB
}

func (c Competition) IsBreaking() bool {
	return c.CurrentBlindLevel().Level == -1
}

// BlindState Getters
func (bs BlindState) IsFinalBuyInLevel() bool {
	// 沒有預設 FinalBuyInLevelIndex 代表不能補碼，永遠都是停止買入階段
	if bs.FinalBuyInLevelIndex == UnsetValue {
		return true
	}

	return bs.CurrentLevelIndex >= bs.FinalBuyInLevelIndex
}
