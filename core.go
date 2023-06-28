package pokercompetition

import (
	"fmt"
	"time"

	"github.com/thoas/go-funk"
	"github.com/weedbox/pokertable"
)

func (ce *competitionEngine) incomingRequest(competitionID string, action RequestAction, param interface{}) error {
	competition, exist := ce.competitions.Load(competitionID)
	if !exist {
		return ErrCompetitionNotFound
	}

	ce.incoming <- &Request{
		Action: action,
		Payload: Payload{
			Competition: competition.(*Competition),
			Param:       param,
		},
	}

	return nil
}

func (ce *competitionEngine) emitEvent(eventName string, playerID string, competition *Competition) {
	competition.RefreshUpdateAt()
	// fmt.Printf("->[Competition][#%d][%s] emit Event: %s\n", competition.UpdateSerial, playerID, eventName)
	ce.onCompetitionUpdated(competition)
}

func (ce *competitionEngine) emitErrorEvent(eventName RequestAction, playerID string, err error, competition *Competition) {
	competition.RefreshUpdateAt()
	// fmt.Printf("->[Competition][#%d][%s] emit ERROR Event: %s, Error: %v\n", competition.UpdateSerial, playerID, eventName, err)
	ce.onCompetitionErrorUpdated(competition, err)
}

func (ce *competitionEngine) run() {
	for req := range ce.incoming {
		ce.requestHandler(req)
	}
}

func (ce *competitionEngine) requestHandler(req *Request) {
	handlers := map[RequestAction]func(Payload){
		RequestAction_PlayerJoin:   ce.handlePlayerJoin,
		RequestAction_PlayerAddon:  ce.handlePlayerAddon,
		RequestAction_PlayerRefund: ce.handlePlayerRefund,
		RequestAction_PlayerLeave:  ce.handlePlayerLeave,
	}

	handler, ok := handlers[req.Action]
	if !ok {
		return
	}
	handler(req.Payload)
}

func (ce *competitionEngine) handlePlayerJoin(payload Payload) {
	param := payload.Param.(PlayerJoinParam)
	tableID := param.TableID
	joinPlayer := param.JoinPlayer
	competition := payload.Competition

	// validate join player data
	if joinPlayer.RedeemChips <= 0 {
		ce.emitErrorEvent("PlayerJoin", joinPlayer.PlayerID, ErrNoRedeemChips, competition)
		return
	}

	playerIdx := competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == joinPlayer.PlayerID
	})
	isBuyIn := playerIdx == UnsetValue
	validStatuses := []CompetitionStateStatus{
		CompetitionStateStatus_Registering,
		CompetitionStateStatus_DelayedBuyin,
	}
	if !funk.Contains(validStatuses, competition.State.Status) {
		if playerIdx == UnsetValue {
			ce.emitErrorEvent("PlayerJoin", joinPlayer.PlayerID, ErrBuyInRejected, competition)
			return
		} else {
			ce.emitErrorEvent("PlayerJoin", joinPlayer.PlayerID, ErrReBuyRejected, competition)
			return
		}
	}

	if !isBuyIn {
		// validate ReBuy times
		if competition.State.Players[playerIdx].ReBuyTimes >= competition.Meta.ReBuySetting.MaxTime {
			ce.emitErrorEvent("PlayerJoin", joinPlayer.PlayerID, ErrExceedReBuyLimit, competition)
			return
		}
	}

	// do logic
	buyInPlayerCacheHandler := func(playerCache PlayerCache) {
		ce.insertPlayerCache(competition.ID, joinPlayer.PlayerID, playerCache)
	}
	reBuyPlayerCacheHandler := func(reBuyTimes int) {
		if playerCache, exist := ce.getPlayerCache(competition.ID, joinPlayer.PlayerID); exist {
			playerCache.ReBuyTimes = reBuyTimes
		}
	}
	competition.PlayerJoin(tableID, joinPlayer.PlayerID, playerIdx, joinPlayer.RedeemChips, isBuyIn, buyInPlayerCacheHandler, reBuyPlayerCacheHandler)

	if isBuyIn {
		ce.emitEvent("PlayerJoin -> Buy In", joinPlayer.PlayerID, competition)
	} else {
		ce.emitEvent("PlayerJoin -> Re Buy", joinPlayer.PlayerID, competition)
	}

	switch competition.Meta.Mode {
	case CompetitionMode_CT:
		// call tableEngine
		jp := pokertable.JoinPlayer{
			PlayerID:    joinPlayer.PlayerID,
			RedeemChips: joinPlayer.RedeemChips,
		}
		if err := ce.tableBackend.PlayerJoin(tableID, jp); err != nil {
			ce.emitErrorEvent("CT PlayerJoin -> Table", joinPlayer.PlayerID, err, competition)
		}
	case CompetitionMode_MTT:
		// 開賽前 (status = registering) 玩家配桌會等到正式開賽後才進入拆併桌程式，所以都先留在等待區
		if competition.State.Status == CompetitionStateStatus_DelayedBuyin {
			if isBuyIn {
				// 已經開賽了，BuyIn 玩家丟到拆併桌 Queue
				ce.seatManagerJoinPlayer(competition.ID, []string{joinPlayer.PlayerID})
			} else {
				// ReBuy 玩家 call tableEngine
				jp := pokertable.JoinPlayer{
					PlayerID:    joinPlayer.PlayerID,
					RedeemChips: joinPlayer.RedeemChips,
				}
				if err := ce.tableBackend.PlayerJoin(tableID, jp); err != nil {
					ce.emitErrorEvent("MTT PlayerJoin -> Table", joinPlayer.PlayerID, err, competition)
				}
			}
		}
	}
}

func (ce *competitionEngine) handlePlayerAddon(payload Payload) {
	param := payload.Param.(PlayerAddonParam)
	tableID := param.TableID
	joinPlayer := param.JoinPlayer
	competition := payload.Competition

	// validate join player data
	if joinPlayer.RedeemChips <= 0 {
		ce.emitErrorEvent("PlayerAddon", joinPlayer.PlayerID, ErrNoRedeemChips, competition)
		return
	}

	playerIdx := competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == joinPlayer.PlayerID
	})
	if playerIdx == UnsetValue {
		ce.emitErrorEvent("PlayerAddon", joinPlayer.PlayerID, ErrAddonRejected, competition)
		return
	}

	// validate Addon times
	if competition.State.Players[playerIdx].AddonTimes >= competition.Meta.AddonSetting.MaxTime {
		ce.emitErrorEvent("PlayerAddon", joinPlayer.PlayerID, ErrExceedAddonLimit, competition)
		return
	}

	// do logic
	competition.PlayerAddon(tableID, joinPlayer.PlayerID, playerIdx, joinPlayer.RedeemChips)

	// call tableEngine
	jp := pokertable.JoinPlayer{
		PlayerID:    joinPlayer.PlayerID,
		RedeemChips: joinPlayer.RedeemChips,
	}
	if err := ce.tableBackend.PlayerRedeemChips(tableID, jp); err != nil {
		ce.emitErrorEvent("PlayerAddon -> PlayerRedeemChips", joinPlayer.PlayerID, err, competition)
		return
	}

	ce.emitEvent("PlayerAddon", joinPlayer.PlayerID, competition)
}

func (ce *competitionEngine) handlePlayerRefund(payload Payload) {
	param := payload.Param.(PlayerRefundParam)
	tableID := param.TableID
	playerID := param.PlayerID
	competition := payload.Competition

	// validate refund conditions
	playerIdx := competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == playerID
	})
	if playerIdx == UnsetValue {
		ce.emitErrorEvent("PlayerRefund -> No player", playerID, ErrRefundRejected, competition)
		return
	}

	if competition.State.Status != CompetitionStateStatus_Registering {
		ce.emitErrorEvent("PlayerRefund -> Wrong Status", playerID, ErrRefundRejected, competition)
		return
	}

	if competition.Meta.Mode == CompetitionMode_CT {
		// call tableEngine
		if err := ce.tableBackend.PlayersLeave(tableID, []string{playerID}); err != nil {
			ce.emitErrorEvent("PlayerRefund -> PlayersLeave Table", playerID, err, competition)
			return
		}
	}

	// refund logic
	competition.DeletePlayer(playerIdx)
	ce.deletePlayerCache(competition.ID, playerID)

	ce.emitEvent("PlayerRefund", playerID, competition)
}

func (ce *competitionEngine) handlePlayerLeave(payload Payload) {
	param := payload.Param.(PlayerLeaveParam)
	tableID := param.TableID
	playerID := param.PlayerID
	competition := payload.Competition

	// validate refund conditions
	playerIdx := competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == playerID
	})
	if playerIdx == UnsetValue {
		ce.emitErrorEvent("PlayerLeave -> No player", playerID, ErrLeaveRejected, competition)
		return
	}

	if competition.Meta.Mode != CompetitionMode_Cash {
		ce.emitErrorEvent("PlayerLeave -> Not Cash Mode", playerID, ErrLeaveRejected, competition)
		return
	}

	// call tableEngine
	if err := ce.tableBackend.PlayersLeave(tableID, []string{playerID}); err != nil {
		ce.emitErrorEvent("PlayerLeave -> PlayersLeave Table", playerID, err, competition)
		return
	}

	// TODO: player settlement (現金桌)

	// logic
	competition.DeletePlayer(playerIdx)
	ce.deletePlayerCache(competition.ID, playerID)

	ce.emitEvent("PlayerLeave", playerID, competition)
}

// func (ce *competitionEngine) onCompetitionTableUpdated(table *pokertable.Table) {
func (ce *competitionEngine) UpdateTable(table *pokertable.Table) {
	fmt.Println("[competitionEngine#UpdateTable] Table: ", table.State.Status)
	c, exist := ce.competitions.Load(table.Meta.CompetitionMeta.ID)
	if !exist {
		return
	}
	competition := c.(*Competition)

	tableIdx := competition.FindTableIdx(func(t *pokertable.Table) bool {
		return table.ID == t.ID
	})
	if tableIdx == UnsetValue {
		return
	}

	// 更新 competition table
	competition.State.Tables[tableIdx] = table

	// 處理因 table status 產生的變化
	tableStatusHandlerMap := map[pokertable.TableStateStatus]func(*Competition, *pokertable.Table, int){
		pokertable.TableStateStatus_TableCreated:     ce.handleCompetitionTableCreated,
		pokertable.TableStateStatus_TableGameSettled: ce.settleCompetitionTable,
		pokertable.TableStateStatus_TableClosed:      ce.closeCompetitionTable,
		pokertable.TableStateStatus_TablePausing:     ce.updatePauseCompetition,
	}
	handler, ok := tableStatusHandlerMap[table.State.Status]
	if !ok {
		return
	}
	handler(competition, table, tableIdx)
}

func (ce *competitionEngine) addCompetitionTable(competition *Competition, tableSetting TableSetting) (string, error) {
	// create table
	setting := NewPokerTableSetting(competition.ID, competition.Meta, tableSetting)
	table, err := ce.tableBackend.CreateTable(setting)
	if err != nil {
		return "", err
	}

	// add table
	competition.AddTable(table, func(playerCache PlayerCache) {
		ce.insertPlayerCache(competition.ID, playerCache.PlayerID, playerCache)
	})
	return table.ID, nil
}

func (ce *competitionEngine) handleCompetitionTableCreated(competition *Competition, table *pokertable.Table, tableIdx int) {
	switch competition.Meta.Mode {
	case CompetitionMode_CT:
		ce.handleCTTableCreated(competition, table, tableIdx)
	}
}

func (ce *competitionEngine) handleCTTableCreated(competition *Competition, table *pokertable.Table, tableIdx int) {
	if !competition.CanStart() {
		return
	}
	// auto start game if condition is reached
	if err := ce.StartCompetition(competition.ID); err != nil {
		ce.emitErrorEvent("CT Auto StartCompetition", "", err, competition)
		return
	}

	if err := ce.tableBackend.StartTableGame(table.ID); err != nil {
		ce.emitErrorEvent("CT Auto StartTableGame", "", err, competition)
		return
	}
}

func (ce *competitionEngine) updatePauseCompetition(competition *Competition, table *pokertable.Table, tableIdx int) {
	if table.IsClose() {
		ce.closeCompetitionTable(competition, table, tableIdx)
	} else if len(table.AlivePlayers()) > competition.Meta.TableMinPlayerCount {
		// 人數又足以開始新遊戲一定是延遲買入階段
		competition.State.Status = CompetitionStateStatus_DelayedBuyin
		_ = ce.tableBackend.TableGameOpen(table.ID)
		ce.emitEvent("Game Reopen:", "", competition)
	}
}

/*
	closeCompetitionTable 桌次關閉
	  - 適用時機: 桌次結束已發生
*/
func (ce *competitionEngine) closeCompetitionTable(competition *Competition, table *pokertable.Table, tableIdx int) {
	// competition close table
	competition.DeleteTable(tableIdx)
	ce.emitEvent("closeCompetitionTable", "", competition)

	if len(competition.State.Tables) == 0 {
		ce.settleCompetition(competition)
	}
}

/*
	settleCompetitionTable 桌次結算
	  - 適用時機: 每手結束
*/
func (ce *competitionEngine) settleCompetitionTable(competition *Competition, table *pokertable.Table, tableIdx int) {
	// 桌次結算: 更新玩家桌內即時排名 & 當前後手碼量(該手有參賽者會更新排名，若沒參賽者排名為 0)
	playerRankingData := ce.GetParticipatedPlayerTableRankingData(competition.ID, table.State.PlayerStates, table.State.GamePlayerIndexes)
	for playerIdx := 0; playerIdx < len(competition.State.Players); playerIdx++ {
		player := competition.State.Players[playerIdx]
		if rankData, exist := playerRankingData[player.PlayerID]; exist {
			competition.State.Players[playerIdx].Rank = rankData.Rank
			competition.State.Players[playerIdx].Chips = rankData.Chips
		}
	}

	// 根據是否達到停止買入做處理
	if !table.State.BlindState.IsFinalBuyInLevel() {
		// 延遲買入: 處理可補碼玩家
		reBuyEndAt := time.Now().Add(time.Second * time.Duration(competition.Meta.ReBuySetting.WaitingTime)).Unix()
		for _, player := range table.State.PlayerStates {
			playerCache, exist := ce.getPlayerCache(competition.ID, player.PlayerID)
			if !exist {
				continue
			}
			if playerCache.ReBuyTimes < competition.Meta.ReBuySetting.MaxTime {
				competition.State.Players[playerCache.PlayerIdx].IsReBuying = true
				competition.State.Players[playerCache.PlayerIdx].ReBuyEndAt = reBuyEndAt
			}
		}
	} else {
		// 停止買入
		// 更新賽事狀態: 停止買入
		competition.State.Status = CompetitionStateStatus_StoppedBuyin

		// 初始化排名陣列
		if len(competition.State.Rankings) == 0 {
			for i := 0; i < len(competition.State.Players); i++ {
				competition.State.Rankings = append(competition.State.Rankings, nil)
			}
		}
	}

	// 處理淘汰玩家
	// 列出淘汰玩家
	knockoutPlayerRankings := ce.GetSortedKnockoutPlayerRankings(competition.ID, table.State.PlayerStates, competition.Meta.ReBuySetting.MaxTime, table.State.BlindState.IsFinalBuyInLevel())
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
		playerCache, exist := ce.getPlayerCache(competition.ID, knockoutPlayerID)
		if !exist {
			continue
		}
		competition.State.Players[playerCache.PlayerIdx].Status = CompetitionPlayerStatus_Knockout
	}

	// TableEngine Player Leave
	_ = ce.tableBackend.PlayersLeave(table.ID, knockoutPlayerIDs)

	// 桌次處理
	switch competition.Meta.Mode {
	case CompetitionMode_CT:
		// 結束桌
		if table.IsClose() {
			_ = ce.tableBackend.DeleteTable(table.ID)
		}
	case CompetitionMode_MTT:
		// 拆併桌更新賽事狀態
		ce.seatManagerUpdateCompetitionStatus(competition.ID, table.State.BlindState.IsFinalBuyInLevel())

		currentPlayerIDs := make([]string, 0)
		for _, player := range table.State.PlayerStates {
			if !funk.Contains(knockoutPlayerIDs, player.PlayerID) {
				currentPlayerIDs = append(currentPlayerIDs, player.PlayerID)
			}
		}

		// 拆併桌更新桌次狀態
		isSuspend, err := ce.seatManagerUpdateTable(competition.ID, table, currentPlayerIDs)
		if err == nil && isSuspend {
			// call table agent to balance table
			_ = ce.tableBackend.BalanceTable(table.ID)

			// update player status
			for _, playerID := range currentPlayerIDs {
				if playerCache, exist := ce.getPlayerCache(competition.ID, playerID); exist {
					competition.State.Players[playerCache.PlayerIdx].Status = CompetitionPlayerStatus_WaitingTableBalancing
				}
			}
		}
	}
}

/*
	settleCompetition 賽事結算
	- 適用時機: 賽事結束
*/
func (ce *competitionEngine) settleCompetition(competition *Competition) {
	// update final player rankings
	finalRankings := ce.GetParticipatedPlayerCompetitionRankingData(competition.ID, competition.State.Players)
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
	ce.emitEvent("settleCompetition", "", competition)

	// clear cache
	ce.deletePlayerCachesByCompetition(competition.ID)

	if competition.Meta.Mode == CompetitionMode_MTT {
		// unregister seat manager
		ce.deactivateSeatManager(competition.ID)
	}

	// delete from competitions
	ce.competitions.Delete(competition.ID)
}

func newDefaultCompetitionPlayerData(tableID, playerID string, redeemChips int64) (CompetitionPlayer, PlayerCache) {
	joinAt := time.Now().Unix()
	playerCache := PlayerCache{
		PlayerID:   playerID,
		JoinAt:     joinAt,
		ReBuyTimes: 0,
		TableID:    tableID,
	}

	player := CompetitionPlayer{
		PlayerID:              playerID,
		CurrentTableID:        tableID,
		JoinAt:                joinAt,
		Status:                CompetitionPlayerStatus_Playing,
		Rank:                  UnsetValue,
		Chips:                 redeemChips,
		IsReBuying:            false,
		ReBuyEndAt:            UnsetValue,
		ReBuyTimes:            0,
		AddonTimes:            0,
		BestWinningPotChips:   0,
		BestWinningCombo:      make([]string, 0),
		TotalRedeemChips:      redeemChips,
		TotalGameCounts:       0,
		TotalWalkTimes:        0,
		TotalVPIPTimes:        0,
		TotalFoldTimes:        0,
		TotalPreflopFoldTimes: 0,
		TotalFlopFoldTimes:    0,
		TotalTurnFoldTimes:    0,
		TotalRiverFoldTimes:   0,
		TotalActionTimes:      0,
		TotalRaiseTimes:       0,
		TotalCallTimes:        0,
		TotalCheckTimes:       0,
	}

	return player, playerCache
}
