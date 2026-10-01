package domain

type UserID string

type Interests struct {
	Movies []string
	Music  []string
	Books  []string
}

type Profile struct {
	ID        UserID
	Name      string
	PhotoURL  string
	Bio       string
	Active    bool
	Interests Interests
}

type ReactionKind string

const (
	ReactionLike    ReactionKind = "like"
	ReactionDislike ReactionKind = "dislike"
)

type Reaction struct {
	SenderID   UserID
	ReceiverID UserID
	Kind       ReactionKind
}

type Block struct {
	BlockerID UserID
	BlockedID UserID
}

type View struct {
	ViewerID    UserID
	CandidateID UserID
}

type Match struct {
	FirstUserID  UserID
	SecondUserID UserID
}
