# Build stage
FROM golang:1.24-alpine AS build
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Static binary, strip symbols for smaller size
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /flux-server ./cmd/server

# Distroless: no shell, CA certs, non-root
FROM gcr.io/distroless/static:nonroot
COPY --from=build /flux-server /flux-server
EXPOSE 8080
ENTRYPOINT ["/flux-server"]
