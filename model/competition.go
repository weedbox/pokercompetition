package model

import (
	"encoding/json"
	"time"

	"github.com/thoas/go-funk"
	pokertablemodel "github.com/weedbox/pokertable/model"
)

type CompetitionStateStatus string
type CompetitionPlayerStatus string
type CompetitionMode string
type CompetitionRule string

const (
	// CompetitionStateStatus
	CompetitionStateStatus_Uncreated     CompetitionStateStatus = "uncreated"     // 賽事尚未建立
	CompetitionStateStatus_Unregistering CompetitionStateStatus = "unregistering" // 賽事已建立 (但不可報名)
	CompetitionStateStatus_Registering   CompetitionStateStatus = "registering"   // 賽事已建立 (可報名參賽)
	CompetitionStateStatus_DelayedBuyin  CompetitionStateStatus = "delayed_buyin" // 賽事已建立 (延遲買入)
	CompetitionStateStatus_StoppedBuyin  CompetitionStateStatus = "stopped_buyin" // 賽事已建立 (停止買入)
	CompetitionStateStatus_End           CompetitionStateStatus = "end"           // 賽事已結束
	CompetitionStateStatus_Restoring     CompetitionStateStatus = "restoring"     // 賽事資料轉移中 (Graceful Shutdown Use)

	// CompetitionPlayerStatus
	CompetitionPlayerStatus_WaitingDistributeTables CompetitionPlayerStatus = "waiting_distribute_tables" // 等待拆併桌
	CompetitionPlayerStatus_Playing                 CompetitionPlayerStatus = "playing"                   // 比賽中
	CompetitionPlayerStatus_Knockout                CompetitionPlayerStatus = "knockout"                  // 已淘汰

	// CompetitionMode
	CompetitionMode_CT   = "ct"   // 倒數錦標賽
	CompetitionMode_MTT  = "mtt"  // 大型錦標賽
	CompetitionMode_Cash = "cash" // 現金桌

	// CompetitionRule
	CompetitionRule_Default   = "default"    // 常牌
	CompetitionRule_ShortDeck = "short_deck" // 短牌
	CompetitionRule_Omaha     = "omaha"      // 奧瑪哈
)

type Competition struct {
	ID       string            `json:"id"`        // 賽事 Unique ID
	Meta     CompetitionMeta   `json:"meta"`      // 賽事固定資料
	State    *CompetitionState `json:"state"`     // 賽事動態資料
	UpdateAt int64             `json:"update_at"` // 更新時間
}

type CompetitionMeta struct {
	Blind                Blind           `json:"blind"`                   // 盲注資訊
	Ticket               Ticket          `json:"ticket"`                  // 票券資訊
	Scene                string          `json:"scene"`                   // 場景
	MaxDurationMins      int             `json:"max_duration_mins"`       // 比賽時間總長 (分鐘)
	MinPlayerCount       int             `json:"min_player_count"`        // 最小參賽人數
	MaxPlayerCount       int             `json:"max_player_count"`        // 最大參賽人數
	Rule                 CompetitionRule `json:"rule"`                    // 德州撲克規則, 常牌(default), 短牌(short_deck), 奧瑪哈(omaha)
	Mode                 CompetitionMode `json:"mode"`                    // 賽事模式 (CT, MTT, Cash)
	BuyInSetting         BuyInSetting    `json:"buy_in_setting"`          // BuyIn 設定
	ReBuySetting         ReBuySetting    `json:"re_buy_setting"`          // ReBuy 設定
	AddonSetting         AddonSetting    `json:"addon_setting"`           // Addon 設定
	ActionTimeSecs       int             `json:"action_time_secs"`        // 思考時間 (秒數)
	TableMaxSeatCount    int             `json:"table_max_seat_count"`    // 每桌人數上限
	TableMinPlayingCount int             `json:"table_min_playing_count"` // 每桌最小開打數
	MinChipsUnit         int64           `json:"min_chips_unit"`          // 最小單位籌碼量
}

type CompetitionState struct {
	OpenAt    int64                    `json:"open_game_at"`    // 賽事開始時間 (可報名、尚未開打)
	DisableAt int64                    `json:"disable_game_at"` // 賽事未開打前，賽局可見時間
	StartAt   int64                    `json:"start_game_at"`   // 賽事開打時間 (可報名、開打)
	EndAt     int64                    `json:"end_game_at"`     // 賽事結束時間
	Players   []*CompetitionPlayer     `json:"players"`         // 參與過賽事玩家陣列
	Status    CompetitionStateStatus   `json:"status"`          // 賽事狀態
	Tables    []*pokertablemodel.Table `json:"tables"`          // 多桌
	Rankings  []*CompetitionRank       `json:"rankings"`        // 玩家排名 (陣列 Index 即是排名 rank - 1, ex: index 0 -> 第一名, index 1 -> 第二名...)
	// TODO: 停止買入後要依照目前賽事人數建立對應容量的 CompetitionRank 陣列
}

type CompetitionRank struct {
	PlayerID   string `json:"player_id"`   // 玩家 ID
	FinalChips int64  `json:"final_chips"` // 玩家最後籌碼數
}

type CompetitionPlayer struct {
	PlayerID       string `json:"player_id"` // 玩家 ID
	CurrentTableID string `json:"table_id"`  // 當前桌次 ID
	JoinAt         int64  `json:"join_at"`   // 加入時間

	// current info
	Status     CompetitionPlayerStatus `json:"status"`        // 參與玩家狀態
	Rank       int                     `json:"rank"`          // 當前桌次排名
	Chips      int64                   `json:"chips"`         // 當前籌碼
	IsReBuying bool                    `json:"is_re_buying"`  // 是否正在補碼
	ReBuyEndAt int64                   `json:"re_buy_end_at"` // 最後補碼時間
	ReBuyTimes int                     `json:"re_buy_times"`  // 補碼次數
	AddonTimes int                     `json:"addon_times"`   // 增購次數

	// statistics info
	// best
	BestWinningPotChips int64    `json:"best_winning_pot_chips"` // 贏得最大底池籌碼數
	BestWinningCombo    []string `json:"best_winning_combo"`     // 身為贏家時最大的牌型組合

	// accumulated info
	// competition/table
	TotalRedeemChips int64 `json:"total_redeem_chips"` // 累積兌換籌碼
	TotalGameCounts  int64 `json:"total_game_counts"`  // 總共玩幾手牌

	// game: round & actions
	TotalWalks            int64 `json:"total_walks"`              // Preflop 除了大盲以外的人全部 Fold，而贏得籌碼的次數
	TotalVPIPTimes        int   `json:"total_vpip_times"`         // 入池總次數
	TotalFoldTimes        int   `json:"total_fold_times"`         // 棄牌總次數
	TotalPreflopFoldTimes int   `json:"total_preflop_fold_times"` // Preflop 棄牌總次數
	TotalFlopFoldTimes    int   `json:"total_flop_fold_times"`    // Flop 棄牌總次數
	TotalTurnFoldTimes    int   `json:"total_turn_fold_times"`    // Turn 棄牌總次數
	TotalRiverFoldTimes   int   `json:"total_river_fold_times"`   // River 棄牌總次數
	TotalActionTimes      int   `json:"total_action_times"`       // 下注動作總次數
	TotalRaiseTimes       int   `json:"total_raise_times"`        // 加注/入池總次數(AllIn&Raise、Raise、Bet)
	TotalCallTimes        int   `json:"total_call_times"`         // 跟注總次數
	TotalCheckTimes       int   `json:"total_check_times"`        // 過牌總次數
}

type Ticket struct {
	ID   string `json:"id"`   // ID
	Name string `json:"name"` // 名稱
}

type Blind struct {
	ID               string       `json:"id"`                 // ID
	Name             string       `json:"name"`               // 名稱
	InitialLevel     int          `json:"initial_level"`      // 起始盲注級別
	FinalBuyInLevel  int          `json:"final_buy_in_level"` // 最後買入盲注等級
	DealerBlindTimes int          `json:"dealer_blind_times"` // Dealer 位置要收取的前注倍數 (短牌用)
	Levels           []BlindLevel `json:"levels"`             // 級別資訊列表
}

type BlindLevel struct {
	Level        int   `json:"level"`         // 盲注等級(-1 表示中場休息)
	SBChips      int64 `json:"sb_chips"`      // 小盲籌碼量
	BBChips      int64 `json:"bb_chips"`      // 大盲籌碼量
	AnteChips    int64 `json:"ante_chips"`    // 前注籌碼量
	DurationMins int   `json:"duration_mins"` // 等級持續時間
}

type BuyInSetting struct {
	IsFree     bool `json:"is_free"`     // 是否免費參賽
	MinTickets int  `json:"min_tickets"` // 最小票數
	MaxTickets int  `json:"max_tickets"` // 最大票數
}

type ReBuySetting struct {
	MinTicket        int `json:"min_ticket"`          // 最小票數
	MaxTicket        int `json:"max_ticket"`          // 最大票數
	MaxTimes         int `json:"max_times"`           // 最大次數
	WaitingTimeInSec int `json:"waiting_time_in_sec"` // 玩家可補碼時間
}

type AddonSetting struct {
	IsBreakOnly bool    `json:"is_break_only"` // 是否中場休息限定
	RedeemChips []int64 `json:"redeem_chips"`  // 可兌換籌碼數
	MaxTimes    int     `json:"max_times"`     // 最大次數
}

// Competition Setters
func (competition *Competition) Start() {
	competition.State.Status = CompetitionStateStatus_DelayedBuyin
	competition.State.StartAt = time.Now().Unix()

	if competition.Meta.Mode == CompetitionMode_CT {
		competition.State.EndAt = competition.State.StartAt + int64((time.Duration(competition.Meta.MaxDurationMins) * time.Minute).Seconds())
	}
}

func (competition *Competition) DeleteTable(targetIdx int) {
	competition.State.Tables = append(competition.State.Tables[:targetIdx], competition.State.Tables[targetIdx+1:]...)
}

func (competition *Competition) DeletePlayer(targetIdx int) {
	competition.State.Players = append(competition.State.Players[:targetIdx], competition.State.Players[targetIdx+1:]...)
}

func (competition *Competition) FindTableIdx(predicate func(*pokertablemodel.Table) bool) int {
	for idx, table := range competition.State.Tables {
		if predicate(table) {
			return idx
		}
	}
	return -1
}

func (competition *Competition) FindPlayerIdx(predicate func(*CompetitionPlayer) bool) int {
	for idx, player := range competition.State.Players {
		if predicate(player) {
			return idx
		}
	}
	return -1
}

// Competition Getters
func (competition Competition) GetJSON() (*string, error) {
	encoded, err := json.Marshal(competition)
	if err != nil {
		return nil, err
	}
	json := string(encoded)
	return &json, nil
}

func (competition Competition) CanStart() bool {
	currentPlayerCount := 0
	for _, table := range competition.State.Tables {
		currentPlayerCount += len(table.State.PlayerStates)
	}

	if currentPlayerCount >= competition.Meta.MinPlayerCount {
		// 開打條件一: 當賽局已經設定 StartAt (開打時間) & 現在時間已經大於等於開打時間且達到最小開桌人數
		if competition.State.StartAt > 0 && time.Now().Unix() >= competition.State.StartAt {
			return true
		}

		// 開打條件二: 賽局沒有設定 StartAt (開打時間) & 達到最小開桌人數
		if competition.State.StartAt == 0 {
			return true
		}
	}
	return false
}

func (competition Competition) PlayingPlayerCount() int {
	return len(funk.Filter(competition.State.Players, func(player *CompetitionPlayer) bool {
		return (player.Status != CompetitionPlayerStatus_Knockout)
	}).([]*CompetitionPlayer))
}
