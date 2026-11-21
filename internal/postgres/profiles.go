package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/kmradished/GOsapiens/internal/domain"
)

func (s *Store) ReadProfile(ctx context.Context, actorID, profileID domain.UserID) (domain.ProfileRevision, error) {
	var profile domain.ProfileRevision
	err := s.Pool.QueryRow(ctx, `
		SELECT id, name, bio, version
		FROM profiles
		WHERE id = $1
	`, profileID).Scan(&profile.ID, &profile.Name, &profile.Bio, &profile.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProfileRevision{}, domain.ErrProfileNotFound
	}
	if err != nil {
		return domain.ProfileRevision{}, fmt.Errorf("read profile: %w", classify(err))
	}
	if actorID != profileID {
		return domain.ProfileRevision{}, domain.ErrForbidden
	}
	return profile, nil
}

func (s *Store) UpdateProfile(
	ctx context.Context, actorID, profileID domain.UserID, edit domain.ProfileEdit,
) (domain.ProfileRevision, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.ProfileRevision{}, fmt.Errorf("begin profile update: %w", classify(err))
	}
	defer rollback(tx)

	var version int64
	err = tx.QueryRow(ctx, `
		SELECT version
		FROM profiles
		WHERE id = $1
		FOR UPDATE
	`, profileID).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProfileRevision{}, domain.ErrProfileNotFound
	}
	if err != nil {
		return domain.ProfileRevision{}, fmt.Errorf("lock profile: %w", classify(err))
	}
	if actorID != profileID {
		return domain.ProfileRevision{}, domain.ErrForbidden
	}
	if version != edit.ExpectedVersion {
		return domain.ProfileRevision{}, domain.ErrVersionConflict
	}

	var profile domain.ProfileRevision
	err = tx.QueryRow(ctx, `
		UPDATE profiles
		SET name = $2, bio = $3, version = version + 1
		WHERE id = $1 AND version = $4
		RETURNING id, name, bio, version
	`, profileID, edit.Name, edit.Bio, edit.ExpectedVersion).Scan(
		&profile.ID, &profile.Name, &profile.Bio, &profile.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ProfileRevision{}, domain.ErrVersionConflict
	}
	if err != nil {
		return domain.ProfileRevision{}, fmt.Errorf("update profile: %w", classify(err))
	}
	if err := commit(ctx, tx); err != nil {
		return domain.ProfileRevision{}, fmt.Errorf("commit profile update: %w", err)
	}
	return profile, nil
}
