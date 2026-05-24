package persistence

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	maxConns          = int32(1)
	minConns          = int32(0)
	maxConnLifetime   = time.Hour
	maxConnIdleTime   = time.Minute * 30
	healthCheckPeriod = time.Minute
	connectTimeout    = time.Second * 5
)

type Next func(row pgx.Rows) error
type Transaction func(tx pgx.Tx) error

type Postgresql struct {
	Pool      *pgxpool.Pool
	Connected bool
	Url       string
	MaxConns  int32
}

func (p *Postgresql) pconfig() (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(p.Url)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = maxConns
	cfg.MinConns = minConns
	cfg.MaxConnLifetime = maxConnLifetime
	cfg.MaxConnIdleTime = maxConnIdleTime
	cfg.HealthCheckPeriod = healthCheckPeriod
	cfg.ConnConfig.ConnectTimeout = connectTimeout
	if p.MaxConns > 0 {
		cfg.MaxConns = p.MaxConns
	}
	//cfg.BeforeAcquire = beforeAcquire
	//cfg.AfterRelease = afterRelease
	//cfg.BeforeClose = beforeClose
	return cfg, nil
}

// one time connection
func (p *Postgresql) CreateDatabase(ddl string) error {
	conn, err := pgx.Connect(context.Background(), p.Url)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(context.Background(), ddl)
	if err != nil {
		return err
	}
	return nil
}

func (p *Postgresql) Create() error {
	cfg, err := p.pconfig()
	if err != nil {
		return err
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return err
	}
	p.Pool = pool
	p.Connected = true
	return nil
}

func (p *Postgresql) Query(next Next, query string, values ...any) error {
	conn, err := p.Pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer conn.Release()
	rows, err := conn.Query(context.Background(), query, values...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if next(rows) != nil {
			break
		}
	}
	return nil
}

func (p *Postgresql) Exec(query string, values ...any) (int64, error) {
	conn, err := p.Pool.Acquire(context.Background())
	if err != nil {
		return 0, err
	}
	defer conn.Release()
	tag, err := conn.Exec(context.Background(), query, values...)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (p *Postgresql) Txn(tx Transaction) error {
	conn, err := p.Pool.Acquire(context.Background())
	if err != nil {
		return err
	}
	defer conn.Release()
	ptx, err := conn.BeginTx(context.Background(), pgx.TxOptions{})
	if err != nil {
		return err
	}
	err = tx(ptx)
	defer func() {
		if err != nil {
			ptx.Rollback(context.Background())
		} else {
			ptx.Commit(context.Background())
		}
	}()

	return err
}

func (p *Postgresql) Close() {
	if !p.Connected {
		return
	}
	p.Pool.Close()
}
