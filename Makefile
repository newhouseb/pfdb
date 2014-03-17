all:
	go build -o pfdb *.go

install: all
	cp pfdb /usr/local/bin/