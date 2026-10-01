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

	"chimera/internal/testkit"
)

func TestDemoScenario(t *testing.T) {
	base := os.Getenv("E2E_API_URL")
	if base == "" {
		t.Skip("E2E_API_URL is not set")
	}
	base = strings.TrimRight(base, "/")

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	artistEmail := "artist-" + suffix + "@example.com"
	listenerEmail := "listener-" + suffix + "@example.com"
	const password = "password1"
	artistName := "DemoArtist" + suffix
	title := "Demo Track " + suffix
	audio := bytes.Repeat([]byte("chimera-demo-audio\n"), 64)

	api := &client{base: base, http: &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}

	var artistToken, listenerToken, trackID, uploadURL, streamURL string

	testkit.RunSpec(t, testkit.Spec{
		ID:        "E2E-DEMO-01",
		Layer:     testkit.LayerE2E,
		Component: "DemoScenario",
		Method:    "MVP",
		Title:     "publishes a track and lets another user like and stream it",
		Given:     "a running API, PostgreSQL and object storage",
		When:      "an artist registers, uploads and publishes a track, and a listener likes and streams it",
		Then:      "the track stays hidden until it is ready, a repeat like conflicts, and the streamed bytes match the upload",
		Technique: testkit.TechniqueState,
		Severity:  testkit.SeverityBlocker,
		Kind:      testkit.KindE2E,
	}, func(t *testing.T, r testkit.Report) {
		r.Step("01 ready", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodGet, "/ready", "", nil)
			requireStatus(t, http.StatusOK, status, body)
		})

		r.Step("02 register artist", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPost, "/v1/auth/register", "", jsonBody(t, map[string]string{
				"email": artistEmail, "password": password, "name": "Demo Artist",
			}))
			requireStatus(t, http.StatusCreated, status, body)
			auth := decode[authResponse](t, body)
			if auth.Token == "" || auth.User.Email != artistEmail {
				t.Fatalf("register response: %s", body)
			}
			artistToken = auth.Token
		})

		r.Step("03 login artist", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPost, "/v1/auth/login", "", jsonBody(t, map[string]string{
				"email": artistEmail, "password": password,
			}))
			requireStatus(t, http.StatusOK, status, body)
			auth := decode[authResponse](t, body)
			if auth.Token == "" {
				t.Fatalf("login response: %s", body)
			}
			artistToken = auth.Token
		})

		r.Step("04 current user", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodGet, "/v1/me", artistToken, nil)
			requireStatus(t, http.StatusOK, status, body)
			user := decode[userResponse](t, body)
			if user.Email != artistEmail {
				t.Fatalf("me email %q", user.Email)
			}
		})

		r.Step("05 init upload", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPost, "/v1/tracks/upload-init", artistToken, jsonBody(t, map[string]any{
				"title": title, "artist": artistName, "size": len(audio),
			}))
			requireStatus(t, http.StatusCreated, status, body)
			session := decode[uploadResponse](t, body)
			if session.Track.ID == "" || session.Track.Status != "pending" || session.UploadURL == "" {
				t.Fatalf("init response: %s", body)
			}
			if session.Track.SizeBytes != int64(len(audio)) {
				t.Fatalf("declared size %d", session.Track.SizeBytes)
			}
			trackID = session.Track.ID
			uploadURL = session.UploadURL
		})

		r.Step("06 put object", func(t *testing.T) {
			status, _, body := api.doURL(t, http.MethodPut, uploadURL, "", audio)
			if status != http.StatusOK && status != http.StatusNoContent {
				t.Fatalf("put object status %d body %s", status, body)
			}
		})

		r.Step("07 feed hides pending track", func(t *testing.T) {
			page := api.feed(t, artistName, "", "")
			if page.has(trackID) {
				t.Fatalf("pending track %s is in the public feed", trackID)
			}
		})

		r.Step("08 complete upload", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPost, "/v1/tracks/"+trackID+"/upload-complete", artistToken, nil)
			requireStatus(t, http.StatusOK, status, body)
			track := decode[trackResponse](t, body)
			if track.ID != trackID || track.Status != "ready" || track.Artist != artistName {
				t.Fatalf("complete response: %s", body)
			}
		})

		r.Step("09 feed shows ready track", func(t *testing.T) {
			page := api.feed(t, artistName, "", "")
			track, ok := page.find(trackID)
			if !ok || track.Status != "ready" {
				t.Fatalf("ready track missing from feed: %+v", page.Items)
			}
		})

		r.Step("10 register listener", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPost, "/v1/auth/register", "", jsonBody(t, map[string]string{
				"email": listenerEmail, "password": password, "name": "Demo Listener",
			}))
			requireStatus(t, http.StatusCreated, status, body)
			auth := decode[authResponse](t, body)
			if auth.Token == "" {
				t.Fatalf("listener register: %s", body)
			}
			listenerToken = auth.Token
		})

		r.Step("11 like without token", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPost, "/v1/tracks/"+trackID+"/like", "", nil)
			requireStatus(t, http.StatusUnauthorized, status, body)
			errBody := decode[errorResponse](t, body)
			if errBody.Code != "unauthorized" {
				t.Fatalf("like without token: %s", body)
			}
		})

		r.Step("12 like track", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPost, "/v1/tracks/"+trackID+"/like", listenerToken, nil)
			requireStatus(t, http.StatusNoContent, status, body)
		})

		r.Step("13 like track again", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodPost, "/v1/tracks/"+trackID+"/like", listenerToken, nil)
			requireStatus(t, http.StatusConflict, status, body)
			errBody := decode[errorResponse](t, body)
			if errBody.Code != "conflict" || errBody.Message != "track already liked" {
				t.Fatalf("repeat like: %s", body)
			}
		})

		r.Step("14 liked list", func(t *testing.T) {
			page := api.feed(t, "", "/v1/me/likes?limit=100", listenerToken)
			if _, ok := page.find(trackID); !ok {
				t.Fatalf("liked list: %+v", page.Items)
			}
		})

		r.Step("15 stream redirect", func(t *testing.T) {
			status, header, body := api.do(t, http.MethodGet, "/v1/tracks/"+trackID+"/stream", "", nil)
			requireStatus(t, http.StatusFound, status, body)
			streamURL = header.Get("Location")
			if streamURL == "" {
				t.Fatal("stream response has no Location")
			}
		})

		r.Step("16 download audio", func(t *testing.T) {
			status, _, body := api.doURL(t, http.MethodGet, streamURL, "", nil)
			requireStatus(t, http.StatusOK, status, body)
			if !bytes.Equal(body, audio) {
				t.Fatalf("downloaded %d bytes, uploaded %d", len(body), len(audio))
			}
		})

		r.Step("17 unlike", func(t *testing.T) {
			status, _, body := api.do(t, http.MethodDelete, "/v1/tracks/"+trackID+"/like", listenerToken, nil)
			requireStatus(t, http.StatusNoContent, status, body)
		})

		r.Step("18 liked list is empty", func(t *testing.T) {
			page := api.feed(t, "", "/v1/me/likes?limit=100", listenerToken)
			if page.has(trackID) || len(page.Items) != 0 {
				t.Fatalf("liked list after unlike: %+v", page.Items)
			}
		})
	})
}

type client struct {
	base string
	http *http.Client
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
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return res.StatusCode, res.Header, raw
}

func (c *client) feed(t *testing.T, artist, path, token string) pageResponse {
	t.Helper()
	if path == "" {
		path = "/v1/tracks?limit=100&artist=" + artist
	}
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
		t.Fatalf("status %d, want %d, body %s", got, want, body)
	}
}

type authResponse struct {
	Token string       `json:"token"`
	User  userResponse `json:"user"`
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

type pageResponse struct {
	Items []trackResponse `json:"items"`
	Limit int             `json:"limit"`
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
