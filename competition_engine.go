package pokercompetition

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/thoas/go-funk"
	pokerblind "github.com/weedbox/pokercompetition/blind"
	"github.com/weedbox/pokerface/competition"
	"github.com/weedbox/pokerface/match"
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
	OnTableCreated(fn func(table *pokertable.Table)) // TODO: test only, delete it later on
	OnTableClosed(fn func(table *pokertable.Table))  // TODO: test only, delete it later on

	// Others
	UpdateTable(table *pokertable.Table)                                                    // 桌次更新
	UpdateReserveTablePlayerState(tableID string, playerState *pokertable.TablePlayerState) // 更新 Reserve 桌次玩家狀態

	// Events
	OnCompetitionUpdated(fn func(competition *Competition))                                         // 賽事更新事件監聽器
	OnCompetitionErrorUpdated(fn func(competition *Competition, err error))                         // 賽事錯誤更新事件監聽器
	OnCompetitionPlayerUpdated(fn func(competitionID string, competitionPlayer *CompetitionPlayer)) // 賽事玩家更新事件監聽器
	OnCompetitionFinalPlayerRankUpdated(fn func(competitionID, playerID string, rank int))          // 賽事玩家最終名次監聽器
	OnCompetitionStateUpdated(fn func(competitionID string, competition *Competition))              // 賽事狀態監聽器
	OnAdvancePlayerCountUpdated(fn func(competitionID string, totalBuyInCount int) int)             // 賽事晉級人數更新監聽器
	OnCompetitionPlayerCashOut(fn func(competitionID string, competitionPlayer *CompetitionPlayer)) // 現金桌賽事玩家結算事件監聽器

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

	// Match Apis
	MatchCreateTable() (string, error)                                // 拆併桌自動開桌
	MatchCloseTable(tableID string) error                             // 拆併桌自動關桌
	MatchTableReservePlayer(tableID, playerID string, seat int) error // 拆併桌玩家配至新桌
	MatchTableReservePlayerDone(tableID string) error                 // 拆併桌玩家配桌完成
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
	onCompetitionStateUpdated           func(competitionID string, competition *Competition)
	onAdvancePlayerCountUpdated         func(competitionID string, totalBuyInCount int) int
	onCompetitionPlayerCashOut          func(competitionID string, competitionPlayer *CompetitionPlayer)
	breakingPauseResumeStates           map[string]map[int]bool // key: tableID, value: (k,v): (breaking blind level index, is resume from pause)
	blind                               pokerblind.Blind
	match                               match.Match
	matchTableBackend                   match.TableBackend
	qm                                  match.QueueManager
	tablePlayerWaitingQueue             map[string]map[int]int        // key: tableID, value: (k,v): playerIdx, seat
	reBuyTimerStates                    map[string]*timebank.TimeBank // key: playerID, value: *timebank.TimeBank

	onTableCreated func(table *pokertable.Table) // TODO: test only, delete it later on
	onTableClosed  func(table *pokertable.Table) // TODO: test only, delete it later on
}

func NewCompetitionEngine(opts ...CompetitionEngineOpt) CompetitionEngine {
	ce := &competitionEngine{
		playerCaches:                        sync.Map{},
		gameSettledRecords:                  sync.Map{},
		onCompetitionUpdated:                func(competition *Competition) {},
		onCompetitionErrorUpdated:           func(competition *Competition, err error) {},
		onCompetitionPlayerUpdated:          func(competitionID string, competitionPlayer *CompetitionPlayer) {},
		onCompetitionFinalPlayerRankUpdated: func(competitionID, playerID string, rank int) {},
		onCompetitionStateUpdated:           func(competitionID string, competition *Competition) {},
		onAdvancePlayerCountUpdated:         func(competitionID string, totalBuyInCount int) int { return 0 },
		onCompetitionPlayerCashOut:          func(competitionID string, competitionPlayer *CompetitionPlayer) {},
		breakingPauseResumeStates:           make(map[string]map[int]bool),
		blind:                               pokerblind.NewBlind(),
		tablePlayerWaitingQueue:             make(map[string]map[int]int),
		reBuyTimerStates:                    make(map[string]*timebank.TimeBank),

		onTableCreated: func(table *pokertable.Table) {}, // TODO: test only, delete it later on
		onTableClosed:  func(table *pokertable.Table) {}, // TODO: test only, delete it later on
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

func WithMatchBackend(m match.Match) CompetitionEngineOpt {
	return func(ce *competitionEngine) {
		ce.match = m
	}
}

func WithQueueManagerOptions(qm match.QueueManager) CompetitionEngineOpt {
	return func(ce *competitionEngine) {
		ce.qm = qm
	}
}

// TODO: test only, delete it later on
func (ce *competitionEngine) OnTableCreated(fn func(table *pokertable.Table)) {
	ce.onTableCreated = fn
}

// TODO: test only, delete it later on
func (ce *competitionEngine) OnTableClosed(fn func(table *pokertable.Table)) {
	ce.onTableClosed = fn
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

func (ce *competitionEngine) OnCompetitionStateUpdated(fn func(competitionID string, competition *Competition)) {
	ce.onCompetitionStateUpdated = fn
}

func (ce *competitionEngine) OnAdvancePlayerCountUpdated(fn func(competitionID string, totalBuyInCount int) int) {
	ce.onAdvancePlayerCountUpdated = fn
}

func (ce *competitionEngine) OnCompetitionPlayerCashOut(fn func(competitionID string, competitionPlayer *CompetitionPlayer)) {
	ce.onCompetitionPlayerCashOut = fn
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
			if _, err := ce.addCompetitionTable(tableSetting, CompetitionPlayerStatus_Playing); err != nil {
				return nil, err
			}
		}
	case CompetitionMode_MTT:
		// Table backend of match
		if ce.matchTableBackend == nil {
			ce.matchTableBackend = NewNativeMatchTableBackend(ce)
		}

		if ce.match == nil {
			// Initializing match
			opts := match.NewOptions(ce.competition.ID)
			defaultOpts := competition.NewOptions()
			defaultOpts.TableAllocationPeriod = 3
			opts.WaitingPeriod = defaultOpts.TableAllocationPeriod
			opts.MaxTables = -1
			opts.MaxSeats = defaultOpts.Table.MaxSeats

			if ce.qm == nil {
				ce.match = match.NewMatch(
					opts,
					match.WithTableBackend(ce.matchTableBackend),
				)
			} else {
				ce.match = match.NewMatch(
					opts,
					match.WithTableBackend(ce.matchTableBackend),
					match.WithQueueManager(ce.qm),
				)
			}
		}

		if ce.match == nil {
			return nil, ErrMatchInitFailed
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
	return nil
}

/*
StartCompetition 開賽
  - 適用時機: MTT 手動開賽、MTT 自動開賽、CT 開賽
    TODO: 未達開賽人數時，不啟動盲注
*/
func (ce *competitionEngine) StartCompetition() (int64, error) {
	if ce.competition.State.Status != CompetitionStateStatus_Registering {
		return ce.competition.State.StartAt, ErrCompetitionStartRejected
	}

	// start the competition
	if ce.competition.Meta.Blind.FinalBuyInLevelIndex == UnsetValue || ce.competition.Meta.Blind.FinalBuyInLevelIndex < NoStopBuyInIndex {
		ce.competition.State.Status = CompetitionStateStatus_StoppedBuyIn
	} else {
		ce.competition.State.Status = CompetitionStateStatus_DelayedBuyIn
	}

	// update start & end at
	ce.competition.State.StartAt = time.Now().Unix()

	if ce.competition.Meta.Mode == CompetitionMode_CT {
		ce.competition.State.EndAt = ce.competition.State.StartAt + int64((time.Duration(ce.competition.Meta.MaxDuration) * time.Second).Seconds())
	} else if ce.competition.Meta.Mode == CompetitionMode_MTT {
		ce.competition.State.EndAt = -1
	}

	// 初始化盲注
	bs, err := ce.blind.Start()
	if err != nil {
		ce.emitErrorEvent("Start Blind Error", "", err)
		return 0, err
	}
	ce.competition.State.BlindState.CurrentLevelIndex = bs.Status.CurrentLevelIndex
	ce.competition.State.BlindState.FinalBuyInLevelIndex = bs.Status.FinalBuyInLevelIndex
	copy(ce.competition.State.BlindState.EndAts, bs.Status.LevelEndAts)

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

					// close
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
		ce.mu.RLock()

		// 拆併桌加入玩家
		for _, player := range ce.competition.State.Players {
			err := ce.match.Register(player.PlayerID)
			if err != nil {
				ce.emitErrorEvent("MTT StartCompetition Register Player to Match failed", player.PlayerID, err)
				ce.mu.RUnlock()
				return ce.competition.State.StartAt, err
			}
		}

		ce.mu.RUnlock()
	}

	ce.emitEvent("StartCompetition", "")
	ce.emitCompetitionStateEvent(CompetitionStateEvent_Started)
	return ce.competition.State.StartAt, nil
}

func (ce *competitionEngine) PlayerBuyIn(joinPlayer JoinPlayer) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

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

	// do logic
	if isBuyIn {
		ce.reBuyTimerStates[joinPlayer.PlayerID] = timebank.NewTimeBank()
		player, playerCache := ce.newDefaultCompetitionPlayerData(tableID, joinPlayer.PlayerID, joinPlayer.RedeemChips, playerStatus)
		ce.competition.State.Players = append(ce.competition.State.Players, &player)
		playerCache.PlayerIdx = len(ce.competition.State.Players) - 1
		ce.insertPlayerCache(ce.competition.ID, joinPlayer.PlayerID, playerCache)
		ce.emitEvent("PlayerBuyIn -> Buy In", joinPlayer.PlayerID)
		ce.emitPlayerEvent("PlayerBuyIn -> Buy In", &player)
	} else {
		// ReBuy logic
		playerCache, exist := ce.getPlayerCache(ce.competition.ID, joinPlayer.PlayerID)
		if !exist {
			return ErrCompetitionPlayerNotFound
		}

		if _, exist := ce.reBuyTimerStates[joinPlayer.PlayerID]; !exist {
			ce.reBuyTimerStates[joinPlayer.PlayerID] = timebank.NewTimeBank()
		}
		ce.reBuyTimerStates[joinPlayer.PlayerID].Cancel()

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
		}
		ce.emitEvent("PlayerBuyIn -> Re Buy", joinPlayer.PlayerID)
		ce.emitPlayerEvent("PlayerBuyIn -> Re Buy", cp)
	}

	// 更新統計數據 (CT & MTT)
	if ce.competition.Meta.Mode == CompetitionMode_CT || ce.competition.Meta.Mode == CompetitionMode_MTT {
		ce.competition.State.Statistic.TotalBuyInCount++
	}

	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT, CompetitionMode_Cash:
		// call tableEngine
		jp := pokertable.JoinPlayer{
			PlayerID:    joinPlayer.PlayerID,
			RedeemChips: joinPlayer.RedeemChips,
			Seat:        UnsetValue,
		}
		if err := ce.tableManagerBackend.PlayerReserve(tableID, jp); err != nil {
			ce.emitErrorEvent("PlayerBuyIn -> PlayerReserve", joinPlayer.PlayerID, err)
		}
	case CompetitionMode_MTT:
		// 比賽開打後 MTT 一律丟到拆併桌程式配桌
		if ce.competition.State.Status == CompetitionStateStatus_DelayedBuyIn {
			if err := ce.match.Register(joinPlayer.PlayerID); err != nil {
				ce.emitErrorEvent("PlayerBuyIn -> Register Player to Match failed", joinPlayer.PlayerID, err)
				return err
			}
		}
	}

	return nil
}

func (ce *competitionEngine) PlayerAddon(tableID string, joinPlayer JoinPlayer) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

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
	cp.CurrentTableID = tableID
	cp.Chips += joinPlayer.RedeemChips
	cp.AddonTimes++
	cp.TotalRedeemChips += joinPlayer.RedeemChips

	// call tableEngine
	jp := pokertable.JoinPlayer{
		PlayerID:    joinPlayer.PlayerID,
		RedeemChips: joinPlayer.RedeemChips,
	}
	if err := ce.tableManagerBackend.PlayerRedeemChips(tableID, jp); err != nil {
		return err
	}

	ce.emitEvent("PlayerAddon", joinPlayer.PlayerID)
	ce.emitPlayerEvent("PlayerAddon", cp)
	return nil
}

func (ce *competitionEngine) PlayerRefund(playerID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

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

	if ce.competition.Meta.Mode == CompetitionMode_CT {
		playerCache, exist := ce.getPlayerCache(ce.competition.ID, playerID)
		if !exist {
			return ErrCompetitionPlayerNotFound
		}
		// call tableEngine
		if err := ce.tableManagerBackend.PlayersLeave(playerCache.TableID, []string{playerID}); err != nil {
			return err
		}
	}

	// refund logic
	ce.deletePlayer(playerIdx)
	ce.deletePlayerCache(ce.competition.ID, playerID)
	delete(ce.reBuyTimerStates, playerID)

	ce.emitEvent("PlayerRefund", playerID)
	return nil
}

func (ce *competitionEngine) PlayerCashOut(tableID, playerID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

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

	// 尚未開賽時，玩家直接離桌結算
	if ce.competition.State.Status == CompetitionStateStatus_Registering {
		leavePlayerIndexes := map[string]int{
			playerID: playerIdx,
		}
		leavePlayerIDs := []string{playerID}
		ce.handleCashOut(tableID, leavePlayerIndexes, leavePlayerIDs)
	}

	return nil
}

func (ce *competitionEngine) PlayerQuit(tableID, playerID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

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

	if _, exist := ce.reBuyTimerStates[playerID]; !exist {
		ce.reBuyTimerStates[playerID] = timebank.NewTimeBank()
	}
	ce.reBuyTimerStates[playerID].Cancel()

	cp := ce.competition.State.Players[playerIdx]
	cp.Status = CompetitionPlayerStatus_ReBuyWaiting
	cp.IsReBuying = false
	cp.ReBuyEndAt = UnsetValue
	ce.emitPlayerEvent("quit knockout", cp)

	if ce.competition.State.BlindState.IsStopBuyIn() {
		// 更新賽事排名
		ce.competition.State.Rankings = append(ce.competition.State.Rankings, &CompetitionRank{
			PlayerID:   playerID,
			FinalChips: 0,
		})
		rank := ce.competition.PlayingPlayerCount() + 1
		ce.emitCompetitionStateFinalPlayerRankEvent(playerID, rank)
	}
	ce.emitEvent("Player Quit", "")
	ce.emitCompetitionStateEvent(CompetitionStateEvent_KnockoutPlayers)

	if err := ce.tableManagerBackend.PlayersLeave(tableID, []string{playerID}); err != nil {
		ce.emitErrorEvent("Player Quit Knockout Players -> PlayersLeave", playerID, err)
	}

	// 結束桌
	table := ce.competition.State.Tables[tableIdx]
	if ce.competition.Meta.Mode == CompetitionMode_CT && ce.shouldCloseCTTable(table.State.StartAt, len(table.AlivePlayers())) {
		if err := ce.tableManagerBackend.CloseTable(tableID); err != nil {
			ce.emitErrorEvent("Player Quit Knockout Players -> CloseTable", "", err)
		}
	}

	return nil
}

/*
MatchCreateTable 拆併桌自動開桌
  - 適用時機: 拆併桌自動觸發
*/
func (ce *competitionEngine) MatchCreateTable() (string, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	tableSetting := TableSetting{
		TableID:     uuid.New().String(),
		JoinPlayers: []JoinPlayer{},
	}
	tableID, err := ce.addCompetitionTable(tableSetting, CompetitionPlayerStatus_WaitingTableBalancing)
	if err != nil {
		return "", err
	}

	ce.updateTableBlind(tableID)
	ce.tablePlayerWaitingQueue[tableID] = make(map[int]int)

	return tableID, nil
}

/*
MatchCloseTable 拆併桌自動關桌
  - 適用時機: 拆併桌自動觸發
*/
func (ce *competitionEngine) MatchCloseTable(tableID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	delete(ce.tablePlayerWaitingQueue, tableID)

	tableIdx := ce.competition.FindTableIdx(func(t *pokertable.Table) bool {
		return t.ID == tableID
	})
	if tableIdx == UnsetValue {
		return ErrCompetitionTableNotFound
	}

	for _, player := range ce.competition.State.Tables[tableIdx].State.PlayerStates {
		playerCache, exist := ce.getPlayerCache(ce.competition.ID, player.PlayerID)
		if !exist {
			continue
		}
		playerCache.TableID = ""

		cp := ce.competition.State.Players[playerCache.PlayerIdx]
		cp.CurrentTableID = ""

		// 只有有籌碼玩家才會改變狀態
		if cp.Chips > 0 {
			cp.Status = CompetitionPlayerStatus_WaitingTableBalancing
		}
		ce.emitPlayerEvent("[MatchCloseTable] table is closed, wait for allocate to new table", cp)
	}
	ce.emitEvent("[MatchCloseTable]", "")

	return ce.tableManagerBackend.CloseTable(tableID)
}

/*
MatchTableReservePlayer 拆併桌玩家配至新桌
  - 適用時機: 拆併桌自動觸發
*/
func (ce *competitionEngine) MatchTableReservePlayer(tableID, playerID string, seat int) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	fmt.Printf("[DEBUG#MTT#MatchTableReservePlayer] [%s][%s] Seat: %d\n", tableID, playerID, seat)

	tableIdx := ce.competition.FindTableIdx(func(t *pokertable.Table) bool {
		return t.ID == tableID
	})
	if tableIdx == UnsetValue {
		return ErrCompetitionTableNotFound
	}

	playerCache, exist := ce.getPlayerCache(ce.competition.ID, playerID)
	if !exist {
		return ErrCompetitionPlayerNotFound
	}

	cp := ce.competition.State.Players[playerCache.PlayerIdx]
	if cp.Chips <= 0 {
		fmt.Printf("[DEBUG#MTT#MatchTableReservePlayer#ErrMatchTableReservePlayerFailed] competition (%s), table (%s), seat (%d), player (%s), chips (%d)\n", ce.competition.ID, tableID, seat, playerID, cp.Chips)
		return ErrMatchTableReservePlayerFailed
	}

	if ce.competition.State.Tables[tableIdx].State.GameCount <= 0 {
		// 桌次還未開打，將玩家先暫時放在等待配桌的隊列中，等到配桌完成後再一次性丟到桌次中
		_, exist = ce.tablePlayerWaitingQueue[tableID]
		if !exist {
			ce.tablePlayerWaitingQueue[tableID] = make(map[int]int)
		}

		ce.tablePlayerWaitingQueue[tableID][playerCache.PlayerIdx] = seat
		return nil
	}

	// 桌次已開打，直接將玩家丟到桌次中
	// update cache & competition players
	playerCache.TableID = tableID
	cp.CurrentTableID = tableID
	cp.Status = CompetitionPlayerStatus_Playing
	cp.CurrentSeat = seat
	ce.emitPlayerEvent("[MatchTableReservePlayer] reserve table", cp)

	jp := pokertable.JoinPlayer{
		PlayerID:    playerID,
		RedeemChips: cp.Chips,
		Seat:        seat,
	}

	// call tableEngine
	if err := ce.tableManagerBackend.PlayerReserve(tableID, jp); err != nil {
		ce.emitErrorEvent("MatchTableReservePlayer -> PlayerReserve", jp.PlayerID, err)
		return err
	}

	return nil
}

/*
MatchTableReservePlayerDone 拆併桌玩家配桌完成
  - 適用時機: 拆併桌自動觸發
*/
func (ce *competitionEngine) MatchTableReservePlayerDone(tableID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	targetPlayerIndexes, exist := ce.tablePlayerWaitingQueue[tableID]
	if !exist {
		return ErrCompetitionTableNotFound
	}

	joinPlayers := make([]pokertable.JoinPlayer, 0)
	for playerIdx, seat := range targetPlayerIndexes {
		cp := ce.competition.State.Players[playerIdx]
		playerCache, exist := ce.getPlayerCache(ce.competition.ID, cp.PlayerID)
		if !exist {
			return ErrCompetitionPlayerNotFound
		}

		// update cache & competition players
		playerCache.TableID = tableID
		cp.CurrentTableID = tableID
		cp.Status = CompetitionPlayerStatus_Playing
		cp.CurrentSeat = seat
		ce.emitPlayerEvent("[MatchTableReservePlayer] wait balance table", cp)

		jp := pokertable.JoinPlayer{
			PlayerID:    cp.PlayerID,
			RedeemChips: cp.Chips,
			Seat:        seat,
		}
		joinPlayers = append(joinPlayers, jp)
	}

	ce.emitEvent("[MatchTableReservePlayerDone] players batch reserve table", "")

	// call tableEngine
	if err := ce.tableManagerBackend.PlayersBatchReserve(tableID, joinPlayers); err != nil {
		ce.emitErrorEvent("PlayersBatchReserve", "", err)
		return err
	}

	// clear queue
	ce.tablePlayerWaitingQueue[tableID] = make(map[int]int)

	return nil
}
