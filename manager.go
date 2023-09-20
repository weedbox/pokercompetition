package pokercompetition

import (
	"errors"
	"sync"

	"github.com/weedbox/pokerface/match"
	"github.com/weedbox/pokertable"
)

var (
	ErrManagerCompetitionNotFound = errors.New("manager: competition not found")
)

type Manager interface {
	Reset()

	// CompetitionEngine Actions
	GetCompetitionEngine(competitionID string) (CompetitionEngine, error)

	// Competition Actions
	CreateCompetition(competitionSetting CompetitionSetting, options *CompetitionEngineOptions) (*Competition, error)
	CloseCompetition(competitionID string, endStatus CompetitionStateStatus) error
	StartCompetition(competitionID string) error

	// Table Actions
	GetTableEngineOptions() *pokertable.TableEngineOptions
	SetTableEngineOptions(tableOptions *pokertable.TableEngineOptions)
	UpdateTable(competitionID string, table *pokertable.Table) error
	UpdateReserveTablePlayerState(competitionID, tableID string, playerState *pokertable.TablePlayerState) error

	// Player Operations
	PlayerBuyIn(competitionID string, joinPlayer JoinPlayer) error
	PlayerAddon(competitionID string, tableID string, joinPlayer JoinPlayer) error
	PlayerRefund(competitionID string, playerID string) error
	PlayerLeave(competitionID string, tableID, playerID string) error
	PlayerQuit(competitionID string, tableID, playerID string) error
}

type manager struct {
	tableOptions        *pokertable.TableEngineOptions
	competitionEngines  sync.Map
	tableManagerBackend TableManagerBackend
	qm                  match.QueueManager
}

func NewManager(tableManagerBackend TableManagerBackend, qm match.QueueManager) Manager {
	tableOptions := pokertable.NewTableEngineOptions()
	tableOptions.Interval = 6

	return &manager{
		tableOptions:        tableOptions,
		competitionEngines:  sync.Map{},
		tableManagerBackend: tableManagerBackend,
		qm:                  qm,
	}
}

func (m *manager) Reset() {
	m.competitionEngines = sync.Map{}
}

func (m *manager) GetCompetitionEngine(competitionID string) (CompetitionEngine, error) {
	competitionEngine, exist := m.competitionEngines.Load(competitionID)
	if !exist {
		return nil, ErrManagerCompetitionNotFound
	}
	return competitionEngine.(CompetitionEngine), nil
}

func (m *manager) CreateCompetition(competitionSetting CompetitionSetting, options *CompetitionEngineOptions) (*Competition, error) {
	competitionEngine := NewCompetitionEngine(
		WithTableManagerBackend(m.tableManagerBackend),
		WithTableOptions(m.tableOptions),
		WithQueueManagerOptions(m.qm),
	)
	competitionEngine.OnCompetitionUpdated(options.OnCompetitionUpdated)
	competitionEngine.OnCompetitionErrorUpdated(options.OnCompetitionErrorUpdated)
	competitionEngine.OnCompetitionPlayerUpdated(options.OnCompetitionPlayerUpdated)
	competitionEngine.OnCompetitionFinalPlayerRankUpdated(options.OnCompetitionFinalPlayerRankUpdated)
	competitionEngine.OnCompetitionStateUpdated(options.OnCompetitionStateUpdated)
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

	if err := competitionEngine.CloseCompetition(endStatus); err != nil {
		return err
	}

	m.competitionEngines.Delete(competitionID)
	return nil
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

func (m *manager) UpdateReserveTablePlayerState(competitionID, tableID string, playerState *pokertable.TablePlayerState) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	competitionEngine.UpdateReserveTablePlayerState(tableID, playerState)
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

func (m *manager) PlayerQuit(competitionID string, tableID, playerID string) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.PlayerQuit(tableID, playerID)
}
