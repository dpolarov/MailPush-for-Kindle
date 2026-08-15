.PHONY: test vet build-arm build-host docker clean

test:
	go test ./...
vet:
	go vet ./...
build-arm:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags="-s -w" -o dist/mailpush-armv7 ./cmd/mailpush
build-host:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/mailpush-linux-amd64 ./cmd/mailpush
docker:
	./scripts/build.sh
clean:
	rm -rf dist
