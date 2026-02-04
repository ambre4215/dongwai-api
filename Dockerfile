FROM golang:1.22-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /out/server ./cmd

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

ENV PORT=8080

COPY --from=builder /out/server /app/server

EXPOSE 8080

ENTRYPOINT ["/app/server"]
