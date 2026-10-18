package domain

import "errors"

var ErrInvalidPair = errors.New("pair must contain two distinct non-empty user IDs")

type PairKey struct {
	LeftID  UserID
	RightID UserID
}

func NewPairKey(firstID, secondID UserID) (PairKey, error) {
	if firstID == "" || secondID == "" || firstID == secondID {
		return PairKey{}, ErrInvalidPair
	}

	if secondID < firstID {
		firstID, secondID = secondID, firstID
	}

	return PairKey{
		LeftID:  firstID,
		RightID: secondID,
	}, nil
}

func (key PairKey) Valid() bool {
	return key.LeftID != "" &&
		key.RightID != "" &&
		key.LeftID < key.RightID
}

type PairState struct {
	LeftLiked  bool
	RightLiked bool
}

type LikeDecision struct {
	State      PairState
	Changed    bool
	Matched    bool
	MatchedNow bool
}
