.PHONY: fmt test vet run

fmt:
	gofmt -w cmd internal

test:
	go test ./...

vet:
	go vet ./...

run:
	go run ./cmd/provisioner
