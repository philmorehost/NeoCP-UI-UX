package api

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
	"neocp/internal/core"
	"neocp/internal/core/cluster"
)

// Global CA certificate store for mTLS validation
var (
	caCertPool  *x509.CertPool
	caKeyPair   *cluster.PEMKeyPair
	masterKeys  *cluster.PEMKeyPair
	clusterMu   sync.Mutex
	initialized bool
)

// InitializeClusterCertificates sets up the CA and Master key pair
func InitializeClusterCertificates() error {
	clusterMu.Lock()
	defer clusterMu.Unlock()

	if initialized {
		return nil
	}

	// Generate cluster keys dynamically (Master at 127.0.0.1 and localhost)
	ca, err := cluster.GenerateCA()
	if err != nil {
		return err
	}
	master, err := cluster.GenerateCert(ca, "neocp-master", []string{"127.0.0.1", "localhost"}, true)
	if err != nil {
		return err
	}

	caKeyPair = ca
	masterKeys = master

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caKeyPair.CertPEM) {
		return errors.New("failed to append CA certificate to pool")
	}
	caCertPool = caPool
	initialized = true
	return nil
}

// ClusterAttachRequest is the body for node registration
type ClusterAttachRequest struct {
	NodeID string `json:"node_id"`
	IP     string `json:"ip"`
	Role   string `json:"role"` // web, db, mail
}

// HandleCluster REST bindings for panel views
func HandleCluster(w http.ResponseWriter, r *http.Request) {
	role := r.Header.Get("NeoCP-Role")
	isAdmin := (role == "admin")

	if !isAdmin {
		http.Error(w, "Forbidden: administrators only", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	db := core.GetDB()

	if r.Method == http.MethodGet {
		nodes := db.GetClusterNodes()
		json.NewEncoder(w).Encode(nodes)
		return
	}

	if r.Method == http.MethodPost {
		// Initialize CA on-the-fly if needed
		if err := InitializeClusterCertificates(); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		var req ClusterAttachRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request payload"})
			return
		}

		if req.NodeID == "" || req.IP == "" || req.Role == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Missing required fields: node_id, ip, role"})
			return
		}

		// Generate dynamic certificate pair for this specific worker node IP
		workerPair, err := cluster.GenerateCert(caKeyPair, "neocp-worker", []string{req.IP}, false)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Failed to generate client cert: %s", err)})
			return
		}

		// Save node metadata in control DB
		node := core.ClusterNode{
			NodeID:   req.NodeID,
			IP:       req.IP,
			Role:     req.Role,
			IsActive: false, // Inactive until client connects over TLS
			CPULoad:  0.0,
			RAMLoad:  0.0,
			LastPing: time.Now(),
		}

		if err := db.CreateClusterNode(node); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Return credentials package to attaching worker
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"node_id":    req.NodeID,
			"ca_cert":    string(caKeyPair.CertPEM),
			"client_cert": string(workerPair.CertPEM),
			"client_key":  string(workerPair.KeyPEM),
		})
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

// StartClusterServer boots a secure, mutually authenticated TCP server representing the gRPC cluster network
func StartClusterServer(addr string) error {
	if err := InitializeClusterCertificates(); err != nil {
		return err
	}

	// Configure Mutual TLS (mTLS)
	tlsCert, err := tls.X509KeyPair(masterKeys.CertPEM, masterKeys.KeyPEM)
	if err != nil {
		return err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
	}

	listener, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleWorkerStream(conn)
	}
}

// TelemetryPacket is the packet shape sent by workers
type TelemetryPacket struct {
	NodeID  string              `json:"node_id"`
	Metrics cluster.NodeMetrics `json:"metrics"`
}

// handleWorkerStream manages the live mutual TLS bidirectional connection
func handleWorkerStream(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var nodeID string
	var connStruct *cluster.NodeConnection
	mgr := cluster.GetManager()

	// Clean up connection on exit
	defer func() {
		if nodeID != "" {
			mgr.DeregisterNodeConnection(nodeID)
		}
	}()

	// Channel to signal read termination
	doneChan := make(chan struct{})

	for {
		// Read Worker telemetry/metrics packet
		var pkt TelemetryPacket
		conn.SetReadDeadline(time.Now().Add(6 * time.Second))
		err := decoder.Decode(&pkt)
		if err != nil {
			if err != io.EOF {
				// Log or handle read error
			}
			break
		}

		nodeID = pkt.NodeID

		// Register active connection in clustering manager if not already done
		if connStruct == nil {
			c, err := mgr.RegisterNodeConnection(nodeID)
			if err != nil {
				// Reject unregistered nodes
				_ = encoder.Encode(map[string]string{"error": err.Error()})
				break
			}
			connStruct = c

			// Spawn a goroutine to stream MasterJobs to worker
			go func() {
				for {
					select {
					case <-connStruct.Ctx.Done():
						return
					case job, ok := <-connStruct.JobChan:
						if !ok {
							return
						}
						conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
						if err := encoder.Encode(job); err != nil {
							return
						}
					}
				}
			}()
		}

		// Update metrics in persistence manager
		_ = mgr.UpdateMetrics(nodeID, pkt.Metrics)

		// Simple ACK keep-alive back
		conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		_ = encoder.Encode(map[string]string{"status": "ack"})
	}

	close(doneChan)
}
