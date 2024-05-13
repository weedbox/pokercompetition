package pokercompetition

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/weedbox/pokertable"
)

/*
regulatorCreateAndDistributePlayers 建立新桌次並分配玩家至該桌次
- 適用時機: 拆併桌監管器自動觸發
*/
func (ce *competitionEngine) regulatorCreateAndDistributePlayers(playerIDs []string) (string, error) {
	joinPlayers := make([]pokertable.JoinPlayer, 0)
	for _, playerID := range playerIDs {
		playerIdx := ce.competition.FindPlayerIdx(func(cp *CompetitionPlayer) bool {
			return cp.PlayerID == playerID
		})
		if playerIdx == UnsetValue {
			continue
		}

		chips := ce.competition.State.Players[playerIdx].Chips
		if chips <= 0 {
			return "", fmt.Errorf("competition: failed to regulate player (%s) to new table", playerID)
		}

		joinPlayers = append(joinPlayers, pokertable.JoinPlayer{
			PlayerID:    playerID,
			RedeemChips: chips,
			Seat:        pokertable.UnsetValue,
		})
	}

	tableSetting := TableSetting{
		TableID:     uuid.New().String(),
		JoinPlayers: joinPlayers,
	}
	tableID, err := ce.addCompetitionTable(tableSetting)
	if err != nil {
		return "", err
	}

	fmt.Printf("[MTT#DEBUG#regulatorCreateAndDistributePlayers] Table (%s), Players: %d\n", tableID, len(playerIDs))
	return tableID, nil
}

/*
regulatorDistributePlayers 分配玩家至某桌次
- 適用時機: 拆併桌監管器自動觸發
*/
func (ce *competitionEngine) regulatorDistributePlayers(tableID string, playerIDs []string) error {
	joinPlayers := make([]pokertable.JoinPlayer, 0)
	for _, playerID := range playerIDs {
		playerIdx := ce.competition.FindPlayerIdx(func(cp *CompetitionPlayer) bool {
			return cp.PlayerID == playerID
		})
		if playerIdx == UnsetValue {
			return ErrCompetitionPlayerNotFound
		}

		chips := ce.competition.State.Players[playerIdx].Chips
		if chips <= 0 {
			return fmt.Errorf("competition: failed to regulate player (%s) to table (%s)", playerID, tableID)
		}
		joinPlayers = append(joinPlayers, pokertable.JoinPlayer{
			PlayerID:    playerID,
			RedeemChips: chips,
			Seat:        UnsetValue,
		})
	}
	if _, err := ce.tableManagerBackend.UpdateTablePlayers(tableID, joinPlayers, []string{}); err != nil {
		return err
	}

	fmt.Printf("[MTT#DEBUG#regulatorDistributePlayers] Table (%s), Players: %d\n", tableID, len(playerIDs))
	return nil
}

func (ce *competitionEngine) regulatorAddPlayers(playerIDs []string) error {
	if ce.competition.Meta.MinPlayerCount > len(ce.competition.State.Players) {
		// 達到開賽最低人數之前，都把玩家放到等待佇列
		ce.waitingPlayers = append(ce.waitingPlayers, playerIDs...)
		return nil
	}

	// 達到開賽最低人數之後，才丟到拆併桌程式
	ce.waitingPlayers = append(ce.waitingPlayers, playerIDs...)
	fmt.Printf("[MTT#DEBUG#regulatorAddPlayers] Add %d Players: %v\n", len(ce.waitingPlayers), ce.waitingPlayers)

	if err := ce.regulator.AddPlayers(playerIDs); err != nil {
		return err
	}

	ce.waitingPlayers = make([]string, 0)
	return nil
}
