BINARY := skillmap

.PHONY: build build-windows build-mac build-linux run test tidy clean

build: build-windows

build-windows:
	GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o $(BINARY).exe .

build-mac:
	GOOS=darwin GOARCH=arm64 go build -o $(BINARY)_mac .

build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY)_linux .

run:
	go run .

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY).exe $(BINARY)_mac $(BINARY)_linux
	rm -f cache_*.json *_навыки.xlsx
