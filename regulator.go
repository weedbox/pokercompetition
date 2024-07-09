package pokercompetition

import (
	"fmt"
	"strings"

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

func (ce *competitionEngine) shouldActivateRegulator() bool {
	if ce.isRegulatorStarted {
		return false
	}

	if !ce.isStarted {
		return false
	}

	return len(ce.competition.State.Players) >= ce.competition.Meta.MinPlayerCount
}

func (ce *competitionEngine) activateRegulator() {
	// MTT 啟動盲注
	if !ce.blind.IsStarted() {
		err := ce.activateBlind()
		if err != nil {
			ce.emitErrorEvent("MTT Activate Blind Error", "", err)
		}
	}

	//  把等待區玩家加入拆併桌程式
	if err := ce.regulatorAddPlayers(ce.waitingPlayers); err != nil {
		ce.emitErrorEvent("MTT Regulator Add Players Error", strings.Join(ce.waitingPlayers, ","), err)
	}
	ce.waitingPlayers = make([]string, 0)

	// 啟動拆併桌程式
	ce.regulator.SetStatus(regulator.CompetitionStatus_Normal)
	ce.isRegulatorStarted = true
}

func (ce *competitionEngine) regulatorAddPlayers(playerIDs []string) error {
	if err := ce.regulator.AddPlayers(playerIDs); err != nil {
		return err
	}

	fmt.Printf("---------- [c: %s] Regulator Add %d Players: %v ----------\n",
		ce.competition.ID,
		len(playerIDs),
		playerIDs,
	)

	fmt.Printf("---------- [c: %s][RegulatorAddPlayers 後 regulator 有 %d 人][賽事正在玩: %d 人, 等待區: %d 人] ----------\n",
		ce.competition.ID,
		ce.regulator.GetPlayerCount(),
		ce.competition.GetPlayerCountByStatus(CompetitionPlayerStatus_Playing),
		ce.competition.GetPlayerCountByStatus(CompetitionPlayerStatus_WaitingTableBalancing),
	)

	return nil
}
