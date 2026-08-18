.PHONY: all build build-dll build-so test clean

all: build

build:
	go build -o forexcalendar ./cmd/forexcalendar-go
	go build -o fc-notifier ./cmd/fc-notifier

build-dll:
	go build -buildmode=c-shared -o libforexcalendar.dll pkg/bindings/bindings.go

build-so:
	go build -buildmode=c-shared -o libforexcalendar.so pkg/bindings/bindings.go

test:
	go test -v ./...

clean:
	rm -f forexcalendar forexcalendar.exe fc-notifier fc-notifier.exe libforexcalendar.dll libforexcalendar.h libforexcalendar.so
