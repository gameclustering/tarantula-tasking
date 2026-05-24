package clustering

import (
	"time"

	"gameclustering.com/internal/event"
	"gameclustering.com/internal/protocol"
)

func NewJobResource(job *protocol.Job, t *TaskResource) *JobResource {
	jr := JobResource{resource: job, joining: make(map[uint64]*TransactionResource)}
	matched := false
	for _, j := range t.resource.Jobs {
		if job.Meta.Id == j.Meta.Id {
			jr.jb = t.tb.JobBuilder
			jr.jb.Description("transaction job")
			matched = true
			break
		}
	}
	if !matched {
		jr.jb = t.tb.ValidatorBuilder
		jr.jb.Description("validator job")
	}
	jr.jb.Meta(copy(job.Meta)).Start(time.Now())
	return &jr
}

type JobResource struct {
	resource    *protocol.Job
	joining     map[uint64]*TransactionResource
	joinParties int
	confirmed   int
	canceled    bool

	jb *event.JobEventBuilder
}

type TransactionResource struct {
	resource  *protocol.Transaction
	confirmed int
	finished  int
	canceled  bool
	retried   int
}

func (j *JobResource) transaction(meta *protocol.Meta) {
	mt := copy(meta)
	j.jb.Transaction().New(mt).Start(meta.Time.AsTime()).End(time.Now()).Description(mt.Description).Build()
}

func (j *JobResource) cancel(tc *protocol.Meta) {
	j.canceled = true
	j.joining[tc.Id].canceled = true
}

func (j *JobResource) join(meta *protocol.Meta) bool {
	t := j.joining[meta.Id]
	switch meta.State {
	case protocol.TCC_CONFIRMED:
		if t.confirmed > 0 {
			return false
		}
		t.resource.Meta.State = protocol.TCC_CONFIRMED
		t.confirmed++

	case protocol.TCC_FINISHED:
		if t.finished > 0 {
			return false
		}
		t.resource.Meta.State = protocol.TCC_FINISHED
		t.finished++
	default:
		return false
	}
	//core.AppLog.Debug().Msgf("meta ID : %d STATE : %d CONFIRMED : %d FINISHED %d PREFIX %d", meta.Id, meta.State, t.confirmed, t.finished, meta.Prefix)
	j.jb.Transaction().New(copy(meta)).Start(meta.Time.AsTime()).End(time.Now()).Description(meta.Description).Build()
	j.confirmed++
	return j.joinParties == j.confirmed
}
