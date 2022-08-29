all:
#	go mod init dnscache
#	go mod tidy 	
	go test -c -o test
	go run main.go
