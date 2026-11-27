//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"chimera/testkit"
)

// eventualTimeout bounds how long a deletion may take to reach the other
// services through Kafka.
const eventualTimeout = 60 * time.Second

func TestDemoScenario(t *testing.T) {
	base := os.Getenv("E2E_API_URL")
	if base == "" {
		t.Skip("E2E_API_URL is not set")
	}
	base = strings.TrimRight(base, "/")

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	run := os.Getenv("E2E_RUN")
	if run == "" {
		run = suffix
	}
	artistEmail := "artist-" + suffix + "@example.com"
	listenerEmail := "listener-" + suffix + "@example.com"
	password, _, err := testPasswords()
	if err != nil {
		t.Fatal(err)
	}
	artistName := "DemoArtist" + suffix
	title := "Demo Track " + suffix
	audio := bytes.Repeat([]byte("chimera-demo-audio\n"), 64)

	api := &client{base: base, run: "go-" + run, http: &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}

	var artist, listener session
	var trackID, draftID, uploadURL, streamURL, playlistID string

	testkit.RunSpec(t, testkit.Spec{
		ID:        "E2E-DEMO-01",
		Layer:     testkit.LayerE2E,
		Component: "DemoScenario",
		Method:    "MVP",
		Title:     "publishes a track, lets another user like and stream it, and cascades deletions",
		Given:     "a running Gateway, Identity, Catalog, Social, Kafka, PostgreSQL and object storage",
		When:      "an artist uploads and publishes a track, a listener likes, streams and saves it, then the track and the artist are deleted",
		Then:      "the track stays hidden until ready, the streamed bytes match, and Kafka events remove likes, playlist entries, follows and the artist's tracks",
		Technique: testkit.TechniqueState,
		Severity:  testkit.SeverityBlocker,
		Kind:      testkit.KindE2E,
	}, func(t *testing.T, r testkit.Report) {
		api.event = "start"
		t.Cleanup(func() {
			result := "passed"
			if t.Failed() {
				result = "failed"
			}
			api.mark(t, "finish", result)
		})

		step := func(name string, body func(*testing.T)) {
			r.Step(name, func(t *testing.T) {
				api.step = name[:strings.IndexByte(name, ' ')]
				body(t)
			})
		}

		step("01 ready", func(t *testing.T) {
			// The Gateway re-probes its upstreams on an interval, so right after a
			// deploy /ready may still report the previous round.
			eventually(t, "gateway ready", func() bool {
				status, _, _ := api.do(t, http.MethodGet, "/ready", "", nil)
				return status == http.StatusOK
			})
		})

		step("02 register artist", func(t *testing.T) {
			artist = api.register(t, artistEmail, "Demo Artist", password)
		})

		step("03 artist profile", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodGet, "/v1/users/"+artist.UserID, "", nil)
			requireStatus(t, http.StatusOK, status, body)
			if user := decode[userResponse](t, body); user.Email != artistEmail {
				t.Fatalf("profile email %q", user.Email)
			}
		})

		step("04 create track", func(t *testing.T) {
			status, header, body := api.do(t, http.MethodPost, "/v1/tracks", artist.Token, jsonBody(t, map[string]any{
				"title": title, "artist": artistName, "size": len(audio),
			}))
			requireStatus(t, http.StatusCreated, status, body)
			upload := decode[uploadResponse](t, body)
			if upload.Track.ID == "" || upload.Track.Status != "pending" || upload.UploadURL == "" {
				t.Fatalf("create response: %s", body)
			}
			if upload.Track.SizeBytes != int64(len(audio)) {
				t.Fatalf("declared size %d", upload.Track.SizeBytes)
			}
			if header.Get("Location") != "/v1/tracks/"+upload.Track.ID {
				t.Fatalf("Location %q", header.Get("Location"))
			}
			trackID = upload.Track.ID
			uploadURL = upload.UploadURL
		})

		step("05 put object", func(t *testing.T) {
			status, _, body := api.doURL(t, http.MethodPut, uploadURL, "", audio)
			if status != http.StatusOK && status != http.StatusNoContent {
				t.Fatalf("put object status %d body %s", status, body)
			}
		})

		step("06 feed hides pending track", func(t *testing.T) {
			if page := api.tracks(t, "/v1/tracks?limit=100&artist="+artistName, ""); page.has(trackID) {
				t.Fatalf("pending track %s is in the public feed", trackID)
			}
		})

		step("07 publish track", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPatch, "/v1/tracks/"+trackID, artist.Token, jsonBody(t, map[string]string{"status": "ready"}))
			requireStatus(t, http.StatusOK, status, body)
			track := decode[trackResponse](t, body)
			if track.ID != trackID || track.Status != "ready" || track.Artist != artistName {
				t.Fatalf("publish response: %s", body)
			}
		})

		step("08 feed shows ready track", func(t *testing.T) {
			// ListTracks may read a replica, so the row can arrive a moment after the write.
			eventually(t, "ready track in the feed", func() bool {
				track, ok := api.tracks(t, "/v1/tracks?limit=100&artist="+artistName, "").find(trackID)
				return ok && track.Status == "ready"
			})
		})

		step("09 register listener", func(t *testing.T) {
			listener = api.register(t, listenerEmail, "Demo Listener", password)
		})

		likePath := func() string { return "/v1/users/" + listener.UserID + "/likes/" + trackID }

		step("10 like without token", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPut, likePath(), "", nil)
			requireStatus(t, http.StatusUnauthorized, status, body)
			if e := decode[errorResponse](t, body); e.Code != "unauthorized" {
				t.Fatalf("like without token: %s", body)
			}
		})

		step("11 like for another user", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPut, "/v1/users/"+artist.UserID+"/likes/"+trackID, listener.Token, nil)
			requireStatus(t, http.StatusForbidden, status, body)
		})

		step("12 like track", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPut, likePath(), listener.Token, nil)
			requireStatus(t, http.StatusNoContent, status, body)
		})

		step("13 like track again", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPut, likePath(), listener.Token, nil)
			requireStatus(t, http.StatusNoContent, status, body)
		})

		likedPath := func() string { return "/v1/tracks?limit=100&liked_by=" + listener.UserID }

		step("14 liked list", func(t *testing.T) {
			eventually(t, "liked track in the list", func() bool {
				return api.tracks(t, likedPath(), listener.Token).has(trackID)
			})
		})

		step("15 stream redirect", func(t *testing.T) {
			status, header, body := api.do(t, http.MethodGet, "/v1/tracks/"+trackID+"/stream", "", nil)
			requireStatus(t, http.StatusFound, status, body)
			if streamURL = header.Get("Location"); streamURL == "" {
				t.Fatal("stream response has no Location")
			}
		})

		step("16 download audio", func(t *testing.T) {
			status, _, body := api.doURL(t, http.MethodGet, streamURL, "", nil)
			requireStatus(t, http.StatusOK, status, body)
			if !bytes.Equal(body, audio) {
				t.Fatalf("downloaded %d bytes, uploaded %d", len(body), len(audio))
			}
		})

		step("17 save to playlist", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPost, "/v1/playlists", listener.Token, jsonBody(t, map[string]any{
				"title": "Demo " + suffix, "is_public": true,
			}))
			requireStatus(t, http.StatusCreated, status, body)
			playlistID = decode[playlistResponse](t, body).ID
			status, _, body = api.do(t, http.MethodPost, "/v1/playlists/"+playlistID+"/tracks", listener.Token, jsonBody(t, map[string]any{
				"track_id": trackID, "position": 0,
			}))
			requireStatus(t, http.StatusCreated, status, body)
		})

		step("18 follow artist", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPut, "/v1/follows/"+listener.UserID+"/"+artist.UserID, listener.Token, nil)
			requireStatus(t, http.StatusNoContent, status, body)
		})

		step("19 delete track", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodDelete, "/v1/tracks/"+trackID, artist.Token, nil)
			requireStatus(t, http.StatusNoContent, status, body)
		})

		step("20 like and playlist entry disappear", func(t *testing.T) {
			eventually(t, "TrackDeleted reached Social", func() bool {
				status, _, body := api.do(t, http.MethodGet, "/v1/playlists/"+playlistID+"/tracks", listener.Token, nil)
				requireStatus(t, http.StatusOK, status, body)
				entries := decode[itemsResponse](t, body)
				return len(entries.Items) == 0 && !api.tracks(t, likedPath(), listener.Token).has(trackID)
			})
		})

		step("21 artist keeps a draft", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPost, "/v1/tracks", artist.Token, jsonBody(t, map[string]any{
				"title": "Draft " + suffix, "artist": artistName, "size": 1,
			}))
			requireStatus(t, http.StatusCreated, status, body)
			draftID = decode[uploadResponse](t, body).Track.ID
			// A public stream probe distinguishes a pending row from a deleted
			// row without relying on the owner's token after account deletion.
			status, _, body = api.do(t, http.MethodGet, "/v1/tracks/"+draftID+"/stream", "", nil)
			requireStatus(t, http.StatusConflict, status, body)
		})

		step("22 delete artist", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodDelete, "/v1/users/"+artist.UserID, artist.Token, nil)
			requireStatus(t, http.StatusNoContent, status, body)
		})

		step("23 artist data disappears", func(t *testing.T) {
			eventually(t, "UserDeleted reached Catalog and Social", func() bool {
				status, _, body := api.do(t, http.MethodGet, "/v1/tracks/"+draftID+"/stream", "", nil)
				if status != http.StatusNotFound {
					requireStatus(t, http.StatusConflict, status, body)
					return false
				}
				status, _, body = api.do(t, http.MethodGet, "/v1/follows?follower_id="+listener.UserID, "", nil)
				requireStatus(t, http.StatusOK, status, body)
				return len(decode[itemsResponse](t, body).Items) == 0
			})
		})
	})
}

func eventually(t *testing.T, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(eventualTimeout)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: not observed within %s", what, eventualTimeout)
		}
		time.Sleep(time.Second)
	}
}

// client tags every API request with X-Run-Id go-<run> and X-Run-Step, which
// the Gateway passes to every service and log line as run_id and step.
type client struct {
	base  string
	run   string
	step  string
	event string
	http  *http.Client
}

func (c *client) do(t *testing.T, method, path, token string, body []byte) (int, http.Header, []byte) {
	t.Helper()
	return c.doURL(t, method, c.base+path, token, body)
}

func (c *client) doURL(t *testing.T, method, rawURL, token string, body []byte) (int, http.Header, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, rawURL, reader)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if strings.HasPrefix(rawURL, c.base+"/") {
		req.Header.Set("X-Run-Id", c.run)
		req.Header.Set("X-Run-Step", c.step)
		if c.event != "" {
			req.Header.Set("X-Run-Event", c.event)
			c.event = ""
		}
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil && method != http.MethodPut {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, rawURL, err)
	}
	defer res.Body.Close()
	if strings.HasPrefix(rawURL, c.base+"/v1/") {
		if got := res.Header.Get("X-Request-Id"); len(got) != 32 {
			t.Fatalf("X-Request-Id %q", got)
		}
		if res.Header.Get("X-Chimera-Instance") == "" {
			t.Fatal("X-Chimera-Instance is empty")
		}
	}
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return res.StatusCode, res.Header, raw
}

// mark sends X-Run-Event so the Gateway logs the start or the finish of the run.
// It runs from Cleanup too, so it reports errors instead of failing the test.
func (c *client) mark(t *testing.T, event, result string) {
	req, err := http.NewRequest(http.MethodGet, c.base+"/live", nil)
	if err != nil {
		t.Logf("run %s: %v", event, err)
		return
	}
	req.Header.Set("X-Run-Id", c.run)
	req.Header.Set("X-Run-Step", event)
	req.Header.Set("X-Run-Event", event)
	if result != "" {
		req.Header.Set("X-Run-Result", result)
	}
	res, err := c.http.Do(req)
	if err != nil {
		t.Logf("run %s: %v", event, err)
		return
	}
	res.Body.Close()
}

// register creates an account and opens a session for it.
func (c *client) register(t *testing.T, email, name, password string) session {
	t.Helper()
	status, _, body := c.do(t, http.MethodPost, "/v1/users", "", jsonBody(t, map[string]string{
		"email": email, "name": name, "password": password,
	}))
	requireStatus(t, http.StatusCreated, status, body)
	user := decode[userResponse](t, body)
	if user.ID == "" || user.Email != email {
		t.Fatalf("register response: %s", body)
	}
	status, _, body = c.do(t, http.MethodPost, "/v1/sessions", "", jsonBody(t, map[string]string{
		"email": email, "password": password,
	}))
	if status == http.StatusAccepted {
		challenge := decode[struct {
			ID string `json:"challenge_id"`
		}](t, body)
		code, err := emailCode(email, "login")
		if err != nil {
			t.Fatal(err)
		}
		status, _, body = c.do(t, http.MethodPost, "/v1/sessions/verify", "", jsonBody(t, map[string]string{"challenge_id": challenge.ID, "code": code}))
	}
	requireStatus(t, http.StatusCreated, status, body)
	s := decode[session](t, body)
	if s.Token == "" || s.UserID != user.ID {
		t.Fatal("invalid session response")
	}
	return s
}

func (c *client) tracks(t *testing.T, path, token string) pageResponse {
	t.Helper()
	status, _, body := c.do(t, http.MethodGet, path, token, nil)
	requireStatus(t, http.StatusOK, status, body)
	return decode[pageResponse](t, body)
}

func jsonBody(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return raw
}

func decode[T any](t *testing.T, raw []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return v
}

func requireStatus(t *testing.T, want, got int, body []byte) {
	t.Helper()
	if got != want {
		t.Fatalf("status %d, want %d", got, want)
	}
}

type session struct {
	Token  string `json:"token"`
	UserID string `json:"user_id"`
}

type userResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

type trackResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Status    string `json:"status"`
	SizeBytes int64  `json:"size_bytes"`
}

type uploadResponse struct {
	Track     trackResponse `json:"track"`
	UploadURL string        `json:"upload_url"`
}

type playlistResponse struct {
	ID string `json:"id"`
}

type pageResponse struct {
	Items []trackResponse `json:"items"`
	Limit int             `json:"limit"`
}

type itemsResponse struct {
	Items []json.RawMessage `json:"items"`
}

func (p pageResponse) has(id string) bool {
	_, ok := p.find(id)
	return ok
}

func (p pageResponse) find(id string) (trackResponse, bool) {
	for _, item := range p.Items {
		if item.ID == id {
			return item, true
		}
	}
	return trackResponse{}, false
}

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
