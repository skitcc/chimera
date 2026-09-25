package usecase

import (
	"context"
	"errors"

	"chimera/internal/domain"
)

type fakeUserRepository struct {
	users      map[string]domain.User
	passwords  map[string]string
	listErr    error
	getErr     error
	createErr  error
	updateErr  error
	deleteErr  error
	nextUserID string
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		users:      make(map[string]domain.User),
		passwords:  make(map[string]string),
		nextUserID: testUserID,
	}
}

func (f *fakeUserRepository) List(context.Context) ([]domain.User, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	users := make([]domain.User, 0, len(f.users))
	for _, user := range f.users {
		users = append(users, user)
	}
	return users, nil
}

func (f *fakeUserRepository) GetByID(_ context.Context, id string) (domain.User, error) {
	if f.getErr != nil {
		return domain.User{}, f.getErr
	}
	user, ok := f.users[id]
	if !ok {
		return domain.User{}, domain.NotFound("user not found")
	}
	return user, nil
}

func (f *fakeUserRepository) Create(_ context.Context, user domain.User, passwordHash string) (domain.User, error) {
	if f.createErr != nil {
		return domain.User{}, f.createErr
	}
	if user.ID == "" {
		user.ID = f.nextUserID
	}
	f.users[user.ID] = user
	f.passwords[user.ID] = passwordHash
	return user, nil
}

func (f *fakeUserRepository) Update(_ context.Context, user domain.User) (domain.User, error) {
	if f.updateErr != nil {
		return domain.User{}, f.updateErr
	}
	if _, ok := f.users[user.ID]; !ok {
		return domain.User{}, domain.NotFound("user not found")
	}
	f.users[user.ID] = user
	return user, nil
}

func (f *fakeUserRepository) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.users[id]; !ok {
		return domain.NotFound("user not found")
	}
	delete(f.users, id)
	delete(f.passwords, id)
	return nil
}

func (f *fakeUserRepository) GetByEmail(_ context.Context, email string) (domain.AuthUser, error) {
	if f.getErr != nil {
		return domain.AuthUser{}, f.getErr
	}
	for id, user := range f.users {
		if user.Email == email {
			return domain.AuthUser{User: user, PasswordHash: f.passwords[id]}, nil
		}
	}
	return domain.AuthUser{}, domain.NotFound("user not found")
}

type fakePasswordHasher struct {
	hashErr    error
	compareErr error
}

func (f *fakePasswordHasher) Hash(password string) (string, error) {
	if f.hashErr != nil {
		return "", f.hashErr
	}
	return "hash:" + password, nil
}

func (f *fakePasswordHasher) Compare(hash, password string) error {
	if f.compareErr != nil {
		return f.compareErr
	}
	if hash != "hash:"+password {
		return errors.New("password mismatch")
	}
	return nil
}

type fakeTokenIssuer struct {
	token string
	err   error
}

func (f *fakeTokenIssuer) Issue(domain.User) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

type fakeTrackRepository struct {
	tracks       map[string]domain.Track
	order        []string
	listErr      error
	getErr       error
	createErr    error
	updateErrors []error
	deleteErr    error
	nextTrackID  string
}

func newFakeTrackRepository() *fakeTrackRepository {
	return &fakeTrackRepository{
		tracks:      make(map[string]domain.Track),
		nextTrackID: testTrackID,
	}
}

func (f *fakeTrackRepository) seed(tracks ...domain.Track) {
	for _, track := range tracks {
		if _, ok := f.tracks[track.ID]; !ok {
			f.order = append(f.order, track.ID)
		}
		f.tracks[track.ID] = track
	}
}

func (f *fakeTrackRepository) List(_ context.Context, filter domain.TrackFilter) ([]domain.Track, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	tracks := make([]domain.Track, 0, len(f.tracks))
	for _, id := range f.order {
		track := f.tracks[id]
		if filter.Status != "" && track.Status != filter.Status {
			continue
		}
		if filter.UserID != "" && track.UserID != filter.UserID {
			continue
		}
		if filter.Artist != "" && track.Artist != filter.Artist {
			continue
		}
		tracks = append(tracks, track)
	}
	return tracks, nil
}

func (f *fakeTrackRepository) GetByID(_ context.Context, id string) (domain.Track, error) {
	if f.getErr != nil {
		return domain.Track{}, f.getErr
	}
	track, ok := f.tracks[id]
	if !ok {
		return domain.Track{}, domain.NotFound("track not found")
	}
	return track, nil
}

func (f *fakeTrackRepository) Create(_ context.Context, track domain.Track) (domain.Track, error) {
	if f.createErr != nil {
		return domain.Track{}, f.createErr
	}
	if track.ID == "" {
		track.ID = f.nextTrackID
	}
	f.seed(track)
	return track, nil
}

func (f *fakeTrackRepository) Update(_ context.Context, track domain.Track) (domain.Track, error) {
	if len(f.updateErrors) > 0 {
		err := f.updateErrors[0]
		f.updateErrors = f.updateErrors[1:]
		if err != nil {
			return domain.Track{}, err
		}
	}
	if _, ok := f.tracks[track.ID]; !ok {
		return domain.Track{}, domain.NotFound("track not found")
	}
	f.tracks[track.ID] = track
	return track, nil
}

func (f *fakeTrackRepository) Delete(_ context.Context, id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	if _, ok := f.tracks[id]; !ok {
		return domain.NotFound("track not found")
	}
	delete(f.tracks, id)
	for i, trackID := range f.order {
		if trackID == id {
			f.order = append(f.order[:i], f.order[i+1:]...)
			break
		}
	}
	return nil
}

type fakeTrackLikeRepository struct {
	tracks    *fakeTrackRepository
	likes     map[string]map[string]bool
	addErr    error
	removeErr error
	listErr   error
}

func newFakeTrackLikeRepository(tracks *fakeTrackRepository) *fakeTrackLikeRepository {
	return &fakeTrackLikeRepository{
		tracks: tracks,
		likes:  make(map[string]map[string]bool),
	}
}

func (f *fakeTrackLikeRepository) Add(_ context.Context, userID, trackID string) error {
	if f.addErr != nil {
		return f.addErr
	}
	if f.likes[userID] == nil {
		f.likes[userID] = make(map[string]bool)
	}
	f.likes[userID][trackID] = true
	return nil
}

func (f *fakeTrackLikeRepository) Remove(_ context.Context, userID, trackID string) error {
	if f.removeErr != nil {
		return f.removeErr
	}
	delete(f.likes[userID], trackID)
	return nil
}

func (f *fakeTrackLikeRepository) ListReadyByUser(_ context.Context, userID string) ([]domain.Track, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	tracks := make([]domain.Track, 0)
	for _, id := range f.tracks.order {
		track := f.tracks.tracks[id]
		if f.likes[userID][id] && track.Status == domain.TrackReady {
			tracks = append(tracks, track)
		}
	}
	return tracks, nil
}

type fakeObjectStorage struct {
	sizes      map[string]int64
	putURL     string
	getURL     string
	putErr     error
	getErr     error
	statErrors []error
}

func newFakeObjectStorage() *fakeObjectStorage {
	return &fakeObjectStorage{
		sizes:  make(map[string]int64),
		putURL: "https://uploads.example/track",
		getURL: "https://streams.example/track",
	}
}

func (f *fakeObjectStorage) PresignPut(context.Context, string) (string, error) {
	if f.putErr != nil {
		return "", f.putErr
	}
	return f.putURL, nil
}

func (f *fakeObjectStorage) PresignGet(context.Context, string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.getURL, nil
}

func (f *fakeObjectStorage) Stat(_ context.Context, key string) (int64, error) {
	if len(f.statErrors) > 0 {
		err := f.statErrors[0]
		f.statErrors = f.statErrors[1:]
		if err != nil {
			return 0, err
		}
	}
	return f.sizes[key], nil
}
