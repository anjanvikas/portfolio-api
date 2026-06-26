# syntax=docker/dockerfile:1

# --- build stage ---
FROM golang:1.26-alpine AS build
WORKDIR /src

# fogleman/gg needs cgo-free freetype, but build deps below cover anything
# that pulls in cgo transitively (sqlite-free here, kept minimal on purpose).
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

# --- run stage ---
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/api /api

# Cloud Run injects PORT; config.go defaults to 8080 if unset.
EXPOSE 8080
ENTRYPOINT ["/api"]
