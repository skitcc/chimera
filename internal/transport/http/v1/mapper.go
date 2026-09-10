package v1

import "chimera/internal/domain"

func userToResponse(u domain.User) UserResponse {
	return UserResponse{ID: u.ID, Email: u.Email, Name: u.Name}
}

func usersToResponse(users []domain.User) []UserResponse {
	out := make([]UserResponse, 0, len(users))
	for _, u := range users {
		out = append(out, userToResponse(u))
	}
	return out
}

func trackToResponse(t domain.Track) TrackResponse {
	return TrackResponse{ID: t.ID, Title: t.Title, Artist: t.Artist}
}

func trackPageToResponse(p domain.TrackPage) TrackPageResponse {
	items := make([]TrackResponse, 0, len(p.Items))
	for _, t := range p.Items {
		items = append(items, trackToResponse(t))
	}
	return TrackPageResponse{
		Items:      items,
		NextCursor: p.NextCursor,
		Limit:      p.Limit,
	}
}

func authToResponse(a domain.AuthResult) AuthResponse {
	return AuthResponse{
		Token: a.Token,
		User:  userToResponse(a.User),
	}
}

func (r UserWriteRequest) toDomain() domain.UserWrite {
	return domain.UserWrite{Email: r.Email, Name: r.Name, Password: r.Password}
}

func (r TrackWriteRequest) toDomain() domain.TrackWrite {
	return domain.TrackWrite{Title: r.Title, Artist: r.Artist}
}

func (r RegisterRequest) toDomain() domain.RegisterInput {
	return domain.RegisterInput{Email: r.Email, Password: r.Password, Name: r.Name}
}

func (r LoginRequest) toDomain() domain.LoginInput {
	return domain.LoginInput{Email: r.Email, Password: r.Password}
}
