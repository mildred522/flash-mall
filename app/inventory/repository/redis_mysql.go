package repository

import (
	"context"
	"database/sql"

	"flash-mall/app/common/apperror"
)

const (
	reservationExpirySeconds       = 24 * 60 * 60
	reservationKeyTTLSeconds       = 48 * 60 * 60
	confirmedReservationTTLSeconds = 180 * 24 * 60 * 60
	reservationExpiryIndexKey      = "inventory:reservation:expirations"
)

type RedisClient interface {
	EvalCtx(ctx context.Context, script string, keys []string, args ...any) (any, error)
}

type RedisMySQLRepository struct {
	redis                 RedisClient
	db                    *sql.DB
	shardCount            int
	finalDeductEnabled    bool
	reservationLedgerMode string
}

func NewRedisMySQLRepository(redis RedisClient, db *sql.DB, shardCount int) *RedisMySQLRepository {
	return &RedisMySQLRepository{redis: redis, db: db, shardCount: NormalizeShardCount(shardCount)}
}

func (r *RedisMySQLRepository) WithFinalDeductEnabled(enabled bool) *RedisMySQLRepository {
	r.finalDeductEnabled = enabled
	return r
}

func (r *RedisMySQLRepository) CheckRuntime(ctx context.Context) error {
	if _, err := r.redis.EvalCtx(ctx, "return 1", nil); err != nil {
		return apperror.Wrap(apperror.CodeInternal, "inventory redis unavailable", err)
	}
	if r.db != nil {
		if err := r.db.PingContext(ctx); err != nil {
			return apperror.Wrap(apperror.CodeInternal, "inventory mysql unavailable", err)
		}
		if err := r.checkSchema(ctx); err != nil {
			return err
		}
	}
	return nil
}
