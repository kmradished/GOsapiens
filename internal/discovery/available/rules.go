package available

import "github.com/kmradished/GOsapiens/internal/domain"

func FilterCandidates(
	currentUserID domain.UserID,
	profiles []domain.Profile,
	blockedIDs []domain.UserID,
	viewedIDs []domain.UserID,
	reactedIDs []domain.UserID,
) []Candidate {
	result := make([]Candidate, 0)

	for _, profile := range profiles {
		if profile.ID == currentUserID {
			continue
		}
		if !profile.Active {
			continue
		}
		if contains(blockedIDs, profile.ID) {
			continue
		}
		if contains(viewedIDs, profile.ID) {
			continue
		}
		if contains(reactedIDs, profile.ID) {
			continue
		}

		result = append(result, Candidate{ID: profile.ID})
	}

	return result
}

func contains(ids []domain.UserID, id domain.UserID) bool {
	for _, currentID := range ids {
		if currentID == id {
			return true
		}
	}
	return false
}
