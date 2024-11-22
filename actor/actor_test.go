package actor

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thoas/go-funk"
	"github.com/weedbox/pokercompetition"
	pokertable "github.com/weedbox/pokertable"
)

func TestActor_CT_Breaking(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	logData := make([]string, 0)
	makeLog := func(logTitle string, jsonPrinter func() (string, error)) string {
		jsonString, _ := jsonPrinter()
		return fmt.Sprintf("========== [%s] ==========\n%s", logTitle, jsonString)
	}
	jsonStringfy := func(data interface{}) (string, error) {
		encoded, err := json.Marshal(data)
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	}

	// 建立賽事管理
	tableManager := pokertable.NewManager()
	tableManagerBackend := pokercompetition.NewNativeTableManagerBackend(tableManager)
	manager := pokercompetition.NewManager(tableManagerBackend)
	tableOptions := pokertable.NewTableEngineOptions()
	tableOptions.GameContinueInterval = 1
	tableOptions.OpenGameTimeout = 2
	manager.SetTableEngineOptions(tableOptions)

	// 建立賽事
	competitionSetting := NewCTCompetitionSetting_Breaking()
	options := pokercompetition.NewDefaultCompetitionEngineOptions()
	options.OnCompetitionUpdated = func(competition *pokercompetition.Competition) {
		logData = append(logData, makeLog(fmt.Sprintf("[Competition][%d]", competition.UpdateSerial), competition.GetJSON))
		if competition.State.Status == pokercompetition.CompetitionStateStatus_End {
			DebugPrintCompetitionEnded(*competition)

			// write log
			data := strings.Join(logData, "\n\n")
			err := os.WriteFile("./game_log.txt", []byte(data), 0644)
			if err != nil {
				t.Log("Log failed", err)
			} else {
				t.Log("Log completed")
			}
			wg.Done()
			return
		}
	}
	options.OnCompetitionErrorUpdated = func(c *pokercompetition.Competition, err error) {
		t.Log("[Competition] Error:", err)
	}
	options.OnCompetitionPlayerUpdated = func(cID string, cp *pokercompetition.CompetitionPlayer) {
		json, err := jsonStringfy(cp)
		assert.Nil(t, err, "json.Marshal CompetitionPlayer failed")
		logData = append(logData, fmt.Sprintf("========== [CompetitionPlayer] ==========\n%s", json))
	}

	competition, err := manager.CreateCompetition(competitionSetting, options)
	assert.Nil(t, err, "create competition failed")
	assert.Equal(t, pokercompetition.CompetitionStateStatus_Registering, competition.State.Status, "status should be registering")
	logData = append(logData, makeLog(fmt.Sprintf("[Competition][%d] Create CT Competition", competition.UpdateSerial), competition.GetJSON))
	t.Log("create ct competition")

	// get competition engine
	competitionEngine, err := manager.GetCompetitionEngine(competition.ID)
	assert.Nil(t, err, "get competition engine engine failed")

	// 取得桌次管理
	tableEngine, err := tableManager.GetTableEngine(competition.State.Tables[0].ID)
	assert.Nil(t, err, "get table engine engine failed")

	// 建立 Bot 玩家
	playerIDs := []string{"Jeffrey", "Fred", "Chuck"}
	joinPlayers := funk.Map(playerIDs, func(playerID string) pokercompetition.JoinPlayer {
		return pokercompetition.JoinPlayer{
			PlayerID:    playerID,
			RedeemChips: 10000,
		}
	}).([]pokercompetition.JoinPlayer)

	// Preparing actors
	table := competition.State.Tables[0]
	actors := make([]Actor, 0)
	for _, p := range joinPlayers {

		// Create new actor
		a := NewActor()

		// Initializing table engine adapter to communicate with table engine
		tc := NewTableEngineAdapter(tableEngine, table)
		a.SetAdapter(tc)

		// Initializing bot runner
		bot := NewBotRunner(p.PlayerID)
		bot.OnTableAutoJoinActionRequested(func(competitionID, tableID, playerID string) {
			go func(pID string) {
				assert.Nil(t, tableEngine.PlayerJoin(pID), fmt.Sprintf("%s join table failed", pID))
				t.Logf("%s is joined", pID)
			}(playerID)
		})
		a.SetRunner(bot)

		actors = append(actors, a)
	}

	isGameSettledRecords := make(map[string]bool)
	tableEngine.OnAutoGameOpenEnd(func(competitionID, tableID string) {
		manager.AutoGameOpenEnd(competitionID, tableID)
	})
	tableEngine.OnTableUpdated(func(table *pokertable.Table) {
		logData = append(logData, makeLog(fmt.Sprintf("[Table][%d]", table.UpdateSerial), table.GetJSON))

		// Update table state via adapter
		for _, a := range actors {
			a.GetTable().UpdateTableState(table)
		}

		switch table.State.Status {
		case pokertable.TableStateStatus_TableGameOpened:
			DebugPrintTableGameOpened(*table)
		case pokertable.TableStateStatus_TableGameSettled:
			if isGameSettled, ok := isGameSettledRecords[table.State.GameState.GameID]; ok && isGameSettled {
				break
			}
			isGameSettledRecords[table.State.GameState.GameID] = true
			t.Log("[Competition Status]", competition.State.Status)
			DebugPrintTableGameSettled(*table)
		case pokertable.TableStateStatus_TablePausing:
			t.Logf("table [%s] is pausing. is final buy in: %+v", table.ID, competitionEngine.GetCompetition().State.BlindState.IsStopBuyIn())
		case pokertable.TableStateStatus_TableClosed:
			t.Logf("table [%s] is closed", table.ID)
		}

		var cloneTable pokertable.Table
		if encoded, err := json.Marshal(table); err == nil {
			json.Unmarshal(encoded, &cloneTable)
		} else {
			cloneTable = *table
		}
		tableManagerBackend.UpdateTable(&cloneTable)
	})
	tableEngine.OnReadyOpenFirstTableGame(func(competitionID, tableID string, gameCount int, playerStates []*pokertable.TablePlayerState) {
		participates := map[string]int{}
		for idx, player := range playerStates {
			participates[player.PlayerID] = idx
		}
		tableEngine.SetUpTableGame(gameCount, participates)
	})

	// 玩家報名賽事
	for _, joinPlayer := range joinPlayers {
		err := manager.PlayerBuyIn(competition.ID, joinPlayer)
		assert.Nil(t, err, fmt.Sprintf("%s buy in competition failed", joinPlayer.PlayerID))
		logData = append(logData, makeLog(fmt.Sprintf("[Competition][%d] %s join competition", competitionEngine.GetCompetition().UpdateSerial, joinPlayer.PlayerID), competition.GetJSON))
		t.Logf("%s buy in", joinPlayer.PlayerID)
	}

	wg.Wait()

	// 清空賽事
	manager.ReleaseCompetition(competition.ID)

	_, err = manager.GetCompetitionEngine(competition.ID)
	assert.EqualError(t, err, pokercompetition.ErrManagerCompetitionNotFound.Error())
}

func TestActor_CT_Normal(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	logData := make([]string, 0)
	makeLog := func(logTitle string, jsonPrinter func() (string, error)) string {
		jsonString, _ := jsonPrinter()
		return fmt.Sprintf("========== [%s] ==========\n%s", logTitle, jsonString)
	}
	jsonStringfy := func(data interface{}) (string, error) {
		encoded, err := json.Marshal(data)
		if err != nil {
			return "", err
		}
		return string(encoded), nil
	}

	// 建立賽事管理
	tableManager := pokertable.NewManager()
	tableManagerBackend := pokercompetition.NewNativeTableManagerBackend(tableManager)
	manager := pokercompetition.NewManager(tableManagerBackend)
	tableOptions := pokertable.NewTableEngineOptions()
	tableOptions.GameContinueInterval = 1
	tableOptions.OpenGameTimeout = 2
	manager.SetTableEngineOptions(tableOptions)

	// 建立賽事
	competitionSetting := NewCTCompetitionSetting_Normal()
	options := pokercompetition.NewDefaultCompetitionEngineOptions()
	options.OnCompetitionUpdated = func(competition *pokercompetition.Competition) {
		logData = append(logData, makeLog(fmt.Sprintf("[Competition][%d]", competition.UpdateSerial), competition.GetJSON))
		if competition.State.Status == pokercompetition.CompetitionStateStatus_End {
			DebugPrintCompetitionEnded(*competition)

			// write log
			data := strings.Join(logData, "\n\n")
			err := os.WriteFile("./game_log.txt", []byte(data), 0644)
			if err != nil {
				t.Log("Log failed", err)
			} else {
				t.Log("Log completed")
			}
			wg.Done()
			return
		}
	}
	options.OnCompetitionErrorUpdated = func(c *pokercompetition.Competition, err error) {
		t.Log("[Competition] Error:", err)
	}
	options.OnCompetitionPlayerUpdated = func(cID string, cp *pokercompetition.CompetitionPlayer) {
		json, err := jsonStringfy(cp)
		assert.Nil(t, err, "json.Marshal CompetitionPlayer failed")
		logData = append(logData, fmt.Sprintf("========== [CompetitionPlayer] ==========\n%s", json))
	}

	competition, err := manager.CreateCompetition(competitionSetting, options)
	assert.Nil(t, err, "create competition failed")
	assert.Equal(t, pokercompetition.CompetitionStateStatus_Registering, competition.State.Status, "status should be registering")
	logData = append(logData, makeLog(fmt.Sprintf("[Competition][%d] Create CT Competition", competition.UpdateSerial), competition.GetJSON))
	t.Log("create ct competition")

	// get competition engine
	competitionEngine, err := manager.GetCompetitionEngine(competition.ID)
	assert.Nil(t, err, "get competition engine engine failed")

	// 取得桌次管理
	tableEngine, err := tableManager.GetTableEngine(competition.State.Tables[0].ID)
	assert.Nil(t, err, "get table engine engine failed")

	// 建立 Bot 玩家
	playerIDs := []string{"Jeffrey", "Fred", "Chuck"}
	joinPlayers := funk.Map(playerIDs, func(playerID string) pokercompetition.JoinPlayer {
		return pokercompetition.JoinPlayer{
			PlayerID:    playerID,
			RedeemChips: 1000,
		}
	}).([]pokercompetition.JoinPlayer)

	// Preparing actors
	table := competition.State.Tables[0]
	actors := make([]Actor, 0)
	for _, p := range joinPlayers {

		// Create new actor
		a := NewActor()

		// Initializing table engine adapter to communicate with table engine
		tc := NewTableEngineAdapter(tableEngine, table)
		a.SetAdapter(tc)

		// Initializing bot runner
		bot := NewBotRunner(p.PlayerID)
		bot.OnTableAutoJoinActionRequested(func(competitionID, tableID, playerID string) {
			go func(pID string) {
				assert.Nil(t, tableEngine.PlayerJoin(pID), fmt.Sprintf("%s join table failed", pID))
				t.Logf("%s is joined", pID)
			}(playerID)
		})
		a.SetRunner(bot)

		actors = append(actors, a)
	}

	isGameSettledRecords := make(map[string]bool)
	tableEngine.OnAutoGameOpenEnd(func(competitionID, tableID string) {
		manager.AutoGameOpenEnd(competitionID, tableID)
	})
	tableEngine.OnTableUpdated(func(table *pokertable.Table) {
		logData = append(logData, makeLog(fmt.Sprintf("[Table][%d]", table.UpdateSerial), table.GetJSON))

		// Update table state via adapter
		for _, a := range actors {
			a.GetTable().UpdateTableState(table)
		}

		switch table.State.Status {
		case pokertable.TableStateStatus_TableGameOpened:
			DebugPrintTableGameOpened(*table)
		case pokertable.TableStateStatus_TableGameSettled:
			if isGameSettled, ok := isGameSettledRecords[table.State.GameState.GameID]; ok && isGameSettled {
				break
			}
			isGameSettledRecords[table.State.GameState.GameID] = true
			t.Log("[Competition Status]", competition.State.Status)
			DebugPrintTableGameSettled(*table)
		case pokertable.TableStateStatus_TablePausing:
			t.Logf("table [%s] is pausing. is final buy in: %+v", table.ID, competitionEngine.GetCompetition().State.BlindState.IsStopBuyIn())
		case pokertable.TableStateStatus_TableClosed:
			t.Logf("table [%s] is closed", table.ID)
		}

		var cloneTable pokertable.Table
		if encoded, err := json.Marshal(table); err == nil {
			json.Unmarshal(encoded, &cloneTable)
		} else {
			cloneTable = *table
		}
		tableManagerBackend.UpdateTable(&cloneTable)
	})
	tableEngine.OnReadyOpenFirstTableGame(func(competitionID, tableID string, gameCount int, playerStates []*pokertable.TablePlayerState) {
		participates := map[string]int{}
		for idx, player := range playerStates {
			participates[player.PlayerID] = idx
		}
		tableEngine.SetUpTableGame(gameCount, participates)
	})

	// 玩家報名賽事
	for _, joinPlayer := range joinPlayers {
		time.Sleep(time.Millisecond * 100)
		err := manager.PlayerBuyIn(competition.ID, joinPlayer)
		assert.Nil(t, err, fmt.Sprintf("%s buy in competition failed", joinPlayer.PlayerID))
		logData = append(logData, makeLog(fmt.Sprintf("[Competition][%d] %s join competition", competitionEngine.GetCompetition().UpdateSerial, joinPlayer.PlayerID), competition.GetJSON))
		t.Logf("%s buy in", joinPlayer.PlayerID)
	}

	wg.Wait()

	// 清空賽事
	manager.ReleaseCompetition(competition.ID)

	_, err = manager.GetCompetitionEngine(competition.ID)
	assert.EqualError(t, err, pokercompetition.ErrManagerCompetitionNotFound.Error())
}

// func TestActor_MTT(t *testing.T) {
// 	var wg sync.WaitGroup
// 	wg.Add(1)

// 	logData := make([]string, 0)
// 	makeLog := func(logTitle string, jsonPrinter func() (string, error)) string {
// 		jsonString, _ := jsonPrinter()
// 		return fmt.Sprintf("========== [%s] ==========\n%s", logTitle, jsonString)
// 	}
// 	jsonStringfy := func(data interface{}) (string, error) {
// 		encoded, err := json.Marshal(data)
// 		if err != nil {
// 			return "", err
// 		}
// 		return string(encoded), nil
// 	}

// 	// 建立玩家
// 	playerCount := 9
// 	fmt.Println("Total Players: ", playerCount)
// 	playerIDs := make([]string, 0)
// 	for i := 1; i <= playerCount; i++ {
// 		playerID := fmt.Sprintf("p%d", i)
// 		playerIDs = append(playerIDs, playerID)
// 	}
// 	joinPlayers := funk.Map(playerIDs, func(playerID string) pokercompetition.JoinPlayer {
// 		return pokercompetition.JoinPlayer{
// 			PlayerID:    playerID,
// 			RedeemChips: 2000,
// 		}
// 	}).([]pokercompetition.JoinPlayer)

// 	// 建立賽事管理
// 	qm := pokercompetition.NewNativeQueueManager()
// 	err := qm.Connect()
// 	assert.Nil(t, err, "queue manager connect error")
// 	tableManager := pokertable.NewManager()
// 	tableManagerBackend := pokercompetition.NewNativeTableManagerBackend(tableManager)
// 	manager := pokercompetition.NewManager(tableManagerBackend, qm)
// 	tableOptions := pokertable.NewTableEngineOptions()
// 	tableOptions.Interval = 2
// 	manager.SetTableEngineOptions(tableOptions)

// 	// 建立賽事
// 	competitionSetting := NewMTTCompetitionSetting()
// 	options := pokercompetition.NewDefaultCompetitionEngineOptions()
// 	options.OnCompetitionUpdated = func(competition *pokercompetition.Competition) {
// 		logData = append(logData, makeLog(fmt.Sprintf("[Competition][%d]", competition.UpdateSerial), competition.GetJSON))
// 		if competition.State.Status == pokercompetition.CompetitionStateStatus_End {
// 			DebugPrintCompetitionEnded(*competition)

// 			// write log
// 			data := strings.Join(logData, "\n\n")
// 			err := os.WriteFile("./game_log.txt", []byte(data), 0644)
// 			if err != nil {
// 				t.Log("Log failed", err)
// 			} else {
// 				t.Log("Log completed")
// 			}
// 			wg.Done()
// 			return
// 		}
// 	}
// 	options.OnCompetitionErrorUpdated = func(c *pokercompetition.Competition, err error) {
// 		t.Log("[Competition] Error:", err)
// 	}
// 	options.OnCompetitionPlayerUpdated = func(cID string, cp *pokercompetition.CompetitionPlayer) {
// 		json, err := jsonStringfy(cp)
// 		assert.Nil(t, err, "json.Marshal CompetitionPlayer failed")
// 		logData = append(logData, fmt.Sprintf("========== [CompetitionPlayer] ==========\n%s", json))
// 	}

// 	competition, err := manager.CreateCompetition(competitionSetting, options)
// 	assert.Nil(t, err, "create competition failed")
// 	assert.Equal(t, pokercompetition.CompetitionStateStatus_Registering, competition.State.Status, "status should be registering")
// 	logData = append(logData, makeLog("[Competition] Create MTT Competition", competition.GetJSON))
// 	t.Log("create mtt competition")

// 	// get competition engine
// 	competitionEngine, err := manager.GetCompetitionEngine(competition.ID)
// 	assert.Nil(t, err, "get competition engine engine failed")

// 	// binding actors & table engines
// 	tablePlayers := make(map[string]map[string]interface{})
// 	tableEngines := make(map[string]pokertable.TableEngine, 0)
// 	actors := make(map[string]Actor, 0)

// 	// Preparing actors
// 	for _, p := range joinPlayers {
// 		// Create new actor
// 		a := NewActor()

// 		// Initializing bot runner
// 		bot := NewBotRunner(p.PlayerID)
// 		a.SetRunner(bot)

// 		actors[p.PlayerID] = a
// 	}

// 	// Mapping player & table
// 	competitionEngine.OnCompetitionPlayerUpdated(func(cID string, cp *pokercompetition.CompetitionPlayer) {
// 		// t.Logf("[CompetitionPlayer] [%s] %s", cp.CurrentTableID, cp.PlayerID)
// 		json, err := jsonStringfy(cp)
// 		assert.Nil(t, err, "json.Marshal CompetitionPlayer failed")
// 		logData = append(logData, fmt.Sprintf("========== [CompetitionPlayer] ==========\n%s", json))
// 	})

// 	// Preparing table engine
// 	competitionEngine.OnTableCreated(func(ctable *pokertable.Table) {
// 		tableEngine, err := tableManager.GetTableEngine(ctable.ID)
// 		assert.Nil(t, err, "get table engine engine failed")

// 		// delete non-existing players
// 		for playerID := range tablePlayers[ctable.ID] {
// 			if funk.Contains(ctable.State.PlayerStates, playerID) {
// 				delete(tablePlayers[ctable.ID], playerID)
// 			}
// 		}

// 		// add new players & update adapter
// 		if _, exist := tablePlayers[ctable.ID]; !exist {
// 			tablePlayers[ctable.ID] = make(map[string]interface{})
// 		}

// 		tableEngine.OnTableUpdated(func(table *pokertable.Table) {
// 			logData = append(logData, makeLog(fmt.Sprintf("[Table][%d]", table.UpdateSerial), table.GetJSON))

// 			// Update table state via adapter
// 			for _, player := range table.State.PlayerStates {
// 				if _, exist := tablePlayers[table.ID][player.PlayerID]; !exist {
// 					tc := NewTableEngineAdapter(tableEngine, ctable)
// 					actors[player.PlayerID].SetAdapter(tc)
// 				}

// 				if actor, exist := actors[player.PlayerID]; exist {
// 					if actor.GetTable() != nil {
// 						actors[player.PlayerID].GetTable().UpdateTableState(table)
// 					}
// 				}
// 			}

// 			// switch table.State.Status {
// 			// case pokertable.TableStateStatus_TableGameOpened:
// 			// 	DebugPrintTableGameOpenedShort(*table)
// 			// // case pokertable.TableStateStatus_TableGameSettled:
// 			// // 	DebugPrintTableGameSettled(*table)
// 			// case pokertable.TableStateStatus_TablePausing:
// 			// 	t.Logf("table [%s] is pausing. is final buy in: %+v", table.ID, competitionEngine.GetCompetition().State.BlindState.IsFinalBuyInLevel())
// 			// case pokertable.TableStateStatus_TableClosed:
// 			// 	t.Logf("table [%s] is closed", table.ID)
// 			// }

// 			tableManagerBackend.UpdateTable(table)
// 		})

// 		tableEngines[ctable.ID] = tableEngine
// 	})
// 	competitionEngine.OnTableClosed(func(table *pokertable.Table) {
// 		delete(tableEngines, table.ID)
// 		delete(tablePlayers, table.ID)
// 	})

// 	// 玩家報名賽事
// 	for _, joinPlayer := range joinPlayers {
// 		err := manager.PlayerBuyIn(competition.ID, joinPlayer)
// 		assert.Nil(t, err, fmt.Sprintf("%s buy in competition failed", joinPlayer.PlayerID))
// 		logData = append(logData, makeLog(fmt.Sprintf("[Competition] %s join competition", joinPlayer.PlayerID), competition.GetJSON))
// 		// t.Logf("%s buy in", joinPlayer.PlayerID)

// 		// time.Sleep(time.Millisecond * 100)
// 	}

// 	// 手動開賽
// 	_, err = manager.StartCompetition(competition.ID)
// 	assert.Nil(t, err, "start mtt competition failed")
// 	wg.Wait()
// }
