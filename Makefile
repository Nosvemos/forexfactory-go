.PHONY: all build-dll build-so clean

all:
	@echo "Use 'make build-dll' for Windows or 'make build-so' for Linux/macOS."

build-dll:
	go build -buildmode=c-shared -o libforexfactory.dll pkg/bindings/bindings.go

build-so:
	go build -buildmode=c-shared -o libforexfactory.so pkg/bindings/bindings.go

clean:
	rm -f libforexfactory.dll libforexfactory.h libforexfactory.so
