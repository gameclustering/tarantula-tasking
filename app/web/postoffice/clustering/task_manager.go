
package clustering

import (
	"fmt"
	"time"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/event"
	"gameclustering.com/internal/protocol"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// reserveFirstRetryTimeout is the delay before the first reserve retry.
// Kept short so a dropped TRANS_MAIL (e.g. reconnect window) is recovered well
// within the JOB_TIMEOUT window. Subsequent retries use tc.Meta.Timeout.
const reserveFirstRetryTimeout = 60 * time.Second

type TaskResource struct {
	resource *protocol.Task
	//transaction bookkeeping
	revision uint64
	pending  []*protocol.Job
	jobIndex int
	canceled bool

	tb *event.TaskEventBuilder
}

func NewTaskResource(task *protocol.Task, revision uint64) *TaskResource {
	tr := TaskResource{resource: task, revision: revision, pending: make([]*protocol.Job, 0), tb: event.NewTaskEventBuilder(task.Meta)}
	tr.tb.Description("task event")
	tr.tb.Start(time.Now())
	if task.Validator != nil {
		tr.tb.ValidatorBuilder.Meta(copy(task.Validator.Meta))
	}
	tr.tb.JobBuilder.Meta(copy(task.Jobs[0].Meta))
	return &tr
}

type Retrying func()
type Timeout struct {
	t *time.Timer
	d time.Duration
	r uint32
	p Retrying
}

func copy(meta *protocol.Meta) *protocol.Meta {
	cp := protocol.Meta{TaskId: meta.TaskId, JobId: meta.JobId, Id: meta.Id, State: meta.State, NodeId: meta.NodeId, Tag: meta.Tag, Name: meta.Name}
	cp.Timeout = meta.Timeout
	cp.Retries = meta.Retries
	cp.Time = meta.Time
	cp.Prefix = meta.Prefix
	cp.Description = meta.Description
	return &cp
}

type TaskManager struct {
	trs map[uint64]*TaskResource
	tjs map[uint64]*JobResource
	tms map[uint64]*Timeout

	s *DataServiceProvider

	tasks   chan *protocol.Task
	jobs    chan *protocol.Job
	updates chan *protocol.Meta
}

func (m *TaskManager) set(t *TaskResource) {

	if t.resource.Validator != nil {
		t.pending = append(t.pending, t.resource.Validator)
	}
	t.pending = append(t.pending, t.resource.Jobs...)
	t.jobIndex = 0
	job := t.pending[t.jobIndex]
	job.Meta.State = protocol.TCC_JOB_TIMEOUT
	job.Meta.Prefix = t.resource.Meta.Prefix
	m.schedule(t, job)
}

func (m *TaskManager) schedule(t *TaskResource, job *protocol.Job) {
	go m.s.updateTask(t, func() {
		//core.AppLog.Debug().Msgf("task updated %d %d", t.revision, t.resource.Meta.Id)
	})
	go func() {
		m.jobs <- job
	}()
}

func (m *TaskManager) start(j *JobResource) {
	core.AppLog.Info().Msgf("start job=%d task=%d txns=%d", j.resource.Meta.Id, j.resource.Meta.TaskId, len(j.resource.Transactions))
	m.tms[j.resource.Meta.Id] = &Timeout{t: time.AfterFunc(time.Duration(j.resource.Meta.Timeout)*time.Second, func() {
		m.updates <- &protocol.Meta{TaskId: j.resource.Meta.TaskId, JobId: j.resource.Meta.Id, State: protocol.TCC_JOB_TIMEOUT}
	})}
	j.joinParties = len(j.resource.Transactions)
	j.confirmed = 0
	for _, tc := range j.resource.Transactions {
		j.joining[tc.Meta.Id] = &TransactionResource{resource: tc}
		tc.Meta.State = protocol.TCC_RESERVING
		tc.Meta.Time = timestamppb.Now()
		tc.Meta.Prefix = j.resource.Meta.Prefix
		// First retry fires at reserveFirstRetryTimeout (60s) so a dropped
		// TRANS_MAIL is recovered quickly; subsequent retries use tc.Meta.Timeout.
		m.tms[tc.Meta.Id] = &Timeout{t: time.AfterFunc(reserveFirstRetryTimeout, func() {
			m.updates <- &protocol.Meta{TaskId: j.resource.Meta.TaskId, JobId: j.resource.Meta.Id, Id: tc.Meta.Id, State: protocol.TCC_TRANSACTION_TIMEOUT}
		}), p: func() {
			core.AppLog.Debug().Msgf("retry to reserve with timeout on %d", tc.Meta.Id)
			tc.Meta.Time = timestamppb.Now()
			go m.s.runAskReserve(tc)
		}, d: time.Duration(tc.Meta.Timeout) * time.Second, r: tc.Meta.Retries}
		//ask to reserve
	}
	for _, tc := range j.resource.Transactions {
		go m.s.runAskReserve(tc)
	}
}

func (m *TaskManager) stop(t *JobResource) {
	t.canceled = true
	t.jb.Description("job timeout").End(time.Now())
	tr := m.trs[t.resource.Meta.TaskId]
	tr.canceled = true
	tr.resource.Meta.State = protocol.TCC_FINISHED
	if tr.resource.Validator != nil {
		tr.resource.Validator.Meta.State = protocol.TCC_FINISHED
	}
	for _, tc := range t.resource.Transactions {
		m.closeTimer(tc.Meta.Id)
	}
	m.end(tr)
}

func (m *TaskManager) confirmed(t *JobResource) {
	t.confirmed = 0
	for _, tc := range t.resource.Transactions {
		tc.Meta.State = protocol.TCC_CONFIRMED
		tc.Meta.Prefix = t.resource.Meta.Prefix
		//retry to finish
		m.tms[tc.Meta.Id] = &Timeout{t: time.AfterFunc(time.Duration(tc.Meta.Timeout)*time.Second, func() {
			m.updates <- &protocol.Meta{TaskId: t.resource.Meta.TaskId, JobId: t.resource.Meta.Id, Id: tc.Meta.Id, State: protocol.TCC_TRANSACTION_TIMEOUT}
		}), p: func() {
			core.AppLog.Debug().Msgf("retry to finish with confirm/timeout on %d", tc.Meta.Id)
			go m.s.runAskFinish(m.copy(tc.Meta))
		}, d: time.Duration(tc.Meta.Timeout) * time.Second, r: tc.Meta.Retries}

		//ask to finish
		go m.s.runAskFinish(m.copy(tc.Meta))

	}
}

func (m *TaskManager) canceled(c *protocol.Meta, t *JobResource) {
	t.confirmed = 0
	t.cancel(c)
	tr := m.trs[t.resource.Meta.TaskId]
	tr.jobIndex = len(tr.pending)
	tr.canceled = true
	t.transaction(c)
	for _, tc := range t.resource.Transactions {
		m.closeTimer(tc.Meta.Id)
		tc.Meta.State = protocol.TCC_CANCELED
		tc.Meta.Description = c.Description
		tc.Meta.Prefix = t.resource.Meta.Prefix
		//retry to finish on cancel
		m.tms[tc.Meta.Id] = &Timeout{t: time.AfterFunc(time.Duration(tc.Meta.Timeout)*time.Second, func() {
			m.updates <- &protocol.Meta{TaskId: t.resource.Meta.TaskId, JobId: t.resource.Meta.Id, Id: tc.Meta.Id, State: protocol.TCC_TRANSACTION_TIMEOUT}
		}), p: func() {
			core.AppLog.Debug().Msgf("retry to finish with cancel/timeout on %d", tc.Meta.Id)
			go m.s.runAskFinish(m.copy(tc.Meta))
		}, d: time.Duration(tc.Meta.Timeout) * time.Second, r: tc.Meta.Retries}
		//ask to finish
		go m.s.runAskFinish(m.copy(tc.Meta))
	}
}

func (m *TaskManager) finished(t *JobResource) {
	m.closeTimer(t.resource.Meta.Id)
	tr := m.trs[t.resource.Meta.TaskId]
	t.resource.Meta.State = protocol.TCC_FINISHED
	t.jb.End(time.Now())
	if tr.jobIndex+1 < len(tr.pending) {
		tr.jobIndex++
		next := tr.pending[tr.jobIndex]
		next.Meta.State = protocol.TCC_JOB_TIMEOUT
		next.Meta.Prefix = tr.resource.Meta.Prefix
		m.schedule(tr, next)
		return
	}
	m.end(tr)
}

func (m *TaskManager) end(t *TaskResource) {
	
	t.resource.Meta.State = protocol.TCC_FINISHED
	go m.s.updateTask(t, func() {
		m.updates <- &protocol.Meta{Id: t.resource.Meta.Id, State: protocol.TCC_TASK_CLEAR}
	})
	tf := event.NewTaskEventFactory()
	e, _ := tf.FromTaskEvent(t.tb.Build())
	e.Event.Key.Array = core.ToBytes(m.s.seq)
	go m.s.runPublish(e)
}

func (m *TaskManager) closeTimer(mkey uint64) {
	tm, ok := m.tms[mkey]
	if !ok {
		return
	}
	tm.t.Stop()
	delete(m.tms, mkey)
}

func (m *TaskManager) timeout(mkey uint64, meta *protocol.Meta) {
	tm, ok := m.tms[mkey]
	if !ok {
		return
	}
	if tm.d > 0 && tm.r > 0 {
		core.AppLog.Debug().Msgf("retried %d %d", tm.r, meta.JobId)
		j, ok := m.tjs[meta.JobId]
		if ok {
			tc := j.joining[meta.Id]
			tc.retried++
			meta.Description = fmt.Sprintf("timeout with retried %d", tc.retried)
			j.transaction(meta)
			core.AppLog.Debug().Msgf("retried %d %d", tc.retried, meta.JobId)
		}
		// retry
		tm.t = time.AfterFunc(tm.d, func() {
			m.updates <- meta
		})
		tm.r--
		tm.p()
		return
	}
	delete(m.tms, mkey)
}

func (m *TaskManager) clearResource(rkey uint64) {
	tr, ok := m.trs[rkey]
	if !ok {
		core.AppLog.Debug().Msgf("no resource existed")
		return
	}
	delete(m.trs, rkey)
	if tr.resource.Validator != nil && tr.resource.Validator.Meta != nil {
		delete(m.tjs, tr.resource.Validator.Meta.Id)
	}
	for _, j := range tr.resource.Jobs {
		delete(m.tjs, j.Meta.Id)
	}
	core.AppLog.Debug().Msgf("task removed %d %d %d %d", rkey, len(m.tms), len(m.trs), len(m.tjs))
}

func (m *TaskManager) reload(meta *protocol.Meta) (*TaskResource, error) {
	core.AppLog.Debug().Msgf("reload task from %v", meta)
	tr, err := m.s.load(meta.TaskId)
	if err != nil {
		core.AppLog.Warn().Msgf("task not existed %d", meta.TaskId)
		return nil, err
	}
	if tr.resource.Meta.State == protocol.TCC_FINISHED {
		return nil, fmt.Errorf("task alread finished on %d", meta.Id)
	}
	m.trs[meta.TaskId] = tr
	if tr.resource.Validator != nil && tr.resource.Validator.Meta.State != protocol.TCC_FINISHED {
		tr.pending = append(tr.pending, tr.resource.Validator)
	}
	for _, job := range tr.resource.Jobs {
		if job.Meta.State != protocol.TCC_FINISHED {
			tr.pending = append(tr.pending, job)
		}
	}
	if len(tr.pending) == 0 {
		return tr, fmt.Errorf("no job available")
	}
	tr.jobIndex = 0

	job := tr.pending[tr.jobIndex]
	job.Meta.State = protocol.TCC_JOB_TIMEOUT
	go m.s.updateTask(tr, func() {
		core.AppLog.Debug().Msgf("task updated from reload %d", tr.revision)
	})
	tj := NewJobResource(job, tr)
	m.tjs[job.Meta.Id] = tj
	m.start(tj)
	return tr, nil
}

func (m *TaskManager) Update(meta *protocol.Meta) {
	m.updates <- meta
}

func (m *TaskManager) Set(t *protocol.Task) {
	m.tasks <- t
}

func (m *TaskManager) Wait() {
	m.tasks = make(chan *protocol.Task, 10)
	m.updates = make(chan *protocol.Meta, 10)
	m.jobs = make(chan *protocol.Job, 10)
	for m.s.running {
		select {
		case task := <-m.tasks:
			core.AppLog.Info().Msgf("Wait received task=%d prefix=%d", task.Meta.Id, task.Meta.Prefix)
			tr := NewTaskResource(task, 1)
			m.trs[task.Meta.Id] = tr
			m.set(tr)
		case job := <-m.jobs:
			tr := m.trs[job.Meta.TaskId]
			tj := NewJobResource(job, tr)
			m.tjs[job.Meta.Id] = tj
			m.start(tj)
		case meta := <-m.updates:
			if meta.State == protocol.TCC_TASK_CLEAR {
				m.clearResource(meta.Id)
				continue
			}
			tr, existing := m.trs[meta.TaskId]
			if !existing {
				loaded, err := m.reload(meta)
				if err != nil {
					core.AppLog.Warn().Msgf("task load error %s", err.Error())
					continue
				}
				tr = loaded
				core.AppLog.Debug().Msgf("task loaded %v", tr.resource)
			}
			if tr.resource.Meta.State == protocol.TCC_FINISHED {
				core.AppLog.Warn().Msgf("task already finished %v", meta)
				continue
			}
			tj := m.tjs[meta.JobId]
			switch meta.State {
			case protocol.TCC_CONFIRMED:
				m.closeTimer(meta.Id)
				if tj.join(meta) {
					m.confirmed(tj)
				}
			case protocol.TCC_CANCELED:
				m.closeTimer(meta.Id)
				m.canceled(meta, tj)

			case protocol.TCC_FINISHED:
				m.closeTimer(meta.Id)
				if tj.join(meta) {
					m.finished(tj)
				}

			case protocol.TCC_TRANSACTION_TIMEOUT:
				core.AppLog.Debug().Msgf("task transaction timeout %d", tr.jobIndex)
				m.timeout(meta.Id, meta)
			case protocol.TCC_JOB_TIMEOUT:
				core.AppLog.Debug().Msgf("task job timeout %d", tr.jobIndex)
				m.timeout(meta.JobId, meta)
				m.stop(tj)
			}
		}
	}
	clear(m.tms)
	clear(m.trs)
	close(m.tasks)
	close(m.updates)
	core.AppLog.Warn().Msg("task manager stopped")
}

func (m *TaskManager) copy(meta *protocol.Meta) *protocol.Meta {
	return copy(meta)
}
