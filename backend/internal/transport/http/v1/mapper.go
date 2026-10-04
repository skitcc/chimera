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
	return TrackResponse{
		ID:        t.ID,
		UserID:    t.UserID,
		Title:     t.Title,
		Artist:    t.Artist,
		Status:    string(t.Status),
		SizeBytes: t.SizeBytes,
	}
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

func (r TrackUploadInitRequest) toDomain(userID string) domain.TrackUploadInit {
	artist := ""
	if r.Artist != nil {
		artist = *r.Artist
	}
	return domain.TrackUploadInit{UserID: userID, Title: r.Title, Artist: artist, SizeBytes: r.Size}
}

func uploadSessionToResponse(s domain.TrackUploadSession) TrackUploadInitResponse {
	return TrackUploadInitResponse{
		Track:     trackToResponse(s.Track),
		UploadURL: s.UploadURL,
	}
}

func (r RegisterRequest) toDomain() domain.RegisterInput {
	return domain.RegisterInput{Email: r.Email, Password: r.Password, Name: r.Name}
}

func (r LoginRequest) toDomain() domain.LoginInput {
	return domain.LoginInput{Email: r.Email, Password: r.Password}
}

func userPageToResponse(page domain.UserPage) UserPageResponse {
	return UserPageResponse{
		Items: usersToResponse(page.Items), NextCursor: page.NextCursor, Limit: page.Limit,
	}
}

func playlistToResponse(playlist domain.Playlist) PlaylistResponse {
	return PlaylistResponse{
		ID: playlist.ID, OwnerID: playlist.OwnerID, Title: playlist.Title, IsPublic: playlist.IsPublic,
	}
}

func playlistPageToResponse(page domain.PlaylistPage) PlaylistPageResponse {
	items := make([]PlaylistResponse, 0, len(page.Items))
	for _, playlist := range page.Items {
		items = append(items, playlistToResponse(playlist))
	}
	return PlaylistPageResponse{Items: items, NextCursor: page.NextCursor, Limit: page.Limit}
}

func playlistTrackToResponse(item domain.PlaylistTrack) PlaylistTrackResponse {
	return PlaylistTrackResponse{Position: item.Position, Track: trackToResponse(item.Track)}
}

func playlistTrackPageToResponse(page domain.PlaylistTrackPage) PlaylistTrackPageResponse {
	items := make([]PlaylistTrackResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, playlistTrackToResponse(item))
	}
	return PlaylistTrackPageResponse{Items: items, NextCursor: page.NextCursor, Limit: page.Limit}
}

func pickToResponse(pick domain.Pick) PickResponse {
	return PickResponse{
		ID: pick.ID, UserID: pick.UserID, Title: pick.Title, GeneratedAt: pick.GeneratedAt,
	}
}

func pickDetailToResponse(detail domain.PickDetail) PickDetailResponse {
	tracks := make([]PickTrackResponse, 0, len(detail.Tracks))
	for _, item := range detail.Tracks {
		tracks = append(tracks, PickTrackResponse{
			Position: item.Position, Note: item.Note, Track: trackToResponse(item.Track),
		})
	}
	return PickDetailResponse{PickResponse: pickToResponse(detail.Pick), Tracks: tracks}
}

func pickPageToResponse(page domain.PickPage) PickPageResponse {
	items := make([]PickResponse, 0, len(page.Items))
	for _, pick := range page.Items {
		items = append(items, pickToResponse(pick))
	}
	return PickPageResponse{Items: items, NextCursor: page.NextCursor, Limit: page.Limit}
}
