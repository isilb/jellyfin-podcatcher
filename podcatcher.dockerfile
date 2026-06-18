# Step 1: Build the binary inside a Go container
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o jellyfin-podcatcher main.go

# Step 2: Run the binary inside a dead-simple, lightweight image
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/jellyfin-podcatcher .
CMD ["./jellyfin-podcatcher"]
