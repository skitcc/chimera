// Package identity reads user profiles from the Identity service over gRPC.
package identity

import (
	"context"

	commonv1 "chimera/contracts/gen/common/v1"
	identityv1 "chimera/contracts/gen/identity/v1"
	"chimera/platform/readpref"
	"chimera/social/internal/domain"
)

type Client struct {
	api identityv1.IdentityServiceClient
}

func New(api identityv1.IdentityServiceClient) *Client {
	return &Client{api: api}
}

func (c *Client) GetUser(ctx context.Context, id string) (domain.User, error) {
	res, err := c.api.GetUser(ctx, &identityv1.GetUserRequest{Id: id, Consistency: consistency(ctx)})
	if err != nil {
		return domain.User{}, err
	}
	return fromUser(res.GetUser()), nil
}

func (c *Client) GetUsers(ctx context.Context, ids []string) ([]domain.User, error) {
	if len(ids) == 0 {
		return []domain.User{}, nil
	}
	res, err := c.api.BatchGetUsers(ctx, &identityv1.BatchGetUsersRequest{Ids: ids})
	if err != nil {
		return nil, err
	}
	users := make([]domain.User, 0, len(res.GetUsers()))
	for _, u := range res.GetUsers() {
		users = append(users, fromUser(u))
	}
	return users, nil
}

func fromUser(u *identityv1.User) domain.User {
	return domain.User{ID: u.GetId(), Email: u.GetEmail(), Name: u.GetName()}
}

// consistency asks Identity for the primary while Social serves a write, so a check
// that guards the write never sees a lagging replica.
func consistency(ctx context.Context) commonv1.Consistency {
	if readpref.IsPrimary(ctx) {
		return commonv1.Consistency_CONSISTENCY_STRONG
	}
	return commonv1.Consistency_CONSISTENCY_EVENTUAL
}
