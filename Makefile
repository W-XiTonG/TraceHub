.PHONY: ent gen run fe build docker-up

ent:
	go run entgo.io/ent/cmd/ent@v0.14.6 generate ./internal/ent/schema

run:
	go run ./cmd/server -config configs/config.yaml

fe:
	cd web/frontend && npm run dev

build:
	cd web/frontend && npm ci && npm run build
	go build -o bin/tracehub ./cmd/server

docker-up:
	docker compose -f deploy/docker-compose.local.yml up --build
