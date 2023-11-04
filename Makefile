.PHONY: test
test:
	go test -race ./test

.PHONY: cover
cover:
	go test -race -coverpkg=./... -coverprofile coverage.out ./test
	go tool cover -html coverage.out -o coverage.html
	rm -f coverage.out
