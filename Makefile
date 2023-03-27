all: proto-go

proto-go:
	! test -d github.com/openconfig || rm -rf github.com/openconfig
	mkdir -p github.com/openconfig
	protoc \
		--proto_path=./proto \
		--go_out=. \
		--go-grpc_out=. \
		./proto/*.proto

run:
	cd server/cmd && \
		YANGPATH=.:../../../restconf/yang go run main.go
