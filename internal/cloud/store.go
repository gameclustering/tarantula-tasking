package cloud

import (
	"fmt"
	"time"

	"gameclustering.com/internal/persistence"
	"gameclustering.com/internal/protocol"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	createTaskMetaSchema = `CREATE TABLE IF NOT EXISTS task_meta (
		id SERIAL PRIMARY KEY,
		task_id BIGINT NOT NULL,
		job_id BIGINT NOT NULL,
		transaction_id BIGINT NOT NULL,
		node_id VARCHAR(50) NOT NULL,
		tag VARCHAR(50) NOT NULL,
		name VARCHAR(50) NOT NULL,
		state INT NOT NULL,
		time_commited TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`
	insertTaskMeta         = `INSERT INTO task_meta (task_id,job_id,transaction_id,node_id,tag,name,state) VALUES($1,$2,$3,$4,$5,$6,$7)`
	selectTaskMetaByTaskId = `SELECT * FROM task_meta WHERE task_id=$1`
)

type Store struct {
	sql persistence.Postgresql
}

func NewStore(sql persistence.Postgresql) *Store {
	return &Store{sql: sql}
}

func (s *Store) CreateSchema() error {
	_, err := s.sql.Exec(createTaskMetaSchema)
	return err
}

func (s *Store) Insert(meta *protocol.Meta) error {
	inserted, err := s.sql.Exec(insertTaskMeta, meta.TaskId, meta.JobId, meta.Id, meta.NodeId, meta.Tag, meta.Name, meta.State)
	if err != nil {
		return err
	}
	if inserted != 1 {
		return fmt.Errorf("no meta inserted %d", inserted)
	}
	return nil
}

func (s *Store) QueryByTaskId(taskId uint64) ([]*protocol.Meta, error) {
	results := make([]*protocol.Meta, 0)
	err := s.sql.Query(func(row pgx.Rows) error {
		var (
			id            int64
			taskIdVal     uint64
			jobId         uint64
			transactionId uint64
			nodeId        string
			tag           string
			name          string
			state         uint32
			timeCommited  time.Time
		)
		if err := row.Scan(&id, &taskIdVal, &jobId, &transactionId, &nodeId, &tag, &name, &state, &timeCommited); err != nil {
			return err
		}
		results = append(results, &protocol.Meta{
			TaskId: taskIdVal,
			JobId:  jobId,
			Id:     transactionId,
			NodeId: nodeId,
			Tag:    tag,
			Name:   name,
			State:  state,
			Time:   timestamppb.New(timeCommited),
		})
		return nil
	}, selectTaskMetaByTaskId, taskId)
	return results, err
}
