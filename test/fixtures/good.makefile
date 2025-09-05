build: main.go
	go build -o app main.go

run: build
	./app
