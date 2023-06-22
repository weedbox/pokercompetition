package pokercompetition

type RequestAction string

const (
	RequestAction_PlayerJoin   RequestAction = "PlayerJoin"
	RequestAction_PlayerAddon  RequestAction = "PlayerAddon"
	RequestAction_PlayerRefund RequestAction = "PlayerRefund"
	RequestAction_PlayerLeave  RequestAction = "PlayerLeave"
)

type Request struct {
	Action  RequestAction
	Payload Payload
}

type Payload struct {
	Competition *Competition
	Param       interface{}
}

type PlayerJoinParam struct {
	TableID    string
	JoinPlayer JoinPlayer
}

type PlayerAddonParam struct {
	TableID    string
	JoinPlayer JoinPlayer
}

type PlayerRefundParam struct {
	TableID  string
	PlayerID string
}

type PlayerLeaveParam struct {
	TableID  string
	PlayerID string
}
