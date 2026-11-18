package clients

import (
	"context"

	identityv1 "chimera/contracts/gen/identity/v1"
	"chimera/gateway/internal/domain"
	"chimera/platform/idempotency"
)

// Identity serves /v1/sessions and /v1/users.
type Identity struct {
	client identityv1.IdentityServiceClient
}

func NewIdentity(client identityv1.IdentityServiceClient) *Identity {
	return &Identity{client: client}
}

func (c *Identity) Login(ctx context.Context, in domain.LoginInput) (domain.AuthResult, error) {
	resp, err := c.client.CreateSession(ctx, &identityv1.CreateSessionRequest{Email: in.Email, Password: in.Password})
	if err != nil {
		return domain.AuthResult{}, err
	}
	return domain.AuthResult{Token: resp.GetToken(), User: toUser(resp.GetUser())}, nil
}

func (c *Identity) List(ctx context.Context, q domain.PageQuery) (domain.UserPage, error) {
	resp, err := c.client.ListUsers(ctx, &identityv1.ListUsersRequest{Page: page(q)})
	if err != nil {
		return domain.UserPage{}, err
	}
	items := make([]domain.User, 0, len(resp.GetItems()))
	for _, u := range resp.GetItems() {
		items = append(items, toUser(u))
	}
	return domain.UserPage{Items: items, NextCursor: resp.GetNextCursor(), Limit: int(resp.GetLimit())}, nil
}

func (c *Identity) GetByID(ctx context.Context, id string) (domain.User, error) {
	resp, err := c.client.GetUser(ctx, &identityv1.GetUserRequest{Id: id})
	if err != nil {
		return domain.User{}, err
	}
	return toUser(resp.GetUser()), nil
}

func (c *Identity) Create(ctx context.Context, in domain.UserWrite) (domain.User, error) {
	resp, err := c.client.CreateUser(ctx, &identityv1.CreateUserRequest{
		Email:          in.Email,
		Name:           in.Name,
		Password:       in.Password,
		IdempotencyKey: idempotency.From(ctx),
	})
	if err != nil {
		return domain.User{}, err
	}
	return toUser(resp.GetUser()), nil
}

func (c *Identity) Replace(ctx context.Context, actorID, id string, in domain.UserReplace) (domain.User, error) {
	resp, err := c.client.ReplaceUser(ctx, &identityv1.ReplaceUserRequest{ActorId: actorID, Id: id, Email: in.Email, Name: in.Name})
	if err != nil {
		return domain.User{}, err
	}
	return toUser(resp.GetUser()), nil
}

func (c *Identity) Patch(ctx context.Context, actorID, id string, in domain.UserPatch) (domain.User, error) {
	resp, err := c.client.PatchUser(ctx, &identityv1.PatchUserRequest{
		ActorId:  actorID,
		Id:       id,
		Email:    in.Email,
		Name:     in.Name,
		Password: in.Password,
	})
	if err != nil {
		return domain.User{}, err
	}
	return toUser(resp.GetUser()), nil
}

func (c *Identity) Delete(ctx context.Context, actorID, id string) error {
	_, err := c.client.DeleteUser(ctx, &identityv1.DeleteUserRequest{ActorId: actorID, Id: id})
	return err
}
