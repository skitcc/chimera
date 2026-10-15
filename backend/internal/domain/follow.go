package domain

import "time"

const maxFollowBatch = 100

type Follow struct {
	Follower  User
	Following User
	CreatedAt time.Time
}

// FollowFilter selects follows. An empty field matches any user.
type FollowFilter struct {
	FollowerID  string
	FollowingID string
}

type FollowListQuery struct {
	PageQuery
	FollowerID  string
	FollowingID string
}

func (q *FollowListQuery) Validate() error {
	if q.FollowerID == "" && q.FollowingID == "" {
		return Invalid("follower_id or following_id is required")
	}
	return q.PageQuery.Validate()
}

func (q FollowListQuery) Filter() FollowFilter {
	return FollowFilter{FollowerID: q.FollowerID, FollowingID: q.FollowingID}
}

// FollowBatch is one follower and up to 100 distinct authors, applied atomically.
type FollowBatch struct {
	FollowerID   string
	FollowingIDs []string
}

func (b FollowBatch) Validate() error {
	if err := UserID(b.FollowerID).Validate(); err != nil {
		return err
	}
	if len(b.FollowingIDs) == 0 {
		return Invalid("following ids are required")
	}
	if len(b.FollowingIDs) > maxFollowBatch {
		return Invalid("too many following ids")
	}
	seen := make(map[string]bool, len(b.FollowingIDs))
	for _, id := range b.FollowingIDs {
		if !validUUID(id) {
			return Invalid("invalid following id")
		}
		if seen[id] {
			return Invalid("duplicate following id")
		}
		seen[id] = true
	}
	return nil
}

func (b FollowBatch) ValidateCreate() error {
	if err := b.Validate(); err != nil {
		return err
	}
	for _, id := range b.FollowingIDs {
		if id == b.FollowerID {
			return Invalid("cannot follow yourself")
		}
	}
	return nil
}

func validUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F') {
				return false
			}
		}
	}
	return true
}
