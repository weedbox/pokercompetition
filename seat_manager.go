package pokercompetition

import (
	"github.com/thoas/go-funk"
	"github.com/weedbox/pokertable"
	"github.com/weedbox/pokertablebalancer/psae"
)

func (ce *competitionEngine) activateSeatManager(competitionID string, meta CompetitionMeta) {
	if ce.seatManager == nil {
		return
	}

	g := psae.NewGame()
	g.MaxPlayersPerTable = meta.TableMaxSeatCount
	g.MinInitialPlayers = meta.TableMinPlayerCount
	ce.seatManager.RegisterCompetition(competitionID, g)
}

func (ce *competitionEngine) deactivateSeatManager(competitionID string) {
	if ce.seatManager == nil {
		return
	}

	ce.seatManager.UnregisterCompetition(competitionID)
}

func (ce *competitionEngine) seatManagerJoinPlayer(competitionID string, playerIDs []string) {
	if ce.seatManager == nil {
		return
	}

	c := ce.seatManager.GetCompetition(competitionID)

	for _, pId := range playerIDs {
		c.Join(pId)
	}
}

func (ce *competitionEngine) seatManagerUpdateTable(competitionID string, table *pokertable.Table, currentPlayerIDs []string) (bool, error) {
	if ce.seatManager == nil {
		return false, nil
	}

	c := ce.seatManager.GetCompetition(competitionID)

	ts, err := c.GetTableState(table.ID)
	if err != nil {
		return false, nil
	}

	// utgIndex 預設是 dealerIndex
	targetPlayerIdx := table.State.GamePlayerIndexes[0]
	if table.State.CurrentBBSeat != UnsetValue {
		for _, playerIdx := range table.State.GamePlayerIndexes {
			if funk.Contains(table.State.PlayerStates[playerIdx].Positions, pokertable.Position_BB) {
				targetPlayerIdx = playerIdx
				break
			}
		}

	}

	currUGSeat := table.State.PlayerStates[targetPlayerIdx].Seat + 1
	if currUGSeat == table.Meta.CompetitionMeta.TableMaxSeatCount {
		currUGSeat = 0
	}

	tableSeats := 0
	for _, playerIdx := range table.State.GamePlayerIndexes {
		tableSeats += 1 << table.State.PlayerStates[playerIdx].Seat
	}

	ts.LastGameID = table.State.GameState.GameID
	ts.AvailableSeats = ce.calcAvailableSeats(tableSeats, ts.TotalSeats, table.State.CurrentDealerSeat, currUGSeat)

	// 更新玩家列表
	ts.Players = make(map[string]*psae.Player)
	for _, pId := range currentPlayerIDs {
		ts.Players[pId] = &psae.Player{
			ID: pId,
		}
	}

	updated, err := c.UpdateTable(ts)
	if err != nil {
		return false, nil
	}

	isSuspended := false
	if updated.Status == psae.TableStatus_Broken {
		// 將被拆桌，必須停止繼續下一手
		isSuspended = true
	}

	return isSuspended, nil
}

func (ce *competitionEngine) seatManagerUpdateCompetitionStatus(competitionID string, isStoppedBuyIn bool) {
	if ce.seatManager == nil {
		return
	}

	c := ce.seatManager.GetCompetition(competitionID)

	if isStoppedBuyIn {
		c.DisallowRegistration()
	}
}

func (ce *competitionEngine) calcAvailableSeats(seats int, totalSeats int, dealerIndex int, utgIndex int) int {

	// 計算可入坐座位
	seatRange := make([]int, 0)
	if dealerIndex >= utgIndex {
		// 列舉 UTG 到 Dealer 之間的位置
		for i := utgIndex; i <= dealerIndex; i++ {
			seatRange = append(seatRange, i)
		}
	} else {

		// 列舉 UTG 到最後一個位置
		for i := utgIndex; i < totalSeats; i++ {
			seatRange = append(seatRange, i)
		}

		// 列舉第一個位置到 Dealer(Button) 位
		for i := 0; i <= dealerIndex; i++ {
			seatRange = append(seatRange, i)
		}
	}

	// 統計空位
	availableSeats := 0
	for _, seatIdx := range seatRange {
		if seats&(1<<seatIdx) == 0 {
			availableSeats++
		}
	}

	return availableSeats
}
