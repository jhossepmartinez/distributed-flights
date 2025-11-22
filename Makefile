.PHONY: proto build build-datanode build-broker run-node-1 run-node-2 run-node-3 run-broker clean

datanode_image_name = datanode
broker_image_name = broker
coordinator_image_name = coordinator

proto:
	protoc --go_out=. --go-grpc_out=. ./proto/flight.proto

build-datanode:
	docker build -f Dockerfile.datanode -t $(datanode_image_name) .

build-broker:
	docker build -f Dockerfile.broker -t $(broker_image_name) .

build-coordinator:
	docker build -f Dockerfile.coordinator -t $(coordinator_image_name) .

build:
	$(MAKE) build-datanode
	$(MAKE) build-broker
	$(MAKE) build-coordinator


run-node-1:
	docker run \
		--name datanode-1 \
		-p 50051:50051 \
		-e NODE_ID="A" \
		-e PORT="50051" \
		-e PEERS="192.168.1.6:50052,192.168.1.6:50053" \
		$(datanode_image_name)

run-node-2:
	docker run \
		--name datanode-2 \
		-p 50052:50051 \
		-e NODE_ID="B" \
		-e PORT="50051" \
		-e PEERS="192.168.1.6:50051,192.168.1.6:50053" \
		$(datanode_image_name)

run-node-3:
	docker run \
		--name datanode-3 \
		-p 50053:50051 \
		-e NODE_ID="C" \
		-e PORT="50051" \
		-e PEERS="192.168.1.6:50051,192.168.1.6:50052" \
		$(datanode_image_name)

run-broker:
	docker run \
		--name broker \
		-p 6000:6000 \
		-e DATA_NODES_ADDRESSES="192.168.1.6:50051,192.168.1.6:50052,192.168.1.6:50053" \
		$(broker_image_name)

run-coordinator:
	docker run \
		--name coordinator \
		-p 7000:7000 \
		-e PORT="7000" \
		-e DATA_NODES_ADDRESSES="A=192.168.1.6:50051,B=192.168.1.6:50052,C=192.168.1.6:50053" \
		$(coordinator_image_name)

clean:
	docker rm -f datanode-1 datanode-2 datanode-3 broker coordinator
