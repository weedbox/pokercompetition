package testcases

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weedbox/pokercompetition"
	"github.com/weedbox/pokertable"
)

func TestCT(t *testing.T) {
	autoPlaying := func(t *testing.T, tableEngine pokertable.TableEngine, tableID string) {
		// game started
		// all players ready
		table, err := tableEngine.GetTable(tableID)
		assert.Nil(t, err, "failed to get table")
		err = writeToFile("[Table] Game Count 1 Started", table.GetJSON)
		assert.Nil(t, err, "log game count 1 started failed")

		AllGamePlayersReady(t, tableEngine, table)

		currPlayerID := ""
		// preflop
		// pay sb
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "pay sb")
		err = tableEngine.PlayerPaySB(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "pay sb", err))
		fmt.Printf("[PlayerPaySB] dealer receive bb.\n")

		// pay bb
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "pay bb")
		err = tableEngine.PlayerPayBB(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "pay bb", err))
		fmt.Printf("[PlayerPayBB] dealer receive bb.\n")

		// rest players ready
		AllGamePlayersReady(t, tableEngine, table)
		err = writeToFile("[Table] Game Count 1 Preflop All Players Ready", table.GetJSON)
		assert.Nil(t, err, "log game count 1 preflop all players ready failed")

		// dealer move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "call")
		err = tableEngine.PlayerCall(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "call", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count 1 preflop %s[dealer] call", currPlayerID), table.GetJSON)
		assert.Nil(t, err, "log game count 1 preflop dealer call failed")

		// sb move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "call")
		err = tableEngine.PlayerCall(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "call", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count 1 preflop %s[sb] call", currPlayerID), table.GetJSON)
		assert.Nil(t, err, "log game count 1 preflop sb call failed")

		// bb move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "check")
		err = tableEngine.PlayerCheck(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "check", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count 1 preflop %s[bb] check", currPlayerID), table.GetJSON)
		assert.Nil(t, err, "log game count 1 preflop bb check failed")

		// logJSON(t, fmt.Sprintf("Game %d - preflop all players done actions", table.State.GameCount), table.GetJSON)

		// flop
		// all players ready
		AllGamePlayersReady(t, tableEngine, table)
		err = writeToFile("[Table] Game Count 1 Flop All Players Ready", table.GetJSON)
		assert.Nil(t, err, "log game count 1 flop all players ready failed")

		// sb move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "allin")
		err = tableEngine.PlayerAllin(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "all in", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count 1 flop %s[sb] all in", currPlayerID), table.GetJSON)
		assert.Nil(t, err, "log game count 1 preflop sb all in failed")

		// bb move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "allin")
		err = tableEngine.PlayerAllin(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "all in", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count 1 flop %s[bb] all in", currPlayerID), table.GetJSON)
		assert.Nil(t, err, "log game count 1 preflop bb all in failed")

		// dealer move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "allin")
		_ = tableEngine.PlayerAllin(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "all in", err))
	}

	competitionEngine := pokercompetition.NewCompetitionEngine()
	competitionEngine.OnCompetitionUpdated(func(competition *pokercompetition.Competition) {})

	// 後台建立 CT 賽事
	competition, err := competitionEngine.CreateCompetition(NewCTCompetitionSetting())
	assert.Nil(t, err, "create ct competition failed")
	err = writeToFile("[Competition] Create CT Competition", competition.GetJSON)
	assert.Nil(t, err, "log create ct competition failed")

	competitionID := competition.ID
	tableID := competition.State.Tables[0].ID

	// 玩家報名賽事
	joinPlayers := []pokercompetition.JoinPlayer{
		{PlayerID: "Jeffrey", RedeemChips: 1000},
		{PlayerID: "Fred", RedeemChips: 1000},
		{PlayerID: "Chuck", RedeemChips: 1000},
	}

	for _, joinPlayer := range joinPlayers {
		err := competitionEngine.PlayerJoin(competitionID, tableID, joinPlayer)
		assert.Nil(t, err, fmt.Sprintf("%s join competition failed", joinPlayer.PlayerID))
		err = writeToFile(fmt.Sprintf("[Competition] %s join competition", joinPlayer.PlayerID), competition.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("%s join competition failed", joinPlayer.PlayerID))
	}

	// 玩家自動玩比賽
	autoPlaying(t, competitionEngine.TableEngine(), tableID)
	err = writeToFile("[Competition] Game Count 1 Competition Settlement", competition.GetJSON)
	assert.Nil(t, err, "log game count 1 competition settlement failed")
}
