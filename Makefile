.PHONY: tidy test vet lint ci

tidy:
	go mod tidy
	git diff --exit-code go.mod go.sum

test:
	go test -race -shuffle=on -coverprofile=coverage.out ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

ci: tidy test vet lint
