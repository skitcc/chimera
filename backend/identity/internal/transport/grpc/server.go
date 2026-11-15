// Package grpc exposes the Identity use cases as chimera.identity.v1.IdentityService.
package grpc

import (
	"context"

	identityv1 "chimera/contracts/gen/identity/v1"
	"chimera/identity/internal/domain"
)

type UserService interface {
	List(ctx context.Context, q domain.PageQuery) (domain.UserPage, error)
	GetByID(ctx context.Context, id string) (domain.User, error)
	BatchGet(ctx context.Context, ids domain.UserIDs) ([]domain.User, error)
	Create(ctx context.Context, in domain.UserWrite) (domain.User, error)
	Replace(ctx context.Context, actorID, userID string, in domain.UserReplace) (domain.User, error)
	Patch(ctx context.Context, actorID, userID string, in domain.UserPatch) (domain.User, error)
	Delete(ctx context.Context, actorID, id string) error
}

type AuthService interface {
	Login(ctx context.Context, in domain.LoginInput) (domain.AuthResult, error)
}

type Server struct {
	identityv1.UnimplementedIdentityServiceServer
	users UserService
	auth  AuthService
}

func NewServer(users UserService, auth AuthService) *Server {
	return &Server{users: users, auth: auth}
}

func (s *Server) CreateSession(ctx context.Context, req *identityv1.CreateSessionRequest) (*identityv1.CreateSessionResponse, error) {
	res, err := s.auth.Login(ctx, domain.LoginInput{Email: req.GetEmail(), Password: req.GetPassword()})
	if err != nil {
		return nil, err
	}
	return &identityv1.CreateSessionResponse{Token: res.Token, User: toUser(res.User)}, nil
}

func (s *Server) CreateUser(ctx context.Context, req *identityv1.CreateUserRequest) (*identityv1.CreateUserResponse, error) {
	user, err := s.users.Create(ctx, domain.UserWrite{
		Email:    req.GetEmail(),
		Name:     req.GetName(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, err
	}
	return &identityv1.CreateUserResponse{User: toUser(user)}, nil
}

func (s *Server) GetUser(ctx context.Context, req *identityv1.GetUserRequest) (*identityv1.GetUserResponse, error) {
	user, err := s.users.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return &identityv1.GetUserResponse{User: toUser(user)}, nil
}

func (s *Server) BatchGetUsers(ctx context.Context, req *identityv1.BatchGetUsersRequest) (*identityv1.BatchGetUsersResponse, error) {
	users, err := s.users.BatchGet(ctx, req.GetIds())
	if err != nil {
		return nil, err
	}
	return &identityv1.BatchGetUsersResponse{Users: toUsers(users)}, nil
}

func (s *Server) ListUsers(ctx context.Context, req *identityv1.ListUsersRequest) (*identityv1.ListUsersResponse, error) {
	p := req.GetPage()
	page, err := s.users.List(ctx, domain.NewPageQuery(int(p.GetLimit()), p.GetCursor(), p.GetInvalidLimit()))
	if err != nil {
		return nil, err
	}
	return &identityv1.ListUsersResponse{
		Items:      toUsers(page.Items),
		NextCursor: page.NextCursor,
		Limit:      int64(page.Limit),
	}, nil
}

func (s *Server) ReplaceUser(ctx context.Context, req *identityv1.ReplaceUserRequest) (*identityv1.ReplaceUserResponse, error) {
	user, err := s.users.Replace(ctx, req.GetActorId(), req.GetId(), domain.UserReplace{
		Email: req.GetEmail(),
		Name:  req.GetName(),
	})
	if err != nil {
		return nil, err
	}
	return &identityv1.ReplaceUserResponse{User: toUser(user)}, nil
}

func (s *Server) PatchUser(ctx context.Context, req *identityv1.PatchUserRequest) (*identityv1.PatchUserResponse, error) {
	user, err := s.users.Patch(ctx, req.GetActorId(), req.GetId(), domain.UserPatch{
		Email:    req.Email,
		Name:     req.Name,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	return &identityv1.PatchUserResponse{User: toUser(user)}, nil
}

func (s *Server) DeleteUser(ctx context.Context, req *identityv1.DeleteUserRequest) (*identityv1.DeleteUserResponse, error) {
	if err := s.users.Delete(ctx, req.GetActorId(), req.GetId()); err != nil {
		return nil, err
	}
	return &identityv1.DeleteUserResponse{}, nil
}

func toUser(u domain.User) *identityv1.User {
	return &identityv1.User{Id: u.ID, Email: u.Email, Name: u.Name}
}

func toUsers(users []domain.User) []*identityv1.User {
	out := make([]*identityv1.User, 0, len(users))
	for _, u := range users {
		out = append(out, toUser(u))
	}
	return out
}
