.PHONY: all build build-dll build-so test clean

all: build

build:
	go build -o tvcalendar ./cmd/tvcalendar
	go build -o tv-notifier ./cmd/tv-notifier

build-dll:
	go build -buildmode=c-shared -o libtvcalendar.dll pkg/bindings/bindings.go

build-so:
	go build -buildmode=c-shared -o libtvcalendar.so pkg/bindings/bindings.go

test:
	go test -v ./...

clean:
	rm -f tvcalendar tvcalendar.exe tv-notifier tv-notifier.exe libtvcalendar.dll libtvcalendar.h libtvcalendar.so
