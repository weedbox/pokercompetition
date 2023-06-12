package pokercompetition

import (
	"sort"
	"time"

	"github.com/thoas/go-funk"
	"github.com/weedbox/pokertable"
)

type PlayerCache struct {
	PlayerID   string
	JoinAt     int64
	PlayerIdx  int // index of Competition.State.Players
	ReBuyTimes int
	TableID    string
}

type RankData struct {
	PlayerID string
	Rank     int
	Chips    int64
}

func NewDefaultCompetitionPlayerData(tableID, playerID string, redeemChips int64) (CompetitionPlayer, PlayerCache) {
	joinAt := time.Now().Unix()
	playerCache := PlayerCache{
		PlayerID:   playerID,
		JoinAt:     joinAt,
		ReBuyTimes: 0,
		TableID:    tableID,
	}

	player := CompetitionPlayer{
		PlayerID:              playerID,
		CurrentTableID:        tableID,
		JoinAt:                joinAt,
		Status:                CompetitionPlayerStatus_Playing,
		Rank:                  UnsetValue,
		Chips:                 redeemChips,
		IsReBuying:            false,
		ReBuyEndAt:            UnsetValue,
		ReBuyTimes:            0,
		AddonTimes:            0,
		BestWinningPotChips:   0,
		BestWinningCombo:      make([]string, 0),
		TotalRedeemChips:      redeemChips,
		TotalGameCounts:       0,
		TotalWalks:            0,
		TotalVPIPTimes:        0,
		TotalFoldTimes:        0,
		TotalPreflopFoldTimes: 0,
		TotalFlopFoldTimes:    0,
		TotalTurnFoldTimes:    0,
		TotalRiverFoldTimes:   0,
		TotalActionTimes:      0,
		TotalRaiseTimes:       0,
		TotalCallTimes:        0,
		TotalCheckTimes:       0,
	}

	return player, playerCache
}

/*
	GetParticipatedPlayerTableRankingData 計算有參與該手玩家該桌即時排名
	- Algorithm:
	    1. Bankroll 由大到小排名
		2. 如果 Bankroll 相同則用加入時間排序  (早加入者名次高)
	- @return 該桌玩家排名資料 Map, key: player id, value: RankData
*/
func GetParticipatedPlayerTableRankingData(playerCacheData map[string]*PlayerCache, tablePlayers []*pokertable.TablePlayerState, gamePlayerIndexes []int) map[string]RankData {
	playingPlayers := make([]pokertable.TablePlayerState, 0)
	for _, playerIdx := range gamePlayerIndexes {
		player := tablePlayers[playerIdx]
		playingPlayers = append(playingPlayers, *player)
	}

	// sort result
	sort.Slice(playingPlayers, func(i int, j int) bool {
		// 依照 Bankroll 排名由大到小排序，如果 Bankroll 相同則用加入時間排序 (早加入者名次高)
		if playingPlayers[i].Bankroll == playingPlayers[j].Bankroll {
			return playerCacheData[playingPlayers[i].PlayerID].JoinAt < playerCacheData[playingPlayers[j].PlayerID].JoinAt
		}

		return playingPlayers[i].Bankroll > playingPlayers[j].Bankroll
	})

	// build RankingData
	rank := 1
	rankingData := make(map[string]RankData)

	// add playing player ranks
	for _, playingPlayer := range playingPlayers {
		rankingData[playingPlayer.PlayerID] = RankData{
			PlayerID: playingPlayer.PlayerID,
			Rank:     rank,
			Chips:    playingPlayer.Bankroll,
		}
		rank++
	}

	// add not participating player ranks
	for _, player := range tablePlayers {
		if _, exist := rankingData[player.PlayerID]; !exist {
			rankingData[player.PlayerID] = RankData{
				PlayerID: player.PlayerID,
				Rank:     0,
				Chips:    player.Bankroll,
			}
		}
	}

	return rankingData
}

/*
	GetSortedFinalKnockoutPlayerRankings 停止買入後被淘汰玩家的排名 (越早入桌者，排名越前面 index 越小)
	  - @return SortedFinalKnockoutPlayerIDs 排序過後的淘汰玩家 ID 陣列
*/
func GetSortedKnockoutPlayerRankings(playerCacheData map[string]*PlayerCache, players []*pokertable.TablePlayerState, maxReBuyTimes int) []string {
	sortedFinalKnockoutPlayers := make([]pokertable.TablePlayerState, 0)

	// 找出可能的淘汰者們
	for _, p := range players {
		if p.Bankroll > 0 {
			continue
		}

		// 還有補碼次數
		if playerCacheData[p.PlayerID].ReBuyTimes < maxReBuyTimes {
			continue
		}

		sortedFinalKnockoutPlayers = append(sortedFinalKnockoutPlayers, *p)
	}

	// 依加入時間晚到早排序
	sort.Slice(sortedFinalKnockoutPlayers, func(i int, j int) bool {
		return playerCacheData[sortedFinalKnockoutPlayers[i].PlayerID].JoinAt > playerCacheData[sortedFinalKnockoutPlayers[j].PlayerID].JoinAt
	})

	return funk.Map(sortedFinalKnockoutPlayers, func(p pokertable.TablePlayerState) string {
		return p.PlayerID
	}).([]string)
}

/*
	GetParticipatedPlayerCompetitionRankingData 計算賽事所有沒有被淘汰玩家最終排名
	- Algorithm:
	    1. Chips 由大到小排名
		2. 如果 Chips 相同則用加入時間排序  (早加入者名次高)
	- @return 該桌玩家排名資料 Map, key: player id, value: RankData
*/
func GetParticipatedPlayerCompetitionRankingData(playerCacheData map[string]*PlayerCache, players []*CompetitionPlayer) map[string]RankData {
	playingPlayers := make([]CompetitionPlayer, 0)
	for _, player := range players {
		if player.Status != CompetitionPlayerStatus_Knockout {
			playingPlayers = append(playingPlayers, *player)
		}
	}

	// sort result
	sort.Slice(playingPlayers, func(i int, j int) bool {
		// 依照 Chips 排名由大到小排序，如果 Chips 相同則用加入時間排序 (早加入者名次高)
		if playingPlayers[i].Chips == playingPlayers[j].Chips {
			return playerCacheData[playingPlayers[i].PlayerID].JoinAt < playerCacheData[playingPlayers[j].PlayerID].JoinAt
		}

		return playingPlayers[i].Chips > playingPlayers[j].Chips
	})

	// build RankingData
	rank := 1
	rankingData := make(map[string]RankData)

	// add playing player ranks
	for _, playingPlayer := range playingPlayers {
		rankingData[playingPlayer.PlayerID] = RankData{
			PlayerID: playingPlayer.PlayerID,
			Rank:     rank,
			Chips:    playingPlayer.Chips,
		}
		rank++
	}

	return rankingData
}
