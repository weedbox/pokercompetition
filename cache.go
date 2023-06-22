package pokercompetition

import (
	"fmt"
	"strings"
)

type PlayerCache struct {
	PlayerID   string
	JoinAt     int64
	PlayerIdx  int // index of Competition.State.Players
	ReBuyTimes int
	TableID    string
}

func (ce *competitionEngine) buildPlayerCacheKey(competitionID, playerID string) string {
	return fmt.Sprintf("%s.%s", competitionID, playerID)
}

func (ce *competitionEngine) insertPlayerCache(competitionID, playerID string, playerCache PlayerCache) {
	key := ce.buildPlayerCacheKey(competitionID, playerID)
	ce.playerCaches.Store(key, &playerCache)
}

func (ce *competitionEngine) getPlayerCache(competitionID, playerID string) (*PlayerCache, bool) {
	key := ce.buildPlayerCacheKey(competitionID, playerID)
	c, exist := ce.playerCaches.Load(key)
	if !exist {
		return nil, false
	}
	return c.(*PlayerCache), true
}

func (ce *competitionEngine) deletePlayerCachesByCompetition(competitionID string) {
	deleteKeys := map[string]bool{}
	ce.playerCaches.Range(func(k, v interface{}) bool {
		key := k.(string)
		if strings.Split(key, ".")[0] == competitionID {
			deleteKeys[key] = true
		}
		return true
	})

	for key := range deleteKeys {
		ce.playerCaches.Delete(key)
	}
}

func (ce *competitionEngine) deletePlayerCache(competitionID, playerID string) {
	key := ce.buildPlayerCacheKey(competitionID, playerID)
	ce.playerCaches.Delete(key)
}
