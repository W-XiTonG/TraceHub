# syntax=docker/dockerfile:1
FROM node:22-alpine AS web
WORKDIR /src/web/frontend
COPY web/frontend/package.json web/frontend/package-lock.json* ./
RUN npm ci || npm install
COPY web/frontend/ .
RUN npm run build

FROM golang:1.25-alpine AS api
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/frontend/dist ./web/frontend/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tracehub ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=api /tracehub /app/tracehub
COPY configs/config.example.yaml /app/config.example.yaml
EXPOSE 8080
ENTRYPOINT ["/app/tracehub"]
CMD ["-config", "/app/config.yaml"]
