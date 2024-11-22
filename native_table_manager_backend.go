package pokercompetition

import (
	"encoding/json"

	"github.com/weedbox/pokertable"
)

type TableManagerBackend interface {
	// Events
	OnTableUpdated(fn func(table *pokertable.Table))
	OnTablePlayerReserved(fn func(tableID string, playerState *pokertable.TablePlayerState))
	OnReadyOpenFirstTableGame(fn func(tableID string, gameCount int, playerStates []*pokertable.TablePlayerState))

	// TableManager Table Actions
	CreateTable(options *pokertable.TableEngineOptions, setting pokertable.TableSetting) (*pokertable.Table, error)
	PauseTable(tableID string) error
	CloseTable(tableID string) error
	StartTableGame(tableID string) error
	SetUpTableGame(tableID string, gameCount int, participants map[string]int) error
	UpdateBlind(tableID string, level int, ante, dealer, sb, bb int64) error
	UpdateTablePlayers(tableID string, joinPlayers []pokertable.JoinPlayer, leavePlayerIDs []string) (map[string]int, error)

	// TableManager Player Table Actions
	PlayerReserve(tableID string, joinPlayer pokertable.JoinPlayer) error
	PlayerJoin(tableID, playerID string) error // TODO: test only, should remove this
	PlayerRedeemChips(tableID string, joinPlayer pokertable.JoinPlayer) error
	PlayersLeave(tableID string, playerIDs []string) error

	// Others
	UpdateTable(table *pokertable.Table) // TODO: test only, should remove this
	ReleaseTable(tableID string) error
}

func NewNativeTableManagerBackend(manager pokertable.Manager) TableManagerBackend {
	backend := nativeTableManagerBackend{
		manager:                   manager,
		onTableUpdated:            func(t *pokertable.Table) {},
		onTablePlayerReserved:     func(tableID string, playerState *pokertable.TablePlayerState) {},
		onReadyOpenFirstTableGame: func(tableID string, ganeCoubt int, players []*pokertable.TablePlayerState) {},
	}
	return &backend
}

type nativeTableManagerBackend struct {
	manager                   pokertable.Manager
	onTableUpdated            func(table *pokertable.Table)
	onTablePlayerReserved     func(tableID string, playerState *pokertable.TablePlayerState)
	onReadyOpenFirstTableGame func(tableID string, ganeCoubt int, players []*pokertable.TablePlayerState)
}

func (ntmb *nativeTableManagerBackend) OnTableUpdated(fn func(table *pokertable.Table)) {
	ntmb.onTableUpdated = fn
}

func (ntmb *nativeTableManagerBackend) OnTablePlayerReserved(fn func(tableID string, playerState *pokertable.TablePlayerState)) {
	ntmb.onTablePlayerReserved = fn
}

func (ntmb *nativeTableManagerBackend) OnReadyOpenFirstTableGame(fn func(tableID string, gameCount int, playerStates []*pokertable.TablePlayerState)) {
	ntmb.onReadyOpenFirstTableGame = fn
}

func (ntmb *nativeTableManagerBackend) CreateTable(options *pokertable.TableEngineOptions, setting pokertable.TableSetting) (*pokertable.Table, error) {
	callbacks := pokertable.NewTableEngineCallbacks()
	callbacks.OnTableUpdated = func(t *pokertable.Table) {
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
	callbacks.OnTablePlayerReserved = func(competitionID, tableID string, playerState *pokertable.TablePlayerState) {
		data, err := json.Marshal(playerState)
		if err != nil {
			return
		}

		var clonePlayerState pokertable.TablePlayerState
		err = json.Unmarshal([]byte(data), &clonePlayerState)
		if err != nil {
			return
		}

		ntmb.onTablePlayerReserved(tableID, &clonePlayerState)
	}

	table, err := ntmb.manager.CreateTable(options, callbacks, setting)
	if err != nil {
		return nil, err
	}

	return table, nil
}

func (ntbm *nativeTableManagerBackend) PauseTable(tableID string) error {
	return ntbm.manager.PauseTable(tableID)
}

func (ntmb *nativeTableManagerBackend) CloseTable(tableID string) error {
	return ntmb.manager.CloseTable(tableID)
}

func (ntbm *nativeTableManagerBackend) StartTableGame(tableID string) error {
	return ntbm.manager.StartTableGame(tableID)
}

func (ntbm *nativeTableManagerBackend) SetUpTableGame(tableID string, gameCount int, participants map[string]int) error {
	return ntbm.manager.SetUpTableGame(tableID, gameCount, participants)
}

func (ntbm *nativeTableManagerBackend) UpdateBlind(tableID string, level int, ante, dealer, sb, bb int64) error {
	return ntbm.manager.UpdateBlind(tableID, level, ante, dealer, sb, bb)
}

func (ntbm *nativeTableManagerBackend) UpdateTablePlayers(tableID string, joinPlayers []pokertable.JoinPlayer, leavePlayerIDs []string) (map[string]int, error) {
	return ntbm.manager.UpdateTablePlayers(tableID, joinPlayers, leavePlayerIDs)
}

func (ntbm *nativeTableManagerBackend) PlayerReserve(tableID string, joinPlayer pokertable.JoinPlayer) error {
	return ntbm.manager.PlayerReserve(tableID, joinPlayer)
}

func (ntbm *nativeTableManagerBackend) PlayerJoin(tableID, playerID string) error {
	return ntbm.manager.PlayerJoin(tableID, playerID)
}

func (ntmb *nativeTableManagerBackend) PlayerRedeemChips(tableID string, joinPlayer pokertable.JoinPlayer) error {
	return ntmb.manager.PlayerRedeemChips(tableID, joinPlayer)
}

func (ntmb *nativeTableManagerBackend) PlayersLeave(tableID string, playerIDs []string) error {
	return ntmb.manager.PlayersLeave(tableID, playerIDs)
}

func (ntmb *nativeTableManagerBackend) UpdateTable(table *pokertable.Table) {
	ntmb.onTableUpdated(table)
}

func (ntmb *nativeTableManagerBackend) ReleaseTable(tableID string) error {
	return ntmb.manager.ReleaseTable(tableID)
}
