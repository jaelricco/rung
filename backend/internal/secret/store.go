package secret

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// sealingKeyName is the row this server's own sealing key lives under.
const sealingKeyName = "ai_credentials_key"

// Keystore returns the box used to seal athletes' provider keys.
//
// An operator-supplied key wins: it keeps the key out of the database
// entirely, which is the stronger arrangement and the one to prefer. When
// there is none, the server generates a key once and keeps it, so that
// nothing has to be configured before the coaching features work. Generated
// or given, the athletes' keys are never stored in the clear either way.
func Keystore(ctx context.Context, pool *pgxpool.Pool, configured string) (*Box, bool, error) {
	if configured != "" {
		box, err := New(configured)
		if err != nil {
			return nil, false, err
		}
		return box, false, nil
	}

	key, err := loadOrCreateKey(ctx, pool)
	if err != nil {
		return nil, true, err
	}
	box, err := FromKey(key)
	return box, true, err
}

func loadOrCreateKey(ctx context.Context, pool *pgxpool.Pool) ([]byte, error) {
	var key []byte
	err := pool.QueryRow(ctx,
		`select value from server_secrets where name = $1`, sealingKeyName).Scan(&key)
	if err == nil {
		if len(key) != 32 {
			return nil, fmt.Errorf("the stored sealing key is %d bytes, want 32", len(key))
		}
		return key, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	fresh, err := NewKey()
	if err != nil {
		return nil, err
	}
	// Two API containers starting together must not each write their own key,
	// or half the athletes' credentials become unreadable. The insert that
	// loses does nothing, and the select that follows reads the winner's.
	if _, err := pool.Exec(ctx, `
		insert into server_secrets (name, value) values ($1, $2)
		on conflict (name) do nothing`, sealingKeyName, fresh); err != nil {
		return nil, err
	}
	if err := pool.QueryRow(ctx,
		`select value from server_secrets where name = $1`, sealingKeyName).Scan(&key); err != nil {
		return nil, err
	}
	return key, nil
}
