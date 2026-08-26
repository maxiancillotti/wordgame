FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY seed ./seed

RUN CGO_ENABLED=0 go build -o /out/wordgame ./cmd/wordgame

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /out/wordgame ./wordgame
COPY migrations ./migrations
COPY words.txt ./words.txt
COPY docker/config.toml ./config.toml
COPY docker/secrets.yaml ./secrets.yaml

ENV APP_ENV=local

EXPOSE 1337

ENTRYPOINT ["/app/wordgame"]
