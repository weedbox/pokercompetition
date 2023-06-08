package testcases

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weedbox/pokertable"
	pokertablemodel "github.com/weedbox/pokertable/model"
	pokertableutil "github.com/weedbox/pokertable/util"
)

func FindCurrentPlayerID(table pokertablemodel.Table, currPlayerIndex int) string {
	for playingPlayerIndex, playerIndex := range table.State.PlayingPlayerIndexes {
		if playingPlayerIndex == currPlayerIndex {
			return table.State.PlayerStates[playerIndex].PlayerID
		}
	}
	return ""
}

func AllGamePlayersReady(t *testing.T, tableEngine pokertable.TableEngine, table pokertablemodel.Table) pokertablemodel.Table {
	ret := table
	for _, playingPlayerIdx := range table.State.PlayingPlayerIndexes {
		player := table.State.PlayerStates[playingPlayerIdx]
		table, err := tableEngine.PlayerReady(table, player.PlayerID)
		assert.Nil(t, err)
		ret = table
	}
	return ret
}

func AllPlayersPlaying(t *testing.T, tableEngine pokertable.TableEngine, table pokertablemodel.Table) pokertablemodel.Table {
	// game started
	// all players ready
	table = AllGamePlayersReady(t, tableEngine, table)

	// preflop
	// dealer move
	table, err := tableEngine.PlayerWager(table, FindCurrentPlayerID(table, table.State.GameState.Status.CurrentPlayer), pokertableutil.WagerAction_Call, 0)
	assert.Nil(t, err)

	// sb move
	tableEngine.PlayerWager(table, FindCurrentPlayerID(table, table.State.GameState.Status.CurrentPlayer), pokertableutil.WagerAction_Call, 0)

	// bb move
	tableEngine.PlayerWager(table, FindCurrentPlayerID(table, table.State.GameState.Status.CurrentPlayer), pokertableutil.WagerAction_Check, 0)

	// flop
	// all players ready
	table = AllGamePlayersReady(t, tableEngine, table)

	// dealer move
	tableEngine.PlayerWager(table, FindCurrentPlayerID(table, table.State.GameState.Status.CurrentPlayer), pokertableutil.WagerAction_Bet, 10)

	// sb move
	tableEngine.PlayerWager(table, FindCurrentPlayerID(table, table.State.GameState.Status.CurrentPlayer), pokertableutil.WagerAction_Call, 0)

	// bb move
	tableEngine.PlayerWager(table, FindCurrentPlayerID(table, table.State.GameState.Status.CurrentPlayer), pokertableutil.WagerAction_Call, 0)

	// turn
	// all players ready
	table = AllGamePlayersReady(t, tableEngine, table)

	// dealer move
	tableEngine.PlayerWager(table, FindCurrentPlayerID(table, table.State.GameState.Status.CurrentPlayer), pokertableutil.WagerAction_Bet, 10)

	// sb move
	tableEngine.PlayerWager(table, FindCurrentPlayerID(table, table.State.GameState.Status.CurrentPlayer), pokertableutil.WagerAction_Call, 0)

	// bb move
	tableEngine.PlayerWager(table, FindCurrentPlayerID(table, table.State.GameState.Status.CurrentPlayer), pokertableutil.WagerAction_Call, 0)

	// river
	// all players ready
	table = AllGamePlayersReady(t, tableEngine, table)

	// dealer move
	tableEngine.PlayerWager(table, FindCurrentPlayerID(table, table.State.GameState.Status.CurrentPlayer), pokertableutil.WagerAction_Bet, 10)

	// sb move
	tableEngine.PlayerWager(table, FindCurrentPlayerID(table, table.State.GameState.Status.CurrentPlayer), pokertableutil.WagerAction_Call, 0)

	// bb move
	tableEngine.PlayerWager(table, FindCurrentPlayerID(table, table.State.GameState.Status.CurrentPlayer), pokertableutil.WagerAction_Call, 0)

	return table
}