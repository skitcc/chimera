//go:build e2e || authe2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var mailHTTP = &http.Client{Timeout: 5 * time.Second}

func mailURL() string {
	if s := os.Getenv("E2E_MAILPIT_URL"); s != "" {
		return strings.TrimRight(s, "/")
	}
	return "http://mailpit:8025"
}

type mailSummary struct {
	ID      string
	Subject string
}

func findMail(email string) ([]mailSummary, error) {
	res, err := mailHTTP.Get(mailURL() + "/api/v1/search?query=" + url.QueryEscape("to:"+email))
	if err != nil {
		return nil, fmt.Errorf("mail search unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("mail search status %d", res.StatusCode)
	}
	var list struct{ Messages []mailSummary }
	if err = json.NewDecoder(res.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("invalid mail search response")
	}
	return list.Messages, nil
}
func emailCode(email, purpose string) (string, error) {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		list, err := findMail(email)
		if err != nil {
			return "", err
		}
		for _, m := range list {
			if m.Subject != "Chimera "+purpose+" code" {
				continue
			}
			res, err := mailHTTP.Get(mailURL() + "/api/v1/message/" + url.PathEscape(m.ID))
			if err != nil {
				return "", fmt.Errorf("mail fetch unavailable")
			}
			var msg struct{ Text string }
			err = json.NewDecoder(res.Body).Decode(&msg)
			res.Body.Close()
			if err != nil {
				return "", fmt.Errorf("invalid mail response")
			}
			match := regexp.MustCompile(`Your code: ([0-9]{6})`).FindStringSubmatch(msg.Text)
			if len(match) == 2 {
				return match[1], nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return "", fmt.Errorf("email code not delivered")
}
func deleteMail(email string) error {
	list, err := findMail(email)
	if err != nil {
		return err
	}
	for _, m := range list {
		req, _ := http.NewRequest(http.MethodDelete, mailURL()+"/api/v1/messages", strings.NewReader(`{"IDs":["`+m.ID+`"]}`))
		req.Header.Set("Content-Type", "application/json")
		res, e := mailHTTP.Do(req)
		if e != nil {
			return fmt.Errorf("mail cleanup unavailable")
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		if res.StatusCode != 200 {
			return fmt.Errorf("mail cleanup status %d", res.StatusCode)
		}
	}
	return nil
}
