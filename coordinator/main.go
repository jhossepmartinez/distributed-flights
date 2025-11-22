package main

import (
	"context"
	pb "distributed-flights/proto"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const SESSION_TTL = 60 * time.Second

type Session struct {
	nodeId    string
	expiresAt time.Time
}

type Coordinator struct {
	pb.UnimplementedCoordinatorServer
	// Mapa de conexiones a los DataNodes: "A" -> Cliente gRPC A
	dataNodes map[string]pb.DataNodeServiceClient
	// Slice de IDs para elegir aleatoriamente (Load Balancing)
	nodeIds []string
	// Sticky Sessions: ClientID -> Session
	sessions   map[string]Session
	sessionsMu sync.Mutex
}

func (c *Coordinator) ClientWrite(ctx context.Context, req *pb.ClientWriteRequest) (*pb.WriteResponse, error) {
	log.Printf("📝 [Coordinador] ClientWrite recibido: Cliente=%s, Vuelo=%s, Tipo=%s", req.ClientId, req.FlightId, req.UpdateType)

	nodeId := c.nodeIds[rand.Intn(len(c.nodeIds))]
	client := c.dataNodes[nodeId]
	resp, err := client.Write(ctx, &pb.WriteRequest{
		FlightId:   req.FlightId,
		UpdateType: req.UpdateType,
		Value:      req.Value,
	})
	if err != nil {
		log.Printf("❌ Error escribiendo en nodo %s: %v", nodeId, err)
		return nil, err
	}

	c.sessionsMu.Lock()
	c.sessions[req.ClientId] = Session{
		nodeId:    nodeId,
		expiresAt: time.Now().Add(SESSION_TTL),
	}
	log.Printf("🔒 Sesión actualizada: Cliente %s -> Nodo %s", req.ClientId, resp.NodeId)
	return resp, nil
}

func (c *Coordinator) ClientRead(ctx context.Context, req *pb.ClientReadRequest) (*pb.ReadResponse, error) {
	c.sessionsMu.Lock()
	session, exists := c.sessions[req.ClientId]

	if exists && time.Now().After(session.expiresAt) {
		delete(c.sessions, req.ClientId)
		exists = false
	}

	c.sessionsMu.Unlock()

	var targetNodeId string
	var client pb.DataNodeServiceClient

	if exists {
		targetNodeId = session.nodeId
		client = c.dataNodes[targetNodeId]
		log.Printf("🔍 [Coordinador] Lectura RYW (%s): Redirigiendo a Nodo %s", req.ClientId, targetNodeId)
	} else {
		targetNodeId = c.nodeIds[rand.Intn(len(c.nodeIds))]
		client = c.dataNodes[targetNodeId]
		log.Printf("🎲 [Coordinador] Lectura Aleatoria (%s): Redirigiendo a Nodo %s", req.ClientId, targetNodeId)
	}
	resp, err := client.Read(ctx, &pb.ReadRequest{
		FlightId: req.FlightId,
	})

	if err != nil {
		log.Printf("❌ Error leyendo en nodo %s: %v", targetNodeId, err)
		return nil, err
	}

	return resp, nil
}

func main() {
	port := os.Getenv("PORT")
	dataNodesEnv := os.Getenv("DATA_NODES_ADDRESSES")
	if port == "" || dataNodesEnv == "" {
		log.Fatal("❌ Faltan variables de entorno PORT o DATA_NODES_ADDRESSES")
	}

	coordinator := &Coordinator{
		dataNodes: make(map[string]pb.DataNodeServiceClient),
		sessions:  make(map[string]Session),
	}
	// Necesitamos saber qué IP corresponde a qué ID para el enrutamiento RYW
	pairs := strings.Split(dataNodesEnv, ",")
	for _, pair := range pairs {
		// Se espera formato "ID=ADDRESS" (Ej: A=localhost:50051)
		parts := strings.Split(pair, "=")
		if len(parts) != 2 {
			log.Printf("⚠️ Formato de dirección incorrecto: %s (se espera ID=ADDR)", pair)
			continue
		}
		nodeID := strings.TrimSpace(parts[0])
		address := strings.TrimSpace(parts[1])

		conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Fatalf("❌ No se pudo conectar al DataNode %s (%s): %v", nodeID, address, err)
		}

		coordinator.dataNodes[nodeID] = pb.NewDataNodeServiceClient(conn)
		coordinator.nodeIds = append(coordinator.nodeIds, nodeID) // Guardar ID para random picking
		log.Printf("✅ Coordinador conectado a DataNode %s en %s", nodeID, address)
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("❌ Fallo al escuchar en puerto %s: %v", port, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterCoordinatorServer(grpcServer, coordinator)

	log.Printf("🚀 Coordinador listo y escuchando en puerto %s", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("❌ Fallo al servir: %v", err)
	}
}
