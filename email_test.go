package main

import "testing"

func TestParseEmail(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantUser   string
		wantDomain string
		wantErr    bool
	}{
		{"plain", "user@example.com", "user", "example.com", false},
		{"leading and trailing spaces", "  user@example.com  ", "user", "example.com", false},
		{"plus addressing", "user+tag@example.com", "user+tag", "example.com", false},
		{"subdomain", "user@mail.example.com", "user", "mail.example.com", false},
		{"multiple ats uses last", "user@host@example.com", "user@host", "example.com", false},
		{"unicode local part", "üser@example.com", "üser", "example.com", false},
		{"dots everywhere", "first.last@sub.example.co.uk", "first.last", "sub.example.co.uk", false},

		{"empty", "", "", "", true},
		{"whitespace only", "   ", "", "", true},
		{"no at sign", "useratexample.com", "", "", true},
		{"at sign only", "@", "", "", true},
		{"at sign at start", "@example.com", "", "", true},
		{"at sign at end", "user@", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEmail(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseEmail(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got.User != tt.wantUser || got.Domain != tt.wantDomain {
				t.Errorf("ParseEmail(%q) = {%q, %q}, want {%q, %q}",
					tt.input, got.User, got.Domain, tt.wantUser, tt.wantDomain)
			}
			if got.String() != tt.wantUser+"@"+tt.wantDomain {
				t.Errorf("Email.String() = %q, want %q", got.String(), tt.wantUser+"@"+tt.wantDomain)
			}
		})
	}
}
