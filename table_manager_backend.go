package pokercompetition

import (
	"encoding/json"

	"github.com/weedbox/pokertable"
)

type TableManagerBackend interface {
	// Events
	OnTableUpdated(fn func(table *pokertable.Table))

	// TableManager Actions
	CreateTable(setting pokertable.TableSetting) (*pokertable.Table, error)
	CloseTable(tableID string) error
	PlayersBatchReserve(tableID string, joinPlayers []pokertable.JoinPlayer) error
	PlayersLeave(tableID string, playerIDs []string) error
	PlayerRedeemChips(tableID string, joinPlayer pokertable.JoinPlayer) error
	PlayerReserve(tableID string, joinPlayer pokertable.JoinPlayer) error
	StartTableGame(tableID string) error
	TableGameOpen(tableID string) error
	BalanceTable(tableID string) error
	UpdateBlind(tableID string, level int, ante, dealer, sb, bb int64) error

	// TODO: test only, should remove this
	UpdateTable(table *pokertable.Table)
	PlayerJoin(tableID, playerID string) error
}

func NewTableManagerBackend(manager pokertable.Manager) TableManagerBackend {
	backend := tableManagerBackend{
		manager:        manager,
		onTableUpdated: func(t *pokertable.Table) {},
	}
	return &backend
}

type tableManagerBackend struct {
	manager        pokertable.Manager
	onTableUpdated func(table *pokertable.Table)
}

func (tmb *tableManagerBackend) OnTableUpdated(fn func(table *pokertable.Table)) {
	tmb.onTableUpdated = fn
}

func (tmb *tableManagerBackend) CreateTable(setting pokertable.TableSetting) (*pokertable.Table, error) {
	table, err := tmb.manager.CreateTable(setting)
	if err != nil {
		return nil, err
	}

	te, err := tmb.manager.GetTableEngine(table.ID)
	if err != nil {
		return nil, err
	}

	te.OnTableUpdated(func(t *pokertable.Table) {
		data, err := json.Marshal(t)
		if err != nil {
			return
		}

		var cloneTable pokertable.Table
		err = json.Unmarshal([]byte(data), &cloneTable)
		if err != nil {
			return
		}

		tmb.onTableUpdated(&cloneTable)
	})

	return table, nil
}

func (tmb *tableManagerBackend) CloseTable(tableID string) error {
	return tmb.manager.CloseTable(tableID)
}

func (tmb *tableManagerBackend) PlayersBatchReserve(tableID string, joinPlayers []pokertable.JoinPlayer) error {
	return tmb.manager.PlayersBatchReserve(tableID, joinPlayers)
}

func (tmb *tableManagerBackend) PlayersLeave(tableID string, playerIDs []string) error {
	return tmb.manager.PlayersLeave(tableID, playerIDs)
}

func (tmb *tableManagerBackend) PlayerRedeemChips(tableID string, joinPlayer pokertable.JoinPlayer) error {
	return tmb.manager.PlayerRedeemChips(tableID, joinPlayer)
}

func (tbm *tableManagerBackend) PlayerReserve(tableID string, joinPlayer pokertable.JoinPlayer) error {
	return tbm.manager.PlayerReserve(tableID, joinPlayer)
}

func (tbm *tableManagerBackend) StartTableGame(tableID string) error {
	return tbm.manager.StartTableGame(tableID)
}

func (tbm *tableManagerBackend) TableGameOpen(tableID string) error {
	return tbm.manager.TableGameOpen(tableID)
}

func (tbm *tableManagerBackend) BalanceTable(tableID string) error {
	return tbm.manager.BalanceTable(tableID)
}

func (tbm *tableManagerBackend) UpdateBlind(tableID string, level int, ante, dealer, sb, bb int64) error {
	return tbm.manager.UpdateBlind(tableID, level, ante, dealer, sb, bb)
}

func (tbm *tableManagerBackend) UpdateTable(table *pokertable.Table) {
	tbm.onTableUpdated(table)
}

func (tbm *tableManagerBackend) PlayerJoin(tableID, playerID string) error {
	return tbm.manager.PlayerJoin(tableID, playerID)
}
