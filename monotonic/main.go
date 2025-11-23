package main

import (
	"context"
	pb "distributed-flights/proto"
	"log"
	"os"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type MonotonicClient struct {
	id                string
	coordinatorClient pb.CoordinatorClient
	lastSeenVersions  map[string]*pb.VectorClock // Mapa: FlightID -> Último Reloj Visto
}

func (c *MonotonicClient) MonitorFlight(flightID string) {
	for {
		time.Sleep(3 * time.Second) // Consultar cada 3 segundos

		req := &pb.ClientReadRequest{
			ClientId: c.id,
			FlightId: flightID,
		}

		// Adjuntar versión conocida si existe
		if lastClock, ok := c.lastSeenVersions[flightID]; ok {
			req.KnownVersions = lastClock
		} else {
			// Inicialmente vacío o nil par a los ryw
			req.KnownVersions = &pb.VectorClock{Versions: make(map[string]int32)}
		}

		log.Printf(" [Cliente %s] Consultando %s. Mi versión conocida: %v",
			c.id, flightID, req.KnownVersions.Versions)

		// 2. Llamar al Coordinador (que redirige al Broker/Nodos)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		resp, err := c.coordinatorClient.ClientRead(ctx, req)
		cancel()

		if err != nil {
			log.Printf("[Cliente %s] Falló lectura (posiblemente nodo desactualizado): %v", c.id, err)
			// Al fallar, simplemente esperamos al siguiente ciclo.
			// El Broker probablemente nos mandará a otro nodo la próxima vez.
			continue
		}

		// 3. Éxito: Actualizar mi versión local
		// Solo actualizamos si la respuesta es válida
		if resp.State != nil && resp.State.Clock != nil {
			c.lastSeenVersions[flightID] = resp.State.Clock
			log.Printf(" [Cliente %s] Lectura Exitosa. Vuelo: %s | Estado: %s | Puerta: %s | Nueva Versión: %v",
				flightID, c.id, resp.State.Status, resp.State.Gate, resp.State.Clock.Versions)
			// print de los asientos
			log.Printf("    Asientos: %v", resp.State.SeatMap)
		}
	}
}

func main() {
	clientId := os.Getenv("CLIENT_ID")
	flightId := os.Getenv("FLIGHT_ID")
	if clientId == "" || flightId == "" {
		log.Fatal("CLIENT_ID no está configurado")
	}

	coordAddr := os.Getenv("COORDINATOR_ADDR")
	conn, err := grpc.NewClient(coordAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Error conectando al Coordinador: %v", err)
	}
	defer conn.Close()

	client := &MonotonicClient{
		id:                clientId,
		coordinatorClient: pb.NewCoordinatorClient(conn),
		lastSeenVersions:  make(map[string]*pb.VectorClock),
	}

	log.Printf("🔭 Cliente Monotonic Reads %s iniciado.", clientId)

	// Monitorear un vuelo de ejemplo (debe existir en el CSV o ser creado por el Broker)
	// Puedes lanzar varias goroutines para varios vuelos
	go client.MonitorFlight(flightId)

	select {}
}
