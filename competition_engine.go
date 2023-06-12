package pokercompetition

import (
	"errors"
	"time"

	"github.com/weedbox/pokerface"

	"github.com/thoas/go-funk"

	"github.com/google/uuid"
	"github.com/weedbox/pokertable"
)

var (
	ErrCompetitionNotFound             = errors.New("competition not found")
	ErrInvalidCreateCompetitionSetting = errors.New("invalid create competition setting")
	ErrTableNotFound                   = errors.New("table not found")
	ErrPlayerNotFound                  = errors.New("player not found")
	ErrNoRedeemChips                   = errors.New("no redeem any chips")
	ErrExceedReBuyLimit                = errors.New("exceed re-buy limit")
	ErrExceedAddonLimit                = errors.New("exceed addon limit")
	ErrAddonRejected                   = errors.New("not allowed to addon")
	ErrReBuyRejected                   = errors.New("not allowed to re-buy")
	ErrBuyInRejected                   = errors.New("not allowed to buy in")
	ErrRefundRejected                  = errors.New("not allowed to refund")
	ErrLeaveRejected                   = errors.New("not allowed to leave")
)

// TODO: add distribute tables 拆併桌
type CompetitionEngine interface {
	// Competition Actions
	CreateCompetition(competitionSetting CompetitionSetting) (*Competition, error) // 建立賽事
	CloseCompetition(competitionID string) error                                   // 關閉賽事
	StartCompetition(competitionID string) error                                   // 開始賽事

	// Table Operations
	AddTable(competitionID string, tableSetting TableSetting) error // 新增桌次
	CloseTable(competitionID, tableID string) error                 // 關閉桌次

	// Player Operations
	PlayerJoin(competitionID, tableID string, joinPlayer JoinPlayer) error  // 玩家報名或補碼
	PlayerAddon(competitionID, tableID string, joinPlayer JoinPlayer) error // 玩家增購
	PlayerRefund(competitionID, playerID string) error                      // 玩家退賽
	PlayerLeave(competitionID, tableID, playerID string) error              // 玩家離桌結算 (現金桌)
}

func NewCompetitionEngine() CompetitionEngine {
	tableEngine := pokertable.NewTableEngine()
	ce := &competitionEngine{
		tableEngine:     tableEngine,
		competitionMap:  make(map[string]*Competition),
		playerCacheData: make(map[string]*PlayerCache),
	}
	ce.tableEngine.OnTableUpdated(ce.onCompetitionTableUpdated)
	return ce
}

type competitionEngine struct {
	tableEngine          pokertable.TableEngine
	competitionMap       map[string]*Competition
	onCompetitionUpdated func(*Competition)

	// playerCacheData key: playerID, value: PlayerCache
	playerCacheData map[string]*PlayerCache
}

func (ce *competitionEngine) EmitEvent(competition *Competition) {
	competition.RefreshUpdateAt()
	ce.onCompetitionUpdated(competition)
}

func (ce *competitionEngine) OnCompetitionUpdated(fn func(*Competition)) error {
	ce.onCompetitionUpdated = fn
	return nil
}

func (ce *competitionEngine) GetCompetition(competitionID string) (*Competition, error) {
	competition, exist := ce.competitionMap[competitionID]
	if !exist {
		return nil, ErrCompetitionNotFound
	}
	return competition, nil
}

func (ce *competitionEngine) CreateCompetition(competitionSetting CompetitionSetting) (*Competition, error) {
	// validate competitionSetting
	for _, tableSetting := range competitionSetting.TableSettings {
		if len(tableSetting.JoinPlayers) > competitionSetting.Meta.TableMaxSeatCount {
			return nil, ErrInvalidCreateCompetitionSetting
		}
	}

	// create competition instance
	competition := &Competition{
		ID:       uuid.New().String(),
		UpdateAt: time.Now().Unix(),
	}
	competition.ConfigureWithSetting(competitionSetting)
	for _, tableSetting := range competitionSetting.TableSettings {
		if err := ce.addCompetitionTable(competition, tableSetting); err != nil {
			return nil, err
		}

		// TODO: consider AutoEndTable: close table but don't do any settlement
	}

	ce.EmitEvent(competition)

	// update competitionMap
	ce.competitionMap[competition.ID] = competition

	return competition, nil
}

/*
	CloseCompetition 關閉賽事
	  - 適用時機:
	    - 賽事出狀況需要臨時關閉賽事
*/
func (ce *competitionEngine) CloseCompetition(competitionID string) error {
	competition, exist := ce.competitionMap[competitionID]
	if !exist {
		return ErrCompetitionNotFound
	}

	ce.settleCompetition(competition)

	return nil
}

/*
	AddTable 新增桌次
	  - 適用時機:
	    - CT/Cash: 開新桌
		- MTT: 拆併桌 (開新桌)
*/
func (ce *competitionEngine) AddTable(competitionID string, tableSetting TableSetting) error {
	competition, exist := ce.competitionMap[competitionID]
	if !exist {
		return ErrCompetitionNotFound
	}

	if err := ce.addCompetitionTable(competition, tableSetting); err != nil {
		return err
	}

	ce.EmitEvent(competition)
	return nil
}

/*
	CloseTable 關閉桌次
	  - 適用時機:
	    - CT/Cash: 某桌結束
		- MTT: 拆併桌 (關閉某桌)
*/
func (ce *competitionEngine) CloseTable(competitionID, tableID string) error {
	competition, exist := ce.competitionMap[competitionID]
	if !exist {
		return ErrCompetitionNotFound
	}

	if err := ce.closeCompetitionTable(competition, tableID, pokertable.TableStateStatus_TableGameClosed); err != nil {
		return err
	}

	ce.EmitEvent(competition)
	return nil
}

/*
	StartCompetition 開賽
	  - 適用時機:
		- MTT: 手動開賽
*/
func (ce *competitionEngine) StartCompetition(competitionID string) error {
	competition, exist := ce.competitionMap[competitionID]
	if !exist {
		return ErrCompetitionNotFound
	}

	competition.Start()

	// TODO: 啟動拆併桌

	ce.EmitEvent(competition)
	return nil
}

func (ce *competitionEngine) PlayerJoin(competitionID, tableID string, joinPlayer JoinPlayer) error {
	competition, exist := ce.competitionMap[competitionID]
	if !exist {
		return ErrCompetitionNotFound
	}

	// validate join player data
	if joinPlayer.RedeemChips <= 0 {
		return ErrNoRedeemChips
	}

	playerIdx := competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == joinPlayer.PlayerID
	})
	validStatuses := []CompetitionStateStatus{
		CompetitionStateStatus_Registering,
		CompetitionStateStatus_DelayedBuyin,
	}
	if !funk.Contains(validStatuses, competition.State.Status) {
		if playerIdx == UnsetValue {
			return ErrBuyInRejected
		} else {
			return ErrReBuyRejected
		}
	}

	if playerIdx != UnsetValue {
		// validate ReBuy times
		if competition.State.Players[playerIdx].ReBuyTimes >= competition.Meta.ReBuySetting.MaxTimes {
			return ErrExceedReBuyLimit
		}
	}

	// do logic
	buyInPlayerCacheHandler := func(playerCache PlayerCache) {
		ce.playerCacheData[joinPlayer.PlayerID] = &playerCache
	}
	reBuyPlayerCacheHandler := func(reBuyTimes int) {
		ce.playerCacheData[joinPlayer.PlayerID].ReBuyTimes = reBuyTimes
	}
	competition.PlayerJoin(tableID, joinPlayer.PlayerID, playerIdx, joinPlayer.RedeemChips, buyInPlayerCacheHandler, reBuyPlayerCacheHandler)

	// call tableEngine
	jp := pokertable.JoinPlayer{
		PlayerID:    joinPlayer.PlayerID,
		RedeemChips: joinPlayer.RedeemChips,
	}
	if err := ce.tableEngine.PlayerJoin(tableID, jp); err != nil {
		return err
	}

	// auto start game if condition is reached
	tableIdx := competition.FindTableIdx(func(table *pokertable.Table) bool {
		return table.ID == tableID
	})
	if tableIdx != UnsetValue {
		if competition.Meta.Mode == CompetitionMode_CT && competition.CanStart() {
			competition.Start()
			if err := ce.tableEngine.StartGame(tableID); err != nil {
				return err
			}
		}
	}

	ce.EmitEvent(competition)
	return nil
}

func (ce *competitionEngine) PlayerAddon(competitionID, tableID string, joinPlayer JoinPlayer) error {
	competition, exist := ce.competitionMap[competitionID]
	if !exist {
		return ErrCompetitionNotFound
	}

	// validate join player data
	if joinPlayer.RedeemChips <= 0 {
		return ErrNoRedeemChips
	}

	playerIdx := competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == joinPlayer.PlayerID
	})
	if playerIdx == UnsetValue {
		return ErrAddonRejected
	}

	// validate Addon times
	if competition.State.Players[playerIdx].AddonTimes >= competition.Meta.AddonSetting.MaxTimes {
		return ErrExceedAddonLimit
	}

	// do logic
	competition.PlayerAddon(tableID, joinPlayer.PlayerID, playerIdx, joinPlayer.RedeemChips)

	// call tableEngine
	jp := pokertable.JoinPlayer{
		PlayerID:    joinPlayer.PlayerID,
		RedeemChips: joinPlayer.RedeemChips,
	}
	if err := ce.tableEngine.PlayerRedeemChips(tableID, jp); err != nil {
		return err
	}

	ce.EmitEvent(competition)
	return nil
}

func (ce *competitionEngine) PlayerRefund(competitionID, playerID string) error {
	competition, exist := ce.competitionMap[competitionID]
	if !exist {
		return ErrCompetitionNotFound
	}

	// validate refund conditions
	playerIdx := competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == playerID
	})
	if playerIdx == UnsetValue {
		return ErrRefundRejected
	}

	if competition.State.Status != CompetitionStateStatus_Registering {
		return ErrRefundRejected
	}

	if competition.Meta.Mode == CompetitionMode_CT {
		playerCache, exist := ce.playerCacheData[playerID]
		if !exist || playerCache.TableID == "" {
			return ErrTableNotFound
		}

		// call tableEngine
		if err := ce.tableEngine.PlayersLeave(playerCache.TableID, []string{playerID}); err != nil {
			return err
		}
	}

	// refund logic
	competition.DeletePlayer(playerIdx)
	delete(ce.playerCacheData, playerID)

	ce.EmitEvent(competition)
	return nil
}

func (ce *competitionEngine) PlayerLeave(competitionID, tableID, playerID string) error {
	competition, exist := ce.competitionMap[competitionID]
	if !exist {
		return ErrCompetitionNotFound
	}

	// validate refund conditions
	playerIdx := competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == playerID
	})
	if playerIdx == UnsetValue {
		return ErrPlayerNotFound
	}

	if competition.Meta.Mode != CompetitionMode_Cash {
		return ErrLeaveRejected
	}

	playerCache, exist := ce.playerCacheData[playerID]
	if !exist || playerCache.TableID == "" {
		return ErrTableNotFound
	}

	// call tableEngine
	if err := ce.tableEngine.PlayersLeave(playerCache.TableID, []string{playerID}); err != nil {
		return err
	}

	// TODO: player settlement (現金桌)

	// logic
	competition.DeletePlayer(playerIdx)
	delete(ce.playerCacheData, playerID)

	ce.EmitEvent(competition)
	return nil
}

func (ce *competitionEngine) onCompetitionTableUpdated(table *pokertable.Table) {
	competition, exist := ce.competitionMap[table.Meta.CompetitionMeta.ID]
	if !exist {
		return
	}

	tableIdx := competition.FindTableIdx(func(t *pokertable.Table) bool {
		return table.ID == t.ID
	})
	if tableIdx == UnsetValue {
		return
	}

	// 處理桌次已結束
	if table.State.Status == pokertable.TableStateStatus_TableGameClosed {
		ce.deleteCompetitionTable(competition, table.ID)
		return
	}

	// 處理停止買入
	if table.State.BlindState.IsFinalBuyInLevel() {
		competition.State.Status = CompetitionStateStatus_DelayedBuyin

		// 初始化排名陣列
		if len(competition.State.Rankings) == 0 {
			for _, table := range competition.State.Tables {
				for i := 0; i < len(table.State.PlayerStates); i++ {
					competition.State.Rankings = append(competition.State.Rankings, nil)
				}
			}
		}
	}

	// 桌次結算
	if ce.shouldSettleTable(table.State.GameState.Status.Round, table.State.GameState.Status.CurrentEvent.Name) {
		ce.tableSettlement(competition, table)
	}

	// 更新 competition 桌次
	competition.State.Tables[tableIdx] = table
}

func (ce *competitionEngine) addCompetitionTable(competition *Competition, tableSetting TableSetting) error {
	// create table
	setting := NewPokerTableSetting(competition.ID, competition.Meta, tableSetting)
	table, err := ce.tableEngine.CreateTable(setting)
	if err != nil {
		return err
	}

	// add table
	competition.AddTable(table, func(playerCache PlayerCache) {
		ce.playerCacheData[playerCache.PlayerID] = &playerCache
	})
	return nil
}

func (ce *competitionEngine) closeCompetitionTable(competition *Competition, tableID string, tableStatus pokertable.TableStateStatus) error {
	// tableEngine close table
	if err := ce.tableEngine.CloseTable(tableID, tableStatus); err != nil {
		return err
	}

	return ce.deleteCompetitionTable(competition, tableID)
}

func (ce *competitionEngine) deleteCompetitionTable(competition *Competition, tableID string) error {
	// competition close table
	deleteTableIdx := competition.FindTableIdx(func(table *pokertable.Table) bool {
		return table.ID == tableID
	})
	if deleteTableIdx == UnsetValue {
		return ErrTableNotFound
	}

	competition.DeleteTable(deleteTableIdx)
	if len(competition.State.Tables) == 0 {
		ce.settleCompetition(competition)
	}

	return nil
}

/*
	settleCompetition 賽事結算
	- 適用時機:
	    - 賽事結束
*/
func (ce *competitionEngine) settleCompetition(competition *Competition) {
	// update final player rankings
	finalRankings := GetParticipatedPlayerCompetitionRankingData(ce.playerCacheData, competition.State.Players)
	for playerID, rankData := range finalRankings {
		rankIdx := rankData.Rank - 1
		competition.State.Rankings[rankIdx] = &CompetitionRank{
			PlayerID:   playerID,
			FinalChips: rankData.Chips,
		}
	}

	// close competition
	competition.Close()

	// Emit event
	ce.EmitEvent(competition)

	// clear cache
	ce.playerCacheData = make(map[string]*PlayerCache)

	// delete from competitionMap
	delete(ce.competitionMap, competition.ID)
}

/*
	tableSettlement 桌次結算
	- 適用時機:
	    - 每手結束
	  - 邏輯:
	    - 更新玩家狀態 (桌內即時排名 & 當前後手碼量)
		- 更新玩家狀態 (可補碼 Or 淘汰)
*/
func (ce *competitionEngine) tableSettlement(competition *Competition, table *pokertable.Table) {
	// Step 1: 更新玩家桌內即時排名 & 當前後手碼量(該手有參賽者會更新排名，若沒參賽者排名為 0)
	playerRankingData := GetParticipatedPlayerTableRankingData(ce.playerCacheData, table.State.PlayerStates, table.State.GamePlayerIndexes)
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
			playerData := ce.playerCacheData[player.PlayerID]
			if playerData.ReBuyTimes < competition.Meta.ReBuySetting.MaxTimes {
				competition.State.Players[playerData.PlayerIdx].IsReBuying = true
				competition.State.Players[playerData.PlayerIdx].ReBuyEndAt = reBuyEndAt
			}
		}
	}

	// Step 3: 處理玩家淘汰
	// 列出淘汰玩家
	knockoutPlayerRankings := GetSortedKnockoutPlayerRankings(ce.playerCacheData, table.State.PlayerStates, competition.Meta.ReBuySetting.MaxTimes)
	knockoutPlayerIDs := make([]string, 0)
	for knockoutPlayerIDIdx := len(knockoutPlayerRankings) - 1; knockoutPlayerIDIdx >= 0; knockoutPlayerIDIdx-- {
		knockoutPlayerID := knockoutPlayerRankings[knockoutPlayerIDIdx]
		knockoutPlayerIDs = append(knockoutPlayerIDs, knockoutPlayerID)

		// 更新賽事排名
		for rankIdx := len(competition.State.Rankings) - 1; rankIdx >= 0; rankIdx-- {
			if competition.State.Rankings[rankIdx] == nil {
				competition.State.Rankings[rankIdx] = &CompetitionRank{
					PlayerID:   knockoutPlayerID,
					FinalChips: 0,
				}
				break
			}
		}

		// 更新玩家狀態
		competition.State.Players[ce.playerCacheData[knockoutPlayerID].PlayerIdx].Status = CompetitionPlayerStatus_Knockout
	}

	// TableEngine Player Leave
	_ = ce.tableEngine.PlayersLeave(table.ID, knockoutPlayerIDs)
}

func (ce *competitionEngine) shouldSettleTable(round, event string) bool {
	// When game entering preflop, we need to settle previous game player status (knockout/re-buy)
	return (round == pokertable.GameRound_Preflod) && (event == pokerface.GameEventSymbols[pokerface.GameEvent_RoundInitialized])
}
