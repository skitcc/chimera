package auth

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"chimera/identity/internal/domain"
)

type SMTP struct {
	Addr, From, Username, Password string
	RequireTLS                     bool
}

func (s SMTP) SendCode(ctx context.Context, to, purpose, code string) error {
	sender, err := mail.ParseAddress(s.From)
	if err != nil {
		return domain.Invalid("invalid SMTP sender")
	}
	if strings.ContainsAny(to+s.From, "\r\n") {
		return domain.Invalid("invalid mail address")
	}
	// Bound delivery even when a peer stops responding halfway through SMTP.
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", s.Addr)
	if err != nil {
		return domain.Wrap(domain.CodeUnavailable, "email delivery unavailable", err)
	}
	defer conn.Close()
	deadline := time.Now().Add(5 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	conn.SetDeadline(deadline)
	host, _, err := net.SplitHostPort(s.Addr)
	if err != nil {
		return err
	}
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		return err
	}
	defer c.Close()
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err = c.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	} else if s.RequireTLS {
		return domain.Unavailable("SMTP TLS required")
	}
	if s.Username != "" {
		if err = c.Auth(smtp.PlainAuth("", s.Username, s.Password, host)); err != nil {
			return err
		}
	}
	if err = c.Mail(sender.Address); err != nil {
		return err
	}
	if err = c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "From: %s\r\nTo: %s\r\nSubject: Chimera %s code\r\nContent-Type: text/plain; charset=utf-8\r\n\r\nYour code: %s\r\nValid for 5 minutes.\r\n", s.From, to, purpose, code)
	if err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
