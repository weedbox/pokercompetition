package pokercompetition

import (
	"encoding/json"
	"fmt"

	"github.com/thoas/go-funk"
	"github.com/weedbox/pokertable"
)

type CompetitionStateStatus string
type CompetitionPlayerStatus string
type CompetitionMode string
type CompetitionRule string
type CompetitionAdvanceRule string
type CompetitionAdvanceStatus string

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
	CompetitionPlayerStatus_CashLeaving           CompetitionPlayerStatus = "cash_leaving"            // 現金桌離開中 (結算時就會離開)

	// CompetitionMode
	CompetitionMode_CT   CompetitionMode = "ct"   // 倒數錦標賽
	CompetitionMode_MTT  CompetitionMode = "mtt"  // 大型錦標賽
	CompetitionMode_Cash CompetitionMode = "cash" // 現金桌

	// CompetitionRule
	CompetitionRule_Default   CompetitionRule = "default"    // 常牌
	CompetitionRule_ShortDeck CompetitionRule = "short_deck" // 短牌
	CompetitionRule_Omaha     CompetitionRule = "omaha"      // 奧瑪哈

	// CompetitionAdvanceRule
	CompetitionAdvanceRule_PlayerCount CompetitionAdvanceRule = "player_count" // 晉級方式: 尚未淘汰玩家人數
	CompetitionAdvanceRule_BlindLevel  CompetitionAdvanceRule = "blind_level"  // 晉級方式: 盲注等級

	// CompetitionAdvanceStatus
	CompetitionAdvanceStatus_NotStart CompetitionAdvanceStatus = "adv_not_start" // 晉級狀態: 未開始
	CompetitionAdvanceStatus_Updating CompetitionAdvanceStatus = "adv_updating"  // 晉級狀態: 晉級計算中
	CompetitionAdvanceStatus_End      CompetitionAdvanceStatus = "adv_end"       // 晉級狀態: 已結束
)

type Competition struct {
	UpdateSerial int64             `json:"update_serial"` // 更新序列號 (數字越大越晚發生)
	ID           string            `json:"id"`            // 賽事 Unique ID
	Meta         CompetitionMeta   `json:"meta"`          // 賽事固定資料
	State        *CompetitionState `json:"state"`         // 賽事動態資料
	UpdateAt     int64             `json:"update_at"`     // 更新時間 (Seconds)
}

type CompetitionMeta struct {
	Blind                          Blind           `json:"blind"`                              // 盲注資訊
	MaxDuration                    int             `json:"max_duration"`                       // 比賽時間總長 (Seconds)
	MinPlayerCount                 int             `json:"min_player_count"`                   // 最小參賽人數
	MaxPlayerCount                 int             `json:"max_player_count"`                   // 最大參賽人數
	TableMaxSeatCount              int             `json:"table_max_seat_count"`               // 每桌人數上限
	TableMinPlayerCount            int             `json:"table_min_player_count"`             // 每桌最小開打數
	RegulatorMinInitialPlayerCount int             `json:"regulator_min_initial_player_count"` // 拆併桌每桌至少需要人數
	Rule                           CompetitionRule `json:"rule"`                               // 德州撲克規則, 常牌(default), 短牌(short_deck), 奧瑪哈(omaha)
	Mode                           CompetitionMode `json:"mode"`                               // 賽事模式 (CT, MTT, Cash)
	ReBuySetting                   ReBuySetting    `json:"re_buy_setting"`                     // 補碼設定
	AddonSetting                   AddonSetting    `json:"addon_setting"`                      // 增購設定
	AdvanceSetting                 AdvanceSetting  `json:"advance_setting"`                    // 晉級設定
	ActionTime                     int             `json:"action_time"`                        // 思考時間 (Seconds)
	MinChipUnit                    int64           `json:"min_chip_unit"`                      // 最小單位籌碼量
}

type CompetitionState struct {
	OpenAt       int64                  `json:"open_at"`       // 賽事建立時間 (可報名、尚未開打)
	DisableAt    int64                  `json:"disable_at"`    // 賽事未開打前，賽局可見時間 (Seconds)
	StartAt      int64                  `json:"start_at"`      // 賽事開打時間 (可報名、開打) (Seconds)
	EndAt        int64                  `json:"end_at"`        // 賽事結束時間 (Seconds)
	BlindState   *BlindState            `json:"blind_state"`   // 盲注狀態
	Players      []*CompetitionPlayer   `json:"players"`       // 參與過賽事玩家陣列
	Status       CompetitionStateStatus `json:"status"`        // 賽事狀態
	Tables       []*pokertable.Table    `json:"tables"`        // 多桌
	Rankings     []*CompetitionRank     `json:"rankings"`      // 停止買入後玩家排名 (陣列 Index 即是排名 rank - 1, ex: index 0 -> 第一名, index 1 -> 第二名...)
	AdvanceState *AdvanceState          `json:"advance_state"` // 晉級狀態
	Statistic    *Statistic             `json:"statistic"`     // 賽事統計資料
}

type CompetitionRank struct {
	PlayerID   string `json:"player_id"`   // 玩家 ID
	FinalChips int64  `json:"final_chips"` // 玩家最後籌碼數
}

type CompetitionPlayer struct {
	PlayerID       string `json:"player_id"` // 玩家 ID
	CurrentTableID string `json:"table_id"`  // 當前桌次 ID
	CurrentSeat    int    `json:"seat"`      // 當前座位
	JoinAt         int64  `json:"join_at"`   // 加入時間 (Seconds)

	// current info
	Status     CompetitionPlayerStatus `json:"status"`        // 參與玩家狀態
	Rank       int                     `json:"rank"`          // 當前桌次排名
	Chips      int64                   `json:"chips"`         // 當前籌碼
	IsReBuying bool                    `json:"is_re_buying"`  // 是否正在補碼
	ReBuyEndAt int64                   `json:"re_buy_end_at"` // 最後補碼時間 (Seconds)
	ReBuyTimes int                     `json:"re_buy_times"`  // 補碼次數
	AddonTimes int                     `json:"addon_times"`   // 增購次數

	// statistics info
	// best
	BestWinningPotChips int64    `json:"best_winning_pot_chips"` // 贏得最大底池籌碼數
	BestWinningCombo    []string `json:"best_winning_combo"`     // 身為贏家時最大的牌型組合
	BestWinningType     string   `json:"best_winning_type"`      // 身為贏家時最大的牌型類型
	BestWinningPower    int      `json:"best_winning_power"`     // 身為贏家時最大的牌型牌力

	// accumulated info
	// competition/table
	TotalRedeemChips int64 `json:"total_redeem_chips"` // 累積兌換籌碼
	TotalGameCounts  int64 `json:"total_game_counts"`  // 總共玩幾手牌

	// game: round & actions
	TotalWalkTimes        int64 `json:"total_walk_times"`         // Preflop 除了大盲以外的人全部 Fold，而贏得籌碼的次數
	TotalFoldTimes        int   `json:"total_fold_times"`         // 棄牌總次數
	TotalPreflopFoldTimes int   `json:"total_preflop_fold_times"` // Preflop 棄牌總次數
	TotalFlopFoldTimes    int   `json:"total_flop_fold_times"`    // Flop 棄牌總次數
	TotalTurnFoldTimes    int   `json:"total_turn_fold_times"`    // Turn 棄牌總次數
	TotalRiverFoldTimes   int   `json:"total_river_fold_times"`   // River 棄牌總次數
	TotalActionTimes      int   `json:"total_action_times"`       // 下注動作總次數
	TotalRaiseTimes       int   `json:"total_raise_times"`        // 加注/入池總次數(AllIn&Raise、Raise、Bet)
	TotalCallTimes        int   `json:"total_call_times"`         // 跟注總次數
	TotalCheckTimes       int   `json:"total_check_times"`        // 過牌總次數
	TotalProfitTimes      int   `json:"total_profit_times"`       // 總共贏得籌碼次數

	// game: statistics
	TotalVPIPChances            int `json:"total_vpip_chances"`             // 入池總機會
	TotalVPIPTimes              int `json:"total_vpip_times"`               // 入池總次數
	TotalPFRChances             int `json:"total_pfr_chances"`              // PFR 總機會
	TotalPFRTimes               int `json:"total_pfr_times"`                // PFR 總次數
	TotalATSChances             int `json:"total_ats_chances"`              // ATS 總機會
	TotalATSTimes               int `json:"total_ats_times"`                // ATS 總次數
	Total3BChances              int `json:"total_3b_chances"`               // 3-Bet 總機會
	Total3BTimes                int `json:"total_3b_times"`                 // 3-Bet 總次數
	TotalFt3BChances            int `json:"total_ft3b_chances"`             // Ft3B 總機會
	TotalFt3BTimes              int `json:"total_ft3b_times"`               // Ft3B 總次數
	TotalCheckRaiseChances      int `json:"total_check_raise_chances"`      // C/R 總機會
	TotalCheckRaiseTimes        int `json:"total_check_raise_times"`        // C/R 總次數
	TotalCBetChances            int `json:"total_c_bet_chances"`            // C-Bet 總機會
	TotalCBetTimes              int `json:"total_c_bet_times"`              // C-Bet 總次數
	TotalFtCBChances            int `json:"total_ftcb_chances"`             // FtCB 總機會
	TotalFtCBTimes              int `json:"total_ftcb_times"`               // FtCB 總次數
	TotalShowdownWinningChances int `json:"total_showdown_winning_chances"` // Showdown Winning 總機會
	TotalShowdownWinningTimes   int `json:"total_showdown_winning_times"`   // Showdown Winning 總次數
}

type Blind struct {
	ID                   string       `json:"id"`                     // ID
	InitialLevel         int          `json:"initial_level"`          // 起始盲注級別
	FinalBuyInLevelIndex int          `json:"final_buy_in_level_idx"` // 最後買入盲注等級索引值
	DealerBlindTime      int          `json:"dealer_blind_time"`      // Dealer 位置要收取的前注倍數 (短牌用)
	Levels               []BlindLevel `json:"levels"`                 // 級別資訊列表
}

type BlindLevel struct {
	Level      int   `json:"level"`       // 盲注等級(-1 表示中場休息)
	SB         int64 `json:"sb"`          // 小盲籌碼量
	BB         int64 `json:"bb"`          // 大盲籌碼量
	Ante       int64 `json:"ante"`        // 前注籌碼量
	Duration   int   `json:"duration"`    // 等級持續時間 (Seconds)
	AllowAddon bool  `json:"allow_addon"` // 是否允許增購
}

type BlindState struct {
	FinalBuyInLevelIndex int     `json:"final_buy_in_level_idx"` // 最後買入盲注等級索引值
	CurrentLevelIndex    int     `json:"current_level_index"`    // 現在盲注等級級別索引值
	EndAts               []int64 `json:"end_ats"`                // 每個等級結束時間 (Seconds)
}

type AdvanceState struct {
	Status          CompetitionAdvanceStatus `json:"status"`            // 晉級狀態
	TotalTables     int                      `json:"total_tables"`      // 總桌數
	UpdatedTables   int                      `json:"updated_tables"`    // 已更新桌數
	UpdatedTableIDs []string                 `json:"updated_table_ids"` // 已更新桌次 ID
}

type Statistic struct {
	TotalBuyInCount int `json:"total_buy_in_count"` // 總買入次數
}

type ReBuySetting struct {
	MaxTime     int `json:"max_time"`     // 最大次數
	WaitingTime int `json:"waiting_time"` // 玩家可補碼時間 (Seconds)
}

type AddonSetting struct {
	IsBreakOnly bool    `json:"is_break_only"` // 是否中場休息限定
	RedeemChips []int64 `json:"redeem_chips"`  // 可兌換籌碼數
	MaxTime     int     `json:"max_time"`      // 最大次數
}

type AdvanceSetting struct {
	Rule        CompetitionAdvanceRule `json:"rule"`         // 晉級方式
	PlayerCount int                    `json:"player_count"` // 晉級人數
	BlindLevel  int                    `json:"blind_level"`  // 晉級盲注級別
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

func (c Competition) PlayingPlayerCount() int {
	return len(funk.Filter(c.State.Players, func(player *CompetitionPlayer) bool {
		return player.Chips > 0
	}).([]*CompetitionPlayer))
}

func (c Competition) IsTableExist(tableID string) bool {
	for _, table := range c.State.Tables {
		if table.ID == tableID {
			return true
		}
	}
	return false
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
func (bs BlindState) IsStopBuyIn() bool {
	// NoStopBuyInIndex 代表永遠都是延遲買入階段
	if bs.FinalBuyInLevelIndex == NoStopBuyInIndex {
		return false
	}

	// 沒有預設 FinalBuyInLevelIndex 代表不能補碼，永遠都是停止買入階段
	if bs.FinalBuyInLevelIndex == UnsetValue {
		return true
	}

	// 當前盲注等級索引值大於 FinalBuyInLevelIndex 代表停止買入階段
	return bs.CurrentLevelIndex > bs.FinalBuyInLevelIndex
}

// CompetitionPlayer Getters
func (cp CompetitionPlayer) IsOverReBuyWaitingTime() bool {
	if !cp.IsReBuying && cp.ReBuyEndAt == UnsetValue {
		return true
	}
	return false
}
