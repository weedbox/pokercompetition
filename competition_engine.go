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
	ErrCompetitionNotFound             = errors.New("competition not found")
	ErrInvalidCreateCompetitionSetting = errors.New("invalid create competition setting")
	ErrTableNotFound                   = errors.New("table not found")
	ErrPlayerNotFound                  = errors.New("player not found")
	ErrNoRedeemChips                   = errors.New("no redeem any chips")
	ErrExceedReBuyLimit                = errors.New("exceed re-buy limit")
	ErrExceedAddonLimit                = errors.New("exceed addon limit")
	ErrAddonRejected                   = errors.New("not allowed to addon")
	ErrReBuyRejected                   = errors.New("not allowed to re-buy")
	ErrBuyInRejected                   = errors.New("not allowed to buy in")
	ErrRefundRejected                  = errors.New("not allowed to refund")
	ErrLeaveRejected                   = errors.New("not allowed to leave")
)

type CompetitionEngine interface {
	// Others
	TableEngine() pokertable.TableEngine
	OnCompetitionTableUpdated(fn func(*pokertable.Table))      // 桌次更新事件監聽器
	OnCompetitionTableErrorUpdated(fn func(error))             // 桌次錯誤更新事件監聽器
	OnCompetitionUpdated(fn func(*Competition))                // 賽事更新事件監聽器
	OnCompetitionErrorUpdated(fn func(error))                  // 賽事錯誤更新事件監聽器
	SetSeatManager(seatManager pokertablebalancer.SeatManager) // 設定拆併桌管理器

	// Competition Actions
	GetCompetition(competitionID string) (*Competition, error)                     // 取得賽事
	CreateCompetition(competitionSetting CompetitionSetting) (*Competition, error) // 建立賽事
	CloseCompetition(competitionID string) error                                   // 關閉賽事
	StartCompetition(competitionID string) error                                   // 開始賽事

	// Player Operations
	PlayerJoin(competitionID, tableID string, joinPlayer JoinPlayer) error  // 玩家報名或補碼
	PlayerAddon(competitionID, tableID string, joinPlayer JoinPlayer) error // 玩家增購
	PlayerRefund(competitionID, tableID, playerID string) error             // 玩家退賽
	PlayerLeave(competitionID, tableID, playerID string) error              // 玩家離桌結算 (現金桌)

	// SeatManager Api Implementations
	AutoCreateTable(competitionID string) (*pokertablebalancer.CreateTableResp, error)                                       // 拆併桌自動開桌
	AutoCloseTable(competitionID string, tableID string) (*pokertablebalancer.CloseTableResp, error)                         // 拆併桌自動關桌
	AutoJoinTable(competitionID string, entries []*pokertablebalancer.TableEntry) (*pokertablebalancer.JoinTableResp, error) // 拆併桌玩家自動入桌
}

func NewCompetitionEngine() CompetitionEngine {
	ce := &competitionEngine{
		timebank:                  timebank.NewTimeBank(),
		onCompetitionUpdated:      func(*Competition) {},
		onCompetitionErrorUpdated: func(error) {},
		onTableUpdated:            func(*pokertable.Table) {},
		onTableErrorUpdated:       func(error) {},
		incoming:                  make(chan *Request, 1024),
		competitions:              sync.Map{},
		playerCaches:              sync.Map{},
	}
	tableEngine := pokertable.NewTableEngine()
	tableEngine.OnTableUpdated(ce.onCompetitionTableUpdated)
	tableEngine.OnErrorUpdated(ce.onCompetitionTableErrorUpdated)
	ce.tableEngine = tableEngine

	go ce.run()
	return ce
}

type competitionEngine struct {
	lock                      sync.Mutex
	seatManager               pokertablebalancer.SeatManager
	timebank                  *timebank.TimeBank
	tableEngine               pokertable.TableEngine
	onCompetitionUpdated      func(*Competition)
	onCompetitionErrorUpdated func(error)
	onTableUpdated            func(*pokertable.Table)
	onTableErrorUpdated       func(error)
	incoming                  chan *Request
	competitions              sync.Map
	playerCaches              sync.Map // key: <competitionID.playerID>, value: PlayerCache
}

func (ce *competitionEngine) TableEngine() pokertable.TableEngine {
	return ce.tableEngine
}

func (ce *competitionEngine) OnCompetitionTableUpdated(fn func(*pokertable.Table)) {
	ce.onTableUpdated = fn
}

func (ce *competitionEngine) OnCompetitionTableErrorUpdated(fn func(error)) {
	ce.onTableErrorUpdated = fn
}

func (ce *competitionEngine) OnCompetitionUpdated(fn func(*Competition)) {
	ce.onCompetitionUpdated = fn
}

func (ce *competitionEngine) OnCompetitionErrorUpdated(fn func(error)) {
	ce.onCompetitionErrorUpdated = fn
}

func (ce *competitionEngine) SetSeatManager(seatManager pokertablebalancer.SeatManager) {
	ce.seatManager = seatManager
}

func (ce *competitionEngine) GetCompetition(competitionID string) (*Competition, error) {
	ce.lock.Lock()
	defer ce.lock.Unlock()

	c, exist := ce.competitions.Load(competitionID)
	if !exist {
		return nil, ErrCompetitionNotFound
	}
	return c.(*Competition), nil
}

func (ce *competitionEngine) CreateCompetition(competitionSetting CompetitionSetting) (*Competition, error) {
	ce.lock.Lock()
	defer ce.lock.Unlock()

	// validate competitionSetting
	now := time.Now()
	if competitionSetting.StartAt != UnsetValue && competitionSetting.StartAt < now.Unix() {
		return nil, ErrInvalidCreateCompetitionSetting
	}

	if competitionSetting.DisableAt < now.Unix() {
		return nil, ErrInvalidCreateCompetitionSetting
	}

	for _, tableSetting := range competitionSetting.TableSettings {
		if len(tableSetting.JoinPlayers) > competitionSetting.Meta.TableMaxSeatCount {
			return nil, ErrInvalidCreateCompetitionSetting
		}
	}

	// create competition instance
	competition := &Competition{
		ID: uuid.New().String(),
	}
	competition.ConfigureWithSetting(competitionSetting)
	for _, tableSetting := range competitionSetting.TableSettings {
		if _, err := ce.addCompetitionTable(competition, tableSetting); err != nil {
			return nil, err
		}

		if competitionSetting.Meta.Mode == CompetitionMode_CT {
			// AutoEndTable (When Disable Time is reached)
			disableAutoCloseTime := time.Unix(competition.State.DisableAt, 0)
			if err := ce.timebank.NewTaskWithDeadline(disableAutoCloseTime, func(isCancelled bool) {
				if isCancelled {
					return
				}

				if len(competition.State.Players) < competition.Meta.MinPlayerCount {
					ce.CloseCompetition(competition.ID)
				}
			}); err != nil {
				return nil, err
			}
		}
	}
	if competition.Meta.Mode == CompetitionMode_MTT {
		// 啟動拆併桌
		ce.activateSeatManager(competition.ID, competition.Meta)
	}

	ce.emitEvent("CreateCompetition", "", competition)

	// update competitions
	ce.competitions.Store(competition.ID, competition)

	// auto startCompetition until StartAt
	if competition.State.StartAt > 0 {
		autoStartTime := time.Unix(competition.State.StartAt, 0)
		if err := ce.timebank.NewTaskWithDeadline(autoStartTime, func(isCancelled bool) {
			if isCancelled {
				return
			}

			c, _ := ce.GetCompetition(competition.ID)
			if c.State.Status == CompetitionStateStatus_Registering {
				ce.StartCompetition(competition.ID)
			}
		}); err != nil {
			return nil, err
		}
	}

	return competition, nil
}

/*
	CloseCompetition 關閉賽事
	  - 適用時機: 賽事出狀況需要臨時關閉賽事、未達開賽條件自動關閉賽事
*/
func (ce *competitionEngine) CloseCompetition(competitionID string) error {
	ce.lock.Lock()
	defer ce.lock.Unlock()

	c, exist := ce.competitions.Load(competitionID)
	if !exist {
		return ErrCompetitionNotFound
	}
	competition := c.(*Competition)

	ce.settleCompetition(competition)
	return nil
}

/*
	StartCompetition 開賽
	  - 適用時機: MTT 手動開賽、MTT 自動開賽、CT 開賽
*/
func (ce *competitionEngine) StartCompetition(competitionID string) error {
	ce.lock.Lock()
	defer ce.lock.Unlock()

	c, exist := ce.competitions.Load(competitionID)
	if !exist {
		return ErrCompetitionNotFound
	}
	competition := c.(*Competition)

	competition.Start()

	switch competition.Meta.Mode {
	case CompetitionMode_CT:
		// AutoEndTable (Final BuyIn Level & Table Is Pause)
		finalBuyInLevelTime := int64(0)
		for _, level := range competition.Meta.Blind.Levels {
			finalBuyInLevelTime += int64(level.Duration)
			if level.Level == competition.Meta.Blind.FinalBuyInLevel {
				break
			}
		}
		pauseAutoCloseTime := time.Unix(competition.State.StartAt+finalBuyInLevelTime, 0)
		if err := ce.timebank.NewTaskWithDeadline(pauseAutoCloseTime, func(isCancelled bool) {
			if isCancelled {
				return
			}

			c, exist := ce.competitions.Load(competitionID)
			if !exist {
				return
			}

			if len(c.(*Competition).State.Tables[0].AlivePlayers()) < 2 {
				// 初始化排名陣列
				if len(c.(*Competition).State.Rankings) == 0 {
					for i := 0; i < len(c.(*Competition).State.Players); i++ {
						c.(*Competition).State.Rankings = append(c.(*Competition).State.Rankings, nil)
					}
				}
				ce.CloseCompetition(competition.ID)
			}
		}); err != nil {
			return err
		}
	case CompetitionMode_MTT:
		// 拆併桌加入玩家
		playerIDs := funk.Map(competition.State.Players, func(player *CompetitionPlayer) string {
			return player.PlayerID
		}).([]string)
		ce.seatManagerJoinPlayer(competition.ID, playerIDs)
	}

	ce.emitEvent("StartCompetition", "", competition)
	return nil
}

func (ce *competitionEngine) PlayerJoin(competitionID, tableID string, joinPlayer JoinPlayer) error {
	param := PlayerJoinParam{
		TableID:    tableID,
		JoinPlayer: joinPlayer,
	}
	return ce.incomingRequest(competitionID, RequestAction_PlayerJoin, param)
}

func (ce *competitionEngine) PlayerAddon(competitionID, tableID string, joinPlayer JoinPlayer) error {
	param := PlayerAddonParam{
		TableID:    tableID,
		JoinPlayer: joinPlayer,
	}
	return ce.incomingRequest(competitionID, RequestAction_PlayerAddon, param)
}

func (ce *competitionEngine) PlayerRefund(competitionID, tableID, playerID string) error {
	param := PlayerRefundParam{
		TableID:  tableID,
		PlayerID: playerID,
	}
	return ce.incomingRequest(competitionID, RequestAction_PlayerRefund, param)
}

func (ce *competitionEngine) PlayerLeave(competitionID, tableID, playerID string) error {
	param := PlayerLeaveParam{
		TableID:  tableID,
		PlayerID: playerID,
	}
	return ce.incomingRequest(competitionID, RequestAction_PlayerLeave, param)
}

/*
	AutoCreateTable 拆併桌自動開桌
	  - 適用時機: 拆併桌自動觸發
*/
func (ce *competitionEngine) AutoCreateTable(competitionID string) (*pokertablebalancer.CreateTableResp, error) {
	ce.lock.Lock()
	defer ce.lock.Unlock()
	// fmt.Println("[pokercompetition#AutoCreateTable] competitionID: ", competitionID)
	c, exist := ce.competitions.Load(competitionID)
	if !exist {
		return nil, ErrCompetitionNotFound
	}
	competition := c.(*Competition)

	randomString := func(size int) string {
		source := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"
		sourceRune := []rune(source)
		builder := make([]rune, size)
		for i := range builder {
			builder[i] = sourceRune[rand.Intn(len(sourceRune))]
		}
		return string(builder)
	}

	count := len(competition.State.Tables)
	code := fmt.Sprintf("%05d", count+1)
	tableSetting := TableSetting{
		ShortID:        randomString(6),
		Code:           code,
		Name:           code,
		InvitationCode: "",
		JoinPlayers:    []JoinPlayer{},
	}
	tableID, err := ce.addCompetitionTable(competition, tableSetting)
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
	ce.lock.Lock()
	defer ce.lock.Unlock()

	_, exist := ce.competitions.Load(competitionID)
	if !exist {
		return nil, ErrCompetitionNotFound
	}

	if err := ce.tableEngine.DeleteTable(tableID); err != nil {
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
	ce.lock.Lock()
	defer ce.lock.Unlock()

	c, exist := ce.competitions.Load(competitionID)
	if !exist {
		return nil, ErrCompetitionNotFound
	}
	competition := c.(*Competition)
	tableIDs := make(map[string]interface{})

	for _, entry := range entries {
		playerCache, exist := ce.getPlayerCache(competitionID, entry.PlayerId)
		if !exist {
			continue
		}

		playerIdx := playerCache.PlayerIdx
		redeemChips := competition.State.Players[playerIdx].Chips
		tableIDs[entry.TableId] = struct{}{}

		// update cache & competition players
		playerCache.TableID = entry.TableId
		competition.State.Players[playerIdx].CurrentTableID = entry.TableId
		competition.State.Players[playerIdx].Status = CompetitionPlayerStatus_Playing

		// call tableEngine
		jp := pokertable.JoinPlayer{
			PlayerID:    entry.PlayerId,
			RedeemChips: redeemChips,
		}
		// fmt.Printf("[pokercompetition#AutoJoinTable] TableID: %s, JoinPlayer: %+v\n", entry.TableId, jp)
		if err := ce.tableEngine.PlayerJoin(entry.TableId, jp); err != nil {
			ce.emitErrorEvent("AutoJoinTable -> Table PlayerJoin", jp.PlayerID, err, competition)
		}
	}
	ce.emitEvent("Players AutoJoinTable & Playing", "", competition)

	// Auto Start Table if necessary
	for tableID := range tableIDs {
		tableIdx := competition.FindTableIdx(func(t *pokertable.Table) bool {
			return tableID == t.ID
		})
		if tableIdx == UnsetValue {
			continue
		}

		table := competition.State.Tables[tableIdx]
		if competition.State.Tables[tableIdx].State.Status == pokertable.TableStateStatus_TableCreated && table.State.StartAt == UnsetValue {
			if err := ce.tableEngine.StartTableGame(tableID); err != nil {
				ce.emitErrorEvent("AutoJoinTable -> Table StartTableGame", "", err, competition)
			}
		}
	}

	return &pokertablebalancer.JoinTableResp{
		Success: true,
	}, nil
}
