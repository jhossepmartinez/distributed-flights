.PHONY: proto build build-datanode build-broker run-node-1 run-node-2 run-node-3 run-broker clean

datanode_image_name = datanode
broker_image_name = broker
coordinator_image_name = coordinator
monotonic_client_name = monotonic-client
ryw_client_name = ryw-client

proto:
	protoc --go_out=. --go-grpc_out=. ./proto/flight.proto

build-datanode:
	docker build -f Dockerfile.datanode -t $(datanode_image_name) .

build-broker:
	docker build -f Dockerfile.broker -t $(broker_image_name) .

build-coordinator:
	docker build -f Dockerfile.coordinator -t $(coordinator_image_name) .

build-monotonic-client:
	docker build -f Dockerfile.monotonic-client -t $(monotonic_client_name) .

build-ryw:
	docker build -f Dockerfile.ryw -t $(ryw_client_name) .

build:
	$(MAKE) build-datanode
	$(MAKE) build-broker
	$(MAKE) build-coordinator
	$(MAKE) build-monotonic-client
	$(MAKE) build-ryw


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

run-monotonic-1:
	docker run \
		--name monotonic-1 \
		-p 8000:8000 \
		-e CLIENT_ID="client1" \
		-e COORDINATOR_ADDR="192.168.1.6:7000" \
		-e FLIGHT_ID="LA-500" \
		$(monotonic_client_name)

run-monotonic-2:
	docker run \
		--name monotonic-2 \
		-p 9000:9000 \
		-e CLIENT_ID="client2" \
		-e COORDINATOR_ADDR="192.168.1.6:7000" \
		-e FLIGHT_ID="AA-901" \
		$(monotonic_client_name)


run-ryw-1:
	docker run \
		--name ryw-1 \
		-p 10000:10000 \
		-e CLIENT_ID="ryw-1" \
		-e FLIGHT_ID="LA-500" \
		-e COORDINATOR_ADDR="192.168.1.6:7000" \
		$(ryw_client_name)

run-ryw-2:
	docker run \
		--name ryw-2 \
		-p 11000:11000 \
		-e CLIENT_ID="ryw-2" \
		-e FLIGHT_ID="LA-500" \
		-e COORDINATOR_ADDR="192.168.1.6:7000" \
		$(ryw_client_name)

run-ryw-3:
	docker run \
		--name ryw-3 \
		-p 12000:12000 \
		-e CLIENT_ID="ryw-3" \
		-e FLIGHT_ID="AA-901" \
		-e COORDINATOR_ADDR="192.168.1.6:7000" \
		$(ryw_client_name)



clean:
	docker rm -f datanode-1 datanode-2 datanode-3 broker coordinator monotonic-1 monotonic-2 ryw-1 ryw-2 ryw-3
