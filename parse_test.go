package main

import "testing"

func TestExtractAutodiscoverEmail(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{
			name: "soap wrapped",
			body: `<?xml version="1.0" encoding="utf-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
               xmlns:a="http://schemas.microsoft.com/exchange/autodiscover/outlook/requestschema/2006">
  <soap:Body>
    <a:Autodiscover>
      <a:Request>
        <a:EMailAddress>user@example.com</a:EMailAddress>
        <a:AcceptableResponseSchema>http://schemas.microsoft.com/exchange/autodiscover/outlook/responseschema/2006a</a:AcceptableResponseSchema>
      </a:Request>
    </a:Autodiscover>
  </soap:Body>
</soap:Envelope>`,
			want: "user@example.com",
		},
		{
			name: "namespaced autodiscover",
			body: `<?xml version="1.0" encoding="utf-8" ?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/outlook/requestschema/2006">
  <Request>
    <AcceptableResponseSchema>http://schemas.microsoft.com/exchange/autodiscover/outlook/responseschema/2006a</AcceptableResponseSchema>
    <EMailAddress>user@example.com</EMailAddress>
  </Request>
</Autodiscover>`,
			want: "user@example.com",
		},
		{
			name: "bare autodiscover EMailAddress",
			body: `<?xml version="1.0" encoding="utf-8"?>
<Autodiscover>
  <Request>
    <EMailAddress>user@example.com</EMailAddress>
  </Request>
</Autodiscover>`,
			want: "user@example.com",
		},
		{
			name: "bare autodiscover EmailAddress variant spelling",
			body: `<Autodiscover>
  <Request>
    <EmailAddress>user@example.com</EmailAddress>
  </Request>
</Autodiscover>`,
			want: "user@example.com",
		},
		{
			name: "request element only",
			body: `<Request>
  <EMailAddress>user@example.com</EMailAddress>
</Request>`,
			want: "user@example.com",
		},
		{
			name: "all caps tags",
			body: `<AUTODISCOVER><REQUEST><EMAILADDRESS>user@example.com</EMAILADDRESS></REQUEST></AUTODISCOVER>`,
			want: "user@example.com",
		},
		{
			name: "mixed case tags",
			body: `<Autodiscover><Request><EMailAddress>user@example.com</EMailAddress></Request></Autodiscover>`,
			want: "user@example.com",
		},
		{
			name: "whitespace around email trimmed",
			body: `<Autodiscover><Request><EMailAddress>  user@example.com  </EMailAddress></Request></Autodiscover>`,
			want: "user@example.com",
		},
		{
			name: "email outside request is ignored, inner one wins",
			body: `<Autodiscover>
  <EMailAddress>sneaky@example.com</EMailAddress>
  <Request>
    <EMailAddress>real@example.com</EMailAddress>
  </Request>
</Autodiscover>`,
			want: "real@example.com",
		},
		{
			name: "nested requests picks first email found",
			body: `<Autodiscover>
  <Request>
    <Request>
      <EMailAddress>inner@example.com</EMailAddress>
    </Request>
    <EMailAddress>outer@example.com</EMailAddress>
  </Request>
</Autodiscover>`,
			want: "inner@example.com",
		},

		{name: "empty body", body: "", wantErr: true},
		{name: "no email element", body: `<Autodiscover><Request></Request></Autodiscover>`, wantErr: true},
		{name: "email not inside request", body: `<Autodiscover><EMailAddress>lost@example.com</EMailAddress></Autodiscover>`, wantErr: true},
		{name: "malformed xml unclosed tag", body: `<Autodiscover><Request><EMailAddress>user@example.com`, wantErr: true},
		{name: "completely not xml", body: `this is not xml at all`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractAutodiscoverEmail([]byte(tt.body))
			if (err != nil) != tt.wantErr {
				t.Fatalf("extractAutodiscoverEmail() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("extractAutodiscoverEmail() = %q, want %q", got, tt.want)
			}
		})
	}
}
