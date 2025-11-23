package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	pb "distributed-flights/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type RYWClient struct {
	id                string
	coordinatorClient pb.CoordinatorClient
}

// Intenta hacer Check-in en un asiento específico
func (c *RYWClient) AttemptCheckIn(flightID string) {
	// 1. Obtener estado actual (Read Your Writes)
	// leer, escribir, leer
	// Como no tenemos versión previa, mandamos nil o vacío
	log.Printf("[Cliente %s] Consultando disponibilidad vuelo %s...", c.id, flightID)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	readResp, err := c.coordinatorClient.ClientRead(ctx, &pb.ClientReadRequest{
		ClientId:      c.id,
		FlightId:      flightID,
		KnownVersions: nil,
	})
	cancel()

	if err != nil {
		log.Printf("Error consultando vuelo: %v", err)
		return
	}

	// 2. Lógica para elegir asiento (Solo hay 2: 1A y 1B)
	targetSeat := ""
	seats := readResp.State.SeatMap
	if seats == nil {
		seats = make(map[string]string)
	}

	if _, occupied := seats["1A"]; !occupied {
		targetSeat = "1A"
	} else if _, occupied := seats["1B"]; !occupied {
		targetSeat = "1B"
	} else {
		log.Printf("Cliente %s] Vuelo LLENO. No hay asientos disponibles.", c.id)
		return
	}

	log.Printf("[Cliente %s] Intentando reservar asiento %s...", c.id, targetSeat)

	// 3. Enviar Escritura (Check-In)
	// Formato Value: "Asiento,PasajeroID"
	value := fmt.Sprintf("%s,%s", targetSeat, c.id)

	ctxW, cancelW := context.WithTimeout(context.Background(), 2*time.Second)
	_, err = c.coordinatorClient.ClientWrite(ctxW, &pb.ClientWriteRequest{
		ClientId:   c.id,
		FlightId:   flightID,
		UpdateType: "asiento",
		Value:      value,
	})
	cancelW()

	if err != nil {
		log.Printf("Falló la escritura: %v", err)
		return
	}

	// 4. VERIFICACIÓN (Read Your Writes)
	// El Coordinador  nos redirige al nodo de la sesion
	time.Sleep(500 * time.Millisecond) // Pequeña pausa simulando latencia VM

	ctxR, cancelR := context.WithTimeout(context.Background(), 2*time.Second)
	verifyResp, err := c.coordinatorClient.ClientRead(ctxR, &pb.ClientReadRequest{
		ClientId: c.id,
		FlightId: flightID,
	})
	cancelR()

	if err != nil {
		log.Printf("Error verificando: %v", err)
		return
	}

	// Comprobamos si el asiento es nuestro
	owner, ok := verifyResp.State.SeatMap[targetSeat]
	if ok && owner == c.id {
		log.Printf("[Cliente %s] ¡CHECK-IN EXITOSO! Tengo el asiento %s (Confirmado por RYW)", c.id, targetSeat)
	} else {
		log.Printf("[Cliente %s] Check-in FALLIDO. El asiento %s lo tiene: %s (Conflicto o Error)", c.id, targetSeat, owner)
	}
}

func main() {
	clientID := os.Getenv("CLIENT_ID") // Ej: "Pasajero-1"
	if clientID == "" {
		log.Fatal("CLIENT_ID no está definido")
	}
	log.Printf("Cliente RYW %s iniciando...", clientID)

	coordAddr := os.Getenv("COORDINATOR_ADDR") // Ej: "localhost:50055"
	log.Printf("Cliente RYW %s iniciando. Conectando al Coordinador en %s", clientID, coordAddr)

	if coordAddr == "" {
		log.Fatal("COORDINATOR_ADDR no está definido")
	}

	conn, err := grpc.NewClient(coordAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Error conectando: %v", err)
	}
	defer conn.Close()

	client := &RYWClient{
		id:                clientID,
		coordinatorClient: pb.NewCoordinatorClient(conn),
	}

	// Simular comportamiento
	flightID := os.Getenv("FLIGHT_ID") // Asegúrate que este vuelo exista (creado por Broker)

	log.Printf("Cliente %s iniciando proceso de check-in...", clientID)

	// Intentar hacer check-in
	client.AttemptCheckIn(flightID)

	// Esperar un poco antes de salir
	time.Sleep(5 * time.Second)
}
