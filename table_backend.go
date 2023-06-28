package pokercompetition

import (
	"fmt"

	"github.com/weedbox/pokertable"
)

type TableBackend interface {
	DeleteTable(tableID string) error
	PlayerJoin(tableID string, joinPlayer pokertable.JoinPlayer) error
	StartTableGame(tableID string) error
	PlayerRedeemChips(tableID string, joinPlayer pokertable.JoinPlayer) error
	PlayersLeave(tableID string, playerIDs []string) error
	CreateTable(setting pokertable.TableSetting) (*pokertable.Table, error)
	TableGameOpen(tableID string) error
	BalanceTable(tableID string) error
	OnTableUpdated(fn func(table *pokertable.Table))
	UpdateTable(table *pokertable.Table)
}

func NewTableBackend(engine pokertable.TableEngine) TableBackend {
	backend := tableBackend{
		onTableUpdated: func(t *pokertable.Table) {},
		engine:         engine,
	}
	engine.OnTableUpdated(func(table *pokertable.Table) {
		fmt.Println("[competitionEngine#TableBackend] Table: ", table.State.Status)
		backend.onTableUpdated(table)
	})

	return &backend
}

type tableBackend struct {
	engine         pokertable.TableEngine
	onTableUpdated func(table *pokertable.Table)
}

func (tb *tableBackend) DeleteTable(tableID string) error {
	return tb.engine.DeleteTable(tableID)
}

func (tb *tableBackend) PlayerJoin(tableID string, joinPlayer pokertable.JoinPlayer) error {
	// return tb.engine.PlayerJoin(tableID, pokertable.JoinPlayer{
	// 	PlayerID:    playerID,
	// 	RedeemChips: redeemChips,
	// })
	return tb.engine.PlayerJoin(tableID, joinPlayer)
}

func (tb *tableBackend) StartTableGame(tableID string) error {
	return tb.engine.StartTableGame(tableID)
}

func (tb *tableBackend) PlayerRedeemChips(tableID string, joinPlayer pokertable.JoinPlayer) error {
	// return tb.engine.PlayerRedeemChips(tableID, pokertable.JoinPlayer{
	// 	PlayerID:    playerID,
	// 	RedeemChips: redeemChips,
	// })
	return tb.engine.PlayerRedeemChips(tableID, joinPlayer)
}

func (tb *tableBackend) PlayersLeave(tableID string, playerIDs []string) error {
	return tb.engine.PlayersLeave(tableID, playerIDs)
}

func (tb *tableBackend) CreateTable(setting pokertable.TableSetting) (*pokertable.Table, error) {
	return tb.engine.CreateTable(setting)
}

func (tb *tableBackend) TableGameOpen(tableID string) error {
	return tb.engine.TableGameOpen(tableID)
}

func (tb *tableBackend) BalanceTable(tableID string) error {
	return tb.engine.BalanceTable(tableID)
}

func (tb *tableBackend) OnTableUpdated(fn func(table *pokertable.Table)) {
	fmt.Println("[competitionEngine#TableBackend] OnTableUpdated")
	tb.onTableUpdated = fn
}

func (tb *tableBackend) UpdateTable(table *pokertable.Table) {
	tb.onTableUpdated(table)
}
