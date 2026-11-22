package postgres

import (
	"context"
	"fmt"

	"github.com/kmradished/GOsapiens/internal/domain"
)

func (s *Store) Profiles(ctx context.Context) ([]domain.Profile, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, name, bio, photo_url, active, movies, music, books
		FROM profiles ORDER BY position`)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", classify(err))
	}
	defer rows.Close()
	profiles := make([]domain.Profile, 0)
	for rows.Next() {
		var profile domain.Profile
		if err := rows.Scan(&profile.ID, &profile.Name, &profile.Bio, &profile.PhotoURL,
			&profile.Active, &profile.Interests.Movies, &profile.Interests.Music, &profile.Interests.Books); err != nil {
			return nil, fmt.Errorf("scan discovery profile: %w", err)
		}
		profiles = append(profiles, profile)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read discovery profiles: %w", classify(err))
	}
	return profiles, nil
}

func (s *Store) LikedIDs(ctx context.Context, actorID domain.UserID) ([]domain.UserID, error) {
	return s.discoveryIDs(ctx, `
		SELECT right_id AS id FROM pairs WHERE left_id = $1 AND left_liked
		UNION ALL
		SELECT left_id AS id FROM pairs WHERE right_id = $1 AND right_liked
		ORDER BY id`, actorID)
}

func (s *Store) BlockedIDs(ctx context.Context, actorID domain.UserID) ([]domain.UserID, error) {
	return s.discoveryIDs(ctx, `
		SELECT right_id AS id FROM pairs
		WHERE left_id = $1 AND (left_blocked OR right_blocked)
		UNION ALL
		SELECT left_id AS id FROM pairs
		WHERE right_id = $1 AND (left_blocked OR right_blocked)
		ORDER BY id`, actorID)
}

func (s *Store) discoveryIDs(ctx context.Context, query string, actorID domain.UserID) ([]domain.UserID, error) {
	rows, err := s.Pool.Query(ctx, query, actorID)
	if err != nil {
		return nil, fmt.Errorf("query discovery IDs: %w", classify(err))
	}
	defer rows.Close()
	ids := make([]domain.UserID, 0)
	for rows.Next() {
		var id domain.UserID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan discovery ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read discovery IDs: %w", classify(err))
	}
	return ids, nil
}
