package clustering

import (
	context "context"

	"gameclustering.com/internal/core"
	"gameclustering.com/internal/persistence"
	"gameclustering.com/internal/protocol"
	"google.golang.org/grpc/status"
)

const (
	JOB_TIMEOUT_SECONDS         uint32 = 3600
	TRANSACTION_TIMEOUT_SECONDS uint32 = 1800
	TCC_RETRY_MAX               uint32 = 3
)

type ResourceUpdated func()

func (c *DataServiceProvider) Setup(ctx context.Context, task *protocol.Task) (*protocol.Response, error) {
	core.AppLog.Info().Msgf("Setup called task=%d prefix=%d jobs=%d", task.Meta.Id, task.Meta.Prefix, len(task.Jobs))
	c.TManager.Set(task)
	return &protocol.Response{Successful: true, Meta: task.Meta}, nil
}

func (c *DataServiceProvider) AskReserve(ctx context.Context, in *protocol.Transaction) (*protocol.Response, error) {
	in.Meta.State = protocol.TCC_RESERVING
	//down streaming to app
	select {
	case c.DMessager <- &protocol.Mail{Transaction: in, Opt: core.TRANS_MAIL}:
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	return &protocol.Response{Successful: true, Meta: &protocol.Meta{Name: in.Meta.Name}}, nil
}

func (c *DataServiceProvider) AskFinish(ctx context.Context, in *protocol.Meta) (*protocol.Response, error) {
	//down streaming to app meta state could be canceled or confirmed
	select {
	case c.DMessager <- &protocol.Mail{Transaction: &protocol.Transaction{Meta: in}, Opt: core.TRANS_MAIL}:
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	}
	return &protocol.Response{Successful: true, Meta: &protocol.Meta{Name: in.Name}}, nil
}

func (c *DataServiceProvider) Confirmed(ctx context.Context, in *protocol.Meta) (*protocol.Response, error) {
	in.State = protocol.TCC_CONFIRMED
	c.TManager.Update(in)
	return &protocol.Response{Successful: true, Meta: &protocol.Meta{Name: in.Name}}, nil
}

func (c *DataServiceProvider) Canceled(ctx context.Context, in *protocol.Meta) (*protocol.Response, error) {
	in.State = protocol.TCC_CANCELED
	c.TManager.Update(in)
	return &protocol.Response{Successful: true, Meta: &protocol.Meta{Name: in.Name}}, nil
}

func (c *DataServiceProvider) Finished(ctx context.Context, in *protocol.Meta) (*protocol.Response, error) {
	in.State = protocol.TCC_FINISHED
	c.TManager.Update(in)
	return &protocol.Response{Successful: true, Meta: &protocol.Meta{Name: in.Name}}, nil
}

func (c *DataServiceProvider) load(taskId uint64) (*TaskResource, error) {
	buff := core.NewBuffer(8)
	buff.WriteUInt64(taskId)
	buff.Flip()
	k, _ := buff.Read(0)
	prefix := c.RingToken(k)
	req := protocol.Request{Prefix: prefix, Data: &protocol.Data{Key: k, Header: &protocol.Header{FactoryId: core.TASK_FACTORY_ID, ClassId: persistence.TASK_CLASS_ID}}}
	resp, err := c.runGet(&req)
	if err != nil {
		return nil, err
	}
	tb := persistence.TaskBuilder{}
	t, err := tb.From(resp.Data.List[0].Value)
	if err != nil {
		return nil, err
	}
	// Slave replicas receive initial bytes before Hash() sets the prefix.
	// Re-derive it from the task ID key if missing.
	if t.Meta.Prefix == 0 {
		t.Meta.Prefix = prefix
	}
	return NewTaskResource(t, resp.Data.List[0].Header.Revision), nil
}

func (c *DataServiceProvider) updateTask(t *TaskResource, updated ResourceUpdated) {
	suc := false
	defer func() {
		if !suc {
			return
		}
		updated()
	}()
	tb := persistence.TaskBuilder{Target: t.resource}
	req, err := tb.Request()
	if err != nil {
		core.AppLog.Warn().Msgf("cannot request %s", err.Error())
		return
	}
	req.Data.Header.Revision = t.revision
	req.Prefix = tb.Hash(c)
	req.Opt = core.UPDATE_DATA_REQUEST
	_, err = c.runUpdate(req)
	if err != nil {
		core.AppLog.Warn().Msgf("cannot update %s", err.Error())
		return
	}
	t.revision++
	suc = true
	//core.AppLog.Info().Msgf("saved %v", resp)
	//tx, err := c.load(t.resource.Meta.Id)
	//if err != nil {
	//core.AppLog.Error().Msgf("no task loaded %s %d", err.Error(), t.resource.Meta.Id)
	//return
	//}
	//core.AppLog.Debug().Msgf("v %v", tx.resource.Validator.Meta)
	//core.AppLog.Debug().Msgf("j %v", tx.resource.Job.Meta)
	//core.AppLog.Debug().Msgf("t %v", tx.resource.Meta)
}
