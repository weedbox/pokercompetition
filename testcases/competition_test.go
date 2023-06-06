package testcases

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/weedbox/pokercompetition/model"
)

func TestCompetitionModel(t *testing.T) {
	competition := model.Competition{
		ID:       uuid.New().String(),
		UpdateAt: time.Now().Unix(),
	}
	assert.NotZero(t, competition.ID)
	assert.NotZero(t, competition.UpdateAt)
}
