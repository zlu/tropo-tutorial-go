all: sendsms

sendsms: main.6
	6l -o sendsms new.6

main.6: new.go
	6g new.go