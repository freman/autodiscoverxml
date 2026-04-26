FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /autodiscover .

FROM scratch

COPY --from=builder /autodiscover /autodiscover

EXPOSE 8080

ENTRYPOINT ["/autodiscover"]
CMD ["--addr", ":8080", "--templates", "/templates"]
