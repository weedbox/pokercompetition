package actor

import (
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/thoas/go-funk"
	"github.com/weedbox/pokercompetition"
	"github.com/weedbox/pokertable"
	"github.com/weedbox/pokertablebalancer"
)

func TestActor_Basic(t *testing.T) {

	// Initializing table
	tableEngine := pokertable.NewTableEngine()

	table, err := tableEngine.CreateTable(
		pokertable.TableSetting{
			ShortID:        "ABC123",
			Code:           "01",
			Name:           "3300 - 10 sec",
			InvitationCode: "come_to_play",
			CompetitionMeta: pokertable.CompetitionMeta{
				ID: uuid.New().String(),
				Blind: pokertable.Blind{
					ID:              uuid.New().String(),
					Name:            "3300 FAST",
					FinalBuyInLevel: 2,
					InitialLevel:    1,
					Levels: []pokertable.BlindLevel{
						{
							Level:    1,
							SB:       10,
							BB:       20,
							Ante:     0,
							Duration: 1,
						},
						{
							Level:    2,
							SB:       20,
							BB:       30,
							Ante:     0,
							Duration: 1,
						},
						{
							Level:    3,
							SB:       30,
							BB:       40,
							Ante:     0,
							Duration: 1,
						},
					},
				},
				MaxDuration:         10,
				Rule:                pokertable.CompetitionRule_Default,
				Mode:                pokertable.CompetitionMode_CT,
				TableMaxSeatCount:   9,
				TableMinPlayerCount: 2,
				MinChipUnit:         10,
				ActionTime:          10,
			},
		},
	)
	assert.Nil(t, err)

	// Initializing bot
	players := []pokertable.JoinPlayer{
		{PlayerID: "Jeffrey", RedeemChips: 3000},
		{PlayerID: "Chuck", RedeemChips: 3000},
		{PlayerID: "Fred", RedeemChips: 3000},
	}

	// Preparing actors
	actors := make([]Actor, 0)
	for _, p := range players {

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

	var wg sync.WaitGroup
	wg.Add(1)

	// Preparing table state updater
	tableEngine.OnErrorUpdated(func(err error) {
		t.Log("ERROR:", err)
	})
	tableEngine.OnTableUpdated(func(table *pokertable.Table) {
		// t.Log("UPDATED")
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
			tableID := table.ID
			err := tableEngine.DeleteTable(tableID)
			assert.Nil(t, err, "delete table failed")
		case pokertable.TableStateStatus_TableClosed:
			t.Log("table is closed")
			wg.Done()
			return
		}
	})

	// Add player to table
	for _, p := range players {
		err := tableEngine.PlayerJoin(table.ID, p)
		assert.Nil(t, err)
	}

	// Start game
	err = tableEngine.StartTableGame(table.ID)
	assert.Nil(t, err)

	wg.Wait()
}

func TestActor_CT(t *testing.T) {
	logData := make([]string, 0)
	makeLog := func(logTitle string, jsonPrinter func() (string, error)) string {
		jsonString, _ := jsonPrinter()
		return fmt.Sprintf("========== [%s] ==========\n%s", logTitle, jsonString)
	}

	// 建立賽事引擎
	competitionEngine := pokercompetition.NewCompetitionEngine()

	// 後台建立 CT 賽事
	competition, err := competitionEngine.CreateCompetition(NewCTCompetitionSetting())
	assert.Nil(t, err, "create ct competition failed")
	logData = append(logData, makeLog("[Competition] Create CT Competition", competition.GetJSON))
	assert.Equal(t, pokercompetition.CompetitionStateStatus_Registering, competition.State.Status, "status should be registering")

	competitionID := competition.ID
	table := competition.State.Tables[0]

	// 建立 Bot 玩家
	playerIDs := []string{"Jeffrey", "Fred", "Chuck"}
	playerIDs = []string{
		"d6efae4a-f697-447e-8eb7-e3a3f703fe46",
		"69e7b73c-d769-4d0a-88f9-3f1df50fa8ec",
		"fbfe451f-0bbc-44fc-9ba8-57caf3fd6b17",
	}
	joinPlayers := funk.Map(playerIDs, func(playerID string) pokercompetition.JoinPlayer {
		return pokercompetition.JoinPlayer{
			PlayerID:    playerID,
			RedeemChips: 6000,
		}
	}).([]pokercompetition.JoinPlayer)

	// Preparing actors
	actors := make([]Actor, 0)
	for _, p := range joinPlayers {

		// Create new actor
		a := NewActor()

		// Initializing table engine adapter to communicate with table engine
		tc := NewTableEngineAdapter(competitionEngine.TableEngine(), table)
		a.SetAdapter(tc)

		// Initializing bot runner
		bot := NewBotRunner(p.PlayerID)
		a.SetRunner(bot)

		actors = append(actors, a)
	}

	var wg sync.WaitGroup
	wg.Add(1)

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
			wg.Done()
			return
		}
	})
	competitionEngine.OnCompetitionErrorUpdated(func(err error) {
		t.Log("[Competition] Error:", err)
	})

	competitionEngine.OnCompetitionTableUpdated(func(table *pokertable.Table) {
		logData = append(logData, makeLog(fmt.Sprintf("[Table] Serial: %d", table.UpdateSerial), table.GetJSON))

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
			t.Log("table is pausing")
		case pokertable.TableStateStatus_TableClosed:
			t.Log("table is closed")
		}
	})
	competitionEngine.OnCompetitionTableErrorUpdated(func(err error) {
		// t.Log("[Table] ERROR:", err)
	})

	// 玩家報名賽事
	for _, joinPlayer := range joinPlayers {
		time.Sleep(time.Millisecond * 100)
		err := competitionEngine.PlayerJoin(competitionID, table.ID, joinPlayer)
		assert.Nil(t, err, fmt.Sprintf("%s join competition failed", joinPlayer.PlayerID))
		logData = append(logData, makeLog(fmt.Sprintf("[Competition] %s join competition", joinPlayer.PlayerID), competition.GetJSON))
	}

	wg.Wait()
}

func TestActor_MTT(t *testing.T) {
	logData := make([]string, 0)
	makeLog := func(logTitle string, jsonPrinter func() (string, error)) string {
		jsonString, _ := jsonPrinter()
		return fmt.Sprintf("========== [%s] ==========\n%s", logTitle, jsonString)
	}

	// 建立賽事引擎
	competitionEngine := pokercompetition.NewCompetitionEngine()
	competitionEngine.SetSeatManager(pokertablebalancer.NewSeatManager(competitionEngine))

	// 後台建立 MTT 賽事
	competition, err := competitionEngine.CreateCompetition(NewMTTCompetitionSetting())
	assert.Nil(t, err, "create mtt competition failed")
	logData = append(logData, makeLog("[Competition] Create MTT Competition", competition.GetJSON))
	assert.Equal(t, pokercompetition.CompetitionStateStatus_Registering, competition.State.Status, "status should be registering")
	assert.Equal(t, pokercompetition.CompetitionMode_MTT, competition.Meta.Mode, "mode should be mtt")

	competitionID := competition.ID

	// 建立 Bot 玩家
	playerCount := 20
	playerIDs := make([]string, 0)
	for i := 1; i <= playerCount; i++ {
		playerID := fmt.Sprintf("player-%d", i)
		playerIDs = append(playerIDs, playerID)
	}
	playerIDs = []string{
		"83f2d94b-5f65-4ca4-b062-f42988157e5f",
		"69e7b73c-d769-4d0a-88f9-3f1df50fa8ec",
		"fbfe451f-0bbc-44fc-9ba8-57caf3fd6b17",
		"d6efae4a-f697-447e-8eb7-e3a3f703fe46",
		"46841656-82de-47b1-afdd-498b8544169f",
		"01a98f07-c909-4067-8c26-d241ba9e7b8b",
		"47a48dee-acff-49c0-b9aa-465c2a4abcc0",
		"5f29264e-14b1-44f5-85af-a2b1ad8a2c2c",
		"16123aeb-3732-4943-8fd8-ee57a97110c1",
		"27484ba4-cea6-492f-bb8d-598d3b08bf6f",
		"d19dd52e-6b75-4381-b311-0465cf8528d6",
		"d6a3f7fd-89d1-4bf7-b6ed-875ce0d97c55",
		"34fe8fec-4e2c-474e-814c-c243ff2c5c03",
		"b29f6ef9-ef11-4b60-8f26-f9b6a1f90501",
		"bf57b198-c3bb-40b7-92d8-cd77f0089770",
		"4bb53f09-946e-42e1-8712-fce3580d43c3",
		"09aabd19-bddc-41d0-aadc-4bd9a9d19ec9",
		"1725aed5-ade1-4e6c-983c-23393065b378",
		"e8f1835b-df20-4e53-8fdc-ae45e6185752",
		"e2077dd7-a8d7-4ead-a745-4051b35e7023",
	}
	joinPlayers := funk.Map(playerIDs, func(playerID string) pokercompetition.JoinPlayer {
		return pokercompetition.JoinPlayer{
			PlayerID:    playerID,
			RedeemChips: 3000,
		}
	}).([]pokercompetition.JoinPlayer)

	// Preparing actors
	actors := make([]Actor, 0)
	for _, p := range joinPlayers {

		// Create new actor
		a := NewActor()

		// Initializing table engine adapter to communicate with table engine
		tc := NewTableEngineAdapter(competitionEngine.TableEngine(), nil)
		a.SetAdapter(tc)

		// Initializing bot runner
		bot := NewBotRunner(p.PlayerID)
		a.SetRunner(bot)

		actors = append(actors, a)
	}

	var wg sync.WaitGroup
	wg.Add(1)

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
			wg.Done()
			return
		}

		if competition.State.Status == pokercompetition.CompetitionStateStatus_DelayedBuyin {
			for _, player := range competition.State.Players {
				if player.Chips == 0 && player.ReBuyTimes < competition.Meta.ReBuySetting.MaxTime {
					jp := pokercompetition.JoinPlayer{
						PlayerID:    player.PlayerID,
						RedeemChips: 3100,
					}
					err := competitionEngine.PlayerJoin(competitionID, player.CurrentTableID, jp)
					assert.Nil(t, err, fmt.Sprintf("%s re-buy failed", jp.PlayerID))
					t.Logf("%s is rebuying", jp.PlayerID)
				}
			}
		}
	})
	competitionEngine.OnCompetitionErrorUpdated(func(err error) {
		t.Log("[Competition] Error:", err)
	})

	competitionEngine.OnCompetitionTableUpdated(func(table *pokertable.Table) {
		logData = append(logData, makeLog(fmt.Sprintf("[Table] Serial: %d", table.UpdateSerial), table.GetJSON))

		// Update table state via adapter
		for _, a := range actors {
			a.GetTable().UpdateTableState(table)
		}

		switch table.State.Status {
		case pokertable.TableStateStatus_TableGameOpened:
			DebugPrintTableGameOpenedShort(*table)
		case pokertable.TableStateStatus_TableGameSettled:
			ccc, _ := competitionEngine.GetCompetition(competitionID)
			DebugPrintTableGameSettledShort(*table, string(ccc.State.Status))
		case pokertable.TableStateStatus_TablePausing:
			ccc, _ := competitionEngine.GetCompetition(competitionID)
			t.Logf("table %s [%s] is pausing. Final Buy In? %+v", table.ID, ccc.State.Status, table.State.BlindState.IsFinalBuyInLevel())
		case pokertable.TableStateStatus_TableClosed:
			t.Logf("table %s is close", table.ID)
		}
	})
	competitionEngine.OnCompetitionTableErrorUpdated(func(err error) {
		t.Log("[Table] ERROR:", err)
	})

	// 玩家報名賽事
	for _, joinPlayer := range joinPlayers {
		// time.Sleep(time.Millisecond * 100)
		err := competitionEngine.PlayerJoin(competitionID, "", joinPlayer)
		assert.Nil(t, err, fmt.Sprintf("%s buy in failed", joinPlayer.PlayerID))
		logData = append(logData, makeLog(fmt.Sprintf("[Competition] %s buy in", joinPlayer.PlayerID), competition.GetJSON))
		t.Logf("%s buy in", joinPlayer.PlayerID)
	}

	// 手動開賽
	err = competitionEngine.StartCompetition(competitionID)
	assert.Nil(t, err, "start mtt competition failed")

	wg.Wait()
}
