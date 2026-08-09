FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY bot/ ./bot/
COPY go.mod ./

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bot-app ./bot/cmd

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/bot-app /app/bot

ENV TZ=UTC

ENTRYPOINT ["/app/bot"]
