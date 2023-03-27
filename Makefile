all: proto-go

run:
	cd server/cmd && \
		YANGPATH=.:../../../restconf/yang go run main.go
