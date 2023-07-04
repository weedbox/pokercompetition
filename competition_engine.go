package pokercompetition

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/thoas/go-funk"
	"github.com/weedbox/pokertable"
	"github.com/weedbox/pokertablebalancer"
	"github.com/weedbox/timebank"
)

var (
	ErrCompetitionInvalidCreateSetting = errors.New("competition: invalid create competition setting")
	ErrCompetitionLeaveRejected        = errors.New("competition: not allowed to leave")
	ErrCompetitionRefundRejected       = errors.New("competition: not allowed to refund")
	ErrCompetitionNoRedeemChips        = errors.New("competition: not redeem any chips")
	ErrCompetitionAddonRejected        = errors.New("competition: not allowed to addon")
	ErrCompetitionReBuyRejected        = errors.New("competition: not allowed to re-buy")
	ErrCompetitionBuyInRejected        = errors.New("competition: not allowed to buy in")
	ErrCompetitionExceedReBuyLimit     = errors.New("competition: exceed re-buy limit")
	ErrCompetitionExceedAddonLimit     = errors.New("competition: exceed addon limit")
	ErrCompetitionPlayerNotFound       = errors.New("competition: player not found")
)

type CompetitionEngineOpt func(*competitionEngine)

type CompetitionEngine interface {
	// Events
	OnCompetitionUpdated(fn func(*Competition))             // 賽事更新事件監聽器
	OnCompetitionErrorUpdated(fn func(*Competition, error)) // 賽事錯誤更新事件監聽器
	OnCompetitionPlayerUpdated(fn func(*CompetitionPlayer)) // 賽事玩家更新事件監聽器

	// Competition Actions
	SetSeatManager(seatManager pokertablebalancer.SeatManager)                     // 設定拆併桌管理器
	GetCompetition() *Competition                                                  // 取得賽事
	CreateCompetition(competitionSetting CompetitionSetting) (*Competition, error) // 建立賽事
	CloseCompetition() error                                                       // 關閉賽事
	StartCompetition() error                                                       // 開始賽事

	// Player Operations
	PlayerBuyIn(joinPlayer JoinPlayer) error                 // 玩家報名或補碼
	PlayerAddon(tableID string, joinPlayer JoinPlayer) error // 玩家增購
	PlayerRefund(playerID string) error                      // 玩家退賽
	PlayerLeave(tableID, playerID string) error              // 玩家離桌結算 (現金桌)

	// SeatManager Api Implementations
	AutoCreateTable(competitionID string) (*pokertablebalancer.CreateTableResp, error)                                       // 拆併桌自動開桌
	AutoCloseTable(competitionID string, tableID string) (*pokertablebalancer.CloseTableResp, error)                         // 拆併桌自動關桌
	AutoJoinTable(competitionID string, entries []*pokertablebalancer.TableEntry) (*pokertablebalancer.JoinTableResp, error) // 拆併桌玩家自動入桌
}

type competitionEngine struct {
	competition  *Competition
	playerCaches sync.Map // key: <competitionID.playerID>, value: PlayerCache
	// tb                         *timebank.TimeBank
	seatManager                pokertablebalancer.SeatManager
	tableManagerBackend        TableManagerBackend
	onCompetitionUpdated       func(*Competition)
	onCompetitionErrorUpdated  func(*Competition, error)
	onCompetitionPlayerUpdated func(*CompetitionPlayer)
}

func NewCompetitionEngine(opts ...CompetitionEngineOpt) CompetitionEngine {
	ce := &competitionEngine{
		playerCaches: sync.Map{},
		// tb:                         timebank.NewTimeBank(),
		onCompetitionUpdated:       func(*Competition) {},
		onCompetitionErrorUpdated:  func(*Competition, error) {},
		onCompetitionPlayerUpdated: func(*CompetitionPlayer) {},
	}

	for _, opt := range opts {
		opt(ce)
	}

	return ce
}

func WithTableManagerBackend(tmb TableManagerBackend) CompetitionEngineOpt {
	return func(ce *competitionEngine) {
		ce.tableManagerBackend = tmb
		ce.tableManagerBackend.OnTableUpdated(func(table *pokertable.Table) {
			ce.updateTable(table)
		})
	}
}

func (ce *competitionEngine) OnCompetitionUpdated(fn func(*Competition)) {
	ce.onCompetitionUpdated = fn
}

func (ce *competitionEngine) OnCompetitionErrorUpdated(fn func(*Competition, error)) {
	ce.onCompetitionErrorUpdated = fn
}

func (ce *competitionEngine) OnCompetitionPlayerUpdated(fn func(*CompetitionPlayer)) {
	ce.onCompetitionPlayerUpdated = fn
}

func (ce *competitionEngine) SetSeatManager(seatManager pokertablebalancer.SeatManager) {
	ce.seatManager = seatManager
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

	// create competition instance
	ce.competition = &Competition{
		ID:   uuid.New().String(),
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
		},
	}

	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT:
		// 批次建立桌次
		for _, tableSetting := range competitionSetting.TableSettings {
			if _, err := ce.addCompetitionTable(tableSetting, CompetitionPlayerStatus_Playing); err != nil {
				return nil, err
			}

			// AutoEndTable (When Disable Time is reached)
			disableAutoCloseTime := time.Unix(ce.competition.State.DisableAt, 0)
			if err := timebank.NewTimeBank().NewTaskWithDeadline(disableAutoCloseTime, func(isCancelled bool) {
				if isCancelled {
					return
				}

				if len(ce.competition.State.Players) < ce.competition.Meta.MinPlayerCount {
					ce.CloseCompetition()
				}
			}); err != nil {
				return nil, err
			}
		}
	case CompetitionMode_MTT:
		// 啟動拆併桌機制
		ce.activateSeatManager(ce.competition.ID, ce.competition.Meta)
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

	ce.emitEvent("CreateCompetition", "")
	return ce.competition, nil
}

/*
	CloseCompetition 關閉賽事
	  - 適用時機: 賽事出狀況需要臨時關閉賽事、未達開賽條件自動關閉賽事
*/
func (ce *competitionEngine) CloseCompetition() error {
	ce.settleCompetition()
	return nil
}

/*
	StartCompetition 開賽
	  - 適用時機: MTT 手動開賽、MTT 自動開賽、CT 開賽
*/
func (ce *competitionEngine) StartCompetition() error {
	// start the competition
	if ce.competition.Meta.Blind.FinalBuyInLevel <= 0 {
		ce.competition.State.Status = CompetitionStateStatus_StoppedBuyIn
	} else {
		ce.competition.State.Status = CompetitionStateStatus_DelayedBuyIn
	}

	if ce.competition.Meta.Mode == CompetitionMode_CT {
		ce.competition.State.StartAt = time.Now().Unix()
		ce.competition.State.EndAt = ce.competition.State.StartAt + int64((time.Duration(ce.competition.Meta.MaxDuration) * time.Minute).Seconds())
	}

	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT:
		// AutoEndTable (Final BuyIn Level & Table Is Pause)
		finalBuyInLevelTime := int64(0)
		for _, level := range ce.competition.Meta.Blind.Levels {
			finalBuyInLevelTime += int64(level.Duration)
			if level.Level == ce.competition.Meta.Blind.FinalBuyInLevel {
				break
			}
		}
		pauseAutoCloseTime := time.Unix(ce.competition.State.StartAt+finalBuyInLevelTime, 0)
		if err := timebank.NewTimeBank().NewTaskWithDeadline(pauseAutoCloseTime, func(isCancelled bool) {
			if isCancelled {
				return
			}

			if len(ce.competition.State.Tables[0].AlivePlayers()) < 2 {
				// 初始化排名陣列
				if len(ce.competition.State.Rankings) == 0 {
					for i := 0; i < len(ce.competition.State.Players); i++ {
						ce.competition.State.Rankings = append(ce.competition.State.Rankings, nil)
					}
				}
				ce.CloseCompetition()
			}
		}); err != nil {
			return err
		}
	case CompetitionMode_MTT:
		// 拆併桌加入玩家
		playerIDs := funk.Map(ce.competition.State.Players, func(player *CompetitionPlayer) string {
			return player.PlayerID
		}).([]string)
		ce.seatManagerJoinPlayer(ce.competition.ID, playerIDs)
	}

	ce.emitEvent("StartCompetition", "")
	return nil
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

	if !isBuyIn {
		// validate re-buy player
		if ce.competition.State.Players[playerIdx].Status == CompetitionPlayerStatus_Knockout {
			return ErrCompetitionReBuyRejected
		}

		// validate re-buy conditions
		if !ce.competition.State.Players[playerIdx].IsReBuying {
			return ErrCompetitionReBuyRejected
		}

		if ce.competition.Meta.Mode == CompetitionMode_CT {
			if ce.competition.State.Players[playerIdx].ReBuyEndAt > time.Now().Unix() {
				return ErrCompetitionReBuyRejected
			}
		}

		if ce.competition.State.Players[playerIdx].Chips > 0 {
			return ErrCompetitionExceedReBuyLimit
		}

		if ce.competition.State.Players[playerIdx].ReBuyTimes >= ce.competition.Meta.ReBuySetting.MaxTime {
			return ErrCompetitionExceedReBuyLimit
		}
	}

	tableID := ""
	if ce.competition.Meta.Mode == CompetitionMode_CT && len(ce.competition.State.Tables) > 0 {
		tableID = ce.competition.State.Tables[0].ID
	}

	playerStatus := CompetitionPlayerStatus_Playing
	if ce.competition.Meta.Mode == CompetitionMode_MTT {
		// MTT 玩家狀態每次進入 BuyIn/ReBuy 皆為等待拆併桌中
		playerStatus = CompetitionPlayerStatus_WaitingTableBalancing
	}

	// do logic
	if isBuyIn {
		player, playerCache := ce.newDefaultCompetitionPlayerData(tableID, joinPlayer.PlayerID, joinPlayer.RedeemChips, playerStatus)
		ce.competition.State.Players = append(ce.competition.State.Players, &player)
		playerCache.PlayerIdx = len(ce.competition.State.Players) - 1
		ce.insertPlayerCache(ce.competition.ID, joinPlayer.PlayerID, playerCache)
		ce.emitEvent("PlayerJoin -> Buy In", joinPlayer.PlayerID)
		ce.emitPlayerEvent("PlayerJoin -> Buy In", &player)
	} else {
		// ReBuy logic
		ce.competition.State.Players[playerIdx].Chips = joinPlayer.RedeemChips
		ce.competition.State.Players[playerIdx].ReBuyTimes++
		ce.competition.State.Players[playerIdx].IsReBuying = false
		ce.competition.State.Players[playerIdx].ReBuyEndAt = UnsetValue
		if playerCache, exist := ce.getPlayerCache(ce.competition.ID, joinPlayer.PlayerID); exist {
			playerCache.ReBuyTimes = ce.competition.State.Players[playerIdx].ReBuyTimes
		} else {
			return ErrCompetitionPlayerNotFound
		}
		ce.emitEvent("PlayerJoin -> Re Buy", joinPlayer.PlayerID)
		ce.emitPlayerEvent("PlayerJoin -> Re Buy", ce.competition.State.Players[playerIdx])
	}

	switch ce.competition.Meta.Mode {
	case CompetitionMode_CT:
		// call tableEngine
		jp := pokertable.JoinPlayer{
			PlayerID:    joinPlayer.PlayerID,
			RedeemChips: joinPlayer.RedeemChips,
		}
		if err := ce.tableManagerBackend.PlayerReserve(tableID, jp); err != nil {
			ce.emitErrorEvent("PlayerBuyIn -> PlayerReserve", joinPlayer.PlayerID, err)
		}
	case CompetitionMode_MTT:
		// MTT 一律丟到拆併桌程式重新配桌
		ce.seatManagerJoinPlayer(ce.competition.ID, []string{joinPlayer.PlayerID})
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
	if ce.competition.State.Players[playerIdx].AddonTimes >= ce.competition.Meta.AddonSetting.MaxTime {
		return ErrCompetitionExceedAddonLimit
	}

	// do logic
	ce.competition.State.Players[playerIdx].CurrentTableID = tableID
	ce.competition.State.Players[playerIdx].Chips += joinPlayer.RedeemChips
	ce.competition.State.Players[playerIdx].AddonTimes++

	// call tableEngine
	jp := pokertable.JoinPlayer{
		PlayerID:    joinPlayer.PlayerID,
		RedeemChips: joinPlayer.RedeemChips,
	}
	if err := ce.tableManagerBackend.PlayerRedeemChips(tableID, jp); err != nil {
		return err
	}

	ce.emitEvent("PlayerAddon", joinPlayer.PlayerID)
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

	ce.emitEvent("PlayerRefund", playerID)
	return nil
}

func (ce *competitionEngine) PlayerLeave(tableID, playerID string) error {
	// validate refund conditions
	playerIdx := ce.competition.FindPlayerIdx(func(player *CompetitionPlayer) bool {
		return player.PlayerID == playerID
	})
	if playerIdx == UnsetValue {
		return ErrCompetitionLeaveRejected
	}

	if ce.competition.Meta.Mode != CompetitionMode_Cash {
		return ErrCompetitionLeaveRejected
	}

	// call tableEngine
	if err := ce.tableManagerBackend.PlayersLeave(tableID, []string{playerID}); err != nil {
		return err
	}

	// TODO: player settlement (現金桌)

	// logic
	ce.deletePlayer(playerIdx)
	ce.deletePlayerCache(ce.competition.ID, playerID)

	ce.emitEvent("PlayerLeave", playerID)
	return nil
}

/*
	AutoCreateTable 拆併桌自動開桌
	  - 適用時機: 拆併桌自動觸發
*/
func (ce *competitionEngine) AutoCreateTable(competitionID string) (*pokertablebalancer.CreateTableResp, error) {
	randomString := func(size int) string {
		source := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
		sourceRune := []rune(source)
		builder := make([]rune, size)
		for i := range builder {
			builder[i] = sourceRune[rand.Intn(len(sourceRune))]
		}
		return string(builder)
	}

	count := len(ce.competition.State.Tables)
	code := fmt.Sprintf("%05d", count+1)
	tableSetting := TableSetting{
		ShortID:        randomString(6),
		Code:           code,
		Name:           code,
		InvitationCode: "",
		JoinPlayers:    []JoinPlayer{},
	}
	tableID, err := ce.addCompetitionTable(tableSetting, CompetitionPlayerStatus_WaitingTableBalancing)
	if err != nil {
		return nil, err
	}

	return &pokertablebalancer.CreateTableResp{
		Success: true,
		TableId: tableID,
	}, nil
}

/*
	AutoCloseTable 拆併桌自動關桌
	  - 適用時機: 拆併桌自動觸發
*/
func (ce *competitionEngine) AutoCloseTable(competitionID string, tableID string) (*pokertablebalancer.CloseTableResp, error) {
	if err := ce.tableManagerBackend.CloseTable(tableID); err != nil {
		return nil, err
	}

	return &pokertablebalancer.CloseTableResp{
		Success: true,
	}, nil
}

/*
	AutoJoinTable 拆併桌家自動入桌
	  - 適用時機: 拆併桌自動觸發
*/
func (ce *competitionEngine) AutoJoinTable(competitionID string, entries []*pokertablebalancer.TableEntry) (*pokertablebalancer.JoinTableResp, error) {
	tableJoinPlayers := make(map[string][]pokertable.JoinPlayer)
	for _, entry := range entries {
		playerCache, exist := ce.getPlayerCache(competitionID, entry.PlayerId)
		if !exist {
			continue
		}

		playerIdx := playerCache.PlayerIdx
		redeemChips := ce.competition.State.Players[playerIdx].Chips

		// update cache & competition players
		playerCache.TableID = entry.TableId
		ce.competition.State.Players[playerIdx].CurrentTableID = entry.TableId
		ce.competition.State.Players[playerIdx].Status = CompetitionPlayerStatus_WaitingTableBalancing
		ce.emitPlayerEvent("[AutoJoinTable] wait balance table", ce.competition.State.Players[playerCache.PlayerIdx])

		// call tableEngine
		jp := pokertable.JoinPlayer{
			PlayerID:    entry.PlayerId,
			RedeemChips: redeemChips,
		}

		// update tableJoinPlayers
		if _, exist := tableJoinPlayers[entry.TableId]; !exist {
			tableJoinPlayers[entry.TableId] = []pokertable.JoinPlayer{jp}
		} else {
			tableJoinPlayers[entry.TableId] = append(tableJoinPlayers[entry.TableId], jp)
		}
	}

	for tableID, joinPlayers := range tableJoinPlayers {
		if err := ce.tableManagerBackend.PlayersBatchReserve(tableID, joinPlayers); err != nil {
			ce.emitErrorEvent("PlayersBatchReserve", "", err)
			return nil, err
		}
	}

	ce.emitEvent("PlayersBatchReserve", "")
	return &pokertablebalancer.JoinTableResp{
		Success: true,
	}, nil
}
