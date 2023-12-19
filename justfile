default:
  @just --list

test:
    go test ./...

build:
    go build

tidy:
    go mod tidy

run: build
    ./nats-load-traffic -c examples/config/config.yaml

up:
    cd examples && docker compose -f docker-compose-dev.yaml up -d && docker compose -f docker-compose-dev.yaml logs --follow

upb:
    cd examples && docker compose -f docker-compose-dev.yaml up -d --build --force-recreate && docker compose logs --follow

down:
    cd examples && docker compose -f docker-compose-dev.yaml down

infra:
    cd examples && docker compose -f docker-compose-dev.yaml  up -d prometheus nats && docker compose -f docker-compose-dev.yaml logs --follow
