package cluster

import (
	"context"
	"errors"
	"sync"
	"time"
	"neocp/internal/core"
)

// MasterJob defines a command payload pushed from Master to Worker
type MasterJob struct {
	JobID   string `json:"job_id"`
	Command string `json:"command"` // e.g. "provision_db", "create_vhost", "update_waf"
	Payload string `json:"payload"`
}

// NodeMetrics encapsulates real-time resource loads streamed by workers
type NodeMetrics struct {
	CPULoad float64 `json:"cpu_load"`
	RAMLoad float64 `json:"ram_load"`
}

// NodeConnection represents an active, authenticated remote node session
type NodeConnection struct {
	NodeID  string
	JobChan chan MasterJob
	Ctx     context.Context
	Cancel  context.CancelFunc
}

// Manager orchestrates registered worker nodes and dynamic task pipelines
type Manager struct {
	mu          sync.RWMutex
	connections map[string]*NodeConnection
}

var (
	globalManager *Manager
	once          sync.Once
)

// GetManager initializes and returns the cluster manager singleton
func GetManager() *Manager {
	once.Do(func() {
		globalManager = &Manager{
			connections: make(map[string]*NodeConnection),
		}
		// Start a background monitor to check and prune dead nodes
		go globalManager.startHeartbeatWatchdog()
	})
	return globalManager
}

// RegisterNodeConnection establishes a live task queue for an attached worker
func (m *Manager) RegisterNodeConnection(nodeID string) (*NodeConnection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if node exists in DB
	db := core.GetDB()
	nodes := db.GetClusterNodes()
	found := false
	for _, n := range nodes {
		if n.NodeID == nodeID {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("node not registered in cluster database")
	}

	// Close old connection if any
	if old, exists := m.connections[nodeID]; exists {
		old.Cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	conn := &NodeConnection{
		NodeID:  nodeID,
		JobChan: make(chan MasterJob, 100),
		Ctx:     ctx,
		Cancel:  cancel,
	}

	m.connections[nodeID] = conn

	// Set node as active in database
	_ = db.UpdateClusterNodeStatus(nodeID, true)

	return conn, nil
}

// DeregisterNodeConnection cleans up active channels when a worker detaches
func (m *Manager) DeregisterNodeConnection(nodeID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn, exists := m.connections[nodeID]; exists {
		conn.Cancel()
		delete(m.connections, nodeID)
	}

	db := core.GetDB()
	_ = db.UpdateClusterNodeStatus(nodeID, false)
}

// PushJob queues a control command to be dispatched to a worker node
func (m *Manager) PushJob(nodeID string, job MasterJob) error {
	m.mu.RLock()
	conn, exists := m.connections[nodeID]
	m.mu.RUnlock()

	if !exists {
		return errors.New("node is currently offline or unreachable")
	}

	select {
	case conn.JobChan <- job:
		return nil
	default:
		return errors.New("node job queue is full")
	}
}

// UpdateMetrics processes raw metric telemetry from an active worker
func (m *Manager) UpdateMetrics(nodeID string, metrics NodeMetrics) error {
	db := core.GetDB()
	return db.UpdateClusterNodeMetrics(nodeID, metrics.CPULoad, metrics.RAMLoad)
}

// startHeartbeatWatchdog audits pings and marks inactive nodes as offline
func (m *Manager) startHeartbeatWatchdog() {
	ticker := time.NewTicker(2000 * time.Millisecond)
	for range ticker.C {
		db := core.GetDB()
		nodes := db.GetClusterNodes()
		now := time.Now()

		for _, node := range nodes {
			if node.IsActive {
				// If no ping has been received for more than 5 seconds, flag offline
				if now.Sub(node.LastPing) > 5*time.Second {
					m.mu.Lock()
					if conn, exists := m.connections[node.NodeID]; exists {
						conn.Cancel()
						delete(m.connections, node.NodeID)
					}
					m.mu.Unlock()

					_ = db.UpdateClusterNodeStatus(node.NodeID, false)
				}
			}
		}
	}
}
