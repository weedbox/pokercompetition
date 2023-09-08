package pokercompetition

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/thoas/go-funk"
	pokerblind "github.com/weedbox/pokercompetition/blind"
	"github.com/weedbox/pokerface"
	"github.com/weedbox/pokerface/match"
	"github.com/weedbox/pokertable"
	"github.com/weedbox/timebank"
)

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
		BestWinningType:       "",
		BestWinningPower:      0,
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
		TotalProfitTimes:      0,
	}

	return player, playerCache
}

func (ce *competitionEngine) UpdateTablePlayerState(playerState *pokertable.TablePlayerState) {
	// 更新玩家狀態
	playerCache, exist := ce.getPlayerCache(ce.competition.ID, playerState.PlayerID)
	if !exist {
		return
	}

	ce.competition.State.Players[playerCache.PlayerIdx].CurrentSeat = playerState.Seat
}

func (ce *competitionEngine) UpdateTable(table *pokertable.Table) {
	tableIdx := ce.competition.FindTableIdx(func(t *pokertable.Table) bool {
		return table.ID == t.ID
	})
	if tableIdx == UnsetValue {
		return
	}

	// 更新 competition table
	var cloneTable pokertable.Table
	if encoded, err := json.Marshal(table); err == nil {
		json.Unmarshal(encoded, &cloneTable)
	} else {
		cloneTable = *table
	}
	ce.competition.State.Tables[tableIdx] = &cloneTable

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

		ce.updateTableBlind(table.ID)

		if err := ce.tableManagerBackend.StartTableGame(table.ID); err != nil {
			ce.emitErrorEvent("CT Auto StartTableGame", "", err)
			return
		}
	}
}

func (ce *competitionEngine) updatePauseCompetition(table *pokertable.Table, tableIdx int) {
	reopenGame := func(ce *competitionEngine, table *pokertable.Table) {
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

	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT:
		if ce.shouldCloseTable(table.State.StartAt, len(table.AlivePlayers())) {
			ce.closeCompetitionTable(table, tableIdx)
			return
		}

		reopenGame(ce, table)
	case CompetitionMode_MTT:
		// TODO: checkout resume from breaking logic
		if ce.competition.IsBreaking() {
			if !ce.setResumeFromPauseTask {
				// resume game from breaking
				endAt := ce.competition.State.BlindState.EndAts[ce.competition.State.BlindState.CurrentLevelIndex]
				if err := timebank.NewTimeBank().NewTaskWithDeadline(time.Unix(endAt, 0), func(isCancelled bool) {
					if isCancelled {
						return
					}

					endStatus := []CompetitionStateStatus{
						CompetitionStateStatus_End,
						CompetitionStateStatus_AutoEnd,
						CompetitionStateStatus_ForceEnd,
					}
					if funk.Contains(endStatus, ce.competition.State.Status) {
						return
					}

					if len(ce.competition.State.Tables) > tableIdx {
						t := ce.competition.State.Tables[tableIdx]
						tableID := t.ID
						// TODO: close table check might not be necessary
						if ce.shouldCloseTable(t.State.StartAt, len(t.AlivePlayers())) {
							if err := ce.tableManagerBackend.CloseTable(tableID); err != nil {
								ce.emitErrorEvent("resume game from breaking & close table", "", err)
								return
							}
						}

						autoOpenGame := t.State.Status == pokertable.TableStateStatus_TablePausing && len(t.AlivePlayers()) >= t.Meta.TableMinPlayerCount
						if !autoOpenGame {
							return
						}
						if err := ce.tableManagerBackend.TableGameOpen(tableID); err != nil {
							ce.emitErrorEvent("resume game from breaking & auto open next game", "", err)
						}
					}

					ce.setResumeFromPauseTask = false
				}); err != nil {
					ce.emitErrorEvent("new resume game task from breaking", "", err)
					return
				}
				ce.setResumeFromPauseTask = true
			}
		}
		reopenGame(ce, table)
	}
}

func (ce *competitionEngine) addCompetitionTable(tableSetting TableSetting, playerStatus CompetitionPlayerStatus) (string, error) {
	// create table
	setting := NewPokerTableSetting(ce.competition.ID, ce.competition.Meta, tableSetting)
	table, err := ce.tableManagerBackend.CreateTable(ce.tableOptions, setting)
	if err != nil {
		return "", err
	}

	// TODO: test only
	fmt.Printf("[DEBUG#MTT] create table: %s, total tables: %d\n", table.ID, len(ce.competition.State.Tables)+1)
	ce.onTableCreated(table)

	// add table
	ce.competition.State.Tables = append(ce.competition.State.Tables, table)
	ce.emitCompetitionStateEvent(CompetitionStateEvent_TableUpdated)

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
func (ce *competitionEngine) settleCompetition(endCompetitionStatus CompetitionStateStatus) {
	// 更新玩家最終排名
	ce.updatePlayerFinalRankings()

	// close competition
	ce.competition.State.Status = endCompetitionStatus
	if ce.competition.Meta.Mode == CompetitionMode_MTT {
		ce.competition.State.EndAt = time.Now().Unix()
	}

	// close blind
	ce.blind.End()

	// Emit event
	ce.emitEvent("settleCompetition", "")
	ce.emitCompetitionStateEvent(CompetitionStateEvent_Settled)

	// clear caches
	ce.deletePlayerCachesByCompetition(ce.competition.ID)
	ce.gameSettledRecords = sync.Map{}

	if ce.competition.Meta.Mode == CompetitionMode_MTT {
		// close match
		if err := ce.match.Close(); err != nil {
			ce.emitErrorEvent("close mtt match", "", err)
		}
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
	// TODO: test only
	fmt.Printf("[DEBUG#MTT] close table: %s, total tables: %d, alive players: %d\n", table.ID, len(ce.competition.State.Tables)-1, ce.competition.PlayingPlayerCount())
	ce.onTableClosed(table)

	// competition close table
	ce.deleteTable(tableIdx)
	ce.emitEvent("closeCompetitionTable", "")
	ce.emitCompetitionStateEvent(CompetitionStateEvent_TableUpdated)

	if len(ce.competition.State.Tables) == 0 {
		ce.CloseCompetition(CompetitionStateStatus_End)
	}
}

/*
settleCompetitionTable 桌次結算
  - 適用時機: 每手結束
*/
func (ce *competitionEngine) settleCompetitionTable(table *pokertable.Table, tableIdx int) {
	if isGameSettled, ok := ce.gameSettledRecords.Load(table.State.GameState.GameID); ok && isGameSettled.(bool) {
		return
	}
	ce.gameSettledRecords.Store(table.State.GameState.GameID, true)

	// 更新玩家相關賽事數據
	ce.updatePlayerCompetitionTaleRecords(table)

	// 根據是否達到停止買入做處理
	ce.handleReBuy(table)

	// 處理淘汰玩家
	knockoutPlayerIDs := ce.handleKnockoutPlayers(table)

	// 事件更新
	ce.emitEvent("Table Settlement", "")
	ce.emitCompetitionStateEvent(CompetitionStateEvent_TableGameSettled)

	// 桌次處理
	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT:
		// TableEngine Player Leave
		if len(knockoutPlayerIDs) > 0 {
			if err := ce.tableManagerBackend.PlayersLeave(table.ID, knockoutPlayerIDs); err != nil {
				ce.emitErrorEvent("Table Settlement Knockout Players -> PlayersLeave", strings.Join(knockoutPlayerIDs, ","), err)
			}
		}

		// 結束桌
		if ce.shouldCloseTable(table.State.StartAt, len(table.AlivePlayers())) {
			if err := ce.tableManagerBackend.CloseTable(table.ID); err != nil {
				ce.emitErrorEvent("Table Settlement Knockout Players -> CloseTable", "", err)
			}
		}
	case CompetitionMode_MTT:
		// 拆併桌更新桌次狀態
		// Preparing seat changes
		if table.State.SeatChanges != nil {
			sc := match.NewSeatChanges()
			for _, player := range table.State.PlayerStates {
				if player.Bankroll == 0 {
					sc.Seats[player.Seat] = "left"
				}
			}
			sc.Dealer = table.State.SeatChanges.NewDealer
			sc.SB = table.State.SeatChanges.NewSB
			sc.BB = table.State.SeatChanges.NewBB

			ce.matchTableBackend.UpdateTable(table.ID, sc)
		} else {
			ce.emitErrorEvent(fmt.Sprintf("[%s] MTT Match Update Table SeatChanges is nil", table.ID), "", errors.New("nil seat change state is not allowed when settling mtt table"))
		}

		zeroChipPlayerIDs := make([]string, 0)
		alivePlayerIDs := make([]string, 0)
		for _, p := range table.State.PlayerStates {
			if p.Bankroll <= 0 {
				zeroChipPlayerIDs = append(zeroChipPlayerIDs, p.PlayerID)
			} else {
				alivePlayerIDs = append(alivePlayerIDs, p.PlayerID)
			}
		}

		if len(zeroChipPlayerIDs) > 0 {
			if err := ce.tableManagerBackend.PlayersLeave(table.ID, zeroChipPlayerIDs); err != nil {
				ce.emitErrorEvent(fmt.Sprintf("[%s][%d] Clean 0 chip Players -> PlayersLeave", table.ID, table.State.GameCount), strings.Join(zeroChipPlayerIDs, ","), err)
			}
		}

		fmt.Printf("---------- [%s][%s] 第 (%d) 手結算, 停止買入: %+v, [離開 %d 人 (%s), 活著: %d 人 (%s)] ----------\n",
			ce.competition.ID,
			table.ID,
			table.State.GameCount,
			ce.competition.State.BlindState.IsFinalBuyInLevel(),
			len(zeroChipPlayerIDs),
			strings.Join(zeroChipPlayerIDs, ","),
			len(alivePlayerIDs),
			strings.Join(alivePlayerIDs, ","),
		)

		// 結束賽事
		if len(alivePlayerIDs) == 1 && len(ce.competition.State.Tables) == 1 {
			ce.CloseCompetition(CompetitionStateStatus_End)
		}
	}
}

func (ce *competitionEngine) handleKnockoutPlayers(table *pokertable.Table) []string {
	// 列出淘汰玩家
	knockoutPlayerRankings := ce.GetSortedTableSettlementKnockoutPlayerRankings(table.State.PlayerStates)
	knockoutPlayerIDs := make([]string, 0)
	for idx, knockoutPlayerID := range knockoutPlayerRankings {
		knockoutPlayerIDs = append(knockoutPlayerIDs, knockoutPlayerID)

		// 更新玩家狀態
		playerCache, exist := ce.getPlayerCache(ce.competition.ID, knockoutPlayerID)
		if !exist {
			continue
		}
		ce.competition.State.Players[playerCache.PlayerIdx].Status = CompetitionPlayerStatus_Knockout
		ce.emitPlayerEvent("table settlement knockout", ce.competition.State.Players[playerCache.PlayerIdx])

		// 更新賽事排名
		ce.competition.State.Rankings = append(ce.competition.State.Rankings, &CompetitionRank{
			PlayerID:   knockoutPlayerID,
			FinalChips: 0,
		})
		rank := ce.competition.PlayingPlayerCount() + (len(knockoutPlayerRankings) - idx)
		ce.emitCompetitionStateFinalPlayerRankEvent(knockoutPlayerID, rank)
	}

	return knockoutPlayerIDs
}

func (ce *competitionEngine) handleReBuy(table *pokertable.Table) {
	if ce.competition.State.BlindState.IsFinalBuyInLevel() {
		return
	}

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

		if !ce.competition.State.Players[playerCache.PlayerIdx].IsReBuying {
			if playerCache.ReBuyTimes < ce.competition.Meta.ReBuySetting.MaxTime {
				ce.competition.State.Players[playerCache.PlayerIdx].IsReBuying = true
				ce.competition.State.Players[playerCache.PlayerIdx].ReBuyEndAt = reBuyEndAt
				ce.emitPlayerEvent("re-buying", ce.competition.State.Players[playerCache.PlayerIdx])
				reBuyPlayerIDs = append(reBuyPlayerIDs, player.PlayerID)
			}
		}
	}

	if len(reBuyPlayerIDs) > 0 && ce.competition.Meta.Mode == CompetitionMode_CT {
		// AutoKnockout Player (When ReBuyEnd Time is reached)
		reBuyEndAtTime := time.Unix(reBuyEndAt, 0)
		if err := timebank.NewTimeBank().NewTaskWithDeadline(reBuyEndAtTime, func(isCancelled bool) {
			if isCancelled {
				return
			}

			knockoutPlayerIDs := make([]string, 0)
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
					ce.emitPlayerEvent("re buy knockout", ce.competition.State.Players[playerCache.PlayerIdx])
				}
			}

			if len(knockoutPlayerIDs) > 0 {
				knockoutPlayerRankings := ce.GetSortedReBuyKnockoutPlayerRankings(knockoutPlayerIDs)

				for idx, knockoutPlayerID := range knockoutPlayerRankings {
					// 更新賽事排名
					ce.competition.State.Rankings = append(ce.competition.State.Rankings, &CompetitionRank{
						PlayerID:   knockoutPlayerID,
						FinalChips: 0,
					})
					rank := ce.competition.PlayingPlayerCount() + (len(knockoutPlayerRankings) - idx)
					ce.emitCompetitionStateFinalPlayerRankEvent(knockoutPlayerID, rank)
				}
				ce.emitEvent("Re Buy Knockout", "")
				ce.emitCompetitionStateEvent(CompetitionStateEvent_KnockoutPlayers)

				if err := ce.tableManagerBackend.PlayersLeave(table.ID, knockoutPlayerIDs); err != nil {
					ce.emitErrorEvent("Knockout Players -> PlayersLeave", strings.Join(knockoutPlayerIDs, ","), err)
				}

				// 結束桌
				if ce.shouldCloseTable(table.State.StartAt, len(table.AlivePlayers())) {
					if err := ce.tableManagerBackend.CloseTable(table.ID); err != nil {
						ce.emitErrorEvent("Re Buy Knockout Players -> CloseTable", "", err)
					}
				}
			}
		}); err != nil {
			ce.emitErrorEvent("Auto Knockout ReBuy Players", "", err)
			return
		}
	}
}

func (ce *competitionEngine) updatePlayerCompetitionTaleRecords(table *pokertable.Table) {
	// 更新玩家統計數據
	gamePlayerPreflopFoldTimes := 0
	for _, player := range table.State.PlayerStates {
		if !player.IsParticipated {
			continue
		}

		playerCache, exist := ce.getPlayerCache(ce.competition.ID, player.PlayerID)
		if !exist {
			continue
		}

		ce.competition.State.Players[playerCache.PlayerIdx].TotalGameCounts++
		if player.GameStatistics.IsFold {
			ce.competition.State.Players[playerCache.PlayerIdx].TotalFoldTimes++
			switch player.GameStatistics.FoldRound {
			case pokertable.GameRound_Preflop:
				ce.competition.State.Players[playerCache.PlayerIdx].TotalPreflopFoldTimes++
				gamePlayerPreflopFoldTimes++
			case pokertable.GameRound_Flop:
				ce.competition.State.Players[playerCache.PlayerIdx].TotalFlopFoldTimes++
			case pokertable.GameRound_Turn:
				ce.competition.State.Players[playerCache.PlayerIdx].TotalTurnFoldTimes++
			case pokertable.GameRound_River:
				ce.competition.State.Players[playerCache.PlayerIdx].TotalRiverFoldTimes++
			}
		}
		ce.competition.State.Players[playerCache.PlayerIdx].TotalActionTimes += player.GameStatistics.ActionTimes
		ce.competition.State.Players[playerCache.PlayerIdx].TotalRaiseTimes += player.GameStatistics.RaiseTimes
		ce.competition.State.Players[playerCache.PlayerIdx].TotalCallTimes += player.GameStatistics.CallTimes
		ce.competition.State.Players[playerCache.PlayerIdx].TotalCheckTimes += player.GameStatistics.CheckTimes
	}

	// 更新贏家統計數據
	for _, playerResult := range table.State.GameState.Result.Players {
		if playerResult.Changed <= 0 {
			continue
		}

		winnerGameIdx := playerResult.Idx
		tablePlayerIdx := table.State.GamePlayerIndexes[winnerGameIdx]
		tablePlayer := table.State.PlayerStates[tablePlayerIdx]
		playerCache, exist := ce.getPlayerCache(ce.competition.ID, tablePlayer.PlayerID)
		if !exist {
			continue
		}

		ce.competition.State.Players[playerCache.PlayerIdx].TotalProfitTimes++

		gs := table.State.GameState
		gsPlayer := gs.GetPlayer(winnerGameIdx)
		if gsPlayer.VPIP {
			ce.competition.State.Players[playerCache.PlayerIdx].TotalVPIPTimes++
		}

		if table.State.CurrentBBSeat == tablePlayer.Seat && tablePlayer.GameStatistics.ActionTimes == 0 && gamePlayerPreflopFoldTimes == len(table.State.GamePlayerIndexes)-1 {
			ce.competition.State.Players[playerCache.PlayerIdx].TotalWalkTimes++
		}

		if playerResult.Changed > ce.competition.State.Players[playerCache.PlayerIdx].BestWinningPotChips {
			ce.competition.State.Players[playerCache.PlayerIdx].BestWinningPotChips = playerResult.Changed
		}

		if gsPlayer.Combination.Power >= ce.competition.State.Players[playerCache.PlayerIdx].BestWinningPower {
			ce.competition.State.Players[playerCache.PlayerIdx].BestWinningPower = gsPlayer.Combination.Power
			ce.competition.State.Players[playerCache.PlayerIdx].BestWinningCombo = gsPlayer.Combination.Cards
			ce.competition.State.Players[playerCache.PlayerIdx].BestWinningType = gsPlayer.Combination.Type
		}
	}

	// 桌次結算: 更新玩家桌內即時排名 & 當前後手碼量(該手有參賽者會更新排名，若沒參賽者排名為 0)
	playerRankingData := ce.GetParticipatedPlayerTableRankingData(ce.competition.ID, table.State.PlayerStates, table.State.GamePlayerIndexes)
	for playerID, rankData := range playerRankingData {
		playerCache, exist := ce.getPlayerCache(ce.competition.ID, playerID)
		if !exist {
			continue
		}

		ce.competition.State.Players[playerCache.PlayerIdx].Rank = rankData.Rank
		ce.competition.State.Players[playerCache.PlayerIdx].Chips = rankData.Chips
		ce.emitPlayerEvent("table-settlement", ce.competition.State.Players[playerCache.PlayerIdx])
	}
}

func (ce *competitionEngine) updatePlayerFinalRankings() {
	// update final player rankings
	settleStatuses := []CompetitionStateStatus{
		CompetitionStateStatus_DelayedBuyIn,
		CompetitionStateStatus_StoppedBuyIn,
	}
	if funk.Contains(settleStatuses, ce.competition.State.Status) {
		finalRankings := ce.GetParticipatedPlayerCompetitionRankingData(ce.competition.ID, ce.competition.State.Players)
		// 名次由後面到前面 insert 至 Rankings
		for i := len(finalRankings) - 1; i >= 0; i-- {
			ranking := finalRankings[i]
			ce.competition.State.Rankings = append(ce.competition.State.Rankings, &CompetitionRank{
				PlayerID:   ranking.PlayerID,
				FinalChips: ranking.Chips,
			})
			ce.emitCompetitionStateFinalPlayerRankEvent(ranking.PlayerID, ranking.Rank)
		}

		// 把名次由前至後重新排列 (Reverse competition.State.Rankings)
		for i, j := 0, len(ce.competition.State.Rankings)-1; i < j; i, j = i+1, j-1 {
			ce.competition.State.Rankings[i], ce.competition.State.Rankings[j] = ce.competition.State.Rankings[j], ce.competition.State.Rankings[i]
		}
	}
}

/*
ShouldClose 計算桌次是否已達到結束條件
  - 結束條件 1: 達到結束時間
  - 結束條件 2: 停止買入後且存活玩家小於最小開打數
*/
func (ce *competitionEngine) shouldCloseTable(tableStartAt int64, tableAlivePlayerCount int) bool {
	tableEndAt := time.Unix(tableStartAt, 0).Add(time.Second * time.Duration(ce.competition.Meta.MaxDuration)).Unix()
	return time.Now().Unix() > tableEndAt || (ce.competition.State.BlindState.IsFinalBuyInLevel() && tableAlivePlayerCount < ce.competition.Meta.TableMinPlayerCount)
}

func (ce *competitionEngine) updateTableBlind(tableID string) {
	level, ante, dealer, sb, bb := ce.competition.CurrentBlindData()
	if err := ce.tableManagerBackend.UpdateBlind(tableID, level, ante, dealer, sb, bb); err != nil {
		ce.emitErrorEvent("update blind", "", err)
	}
}

func (ce *competitionEngine) initBlind(meta CompetitionMeta) {
	options := &pokerblind.BlindOptions{
		ID:              meta.Blind.ID,
		InitialLevel:    meta.Blind.InitialLevel,
		FinalBuyInLevel: meta.Blind.FinalBuyInLevel,
		Levels: funk.Map(meta.Blind.Levels, func(bl BlindLevel) pokerblind.BlindLevel {
			dealer := int64(0)
			if meta.Rule == CompetitionRule_ShortDeck {
				dealer = (int64(meta.Blind.DealerBlindTime) - 1) * bl.BB
			}
			return pokerblind.BlindLevel{
				Level: bl.Level,
				Ante:  bl.Ante,
				Blind: pokerface.BlindSetting{
					Dealer: dealer,
					SB:     bl.SB,
					BB:     bl.BB,
				},
				Duration: bl.Duration,
			}
		}).([]pokerblind.BlindLevel),
	}
	ce.blind.ApplyOptions(options)
	ce.blind.OnBlindStateUpdated(func(bs *pokerblind.BlindState) {
		if ce.competition.State.Status == CompetitionStateStatus_End {
			return
		}

		ce.competition.State.BlindState.CurrentLevelIndex = bs.Status.CurrentLevelIndex

		// 更新賽事狀態: 停止買入
		if ce.competition.State.BlindState.IsFinalBuyInLevel() {
			ce.competition.State.Status = CompetitionStateStatus_StoppedBuyIn
			ce.emitEvent("Final BuyIn", "")

			// MTT 在停止買入階段，停止拆併桌機制
			if ce.competition.Meta.Mode == CompetitionMode_MTT {
				ce.match.DisableRegistration()
			}
		}

		for _, table := range ce.competition.State.Tables {
			ce.updateTableBlind(table.ID)
		}

		ce.emitCompetitionStateEvent(CompetitionStateEvent_BlindUpdated)
	})
	ce.blind.OnErrorUpdated(func(bs *pokerblind.BlindState, err error) {
		ce.emitErrorEvent("Blind Update Error", "", err)
	})
}
