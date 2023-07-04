package actor

import (
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	pokertable "github.com/weedbox/pokertable"
)

func TestActor_ObserverRunner_PlayerAct(t *testing.T) {

	// Initializing table
	// create manager & table
	manager := pokertable.NewManager()
	table, err := manager.CreateTable(pokertable.TableSetting{
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
	})
	assert.Nil(t, err, "create table failed")

	// get table engine
	tableEngine, err := manager.GetTableEngine(table.ID)
	assert.Nil(t, err, "get table engine failed")

	// Initializing bot
	players := []pokertable.JoinPlayer{
		{PlayerID: "Jeffrey", RedeemChips: 3000},
		{PlayerID: "Chuck", RedeemChips: 3000},
		{PlayerID: "Fred", RedeemChips: 3000},
	}

	actors := make([]Actor, 0)

	// Initializing observer
	a := NewActor()

	tc := NewTableEngineAdapter(tableEngine, table)
	a.SetAdapter(tc)

	observer := NewObserverRunner()
	a.SetRunner(observer)

	var wg sync.WaitGroup
	wg.Add(1)

	observer.OnTableStateUpdated(func(table *pokertable.Table) {

		if table.State.Status == pokertable.TableStateStatus_TableGameSettled {
			if table.State.GameState.Status.CurrentEvent == "GameClosed" {
				t.Log("GameClosed", table.State.GameState.GameID)

				if len(table.AlivePlayers()) == 1 {
					tableEngine.CloseTable()
					wg.Done()
					return
				}
			}
		}

		if table.State.Status == pokertable.TableStateStatus_TableGamePlaying {
			gs := table.State.GameState

			if gs.Status.LastAction == nil {
				return
			}

			if gs.Status.LastAction.Type == "big_blind" {
				//json, _ := table.GetJSON()
				//t.Log(json)

			}

			//t.Log(gs.Status.LastAction.Type, gs.Status.LastAction.Source, gs.Status.LastAction.Value)
		}

	})

	actors = append(actors, a)

	// Preparing players
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

	// Preparing table state updater
	tableEngine.OnTableUpdated(func(table *pokertable.Table) {

		// Update table state via adapter
		for _, a := range actors {
			a.GetTable().UpdateTableState(table)
		}
	})

	// Add player to table
	for _, p := range players {
		assert.Nil(t, tableEngine.PlayerReserve(p), fmt.Sprintf("%s reserve error", p.PlayerID))
		assert.Nil(t, tableEngine.PlayerJoin(p.PlayerID), fmt.Sprintf("%s join error", p.PlayerID))
	}

	// Start game
	err = tableEngine.StartTableGame()
	assert.Nil(t, err)

	wg.Wait()
}
