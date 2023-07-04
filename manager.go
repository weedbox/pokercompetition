package pokercompetition

import (
	"errors"
	"sync"

	"github.com/weedbox/pokertablebalancer"
)

var (
	ErrManagerCompetitionNotFound = errors.New("manager: competition not found")
)

type Manager interface {
	// CompetitionEngine Actions
	GetCompetitionEngine(competitionID string) (CompetitionEngine, error)

	// Competition Actions
	SetCompetitionSeatManager(competitionID string, seatManager pokertablebalancer.SeatManager) error
	CreateCompetition(competitionSetting CompetitionSetting) (*Competition, error)
	CloseCompetition(competitionID string) error
	StartCompetition(competitionID string) error

	// Player Operations
	PlayerBuyIn(competitionID string, joinPlayer JoinPlayer) error
	PlayerAddon(competitionID string, tableID string, joinPlayer JoinPlayer) error
	PlayerRefund(competitionID string, playerID string) error
	PlayerLeave(competitionID string, tableID, playerID string) error
}

type manager struct {
	competitionEngines  sync.Map
	tableManagerBackend TableManagerBackend
}

func NewManager(tableManagerBackend TableManagerBackend) Manager {
	return &manager{
		competitionEngines:  sync.Map{},
		tableManagerBackend: tableManagerBackend,
	}
}

func (m *manager) GetCompetitionEngine(competitionID string) (CompetitionEngine, error) {
	competitionEngine, exist := m.competitionEngines.Load(competitionID)
	if !exist {
		return nil, ErrManagerCompetitionNotFound
	}
	return competitionEngine.(CompetitionEngine), nil
}

func (m *manager) SetCompetitionSeatManager(competitionID string, seatManager pokertablebalancer.SeatManager) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	competitionEngine.SetSeatManager(seatManager)
	return nil

}

func (m *manager) CreateCompetition(competitionSetting CompetitionSetting) (*Competition, error) {
	competitionEngine := NewCompetitionEngine(WithTableManagerBackend(m.tableManagerBackend))
	competition, err := competitionEngine.CreateCompetition(competitionSetting)
	if err != nil {
		return nil, err
	}

	m.competitionEngines.Store(competition.ID, competitionEngine)
	return competition, nil
}

func (m *manager) CloseCompetition(competitionID string) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.CloseCompetition()
}

func (m *manager) StartCompetition(competitionID string) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.StartCompetition()
}

func (m *manager) PlayerBuyIn(competitionID string, joinPlayer JoinPlayer) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.PlayerBuyIn(joinPlayer)
}

func (m *manager) PlayerAddon(competitionID string, tableID string, joinPlayer JoinPlayer) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.PlayerAddon(tableID, joinPlayer)
}

func (m *manager) PlayerRefund(competitionID string, playerID string) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.PlayerRefund(playerID)
}

func (m *manager) PlayerLeave(competitionID string, tableID, playerID string) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.PlayerLeave(tableID, playerID)
}
