package main

import (
	"fmt"
	"strings"
)

type Email struct {
	User   string
	Domain string
}

func (e *Email) String() string {
	if e == nil {
		return ""
	}

	return e.User + "@" + e.Domain
}

func ParseEmail(s string) (Email, error) {
	s = strings.TrimSpace(s)
	at := strings.LastIndex(s, "@")
	if at < 1 || at == len(s)-1 {
		return Email{}, fmt.Errorf("invalid email address: %q", s)
	}

	return Email{
		User:   s[:at],
		Domain: s[at+1:],
	}, nil
}
