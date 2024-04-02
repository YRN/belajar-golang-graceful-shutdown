init:
	go mod init github.com/belajar-golang-graceful-shutdown
	go get -u github.com/gorilla/mux

run : 
	go run main.go
	