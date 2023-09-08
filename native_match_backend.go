package pokercompetition

import (
	"github.com/weedbox/pokerface/match"
)

type NativeMatchTableBackend struct {
	c              CompetitionEngine
	onTableUpdated func(tableID string, sc *match.SeatChanges)
}

func NewNativeMatchTableBackend(c CompetitionEngine) *NativeMatchTableBackend {
	return &NativeMatchTableBackend{
		c:              c,
		onTableUpdated: func(tableID string, sc *match.SeatChanges) {},
	}
}

func (nmtb *NativeMatchTableBackend) Allocate(maxSeats int) (*match.Table, error) {
	// Create a new table
	tableID, err := nmtb.c.MatchCreateTable()
	if err != nil {
		return nil, err
	}

	// Preparing table state
	t := match.NewTable(maxSeats)
	t.SetID(tableID)

	// fmt.Printf("Allocated Table (id=%s, seats=%d)\n", tableID, maxSeats)

	return t, nil
}

func (nmtb *NativeMatchTableBackend) Release(tableID string) error {
	return nmtb.c.MatchCloseTable(tableID)
}

func (nmtb *NativeMatchTableBackend) Activate(tableID string) error {

	// fmt.Printf("Activated Table (id=%s)\n", tableID)

	err := nmtb.c.MatchTableReservePlayerDone(tableID)
	if err != nil {
		return err
	}

	return nil
}

func (nmtb *NativeMatchTableBackend) Reserve(tableID string, seatID int, playerID string) error {

	// fmt.Printf("<= Reseve Seat (table_id=%s, seat=%d, player=%s)\n", tableID, seatID, playerID)

	err := nmtb.c.MatchTableReservePlayer(tableID, playerID, seatID)
	if err != nil {
		return err
	}

	return nil
}

func (nmtb *NativeMatchTableBackend) GetTable(tableID string) (*match.Table, error) {

	t := match.NewTable(nmtb.c.GetCompetition().Meta.TableMaxSeatCount)
	t.SetID(tableID)

	return t, nil
}

func (nmtb *NativeMatchTableBackend) UpdateTable(tableID string, sc *match.SeatChanges) error {
	nmtb.onTableUpdated(tableID, sc)
	return nil
}

func (nmtb *NativeMatchTableBackend) OnTableUpdated(fn func(tableID string, sc *match.SeatChanges)) {
	nmtb.onTableUpdated = fn
}
