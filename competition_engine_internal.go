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

	cp := ce.competition.State.Players[playerCache.PlayerIdx]
	cp.CurrentSeat = playerState.Seat
	cp.CurrentTableID = tableID
	cp.Status = CompetitionPlayerStatus_Playing
	ce.emitPlayerEvent("[UpdateReserveTablePlayerState] player table seat updated", cp)
	ce.emitEvent("[UpdateReserveTablePlayerState] player table reserved", cp.PlayerID)
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
		if !ce.canStartCT() {
			return
		}

		// auto start game if condition is reached
		if _, err := ce.StartCompetition(); err != nil {
			ce.emitErrorEvent("CT Auto StartCompetition", "", err)
			return
		}

		ce.updateTableBlind(table.ID)

		if err := ce.tableManagerBackend.StartTableGame(table.ID); err != nil {
			ce.emitErrorEvent("CT Auto StartTableGame", "", err)
			return
		}
	case CompetitionMode_Cash:
		if !ce.canStartCash() {
			return
		}

		// auto start game if condition is reached
		if _, err := ce.StartCompetition(); err != nil {
			ce.emitErrorEvent("CT Auto StartCompetition", "", err)
			return
		}

		ce.updateTableBlind(table.ID)

		if err := ce.tableManagerBackend.StartTableGame(table.ID); err != nil {
			ce.emitErrorEvent("Cash Auto StartTableGame", "", err)
			return
		}
	}
}

func (ce *competitionEngine) updatePauseCompetition(table *pokertable.Table, tableIdx int) {
	shouldReOpenGame := false
	readyPlayersCount := 0
	alivePlayerCount := 0
	for _, p := range table.State.PlayerStates {
		if p.IsIn && p.Bankroll > 0 {
			readyPlayersCount++
		}

		if p.Bankroll > 0 {
			alivePlayerCount++
		}
	}

	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT:
		if ce.shouldCloseCTTable(table.State.StartAt, len(table.AlivePlayers())) {
			ce.closeCompetitionTable(table, tableIdx)
			return
		}

		shouldReOpenGame = ce.competition.State.Status == CompetitionStateStatus_DelayedBuyIn && readyPlayersCount >= ce.competition.Meta.TableMinPlayerCount

	case CompetitionMode_Cash:
		shouldReOpenGame = readyPlayersCount >= ce.competition.Meta.TableMinPlayerCount

	case CompetitionMode_MTT:
		shouldReOpenGame = readyPlayersCount >= ce.competition.Meta.TableMinPlayerCount
	}

	// re-open game
	if shouldReOpenGame && !ce.competition.IsBreaking() {
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
	fmt.Printf("[DEBUG#addCompetitionTable] create table: %s, total tables: %d\n", table.ID, len(ce.competition.State.Tables)+1)
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
				player.Chips = bankroll
				player.Status = playerStatus
				ce.emitPlayerEvent("[addCompetitionTable] existing player", player)
				ce.emitEvent(fmt.Sprintf("[addCompetitionTable] add existing player to (%s)", table.ID), player.PlayerID)
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
	fmt.Printf("[DEBUG#closeCompetitionTable] close table: %s, total tables: %d, alive players: %d\n", table.ID, len(ce.competition.State.Tables)-1, ce.competition.PlayingPlayerCount())
	ce.onTableClosed(table)

	// clean data
	delete(ce.breakingPauseResumeStates, table.ID)

	// competition close table
	ce.deleteTable(tableIdx)
	ce.emitEvent("closeCompetitionTable", "")
	ce.emitCompetitionStateEvent(CompetitionStateEvent_TableUpdated)

	if len(ce.competition.State.Tables) == 0 && !ce.isEndStatus() {
		ce.CloseCompetition(CompetitionStateStatus_End)
	}
}

/*
settleCompetitionTable 桌次結算
  - 適用時機: 每手結束
*/
func (ce *competitionEngine) settleCompetitionTable(table *pokertable.Table, tableIdx int) {
	gameSettledRecordID := fmt.Sprintf("%s.%d", table.ID, table.State.GameCount)
	ce.mu.Lock()
	if isGameSettled, ok := ce.gameSettledRecords.Load(gameSettledRecordID); ok && isGameSettled.(bool) {
		ce.mu.Unlock()
		return
	}
	ce.gameSettledRecords.Store(gameSettledRecordID, true)
	ce.mu.Unlock()

	// 更新玩家相關賽事數據
	ce.updatePlayerCompetitionTaleRecords(table)

	// 根據是否達到停止買入做處理
	ce.handleReBuy(table)

	// 處理淘汰玩家
	knockoutPlayerIDs := ce.handleTableKnockoutPlayers(table)

	// 桌次處理
	shouldCloseMTTCompetition := false
	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT:
		ce.handleCTTableSettlement(knockoutPlayerIDs, table)
	case CompetitionMode_Cash:
		ce.handleCashTableSettlement(table)
	case CompetitionMode_MTT:
		shouldCloseMTTCompetition = ce.handleMTTTableSettlement(table)
	}

	// 事件更新
	ce.emitEvent("Table Settlement", "")
	ce.emitCompetitionStateEvent(CompetitionStateEvent_TableGameSettled)

	// 賽數結算條件達成處理
	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT:
		// 結束桌
		if ce.shouldCloseCTTable(table.State.StartAt, len(table.AlivePlayers())) {
			if err := ce.tableManagerBackend.CloseTable(table.ID); err != nil {
				ce.emitErrorEvent("Table Settlement -> Close CT Table", "", err)
			}
		}
	case CompetitionMode_Cash:
		// 判斷是否要關閉賽事 (現金桌)
		if ce.shouldCloseCashTable(table.State.StartAt) {
			if err := ce.tableManagerBackend.CloseTable(table.ID); err != nil {
				ce.emitErrorEvent("Table Settlement -> Close Cash Table", "", err)
			}
		}
	case CompetitionMode_MTT:
		if shouldCloseMTTCompetition {
			if err := timebank.NewTimeBank().NewTask(time.Second*3, func(isCancelled bool) {
				if isCancelled {
					return
				}

				// 結束賽事處理
				ce.CloseCompetition(CompetitionStateStatus_End)
			}); err != nil {
				ce.emitErrorEvent("error next stage after settle competition table", "", err)
			}
		}
	}
}

func (ce *competitionEngine) handleCTTableSettlement(knockoutPlayerIDs []string, table *pokertable.Table) {
	// TableEngine Player Leave
	if len(knockoutPlayerIDs) > 0 {
		if err := ce.tableManagerBackend.PlayersLeave(table.ID, knockoutPlayerIDs); err != nil {
			ce.emitErrorEvent("Table Settlement Knockout Players -> PlayersLeave", strings.Join(knockoutPlayerIDs, ","), err)
		}
	}

	// 中場休息處理
	ce.handleBreaking(table.ID)
}

func (ce *competitionEngine) handleCashTableSettlement(table *pokertable.Table) {
	// Cash Out 處理
	leavePlayerIDs := make([]string, 0)
	leavePlayerIndexes := make(map[string]int)
	for idx, cp := range ce.competition.State.Players {
		if cp.Status == CompetitionPlayerStatus_CashLeaving {
			leavePlayerIDs = append(leavePlayerIDs, cp.PlayerID)
			leavePlayerIndexes[cp.PlayerID] = idx
		}
	}

	if len(leavePlayerIDs) > 0 {
		ce.handleCashOut(table.ID, leavePlayerIndexes, leavePlayerIDs)
	}

	// 中場休息處理
	ce.handleBreaking(table.ID)
}

func (ce *competitionEngine) handleMTTTableSettlement(table *pokertable.Table) bool {
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

	// 更新晉級條件 (停買後更新)
	shouldCloseCompetition := false
	if ce.competition.State.AdvanceState.Status == CompetitionAdvanceStatus_Updating {
		// 晉級計算
		ce.competition.State.AdvanceState.TotalTables = len(ce.competition.State.Tables)
		shouldAdvancePauseTableGame := false

		switch ce.competition.Meta.AdvanceSetting.Rule {
		case CompetitionAdvanceRule_BlindLevel:
			if ce.competition.CurrentBlindLevel().Level >= ce.competition.Meta.AdvanceSetting.BlindLevel {
				shouldAdvancePauseTableGame = true
			}
		case CompetitionAdvanceRule_PlayerCount:
			possibleAdvancePlayerCount := ce.competition.PlayingPlayerCount()
			finalAdvancePlayerCount := ce.competition.Meta.AdvanceSetting.PlayerCount
			// 如果要取晉級人數 n 人 (finalAdvancePlayerCount)，必須要活著的人數 (possibleAdvancePlayerCount) 小於 n 人才暫停該桌
			// TODO: 如果 M 取 N 剛好整除，要是 possibleAdvancePlayerCount <= finalAdvancePlayerCount，沒整除才是 possibleAdvancePlayerCount < finalAdvancePlayerCount
			if possibleAdvancePlayerCount < finalAdvancePlayerCount {
				shouldAdvancePauseTableGame = true
			}
		}

		if shouldAdvancePauseTableGame {
			if err := ce.tableManagerBackend.PauseTable(table.ID); err != nil {
				ce.emitErrorEvent(fmt.Sprintf("[%s][%d] Advance Pause Table", table.ID, table.State.GameCount), "", err)
			} else {
				ce.competition.State.AdvanceState.UpdatedTables++
				ce.competition.State.AdvanceState.UpdatedTableIDs = append(ce.competition.State.AdvanceState.UpdatedTableIDs, table.ID)

				// 晉級條件達成: 結束賽事
				if ce.competition.State.AdvanceState.TotalTables == ce.competition.State.AdvanceState.UpdatedTables {
					ce.competition.State.AdvanceState.Status = CompetitionAdvanceStatus_End
					shouldCloseCompetition = true
				}
			}
		} else {
			if table.State.SeatChanges != nil {
				sc := match.NewSeatChanges()
				sc.Dealer = table.State.SeatChanges.NewDealer
				sc.SB = table.State.SeatChanges.NewSB
				sc.BB = table.State.SeatChanges.NewBB
				sc.Seats = leftPlayerSeats
				if err := ce.matchTableBackend.UpdateTable(table.ID, sc); err != nil {
					ce.emitErrorEvent(fmt.Sprintf("[%s][%d] Advance MTT Match Update Table SeatChanges", table.ID, table.State.GameCount), "", err)
				}
			} else {
				ce.emitErrorEvent(fmt.Sprintf("[c: %s][t: %s] Advance MTT Match Update Table SeatChanges is nil", ce.competition.ID, table.ID), "", errors.New("nil seat change state is not allowed when settling mtt table"))
			}
		}
	} else {
		// 無晉級計算
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
			}
		} else {
			ce.emitErrorEvent(fmt.Sprintf("[c: %s][t: %s] MTT Match Update Table SeatChanges is nil", ce.competition.ID, table.ID), "", errors.New("nil seat change state is not allowed when settling mtt table"))
		}

		// 中場休息處理
		ce.handleBreaking(table.ID)

		// 判斷是否要關閉賽事
		shouldCloseCompetition = !ce.isEndStatus() && ce.competition.State.BlindState.IsStopBuyIn() && len(alivePlayerIDs) == 1 && len(ce.competition.State.Tables) == 1
	}

	return shouldCloseCompetition
}

func (ce *competitionEngine) handleCashOut(tableID string, leavePlayerIndexes map[string]int, leavePlayerIDs []string) {
	// TableEngine Player Leave
	if err := ce.tableManagerBackend.PlayersLeave(tableID, leavePlayerIDs); err != nil {
		ce.emitErrorEvent("handleCashOut -> PlayersLeave", strings.Join(leavePlayerIDs, ","), err)
	}

	// Cash Out
	for _, leavePlayerID := range leavePlayerIDs {
		if playerIdx, exist := leavePlayerIndexes[leavePlayerID]; exist {
			// delete timer
			if _, reBuyTimerStateExist := ce.reBuyTimerStates[leavePlayerID]; reBuyTimerStateExist {
				ce.reBuyTimerStates[leavePlayerID].Cancel()
			}
			delete(ce.reBuyTimerStates, leavePlayerID)

			ce.onCompetitionPlayerCashOut(ce.competition.ID, ce.competition.State.Players[playerIdx])
			ce.deletePlayerCache(ce.competition.ID, leavePlayerID)
		}
	}

	// keep players that are cashing out
	newPlayers := make([]*CompetitionPlayer, 0)
	for _, cp := range ce.competition.State.Players {
		if _, exist := leavePlayerIndexes[cp.PlayerID]; !exist {
			newPlayers = append(newPlayers, cp)
		}
	}
	ce.competition.State.Players = newPlayers

	// Emit Event
	ce.emitCompetitionStateEvent(CompetitionStateEvent_CashOutPlayers)
}

func (ce *competitionEngine) handleBreaking(tableID string) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	if !ce.competition.IsBreaking() {
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
		return
	}

	// reopen table game
	endAt := ce.competition.State.BlindState.EndAts[ce.competition.State.BlindState.CurrentLevelIndex] + 1
	if err := timebank.NewTimeBank().NewTaskWithDeadline(time.Unix(endAt, 0), func(isCancelled bool) {
		if isCancelled {
			return
		}

		if ce.isEndStatus() {
			fmt.Println("[DEBUG#handleBreaking] not reopen since competition status is:", ce.competition.State.Status)
			return
		}

		if ce.breakingPauseResumeStates[tableID][ce.competition.State.BlindState.CurrentLevelIndex] {
			return
		}

		tableIdx := ce.competition.FindTableIdx(func(t *pokertable.Table) bool {
			return t.ID == tableID
		})
		if len(ce.competition.State.Tables) > tableIdx && tableIdx >= 0 {
			t := ce.competition.State.Tables[tableIdx]

			autoOpenGame := t.State.Status == pokertable.TableStateStatus_TablePausing && len(t.AlivePlayers()) >= t.Meta.TableMinPlayerCount
			if !autoOpenGame {
				return
			}

			if err := ce.tableManagerBackend.TableGameOpen(tableID); err != nil {
				ce.emitErrorEvent("resume game from breaking & auto open next game", "", err)
			} else {
				ce.breakingPauseResumeStates[tableID][ce.competition.State.BlindState.CurrentLevelIndex] = true
			}
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

		cp := ce.competition.State.Players[playerCache.PlayerIdx]
		cp.Status = CompetitionPlayerStatus_Knockout
		cp.CurrentSeat = UnsetValue
		ce.emitPlayerEvent("table settlement knockout", cp)

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

		rebuyPlayerIdx := ce.competition.FindPlayerIdx(func(competitionPlayer *CompetitionPlayer) bool {
			return competitionPlayer.PlayerID == player.PlayerID
		})
		if rebuyPlayerIdx == UnsetValue {
			fmt.Printf("[handleReBuy#start] player (%s) is not in the competition\n", player.PlayerID)
			continue
		}

		cp := ce.competition.State.Players[rebuyPlayerIdx]
		if !cp.IsReBuying {
			if cp.ReBuyTimes < ce.competition.Meta.ReBuySetting.MaxTime {
				cp.Status = CompetitionPlayerStatus_ReBuyWaiting
				cp.IsReBuying = true
				cp.ReBuyEndAt = reBuyEndAt
				if ce.competition.Meta.Mode == CompetitionMode_MTT {
					cp.CurrentSeat = UnsetValue
					cp.ReBuyEndAt = UnsetValue
				}

				reBuyPlayerIDs = append(reBuyPlayerIDs, player.PlayerID)

				ce.emitPlayerEvent("re-buying", cp)
			}
		}
	}

	// CT/Cash 保留座位時間到後處理
	keepSeatModes := []CompetitionMode{
		CompetitionMode_CT,
		CompetitionMode_Cash,
	}
	if !funk.Contains(keepSeatModes, ce.competition.Meta.Mode) {
		return
	}

	reBuyEndAtTime := time.Unix(reBuyEndAt, 0)
	for _, reBuyPlayerID := range reBuyPlayerIDs {
		if _, exist := ce.reBuyTimerStates[reBuyPlayerID]; !exist {
			ce.reBuyTimerStates[reBuyPlayerID] = timebank.NewTimeBank()
		}

		ce.reBuyTimerStates[reBuyPlayerID].Cancel()
		if err := ce.reBuyTimerStates[reBuyPlayerID].NewTaskWithDeadline(reBuyEndAtTime, func(isCancelled bool) {
			if isCancelled {
				fmt.Printf("[handleReBuy#after] player (%s) rebuy timer is cancelled\n", reBuyPlayerID)
				return
			}

			rebuyPlayerIdx := ce.competition.FindPlayerIdx(func(competitionPlayer *CompetitionPlayer) bool {
				return competitionPlayer.PlayerID == reBuyPlayerID
			})
			if rebuyPlayerIdx == UnsetValue {
				fmt.Printf("[handleReBuy#after] player (%s) is not in the competition\n", reBuyPlayerID)
				return
			}

			cp := ce.competition.State.Players[rebuyPlayerIdx]
			if cp.Chips > 0 {
				fmt.Printf("[handleReBuy#after] player (%s) is already rebuy (%d) chips\n", reBuyPlayerID, cp.Chips)
				return
			}

			switch ce.competition.Meta.Mode {
			case CompetitionMode_CT:
				// 玩家已經被淘汰了 (停止買入階段觸發淘汰)
				if cp.Status == CompetitionPlayerStatus_Knockout {
					fmt.Printf("[handleReBuy#after] player (%s) is already knockout, status: %s\n", reBuyPlayerID, cp.Status)
					return
				}

				cp.Status = CompetitionPlayerStatus_ReBuyWaiting
				cp.IsReBuying = false
				cp.ReBuyEndAt = UnsetValue
				cp.CurrentSeat = UnsetValue
				ce.emitPlayerEvent("re buy leave", cp)
				ce.emitEvent("re buy leave", reBuyPlayerID)

				if err := ce.tableManagerBackend.PlayersLeave(table.ID, []string{reBuyPlayerID}); err != nil {
					ce.emitErrorEvent("re buy Knockout Players -> PlayersLeave", reBuyPlayerID, err)
				}

			case CompetitionMode_Cash:
				leavePlayerIDs := []string{cp.PlayerID}
				leavePlayerIndexes := map[string]int{cp.PlayerID: rebuyPlayerIdx}
				ce.handleCashOut(table.ID, leavePlayerIndexes, leavePlayerIDs)

			}
		}); err != nil {
			ce.emitErrorEvent("Players ReBuy Add Timer", "", err)
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

		playerIdx := ce.competition.FindPlayerIdx(func(competitionPlayer *CompetitionPlayer) bool {
			return competitionPlayer.PlayerID == player.PlayerID
		})
		if playerIdx == UnsetValue {
			fmt.Printf("[updatePlayerCompetitionTaleRecords#statistic] player (%s) is not in the competition\n", player.PlayerID)
			continue
		}

		cp := ce.competition.State.Players[playerIdx]
		cp.TotalGameCounts++
		if player.GameStatistics.IsFold {
			cp.TotalFoldTimes++
			switch player.GameStatistics.FoldRound {
			case pokertable.GameRound_Preflop:
				cp.TotalPreflopFoldTimes++
				gamePlayerPreflopFoldTimes++
			case pokertable.GameRound_Flop:
				cp.TotalFlopFoldTimes++
			case pokertable.GameRound_Turn:
				cp.TotalTurnFoldTimes++
			case pokertable.GameRound_River:
				cp.TotalRiverFoldTimes++
			}
		}
		cp.TotalActionTimes += player.GameStatistics.ActionTimes
		cp.TotalRaiseTimes += player.GameStatistics.RaiseTimes
		cp.TotalCallTimes += player.GameStatistics.CallTimes
		cp.TotalCheckTimes += player.GameStatistics.CheckTimes
	}

	// 更新贏家統計數據
	for _, playerResult := range table.State.GameState.Result.Players {
		if playerResult.Changed <= 0 {
			continue
		}

		winnerGameIdx := playerResult.Idx
		tablePlayerIdx := table.State.GamePlayerIndexes[winnerGameIdx]
		tablePlayer := table.State.PlayerStates[tablePlayerIdx]

		playerIdx := ce.competition.FindPlayerIdx(func(competitionPlayer *CompetitionPlayer) bool {
			return competitionPlayer.PlayerID == tablePlayer.PlayerID
		})
		if playerIdx == UnsetValue {
			fmt.Printf("[updatePlayerCompetitionTaleRecords#winner] player (%s) is not in the competition\n", tablePlayer.PlayerID)
			continue
		}

		cp := ce.competition.State.Players[playerIdx]
		cp.TotalProfitTimes++

		gs := table.State.GameState
		gsPlayer := gs.GetPlayer(winnerGameIdx)
		if gsPlayer.VPIP {
			cp.TotalVPIPTimes++
		}

		if table.State.CurrentBBSeat == tablePlayer.Seat && tablePlayer.GameStatistics.ActionTimes == 0 && gamePlayerPreflopFoldTimes == len(table.State.GamePlayerIndexes)-1 {
			cp.TotalWalkTimes++
		}

		if playerResult.Changed > cp.BestWinningPotChips {
			cp.BestWinningPotChips = playerResult.Changed
		}

		if gsPlayer.Combination.Power >= cp.BestWinningPower {
			cp.BestWinningPower = gsPlayer.Combination.Power
			cp.BestWinningCombo = gsPlayer.Combination.Cards
			cp.BestWinningType = gsPlayer.Combination.Type
		}
	}

	// 桌次結算: 更新玩家桌內即時排名 & 當前後手碼量(該手有參賽者會更新排名，若沒參賽者排名為 0)
	playerRankingData := ce.GetParticipatedPlayerTableRankingData(ce.competition.ID, table.State.PlayerStates, table.State.GamePlayerIndexes)
	for playerID, rankData := range playerRankingData {
		playerIdx := ce.competition.FindPlayerIdx(func(competitionPlayer *CompetitionPlayer) bool {
			return competitionPlayer.PlayerID == playerID
		})
		if playerIdx == UnsetValue {
			fmt.Printf("[updatePlayerCompetitionTaleRecords#table-settlement] player (%s) is not in the competition\n", playerID)
			continue
		}

		cp := ce.competition.State.Players[playerIdx]
		cp.Rank = rankData.Rank
		cp.Chips = rankData.Chips
		ce.emitPlayerEvent("table-settlement", cp)
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

/*
shouldCloseCashTable Cash 計算桌次是否已達到結束條件
  - 結束條件 1: 達到結束時間
*/
func (ce *competitionEngine) shouldCloseCashTable(tableStartAt int64) bool {
	if ce.competition.Meta.Mode != CompetitionMode_Cash {
		return false
	}

	tableEndAt := time.Unix(tableStartAt, 0).Add(time.Second * time.Duration(ce.competition.Meta.MaxDuration)).Unix()
	return time.Now().Unix() > tableEndAt
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
		for _, table := range ce.competition.State.Tables {
			ce.updateTableBlind(table.ID)
			ce.handleBreaking(table.ID)
		}

		ce.emitCompetitionStateEvent(CompetitionStateEvent_BlindUpdated) // change CurrentLevelIndex
		ce.emitEvent("Blind CurrentLevelIndex Update", "")

		// 更新賽事狀態: 停止買入
		if ce.competition.State.BlindState.IsStopBuyIn() {
			if ce.competition.State.Status != CompetitionStateStatus_StoppedBuyIn {
				// 處理晉級
				ce.initAdvancement()

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

					cp := ce.competition.State.Players[playerCache.PlayerIdx]
					cp.Status = CompetitionPlayerStatus_Knockout
					cp.IsReBuying = false
					cp.ReBuyEndAt = UnsetValue
					cp.CurrentSeat = UnsetValue
					ce.emitPlayerEvent("Stopped BuyIn Knockout Players", cp)

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

				// 事件通知
				ce.emitEvent("Stopped BuyIn Knockout Players", "")
				ce.emitCompetitionStateEvent(CompetitionStateEvent_KnockoutPlayers)
				ce.emitCompetitionStateEvent(CompetitionStateEvent_BlindUpdated) // change Status

				// 處理結束賽事
				tableEndConditions := len(ce.competition.State.Tables) == 0 || (len(ce.competition.State.Tables) == 1 && len(ce.competition.State.Tables[0].AlivePlayers()) < 2)
				shouldCloseCompetition := !ce.isEndStatus() && tableEndConditions
				switch ce.competition.Meta.Mode {
				case CompetitionMode_CT:
					if shouldCloseCompetition && len(ce.competition.State.Tables) == 1 {
						fmt.Println("Stopped BuyIn auto close -> CT CloseTable")
						if err := ce.tableManagerBackend.CloseTable(ce.competition.State.Tables[0].ID); err != nil {
							ce.emitErrorEvent("Stopped BuyIn auto close -> CT CloseTable", "", err)
						}
					}
				case CompetitionMode_MTT:
					if shouldCloseCompetition {
						fmt.Println("Stopped BuyIn auto close -> MTT CloseCompetition")
						if err := ce.CloseCompetition(CompetitionStateStatus_End); err != nil {
							ce.emitErrorEvent("Stopped BuyIn auto close -> MTT CloseCompetition", "", err)
						}
					}
				}
			}
		}
	})
	ce.blind.OnErrorUpdated(func(bs *pokerblind.BlindState, err error) {
		ce.emitErrorEvent("Blind Update Error", "", err)
	})
}

func (ce *competitionEngine) canStartCT() bool {
	if ce.competition.State.Status != CompetitionStateStatus_Registering {
		return false
	}

	currentPlayerCount := 0
	for _, table := range ce.competition.State.Tables {
		for _, player := range table.State.PlayerStates {
			if player.IsIn && player.Bankroll > 0 {
				currentPlayerCount++
			}
		}
	}

	if currentPlayerCount >= ce.competition.Meta.MinPlayerCount {
		// 開打條件一: 當賽局已經設定 StartAt (開打時間) & 現在時間已經大於等於開打時間且達到最小開桌人數
		if ce.competition.State.StartAt > 0 && time.Now().Unix() >= ce.competition.State.StartAt {
			return true
		}

		// 開打條件二: 賽局沒有設定 StartAt (開打時間) & 達到最小開桌人數
		if ce.competition.State.StartAt <= 0 {
			return true
		}
	}
	return false
}

func (ce *competitionEngine) canStartCash() bool {
	if ce.competition.State.Status != CompetitionStateStatus_Registering {
		return false
	}

	currentPlayerCount := 0
	for _, table := range ce.competition.State.Tables {
		for _, player := range table.State.PlayerStates {
			if player.IsIn && player.Bankroll > 0 {
				currentPlayerCount++
			}
		}
	}

	return currentPlayerCount >= ce.competition.Meta.MinPlayerCount
}

/*
initAdvancement 初始化晉級機制 (停止買入後)
*/
func (ce *competitionEngine) initAdvancement() {
	if ce.competition.Meta.Mode != CompetitionMode_MTT {
		return
	}

	validAdvanceRules := []CompetitionAdvanceRule{
		CompetitionAdvanceRule_PlayerCount,
		CompetitionAdvanceRule_BlindLevel,
	}
	if !funk.Contains(validAdvanceRules, ce.competition.Meta.AdvanceSetting.Rule) {
		return
	}

	if ce.competition.State.AdvanceState.Status != CompetitionAdvanceStatus_NotStart {
		return
	}

	if ce.competition.Meta.AdvanceSetting.Rule == CompetitionAdvanceRule_PlayerCount {
		ce.competition.Meta.AdvanceSetting.PlayerCount = ce.onAdvancePlayerCountUpdated(ce.competition.ID, ce.competition.State.Statistic.TotalBuyInCount)
	}

	ce.competition.State.AdvanceState.Status = CompetitionAdvanceStatus_Updating
	ce.competition.State.AdvanceState.TotalTables = len(ce.competition.State.Tables)
	ce.competition.State.AdvanceState.UpdatedTables = 0
}
