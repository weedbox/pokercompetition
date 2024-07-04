package pokercompetition

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/weedbox/pokerface/regulator"
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
	level, ante, dealer, sb, bb := ce.competition.CurrentBlindData()
	blind := pokertable.TableBlindState{
		Level:  level,
		Ante:   ante,
		Dealer: dealer,
		SB:     sb,
		BB:     bb,
	}
	tableID, err := ce.addCompetitionTable(tableSetting, blind)
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
	// 開賽前一律放到等待佇列
	if ce.regulatorStatus == regulator.CompetitionStatus_Pending {
		ce.waitingPlayers = append(ce.waitingPlayers, playerIDs...)
		return nil
	}

	// 開賽後且尚未達到開賽最低人數之前，都把玩家放到等待佇列
	if ce.competition.Meta.MinPlayerCount > len(ce.competition.State.Players) {
		ce.waitingPlayers = append(ce.waitingPlayers, playerIDs...)
		return nil
	}

	// MTT 啟動盲注
	if !ce.blind.IsStarted() {
		err := ce.activateBlind()
		if err != nil {
			ce.emitErrorEvent("MTT Activate Blind Error", "", err)
		}
	}

	// 開賽後且達到開賽最低人數之後，才丟到拆併桌程式
	ce.waitingPlayers = append(ce.waitingPlayers, playerIDs...)
	if err := ce.regulator.AddPlayers(ce.waitingPlayers); err != nil {
		return err
	}

	fmt.Printf("---------- [c: %s] Regulator Add %d Players: %v ----------\n",
		ce.competition.ID,
		len(ce.waitingPlayers),
		ce.waitingPlayers,
	)

	fmt.Printf("---------- [c: %s][RegulatorAddPlayers 後 regulator 有 %d 人][賽事正在玩: %d 人, 等待區: %d 人] ----------\n",
		ce.competition.ID,
		ce.regulator.GetPlayerCount(),
		ce.competition.GetPlayerCountByStatus(CompetitionPlayerStatus_Playing),
		ce.competition.GetPlayerCountByStatus(CompetitionPlayerStatus_WaitingTableBalancing),
	)

	ce.waitingPlayers = make([]string, 0)
	return nil
}
