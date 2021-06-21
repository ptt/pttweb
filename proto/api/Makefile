.PHONY: all

all:
	protoc --go_out=. --go-grpc_out=. *.proto

clean:
	rm board.pb.go board_grpc.pb.go

