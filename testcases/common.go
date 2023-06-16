package testcases

import (
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/weedbox/pokercompetition"
	"github.com/weedbox/pokertable"
)

func WriteToFile(logTitle string, jsonPrinter func() (string, error)) error {
	file := "./game_log.txt"
	encodedData, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	jsonString, _ := jsonPrinter()
	newData := ""
	if len(encodedData) > 0 {
		newData = fmt.Sprintf("%s\n\n========== [%s] ==========\n%s", string(encodedData), logTitle, jsonString)
	} else {
		newData = fmt.Sprintf("========== [%s] ==========\n%s", logTitle, jsonString)
	}
	err = ioutil.WriteFile(file, []byte(newData), 0644)
	if err != nil {
		return err
	}

	return nil
}

func LogJSON(t *testing.T, msg string, jsonPrinter func() (string, error)) {
	json, _ := jsonPrinter()
	fmt.Printf("\n===== [%s] =====\n%s\n", msg, json)
}

func FindCurrentPlayerID(table *pokertable.Table) string {
	currGamePlayerIdx := table.State.GameState.Status.CurrentPlayer
	for gamePlayerIdx, playerIdx := range table.State.GamePlayerIndexes {
		if gamePlayerIdx == currGamePlayerIdx {
			return table.State.PlayerStates[playerIdx].PlayerID
		}
	}
	return ""
}

func PrintPlayerActionLog(table *pokertable.Table, playerID, actionLog string) {
	findPlayerIdx := func(players []*pokertable.TablePlayerState, targetPlayerID string) int {
		for idx, player := range players {
			if player.PlayerID == targetPlayerID {
				return idx
			}
		}

		return -1
	}

	positions := make([]string, 0)
	playerIdx := findPlayerIdx(table.State.PlayerStates, playerID)
	if playerIdx != -1 {
		positions = table.State.PlayerStates[playerIdx].Positions
	}

	fmt.Printf("[%s] %s%+v: %s\n", table.State.GameState.Status.Round, playerID, positions, actionLog)
}

func NewPlayerActionErrorLog(table *pokertable.Table, playerID, actionLog string, err error) string {
	if err == nil {
		return ""
	}

	findPlayerIdx := func(players []*pokertable.TablePlayerState, targetPlayerID string) int {
		for idx, player := range players {
			if player.PlayerID == targetPlayerID {
				return idx
			}
		}

		return -1
	}

	positions := make([]string, 0)
	playerIdx := findPlayerIdx(table.State.PlayerStates, playerID)
	if playerIdx != -1 {
		positions = table.State.PlayerStates[playerIdx].Positions
	}

	return fmt.Sprintf("[%s] %s%+v: %s. Error: %s\n", table.State.GameState.Status.Round, playerID, positions, actionLog, err.Error())
}

func AllGamePlayersReady(t *testing.T, tableEngine pokertable.TableEngine, table *pokertable.Table) {
	for _, playerIdx := range table.State.GamePlayerIndexes {
		player := table.State.PlayerStates[playerIdx]
		err := tableEngine.PlayerReady(table.ID, player.PlayerID)
		assert.Nil(t, err, NewPlayerActionErrorLog(table, FindCurrentPlayerID(table), "ready", err))
		PrintPlayerActionLog(table, player.PlayerID, fmt.Sprintf("ready. CurrentEvent: %s", table.State.GameState.Status.CurrentEvent.Name))
		err = WriteToFile(fmt.Sprintf("[Table] Game Count [%d] %s Ready", table.State.GameCount, player.PlayerID), table.GetJSON)
		assert.Nil(t, err, fmt.Sprintf("log game count [%d] %s ready failed", table.State.GameCount, player.PlayerID))
	}
}

func DebugPrintCompetitionEnded(c pokercompetition.Competition) {
	fmt.Println("---------- 賽事已結束 ----------")

	timeString := func(timestamp int64) string {
		return time.Unix(timestamp, 0).Format("2006-01-02 15:04:0")
	}

	fmt.Println("[賽事建立時間] ", timeString(c.State.OpenAt))
	fmt.Println("[賽事開打時間] ", timeString(c.State.StartAt))
	fmt.Println("[賽事結束時間] ", timeString(c.State.EndAt))
	fmt.Println("[賽事產生事件] ", c.UpdateSerial)

	fmt.Println("---------- 參與賽事玩家 ----------")
	playerDataMap := make(map[string]pokercompetition.CompetitionPlayer)
	for _, player := range c.State.Players {
		playerDataMap[player.PlayerID] = *player

		isKnockout := "X"
		if player.Status == pokercompetition.CompetitionPlayerStatus_Knockout {
			isKnockout = "O"
		}
		fmt.Printf("%s, 加入時間: %s, 淘汰: %s, 桌次: %s, 桌排名: %d, 籌碼: %d\n", player.PlayerID, timeString(player.JoinAt), isKnockout, player.CurrentTableID, player.Rank, player.Chips)
	}

	fmt.Println("---------- 最後排名 ----------")
	for idx, rank := range c.State.Rankings {
		fmt.Printf("[%d] %s: %d\n", idx+1, rank.PlayerID, rank.FinalChips)
	}

	fmt.Println()
}
