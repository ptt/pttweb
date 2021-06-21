.PHONY: all

all:
	protoc --go_out=. --go-grpc_out=. *.proto

clean:
	rm man.pb.go man_grpc.pb.go
