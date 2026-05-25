package main

import (
	"os"
	"testing"

	"gameclustering.com/internal/bootstrap"
	"gameclustering.com/internal/persistence"
	"gameclustering.com/internal/protocol"
)

func newTestService(t *testing.T) *CloudService {
	url := os.Getenv("TEST_PG_URL")
	if url == "" {
		t.Skip("TEST_PG_URL not set, skipping PostgreSQL integration test")
	}
	bootstrap.CreateTestLog()
	sql := persistence.Postgresql{Url: url}
	if err := sql.Create(); err != nil {
		t.Fatalf("failed to connect to PostgreSQL: %s", err)
	}
	s := &CloudService{}
	s.Sql = sql
	if err := s.createSchema(); err != nil {
		t.Fatalf("failed to create schema: %s", err)
	}
	return s
}

func TestInsert(t *testing.T) {
	s := newTestService(t)
	defer s.Sql.Close()

	meta := &protocol.Meta{
		TaskId: 1001,
		JobId:  2001,
		Id:     3001,
		NodeId: "node-test",
		Tag:    "cloud",
		Name:   "vm-create",
		State:  1,
	}
	if err := s.insert(meta); err != nil {
		t.Errorf("insert failed: %s", err)
	}
}

func TestQueryByTaskId(t *testing.T) {
	s := newTestService(t)
	defer s.Sql.Close()

	meta := &protocol.Meta{
		TaskId: 2002,
		JobId:  3002,
		Id:     4002,
		NodeId: "node-test",
		Tag:    "cloud",
		Name:   "vm-update",
		State:  2,
	}
	if err := s.insert(meta); err != nil {
		t.Fatalf("insert failed: %s", err)
	}

	results, err := s.queryByTaskId(meta.TaskId)
	if err != nil {
		t.Errorf("queryByTaskId failed: %s", err)
	}
	if len(results) == 0 {
		t.Errorf("expected at least one result, got none")
	}
	got := results[0]
	if got.TaskId != meta.TaskId {
		t.Errorf("TaskId mismatch: got %d, want %d", got.TaskId, meta.TaskId)
	}
	if got.JobId != meta.JobId {
		t.Errorf("JobId mismatch: got %d, want %d", got.JobId, meta.JobId)
	}
	if got.NodeId != meta.NodeId {
		t.Errorf("NodeId mismatch: got %s, want %s", got.NodeId, meta.NodeId)
	}
	if got.State != meta.State {
		t.Errorf("State mismatch: got %d, want %d", got.State, meta.State)
	}
}

func TestQueryByTaskIdNotFound(t *testing.T) {
	s := newTestService(t)
	defer s.Sql.Close()

	results, err := s.queryByTaskId(999999)
	if err != nil {
		t.Errorf("queryByTaskId returned unexpected error: %s", err)
	}
	if len(results) != 0 {
		t.Errorf("expected empty result, got %d rows", len(results))
	}
}
