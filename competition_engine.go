package pokercompetition

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/weedbox/pokercompetition/model"
	pokertablemodel "github.com/weedbox/pokertable/model"
	"github.com/weedbox/pokertable/util"
)

var (
	ErrTableNotFound    = errors.New("table not found")
	ErrNoRedeemChips    = errors.New("no redeem any chips")
	ErrExceedReBuyLimit = errors.New("exceed re-buy limit")
	ErrExceedAddonLimit = errors.New("exceed addon limit")
	ErrAddonRejected    = errors.New("not allowed to addon")
	ErrRefundRejected   = errors.New("not allowed to refund")
	ErrLeaveRejected    = errors.New("not allowed to leave")
)

// TODO: add distribute tables 拆併桌
type CompetitionEngine interface {
	// Competition Operations
	Create(CompetitionSetting) model.Competition                         // 建立賽事
	Start(model.Competition, []*pokertablemodel.Table) model.Competition // 開始賽事
	Settlement(model.Competition) model.Competition                      // 賽事結算

	// Table Operations
	AddTable(model.Competition, *pokertablemodel.Table) model.Competition            // 新增桌次
	CloseTable(model.Competition, *pokertablemodel.Table) (model.Competition, error) // 關閉桌次
	TableSettlement(model.Competition, pokertablemodel.Table) model.Competition      // 桌次結算

	// Player Operations
	PlayerJoin(model.Competition, JoinPlayer) (model.Competition, error)  // 玩家報名或補碼
	PlayerAddon(model.Competition, JoinPlayer) (model.Competition, error) // 玩家增購
	PlayerRefund(model.Competition, string) (model.Competition, error)    // 玩家退賽
	PlayerLeave(model.Competition, string) (model.Competition, error)     // 玩家離桌結算 (現金桌)
}

func NewCompetitionEngine() CompetitionEngine {
	return &competitionEngine{
		playerCacheData: make(map[string]*PlayerCache),
	}
}

type competitionEngine struct {
	// playerCacheData key: playerID, value: PlayerCache
	playerCacheData map[string]*PlayerCache
}

func (engine *competitionEngine) Create(setting CompetitionSetting) model.Competition {
	state := model.CompetitionState{
		OpenAt:    time.Now().Unix(),
		DisableAt: setting.DisableAt,
		StartAt:   setting.StartAt,
		EndAt:     util.UnsetValue,
		Players:   make([]*model.CompetitionPlayer, 0),
		Status:    model.CompetitionStateStatus_Registering,
		Tables:    make([]*pokertablemodel.Table, 0),
		Rankings:  make([]*model.CompetitionRank, 0),
	}

	return model.Competition{
		ID:       uuid.New().String(),
		Meta:     setting.Meta,
		State:    &state,
		UpdateAt: time.Now().Unix(),
	}
}

func (engine *competitionEngine) Start(competition model.Competition, tables []*pokertablemodel.Table) model.Competition {
	// Step 1: 更新開始賽事 & 結束賽事時間
	competition.State.StartAt = time.Now().Unix()
	if competition.Meta.Mode == model.CompetitionMode_CT {
		competition.State.EndAt = time.Now().Add(time.Minute * time.Duration(competition.Meta.MaxDurationMins)).Unix()
	}

	// Step 2: 桌次資料
	copy(competition.State.Tables, tables)
	// for idx, table := range tables {
	// 	competition.State.Tables[idx] = table
	// }

	return competition
}

/*
	Settlement 賽事結算
	- 適用時機:
	    - 賽事結束
	  - 邏輯:
	    - 更新賽事狀態
		- 計算玩家賽事排名
*/
func (engine *competitionEngine) Settlement(competition model.Competition) model.Competition {
	competition.State.Status = model.CompetitionStateStatus_End
	finalRankings := GetParticipatedPlayerCompetitionRankingData(engine.playerCacheData, competition.State.Players)
	for playerID, rankData := range finalRankings {
		rankIdx := rankData.Rank - 1
		competition.State.Rankings[rankIdx] = &model.CompetitionRank{
			PlayerID:   playerID,
			FinalChips: rankData.Chips,
		}
	}

	// clear cache
	engine.playerCacheData = make(map[string]*PlayerCache)
	return competition
}

/*
	AddTable 新增桌次
	  - 適用時機:
	    - CT/Cash: 開新桌
		- MTT: 拆併桌 (開新桌)
	  - 邏輯:
	    - 更新 State.Tables
		- 更新 State.Players
*/
func (engine *competitionEngine) AddTable(competition model.Competition, table *pokertablemodel.Table) model.Competition {
	// update tables
	competition.State.Tables = append(competition.State.Tables, table)

	// update players
	newPlayerData := make(map[string]int64)
	for _, ps := range table.State.PlayerStates {
		newPlayerData[ps.PlayerID] = ps.Bankroll
	}

	// find existing players
	existingPlayerData := make(map[string]int64)
	for _, player := range competition.State.Players {
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
		for i := 0; i < len(competition.State.Players); i++ {
			player := competition.State.Players[i]
			if bankroll, exist := existingPlayerData[player.PlayerID]; exist {
				competition.State.Players[i].Chips = bankroll
			}
		}
	}

	// add new player data
	if len(newPlayerData) > 0 {
		newPlayerIdx := len(competition.State.Players)
		newPlayers := make([]*model.CompetitionPlayer, 0)
		for playerID, bankroll := range newPlayerData {
			player, playerCache := NewDefaultCompetitionPlayerData(table.ID, playerID, bankroll)
			newPlayers = append(newPlayers, &player)

			playerCache.PlayerIdx = newPlayerIdx
			engine.playerCacheData[playerID] = &playerCache
			newPlayerIdx++

		}
		competition.State.Players = append(competition.State.Players, newPlayers...)
	}

	return competition
}

/*
	CloseTable 新增桌次
	  - 適用時機:
	    - CT/Cash: 某桌結束
		- MTT: 拆併桌 (關閉某桌)
	  - 邏輯:
	    - 更新 State.Tables
*/
func (engine *competitionEngine) CloseTable(competition model.Competition, table *pokertablemodel.Table) (model.Competition, error) {
	// validate deleting table
	deleteTableIdx := competition.FindTableIdx(func(t *pokertablemodel.Table) bool {
		return table.ID == t.ID
	})
	if deleteTableIdx == UnsetValue {
		return competition, ErrTableNotFound
	}

	// close table logic
	competition.DeleteTable(deleteTableIdx)

	return competition, nil
}

func (engine *competitionEngine) PlayerJoin(competition model.Competition, joinPlayer JoinPlayer) (model.Competition, error) {
	// validate join player data
	if joinPlayer.RedeemChips <= 0 {
		return competition, ErrNoRedeemChips
	}

	// update player data
	playerIdx := competition.FindPlayerIdx(func(player *model.CompetitionPlayer) bool {
		return player.PlayerID == joinPlayer.PlayerID
	})

	if playerIdx == UnsetValue {
		// BuyIn logic
		player, playerCache := NewDefaultCompetitionPlayerData(joinPlayer.TableID, joinPlayer.PlayerID, joinPlayer.RedeemChips)
		competition.State.Players = append(competition.State.Players, &player)
		playerCache.PlayerIdx = len(competition.State.Players) - 1
		engine.playerCacheData[joinPlayer.PlayerID] = &playerCache
	} else {
		// validate ReBuy times
		if competition.State.Players[playerIdx].ReBuyTimes >= competition.Meta.ReBuySetting.MaxTimes {
			return competition, ErrExceedReBuyLimit
		}

		// ReBuy logic
		competition.State.Players[playerIdx].CurrentTableID = joinPlayer.TableID
		competition.State.Players[playerIdx].Chips += joinPlayer.RedeemChips
		competition.State.Players[playerIdx].ReBuyTimes++
		competition.State.Players[playerIdx].IsReBuying = false
		competition.State.Players[playerIdx].ReBuyEndAt = UnsetValue

		engine.playerCacheData[joinPlayer.PlayerID].ReBuyTimes = competition.State.Players[playerIdx].ReBuyTimes
	}

	return competition, nil
}

/*
	TableSettlement 桌次結算
	- 適用時機:
	    - 每手結束
	  - 邏輯:
	    - 更新玩家狀態 (桌內即時排名 & 當前後手碼量)
		- 更新玩家狀態 (可補碼 Or 淘汰)
*/
func (engine *competitionEngine) TableSettlement(competition model.Competition, table pokertablemodel.Table) model.Competition {
	// Step 1: 更新玩家桌內即時排名 & 當前後手碼量(該手有參賽者會更新排名，若沒參賽者排名為 0)
	playerRankingData := GetParticipatedPlayerTableRankingData(engine.playerCacheData, table.State.PlayerStates, table.State.PlayingPlayerIndexes)
	for playerIdx := 0; playerIdx < len(competition.State.Players); playerIdx++ {
		player := competition.State.Players[playerIdx]
		if rankData, exist := playerRankingData[player.PlayerID]; exist {
			competition.State.Players[playerIdx].Rank = rankData.Rank
			competition.State.Players[playerIdx].Chips = rankData.Chips
		}
	}

	// Step 2: 更新玩家狀態
	if !table.State.BlindState.IsFinalBuyInLevel() {
		// 處理可補碼玩家
		reBuyEndAt := time.Now().Add(time.Second * time.Duration(competition.Meta.ReBuySetting.WaitingTimeInSec)).Unix()
		for _, player := range table.State.PlayerStates {
			playerData := engine.playerCacheData[player.PlayerID]
			if playerData.ReBuyTimes < competition.Meta.ReBuySetting.MaxTimes {
				competition.State.Players[playerData.PlayerIdx].IsReBuying = true
				competition.State.Players[playerData.PlayerIdx].ReBuyEndAt = reBuyEndAt
			}
		}
	}

	// 處理玩家淘汰
	competition = engine.HandleKnockoutPlayers(competition, table.State.PlayerStates, competition.Meta.ReBuySetting.MaxTimes)

	return competition
}

func (engine *competitionEngine) PlayerAddon(competition model.Competition, joinPlayer JoinPlayer) (model.Competition, error) {
	// validate join player data
	if joinPlayer.RedeemChips <= 0 {
		return competition, ErrNoRedeemChips
	}

	playerIdx := competition.FindPlayerIdx(func(player *model.CompetitionPlayer) bool {
		return player.PlayerID == joinPlayer.PlayerID
	})
	if playerIdx == UnsetValue {
		return competition, ErrAddonRejected
	}

	// addon logic
	// validate Addon times
	if competition.State.Players[playerIdx].AddonTimes >= competition.Meta.AddonSetting.MaxTimes {
		return competition, ErrExceedAddonLimit
	}

	competition.State.Players[playerIdx].CurrentTableID = joinPlayer.TableID
	competition.State.Players[playerIdx].Chips += joinPlayer.RedeemChips
	competition.State.Players[playerIdx].AddonTimes++

	return competition, nil
}

func (engine *competitionEngine) PlayerRefund(competition model.Competition, playerID string) (model.Competition, error) {
	// validate refund conditions
	playerIdx := competition.FindPlayerIdx(func(player *model.CompetitionPlayer) bool {
		return player.PlayerID == playerID
	})
	if playerIdx == UnsetValue {
		return competition, ErrRefundRejected
	}

	if competition.State.Status != model.CompetitionStateStatus_Registering {
		return competition, ErrRefundRejected
	}

	// refund logic
	competition.DeletePlayer(playerIdx)

	return competition, nil
}

func (engine *competitionEngine) PlayerLeave(competition model.Competition, playerID string) (model.Competition, error) {
	// validate leave conditions
	playerIdx := competition.FindPlayerIdx(func(player *model.CompetitionPlayer) bool {
		return player.PlayerID == playerID
	})
	if playerIdx == UnsetValue {
		return competition, ErrLeaveRejected
	}

	if competition.Meta.Mode != model.CompetitionMode_Cash {
		return competition, ErrLeaveRejected
	}

	// leave logic
	competition.DeletePlayer(playerIdx)

	return competition, nil
}

func (engine *competitionEngine) HandleKnockoutPlayers(competition model.Competition, tablePlayers []*pokertablemodel.TablePlayerState, maxReBuyTimes int) model.Competition {
	// 列出淘汰玩家
	knockoutPlayerRankings := GetSortedKnockoutPlayerRankings(engine.playerCacheData, tablePlayers, maxReBuyTimes)
	for knockoutPlayerIDIdx := len(knockoutPlayerRankings) - 1; knockoutPlayerIDIdx >= 0; knockoutPlayerIDIdx-- {
		knockPlayerID := knockoutPlayerRankings[knockoutPlayerIDIdx]

		// 更新賽事排名
		for rankIdx := len(competition.State.Rankings) - 1; rankIdx >= 0; rankIdx-- {
			if competition.State.Rankings[rankIdx] == nil {
				competition.State.Rankings[rankIdx] = &model.CompetitionRank{
					PlayerID:   knockPlayerID,
					FinalChips: 0,
				}
				break
			}
		}

		// 更新玩家狀態
		competition.State.Players[engine.playerCacheData[knockPlayerID].PlayerIdx].Status = model.CompetitionPlayerStatus_Knockout
	}

	return competition
}
