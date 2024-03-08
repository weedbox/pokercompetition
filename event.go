package pokercompetition

import (
	"fmt"
	"time"
)

const (
	CompetitionStateEvent_BlindActivated   = "BlindActivated"
	CompetitionStateEvent_BlindUpdated     = "BlindUpdated"
	CompetitionStateEvent_Started          = "Started"
	CompetitionStateEvent_TableUpdated     = "TableUpdated"
	CompetitionStateEvent_TableGameSettled = "TableGameSettled"
	CompetitionStateEvent_KnockoutPlayers  = "KnockoutPlayers"
	CompetitionStateEvent_CashOutPlayers   = "CashOutPlayers"
	CompetitionStateEvent_Settled          = "Settled"
)

func (ce *competitionEngine) emitEvent(eventName string, playerID string) {
	// refresh competition
	ce.competition.UpdateAt = time.Now().Unix()
	ce.competition.UpdateSerial++

	// emit event
	fmt.Printf("->[Competition][#%d][%s] emit Event: %s\n", ce.competition.UpdateSerial, playerID, eventName)
	ce.onCompetitionUpdated(ce.competition)
}

func (ce *competitionEngine) emitErrorEvent(eventName string, playerID string, err error) {
	fmt.Printf("->[Competition][#%d][%s] emit ERROR Event: %s, Error: %v\n", ce.competition.UpdateSerial, playerID, eventName, err)
	ce.onCompetitionErrorUpdated(ce.competition, err)
}

func (ce *competitionEngine) emitPlayerEvent(eventName string, player *CompetitionPlayer) {
	// emit event
	// fmt.Printf("->[CompetitionPlayer][%s] emit Event: %s\n", eventName, fmt.Sprintf("[%s][%s]: %s", player.PlayerID, player.CurrentTableID, player.Status))
	ce.onCompetitionPlayerUpdated(ce.competition.ID, player)
}

func (ce *competitionEngine) emitCompetitionStateFinalPlayerRankEvent(playerID string, rank int) {
	// emit event
	ce.onCompetitionFinalPlayerRankUpdated(ce.competition.ID, playerID, rank)
}

func (ce *competitionEngine) emitCompetitionStateEvent(eventName string) {
	// emit event
	ce.onCompetitionStateUpdated(eventName, ce.competition)
}
