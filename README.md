# autodiscover

A Go service that handles Outlook Autodiscover and Mozilla Autoconfig requests,
returning per-domain XML responses rendered from Go templates.

It accepts every reasonable variant of the autodiscover request format, ignores
the parts it doesn't care about, and serves whatever template you've put in
place for that domain. If you haven't, it returns a 418.

## Build

```
go build .
```

## Usage

```
autodiscover --addr :8080 --templates /etc/autodiscover/templates
```

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | `:8080` | Listen address |
| `--templates` | *(required)* | Path to template root directory |

TLS termination is left to your reverse proxy.

## Routes

All paths are matched case-insensitively.

### Outlook Autodiscover

```
POST /autodiscover/autodiscover.xml
POST /autodiscover.xml
```

Body: XML. The service accepts every variant it has encountered in the wild:

- SOAP-wrapped (`soap:Envelope > ... > Request > EMailAddress`)
- Namespaced (`Autodiscover[xmlns=...] > Request > EMailAddress`)
- Bare (`Autodiscover > Request > EMailAddress`)
- Spelling variant (`EmailAddress` instead of `EMailAddress`)
- Minimal (`Request > EMailAddress` with no outer element)
- All of the above in any combination of upper, lower, or mixed case

If the body is missing, unparseable, or doesn't contain an email address in a
`Request` element: `400`. If your client can't handle a `400`, that's on the
client.

`AcceptableResponseSchema` is read and promptly ignored.

### Mozilla Autoconfig

```
GET /.well-known/autoconfig/mail/config-v1.1.xml?emailaddress=user@example.com
GET /mail/config-v1.1.xml?emailaddress=user@example.com
```

Email is a query parameter. If it's missing or not a valid address: `400`.

### Health Check

```
GET /health
```

Returns `200 ok`. Suitable for load balancer probes.

## Templates

Templates live under `--templates` organized by the `Host` header (port
stripped). Given a request to `autodiscover.example.com`:

```
{--templates}/
  autodiscover.example.com/
    autodiscover.xml      <- Outlook autodiscover response
    config-v1.1.xml       <- Mozilla autoconfig response
    config.json           <- optional domain config (see below)
```

Templates are rendered with Go's `text/template`. The static XML structure of
the template is output as-is. Server names, ports, and any other values can be
hardcoded directly in the template XML - no `config.json` required. The
variables below are available if you want them; ignore them if you don't.

### Template Variables

`.Email` is always available when a valid address was provided. `.Config` is
available when a `config.json` exists in the domain directory - otherwise it is
`nil`. Both are entirely optional; a template with nothing but hardcoded XML
and `{{.Email}}` for the login name will probably work fine, depending on mail client.

| Expression | Example output |
|------------|----------------|
| `{{.Email}}` | `user@example.com` |
| `{{.Email.User}}` | `user` |
| `{{.Email.Domain}}` | `example.com` |
| `{{.Config.Domain}}` | `example.com` |
| `{{.Config.DisplayName}}` | `Example Mail` |
| `{{.Config.ShortName}}` | `Example` |
| `{{.Config.IMAP.Host}}` | `mail.example.com` |
| `{{.Config.IMAP.Port}}` | `993` |
| `{{.Config.IMAP.SocketType}}` | `SSL` |
| `{{.Config.IMAP.Auth}}` | `password-cleartext` |
| `{{.Config.SMTP.Host}}` | `mail.example.com` |
| `{{.Config.SMTP.Port}}` | `465` |
| `{{.Config.SMTP.SocketType}}` | `SSL` |
| `{{.Config.SMTP.Auth}}` | `password-cleartext` |

### Domain Config

An optional `config.json` in each domain directory lets you define server
settings once and reference them from both templates, rather than hardcoding
the same values in two files:

```json
{
  "domain": "example.com",
  "display_name": "Example Mail",
  "short_name": "Example",
  "imap": {
    "host": "mail.example.com",
    "port": 993,
    "socket_type": "SSL",
    "auth": "password-cleartext"
  },
  "smtp": {
    "host": "mail.example.com",
    "port": 465,
    "socket_type": "SSL",
    "auth": "password-cleartext"
  }
}
```

`socket_type` is a string: `SSL`, `STARTTLS`, or `plain`. If the file exists
but is not valid JSON, the service returns `500`.

### Error Responses

| Condition | Status |
|-----------|--------|
| No template found for domain | `418 I'm a Teapot` |
| Malformed or missing request data | `400 Bad Request` |
| Invalid Host header | `400 Bad Request` |
| Body exceeds 16 KB | `413 Request Entity Too Large` |
| Template execution failure | `500 Internal Server Error` |

## Example Templates

See [`templates/example.com/`](templates/example.com/) for working examples
of both the Outlook and Mozilla response formats.

Copy that directory, rename it to match your domain, and adjust the server
names and ports. The `templates/example.com/` directory is the only one
committed - your actual domain directories are gitignored by default.

### Outlook (`autodiscover.xml`)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<Autodiscover xmlns="http://schemas.microsoft.com/exchange/autodiscover/responseschema/2006">
  <Response xmlns="http://schemas.microsoft.com/exchange/autodiscover/outlook/responseschema/2006a">
    <User>
      <DisplayName>{{.Config.DisplayName}} - {{.Email.User}}</DisplayName>
    </User>
    <Account>
      <AccountType>email</AccountType>
      <Action>settings</Action>
      <Protocol>
        <Type>IMAP</Type>
        <Server>{{.Config.IMAP.Host}}</Server>
        <Port>{{.Config.IMAP.Port}}</Port>
        <DomainRequired>off</DomainRequired>
        <SPA>off</SPA>
        <SSL>{{if eq .Config.IMAP.SocketType "SSL"}}on{{else}}off{{end}}</SSL>
        <AuthRequired>on</AuthRequired>
        <LoginName>{{.Email}}</LoginName>
      </Protocol>
      <Protocol>
        <Type>SMTP</Type>
        <Server>{{.Config.SMTP.Host}}</Server>
        <Port>{{.Config.SMTP.Port}}</Port>
        <DomainRequired>off</DomainRequired>
        <SPA>off</SPA>
        <SSL>{{if eq .Config.SMTP.SocketType "SSL"}}on{{else}}off{{end}}</SSL>
        <AuthRequired>on</AuthRequired>
        <LoginName>{{.Email}}</LoginName>
      </Protocol>
    </Account>
  </Response>
</Autodiscover>
```

### Mozilla (`config-v1.1.xml`)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<clientConfig version="1.1">
  <emailProvider id="{{if .Email}}{{.Email.Domain}}{{else}}{{.Config.Domain}}{{end}}">
    <domain>{{if .Email}}{{.Email.Domain}}{{else}}{{.Config.Domain}}{{end}}</domain>
    <displayName>{{.Config.DisplayName}}</displayName>
    <displayShortName>{{.Config.ShortName}}</displayShortName>
    <incomingServer type="imap">
      <hostname>{{.Config.IMAP.Host}}</hostname>
      <port>{{.Config.IMAP.Port}}</port>
      <socketType>{{.Config.IMAP.SocketType}}</socketType>
      <authentication>{{.Config.IMAP.Auth}}</authentication>
      <username>{{if .Email}}{{.Email}}{{else}}%EMAILADDRESS%{{end}}</username>
    </incomingServer>
    <outgoingServer type="smtp">
      <hostname>{{.Config.SMTP.Host}}</hostname>
      <port>{{.Config.SMTP.Port}}</port>
      <socketType>{{.Config.SMTP.SocketType}}</socketType>
      <authentication>{{.Config.SMTP.Auth}}</authentication>
      <username>{{if .Email}}{{.Email}}{{else}}%EMAILADDRESS%{{end}}</username>
    </outgoingServer>
  </emailProvider>
</clientConfig>
```

## Domain Mapping

The `Host` header (port stripped) is used as the directory name. No subdomain
stripping is performed - `autodiscover.example.com` and `example.com` are
distinct directories. If you run the service at `autodiscover.example.com` but
want templates shared with `example.com`, a symlink works fine.

Host headers containing anything other than letters, digits, hyphens, and dots
are rejected outright.

## Reverse Proxy

The critical thing for all of these is that the original `Host` header reaches
the service - that's how it knows which template directory to use.

### Caddy

Caddy handles TLS automatically and passes `Host` through by default. Easiest option.

```
autodiscover.example.com {
    reverse_proxy localhost:8080
}
```

If you run multiple domains from one instance:

```
autodiscover.example.com autoconfig.example.com autodiscover.otherdomain.net {
    reverse_proxy localhost:8080
}
```

### nginx

```nginx
server {
    listen 443 ssl;
    server_name autodiscover.example.com;

    ssl_certificate     /etc/ssl/certs/example.com.pem;
    ssl_certificate_key /etc/ssl/private/example.com.key;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
    }
}
```

`proxy_set_header Host $host` is the important bit - without it nginx sends its
own `server_name` and the template lookup will fail.

### Apache

Requires `mod_proxy` and `mod_proxy_http`.

```apache
<VirtualHost *:443>
    ServerName autodiscover.example.com

    SSLEngine on
    SSLCertificateFile    /etc/ssl/certs/example.com.pem
    SSLCertificateKeyFile /etc/ssl/private/example.com.key

    ProxyPreserveHost On
    ProxyPass        / http://localhost:8080/
    ProxyPassReverse / http://localhost:8080/
</VirtualHost>
```

`ProxyPreserveHost On` is the important bit - without it Apache rewrites `Host`
to `localhost:8080` and the template lookup will fail.

## Docker

### Compose

The included `compose.yaml` mounts `./templates` into the container and listens
on port 8080:

```
docker compose up -d
```

Put your domain template directories under `./templates/` on the host and
they'll be live immediately - no restart needed since templates are read on
every request.

### Building manually

```
docker build -t autodiscover .
docker run -p 8080:8080 -v ./templates:/templates:ro autodiscover
```

### Multi-platform builds

```
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t yourrepo/autodiscover:latest \
  --push .
```

The builder stage runs natively on your machine regardless of target platform -
cross-compilation is handled by the Go toolchain, not emulation, so it stays
fast.

## Notes

- Logs via `log/slog` in structured text format
- Flags and templates are all you need - `config.json` is optional
