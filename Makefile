.PHONY: proto

image_name = datanode

proto:
	protoc --go_out=. --go-grpc_out=. ./proto/flight.proto

build:
	docker build -t $(image_name) .

run-node-1:
	docker run \
		--name datanode-1 \
		-p 50051:50051 \
		-e NODE_ID="A" \
		-e PORT="50051" \
		-e PEERS="192.168.1.6:50052,192.168.1.6:50053" \
		$(image_name)

run-node-2:
	docker run \
		--name datanode-2 \
		-p 50052:50051 \
		-e NODE_ID="B" \
		-e PORT="50051" \
		-e PEERS="192.168.1.6:50051,192.168.1.6:50053" \
		$(image_name)

run-node-3:
	docker run \
		--name datanode-3 \
		-p 50053:50051 \
		-e NODE_ID="C" \
		-e PORT="50051" \
		-e PEERS="192.168.1.6:50051,192.168.1.6:50052" \
		$(image_name)

clean:
	docker rm -f datanode-1 datanode-2 datanode-3
