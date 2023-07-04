package pokercompetition

import (
	"fmt"
	"strings"
	"time"

	"github.com/thoas/go-funk"
	"github.com/weedbox/pokertable"
	"github.com/weedbox/timebank"
)

func (ce *competitionEngine) emitEvent(eventName string, playerID string) {
	// refresh competition
	ce.competition.UpdateAt = time.Now().Unix()
	ce.competition.UpdateSerial++

	// emit event
	fmt.Printf("->[Competition][#%d][%s] emit Event: %s\n", ce.competition.UpdateSerial, playerID, eventName)
	ce.onCompetitionUpdated(ce.competition)
}

func (ce *competitionEngine) emitErrorEvent(eventName string, playerID string, err error) {
	fmt.Printf("->[Competition][#%d][%s] emit ERROR Event: %s, Error: %v\n", ce.competition.UpdateSerial, playerID, eventName, err)
	ce.onCompetitionErrorUpdated(ce.competition, err)
}

func (ce *competitionEngine) emitPlayerEvent(eventName string, player *CompetitionPlayer) {
	// emit event
	fmt.Printf("->[CompetitionPlayer][%s] emit Event: %s\n", eventName, fmt.Sprintf("[%s][%s]: %s", player.PlayerID, player.CurrentTableID, player.Status))
	ce.onCompetitionPlayerUpdated(player)
}

func (ce *competitionEngine) newDefaultCompetitionPlayerData(tableID, playerID string, redeemChips int64, playerStatus CompetitionPlayerStatus) (CompetitionPlayer, PlayerCache) {
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
		Status:                playerStatus,
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

func (ce *competitionEngine) updateTable(table *pokertable.Table) {
	tableIdx := ce.competition.FindTableIdx(func(t *pokertable.Table) bool {
		return table.ID == t.ID
	})
	if tableIdx == UnsetValue {
		return
	}

	// 更新 competition table
	ce.competition.State.Tables[tableIdx] = table

	// 處理因 table status 產生的變化
	tableStatusHandlerMap := map[pokertable.TableStateStatus]func(*pokertable.Table, int){
		pokertable.TableStateStatus_TableCreated:     ce.handleCompetitionTableCreated,
		pokertable.TableStateStatus_TablePausing:     ce.updatePauseCompetition,
		pokertable.TableStateStatus_TableClosed:      ce.closeCompetitionTable,
		pokertable.TableStateStatus_TableGameSettled: ce.settleCompetitionTable,
	}
	handler, ok := tableStatusHandlerMap[table.State.Status]
	if !ok {
		return
	}
	handler(table, tableIdx)
}

func (ce *competitionEngine) handleCompetitionTableCreated(table *pokertable.Table, tableIdx int) {
	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT:
		if !ce.competition.CanStart() {
			return
		}

		// auto start game if condition is reached
		if err := ce.StartCompetition(); err != nil {
			ce.emitErrorEvent("CT Auto StartCompetition", "", err)
			return
		}

		if err := ce.tableManagerBackend.StartTableGame(table.ID); err != nil {
			ce.emitErrorEvent("CT Auto StartTableGame", "", err)
			return
		}
	}
}

func (ce *competitionEngine) updatePauseCompetition(table *pokertable.Table, tableIdx int) {
	if table.ShouldClose() {
		ce.closeCompetitionTable(table, tableIdx)
		return
	}

	if ce.competition.Meta.Mode == CompetitionMode_CT {
		readyPlayersCount := 0
		for _, p := range table.State.PlayerStates {
			if p.IsIn && p.Bankroll > 0 {
				readyPlayersCount++
			}
		}
		if ce.competition.State.Status == CompetitionStateStatus_DelayedBuyIn && readyPlayersCount > ce.competition.Meta.TableMinPlayerCount {
			if err := ce.tableManagerBackend.TableGameOpen(table.ID); err != nil {
				ce.emitErrorEvent("Game Reopen", "", err)
				return
			}
			ce.emitEvent("Game Reopen:", "")
		}
	}
}

func (ce *competitionEngine) addCompetitionTable(tableSetting TableSetting, playerStatus CompetitionPlayerStatus) (string, error) {
	// create table
	setting := NewPokerTableSetting(ce.competition.ID, ce.competition.Meta, tableSetting)
	table, err := ce.tableManagerBackend.CreateTable(setting)
	if err != nil {
		return "", err
	}

	// add table
	ce.competition.State.Tables = append(ce.competition.State.Tables, table)

	// update players
	newPlayerData := make(map[string]int64)
	for _, ps := range table.State.PlayerStates {
		newPlayerData[ps.PlayerID] = ps.Bankroll
	}

	// find existing players
	existingPlayerData := make(map[string]int64)
	for _, player := range ce.competition.State.Players {
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
		for i := 0; i < len(ce.competition.State.Players); i++ {
			player := ce.competition.State.Players[i]
			if bankroll, exist := existingPlayerData[player.PlayerID]; exist {
				ce.competition.State.Players[i].Chips = bankroll
				ce.competition.State.Players[i].Status = playerStatus
				ce.emitPlayerEvent("[addCompetitionTable] existing player", ce.competition.State.Players[i])
			}
		}
	}

	// add new player data
	if len(newPlayerData) > 0 {
		newPlayerIdx := len(ce.competition.State.Players)
		newPlayers := make([]*CompetitionPlayer, 0)
		for playerID, bankroll := range newPlayerData {
			player, playerCache := ce.newDefaultCompetitionPlayerData(table.ID, playerID, bankroll, playerStatus)
			newPlayers = append(newPlayers, &player)
			playerCache.PlayerIdx = newPlayerIdx
			ce.insertPlayerCache(ce.competition.ID, playerCache.PlayerID, playerCache)
			newPlayerIdx++
			ce.emitPlayerEvent("[addCompetitionTable] new player", &player)
		}
		ce.competition.State.Players = append(ce.competition.State.Players, newPlayers...)
	}

	return table.ID, nil
}

/*
	settleCompetition 賽事結算
	- 適用時機: 賽事結束
*/
func (ce *competitionEngine) settleCompetition() {
	// update final player rankings
	settleStatuses := []CompetitionStateStatus{
		CompetitionStateStatus_DelayedBuyIn,
		CompetitionStateStatus_StoppedBuyIn,
	}
	if funk.Contains(settleStatuses, ce.competition.State.Status) {
		finalRankings := ce.GetParticipatedPlayerCompetitionRankingData(ce.competition.ID, ce.competition.State.Players)
		for playerID, rankData := range finalRankings {
			rankIdx := rankData.Rank - 1
			ce.competition.State.Rankings[rankIdx] = &CompetitionRank{
				PlayerID:   playerID,
				FinalChips: rankData.Chips,
			}
		}
	}

	// close competition
	ce.competition.State.Status = CompetitionStateStatus_End
	if ce.competition.Meta.Mode == CompetitionMode_MTT {
		ce.competition.State.EndAt = time.Now().Unix()
	}

	// Emit event
	ce.emitEvent("settleCompetition", "")

	// clear cache
	ce.deletePlayerCachesByCompetition(ce.competition.ID)

	if ce.competition.Meta.Mode == CompetitionMode_MTT {
		// unregister seat manager
		ce.deactivateSeatManager(ce.competition.ID)
	}
}

func (ce *competitionEngine) deleteTable(tableIdx int) {
	ce.competition.State.Tables = append(ce.competition.State.Tables[:tableIdx], ce.competition.State.Tables[tableIdx+1:]...)
}

func (ce *competitionEngine) deletePlayer(playerIdx int) {
	ce.competition.State.Players = append(ce.competition.State.Players[:playerIdx], ce.competition.State.Players[playerIdx+1:]...)
}

/*
	closeCompetitionTable 桌次關閉
	  - 適用時機: 桌次結束已發生
*/
func (ce *competitionEngine) closeCompetitionTable(table *pokertable.Table, tableIdx int) {
	// competition close table
	ce.deleteTable(tableIdx)
	ce.emitEvent("closeCompetitionTable", "")

	if len(ce.competition.State.Tables) == 0 {
		ce.settleCompetition()
	}
}

/*
	settleCompetitionTable 桌次結算
	  - 適用時機: 每手結束
*/
func (ce *competitionEngine) settleCompetitionTable(table *pokertable.Table, tableIdx int) {
	// 桌次結算: 更新玩家桌內即時排名 & 當前後手碼量(該手有參賽者會更新排名，若沒參賽者排名為 0)
	playerRankingData := ce.GetParticipatedPlayerTableRankingData(ce.competition.ID, table.State.PlayerStates, table.State.GamePlayerIndexes)
	for playerIdx := 0; playerIdx < len(ce.competition.State.Players); playerIdx++ {
		player := ce.competition.State.Players[playerIdx]
		if rankData, exist := playerRankingData[player.PlayerID]; exist {
			ce.competition.State.Players[playerIdx].Rank = rankData.Rank
			ce.competition.State.Players[playerIdx].Chips = rankData.Chips
		}
	}

	// 根據是否達到停止買入做處理
	if !table.State.BlindState.IsFinalBuyInLevel() {
		// 延遲買入: 處理可補碼玩家
		reBuyEndAt := time.Now().Add(time.Second * time.Duration(ce.competition.Meta.ReBuySetting.WaitingTime)).Unix()
		reBuyPlayerIDs := make([]string, 0)
		for _, player := range table.State.PlayerStates {
			if player.Bankroll > 0 {
				continue
			}

			playerCache, exist := ce.getPlayerCache(ce.competition.ID, player.PlayerID)
			if !exist {
				continue
			}
			if playerCache.ReBuyTimes < ce.competition.Meta.ReBuySetting.MaxTime {
				ce.competition.State.Players[playerCache.PlayerIdx].IsReBuying = true
				ce.competition.State.Players[playerCache.PlayerIdx].ReBuyEndAt = reBuyEndAt
				ce.emitPlayerEvent("re-buying", ce.competition.State.Players[playerCache.PlayerIdx])
				reBuyPlayerIDs = append(reBuyPlayerIDs, player.PlayerID)
			}
		}

		if len(reBuyPlayerIDs) > 0 && ce.competition.Meta.Mode == CompetitionMode_CT {
			// AutoKnockout Player (When ReBuyEnd Time is reached)
			reBuyEndAtTime := time.Unix(reBuyEndAt, 0)
			if err := timebank.NewTimeBank().NewTaskWithDeadline(reBuyEndAtTime, func(isCancelled bool) {
				if isCancelled {
					return
				}

				knockoutPlayerIDs := make([]string, 0, len(reBuyPlayerIDs))
				for _, playerID := range reBuyPlayerIDs {
					playerCache, exist := ce.getPlayerCache(ce.competition.ID, playerID)
					if !exist {
						fmt.Println("[playerID] is not in the cache")
						continue
					}
					if ce.competition.State.Players[playerCache.PlayerIdx].Chips <= 0 {
						knockoutPlayerIDs = append(knockoutPlayerIDs, playerID)
						ce.competition.State.Players[playerCache.PlayerIdx].Status = CompetitionPlayerStatus_Knockout
						ce.competition.State.Players[playerCache.PlayerIdx].IsReBuying = false
						ce.competition.State.Players[playerCache.PlayerIdx].ReBuyEndAt = UnsetValue
					}
				}

				if len(knockoutPlayerIDs) > 0 {
					if err := ce.tableManagerBackend.PlayersLeave(table.ID, knockoutPlayerIDs); err != nil {
						ce.emitErrorEvent("Knockout Players -> PlayersLeave", strings.Join(knockoutPlayerIDs, ","), err)
					}
				}
			}); err != nil {
				ce.emitErrorEvent("Auto Knockout ReBuy Players", "", err)
				return
			}
		}
	} else {
		// 停止買入
		// 更新賽事狀態: 停止買入
		ce.competition.State.Status = CompetitionStateStatus_StoppedBuyIn

		// 初始化排名陣列
		if len(ce.competition.State.Rankings) == 0 {
			for i := 0; i < len(ce.competition.State.Players); i++ {
				ce.competition.State.Rankings = append(ce.competition.State.Rankings, nil)
			}
		}
	}

	// 處理淘汰玩家
	// 列出淘汰玩家
	knockoutPlayerRankings := ce.GetSortedKnockoutPlayerRankings(ce.competition.ID, table.State.PlayerStates, ce.competition.Meta.ReBuySetting.MaxTime, table.State.BlindState.IsFinalBuyInLevel())
	knockoutPlayerIDs := make([]string, 0)
	for knockoutPlayerIDIdx := len(knockoutPlayerRankings) - 1; knockoutPlayerIDIdx >= 0; knockoutPlayerIDIdx-- {
		knockoutPlayerID := knockoutPlayerRankings[knockoutPlayerIDIdx]
		knockoutPlayerIDs = append(knockoutPlayerIDs, knockoutPlayerID)

		// 更新賽事排名
		for rankIdx := len(ce.competition.State.Rankings) - 1; rankIdx >= 0; rankIdx-- {
			if ce.competition.State.Rankings[rankIdx] == nil {
				ce.competition.State.Rankings[rankIdx] = &CompetitionRank{
					PlayerID:   knockoutPlayerID,
					FinalChips: 0,
				}
				break
			}
		}

		// 更新玩家狀態
		playerCache, exist := ce.getPlayerCache(ce.competition.ID, knockoutPlayerID)
		if !exist {
			continue
		}
		ce.competition.State.Players[playerCache.PlayerIdx].Status = CompetitionPlayerStatus_Knockout
		ce.emitPlayerEvent("knockout", ce.competition.State.Players[playerCache.PlayerIdx])
	}

	// 桌次處理
	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT:
		// TableEngine Player Leave
		if len(knockoutPlayerIDs) > 0 {
			if err := ce.tableManagerBackend.PlayersLeave(table.ID, knockoutPlayerIDs); err != nil {
				ce.emitErrorEvent("Knockout Players -> PlayersLeave", strings.Join(knockoutPlayerIDs, ","), err)
			}
		}

		// 結束桌
		if table.ShouldClose() {
			if err := ce.tableManagerBackend.CloseTable(table.ID); err != nil {
				ce.emitErrorEvent("Knockout Players -> CloseTable", "", err)
			}
		}
	case CompetitionMode_MTT:
		zeroChipPlayerIDs := make([]string, 0)
		alivePlayerIDs := make([]string, 0)
		for _, p := range table.State.PlayerStates {
			if p.Bankroll > 0 {
				alivePlayerIDs = append(alivePlayerIDs, p.PlayerID)
			} else {
				zeroChipPlayerIDs = append(zeroChipPlayerIDs, p.PlayerID)
			}
		}

		if len(zeroChipPlayerIDs) > 0 {
			if err := ce.tableManagerBackend.PlayersLeave(table.ID, zeroChipPlayerIDs); err != nil {
				ce.emitErrorEvent("Clean 0 chip Players -> PlayersLeave", strings.Join(zeroChipPlayerIDs, ","), err)
			}
		}

		// 拆併桌更新賽事狀態
		ce.seatManagerUpdateCompetitionStatus(ce.competition.ID, table.State.BlindState.IsFinalBuyInLevel())

		// 拆併桌更新桌次狀態
		isSuspend, err := ce.seatManagerUpdateTable(ce.competition.ID, table, alivePlayerIDs)
		if err != nil {
			ce.emitErrorEvent("seatManagerUpdateTable", "", err)
			return
		}

		if isSuspend {
			// call table agent to balance table
			if err := ce.tableManagerBackend.BalanceTable(table.ID); err != nil {
				ce.emitErrorEvent("BalanceTable", "", err)
				return
			}

			// update player status
			for _, playerID := range alivePlayerIDs {
				if playerCache, exist := ce.getPlayerCache(ce.competition.ID, playerID); exist {
					ce.competition.State.Players[playerCache.PlayerIdx].Status = CompetitionPlayerStatus_WaitingTableBalancing
					ce.emitPlayerEvent("[settleCompetitionTable] wait balance table", ce.competition.State.Players[playerCache.PlayerIdx])
				}
			}
		}
	}
}
