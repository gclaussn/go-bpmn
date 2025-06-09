package internal

import "github.com/jackc/pgx/v5/pgtype"

type TimerEventEntity struct {
	Id int32

	ElementId int32
	ProcessId int32

	BpmnProcessId string
	IsSuspended   bool
	Time          pgtype.Timestamp
	TimeCycle     pgtype.Text
	TimeDuration  pgtype.Text
	Version       string
}

type TimerEventRepository interface {
	Insert(*TimerEventEntity) error
}
