package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
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
		n.storage[req.FlightId] = &pb.FlightState{Status: resolvedStatus, Clock: mergedClock}
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
				fmt.Printf("   - %s: [%s] Reloj: %v\n", fid, st.Status, st.Clock.Versions)
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
			time.Sleep(3 * time.Second)

			if len(n.peers) == 0 {
				continue
			}
			peerAddr := n.peers[rand.Intn(len(n.peers))]
			n.sendGossipTo(peerAddr)
		}
	}()
}

func (n *Node) SimulateWrite(flightId, status string) {
	n.mu.Lock()
	defer n.mu.Unlock()

	state, exists := n.storage[flightId]
	newVersions := make(Clock)

	if exists {
		for k, v := range state.Clock.Versions {
			newVersions[k] = v
		}
	}
	newVersions[n.id]++

	n.storage[flightId] = &pb.FlightState{
		Status: status,
		Clock:  &pb.VectorClock{Versions: newVersions},
	}
	log.Printf("✏️ WRITE LOCAL: %s -> %s %v", flightId, status, newVersions)
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

	go func() {
		// Espera inicial para que los servidores levanten
		time.Sleep(5 * time.Second)

		if id == "A" {
			log.Println("📂 Nodo A: Iniciando lectura de flight_updates.csv...")

			file, err := os.Open("flight_updates.csv")
			if err != nil {
				log.Printf("⚠️ No se pudo abrir flight_updates.csv: %v", err)
				return
			}
			defer file.Close()

			reader := csv.NewReader(file)

			// Leer encabezado y descartarlo si existe
			_, err = reader.Read()
			if err != nil {
				log.Printf("Error leyendo CSV: %v", err)
				return
			}

			var lastSimTime int = 0

			for {
				record, err := reader.Read()
				if err == io.EOF {
					break
				}
				if err != nil {
					log.Printf("Error leyendo línea CSV: %v", err)
					continue
				}

				// Estructura CSV esperada: sim_time_sec, flight_id, update_type, update_value
				// Ejemplo: 2, AF-021, estado, En vuelo
				if len(record) < 4 {
					continue
				}

				simTime, _ := strconv.Atoi(record[0])
				flightID := record[1]
				updateType := record[2]
				updateValue := record[3]
				if updateType == "puerta" {
					continue
				}

				// Calcular cuánto esperar desde el último evento
				delay := simTime - lastSimTime
				if delay > 0 {
					log.Printf("⏳ Esperando %ds para el siguiente evento...", delay)
					time.Sleep(time.Duration(delay) * time.Second)
				}
				lastSimTime = simTime

				// Formato descriptivo: "estado: En vuelo" o "puerta: A2"
				fullStatus := fmt.Sprintf("%s: %s", updateType, updateValue)

				// Ejecutar la escritura local
				node.SimulateWrite(flightID, fullStatus)
			}
			log.Println("✅ Fin de la simulación CSV.")
		}
	}()

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ Failed to serve: %v", err)
	}
}
