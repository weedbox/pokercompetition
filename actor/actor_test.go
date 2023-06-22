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
	pokertable "github.com/weedbox/pokertable"
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
	joinPlayers := funk.Map(playerIDs, func(playerID string) pokercompetition.JoinPlayer {
		return pokercompetition.JoinPlayer{
			PlayerID:    playerID,
			RedeemChips: 9000,
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
		t.Log("[Table] ERROR:", err)
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
