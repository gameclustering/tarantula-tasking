package core

type Ticket interface {
	CreateTicket(systemId int64, stub int32, accessControl int32,durationSeconds int) (string, error)
	ValidateTicket(ticket string) (OnSession, error)
}
