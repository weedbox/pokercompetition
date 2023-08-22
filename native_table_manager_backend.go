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

func NewNativeTableManagerBackend(manager pokertable.Manager) TableManagerBackend {
	backend := nativeTableManagerBackend{
		manager:        manager,
		onTableUpdated: func(t *pokertable.Table) {},
	}
	return &backend
}

type nativeTableManagerBackend struct {
	manager        pokertable.Manager
	onTableUpdated func(table *pokertable.Table)
}

func (ntmb *nativeTableManagerBackend) OnTableUpdated(fn func(table *pokertable.Table)) {
	ntmb.onTableUpdated = fn
}

func (ntmb *nativeTableManagerBackend) CreateTable(setting pokertable.TableSetting) (*pokertable.Table, error) {
	tableUpdatedCallBack := func(t *pokertable.Table) {
		data, err := json.Marshal(t)
		if err != nil {
			return
		}

		var cloneTable pokertable.Table
		err = json.Unmarshal([]byte(data), &cloneTable)
		if err != nil {
			return
		}

		ntmb.onTableUpdated(&cloneTable)
	}
	tableErrorUpdatedCallBack := func(t *pokertable.Table, err error) {}
	tableStateUpdatedCallBack := func(event string, t *pokertable.Table) {}
	tablePlayerStateUpdatedCallBack := func(competitionID, tableID string, ps *pokertable.TablePlayerState) {}
	table, err := ntmb.manager.CreateTable(setting, tableUpdatedCallBack, tableErrorUpdatedCallBack, tableStateUpdatedCallBack, tablePlayerStateUpdatedCallBack)
	if err != nil {
		return nil, err
	}

	return table, nil
}

func (ntmb *nativeTableManagerBackend) CloseTable(tableID string) error {
	return ntmb.manager.CloseTable(tableID)
}

func (ntmb *nativeTableManagerBackend) PlayersBatchReserve(tableID string, joinPlayers []pokertable.JoinPlayer) error {
	return ntmb.manager.PlayersBatchReserve(tableID, joinPlayers)
}

func (ntmb *nativeTableManagerBackend) PlayersLeave(tableID string, playerIDs []string) error {
	return ntmb.manager.PlayersLeave(tableID, playerIDs)
}

func (ntmb *nativeTableManagerBackend) PlayerRedeemChips(tableID string, joinPlayer pokertable.JoinPlayer) error {
	return ntmb.manager.PlayerRedeemChips(tableID, joinPlayer)
}

func (ntbm *nativeTableManagerBackend) PlayerReserve(tableID string, joinPlayer pokertable.JoinPlayer) error {
	return ntbm.manager.PlayerReserve(tableID, joinPlayer)
}

func (ntbm *nativeTableManagerBackend) StartTableGame(tableID string) error {
	return ntbm.manager.StartTableGame(tableID)
}

func (ntbm *nativeTableManagerBackend) TableGameOpen(tableID string) error {
	return ntbm.manager.TableGameOpen(tableID)
}

func (ntbm *nativeTableManagerBackend) BalanceTable(tableID string) error {
	return ntbm.manager.BalanceTable(tableID)
}

func (ntbm *nativeTableManagerBackend) UpdateBlind(tableID string, level int, ante, dealer, sb, bb int64) error {
	return ntbm.manager.UpdateBlind(tableID, level, ante, dealer, sb, bb)
}

func (ntbm *nativeTableManagerBackend) UpdateTable(table *pokertable.Table) {
	ntbm.onTableUpdated(table)
}

func (ntbm *nativeTableManagerBackend) PlayerJoin(tableID, playerID string) error {
	return ntbm.manager.PlayerJoin(tableID, playerID)
}
