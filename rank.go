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
GetSortedFinalKnockoutPlayerRankings 停止買入後被淘汰玩家的排名 (越早入桌者，排名越前面，但 index 越小 aka. 排名後面者陣列 index 越小)
  - @return SortedFinalKnockoutPlayerIDs 排序過後的淘汰玩家 ID 陣列
*/
func (ce *competitionEngine) GetSortedKnockoutPlayerRankings(competitionID string, players []*pokertable.TablePlayerState, maxReBuyTimes int, isFinalBuyInLevel bool) []string {
	sortedFinalKnockoutPlayers := make([]pokertable.TablePlayerState, 0)

	// 找出可能的淘汰者們
	for _, p := range players {
		// 有籌碼就略過
		if p.Bankroll > 0 {
			continue
		}

		// 延遲買入階段 & 還有補碼次數就略過
		allowToReBuy := false
		if playerCache, exist := ce.getPlayerCache(competitionID, p.PlayerID); exist {
			allowToReBuy = playerCache.ReBuyTimes < maxReBuyTimes
		}
		if !isFinalBuyInLevel && allowToReBuy {
			continue
		}

		sortedFinalKnockoutPlayers = append(sortedFinalKnockoutPlayers, *p)
	}

	// 依加入時間晚到早排序
	sort.Slice(sortedFinalKnockoutPlayers, func(i int, j int) bool {
		playerCacheI, iExist := ce.getPlayerCache(competitionID, players[i].PlayerID)
		playerCacheJ, jExist := ce.getPlayerCache(competitionID, players[j].PlayerID)
		if iExist && jExist {
			return playerCacheI.JoinAt > playerCacheJ.JoinAt
		}
		return true
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

- @return 該桌玩家排名資料陣列 (名次 1 to N)
*/
func (ce *competitionEngine) GetParticipatedPlayerCompetitionRankingData(competitionID string, players []*CompetitionPlayer) []RankData {
	playingPlayers := make([]CompetitionPlayer, 0)
	for _, player := range players {
		if player.Status != CompetitionPlayerStatus_Knockout {
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
