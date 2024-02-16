package pokercompetition

import (
	"github.com/google/uuid"
	"github.com/weedbox/pokertable"
)

/*
regulatorCreateAndDistributePlayers 建立新桌次並分配玩家至該桌次
- 適用時機: 拆併桌監管器自動觸發
*/
func (ce *competitionEngine) regulatorCreateAndDistributePlayers(playerIDs []string) (string, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	joinPlayers := make([]pokertable.JoinPlayer, 0)
	for _, playerID := range playerIDs {
		playerIdx := ce.competition.FindPlayerIdx(func(cp *CompetitionPlayer) bool {
			return cp.PlayerID == playerID
		})
		if playerIdx == UnsetValue {
			continue
		}

		joinPlayers = append(joinPlayers, pokertable.JoinPlayer{
			PlayerID:    playerID,
			RedeemChips: ce.competition.State.Players[playerIdx].Chips,
			Seat:        pokertable.UnsetValue,
		})
	}

	tableSetting := TableSetting{
		TableID:     uuid.New().String(),
		JoinPlayers: joinPlayers,
	}
	tableID, err := ce.addCompetitionTable(tableSetting, CompetitionPlayerStatus_WaitingTableBalancing)
	if err != nil {
		return "", err
	}

	return tableID, nil
}

/*
regulatorDistributePlayers 分配玩家至某桌次
- 適用時機: 拆併桌監管器自動觸發
*/
func (ce *competitionEngine) regulatorDistributePlayers(tableID string, playerIDs []string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	joinPlayers := make([]pokertable.JoinPlayer, 0)
	for _, playerID := range playerIDs {
		playerIdx := ce.competition.FindPlayerIdx(func(cp *CompetitionPlayer) bool {
			return cp.PlayerID == playerID
		})
		if playerIdx == UnsetValue {
			return ErrCompetitionPlayerNotFound
		}

		joinPlayers = append(joinPlayers, pokertable.JoinPlayer{
			PlayerID:    playerID,
			RedeemChips: ce.competition.State.Players[playerIdx].Chips,
		})
	}
	if _, err := ce.tableManagerBackend.UpdateTablePlayers(tableID, joinPlayers, []string{}); err != nil {
		return err
	}

	return nil
}

func (ce *competitionEngine) regulatorAddPlayers(playerIDs []string) error {
	return ce.regulator.AddPlayers(playerIDs)
}
