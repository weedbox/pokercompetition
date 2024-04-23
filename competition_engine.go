package pokercompetition

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/thoas/go-funk"
	pokerblind "github.com/weedbox/pokercompetition/blind"
	"github.com/weedbox/pokerface/regulator"
	"github.com/weedbox/pokertable"
	"github.com/weedbox/timebank"
)

var (
	ErrCompetitionInvalidCreateSetting            = errors.New("competition: invalid create competition setting")
	ErrCompetitionStartRejected                   = errors.New("competition: already started")
	ErrCompetitionUpdateBlindInitialLevelRejected = errors.New("competition: not allowed to update blind initial level")
	ErrCompetitionLeaveRejected                   = errors.New("competition: not allowed to leave")
	ErrCompetitionRefundRejected                  = errors.New("competition: not allowed to refund")
	ErrCompetitionQuitRejected                    = errors.New("competition: not allowed to quit")
	ErrCompetitionNoRedeemChips                   = errors.New("competition: not redeem any chips")
	ErrCompetitionAddonRejected                   = errors.New("competition: not allowed to addon")
	ErrCompetitionReBuyRejected                   = errors.New("competition: not allowed to re-buy")
	ErrCompetitionBuyInRejected                   = errors.New("competition: not allowed to buy in")
	ErrCompetitionExceedReBuyLimit                = errors.New("competition: exceed re-buy limit")
	ErrCompetitionExceedAddonLimit                = errors.New("competition: exceed addon limit")
	ErrCompetitionPlayerNotFound                  = errors.New("competition: player not found")
	ErrCompetitionTableNotFound                   = errors.New("competition: table not found")
	ErrMatchInitFailed                            = errors.New("competition: failed to init match")
	ErrMatchTableReservePlayerFailed              = errors.New("competition: failed to balance player to table by match")
)

type CompetitionEngineOpt func(*competitionEngine)

type CompetitionEngine interface {
	// Events
	OnCompetitionUpdated(fn func(competition *Competition))                                         // 賽事更新事件監聽器
	OnCompetitionErrorUpdated(fn func(competition *Competition, err error))                         // 賽事錯誤更新事件監聽器
	OnCompetitionPlayerUpdated(fn func(competitionID string, competitionPlayer *CompetitionPlayer)) // 賽事玩家更新事件監聽器
	OnCompetitionFinalPlayerRankUpdated(fn func(competitionID, playerID string, rank int))          // 賽事玩家最終名次監聽器
	OnCompetitionStateUpdated(fn func(event string, competition *Competition))                      // 賽事狀態監聽器
	OnAdvancePlayerCountUpdated(fn func(competitionID string, totalBuyInCount int) int)             // 賽事晉級人數更新監聽器
	OnCompetitionPlayerCashOut(fn func(competitionID string, competitionPlayer *CompetitionPlayer)) // 現金桌賽事玩家結算事件監聽器
	OnTableCreated(fn func(table *pokertable.Table))                                                // TODO: Test Only

	// Competition Actions
	GetCompetition() *Competition                                                  // 取得賽事
	CreateCompetition(competitionSetting CompetitionSetting) (*Competition, error) // 建立賽事
	UpdateCompetitionBlindInitialLevel(level int) error                            // 更新賽事盲注初始等級
	CloseCompetition(endStatus CompetitionStateStatus) error                       // 關閉賽事
	StartCompetition() (int64, error)                                              // 開始賽事

	// Player Operations
	PlayerBuyIn(joinPlayer JoinPlayer) error                 // 玩家報名或補碼
	PlayerAddon(tableID string, joinPlayer JoinPlayer) error // 玩家增購
	PlayerRefund(playerID string) error                      // 玩家退賽
	PlayerCashOut(tableID, playerID string) error            // 玩家離桌結算 (現金桌)
	PlayerQuit(tableID, playerID string) error               // 玩家棄賽淘汰

	// Others
	UpdateTable(table *pokertable.Table)                                                    // 桌次更新
	UpdateReserveTablePlayerState(tableID string, playerState *pokertable.TablePlayerState) // 更新 Reserve 桌次玩家狀態
	ReleaseTables() error                                                                   // 釋放所有桌次
}

type competitionEngine struct {
	mu                                  sync.RWMutex
	competition                         *Competition
	playerCaches                        sync.Map // key: <competitionID.playerID>, value: PlayerCache
	gameSettledRecords                  sync.Map // key: <tableID.game_count>, value: IsSettled
	tableOptions                        *pokertable.TableEngineOptions
	tableManagerBackend                 TableManagerBackend
	onCompetitionUpdated                func(competition *Competition)
	onCompetitionErrorUpdated           func(competition *Competition, err error)
	onCompetitionPlayerUpdated          func(competitionID string, competitionPlayer *CompetitionPlayer)
	onCompetitionFinalPlayerRankUpdated func(competitionID, playerID string, rank int)
	onCompetitionStateUpdated           func(event string, competition *Competition)
	onAdvancePlayerCountUpdated         func(competitionID string, totalBuyInCount int) int
	onCompetitionPlayerCashOut          func(competitionID string, competitionPlayer *CompetitionPlayer)
	breakingPauseResumeStates           map[string]map[int]bool // key: tableID, value: (k,v): (breaking blind level index, is resume from pause)
	blind                               pokerblind.Blind
	regulator                           regulator.Regulator
	isStarted                           bool

	// TODO: Test Only
	onTableCreated func(table *pokertable.Table)
}

func NewCompetitionEngine(opts ...CompetitionEngineOpt) CompetitionEngine {
	ce := &competitionEngine{
		playerCaches:                        sync.Map{},
		gameSettledRecords:                  sync.Map{},
		onCompetitionUpdated:                func(competition *Competition) {},
		onCompetitionErrorUpdated:           func(competition *Competition, err error) {},
		onCompetitionPlayerUpdated:          func(competitionID string, competitionPlayer *CompetitionPlayer) {},
		onCompetitionFinalPlayerRankUpdated: func(competitionID, playerID string, rank int) {},
		onCompetitionStateUpdated:           func(event string, competition *Competition) {},
		onAdvancePlayerCountUpdated:         func(competitionID string, totalBuyInCount int) int { return 0 },
		onCompetitionPlayerCashOut:          func(competitionID string, competitionPlayer *CompetitionPlayer) {},
		breakingPauseResumeStates:           make(map[string]map[int]bool),
		blind:                               pokerblind.NewBlind(),
		isStarted:                           false,

		// TODO: Test Only
		onTableCreated: func(table *pokertable.Table) {},
	}

	for _, opt := range opts {
		opt(ce)
	}

	return ce
}

func WithTableOptions(opts *pokertable.TableEngineOptions) CompetitionEngineOpt {
	return func(ce *competitionEngine) {
		ce.tableOptions = opts
	}
}

func WithTableManagerBackend(tmb TableManagerBackend) CompetitionEngineOpt {
	return func(ce *competitionEngine) {
		ce.tableManagerBackend = tmb
		ce.tableManagerBackend.OnTableUpdated(func(table *pokertable.Table) {
			ce.UpdateTable(table)
		})
		ce.tableManagerBackend.OnTablePlayerReserved(func(tableID string, playerState *pokertable.TablePlayerState) {
			ce.UpdateReserveTablePlayerState(tableID, playerState)
		})
	}
}

func (ce *competitionEngine) OnCompetitionUpdated(fn func(competition *Competition)) {
	ce.onCompetitionUpdated = fn
}

func (ce *competitionEngine) OnCompetitionErrorUpdated(fn func(competition *Competition, err error)) {
	ce.onCompetitionErrorUpdated = fn
}

func (ce *competitionEngine) OnCompetitionPlayerUpdated(fn func(competitionID string, competitionPlayer *CompetitionPlayer)) {
	ce.onCompetitionPlayerUpdated = fn
}

func (ce *competitionEngine) OnCompetitionFinalPlayerRankUpdated(fn func(competitionID, playerID string, rank int)) {
	ce.onCompetitionFinalPlayerRankUpdated = fn
}

func (ce *competitionEngine) OnCompetitionStateUpdated(fn func(event string, competition *Competition)) {
	ce.onCompetitionStateUpdated = fn
}

func (ce *competitionEngine) OnAdvancePlayerCountUpdated(fn func(competitionID string, totalBuyInCount int) int) {
	ce.onAdvancePlayerCountUpdated = fn
}

func (ce *competitionEngine) OnCompetitionPlayerCashOut(fn func(competitionID string, competitionPlayer *CompetitionPlayer)) {
	ce.onCompetitionPlayerCashOut = fn
}

// TODO: Test Only
func (ce *competitionEngine) OnTableCreated(fn func(table *pokertable.Table)) {
	ce.onTableCreated = fn
}

func (ce *competitionEngine) GetCompetition() *Competition {
	return ce.competition
}

func (ce *competitionEngine) CreateCompetition(competitionSetting CompetitionSetting) (*Competition, error) {
	// validate competitionSetting
	now := time.Now()
	if competitionSetting.StartAt != UnsetValue && competitionSetting.StartAt < now.Unix() {
		return nil, ErrCompetitionInvalidCreateSetting
	}

	if competitionSetting.DisableAt < now.Unix() {
		return nil, ErrCompetitionInvalidCreateSetting
	}

	for _, tableSetting := range competitionSetting.TableSettings {
		if len(tableSetting.JoinPlayers) > competitionSetting.Meta.TableMaxSeatCount {
			return nil, ErrCompetitionInvalidCreateSetting
		}
	}

	// setup blind
	ce.initBlind(competitionSetting.Meta)

	// create competition instance
	endAts := make([]int64, 0)
	for i := 0; i < len(competitionSetting.Meta.Blind.Levels); i++ {
		endAts = append(endAts, 0)
	}
	ce.competition = &Competition{
		ID:   competitionSetting.CompetitionID,
		Meta: competitionSetting.Meta,
		State: &CompetitionState{
			OpenAt:    time.Now().Unix(),
			DisableAt: competitionSetting.DisableAt,
			StartAt:   competitionSetting.StartAt,
			EndAt:     UnsetValue,
			Players:   make([]*CompetitionPlayer, 0),
			Status:    CompetitionStateStatus_Registering,
			Tables:    make([]*pokertable.Table, 0),
			Rankings:  make([]*CompetitionRank, 0),
			BlindState: &BlindState{
				FinalBuyInLevelIndex: competitionSetting.Meta.Blind.FinalBuyInLevelIndex,
				CurrentLevelIndex:    UnsetValue,
				EndAts:               endAts,
			},
			AdvanceState: &AdvanceState{
				Status:        CompetitionAdvanceStatus_NotStart,
				TotalTables:   -1,
				UpdatedTables: -1,
			},
			Statistic: &Statistic{
				TotalBuyInCount: 0,
			},
		},
	}

	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT, CompetitionMode_Cash:
		// 批次建立桌次
		for _, tableSetting := range competitionSetting.TableSettings {
			if _, err := ce.addCompetitionTable(tableSetting); err != nil {
				return nil, err
			}
		}
	case CompetitionMode_MTT:
		// 初始化拆併桌監管器
		if ce.regulator == nil {
			ce.regulator = regulator.NewRegulator(
				regulator.MinInitialPlayers(competitionSetting.Meta.RegulatorMinInitialPlayerCount),
				regulator.WithRequestTableFn(func(playerIDs []string) (string, error) {
					return ce.regulatorCreateAndDistributePlayers(playerIDs)
				}),
				regulator.WithAssignPlayersFn(func(tableID string, playerIDs []string) error {
					return ce.regulatorDistributePlayers(tableID, playerIDs)
				}),
			)
			ce.regulator.SetStatus(regulator.CompetitionStatus_Pending)
		}
	}

	// auto startCompetition when StartAt is reached
	if ce.competition.State.StartAt > 0 {
		autoStartTime := time.Unix(ce.competition.State.StartAt, 0)
		if err := timebank.NewTimeBank().NewTaskWithDeadline(autoStartTime, func(isCancelled bool) {
			if isCancelled {
				return
			}

			if ce.competition.State.Status == CompetitionStateStatus_Registering {
				ce.StartCompetition()
			}
		}); err != nil {
			return nil, err
		}
	}

	// AutoEnd (When Disable Time is reached)
	disableAutoCloseTime := time.Unix(ce.competition.State.DisableAt, 0)
	if err := timebank.NewTimeBank().NewTaskWithDeadline(disableAutoCloseTime, func(isCancelled bool) {
		if isCancelled {
			return
		}

		if ce.competition.State.Status == CompetitionStateStatus_Registering {
			if len(ce.competition.State.Players) < ce.competition.Meta.MinPlayerCount {
				ce.CloseCompetition(CompetitionStateStatus_AutoEnd)
			}
		}
	}); err != nil {
		return nil, err
	}

	ce.emitEvent("CreateCompetition", "")
	return ce.competition, nil
}

/*
UpdateCompetitionBlindInitialLevel 更新賽事盲注初始等級
  - 適用時機: 主賽 Day 1 所有賽事結束後，更新主賽 Day 2 盲注初始等級
*/
func (ce *competitionEngine) UpdateCompetitionBlindInitialLevel(level int) error {
	// 開賽後就不能再更新初始等級
	if ce.competition.State.Status != CompetitionStateStatus_Registering {
		return ErrCompetitionUpdateBlindInitialLevelRejected
	}

	return ce.blind.UpdateInitialLevel(level)
}

/*
CloseCompetition 關閉賽事
  - 適用時機: 賽事出狀況需要臨時關閉賽事、未達開賽條件自動關閉賽事、正常結束賽事
*/
func (ce *competitionEngine) CloseCompetition(endStatus CompetitionStateStatus) error {
	ce.settleCompetition(endStatus)
	_ = ce.ReleaseTables()
	return nil
}

/*
StartCompetition 開賽
  - 適用時機: MTT 手動開賽、MTT 自動開賽、CT 開賽
*/
func (ce *competitionEngine) StartCompetition() (int64, error) {
	if ce.isStarted {
		return ce.competition.State.StartAt, ErrCompetitionStartRejected
	}

	// update start & end at
	ce.competition.State.StartAt = time.Now().Unix()

	if ce.competition.Meta.Mode == CompetitionMode_CT {
		ce.competition.State.EndAt = ce.competition.State.StartAt + int64((time.Duration(ce.competition.Meta.MaxDuration) * time.Second).Seconds())
	} else if ce.competition.Meta.Mode == CompetitionMode_MTT {
		ce.competition.State.EndAt = UnsetValue
	}

	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT:
		// 時間到了要結束賽事的機制
		normalCloseTime := time.Unix(ce.competition.State.EndAt, 0)
		if err := timebank.NewTimeBank().NewTaskWithDeadline(normalCloseTime, func(isCancelled bool) {
			if isCancelled {
				return
			}

			// 賽事已結算，不再處理
			if ce.isEndStatus() {
				return
			}

			if len(ce.competition.State.Tables) > 0 {
				noneCloseTableStatuses := []pokertable.TableStateStatus{
					// playing
					pokertable.TableStateStatus_TableGameOpened,
					pokertable.TableStateStatus_TableGamePlaying,
					pokertable.TableStateStatus_TableGameSettled,

					// not playing
					pokertable.TableStateStatus_TableClosed,
				}
				// 桌次尚未結束，處理關桌
				if !funk.Contains(noneCloseTableStatuses, ce.competition.State.Tables[0].State.Status) {
					if err := ce.tableManagerBackend.CloseTable(ce.competition.State.Tables[0].ID); err != nil {
						ce.emitErrorEvent("end time auto close -> CloseTable", "", err)
					}
				}
			}
		}); err != nil {
			return ce.competition.State.StartAt, err
		}
	case CompetitionMode_MTT:
		// 更新拆併桌監管器狀態
		ce.regulator.SetStatus(regulator.CompetitionStatus_Normal)
	}

	ce.isStarted = true
	ce.emitEvent("StartCompetition", "")
	ce.emitCompetitionStateEvent(CompetitionStateEvent_Started)
	return ce.competition.State.StartAt, nil
}

func (ce *competitionEngine) PlayerBuyIn(joinPlayer JoinPlayer) error {
	// validate join player data
	if joinPlayer.RedeemChips <= 0 {
		return ErrCompetitionNoRedeemChips
	}

	playerIdx := ce.competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == joinPlayer.PlayerID
	})
	isBuyIn := playerIdx == UnsetValue
	validStatuses := []CompetitionStateStatus{
		CompetitionStateStatus_Registering,
		CompetitionStateStatus_DelayedBuyIn,
	}
	if !funk.Contains(validStatuses, ce.competition.State.Status) {
		if playerIdx == UnsetValue {
			return ErrCompetitionBuyInRejected
		} else {
			return ErrCompetitionReBuyRejected
		}
	}

	if ce.competition.Meta.Mode == CompetitionMode_CT || ce.competition.Meta.Mode == CompetitionMode_MTT {
		if !isBuyIn {
			cp := ce.competition.State.Players[playerIdx]

			// validate re-buy player
			if cp.Status == CompetitionPlayerStatus_Knockout {
				return ErrCompetitionReBuyRejected
			}

			// validate re-buy conditions
			if cp.Status != CompetitionPlayerStatus_ReBuyWaiting {
				return ErrCompetitionReBuyRejected
			}

			if cp.Chips > 0 {
				return ErrCompetitionReBuyRejected
			}

			if cp.ReBuyTimes >= ce.competition.Meta.ReBuySetting.MaxTime {
				return ErrCompetitionExceedReBuyLimit
			}
		} else {
			// check ct buy in conditions
			if ce.competition.Meta.Mode == CompetitionMode_CT {
				if len(ce.competition.State.Tables) == 0 {
					return ErrCompetitionTableNotFound
				}

				if len(ce.competition.State.Tables[0].State.PlayerStates) >= ce.competition.State.Tables[0].Meta.TableMaxSeatCount {
					return ErrCompetitionBuyInRejected
				}
			}
		}
	}

	tableID := ""
	if (ce.competition.Meta.Mode == CompetitionMode_CT || ce.competition.Meta.Mode == CompetitionMode_Cash) && len(ce.competition.State.Tables) > 0 {
		tableID = ce.competition.State.Tables[0].ID
	}

	playerStatus := CompetitionPlayerStatus_Playing
	if ce.competition.Meta.Mode == CompetitionMode_MTT {
		// MTT 玩家狀態每次進入 BuyIn/ReBuy 皆為等待拆併桌中
		playerStatus = CompetitionPlayerStatus_WaitingTableBalancing
	}

	// 更新統計數據 (MTT)
	if ce.competition.Meta.Mode == CompetitionMode_MTT {
		// MTT TotalBuyInCount 一次一發
		ce.competition.State.Statistic.TotalBuyInCount++
	}

	// do logic
	ce.mu.Lock()
	if isBuyIn {
		player, playerCache := ce.newDefaultCompetitionPlayerData(tableID, joinPlayer.PlayerID, joinPlayer.RedeemChips, playerStatus)
		ce.competition.State.Players = append(ce.competition.State.Players, &player)
		playerCache.PlayerIdx = len(ce.competition.State.Players) - 1
		ce.insertPlayerCache(ce.competition.ID, joinPlayer.PlayerID, playerCache)
		ce.emitEvent(fmt.Sprintf("PlayerBuyIn -> %s Buy In", joinPlayer.PlayerID), joinPlayer.PlayerID)
		ce.emitPlayerEvent("PlayerBuyIn -> Buy In", &player)
	} else {
		// ReBuy logic
		playerCache, exist := ce.getPlayerCache(ce.competition.ID, joinPlayer.PlayerID)
		if !exist {
			return ErrCompetitionPlayerNotFound
		}

		cp := ce.competition.State.Players[playerIdx]
		cp.Status = playerStatus
		cp.Chips = joinPlayer.RedeemChips
		cp.ReBuyTimes++
		playerCache.ReBuyTimes = cp.ReBuyTimes
		cp.IsReBuying = false
		cp.ReBuyEndAt = UnsetValue
		cp.TotalRedeemChips += joinPlayer.RedeemChips
		if (ce.competition.Meta.Mode == CompetitionMode_CT || ce.competition.Meta.Mode == CompetitionMode_Cash) && len(ce.competition.State.Tables) > 0 {
			playerCache.TableID = ce.competition.State.Tables[0].ID
			cp.CurrentTableID = ce.competition.State.Tables[0].ID
		}
		if ce.competition.Meta.Mode == CompetitionMode_MTT {
			playerCache.TableID = ""
			cp.CurrentTableID = "" // re-buy 時要清空 CurrentTableID 等待重新配桌
			cp.CurrentSeat = UnsetValue
		}
		ce.emitEvent(fmt.Sprintf("PlayerBuyIn -> %s Re Buy", joinPlayer.PlayerID), joinPlayer.PlayerID)
		ce.emitPlayerEvent("PlayerBuyIn -> Re Buy", cp)
	}
	defer ce.mu.Unlock()

	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT, CompetitionMode_Cash:
		// call tableEngine
		jp := pokertable.JoinPlayer{
			PlayerID:    joinPlayer.PlayerID,
			RedeemChips: joinPlayer.RedeemChips,
			Seat:        pokertable.UnsetValue,
		}
		if err := ce.tableManagerBackend.PlayerReserve(tableID, jp); err != nil {
			ce.emitErrorEvent("PlayerBuyIn -> PlayerReserve", joinPlayer.PlayerID, err)
		}
	case CompetitionMode_MTT:
		// 一律丟到拆併桌監管器等待配桌
		err := ce.regulatorAddPlayers([]string{joinPlayer.PlayerID})
		if err != nil {
			ce.emitErrorEvent("PlayerBuyIn -> AddPlayers to regulator failed", joinPlayer.PlayerID, err)
			return err
		}
	}

	return nil
}

func (ce *competitionEngine) PlayerAddon(tableID string, joinPlayer JoinPlayer) error {
	// validate join player data
	if joinPlayer.RedeemChips <= 0 {
		return ErrCompetitionNoRedeemChips
	}

	playerIdx := ce.competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == joinPlayer.PlayerID
	})
	if playerIdx == UnsetValue {
		return ErrCompetitionAddonRejected
	}

	// validate Addon times
	if ce.competition.Meta.AddonSetting.IsBreakOnly && !ce.competition.IsBreaking() {
		return ErrCompetitionAddonRejected
	}

	if !ce.competition.CurrentBlindLevel().AllowAddon {
		return ErrCompetitionAddonRejected
	}

	cp := ce.competition.State.Players[playerIdx]
	if cp.AddonTimes >= ce.competition.Meta.AddonSetting.MaxTime {
		return ErrCompetitionExceedAddonLimit
	}

	// do logic
	ce.mu.Lock()
	cp.CurrentTableID = tableID
	cp.Chips += joinPlayer.RedeemChips
	cp.AddonTimes++
	cp.TotalRedeemChips += joinPlayer.RedeemChips
	defer ce.mu.Unlock()

	// emit events
	ce.emitEvent("PlayerAddon", joinPlayer.PlayerID)
	ce.emitPlayerEvent("PlayerAddon", cp)

	// call tableEngine
	jp := pokertable.JoinPlayer{
		PlayerID:    joinPlayer.PlayerID,
		RedeemChips: joinPlayer.RedeemChips,
		Seat:        pokertable.UnsetValue,
	}
	if err := ce.tableManagerBackend.PlayerRedeemChips(tableID, jp); err != nil {
		return err
	}

	return nil
}

func (ce *competitionEngine) PlayerRefund(playerID string) error {
	// validate refund conditions
	playerIdx := ce.competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == playerID
	})
	if playerIdx == UnsetValue {
		return ErrCompetitionRefundRejected
	}

	if ce.competition.State.Status != CompetitionStateStatus_Registering {
		return ErrCompetitionRefundRejected
	}

	playerTableID := ""
	if ce.competition.Meta.Mode == CompetitionMode_CT {
		playerCache, exist := ce.getPlayerCache(ce.competition.ID, playerID)
		if !exist {
			return ErrCompetitionPlayerNotFound
		}
		playerTableID = playerCache.TableID
	}

	// refund logic
	ce.mu.Lock()
	if ce.competition.Meta.Mode == CompetitionMode_MTT {
		// MTT TotalBuyInCount 一次一發
		ce.competition.State.Statistic.TotalBuyInCount--
	}
	ce.deletePlayer(playerIdx)
	ce.deletePlayerCache(ce.competition.ID, playerID)
	defer ce.mu.Unlock()

	// emit events
	ce.emitEvent("PlayerRefund", playerID)

	// call tableEngine
	if playerTableID != "" {
		if err := ce.tableManagerBackend.PlayersLeave(playerTableID, []string{playerID}); err != nil {
			return err
		}
	}

	return nil
}

func (ce *competitionEngine) PlayerCashOut(tableID, playerID string) error {
	// validate leave conditions
	playerIdx := ce.competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == playerID
	})
	if playerIdx == UnsetValue {
		return ErrCompetitionLeaveRejected
	}

	if ce.competition.Meta.Mode != CompetitionMode_Cash {
		return ErrCompetitionLeaveRejected
	}

	// update player status
	cp := ce.competition.State.Players[playerIdx]
	cp.Status = CompetitionPlayerStatus_CashLeaving
	ce.emitPlayerEvent("PlayerCashOut -> Cash Leaving", cp)

	// 尚未開賽時 or 已經開賽但是賽事只剩一人，則直接結算，玩家直接離桌結算
	competitionNotStart := ce.competition.State.Status == CompetitionStateStatus_Registering
	pauseCompetition := false
	if ce.competition.State.Status == CompetitionStateStatus_DelayedBuyIn && len(ce.competition.State.Tables) > 0 {
		if ce.competition.State.Tables[0].State.Status == pokertable.TableStateStatus_TablePausing {
			pauseCompetition = true
		}
	}

	if competitionNotStart || pauseCompetition {
		leavePlayerIndexes := map[string]int{
			playerID: playerIdx,
		}
		leavePlayerIDs := []string{playerID}
		ce.handleCashOut(tableID, leavePlayerIndexes, leavePlayerIDs)
	}

	return nil
}

func (ce *competitionEngine) PlayerQuit(tableID, playerID string) error {
	// validate quit conditions
	playerIdx := ce.competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == playerID
	})
	if playerIdx == UnsetValue {
		return ErrCompetitionQuitRejected
	}

	tableIdx := ce.competition.FindTableIdx(func(t *pokertable.Table) bool {
		return tableID == t.ID
	})
	if tableIdx == UnsetValue {
		return ErrCompetitionQuitRejected
	}

	// 停止買入時會自動淘汰沒籌碼玩家，不讓玩家主動棄賽
	if ce.competition.State.BlindState.IsStopBuyIn() {
		return ErrCompetitionQuitRejected
	}

	ce.mu.Lock()
	cp := ce.competition.State.Players[playerIdx]
	cp.Status = CompetitionPlayerStatus_ReBuyWaiting
	cp.IsReBuying = false
	cp.ReBuyEndAt = UnsetValue
	cp.CurrentSeat = UnsetValue
	defer ce.mu.Unlock()

	ce.emitPlayerEvent("quit knockout", cp)
	ce.emitEvent("Player Quit", "")
	ce.emitCompetitionStateEvent(CompetitionStateEvent_KnockoutPlayers)

	if err := ce.tableManagerBackend.PlayersLeave(tableID, []string{playerID}); err != nil {
		ce.emitErrorEvent("Player Quit Knockout Players -> PlayersLeave", playerID, err)
	}

	return nil
}

func (ce *competitionEngine) UpdateReserveTablePlayerState(tableID string, playerState *pokertable.TablePlayerState) {
	// 更新玩家狀態
	playerCache, exist := ce.getPlayerCache(ce.competition.ID, playerState.PlayerID)
	if !exist {
		return
	}

	cp := ce.competition.State.Players[playerCache.PlayerIdx]
	cp.CurrentSeat = playerState.Seat
	cp.CurrentTableID = tableID
	cp.Status = CompetitionPlayerStatus_Playing
	ce.emitPlayerEvent("[UpdateReserveTablePlayerState] player table seat updated", cp)
	ce.emitEvent(fmt.Sprintf("[UpdateReserveTablePlayerState] player (%s) is reserved to table (%s) at seat (%d)", cp.PlayerID, cp.CurrentTableID, cp.CurrentSeat), cp.PlayerID)
}

func (ce *competitionEngine) ReleaseTables() error {
	for _, table := range ce.competition.State.Tables {
		_ = ce.tableManagerBackend.ReleaseTable(table.ID)
	}
	return nil
}

func (ce *competitionEngine) UpdateTable(table *pokertable.Table) {
	tableIdx := ce.competition.FindTableIdx(func(t *pokertable.Table) bool {
		return table.ID == t.ID
	})
	if tableIdx == UnsetValue {
		return
	}

	if ce.isEndStatus() {
		// fmt.Println("[DEBUG#UpdateTable] status is end, no need to update table. Status:", string(ce.competition.State.Status))
		return
	}

	// 更新 competition table
	var cloneTable pokertable.Table
	if encoded, err := json.Marshal(table); err == nil {
		json.Unmarshal(encoded, &cloneTable)
	} else {
		cloneTable = *table
	}
	ce.competition.State.Tables[tableIdx] = &cloneTable

	// 處理因 table status 產生的變化
	tableStatusHandlerMap := map[pokertable.TableStateStatus]func(pokertable.Table, int){
		pokertable.TableStateStatus_TableCreated:     ce.handleCompetitionTableCreated,
		pokertable.TableStateStatus_TableBalancing:   ce.handleCompetitionTableBalancing,
		pokertable.TableStateStatus_TablePausing:     ce.updatePauseCompetition,
		pokertable.TableStateStatus_TableClosed:      ce.closeCompetitionTable,
		pokertable.TableStateStatus_TableGameSettled: ce.settleCompetitionTable,
	}
	handler, ok := tableStatusHandlerMap[cloneTable.State.Status]
	if !ok {
		return
	}
	handler(cloneTable, tableIdx)
}
