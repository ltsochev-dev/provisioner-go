.PHONY: fmt test vet run

fmt:
	gofmt -w cmd internal

test:
	go test ./...

vet:
	go vet ./...

run:
	docker compose up -d
	go run ./cmd/provisioner
