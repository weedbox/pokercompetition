package testcases

import (
	"testing"
)

func TestCT(t *testing.T) {
	// TODO: refactor this
	// tableEngine := pokertable.NewTableEngine(pokertable.NewGameEngine())
	// competitionEngine := pokercompetition.NewCompetitionEngine()

	// // 後台建立 CT 賽事
	// // Step 1: 建立賽事
	// tableSettingPayload := NewTableSettingPayload()
	// competitionSetting := NewDefaultCompetitionSetting(NewCTCompetitionSettingPayload(tableSettingPayload))
	// competition := competitionEngine.Create(competitionSetting)
	// // logData(t, "create competition", competition.GetJSON)

	// // Step 2: 開桌
	// tableSetting := NewDefaultTableSetting(competition.Meta, tableSettingPayload)
	// tableSetting.CompetitionMeta.ID = competition.ID
	// table, err := tableEngine.CreateTable(tableSetting)
	// assert.Nil(t, err, "create table failed")
	// // logData(t, "create table", table.GetJSON)

	// // Step 3: 賽事加入桌次
	// competition = competitionEngine.AddTable(competition, &table)
	// // logData(t, "add a table to competition", competition.GetJSON)

	// // 玩家報名賽事
	// joinPlayers := []pokercompetition.JoinPlayer{
	// 	{PlayerID: "Jeffrey", RedeemChips: 1000, TableID: table.ID},
	// 	{PlayerID: "Fred", RedeemChips: 1000, TableID: table.ID},
	// 	{PlayerID: "Chuck", RedeemChips: 1000, TableID: table.ID},
	// }

	// for _, joinPlayer := range joinPlayers {
	// 	// Jeffrey 報名
	// 	// Step 1: 加入賽事
	// 	newCompetition, err := competitionEngine.PlayerJoin(competition, joinPlayer)
	// 	assert.Nil(t, err, fmt.Sprintf("%s join competition failed", joinPlayer.PlayerID))
	// 	competition = newCompetition

	// 	// Step 2: 加入桌
	// 	newTable, err := tableEngine.PlayerJoin(table, pokertable.JoinPlayer{PlayerID: joinPlayer.PlayerID, RedeemChips: joinPlayer.RedeemChips})
	// 	assert.Nil(t, err, fmt.Sprintf("%s join table failed", joinPlayer.PlayerID))
	// 	table = newTable
	// }
	// // logData(t, "all players join competition success", competition.GetJSON)

	// // 開打
	// // Step 1: 賽事內所有桌次開打
	// newTables := make([]*pokertable.Table, 0)
	// for _, table := range competition.State.Tables {
	// 	newTable, err := tableEngine.StartGame(*table)
	// 	assert.Nil(t, err, fmt.Sprintf("table %s start game failed", table.ID))
	// 	newTables = append(newTables, &newTable)
	// }

	// // Step 2: 賽事開打
	// competition = competitionEngine.Start(competition, newTables)
	// // logData(t, "competition is started", competition.GetJSON)

	// // 玩家自動玩比賽 (game count 1)
	// newTable := AllPlayersPlaying(t, tableEngine, *competition.State.Tables[0])

	// // Subscribe TableUpdated
	// competition.State.Tables[0] = &newTable
	// // logData(t, "table game count 1 is closed", competition.GetJSON)

	// // // Make 1 person ReBuy
	// // competition.State.Tables[0].State.PlayerStates[0].Bankroll = 0

	// // Make 1 person Knockout
	// // competition.State.Tables[0].State.PlayerStates[0].Bankroll = 0
	// // targetPlayerIdx := -1
	// // for idx, player := range competition.State.Players {
	// // 	if player.PlayerID == competition.State.Tables[0].State.PlayerStates[0].PlayerID {
	// // 		targetPlayerIdx = idx
	// // 		break
	// // 	}
	// // }
	// // competition.State.Players[targetPlayerIdx].ReBuyTimes = competition.Meta.ReBuySetting.MaxTimes

	// // 桌結算
	// competition = competitionEngine.TableSettlement(competition, *competition.State.Tables[0])
	// // logData(t, "table game count 1 is settled", competition.GetJSON)

	// // 如果是停止買入，要建立 Competition.State.Rankings 陣列
	// for _, table := range competition.State.Tables {
	// 	for i := 0; i < len(table.State.PlayerStates); i++ {
	// 		competition.State.Rankings = append(competition.State.Rankings, nil)
	// 	}
	// }

	// // Make TableClosed
	// competition.State.Tables[0].State.Status = pokertable.TableStateStatus_TableGameClosed

	// // 桌次關閉
	// tableID := competition.State.Tables[0].ID
	// newCompetition, err := competitionEngine.CloseTable(competition, competition.State.Tables[0])
	// assert.Nil(t, err, fmt.Sprintf("close table %s failed", tableID))
	// competition = newCompetition
	// // logData(t, fmt.Sprintf("close table %s success", tableID), competition.GetJSON)

	// // 賽事結算
	// shouldSettleCompetition := len(competition.State.Tables) == 0
	// if shouldSettleCompetition {
	// 	competition.State.Status = pokercompetition.CompetitionStateStatus_End
	// 	newCompetition = competitionEngine.Settlement(competition)
	// 	competition = newCompetition
	// 	logData(t, "competition is settled & end", competition.GetJSON)
	// }
}
