package main

import (
	"fmt"

	"gameclustering.com/internal/protocol"
)

const (
	CREATE_TASK_META_SCHEMA string = `CREATE TABLE IF NOT EXISTS task_meta (
															id SERIAL PRIMARY KEY,
															task_id BIGINT NOT NULL,
															job_id BIGINT NOT NULL,
															transaction_id BIGINT NOT NULL,
															node_id VARCHAR(50) NOT NULL, 
															tag VARCHAR(50) NOT NULL,
															name VARCHAR(50) NOT NULL,
															state INT NOT NULL,
															time_commited TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`
	INSERT_TASK_META              string = `INSERT INTO task_meta (task_id,job_id,transaction_id,node_id,tag,name,state) VALUES($1,$2,$3,$4,$5,$6,$7)`
	SELECT_TASK_META_WITH_TASK_ID string = `SELECT * FROM task_meta WHERE task_id =$1`
)

func (s *CloudService) createSchema() error {
	_, err := s.Sql.Exec(CREATE_TASK_META_SCHEMA)
	if err != nil {
		return err
	}
	return nil
}

func (s *CloudService) insert(meta *protocol.Meta) error {
	inserted, err := s.Sql.Exec(INSERT_TASK_META, meta.TaskId, meta.JobId, meta.Id, meta.NodeId, meta.Tag, meta.Name, meta.State)
	if err != nil {
		return err
	}
	if inserted != 1 {
		return fmt.Errorf("no meta inserted %d", inserted)
	}
	return nil
}
