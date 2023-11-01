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
		CurrentSeat:           UnsetValue,
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

func (ce *competitionEngine) UpdateReserveTablePlayerState(tableID string, playerState *pokertable.TablePlayerState) {
	// 更新玩家狀態
	playerCache, exist := ce.getPlayerCache(ce.competition.ID, playerState.PlayerID)
	if !exist {
		return
	}

	ce.competition.State.Players[playerCache.PlayerIdx].CurrentSeat = playerState.Seat
	ce.competition.State.Players[playerCache.PlayerIdx].CurrentTableID = tableID
	ce.competition.State.Players[playerCache.PlayerIdx].Status = CompetitionPlayerStatus_Playing
	ce.emitPlayerEvent("[UpdateReserveTablePlayerState] player table seat updated", ce.competition.State.Players[playerCache.PlayerIdx])
	// ce.emitEvent(fmt.Sprintf("Player (%s) table seat updated to %d", playerState.PlayerID, playerState.Seat), playerState.PlayerID)
}

func (ce *competitionEngine) UpdateTable(table *pokertable.Table) {
	tableIdx := ce.competition.FindTableIdx(func(t *pokertable.Table) bool {
		return table.ID == t.ID
	})
	if tableIdx == UnsetValue {
		return
	}

	if ce.isEndStatus() {
		fmt.Println("[DEBUG#UpdateTable] status is end, no need to update table. Status:", string(ce.competition.State.Status))
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
	shouldReopenGame := false
	readyPlayersCount := 0
	for _, p := range table.State.PlayerStates {
		if p.IsIn && p.Bankroll > 0 {
			readyPlayersCount++
		}
	}

	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT:
		if ce.shouldCloseCTTable(table.State.StartAt, len(table.AlivePlayers())) {
			ce.closeCompetitionTable(table, tableIdx)
			return
		}

		shouldReopenGame = ce.competition.State.Status == CompetitionStateStatus_DelayedBuyIn && readyPlayersCount >= ce.competition.Meta.TableMinPlayerCount
	case CompetitionMode_MTT:
		shouldReopenGame = readyPlayersCount >= ce.competition.Meta.TableMinPlayerCount
	}

	// re-open game
	if shouldReopenGame && !ce.competition.IsBreaking() {
		if err := ce.tableManagerBackend.TableGameOpen(table.ID); err != nil {
			ce.emitErrorEvent("Game Reopen", "", err)
			return
		}
		ce.emitEvent("Game Reopen:", "")
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
				ce.emitEvent(fmt.Sprintf("[addCompetitionTable] add existing player to (%s)", table.ID), ce.competition.State.Players[i].PlayerID)
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
			ce.emitEvent(fmt.Sprintf("[addCompetitionTable] add new player to (%s)", table.ID), playerID)
		}
		ce.competition.State.Players = append(ce.competition.State.Players, newPlayers...)
	}

	// init breakingPauseResumeStates
	ce.breakingPauseResumeStates[table.ID] = make(map[int]bool)
	for idx, bl := range ce.competition.Meta.Blind.Levels {
		if bl.Level == -1 {
			ce.breakingPauseResumeStates[table.ID][idx] = false
		}
	}

	ce.emitEvent("[addCompetitionTable]", "")

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

	// clean data
	delete(ce.breakingPauseResumeStates, table.ID)

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
	knockoutPlayerIDs := ce.handleTableKnockoutPlayers(table)

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

		// 中場休息處理
		ce.handleBreaking(table.ID, tableIdx)

		// 結束桌
		if ce.shouldCloseCTTable(table.State.StartAt, len(table.AlivePlayers())) {
			if err := ce.tableManagerBackend.CloseTable(table.ID); err != nil {
				ce.emitErrorEvent("Table Settlement Knockout Players -> CloseTable", "", err)
			}
		}
	case CompetitionMode_MTT:
		zeroChipPlayerIDs := make([]string, 0)
		alivePlayerIDs := make([]string, 0)
		leftPlayerSeats := make(map[int]string)
		for _, p := range table.State.PlayerStates {
			if p.Bankroll <= 0 {
				zeroChipPlayerIDs = append(zeroChipPlayerIDs, p.PlayerID)
				leftPlayerSeats[p.Seat] = "left"
			} else {
				alivePlayerIDs = append(alivePlayerIDs, p.PlayerID)
			}
		}

		if len(zeroChipPlayerIDs) > 0 {
			if err := ce.tableManagerBackend.PlayersLeave(table.ID, zeroChipPlayerIDs); err != nil {
				ce.emitErrorEvent(fmt.Sprintf("[%s][%d] Clean 0 chip Players -> PlayersLeave", table.ID, table.State.GameCount), strings.Join(zeroChipPlayerIDs, ","), err)
			}
		}

		// 拆併桌更新桌次狀態
		// Preparing seat changes
		if table.State.SeatChanges != nil {
			sc := match.NewSeatChanges()
			sc.Dealer = table.State.SeatChanges.NewDealer
			sc.SB = table.State.SeatChanges.NewSB
			sc.BB = table.State.SeatChanges.NewBB
			sc.Seats = leftPlayerSeats
			if err := ce.matchTableBackend.UpdateTable(table.ID, sc); err != nil {
				ce.emitErrorEvent(fmt.Sprintf("[%s][%d] MTT Match Update Table SeatChanges", table.ID, table.State.GameCount), "", err)
			} else {
				fmt.Printf("---------- [c: %s][t: %s] 第 (%d) 手結算, NewDealer: %d, NewSB: %d, NewBB: %d, leftPlayerSeats: %+v ----------\n",
					ce.competition.ID,
					table.ID,
					table.State.GameCount,
					sc.Dealer,
					sc.SB,
					sc.BB,
					sc.Seats,
				)
				ce.match.PrintTables()
			}
		} else {
			ce.emitErrorEvent(fmt.Sprintf("[c: %s][t: %s] MTT Match Update Table SeatChanges is nil", ce.competition.ID, table.ID), "", errors.New("nil seat change state is not allowed when settling mtt table"))
		}

		fmt.Printf("---------- [c: %s][t: %s] 第 (%d) 手結算, 停止買入: %+v, [離開 %d 人 (%s), 活著: %d 人 (%s)] ----------\n",
			ce.competition.ID,
			table.ID,
			table.State.GameCount,
			ce.competition.State.BlindState.IsStopBuyIn(),
			len(zeroChipPlayerIDs),
			strings.Join(zeroChipPlayerIDs, ","),
			len(alivePlayerIDs),
			strings.Join(alivePlayerIDs, ","),
		)

		if err := timebank.NewTimeBank().NewTask(time.Second*3, func(isCancelled bool) {
			if isCancelled {
				return
			}

			// 中場休息處理
			ce.handleBreaking(table.ID, tableIdx)

			// 結束賽事處理
			if ce.competition.State.BlindState.IsStopBuyIn() && len(alivePlayerIDs) == 1 && len(ce.competition.State.Tables) == 1 {
				ce.CloseCompetition(CompetitionStateStatus_End)
			}
		}); err != nil {
			ce.emitErrorEvent("error next stage after settle competition table", "", err)
		}
	}
}

func (ce *competitionEngine) handleBreaking(tableID string, tableIdx int) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	fmt.Println("[DEBUG#handleBreaking] Current Blind:", ce.blind.GetState().CurrentLevel())
	if !ce.competition.IsBreaking() {
		fmt.Println("[DEBUG#handleBreaking] is not breaking")
		return
	}

	// check breakingPauseResumeStates
	if _, exist := ce.breakingPauseResumeStates[tableID]; !exist {
		ce.breakingPauseResumeStates[tableID] = make(map[int]bool)
	}
	if _, exist := ce.breakingPauseResumeStates[tableID][ce.competition.State.BlindState.CurrentLevelIndex]; !exist {
		ce.breakingPauseResumeStates[tableID][ce.competition.State.BlindState.CurrentLevelIndex] = false
	} else {
		fmt.Println("[DEBUG#handleBreaking] already handle breaking & start timer")
		return
	}

	// already resume table games from breaking
	if ce.breakingPauseResumeStates[tableID][ce.competition.State.BlindState.CurrentLevelIndex] {
		fmt.Println("[DEBUG#handleBreaking] already resume table games from breaking")
		return
	}

	// reopen table game
	endAt := ce.competition.State.BlindState.EndAts[ce.competition.State.BlindState.CurrentLevelIndex] + 1
	fmt.Println("[DEBUG#handleBreaking] break pause at:", time.Now())
	fmt.Println("[DEBUG#handleBreaking] break pause stop & reopen game at:", time.Unix(endAt, 0))
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
			fmt.Println("[DEBUG#handleBreaking] not reopen since competition status is:", ce.competition.State.Status)
			return
		}

		if ce.breakingPauseResumeStates[tableID][ce.competition.State.BlindState.CurrentLevelIndex] {
			fmt.Println("[DEBUG#handleBreaking] not reopen since already resume table games from breaking")
			return
		}

		if len(ce.competition.State.Tables) > tableIdx {
			t := ce.competition.State.Tables[tableIdx]

			autoOpenGame := t.State.Status == pokertable.TableStateStatus_TablePausing && len(t.AlivePlayers()) >= t.Meta.TableMinPlayerCount
			if !autoOpenGame {
				fmt.Printf("[DEBUG#handleBreaking] not autoOpenGame. table status: %s, AlivePlayers: %d, TableMinPlayerCount: %d\n", t.State.Status, len(t.AlivePlayers()), t.Meta.TableMinPlayerCount)
				return
			}
			if err := ce.tableManagerBackend.TableGameOpen(tableID); err != nil {
				ce.emitErrorEvent("resume game from breaking & auto open next game", "", err)
			}

			ce.breakingPauseResumeStates[tableID][ce.competition.State.BlindState.CurrentLevelIndex] = true
			fmt.Println("[DEBUG#handleBreaking] reopen success")
		} else {
			fmt.Println("[DEBUG#handleBreaking] not find table at index:", tableIdx)
		}
	}); err != nil {
		ce.emitErrorEvent("new resume game task from breaking", "", err)
		return
	}
}

func (ce *competitionEngine) handleTableKnockoutPlayers(table *pokertable.Table) []string {
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
		ce.competition.State.Players[playerCache.PlayerIdx].CurrentSeat = UnsetValue
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
	ce.mu.Lock()
	defer ce.mu.Unlock()

	if ce.competition.State.BlindState.IsStopBuyIn() {
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
				ce.competition.State.Players[playerCache.PlayerIdx].Status = CompetitionPlayerStatus_ReBuyWaiting
				ce.competition.State.Players[playerCache.PlayerIdx].IsReBuying = true
				ce.competition.State.Players[playerCache.PlayerIdx].ReBuyEndAt = reBuyEndAt
				if ce.competition.Meta.Mode == CompetitionMode_MTT {
					ce.competition.State.Players[playerCache.PlayerIdx].CurrentSeat = UnsetValue
					ce.competition.State.Players[playerCache.PlayerIdx].ReBuyEndAt = UnsetValue
				}
				ce.emitPlayerEvent("re-buying", ce.competition.State.Players[playerCache.PlayerIdx])
				reBuyPlayerIDs = append(reBuyPlayerIDs, player.PlayerID)
			}
		}
	}

	if len(reBuyPlayerIDs) > 0 && ce.competition.Meta.Mode == CompetitionMode_CT {
		// AutoKnockout Player (When ReBuyEnd Time is reached)
		reBuyEndAtTime := time.Unix(reBuyEndAt, 0)

		for _, reBuyPlayerID := range reBuyPlayerIDs {
			if _, exist := ce.reBuyTimerStates[reBuyPlayerID]; !exist {
				ce.reBuyTimerStates[reBuyPlayerID] = timebank.NewTimeBank()
			}

			ce.reBuyTimerStates[reBuyPlayerID].Cancel()
			if err := ce.reBuyTimerStates[reBuyPlayerID].NewTaskWithDeadline(reBuyEndAtTime, func(isCancelled bool) {
				if isCancelled {
					return
				}

				playerCache, exist := ce.getPlayerCache(ce.competition.ID, reBuyPlayerID)
				if !exist {
					fmt.Println("[playerID] is not in the cache")
					return
				}

				if ce.competition.State.Players[playerCache.PlayerIdx].Chips > 0 {
					return
				}

				// 玩家已經被淘汰了 (停止買入階段觸發淘汰)
				if ce.competition.State.Players[playerCache.PlayerIdx].Status == CompetitionPlayerStatus_Knockout {
					return
				}

				ce.competition.State.Players[playerCache.PlayerIdx].Status = CompetitionPlayerStatus_ReBuyWaiting
				ce.competition.State.Players[playerCache.PlayerIdx].IsReBuying = false
				ce.competition.State.Players[playerCache.PlayerIdx].ReBuyEndAt = UnsetValue
				ce.competition.State.Players[playerCache.PlayerIdx].CurrentSeat = UnsetValue
				ce.emitPlayerEvent("re buy leave", ce.competition.State.Players[playerCache.PlayerIdx])
				ce.emitEvent("re buy leave", reBuyPlayerID)

				if err := ce.tableManagerBackend.PlayersLeave(table.ID, []string{reBuyPlayerID}); err != nil {
					ce.emitErrorEvent("re buy Knockout Players -> PlayersLeave", reBuyPlayerID, err)
				}
			}); err != nil {
				ce.emitErrorEvent("Players ReBuy Add Timer", "", err)
				continue
			}
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
shouldCloseCTTable CT 計算桌次是否已達到結束條件
  - 結束條件 1: 達到結束時間
  - 結束條件 2: 停止買入後且存活玩家小於最小開打數
*/
func (ce *competitionEngine) shouldCloseCTTable(tableStartAt int64, tableAlivePlayerCount int) bool {
	if ce.competition.Meta.Mode != CompetitionMode_CT {
		return false
	}

	tableEndAt := time.Unix(tableStartAt, 0).Add(time.Second * time.Duration(ce.competition.Meta.MaxDuration)).Unix()
	return time.Now().Unix() > tableEndAt || (ce.competition.State.BlindState.IsStopBuyIn() && tableAlivePlayerCount < ce.competition.Meta.TableMinPlayerCount)
}

func (ce *competitionEngine) updateTableBlind(tableID string) {
	level, ante, dealer, sb, bb := ce.competition.CurrentBlindData()
	if err := ce.tableManagerBackend.UpdateBlind(tableID, level, ante, dealer, sb, bb); err != nil {
		ce.emitErrorEvent("update blind", "", err)
	}
}

func (ce *competitionEngine) isEndStatus() bool {
	endStatuses := []CompetitionStateStatus{
		CompetitionStateStatus_End,
		CompetitionStateStatus_AutoEnd,
		CompetitionStateStatus_ForceEnd,
	}
	return funk.Contains(endStatuses, ce.competition.State.Status)
}

func (ce *competitionEngine) initBlind(meta CompetitionMeta) {
	options := &pokerblind.BlindOptions{
		ID:                   meta.Blind.ID,
		InitialLevel:         meta.Blind.InitialLevel,
		FinalBuyInLevelIndex: meta.Blind.FinalBuyInLevelIndex,
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
		if ce.isEndStatus() {
			return
		}

		ce.competition.State.BlindState.CurrentLevelIndex = bs.Status.CurrentLevelIndex
		fmt.Println("[DEBUG#initBlind] BlindState.CurrentLevelIndex:", ce.competition.State.BlindState.CurrentLevelIndex)
		for idx, table := range ce.competition.State.Tables {
			ce.updateTableBlind(table.ID)
			ce.handleBreaking(table.ID, idx)
		}

		ce.emitCompetitionStateEvent(CompetitionStateEvent_BlindUpdated) // change CurrentLevelIndex
		ce.emitEvent("Blind CurrentLevelIndex Update", "")

		// 更新賽事狀態: 停止買入
		if ce.competition.State.BlindState.IsStopBuyIn() {
			if ce.competition.State.Status != CompetitionStateStatus_StoppedBuyIn {
				ce.competition.State.Status = CompetitionStateStatus_StoppedBuyIn

				// MTT 在停止買入階段，停止拆併桌機制
				if ce.competition.Meta.Mode == CompetitionMode_MTT {
					ce.match.DisableRegistration()
				}

				// 淘汰沒資格玩家
				knockoutPlayerRankings := ce.GetSortedStopBuyInKnockoutPlayerRankings()
				for idx, knockoutPlayerID := range knockoutPlayerRankings {
					playerCache, exist := ce.getPlayerCache(ce.competition.ID, knockoutPlayerID)
					if !exist {
						continue
					}

					ce.competition.State.Players[playerCache.PlayerIdx].Status = CompetitionPlayerStatus_Knockout
					ce.competition.State.Players[playerCache.PlayerIdx].IsReBuying = false
					ce.competition.State.Players[playerCache.PlayerIdx].ReBuyEndAt = UnsetValue
					ce.competition.State.Players[playerCache.PlayerIdx].CurrentSeat = UnsetValue
					ce.emitPlayerEvent("Stopped BuyIn Knockout Players", ce.competition.State.Players[playerCache.PlayerIdx])

					// 玩家離座 (CT only), 因為 MTT 在結算沒籌碼時就已經離開該桌次了
					if ce.competition.Meta.Mode == CompetitionMode_CT {
						if len(ce.competition.State.Tables) > 0 {
							if _, exist := ce.reBuyTimerStates[knockoutPlayerID]; exist {
								ce.reBuyTimerStates[knockoutPlayerID].Cancel()
							}
							if err := ce.tableManagerBackend.PlayersLeave(ce.competition.State.Tables[0].ID, []string{knockoutPlayerID}); err != nil {
								ce.emitErrorEvent("Stopped BuyIn Knockout Players -> PlayersLeave", knockoutPlayerID, err)
							}
						}
					}

					// 更新賽事排名
					ce.competition.State.Rankings = append(ce.competition.State.Rankings, &CompetitionRank{
						PlayerID:   knockoutPlayerID,
						FinalChips: 0,
					})
					rank := ce.competition.PlayingPlayerCount() + (len(knockoutPlayerRankings) - idx)
					ce.emitCompetitionStateFinalPlayerRankEvent(knockoutPlayerID, rank)
				}

				ce.emitEvent("Stopped BuyIn Knockout Players", "")
				ce.emitCompetitionStateEvent(CompetitionStateEvent_KnockoutPlayers)
				ce.emitCompetitionStateEvent(CompetitionStateEvent_BlindUpdated) // change Status

				if ce.competition.Meta.Mode == CompetitionMode_CT && len(ce.competition.State.Tables) > 0 && len(ce.competition.State.Tables[0].AlivePlayers()) < 2 {
					if err := ce.tableManagerBackend.CloseTable(ce.competition.State.Tables[0].ID); err != nil {
						ce.emitErrorEvent("Stopped BuyIn auto close -> CloseTable", "", err)
					}
				}
			}
		}
	})
	ce.blind.OnErrorUpdated(func(bs *pokerblind.BlindState, err error) {
		ce.emitErrorEvent("Blind Update Error", "", err)
	})
}
