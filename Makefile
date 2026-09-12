.PHONY: tidy test vet lint smoke ci

tidy:
	go mod tidy
	git diff --exit-code go.mod go.sum

test:
	go test -race -shuffle=on -coverprofile=coverage.out ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

smoke:
	./scripts/smoke-e2e.sh

ci: tidy test vet lint smoke
