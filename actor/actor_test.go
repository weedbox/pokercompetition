package actor

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thoas/go-funk"
	"github.com/weedbox/pokercompetition"
	pokertable "github.com/weedbox/pokertable"
	"github.com/weedbox/pokertablebalancer"
)

func TestActor_CT(t *testing.T) {
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
	tableManagerBackend := pokercompetition.NewTableManagerBackend(tableManager)
	manager := pokercompetition.NewManager(tableManagerBackend)

	// 建立賽事
	competitionSetting := NewCTCompetitionSetting()
	competition, err := manager.CreateCompetition(competitionSetting)
	assert.Nil(t, err, "create competition failed")
	assert.Equal(t, pokercompetition.CompetitionStateStatus_Registering, competition.State.Status, "status should be registering")
	logData = append(logData, makeLog(fmt.Sprintf("[Competition][%d] Create CT Competition", competition.UpdateSerial), competition.GetJSON))
	t.Log("create ct competition")

	// get competition engine
	competitionEngine, err := manager.GetCompetitionEngine(competition.ID)
	assert.Nil(t, err, "get competition engine engine failed")

	competitionEngine.OnCompetitionErrorUpdated(func(c *pokercompetition.Competition, err error) {
		t.Log("[Competition] Error:", err)
	})
	competitionEngine.OnCompetitionUpdated(func(competition *pokercompetition.Competition) {
		logData = append(logData, makeLog(fmt.Sprintf("[Competition][%d]", competition.UpdateSerial), competition.GetJSON))
		if competition.State.Status == pokercompetition.CompetitionStateStatus_End {
			DebugPrintCompetitionEnded(*competition)

			// write log
			data := strings.Join(logData, "\n\n")
			err := ioutil.WriteFile("./game_log.txt", []byte(data), 0644)
			if err != nil {
				t.Log("Log failed", err)
			} else {
				t.Log("Log completed")
			}
			wg.Done()
			return
		}
	})
	competitionEngine.OnCompetitionPlayerUpdated(func(cp *pokercompetition.CompetitionPlayer) {
		json, err := jsonStringfy(cp)
		assert.Nil(t, err, "json.Marshal CompetitionPlayer failed")
		logData = append(logData, fmt.Sprintf("========== [CompetitionPlayer] ==========\n%s", json))
	})

	// 取得桌次管理
	tableEngine, err := tableManager.GetTableEngine(competition.State.Tables[0].ID)
	assert.Nil(t, err, "get table engine engine failed")

	// 建立 Bot 玩家
	playerIDs := []string{"Jeffrey", "Fred", "Chuck"}
	joinPlayers := funk.Map(playerIDs, func(playerID string) pokercompetition.JoinPlayer {
		return pokercompetition.JoinPlayer{
			PlayerID:    playerID,
			RedeemChips: 6000,
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
		a.SetRunner(bot)

		actors = append(actors, a)
	}

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
			DebugPrintTableGameSettled(*table)
		case pokertable.TableStateStatus_TablePausing:
			t.Log("table is pausing. is final buy in:", competitionEngine.GetCompetition().State.BlindState.IsFinalBuyInLevel())
		case pokertable.TableStateStatus_TableClosed:
			t.Log("table is closed")
		}

		tableManagerBackend.UpdateTable(table)
	})

	// 玩家報名賽事
	for _, joinPlayer := range joinPlayers {
		err := manager.PlayerBuyIn(competition.ID, joinPlayer)
		assert.Nil(t, err, fmt.Sprintf("%s buy in competition failed", joinPlayer.PlayerID))
		logData = append(logData, makeLog(fmt.Sprintf("[Competition][%d] %s join competition", competitionEngine.GetCompetition().UpdateSerial, joinPlayer.PlayerID), competition.GetJSON))
		t.Logf("%s buy in", joinPlayer.PlayerID)

		time.Sleep(time.Millisecond * 300)

		err = tableEngine.PlayerJoin(joinPlayer.PlayerID)
		assert.Nil(t, err, fmt.Sprintf("%s join table failed", joinPlayer.PlayerID))
		t.Logf("%s is joined", joinPlayer.PlayerID)
	}

	wg.Wait()
}

func TestActor_MTT(t *testing.T) {
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
	tableManagerBackend := pokercompetition.NewTableManagerBackend(tableManager)
	manager := pokercompetition.NewManager(tableManagerBackend)

	// 建立賽事
	competitionSetting := NewMTTCompetitionSetting()
	competition, err := manager.CreateCompetition(competitionSetting)
	assert.Nil(t, err, "create competition failed")
	assert.Equal(t, pokercompetition.CompetitionStateStatus_Registering, competition.State.Status, "status should be registering")
	logData = append(logData, makeLog("[Competition] Create CT Competition", competition.GetJSON))
	t.Log("create ct competition")

	// get competition engine
	competitionEngine, err := manager.GetCompetitionEngine(competition.ID)
	assert.Nil(t, err, "get competition engine engine failed")

	competitionEngine.SetSeatManager(pokertablebalancer.NewSeatManager(competitionEngine))

	competitionEngine.OnCompetitionErrorUpdated(func(c *pokercompetition.Competition, err error) {
		t.Log("[Competition] Error:", err)
	})
	competitionEngine.OnCompetitionUpdated(func(competition *pokercompetition.Competition) {
		logData = append(logData, makeLog(fmt.Sprintf("[Competition] Serial: %d", competition.UpdateSerial), competition.GetJSON))
		if competition.State.Status == pokercompetition.CompetitionStateStatus_End {
			DebugPrintCompetitionEnded(*competition)

			// write log
			data := strings.Join(logData, "\n\n")
			err := ioutil.WriteFile("./game_log.txt", []byte(data), 0644)
			if err != nil {
				t.Log("Log failed", err)
			} else {
				t.Log("Log completed")
			}
			// wg.Done()
			return
		}
	})
	competitionEngine.OnCompetitionPlayerUpdated(func(cp *pokercompetition.CompetitionPlayer) {
		// t.Logf("[CompetitionPlayer] %+v", *cp)
		json, err := jsonStringfy(cp)
		assert.Nil(t, err, "json.Marshal CompetitionPlayer failed")
		logData = append(logData, fmt.Sprintf("========== [CompetitionPlayer] ==========\n%s", json))
	})

	// // 取得桌次管理
	// tableEngine, err := tableManager.GetTableEngine(competition.State.Tables[0].ID)
	// assert.Nil(t, err, "get table engine engine failed")

	// 建立 Bot 玩家
	playerCount := 20
	playerIDs := make([]string, 0)
	for i := 1; i <= playerCount; i++ {
		playerID := fmt.Sprintf("player-%d", i)
		playerIDs = append(playerIDs, playerID)
	}
	joinPlayers := funk.Map(playerIDs, func(playerID string) pokercompetition.JoinPlayer {
		return pokercompetition.JoinPlayer{
			PlayerID:    playerID,
			RedeemChips: 2000,
		}
	}).([]pokercompetition.JoinPlayer)

	// // Preparing actors
	// table := competition.State.Tables[0]
	// actors := make([]Actor, 0)
	// for _, p := range joinPlayers {

	// 	// Create new actor
	// 	a := NewActor()

	// 	// Initializing table engine adapter to communicate with table engine
	// 	tc := NewTableEngineAdapter(tableEngine, table)
	// 	a.SetAdapter(tc)

	// 	// Initializing bot runner
	// 	bot := NewBotRunner(p.PlayerID)
	// 	a.SetRunner(bot)

	// 	actors = append(actors, a)
	// }

	// tableEngine.OnTableUpdated(func(table *pokertable.Table) {
	// 	logData = append(logData, makeLog(fmt.Sprintf("[Table] Serial: %d", table.UpdateSerial), table.GetJSON))

	// 	// Update table state via adapter
	// 	for _, a := range actors {
	// 		a.GetTable().UpdateTableState(table)
	// 	}

	// 	switch table.State.Status {
	// 	case pokertable.TableStateStatus_TableGameOpened:
	// 		DebugPrintTableGameOpened(*table)
	// 	case pokertable.TableStateStatus_TableGameSettled:
	// 		DebugPrintTableGameSettled(*table)
	// 	case pokertable.TableStateStatus_TablePausing:
	// 		t.Log("table is pausing. final buy in: ", table.State.BlindState.IsFinalBuyInLevel())
	// 	case pokertable.TableStateStatus_TableClosed:
	// 		t.Log("table is closed")
	// 	}

	// 	tableManagerBackend.UpdateTable(table)
	// })

	// 玩家報名賽事
	for _, joinPlayer := range joinPlayers {
		err := manager.PlayerBuyIn(competition.ID, joinPlayer)
		assert.Nil(t, err, fmt.Sprintf("%s buy in competition failed", joinPlayer.PlayerID))
		logData = append(logData, makeLog(fmt.Sprintf("[Competition] %s join competition", joinPlayer.PlayerID), competition.GetJSON))
		t.Logf("%s buy in", joinPlayer.PlayerID)

		time.Sleep(time.Millisecond * 100)
	}

	// 手動開賽
	err = manager.StartCompetition(competition.ID)
	assert.Nil(t, err, "start mtt competition failed")

	wg.Wait()
}

// func TestActor_MTT_OLD(t *testing.T) {
// 	logData := make([]string, 0)
// 	makeLog := func(logTitle string, jsonPrinter func() (string, error)) string {
// 		jsonString, _ := jsonPrinter()
// 		return fmt.Sprintf("========== [%s] ==========\n%s", logTitle, jsonString)
// 	}

// 	// 建立賽事引擎
// 	tableEngine := pokertable.NewTableEngine()
// 	tableBackend := pokercompetition.NewTableBackend(tableEngine)
// 	competitionEngine := pokercompetition.NewCompetitionEngine(tableBackend)
// 	competitionEngine.SetSeatManager(pokertablebalancer.NewSeatManager(competitionEngine))

// 	// 後台建立 MTT 賽事
// 	competition, err := competitionEngine.CreateCompetition(NewMTTCompetitionSetting())
// 	assert.Nil(t, err, "create mtt competition failed")
// 	logData = append(logData, makeLog("[Competition] Create MTT Competition", competition.GetJSON))
// 	assert.Equal(t, pokercompetition.CompetitionStateStatus_Registering, competition.State.Status, "status should be registering")
// 	assert.Equal(t, pokercompetition.CompetitionMode_MTT, competition.Meta.Mode, "mode should be mtt")

// 	competitionID := competition.ID

// 	// 建立 Bot 玩家
// 	playerCount := 18
// 	playerIDs := make([]string, 0)
// 	for i := 1; i <= playerCount; i++ {
// 		playerID := fmt.Sprintf("player-%d", i)
// 		playerIDs = append(playerIDs, playerID)
// 	}
// 	joinPlayers := funk.Map(playerIDs, func(playerID string) pokercompetition.JoinPlayer {
// 		return pokercompetition.JoinPlayer{
// 			PlayerID:    playerID,
// 			RedeemChips: 2000,
// 		}
// 	}).([]pokercompetition.JoinPlayer)

// 	// Preparing actors
// 	actors := make([]Actor, 0)
// 	for _, p := range joinPlayers {

// 		// Create new actor
// 		a := NewActor()

// 		// Initializing table engine adapter to communicate with table engine
// 		tc := NewTableEngineAdapter(tableEngine, nil)
// 		a.SetAdapter(tc)

// 		// Initializing bot runner
// 		bot := NewBotRunner(p.PlayerID)
// 		a.SetRunner(bot)

// 		actors = append(actors, a)
// 	}

// 	tableEngine.OnTableUpdated(func(table *pokertable.Table) {
// 		logData = append(logData, makeLog(fmt.Sprintf("[Table] Serial: %d", table.UpdateSerial), table.GetJSON))

// 		// Update table state via adapter
// 		for _, a := range actors {
// 			a.GetTable().UpdateTableState(table)
// 		}

// 		switch table.State.Status {
// 		case pokertable.TableStateStatus_TableGameOpened:
// 			DebugPrintTableGameOpenedShort(*table)
// 		case pokertable.TableStateStatus_TableGameSettled:
// 			ccc, _ := competitionEngine.GetCompetition(competitionID)
// 			DebugPrintTableGameSettledShort(*table, string(ccc.State.Status))
// 		case pokertable.TableStateStatus_TablePausing:
// 			ccc, _ := competitionEngine.GetCompetition(competitionID)
// 			t.Logf("table %s [%s] is pausing. Final Buy In? %+v", table.ID, ccc.State.Status, table.State.BlindState.IsFinalBuyInLevel())
// 		case pokertable.TableStateStatus_TableClosed:
// 			t.Logf("table %s is close", table.ID)
// 		}
// 		tableBackend.UpdateTable(table)
// 	})

// 	var wg sync.WaitGroup
// 	wg.Add(1)

// 	competitionEngine.OnCompetitionUpdated(func(competition *pokercompetition.Competition) {
// 		logData = append(logData, makeLog(fmt.Sprintf("[Competition] Serial: %d", competition.UpdateSerial), competition.GetJSON))

// 		if competition.State.Status == pokercompetition.CompetitionStateStatus_End {
// 			DebugPrintCompetitionEnded(*competition)

// 			// write log
// 			data := strings.Join(logData, "\n\n")
// 			err := ioutil.WriteFile("./game_log.txt", []byte(data), 0644)
// 			if err != nil {
// 				t.Log("Log failed", err)
// 			} else {
// 				t.Log("Log completed")
// 			}
// 			wg.Done()
// 			return
// 		}

// 		// rebuy := func(competition *pokercompetition.Competition) {
// 		// 	if competition.State.Status == pokercompetition.CompetitionStateStatus_DelayedBuyIn {
// 		// 		for _, player := range competition.State.Players {
// 		// 			if player.Chips == 0 && player.ReBuyTimes < competition.Meta.ReBuySetting.MaxTime {
// 		// 				jp := pokercompetition.JoinPlayer{
// 		// 					PlayerID:    player.PlayerID,
// 		// 					RedeemChips: 3000,
// 		// 				}
// 		// 				err := competitionEngine.PlayerJoin(competitionID, player.CurrentTableID, jp)
// 		// 				assert.Nil(t, err, fmt.Sprintf("%s re-buy failed", jp.PlayerID))
// 		// 				t.Logf("%s is rebuying", jp.PlayerID)
// 		// 			}
// 		// 		}
// 		// 	}
// 		// }

// 		// go rebuy(competition)
// 	})
// 	competitionEngine.OnCompetitionErrorUpdated(func(competition *pokercompetition.Competition, err error) {
// 		t.Log("[Competition] Error:", err)
// 	})

// 	// 玩家報名賽事
// 	for _, joinPlayer := range joinPlayers {
// 		// time.Sleep(time.Millisecond * 100)
// 		err := competitionEngine.PlayerJoin(competitionID, "", joinPlayer)
// 		assert.Nil(t, err, fmt.Sprintf("%s buy in failed", joinPlayer.PlayerID))
// 		logData = append(logData, makeLog(fmt.Sprintf("[Competition] %s buy in", joinPlayer.PlayerID), competition.GetJSON))
// 		t.Logf("%s buy in", joinPlayer.PlayerID)
// 	}

// // 手動開賽
// err = competitionEngine.StartCompetition(competitionID)
// assert.Nil(t, err, "start mtt competition failed")

// 	wg.Wait()
// }
