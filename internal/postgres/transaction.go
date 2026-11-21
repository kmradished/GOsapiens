package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func rollback(tx pgx.Tx) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		slog.Error("rollback failed", "error", err)
	}
}

func classify(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "40001", "40P01":
		return fmt.Errorf("%w: %w", domain.ErrTransactionRetryable, err)
	case "23503":
		if pgErr.ConstraintName == "pairs_left_profile_fk" || pgErr.ConstraintName == "pairs_right_profile_fk" {
			return fmt.Errorf("%w: %w", domain.ErrProfileNotFound, err)
		}
	}
	return err
}

func commit(ctx context.Context, tx pgx.Tx) error {
	err := tx.Commit(ctx)
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) || errors.Is(err, pgx.ErrTxCommitRollback) {
		return classify(err)
	}
	// Нет подтверждения от сервера: нельзя обещать откат или слепо повторять.
	return fmt.Errorf("%w: %w", domain.ErrCommitUnconfirmed, err)
}
