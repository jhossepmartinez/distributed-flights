package main

import (
	pb "distributed-flights/proto"
	"time"
)

type Session struct {
	nodeAddress string
	timeStamp   time.Time
}

type Coordinator struct {
	pb.UnimplementedCoordinatorServer
	dataNodes []pb.DataNodeServiceClient
	sessions  map[string]Session // clientId: Session
}
