package actor

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/weedbox/pokerface"
	"github.com/weedbox/pokertable"
	"github.com/weedbox/timebank"
)

type PlayerStatus int32
type TableGameWagerActionUpdatedFunc func(tableID, gameID string, gameCount int, round, action string, chips int64)

const (
	PlayerStatus_Running PlayerStatus = iota
	PlayerStatus_Idle
	PlayerStatus_Suspend
)

type PlayerRunner struct {
	actor                         Actor
	actions                       Actions
	playerID                      string
	curGameID                     string
	lastGameStateTime             int64
	tableInfo                     *pokertable.Table
	timebank                      *timebank.TimeBank
	onAutoModeUpdated             func(string, bool) // tableID(string), isOn(bool)
	onTableStateUpdated           func(*pokertable.Table)
	onTableGameWagerActionUpdated TableGameWagerActionUpdatedFunc
	isEventSubscribed             bool

	// status
	status           PlayerStatus
	idleCount        int
	suspendThreshold int
}

func NewPlayerRunner(playerID string) *PlayerRunner {
	return &PlayerRunner{
		playerID:                      playerID,
		timebank:                      timebank.NewTimeBank(),
		status:                        PlayerStatus_Running,
		suspendThreshold:              2,
		onAutoModeUpdated:             func(string, bool) {},
		onTableStateUpdated:           func(*pokertable.Table) {},
		onTableGameWagerActionUpdated: func(string, string, int, string, string, int64) {},
		isEventSubscribed:             true,
	}
}

func (pr *PlayerRunner) SetActor(a Actor) {
	pr.actor = a
	pr.actions = NewActions(a, pr.playerID)
}

func (pr *PlayerRunner) SetEventSubscribed(isEventSubscribed bool) {
	pr.isEventSubscribed = isEventSubscribed
}

func (pr *PlayerRunner) UpdateTableState(table *pokertable.Table) error {

	// gs := table.State.GameState
	pr.tableInfo = table

	// Emit event
	// TODO: ask Fred if emit event unconditionally is okay
	var cloneTable pokertable.Table
	if encoded, err := json.Marshal(table); err == nil {
		if err := json.Unmarshal(encoded, &cloneTable); err != nil {
			cloneTable = *table
		}
	} else {
		cloneTable = *table
	}
	// hide certain game state fields for player when table game is playing
	if cloneTable.State.Status == pokertable.TableStateStatus_TableGamePlaying || cloneTable.State.Status == pokertable.TableStateStatus_TableGameSettled {
		gpIdx := table.GamePlayerIndex(pr.playerID)
		cloneTable.State.GameState.AsPlayer(gpIdx)
	}
	if pr.isEventSubscribed {
		pr.onTableStateUpdated(&cloneTable)
	}

	gs := cloneTable.State.GameState

	// The state remains unchanged or is outdated
	if gs != nil {

		// New game
		if gs.GameID != pr.curGameID {
			pr.curGameID = gs.GameID
		}

		//fmt.Println(br.lastGameStateTime, br.tableInfo.State.GameState.UpdatedAt)
		if pr.lastGameStateTime >= gs.UpdatedAt {
			fmt.Printf("[DEBUG#PlayerRunner#UpdateTableState] [1] No reaction required since lastGameStateTime >= currentGameStateTime. PlayerID: %s, TableID: %s\n", pr.playerID, table.ID)
			return nil
		}

		pr.lastGameStateTime = gs.UpdatedAt
	}

	// Check if you have been eliminated
	isEliminated := true
	for _, ps := range table.State.PlayerStates {
		if ps.PlayerID == pr.playerID {
			isEliminated = false
		}
	}

	if isEliminated {
		fmt.Printf("[DEBUG#PlayerRunner#UpdateTableState] [2] No reaction required since player (%s), table (%s) is eliminated.\n", pr.playerID, table.ID)
		return nil
	}

	// Update seat index
	gamePlayerIdx := table.GamePlayerIndex(pr.playerID)

	// // Emit event
	// pr.onTableStateUpdated(table)

	// Game is running right now
	switch table.State.Status {
	case pokertable.TableStateStatus_TableGamePlaying:

		// Somehow, this player is not in the game.
		// It probably has no chips already.
		if gamePlayerIdx == -1 {
			fmt.Printf("[DEBUG#PlayerRunner#UpdateTableState] [3] No reaction required since player (%s) gamePlayerIdx is -1 at table (%s) when table status is %s.\n", pr.playerID, table.ID, pokertable.TableStateStatus_TableGamePlaying)
			return nil
		}

		// Filtering private information fpr player
		gs.AsPlayer(gamePlayerIdx)

		// We have actions allowed by game engine
		player := gs.GetPlayer(gamePlayerIdx)
		if len(player.AllowedActions) > 0 {
			return pr.requestMove(gs, gamePlayerIdx)
		}
	}

	return nil
}

func (pr *PlayerRunner) OnAutoModeUpdated(fn func(string, bool)) error {
	pr.onAutoModeUpdated = fn
	return nil
}

func (pr *PlayerRunner) OnTableStateUpdated(fn func(*pokertable.Table)) error {
	pr.onTableStateUpdated = fn
	return nil
}

func (pr *PlayerRunner) OnTableGameWagerActionUpdated(fn TableGameWagerActionUpdatedFunc) error {
	pr.onTableGameWagerActionUpdated = fn
	return nil
}

func (pr *PlayerRunner) requestMove(gs *pokerface.GameState, playerIdx int) error {

	// Do pass automatically
	if gs.HasAction(playerIdx, "pass") {
		fmt.Printf("[%s#%d][%d][%s][%s] PASS\n", pr.tableInfo.ID, pr.tableInfo.UpdateSerial, pr.tableInfo.State.GameCount, pr.playerID, pr.tableInfo.State.GameState.Status.Round)
		return pr.actions.Pass()
	}

	// Pay for ante and blinds
	switch gs.Status.CurrentEvent {
	case pokerface.GameEventSymbols[pokerface.GameEvent_AnteRequested]:

		// Ante
		fmt.Printf("[%s#%d][%d][%s][%s] PAY ANTE\n", pr.tableInfo.ID, pr.tableInfo.UpdateSerial, pr.tableInfo.State.GameCount, pr.playerID, pr.tableInfo.State.GameState.Status.Round)
		return pr.actions.Pay(gs.Meta.Ante)

	case pokerface.GameEventSymbols[pokerface.GameEvent_BlindsRequested]:

		// blinds
		if gs.HasPosition(playerIdx, "sb") {
			fmt.Printf("[%s#%d][%d][%s][%s] PAY SB (%d)\n", pr.tableInfo.ID, pr.tableInfo.UpdateSerial, pr.tableInfo.State.GameCount, pr.playerID, pr.tableInfo.State.GameState.Status.Round, gs.Meta.Blind.SB)
			return pr.actions.Pay(gs.Meta.Blind.SB)
		} else if gs.HasPosition(playerIdx, "bb") {
			fmt.Printf("[%s#%d][%d][%s][%s] PAY BB  (%d)\n", pr.tableInfo.ID, pr.tableInfo.UpdateSerial, pr.tableInfo.State.GameCount, pr.playerID, pr.tableInfo.State.GameState.Status.Round, gs.Meta.Blind.BB)
			return pr.actions.Pay(gs.Meta.Blind.BB)
		}

		fmt.Printf("[%s#%d][%d][%s][%s] PAY DEALER (%d)\n", pr.tableInfo.ID, pr.tableInfo.UpdateSerial, pr.tableInfo.State.GameCount, pr.playerID, pr.tableInfo.State.GameState.Status.Round, gs.Meta.Blind.Dealer)
		return pr.actions.Pay(gs.Meta.Blind.Dealer)
	}

	// Player is suspended
	if pr.status == PlayerStatus_Suspend {
		return pr.automate(gs, playerIdx)
	}

	// Setup timebank to wait for player
	thinkingTime := time.Duration(pr.tableInfo.Meta.ActionTime) * time.Second
	return pr.timebank.NewTask(thinkingTime, func(isCancelled bool) {

		if isCancelled {
			return
		}

		// Stay idle already
		pr.Idle()

		// Do default actions if player has no response
		pr.automate(gs, playerIdx)
	})
}

func (pr *PlayerRunner) automate(gs *pokerface.GameState, playerIdx int) error {

	// Default actions for automation when player has no response
	if gs.HasAction(playerIdx, "ready") {
		err := pr.actions.Ready()
		if err != nil {
			return err
		}

		fmt.Printf("[%s#%d][%d][%s][%s][%+v] AUTO READY\n", pr.tableInfo.ID, pr.tableInfo.UpdateSerial, pr.tableInfo.State.GameCount, pr.playerID, pr.tableInfo.State.GameState.Status.Round, pr.status)
		return nil
	} else if gs.HasAction(playerIdx, "check") {
		err := pr.actions.Check()
		if err != nil {
			return err
		}

		fmt.Printf("[%s#%d][%d][%s][%s][%+v] AUTO CHECK\n", pr.tableInfo.ID, pr.tableInfo.UpdateSerial, pr.tableInfo.State.GameCount, pr.playerID, pr.tableInfo.State.GameState.Status.Round, pr.status)
		pr.updateWagerAction(pokertable.WagerAction_Check, 0)
		return nil
	} else if gs.HasAction(playerIdx, "fold") {
		err := pr.actions.Fold()
		if err != nil {
			return err
		}

		fmt.Printf("[%s#%d][%d][%s][%s][%+v] AUTO FOLD\n", pr.tableInfo.ID, pr.tableInfo.UpdateSerial, pr.tableInfo.State.GameCount, pr.playerID, pr.tableInfo.State.GameState.Status.Round, pr.status)
		pr.updateWagerAction(pokertable.WagerAction_Fold, 0)
		return nil
	}

	return nil
}

func (pr *PlayerRunner) SetSuspendThreshold(count int) {
	pr.suspendThreshold = count
}

func (pr *PlayerRunner) Resume() error {

	if pr.status == PlayerStatus_Running {
		return nil
	}

	pr.status = PlayerStatus_Running
	pr.idleCount = 0
	pr.updateAutoMode(false)

	return nil
}

func (pr *PlayerRunner) Idle() error {
	if pr.status != PlayerStatus_Idle {
		pr.status = PlayerStatus_Idle
		pr.idleCount = 0
		fmt.Printf("[DEBUG#PlayerRunner#Idle][%s] Player (%s) is entering idle mode. Idle Count: %d\n", pr.tableInfo.ID, pr.playerID, pr.idleCount)
	} else {
		pr.idleCount++
		fmt.Printf("[DEBUG#PlayerRunner#Idle][%s] Player (%s) is continuing idle mode. Idle Count: %d\n", pr.tableInfo.ID, pr.playerID, pr.idleCount)
	}

	if pr.idleCount == pr.suspendThreshold {
		fmt.Printf("[DEBUG#PlayerRunner#Idle][%s] Player (%s) is switching to auto mode. Idle Count: %d\n", pr.tableInfo.ID, pr.playerID, pr.idleCount)
		return pr.Suspend()
	}

	return nil
}

func (pr *PlayerRunner) Suspend() error {
	pr.status = PlayerStatus_Suspend
	pr.updateAutoMode(true)
	return nil
}

func (pr *PlayerRunner) Pass() error {

	err := pr.actions.Pass()
	if err != nil {
		return err
	}

	pr.idleCount = 0
	pr.timebank.Cancel()

	return nil
}

func (pr *PlayerRunner) Ready() error {

	err := pr.actions.Ready()
	if err != nil {
		return err
	}

	pr.idleCount = 0
	pr.timebank.Cancel()

	return nil
}

func (pr *PlayerRunner) Pay(chips int64) error {

	err := pr.actions.Pay(chips)
	if err != nil {
		return err
	}

	pr.idleCount = 0
	pr.timebank.Cancel()

	return nil
}

func (pr *PlayerRunner) Check() error {

	err := pr.actions.Check()
	if err != nil {
		return err
	}

	pr.idleCount = 0
	pr.updateWagerAction(pokertable.WagerAction_Check, 0)
	pr.timebank.Cancel()

	return nil
}

func (pr *PlayerRunner) Bet(chips int64) error {

	err := pr.actions.Bet(chips)
	if err != nil {
		return err
	}

	pr.idleCount = 0
	pr.updateWagerAction(pokertable.WagerAction_Bet, chips)
	pr.timebank.Cancel()

	return nil
}

func (pr *PlayerRunner) Call() error {
	// calc chips when action is call
	// TODO: should move logic to pokertable
	wager := int64(0)
	gamePlayerIdx := pr.tableInfo.FindGamePlayerIdx(pr.playerID)
	if gamePlayerIdx >= 0 && pr.tableInfo != nil && pr.tableInfo.State.GameState != nil && gamePlayerIdx < len(pr.tableInfo.State.GameState.Players) {
		wager = pr.tableInfo.State.GameState.Status.CurrentWager - pr.tableInfo.State.GameState.GetPlayer(gamePlayerIdx).Wager
	}

	err := pr.actions.Call()
	if err != nil {
		return err
	}

	pr.idleCount = 0
	pr.updateWagerAction(pokertable.WagerAction_Call, wager)
	pr.timebank.Cancel()

	return nil
}

func (pr *PlayerRunner) Fold() error {
	pr.Resume()
	err := pr.actions.Fold()
	if err != nil {
		return err
	}

	pr.updateWagerAction(pokertable.WagerAction_Fold, 0)
	pr.timebank.Cancel()

	return nil
}

func (pr *PlayerRunner) Allin() error {
	// TODO: should move logic to pokertable
	wager := int64(0)
	gamePlayerIdx := pr.tableInfo.FindGamePlayerIdx(pr.playerID)
	if gamePlayerIdx >= 0 && pr.tableInfo != nil && pr.tableInfo.State.GameState != nil && gamePlayerIdx < len(pr.tableInfo.State.GameState.Players) {
		wager = pr.tableInfo.State.GameState.GetPlayer(gamePlayerIdx).StackSize
	}

	err := pr.actions.Allin()
	if err != nil {
		return err
	}

	pr.idleCount = 0
	pr.updateWagerAction(pokertable.WagerAction_AllIn, wager)
	pr.timebank.Cancel()

	return nil
}

func (pr *PlayerRunner) Raise(chipLevel int64) error {

	err := pr.actions.Raise(chipLevel)
	if err != nil {
		return err
	}

	pr.idleCount = 0
	pr.updateWagerAction(pokertable.WagerAction_Raise, chipLevel)
	pr.timebank.Cancel()

	return nil
}

func (pr *PlayerRunner) AutoMode(isOn bool) error {
	if isOn {
		return pr.Suspend()
	} else {
		return pr.Resume()
	}
}

func (pr *PlayerRunner) IsAutoMode() (string, bool) {
	tableID := ""
	if pr.tableInfo != nil {
		tableID = pr.tableInfo.ID
	}

	isOn := pr.status != PlayerStatus_Running

	return tableID, isOn
}

func (pr *PlayerRunner) updateWagerAction(action string, chips int64) {
	tableID := ""
	gameCount := 0
	round := ""
	if pr.tableInfo != nil {
		tableID = pr.tableInfo.ID
		gameCount = pr.tableInfo.State.GameCount

		if pr.tableInfo.State.GameState != nil {
			round = pr.tableInfo.State.GameState.Status.Round
		}
	}
	pr.onTableGameWagerActionUpdated(tableID, pr.curGameID, gameCount, round, action, chips)
}

func (pr *PlayerRunner) updateAutoMode(isOn bool) {
	if !pr.isEventSubscribed {
		return
	}

	tableID := ""
	if pr.tableInfo != nil {
		tableID = pr.tableInfo.ID
	}
	pr.onAutoModeUpdated(tableID, isOn)
}
