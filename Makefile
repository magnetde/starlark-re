.PHONY: test
test:
	go test -race ./...

.PHONY: cover
cover:
	go test -race -coverpkg=./... -coverprofile coverage.out ./...
	go tool cover -html coverage.out -o coverage.html
	rm -f coverage.out
