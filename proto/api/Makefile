.PHONY: all

all:
	protoc --go_out=. --go-grpc_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative \
		--go_opt=Mboard.proto=github.com/ptt/pttweb/proto/api board.proto \
		--go-grpc_opt=Mboard.proto=github.com/ptt/pttweb/proto/api board.proto
