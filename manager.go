package pokercompetition

import (
	"errors"
	"sync"

	"github.com/weedbox/pokertable"
)

var (
	ErrManagerCompetitionNotFound = errors.New("manager: competition not found")
)

type Manager interface {
	Reset()
	ReleaseCompetition(competitionID string)

	// CompetitionEngine Actions
	GetCompetitionEngine(competitionID string) (CompetitionEngine, error)

	// Competition Actions
	CreateCompetition(competitionSetting CompetitionSetting, options *CompetitionEngineOptions) (*Competition, error)
	UpdateCompetitionBlindInitialLevel(competitionID string, level int) error
	CloseCompetition(competitionID string, endStatus CompetitionStateStatus) error
	StartCompetition(competitionID string) (int64, error)

	// Table Actions
	GetTableEngineOptions() *pokertable.TableEngineOptions
	SetTableEngineOptions(tableOptions *pokertable.TableEngineOptions)
	UpdateTable(competitionID string, table *pokertable.Table) error
	UpdateReserveTablePlayerState(competitionID, tableID string, playerState *pokertable.TablePlayerState) error
	AutoGameOpenEnd(competitionID, tableID string) error

	// Player Operations
	PlayerBuyIn(competitionID string, joinPlayer JoinPlayer) error
	PlayerAddon(competitionID string, tableID string, joinPlayer JoinPlayer) error
	PlayerRefund(competitionID string, playerID string, unit int) error
	PlayerCashOut(competitionID string, tableID, playerID string) error
	PlayerQuit(competitionID string, tableID, playerID string) error
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

func (m *manager) Reset() {
	m.competitionEngines.Range(func(key, value interface{}) bool {
		if ce, ok := value.(CompetitionEngine); ok {
			_ = ce.CloseCompetition(CompetitionStateStatus_ForceEnd)
		}
		return true
	})

	m.competitionEngines = sync.Map{}
}

func (m *manager) ReleaseCompetition(competitionID string) {
	m.competitionEngines.Delete(competitionID)
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
	)
	competitionEngine.OnCompetitionUpdated(options.OnCompetitionUpdated)
	competitionEngine.OnCompetitionErrorUpdated(options.OnCompetitionErrorUpdated)
	competitionEngine.OnCompetitionPlayerUpdated(options.OnCompetitionPlayerUpdated)
	competitionEngine.OnCompetitionFinalPlayerRankUpdated(options.OnCompetitionFinalPlayerRankUpdated)
	competitionEngine.OnCompetitionStateUpdated(options.OnCompetitionStateUpdated)
	competitionEngine.OnAdvancePlayerCountUpdated(options.OnAdvancePlayerCountUpdated)
	competitionEngine.OnCompetitionPlayerCashOut(options.OnCompetitionPlayerCashOut)
	competition, err := competitionEngine.CreateCompetition(competitionSetting)
	if err != nil {
		return nil, err
	}

	m.competitionEngines.Store(competition.ID, competitionEngine)
	return competition, nil
}

func (m *manager) UpdateCompetitionBlindInitialLevel(competitionID string, level int) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.UpdateCompetitionBlindInitialLevel(level)
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

func (m *manager) StartCompetition(competitionID string) (int64, error) {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return 0, ErrManagerCompetitionNotFound
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

func (m *manager) AutoGameOpenEnd(competitionID, tableID string) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.AutoGameOpenEnd(tableID)
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

func (m *manager) PlayerRefund(competitionID string, playerID string, unit int) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.PlayerRefund(playerID, unit)
}

func (m *manager) PlayerCashOut(competitionID string, tableID, playerID string) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.PlayerCashOut(tableID, playerID)
}

func (m *manager) PlayerQuit(competitionID string, tableID, playerID string) error {
	competitionEngine, err := m.GetCompetitionEngine(competitionID)
	if err != nil {
		return ErrManagerCompetitionNotFound
	}

	return competitionEngine.PlayerQuit(tableID, playerID)
}
