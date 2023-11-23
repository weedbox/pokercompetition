package pokercompetition

import (
	"sort"

	"github.com/thoas/go-funk"
	"github.com/weedbox/pokertable"
)

type RankData struct {
	PlayerID string
	Rank     int
	Chips    int64
}

/*
GetParticipatedPlayerTableRankingData 計算有參與該手玩家該桌即時排名
- Algorithm:
 1. Bankroll 由大到小排名
 2. 如果 Bankroll 相同則用加入時間排序  (早加入者名次高)

- @return 該桌玩家排名資料 Map, key: player id, value: RankData
*/
func (ce *competitionEngine) GetParticipatedPlayerTableRankingData(competitionID string, tablePlayers []*pokertable.TablePlayerState, gamePlayerIndexes []int) map[string]RankData {
	playingPlayers := make([]pokertable.TablePlayerState, 0)
	for _, playerIdx := range gamePlayerIndexes {
		player := tablePlayers[playerIdx]
		playingPlayers = append(playingPlayers, *player)
	}

	// sort result
	sort.Slice(playingPlayers, func(i int, j int) bool {
		// 依照 Bankroll 排名由大到小排序，如果 Bankroll 相同則用加入時間排序 (早加入者名次高)
		playerCacheI, iExist := ce.getPlayerCache(competitionID, playingPlayers[i].PlayerID)
		playerCacheJ, jExist := ce.getPlayerCache(competitionID, playingPlayers[j].PlayerID)
		if playingPlayers[i].Bankroll == playingPlayers[j].Bankroll && (iExist && jExist) {
			return playerCacheI.JoinAt < playerCacheJ.JoinAt
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
GetSortedTableSettlementKnockoutPlayerRankings 桌次結算後預計被淘汰玩家的排名 (越早入桌者，排名越前面，但 index 越小 aka. 排名後面者陣列 index 越小)
  - @return SortedKnockoutPlayers 排序過後的淘汰玩家 ID 陣列
*/
func (ce *competitionEngine) GetSortedTableSettlementKnockoutPlayerRankings(tablePlayers []*pokertable.TablePlayerState) []string {
	competitionID := ce.competition.ID
	sortedKnockoutPlayers := make([]pokertable.TablePlayerState, 0)

	// 找出可能的淘汰者們
	for _, p := range tablePlayers {
		// 有籌碼就略過
		if p.Bankroll > 0 {
			continue
		}

		// 延遲買入階段 & 還有補碼次數就略過
		allowToReBuy := false
		isAlreadyKnockout := false
		if playerCache, exist := ce.getPlayerCache(competitionID, p.PlayerID); exist {
			allowToReBuy = playerCache.ReBuyTimes < ce.competition.Meta.ReBuySetting.MaxTime
			isAlreadyKnockout = ce.competition.State.Players[playerCache.PlayerIdx].Status == CompetitionPlayerStatus_Knockout
		}
		if !ce.competition.State.BlindState.IsStopBuyIn() && allowToReBuy {
			continue
		}

		if isAlreadyKnockout {
			continue
		}

		sortedKnockoutPlayers = append(sortedKnockoutPlayers, *p)
	}

	// 依加入時間晚到早排序
	sort.Slice(sortedKnockoutPlayers, func(i int, j int) bool {
		playerCacheI, iExist := ce.getPlayerCache(competitionID, sortedKnockoutPlayers[i].PlayerID)
		playerCacheJ, jExist := ce.getPlayerCache(competitionID, sortedKnockoutPlayers[j].PlayerID)
		if iExist && jExist {
			return playerCacheI.JoinAt > playerCacheJ.JoinAt
		}
		return true
	})

	return funk.Map(sortedKnockoutPlayers, func(p pokertable.TablePlayerState) string {
		return p.PlayerID
	}).([]string)
}

/*
GetSortedStopBuyInKnockoutPlayerRankings 停止買入後預計被淘汰玩家的排名 (越早入桌者，排名越前面，但 index 越小 aka. 排名後面者陣列 index 越小)
  - @return SortedStopKnockoutPlayerIDs 排序過後的淘汰玩家 ID 陣列
*/
func (ce *competitionEngine) GetSortedStopBuyInKnockoutPlayerRankings() []string {
	sortedKnockoutPlayers := make([]CompetitionPlayer, 0)

	// 找出可能的淘汰者們
	for _, p := range ce.competition.State.Players {
		if p.Chips == 0 && p.Status == CompetitionPlayerStatus_ReBuyWaiting {
			sortedKnockoutPlayers = append(sortedKnockoutPlayers, *p)
		}
	}

	// 依加入時間晚到早排序
	sort.Slice(sortedKnockoutPlayers, func(i int, j int) bool {
		return sortedKnockoutPlayers[i].JoinAt > sortedKnockoutPlayers[j].JoinAt
	})

	playerIDs := make([]string, 0)
	for _, p := range sortedKnockoutPlayers {
		playerIDs = append(playerIDs, p.PlayerID)
	}
	return playerIDs
}

/*
GetParticipatedPlayerCompetitionRankingData 計算賽事所有沒有被淘汰玩家最終排名
- Algorithm:
 1. Chips 由大到小排名
 2. 如果 Chips 相同則用加入時間排序  (早加入者名次高)

- @return 該桌玩家排名資料陣列 (名次 1 to N)
*/
func (ce *competitionEngine) GetParticipatedPlayerCompetitionRankingData(competitionID string, players []*CompetitionPlayer) []RankData {
	playingPlayers := make([]CompetitionPlayer, 0)
	for _, player := range players {
		if player.Chips > 0 {
			playingPlayers = append(playingPlayers, *player)
		}
	}

	// sort result
	sort.Slice(playingPlayers, func(i int, j int) bool {
		// 依照 Chips 排名由大到小排序，如果 Chips 相同則用加入時間排序 (早加入者名次高)
		playerCacheI, iExist := ce.getPlayerCache(competitionID, playingPlayers[i].PlayerID)
		playerCacheJ, jExist := ce.getPlayerCache(competitionID, playingPlayers[j].PlayerID)
		if playingPlayers[i].Chips == playingPlayers[j].Chips && (iExist && jExist) {
			return playerCacheI.JoinAt < playerCacheJ.JoinAt
		}
		return playingPlayers[i].Chips > playingPlayers[j].Chips
	})

	// build RankingData
	rank := 1
	rankingData := make([]RankData, 0)

	// add playing player ranks
	for _, playingPlayer := range playingPlayers {
		rankingData = append(rankingData, RankData{
			PlayerID: playingPlayer.PlayerID,
			Rank:     rank,
			Chips:    playingPlayer.Chips,
		})
		rank++
	}

	return rankingData
}
