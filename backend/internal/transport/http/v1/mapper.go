package v1

import "chimera/internal/domain"

func userToResponse(u domain.User) User {
	return User{ID: u.ID, Email: u.Email, Name: u.Name}
}

func userPageToResponse(page domain.UserPage) UserPage {
	items := make([]User, 0, len(page.Items))
	for _, u := range page.Items {
		items = append(items, userToResponse(u))
	}
	return UserPage{Items: items, NextCursor: page.NextCursor, Limit: page.Limit}
}

func sessionToResponse(a domain.AuthResult) Session {
	return Session{Token: a.Token, UserID: a.User.ID}
}

func followToResponse(f domain.Follow) Follow {
	return Follow{
		Follower:  userToResponse(f.Follower),
		Following: userToResponse(f.Following),
		CreatedAt: f.CreatedAt,
	}
}

func followPageToResponse(page domain.FollowPage) FollowPage {
	items := make([]Follow, 0, len(page.Items))
	for _, f := range page.Items {
		items = append(items, followToResponse(f))
	}
	return FollowPage{Items: items, NextCursor: page.NextCursor, Limit: page.Limit}
}

func trackToResponse(t domain.Track) Track {
	return Track{
		ID:        t.ID,
		UserID:    t.UserID,
		Title:     t.Title,
		Artist:    t.Artist,
		Status:    string(t.Status),
		SizeBytes: t.SizeBytes,
	}
}

func trackPageToResponse(p domain.TrackPage) TrackPage {
	items := make([]Track, 0, len(p.Items))
	for _, t := range p.Items {
		items = append(items, trackToResponse(t))
	}
	return TrackPage{Items: items, NextCursor: p.NextCursor, Limit: p.Limit}
}

func uploadSessionToResponse(s domain.TrackUploadSession) TrackUpload {
	return TrackUpload{Track: trackToResponse(s.Track), UploadURL: s.UploadURL}
}

func (r TrackCreate) toDomain(userID string) domain.TrackUploadInit {
	return domain.TrackUploadInit{UserID: userID, Title: r.Title, Artist: *r.Artist, SizeBytes: r.Size}
}

func playlistToResponse(playlist domain.Playlist) Playlist {
	return Playlist{
		ID: playlist.ID, OwnerID: playlist.OwnerID, Title: playlist.Title, IsPublic: playlist.IsPublic,
	}
}

func playlistPageToResponse(page domain.PlaylistPage) PlaylistPage {
	items := make([]Playlist, 0, len(page.Items))
	for _, playlist := range page.Items {
		items = append(items, playlistToResponse(playlist))
	}
	return PlaylistPage{Items: items, NextCursor: page.NextCursor, Limit: page.Limit}
}

func playlistTrackToResponse(item domain.PlaylistTrack) PlaylistTrack {
	return PlaylistTrack{Position: item.Position, Track: trackToResponse(item.Track)}
}

func playlistTrackPageToResponse(page domain.PlaylistTrackPage) PlaylistTrackPage {
	items := make([]PlaylistTrack, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, playlistTrackToResponse(item))
	}
	return PlaylistTrackPage{Items: items, NextCursor: page.NextCursor, Limit: page.Limit}
}
