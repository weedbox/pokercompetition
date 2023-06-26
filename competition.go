package pokercompetition

import (
	"encoding/json"
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
	CompetitionStateStatus_Uncreated     CompetitionStateStatus = "uncreated"     // 賽事尚未建立
	CompetitionStateStatus_Unregistering CompetitionStateStatus = "unregistering" // 賽事已建立 (但不可報名)
	CompetitionStateStatus_Registering   CompetitionStateStatus = "registering"   // 賽事已建立 (可報名參賽)
	CompetitionStateStatus_DelayedBuyin  CompetitionStateStatus = "delayed_buyin" // 賽事已建立 (延遲買入)
	CompetitionStateStatus_StoppedBuyin  CompetitionStateStatus = "stopped_buyin" // 賽事已建立 (停止買入)
	CompetitionStateStatus_End           CompetitionStateStatus = "end"           // 賽事已結束
	CompetitionStateStatus_Restoring     CompetitionStateStatus = "restoring"     // 賽事資料轉移中 (Graceful Shutdown Use)

	// CompetitionPlayerStatus
	CompetitionPlayerStatus_WaitingTableBalancing CompetitionPlayerStatus = "waiting_table_balancing" // 等待拆併桌中
	CompetitionPlayerStatus_Playing               CompetitionPlayerStatus = "playing"                 // 比賽中
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
	ID           string            `json:"id"`            // 賽事 Unique ID
	Meta         CompetitionMeta   `json:"meta"`          // 賽事固定資料
	State        *CompetitionState `json:"state"`         // 賽事動態資料
	UpdateAt     int64             `json:"update_at"`     // 更新時間 (Seconds)
	UpdateSerial int64             `json:"update_serial"` // 更新序列號 (數字越大越晚發生)
}

type CompetitionMeta struct {
	Blind               Blind           `json:"blind"`                  // 盲注資訊
	Ticket              Ticket          `json:"ticket"`                 // 票券資訊
	Scene               string          `json:"scene"`                  // 場景
	MaxDuration         int             `json:"max_duration"`           // 比賽時間總長 (Seconds)
	MinPlayerCount      int             `json:"min_player_count"`       // 最小參賽人數
	MaxPlayerCount      int             `json:"max_player_count"`       // 最大參賽人數
	TableMaxSeatCount   int             `json:"table_max_seat_count"`   // 每桌人數上限
	TableMinPlayerCount int             `json:"table_min_player_count"` // 每桌最小開打數
	Rule                CompetitionRule `json:"rule"`                   // 德州撲克規則, 常牌(default), 短牌(short_deck), 奧瑪哈(omaha)
	Mode                CompetitionMode `json:"mode"`                   // 賽事模式 (CT, MTT, Cash)
	BuyInSetting        BuyInSetting    `json:"buy_in_setting"`         // BuyIn 設定
	ReBuySetting        ReBuySetting    `json:"re_buy_setting"`         // ReBuy 設定
	AddonSetting        AddonSetting    `json:"addon_setting"`          // Addon 設定
	ActionTime          int             `json:"action_time"`            // 思考時間 (Seconds)
	MinChipUnit         int64           `json:"min_chip_unit"`          // 最小單位籌碼量
}

type CompetitionState struct {
	OpenAt    int64                  `json:"open_at"`    // 賽事建立時間 (可報名、尚未開打)
	DisableAt int64                  `json:"disable_at"` // 賽事未開打前，賽局可見時間 (Seconds)
	StartAt   int64                  `json:"start_at"`   // 賽事開打時間 (可報名、開打) (Seconds)
	EndAt     int64                  `json:"end_at"`     // 賽事結束時間 (Seconds)
	Players   []*CompetitionPlayer   `json:"players"`    // 參與過賽事玩家陣列
	Status    CompetitionStateStatus `json:"status"`     // 賽事狀態
	Tables    []*pokertable.Table    `json:"tables"`     // 多桌
	Rankings  []*CompetitionRank     `json:"rankings"`   // 玩家排名 (陣列 Index 即是排名 rank - 1, ex: index 0 -> 第一名, index 1 -> 第二名...)
}

type CompetitionRank struct {
	PlayerID   string `json:"player_id"`   // 玩家 ID
	FinalChips int64  `json:"final_chips"` // 玩家最後籌碼數
}

type CompetitionPlayer struct {
	PlayerID       string `json:"player_id"` // 玩家 ID
	CurrentTableID string `json:"table_id"`  // 當前桌次 ID
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

	// accumulated info
	// competition/table
	TotalRedeemChips int64 `json:"total_redeem_chips"` // 累積兌換籌碼
	TotalGameCounts  int64 `json:"total_game_counts"`  // 總共玩幾手牌

	// game: round & actions
	TotalWalkTimes        int64 `json:"total_walk_times"`         // Preflop 除了大盲以外的人全部 Fold，而贏得籌碼的次數
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
	ID              string       `json:"id"`                 // ID
	Name            string       `json:"name"`               // 名稱
	InitialLevel    int          `json:"initial_level"`      // 起始盲注級別
	FinalBuyInLevel int          `json:"final_buy_in_level"` // 最後買入盲注等級
	DealerBlindTime int          `json:"dealer_blind_time"`  // Dealer 位置要收取的前注倍數 (短牌用)
	Levels          []BlindLevel `json:"levels"`             // 級別資訊列表
}

type BlindLevel struct {
	Level    int   `json:"level"`    // 盲注等級(-1 表示中場休息)
	SB       int64 `json:"sb"`       // 小盲籌碼量
	BB       int64 `json:"bb"`       // 大盲籌碼量
	Ante     int64 `json:"ante"`     // 前注籌碼量
	Duration int   `json:"duration"` // 等級持續時間 (Seconds)
}

type BuyInSetting struct {
	IsFree    bool `json:"is_free"`    // 是否免費參賽
	MinTicket int  `json:"min_ticket"` // 最小票數
	MaxTicket int  `json:"max_ticket"` // 最大票數
}

type ReBuySetting struct {
	MinTicket   int `json:"min_ticket"`   // 最小票數
	MaxTicket   int `json:"max_ticket"`   // 最大票數
	MaxTime     int `json:"max_time"`     // 最大次數
	WaitingTime int `json:"waiting_time"` // 玩家可補碼時間 (Seconds)
}

type AddonSetting struct {
	IsBreakOnly bool    `json:"is_break_only"` // 是否中場休息限定
	RedeemChips []int64 `json:"redeem_chips"`  // 可兌換籌碼數
	MaxTime     int     `json:"max_time"`      // 最大次數
}

// Competition Setters
func (c *Competition) RefreshUpdateAt() {
	c.UpdateAt = time.Now().Unix()
	c.UpdateSerial++
}

func (c *Competition) ConfigureWithSetting(setting CompetitionSetting) {
	// configure meta
	c.Meta = setting.Meta

	// configure state
	state := CompetitionState{
		OpenAt:    time.Now().Unix(),
		DisableAt: setting.DisableAt,
		StartAt:   setting.StartAt,
		EndAt:     UnsetValue,
		Players:   make([]*CompetitionPlayer, 0),
		Status:    CompetitionStateStatus_Registering,
		Tables:    make([]*pokertable.Table, 0),
		Rankings:  make([]*CompetitionRank, 0),
	}
	c.State = &state
}

func (c *Competition) Close() {
	if c.Meta.Mode == CompetitionMode_MTT {
		c.State.EndAt = time.Now().Unix()
	}
	c.State.Status = CompetitionStateStatus_End
}

func (c *Competition) AddTable(table *pokertable.Table, playerCacheHandler func(PlayerCache)) {
	// update tables
	c.State.Tables = append(c.State.Tables, table)

	// update players
	newPlayerData := make(map[string]int64)
	for _, ps := range table.State.PlayerStates {
		newPlayerData[ps.PlayerID] = ps.Bankroll
	}

	// find existing players
	existingPlayerData := make(map[string]int64)
	for _, player := range c.State.Players {
		if bankroll, exist := newPlayerData[player.PlayerID]; exist {
			existingPlayerData[player.PlayerID] = bankroll
		}
	}

	// remove existing player data from new player data
	for playerID := range existingPlayerData {
		delete(newPlayerData, playerID)
	}

	// update existing player data
	if len(existingPlayerData) > 0 {
		for i := 0; i < len(c.State.Players); i++ {
			player := c.State.Players[i]
			if bankroll, exist := existingPlayerData[player.PlayerID]; exist {
				c.State.Players[i].Chips = bankroll
				c.State.Players[i].Status = CompetitionPlayerStatus_Playing
			}
		}
	}

	// add new player data
	if len(newPlayerData) > 0 {
		newPlayerIdx := len(c.State.Players)
		newPlayers := make([]*CompetitionPlayer, 0)
		for playerID, bankroll := range newPlayerData {
			player, playerCache := newDefaultCompetitionPlayerData(table.ID, playerID, bankroll)
			newPlayers = append(newPlayers, &player)
			playerCache.PlayerIdx = newPlayerIdx
			playerCacheHandler(playerCache)
			newPlayerIdx++

		}
		c.State.Players = append(c.State.Players, newPlayers...)
	}
}

func (c *Competition) Start() {
	if c.Meta.Blind.FinalBuyInLevel <= 0 {
		c.State.Status = CompetitionStateStatus_StoppedBuyin
	} else {
		c.State.Status = CompetitionStateStatus_DelayedBuyin
	}

	if c.Meta.Mode == CompetitionMode_CT {
		c.State.StartAt = time.Now().Unix()
		c.State.EndAt = c.State.StartAt + int64((time.Duration(c.Meta.MaxDuration) * time.Minute).Seconds())
	}
}

func (c *Competition) DeleteTable(targetIdx int) {
	c.State.Tables = append(c.State.Tables[:targetIdx], c.State.Tables[targetIdx+1:]...)
}

func (c *Competition) DeletePlayer(targetIdx int) {
	c.State.Players = append(c.State.Players[:targetIdx], c.State.Players[targetIdx+1:]...)
}

func (c *Competition) PlayerJoin(tableID, playerID string, playerIdx int, redeemChips int64, isBuyIn bool, buyInPlayerCacheHandler func(PlayerCache), reBuyPlayerCacheHandler func(int)) {
	if isBuyIn {
		player, playerCache := newDefaultCompetitionPlayerData(tableID, playerID, redeemChips)
		if c.Meta.Mode == CompetitionMode_MTT && c.State.Status == CompetitionStateStatus_Registering {
			// MTT 且尚未開賽: 玩家狀態為等待拆併桌中
			player.Status = CompetitionPlayerStatus_WaitingTableBalancing
		}
		c.State.Players = append(c.State.Players, &player)
		playerCache.PlayerIdx = len(c.State.Players) - 1
		buyInPlayerCacheHandler(playerCache)
	} else {
		// 若是 IsReBuying 則更新相關數據，否則是拆併桌移動到新桌
		c.State.Players[playerIdx].CurrentTableID = tableID
		if c.State.Players[playerIdx].IsReBuying {
			// ReBuy logic
			c.State.Players[playerIdx].Chips = redeemChips
			c.State.Players[playerIdx].ReBuyTimes++
			c.State.Players[playerIdx].IsReBuying = false
			c.State.Players[playerIdx].ReBuyEndAt = UnsetValue
			reBuyPlayerCacheHandler(c.State.Players[playerIdx].ReBuyTimes)
		}
	}
}

func (c *Competition) PlayerAddon(tableID, playerID string, playerIdx int, redeemChips int64) {
	c.State.Players[playerIdx].CurrentTableID = tableID
	c.State.Players[playerIdx].Chips += redeemChips
	c.State.Players[playerIdx].AddonTimes++
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
		currentPlayerCount += len(table.State.PlayerStates)
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
		return (player.Status != CompetitionPlayerStatus_Knockout)
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
