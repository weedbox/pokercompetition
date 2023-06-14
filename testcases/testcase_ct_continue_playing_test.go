package testcases

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thoas/go-funk"
	"github.com/weedbox/pokercompetition"
	"github.com/weedbox/pokertable"
)

func TestCT_ContinuePlaying(t *testing.T) {
	autoPlaying := func(t *testing.T, tableEngine pokertable.TableEngine, tableID string) {
		// game started
		// all players ready
		table, err := tableEngine.GetTable(tableID)
		assert.Nil(t, err, "failed to get table")
		err = writeToFile(fmt.Sprintf("[Table] Game Count [%d] Started", table.State.GameCount), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] started failed", table.State.GameCount))

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

		// all players ready
		AllGamePlayersReady(t, tableEngine, table)

		// dealer move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "call")
		err = tableEngine.PlayerCall(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "call", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count [%d] preflop %s[dealer] call", table.State.GameCount, currPlayerID), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] preflop dealer call failed", table.State.GameCount))

		// sb move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "call")
		err = tableEngine.PlayerCall(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "call", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count [%d] preflop %s[sb] call", table.State.GameCount, currPlayerID), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] preflop sb call failed", table.State.GameCount))

		// bb move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "check")
		err = tableEngine.PlayerCheck(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "check", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count [%d] preflop %s[bb] check", table.State.GameCount, currPlayerID), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] preflop bb check failed", table.State.GameCount))

		// flop
		// all players ready
		AllGamePlayersReady(t, tableEngine, table)

		// sb move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "bet 10")
		err = tableEngine.PlayerBet(tableID, currPlayerID, 10)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "bet 10", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count [%d] flop %s[sb] bet 10", table.State.GameCount, currPlayerID), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] flop sb bet 10 failed", table.State.GameCount))

		// bb move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "call")
		err = tableEngine.PlayerCall(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "call", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count [%d] flop %s[bb] call", table.State.GameCount, currPlayerID), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] flop bb call failed", table.State.GameCount))

		// dealer move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "call")
		err = tableEngine.PlayerCall(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "call", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count [%d] flop %s[dealer] call", table.State.GameCount, currPlayerID), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] flop dealer call failed", table.State.GameCount))

		// turn
		// all players ready
		AllGamePlayersReady(t, tableEngine, table)

		// sb move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "bet 10")
		err = tableEngine.PlayerBet(tableID, currPlayerID, 10)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "bet 10", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count [%d] turn %s[sb] bet 10", table.State.GameCount, currPlayerID), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] turn sb bet 10 failed", table.State.GameCount))

		// bb move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "call")
		err = tableEngine.PlayerCall(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "call", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count [%d] turn %s[bb] call", table.State.GameCount, currPlayerID), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] turn bb call failed", table.State.GameCount))

		// dealer move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "call")
		err = tableEngine.PlayerCall(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "call", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count [%d] turn %s[dealer] call", table.State.GameCount, currPlayerID), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] turn dealer call failed", table.State.GameCount))

		// river
		// all players ready
		AllGamePlayersReady(t, tableEngine, table)

		// sb move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "bet 10")
		err = tableEngine.PlayerBet(tableID, currPlayerID, 10)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "bet 10", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count [%d] river %s[sb] bet 10", table.State.GameCount, currPlayerID), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] river sb bet 10 failed", table.State.GameCount))

		// bb move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "call")
		err = tableEngine.PlayerCall(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "call", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count [%d] river %s[bb] call", table.State.GameCount, currPlayerID), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] river bb call failed", table.State.GameCount))

		// dealer move
		currPlayerID = FindCurrentPlayerID(table)
		PrintPlayerActionLog(table, currPlayerID, "call")
		err = tableEngine.PlayerCall(tableID, currPlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, currPlayerID, "call", err))
		err = writeToFile(fmt.Sprintf("[Table] Game Count [%d] river %s[dealer] call", table.State.GameCount, currPlayerID), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] river dealer call failed", table.State.GameCount))
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
	playerIDs := []string{"Jeffrey", "Fred", "Chuck"}
	joinPlayers := funk.Map(playerIDs, func(playerID string) pokercompetition.JoinPlayer {
		return pokercompetition.JoinPlayer{
			PlayerID:    playerID,
			RedeemChips: 1000,
		}
	}).([]pokercompetition.JoinPlayer)
	for _, joinPlayer := range joinPlayers {
		err := competitionEngine.PlayerJoin(competitionID, tableID, joinPlayer)
		assert.Nil(t, err, fmt.Sprintf("%s join competition failed", joinPlayer.PlayerID))
		err = writeToFile(fmt.Sprintf("[Competition] %s join competition", joinPlayer.PlayerID), competition.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("%s join competition failed", joinPlayer.PlayerID))
	}

	// 玩家自動玩比賽
	for i := 0; i < 3; i++ {
		autoPlaying(t, competitionEngine.TableEngine(), tableID)
		err = writeToFile(fmt.Sprintf("[Competition] Game Count [%d] Competition Settlement", (i+1)), competition.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] competition settlement failed", (i+1)))
		time.Sleep(2 * time.Second)
	}
}
