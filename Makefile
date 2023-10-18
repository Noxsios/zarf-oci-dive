build: *.go
	CGO_ENABLED=0 go build -o build/dive .

clean:
	rm -rf build
