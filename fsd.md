# Functional Specification: autodiscover

A Go service that handles Outlook Autodiscover and Mozilla Autoconfig requests,
returning per-domain XML responses rendered from Go `html/template` templates.

---

## CLI

```
autodiscover [flags]
  --addr       Listen address (default: :8080)
  --templates  Path to template root directory (required)
```

---

## Routes

All path matching is case-insensitive.

### Outlook Autodiscover (POST)

| Path | Notes |
|------|-------|
| `/autodiscover/autodiscover.xml` | Standard |
| `/autodiscover.xml` | Shorthand, some clients |

Method: `POST`, body: XML. Any other method returns `405`.

### Mozilla Autoconfig (GET)

| Path | Notes |
|------|-------|
| `/.well-known/autoconfig/mail/config-v1.1.xml` | Preferred |
| `/mail/config-v1.1.xml` | Fallback some clients use |

Method: `GET`, email passed as `?emailaddress=user@example.com`.
Any other method returns `405`.

---

## Host Header Processing

1. Strip port from `Host` header using `net.SplitHostPort` (handle the no-port case).
2. Use the resulting hostname as the domain key for template lookup.
   - e.g. `autodiscover.fremnet.net:8080` -> `autodiscover.fremnet.net`
3. Reject any derived hostname containing `/`, `\`, or `.` sequences that escape
   the template root (path traversal check). If validation fails: `400`.

The operator maps hostnames to template directories directly. If the service runs
at `autodiscover.fremnet.net` but templates live under `fremnet.net/`, a symlink
or duplicate directory is the operator's responsibility.

---

## Template File Layout

```
{--templates}/
  example.com/
    autodiscover.xml      <- Outlook autodiscover response
    config-v1.1.xml       <- Mozilla autoconfig response
  fremnet.net/            <- not committed, operator-provided
    autodiscover.xml
    config-v1.1.xml
```

If the resolved template file does not exist: `418`.

---

## Outlook Autodiscover - Request Parsing

The POST body is XML. All tag names are lowercased before matching to handle
client variance.

Extract the email address from whichever structure is present, trying in order:

1. **SOAP envelope**
   `soap:Envelope > soap:Body > a:Autodiscover > a:Request > a:EMailAddress`

2. **Namespaced Autodiscover**
   `Autodiscover[xmlns=...requestschema...] > Request > EMailAddress`

3. **Bare Autodiscover**
   `Autodiscover > Request > EMailAddress`

4. **Bare Autodiscover (variant spelling)**
   `Autodiscover > Request > EmailAddress`

5. **Request only**
   `Request > EMailAddress`  /  `Request > EmailAddress`

If the body is empty, unparseable, or no email is found: `400`.

`AcceptableResponseSchema` is read but intentionally ignored. We serve whatever
template we have.

---

## Mozilla Autoconfig - Request Parsing

Email comes from the `emailaddress` query parameter (case-insensitive key).
If absent or invalid: `400`.

---

## Email Object

Parsed from the extracted address string. Must contain `@` or the request is `400`.

Available in templates as:

| Expression | Value |
|------------|-------|
| `{{.Email}}` | `user@example.com` |
| `{{.Email.User}}` | `user` |
| `{{.Email.Domain}}` | `example.com` |

`{{.Email}}` produces the full address via the type's `String()` method.

---

## Template Rendering

Templates are rendered with `html/template` (auto-escapes values inside `{{}}`).
The response `Content-Type` is `application/xml`.

Template data passed to `Execute`:

```go
struct {
    Email EmailAddress
}
```

If template execution fails: `500`, log the error via `slog`.

---

## Error Handling

| Condition | Response |
|-----------|----------|
| Wrong HTTP method | `405 Method Not Allowed` |
| Empty body / unparseable XML | `400 Bad Request` |
| No email found in request | `400 Bad Request` |
| Invalid/malicious Host header | `400 Bad Request` |
| Template file not found | `418 I'm a Teapot` |
| Template execution failure | `500 Internal Server Error` |

Error bodies are plain text. Client error handling beyond receiving the status
code is the client's problem.

---

## Logging

Use `log/slog` structured logging throughout.

- Each request: method, path, host, resolved domain, status code.
- Template not found: domain, template path.
- Parse failures: reason (without echoing raw body content).
- Template execution errors: error string.

---

## Example Templates

### `example.com/autodiscover.xml`

```xml
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/outlook/responseschema/2006a">
    <User>
      <DisplayName>{{.Email}}</DisplayName>
    </User>
    <Account>
      <AccountType>email</AccountType>
      <Action>settings</Action>
      <Protocol>
        <Type>IMAP</Type>
        <Server>mail.example.com</Server>
        <Port>993</Port>
        <DomainRequired>off</DomainRequired>
        <SPA>off</SPA>
        <SSL>on</SSL>
        <AuthRequired>on</AuthRequired>
        <LoginName>{{.Email}}</LoginName>
      </Protocol>
      <Protocol>
        <Type>SMTP</Type>
        <Server>mail.example.com</Server>
        <Port>465</Port>
        <DomainRequired>off</DomainRequired>
        <SPA>off</SPA>
        <SSL>on</SSL>
        <AuthRequired>on</AuthRequired>
        <LoginName>{{.Email}}</LoginName>
      </Protocol>
    </Account>
  </Response>
</Autodiscover>
```

### `example.com/config-v1.1.xml`

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!--
  Available Go template variables: {{.Email}} {{.Email.User}} {{.Email.Domain}}
  Mozilla also defines its own substitution tokens that some clients expand
  client-side after download: %EMAILADDRESS% %EMAILLOCALPART% %EMAILDOMAIN%
  Pick one system and use it consistently - do not mix them in the same file.
  This example uses Go template variables so the service handles substitution.
-->
<clientConfig version="1.1">
  <emailProvider id="example.com">
    <domain>example.com</domain>
    <displayName>Example Mail</displayName>
    <displayShortName>Example</displayShortName>
    <incomingServer type="imap">
      <hostname>mail.example.com</hostname>
      <port>993</port>
      <socketType>SSL</socketType>
      <authentication>password-cleartext</authentication>
      <username>{{.Email}}</username>
    </incomingServer>
    <outgoingServer type="smtp">
      <hostname>mail.example.com</hostname>
      <port>465</port>
      <socketType>SSL</socketType>
      <authentication>password-cleartext</authentication>
      <username>{{.Email}}</username>
    </outgoingServer>
  </emailProvider>
</clientConfig>
```

---

## Out of Scope

- TLS termination (run behind a reverse proxy).
- Apple mobileconfig.
- SRV/DNS record handling.
- Graceful shutdown (can be added later; not needed for initial version).
