export PATH := /home/isucon/.local/go/bin:${PATH}
all: app

.PHONY: app
app: *.go go.mod go.sum
	go build -o app
