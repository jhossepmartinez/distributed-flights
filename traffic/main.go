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

const (
	Follower  = "Follower"
	Candidate = "Candidate"
	Leader    = "Leader"
)

var RUNWAYS_IDS = []string{"RWY-01", "RWY-02", "RWY-03"}

type LogEntry struct {
	Term    int32
	Command string
}

type Node struct {
	pb.UnimplementedTrafficServiceServer
	mu    sync.Mutex
	id    string
	peers []string

	currentTerm int32
	votedFor    string
	log         []LogEntry

	state       string
	commitIndex int32
	lastApplied int32
	leaderId    string

	heartbeatCh chan bool
	voteCh      chan bool
}

func (n *Node) RequestLanding(ctx context.Context, req *pb.LandingRequest) (*pb.LandingResponse, error) {
	n.mu.Lock()
	// 1. Validar Liderazgo
	if n.state != Leader {
		leader := n.leaderId
		n.mu.Unlock()
		return &pb.LandingResponse{Success: false, Message: "No soy lider", LeaderHint: leader}, nil
	}

	// 2. ASIGNACIÓN ALEATORIA INTELIGENTE
	// Intentamos buscar una pista libre probando en orden aleatorio
	selectedRunway := ""

	// Creamos una copia barajada de las pistas para probar suerte
	shuffledRunways := make([]string, len(RUNWAYS_IDS))
	copy(shuffledRunways, RUNWAYS_IDS)
	rand.Shuffle(len(shuffledRunways), func(i, j int) { shuffledRunways[i], shuffledRunways[j] = shuffledRunways[j], shuffledRunways[i] })

	for _, runway := range shuffledRunways {
		if !n.isRunwayTaken(runway) {
			selectedRunway = runway
			break // ¡Encontramos una!
		}
	}

	// Si después de revisar todas no hay ninguna libre:
	if selectedRunway == "" {
		n.mu.Unlock()
		log.Printf("⚠️ [LIDER] Aeropuerto lleno. Rechazando vuelo %s", req.FlightId)
		return &pb.LandingResponse{Success: false, Message: "Todas las pistas ocupadas"}, nil
	}

	// 3. Crear el comando con la pista seleccionada
	// Formato: "PISTA:VUELO"
	command := fmt.Sprintf("%s:%s", selectedRunway, req.FlightId)

	// Agregar al log local
	entry := LogEntry{Term: n.currentTerm, Command: command}
	n.log = append(n.log, entry)
	logIndex := int32(len(n.log) - 1)
	n.mu.Unlock()

	log.Printf("📝 [LIDER] Propuesta creada: Asignar %s a %s (Index: %d). Replicando...", selectedRunway, req.FlightId, logIndex)

	// 4. Esperar Consenso (Polling simple)
	for i := 0; i < 20; i++ { // Esperamos hasta 2 segundos aprox (20 * 100ms)
		time.Sleep(100 * time.Millisecond)
		n.mu.Lock()
		if n.commitIndex >= logIndex {
			n.mu.Unlock()
			log.Printf("✅ [LIDER] Consenso alcanzado. %s asignada a %s.", selectedRunway, req.FlightId)

			return &pb.LandingResponse{
				Success:          true,
				Message:          "Aterrizaje Autorizado",
				AssignedRunwayId: selectedRunway,
			}, nil
		}
		n.mu.Unlock()
	}

	return &pb.LandingResponse{Success: false, Message: "Timeout esperando consenso"}, nil
}

// RequestVote: Un candidato nos pide el voto
func (n *Node) RequestVote(ctx context.Context, req *pb.VoteRequest) (*pb.VoteResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if req.Term > n.currentTerm {
		n.becomeFollower(req.Term)
	}

	voteGranted := false
	if req.Term == n.currentTerm && (n.votedFor == "" || n.votedFor == req.CandidateId) {
		// Aquí deberíamos chequear si el log del candidato está al día (omitido por simplicidad del lab)
		voteGranted = true
		n.votedFor = req.CandidateId
		n.voteCh <- true // Reiniciar timer de elección
		log.Printf("🗳️ Voto otorgado a %s para termino %d", req.CandidateId, req.Term)
	}

	return &pb.VoteResponse{Term: n.currentTerm, VoteGranted: voteGranted}, nil
}

// AppendEntries: El líder nos manda heartbeats o logs
func (n *Node) AppendEntries(ctx context.Context, req *pb.AppendEntriesRequest) (*pb.AppendEntriesResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if req.Term > n.currentTerm {
		n.becomeFollower(req.Term)
	}

	success := false
	if req.Term == n.currentTerm {
		n.state = Follower
		n.leaderId = req.LeaderId

		// Importante: Usamos select default para no bloquear si el canal está lleno
		select {
		case n.heartbeatCh <- true:
		default:
		}

		success = true

		// Si recibimos entradas, debemos asegurar que nuestro log coincida con el del líder.
		// sobreescribir log si el lider tiene cambio
		if len(req.Entries) > 0 {
			newLog := make([]LogEntry, 0)
			for _, entry := range req.Entries {
				newLog = append(newLog, LogEntry{Term: entry.Term, Command: entry.Command})
			}
			n.log = newLog

			log.Printf("DEBUG log actualizado: %+v", n.log)
		}

		if req.LeaderCommit > n.commitIndex {
			if req.LeaderCommit < int32(len(n.log)) {
				n.commitIndex = req.LeaderCommit
			} else {
				n.commitIndex = int32(len(n.log)) - 1
			}
			log.Printf("⚙️ [FOLLOWER] Commit Index actualizado a %d", n.commitIndex)
		}
	}

	return &pb.AppendEntriesResponse{Term: n.currentTerm, Success: success}, nil
}

func (n *Node) becomeFollower(term int32) {
	n.state = Follower
	n.currentTerm = term
	n.votedFor = ""
	log.Printf("⬇️ Soy SEGUIDOR (Term: %d)", n.currentTerm)
}

func (n *Node) runElectionTimer() {
	for {
		timeout := time.Duration(1500+rand.Intn(1500)) * time.Millisecond
		select {
		case <-n.heartbeatCh:
		case <-n.voteCh:
			// Voté por alguien, reinicio timer
		case <-time.After(timeout):
			n.mu.Lock()
			if n.state != Leader {
				n.startElection()
			}
			n.mu.Unlock()
		}
	}
}

func (n *Node) startElection() {
	n.state = Candidate
	n.currentTerm++
	n.votedFor = n.id
	votes := 1 // Voto por mí mismo
	log.Printf("✋ Iniciando ELECCIÓN (Term: %d)", n.currentTerm)

	// Pedir votos a los peers en paralelo
	for _, peer := range n.peers {
		go func(addr string) {
			conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
			if err != nil {
				return
			}
			defer conn.Close()
			client := pb.NewTrafficServiceClient(conn)

			resp, err := client.RequestVote(context.Background(), &pb.VoteRequest{
				Term: n.currentTerm, CandidateId: n.id,
			})

			if err == nil && resp.VoteGranted {
				n.mu.Lock()
				if n.state == Candidate && resp.Term == n.currentTerm {
					votes++
					// Mayoría simple (Quórum)
					if votes > (len(n.peers)+1)/2 {
						n.becomeLeader()
					}
				}
				n.mu.Unlock()
			}
		}(peer)
	}
}

func (n *Node) becomeLeader() {
	if n.state == Leader {
		return
	}
	n.state = Leader
	n.leaderId = n.id
	log.Printf(" ¡SOY EL LÍDER! (Term: %d)", n.currentTerm)

	// Iniciar loop de heartbeat inmediato
	go n.sendHeartbeats()
}

func (n *Node) sendHeartbeats() {
	for {
		n.mu.Lock()
		if n.state != Leader {
			n.mu.Unlock()
			return
		}

		term := n.currentTerm
		id := n.id
		leaderCommit := n.commitIndex

		// --- CORRECCIÓN 1: Enviar el Log Real, no vacío ---
		// Para simplificar el lab, enviamos TODO el log en cada heartbeat.
		// (En producción esto sería ineficiente, pero garantiza convergencia aquí).
		var entries []*pb.LogEntry
		for _, l := range n.log {
			entries = append(entries, &pb.LogEntry{Term: l.Term, Command: l.Command})
		}

		// El índice que estamos intentando replicar es el último del log
		targetIndex := int32(len(n.log) - 1)

		n.mu.Unlock()

		// Contador de ACKs (El líder ya tiene la entrada, así que empieza en 1)
		acks := 1
		totalNodes := len(n.peers) + 1
		quorum := totalNodes/2 + 1

		// Usamos WaitGroup para esperar a que los RPCs terminen (o timeout) antes de evaluar quórum
		// O simplemente lanzamos goroutines y protegemos 'acks' con mutex.

		for _, peer := range n.peers {
			go func(addr string) {
				conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
				if err != nil {
					return
				}
				defer conn.Close()
				client := pb.NewTrafficServiceClient(conn)

				// Timeout breve para no bloquear el heartbeat
				ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				defer cancel()

				resp, err := client.AppendEntries(ctx, &pb.AppendEntriesRequest{
					Term: term, LeaderId: id, LeaderCommit: leaderCommit, Entries: entries,
				})

				if err == nil {
					n.mu.Lock()
					defer n.mu.Unlock()

					if resp.Term > n.currentTerm {
						n.becomeFollower(resp.Term)
						return
					}

					if resp.Success {
						// --- CORRECCIÓN 2: Contar ACKs y Avanzar Commit ---
						acks++
						if acks >= quorum {
							// Si alcanzamos mayoría y el target es mayor al commit actual, avanzamos.
							if targetIndex > n.commitIndex {
								n.commitIndex = targetIndex
								log.Printf("✅ [LIDER] CommitIndex avanzado a %d (Quórum alcanzado)", n.commitIndex)
							}
						}
					}
				}
			}(peer)
		}

		// Aumentamos frecuencia a 500ms para que reaccione antes de los 2s del Broker
		time.Sleep(500 * time.Millisecond)
	}
}

// Verifica si una pista está tomada en el Log COMPROMETIDO
func (n *Node) isRunwayTaken(runwayID string) bool {
	for i := int32(0); i <= n.commitIndex && i < int32(len(n.log)); i++ {
		entry := n.log[i]
		parts := strings.Split(entry.Command, ":")
		if len(parts) == 2 && parts[0] == runwayID {
			return true // Pista ocupada
		}
	}
	return false
}

// --- MAIN ---

func main() {
	id := os.Getenv("NODE_ID")     // Ej: "ATC-1"
	port := os.Getenv("PORT")      // Ej: "50060"
	peersEnv := os.Getenv("PEERS") // Ej: "10.0.0.2:50060,10.0.0.3:50060"

	node := &Node{
		id:          id,
		peers:       strings.Split(peersEnv, ","),
		state:       Follower,
		currentTerm: 0,
		log:         make([]LogEntry, 0),
		commitIndex: -1,
		heartbeatCh: make(chan bool),
		voteCh:      make(chan bool),
	}

	// Iniciar lógica Raft
	go node.runElectionTimer()

	// Servidor gRPC
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterTrafficServiceServer(grpcServer, node)

	log.Printf("✈️ Nodo ATC %s escuchando en %s", id, port)
	grpcServer.Serve(lis)
}
