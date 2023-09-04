package pokercompetition

import (
	"errors"
	"sync"

	"github.com/weedbox/pokertable"
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
	CreateCompetition(competitionSetting CompetitionSetting, competitionUpdatedCallBack func(*Competition), competitionErrorUpdatedCallBack func(*Competition, error), competitionPlayerUpdatedCallBack func(string, *CompetitionPlayer), competitionFinalPlayerRankUpdatedCallBack func(string, string, int), competitionStateUpdatedCallBack func(string, *Competition)) (*Competition, error)
	CloseCompetition(competitionID string, endStatus CompetitionStateStatus) error
	StartCompetition(competitionID string) error

	// Table Actions
	GetTableEngineOptions() *pokertable.TableEngineOptions
	SetTableEngineOptions(tableOptions *pokertable.TableEngineOptions)
	UpdateTable(competitionID string, table *pokertable.Table) error

	// Player Operations
	PlayerBuyIn(competitionID string, joinPlayer JoinPlayer) error
	PlayerAddon(competitionID string, tableID string, joinPlayer JoinPlayer) error
	PlayerRefund(competitionID string, playerID string) error
	PlayerLeave(competitionID string, tableID, playerID string) error
}

type manager struct {
	tableOptions        *pokertable.TableEngineOptions
	competitionEngines  sync.Map
	tableManagerBackend TableManagerBackend
}

func NewManager(tableManagerBackend TableManagerBackend) Manager {
	tableOptions := pokertable.NewTableEngineOptions()
	tableOptions.Interval = 6

	return &manager{
		tableOptions:        tableOptions,
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

func (m *manager) CreateCompetition(competitionSetting CompetitionSetting, competitionUpdatedCallBack func(*Competition), competitionErrorUpdatedCallBack func(*Competition, error), competitionPlayerUpdatedCallBack func(string, *CompetitionPlayer), competitionFinalPlayerRankUpdatedCallBack func(string, string, int), competitionStateUpdatedCallBack func(string, *Competition)) (*Competition, error) {
	competitionEngine := NewCompetitionEngine(
		WithTableManagerBackend(m.tableManagerBackend),
		WithTableOptions(m.tableOptions),
	)
	competitionEngine.OnCompetitionUpdated(competitionUpdatedCallBack)
	competitionEngine.OnCompetitionErrorUpdated(competitionErrorUpdatedCallBack)
	competitionEngine.OnCompetitionPlayerUpdated(competitionPlayerUpdatedCallBack)
	competitionEngine.OnCompetitionFinalPlayerRankUpdated(competitionFinalPlayerRankUpdatedCallBack)
	competitionEngine.OnCompetitionStateUpdated(competitionStateUpdatedCallBack)
	competition, err := competitionEngine.CreateCompetition(competitionSetting)
	if err != nil {
		return nil, err
	}

	m.competitionEngines.Store(competition.ID, competitionEngine)
	return competition, nil
}

func (m *manager) CloseCompetition(competitionID string, endStatus CompetitionStateStatus) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.CloseCompetition(endStatus)
}

func (m *manager) StartCompetition(competitionID string) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.StartCompetition()
}

func (m *manager) GetTableEngineOptions() *pokertable.TableEngineOptions {
	return m.tableOptions
}

func (m *manager) SetTableEngineOptions(tableOptions *pokertable.TableEngineOptions) {
	m.tableOptions = tableOptions
}

func (m *manager) UpdateTable(competitionID string, table *pokertable.Table) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	competitionEngine.UpdateTable(table)
	return nil
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
