package ledger

import (
	"context"
	"errors"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const kvTable = `CREATE TABLE IF NOT EXISTS ledger_kv (
	key text PRIMARY KEY,
	value text NOT NULL
)`

var errNoDatabase = errors.New("Set DATABASE_URL to the Neon connection string.")

var (
	poolMu sync.Mutex
	pool   *pgxpool.Pool
)

func databaseURL() string {
	for _, key := range []string{"DATABASE_URL", "POSTGRES_URL", "NEON_DATABASE_URL"} {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func normalizeDSN(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	query := parsed.Query()
	query.Del("channel_binding")
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func kvPool() (*pgxpool.Pool, error) {
	dsn := databaseURL()
	if dsn == "" {
		return nil, errNoDatabase
	}
	poolMu.Lock()
	defer poolMu.Unlock()
	if pool != nil {
		return pool, nil
	}
	cfg, err := pgxpool.ParseConfig(normalizeDSN(dsn))
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 2
	cfg.MinConns = 0
	cfg.HealthCheckPeriod = 15 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	next, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if _, err := next.Exec(ctx, kvTable); err != nil {
		next.Close()
		return nil, err
	}
	pool = next
	return pool, nil
}

func resetPool() {
	poolMu.Lock()
	if pool != nil {
		pool.Close()
		pool = nil
	}
	poolMu.Unlock()
}

func kvGet(key string) (string, error) {
	db, err := kvPool()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var value string
	err = db.QueryRow(ctx, `SELECT value FROM ledger_kv WHERE key = $1`, key).Scan(&value)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		resetPool()
		return "", err
	}
	return value, nil
}

func kvSet(key, value string) error {
	db, err := kvPool()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err = db.Exec(ctx, `
		INSERT INTO ledger_kv (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value)
	if err != nil {
		resetPool()
	}
	return err
}
