package main

import (
	"context"
	pb "distributed-flights/proto"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Broker struct {
	clients        []pb.DataNodeServiceClient
	trafficClients []pb.TrafficServiceClient
}

func (b *Broker) SetupClients(addresses []string) {
	for _, address := range addresses {
		conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("❌ No se pudo conectar a %s: %v", address, err)
			continue
		}
		client := pb.NewDataNodeServiceClient(conn)
		b.clients = append(b.clients, client)
		log.Printf("✅ Conectado a Data Node en %s", address)
	}
}

func (b *Broker) SetupTrafficClients(addresses []string) {
	for _, address := range addresses {
		conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("❌ No se pudo conectar a %s: %v", address, err)
			continue
		}
		client := pb.NewTrafficServiceClient(conn)
		b.trafficClients = append(b.trafficClients, client)
		log.Printf("Conecado a Traffic Node en %s", address)
	}

}

func (b *Broker) RequestLanding(flightID string) {
	log.Printf("🛬 Solicitando pista para vuelo %s...", flightID)

	landingExitoso := false

	for _, atcClient := range b.trafficClients {
		// Timeout corto para probar rápido
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		resp, err := atcClient.RequestLanding(ctx, &pb.LandingRequest{
			FlightId: flightID,
		})
		cancel()

		if err != nil {
			// Puede que el nodo esté caído, probamos el siguiente
			continue
		}

		if resp.Success {
			log.Printf("✅ PISTA ASIGNADA: Vuelo %s aterrizará en %s", flightID, resp.AssignedRunwayId)
			landingExitoso = true
			break // ¡Listo!
		} else if resp.Message == "No soy lider" {
			// Este nodo no es líder, seguimos iterando buscando al correcto
			continue
		} else {
			// Falló por otra razón (ej: Pistas llenas)
			log.Printf("⚠️ ATC rechazó aterrizaje: %s", resp.Message)
			break
		}
	}

	if !landingExitoso {
		log.Printf("❌ No se pudo asignar pista para %s (Sin líder o Error)", flightID)
	}
}

func (b *Broker) GetRandomClient() pb.DataNodeServiceClient {
	if len(b.clients) == 0 {
		return nil
	}
	return b.clients[rand.Intn(len(b.clients))]
}

func (b *Broker) ProcessCSV(filename string) {
	log.Println("📂 Broker: Iniciando procesamiento de CSV...")

	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("⚠️ No se pudo abrir %s: %v", filename, err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// Leer encabezado
	_, _ = reader.Read()

	var lastSimTime int = 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error leyendo línea: %v", err)
			continue
		}

		// CSV: sim_time_sec, flight_id, update_type, update_value
		if len(record) < 4 {
			continue
		}

		simTime, _ := strconv.Atoi(record[0])
		flightID := record[1]
		updateType := record[2]
		updateValue := record[3]
		fullStatus := fmt.Sprintf("%s: %s", record[2], record[3])

		// 1. Simular espera temporal
		delay := simTime - lastSimTime
		if delay > 0 {
			log.Printf("⏳ Broker esperando %ds...", delay)
			time.Sleep(time.Duration(delay) * time.Second)
		}
		lastSimTime = simTime

		// 2. Seleccionar DataNode destino
		client := b.GetRandomClient()
		if client == nil {
			log.Println("❌ Error: No hay DataNodes disponibles para escribir")
			continue
		}

		// 3. Enviar Escritura gRPC
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		resp, err := client.Write(ctx, &pb.WriteRequest{
			FlightId:   flightID,
			UpdateType: updateType,
			Value:      updateValue,
		})
		cancel()

		if err != nil {
			log.Printf("❌ Falló escritura de %s: %v", flightID, err)
		} else {
			if updateType == "estado" && updateValue == "En vuelo" {
				go b.RequestLanding(flightID)
			}
			log.Printf("📨 Broker -> DataNode [%s]: %s | Reloj resultante: %v",
				resp.NodeId, fullStatus, resp.Clock.Versions)
		}
	}
	log.Println("🏁 Fin del archivo CSV")
}

func main() {
	fmt.Printf("Hello, World!\n")
	dataNodesAddresses := os.Getenv("DATA_NODES_ADDRESSES")
	trafficNodesAddresses := os.Getenv("TRAFFIC_ADDRESSES")

	if dataNodesAddresses == "" || trafficNodesAddresses == "" {
		log.Fatal(" DATA_NODES_ADDRESSES o TRAFFIC_ADDRESSES no configurados")
	}

	broker := &Broker{}
	broker.SetupClients(strings.Split(dataNodesAddresses, ","))
	broker.SetupTrafficClients(strings.Split(trafficNodesAddresses, ","))
	time.Sleep(6 * time.Second)
	broker.ProcessCSV("flight_updates.csv")
	select {}
}
