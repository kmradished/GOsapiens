package interests

import "github.com/kmradished/GOsapiens/internal/domain"

func Discover(current domain.Profile, candidates []domain.Profile) []AnonymousCard {
	result := make([]AnonymousCard, 0)

	for _, candidate := range candidates {
		if candidate.ID == current.ID {
			continue
		}

		if !candidate.Active {
			continue
		}

		commonMovies := common(current.Interests.Movies, candidate.Interests.Movies)
		commonMusic := common(current.Interests.Music, candidate.Interests.Music)
		commonBooks := common(current.Interests.Books, candidate.Interests.Books)

		if len(commonMovies) == 0 && len(commonMusic) == 0 && len(commonBooks) == 0 {
			continue
		}

		result = append(result, AnonymousCard{
			CandidateID:  candidate.ID,
			CommonMovies: commonMovies,
			CommonMusic:  commonMusic,
			CommonBooks:  commonBooks,
		})
	}

	return result
}

func common(left, right []string) []string {
	rightSet := make(map[string]struct{}, len(right))
	for _, value := range right {
		rightSet[value] = struct{}{}
	}

	result := make([]string, 0)
	seen := make(map[string]struct{})

	for _, value := range left {
		if _, exists := rightSet[value]; !exists {
			continue
		}

		if _, alreadyAdded := seen[value]; alreadyAdded {
			continue
		}

		result = append(result, value)
		seen[value] = struct{}{}
	}

	return result
}
