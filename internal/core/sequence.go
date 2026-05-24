package core

type Sequence interface {
	Id() (int64, error)
	UId() uint64
	Parse(snowflakeId int64) (int64, int64, int64)
}
