# Stage 1: Build
FROM golang:1.22-alpine AS build

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /bridge ./cmd/bridge/

# Stage 2: Runtime
FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=build /bridge /usr/local/bin/bridge

ENTRYPOINT ["/usr/local/bin/bridge"]
CMD ["-config", "/config/config.yaml"]
