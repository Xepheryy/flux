.PHONY: test cover cover-check vet

test:
	go test ./... -race

cover:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -func=coverage.out

cover-check: cover
	@total=$$(go tool cover -func=coverage.out | grep '^total:' | awk '{gsub(/%/,""); print $$3}'); \
	echo "Coverage: $$total%"; \
	if [ "$$(echo "$$total < 79" | bc)" -eq 1 ]; then echo "Coverage below 79%"; exit 1; fi

vet:
	go vet ./...
