package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// extractAutodiscoverEmail parses an Outlook autodiscover POST body and returns
// the email address. Handles SOAP-wrapped, namespaced, and bare XML variants.
// All element names are compared case-insensitively by lowercasing before matching,
// which also collapses EMailAddress and EmailAddress to the same token.
func extractAutodiscoverEmail(body []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.Strict = false

	inRequest := 0
	inEmail := false
	var buf strings.Builder

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("xml parse: %w", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch strings.ToLower(t.Name.Local) {
			case "request":
				inRequest++
			case "emailaddress":
				if inRequest > 0 {
					inEmail = true
				}
			}
		case xml.EndElement:
			switch strings.ToLower(t.Name.Local) {
			case "request":
				if inRequest > 0 {
					inRequest--
				}
			case "emailaddress":
				inEmail = false
				if result := strings.TrimSpace(buf.String()); result != "" {
					return result, nil
				}
				buf.Reset()
			}
		case xml.CharData:
			if inEmail {
				buf.Write(t)
			}
		}
	}

	result := strings.TrimSpace(buf.String())
	if result == "" {
		return "", fmt.Errorf("no email address found in request")
	}

	return result, nil
}
