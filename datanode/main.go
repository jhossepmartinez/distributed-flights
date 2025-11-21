package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	pb "distributed-flights/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Clock = map[string]int32

type Node struct {
	pb.UnimplementedDataNodeServiceServer
	id      string
	storage map[string]*pb.FlightState
	peers   []string
	mu      sync.Mutex
}

func CompareClocks(c1, c2 Clock) string {
	var greater, less bool
	keys := make(map[string]bool)

	for k := range c1 {
		keys[k] = true
	}
	for k := range c2 {
		keys[k] = true
	}

	for k := range keys {
		v1 := c1[k]
		v2 := c2[k]

		if v1 > v2 {
			greater = true
		}
		if v2 > v1 {
			less = true
		}
	}

	if greater && less {
		return "concurrent"
	}
	if greater {
		return "after"
	}
	if less {
		return "before"
	}
	return "equal"

}

func MergeClocks(c1, c2 Clock) Clock {
	merged := make(Clock)
	for k, v1 := range c1 {
		merged[k] = v1
	}

	for k, v2 := range c2 {
		if v2 > merged[k] {
			merged[k] = v2
		}
	}

	return merged
}

func (n *Node) Gossip(ctx context.Context, req *pb.GossipRequest) (*pb.GossipResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	localState, exists := n.storage[req.FlightId]

	if !exists {
		n.storage[req.FlightId] = req.State
		log.Printf("📥 GOSSIP RECIBIDO de %s: Nuevo dato aceptado -> %s", req.SenderId, req.FlightId)
		return &pb.GossipResponse{Success: true}, nil
	}

	relation := CompareClocks(req.State.Clock.Versions, localState.Clock.Versions)
	mergedClock := &pb.VectorClock{Versions: MergeClocks(req.State.Clock.Versions, localState.Clock.Versions)}

	switch relation {
	case "after":
		n.storage[req.FlightId] = req.State
		log.Printf("📥 GOSSIP UPDATE de %s: Actualizado (Newer)", req.SenderId)
	case "concurrent":
		resolvedStatus := localState.Status
		if req.State.Status > localState.Status {
			resolvedStatus = req.State.Status
		}
		resolvedGate := localState.Gate
		if req.State.Gate > localState.Gate {
			resolvedGate = req.State.Gate
		}

		// Si uno de los dos tiene puerta vacía y el otro no, nos quedamos con la que tiene dato
		if localState.Gate != "" && req.State.Gate == "" {
			resolvedGate = localState.Gate
		}
		if req.State.Gate != "" && localState.Gate == "" {
			resolvedGate = req.State.Gate
		}

		n.storage[req.FlightId] = &pb.FlightState{
			Status: resolvedStatus, Gate: resolvedGate,
			Clock: mergedClock,
		}
		log.Printf("⚔️ CONFLICTO con %s: Resuelto a '%s'", req.SenderId, resolvedStatus)
	default:
		n.storage[req.FlightId].Clock = mergedClock
		log.Printf("📥 GOSSIP RECIBIDO de %s: Ignorado (Older or Equal)", req.SenderId)
	}
	return &pb.GossipResponse{Success: true}, nil
}

func (n *Node) showPeriodicState() {
	go func() {
		for {
			time.Sleep(10 * time.Second)
			n.mu.Lock()
			log.Printf("📊 Estado actual en nodo %s (%d vuelos):", n.id, len(n.storage))
			for fid, st := range n.storage {
				fmt.Printf("   - %s: [Estado: %s] [Puerta: %s] Reloj: %v\n", fid, st.Status, st.Gate, st.Clock.Versions)
			}
			n.mu.Unlock()
		}

	}()
}

func (n *Node) sendGossipTo(address string) {
	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("❌ No se pudo conectar a %s: %v", address, err)
	}
	defer conn.Close()
	client := pb.NewDataNodeServiceClient(conn)

	n.mu.Lock()
	storageCopy := make(map[string]*pb.FlightState)
	for k, v := range n.storage {
		storageCopy[k] = v
	}
	n.mu.Unlock()

	for flightId, state := range storageCopy {
		_, err := client.Gossip(context.Background(), &pb.GossipRequest{
			FlightId: flightId,
			State:    state,
			SenderId: n.id,
		})
		if err != nil {
			log.Printf("📤 GOSSIP ENVIADO a %s: Vuelo %s", address, flightId)
		}
	}
}

func (n *Node) StartGossipLoop() {
	go func() {
		for {
			time.Sleep(7 * time.Second)

			if len(n.peers) == 0 {
				continue
			}
			peerAddr := n.peers[rand.Intn(len(n.peers))]
			n.sendGossipTo(peerAddr)
		}
	}()
}

// func (n *Node) SimulateWrite(flightId, status string) {
// 	n.mu.Lock()
// 	defer n.mu.Unlock()
//
// 	state, exists := n.storage[flightId]
// 	newVersions := make(Clock)
//
// 	if exists {
// 		for k, v := range state.Clock.Versions {
// 			newVersions[k] = v
// 		}
// 	}
// 	newVersions[n.id]++
//
// 	n.storage[flightId] = &pb.FlightState{
// 		Status: status,
// 		Clock:  &pb.VectorClock{Versions: newVersions},
// 	}
// 	log.Printf("✏️ WRITE LOCAL: %s -> %s %v", flightId, status, newVersions)
// }

func (n *Node) Write(ctx context.Context, req *pb.WriteRequest) (*pb.WriteResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	state, exists := n.storage[req.FlightId]
	var currentStatus, currentGate string
	newVersions := make(Clock)

	if exists {
		currentStatus = state.Status
		currentGate = state.Gate
		for k, v := range state.Clock.Versions {
			newVersions[k] = v
		}
	}

	if req.UpdateType == "estado" {
		currentStatus = req.Value
	} else if req.UpdateType == "puerta" {
		currentGate = req.Value
	}

	newVersions[n.id]++

	n.storage[req.FlightId] = &pb.FlightState{
		Status: currentStatus,
		Gate:   currentGate,
		Clock:  &pb.VectorClock{Versions: newVersions},
	}

	return &pb.WriteResponse{
		Success: true,
		NodeId:  n.id,
		Clock:   &pb.VectorClock{Versions: newVersions},
	}, nil
}

func main() {
	port := os.Getenv("PORT")
	id := os.Getenv("NODE_ID")
	peers := os.Getenv("PEERS")

	node := &Node{
		id:      id,
		storage: make(map[string]*pb.FlightState),
		peers:   strings.Split(peers, ","),
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("❌ Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterDataNodeServiceServer(grpcServer, node)

	node.StartGossipLoop()

	node.showPeriodicState()

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ Failed to serve: %v", err)
	}
}
