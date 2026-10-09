package game

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// SyncBundle packages profiles, domain rules, and routing configurations for distribution to edge nodes.
type SyncBundle struct {
	Version     int           `json:"version"`
	GeneratedAt time.Time     `json:"generated_at"`
	Profiles    []GameProfile `json:"profiles"`
	Nodes       []RouteNode   `json:"nodes"`
}

// MultiNodeController coordinates cluster topology, configuration synchronization, and edge health.
type MultiNodeController struct {
	store  *Store
	routes *RouteManager
}

// NewMultiNodeController creates a new MultiNodeController.
func NewMultiNodeController(store *Store, routes *RouteManager) *MultiNodeController {
	return &MultiNodeController{
		store:  store,
		routes: routes,
	}
}

// RegisterNode adds or activates an edge VPS node in the cluster.
func (c *MultiNodeController) RegisterNode(node RouteNode, token, actor string) error {
	if node.ID == "" || node.Endpoint == "" {
		return fmt.Errorf("node ID and endpoint are required")
	}

	node.AuthToken = token
	node.Active = true
	node.SyncStatus = SyncPending
	node.LastCheck = time.Now()

	if err := c.store.SaveRouteNode(node); err != nil {
		return err
	}

	_ = c.store.AppendAuditLog(AuditEntry{
		Actor:      actor,
		Action:     "node_registered",
		TargetType: "route",
		TargetID:   node.ID,
		Details:    fmt.Sprintf("Registered edge node %s (%s)", node.Name, node.Endpoint),
	})

	return nil
}

// RevokeNode removes an edge node and invalidates its access.
func (c *MultiNodeController) RevokeNode(nodeID, actor string) error {
	node, err := c.store.GetRouteNode(nodeID)
	if err != nil {
		return err
	}

	if err := c.store.DeleteRouteNode(nodeID); err != nil {
		return err
	}

	_ = c.store.AppendAuditLog(AuditEntry{
		Actor:      actor,
		Action:     "node_revoked",
		TargetType: "route",
		TargetID:   nodeID,
		Details:    fmt.Sprintf("Revoked edge node %s (%s)", node.Name, node.Endpoint),
	})

	return nil
}

// BuildSyncBundle packages current game profiles and active topology with cryptographic signature.
func (c *MultiNodeController) BuildSyncBundle(secretKey string) ([]byte, string, error) {
	profiles, err := c.store.ListGameProfiles()
	if err != nil {
		return nil, "", err
	}

	nodes, err := c.store.ListRouteNodes()
	if err != nil {
		return nil, "", err
	}

	bundle := SyncBundle{
		Version:     int(time.Now().Unix()),
		GeneratedAt: time.Now(),
		Profiles:    profiles,
		Nodes:       nodes,
	}

	data, err := json.Marshal(bundle)
	if err != nil {
		return nil, "", err
	}

	// Calculate HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write(data)
	sig := hex.EncodeToString(mac.Sum(nil))

	return data, sig, nil
}

// VerifyBundle checks the authenticity of a sync bundle received on an edge node.
func (c *MultiNodeController) VerifyBundle(data []byte, signature, secretKey string) bool {
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write(data)
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedSig))
}

// SyncNode marks a node as Synced after successfully transferring rules.
func (c *MultiNodeController) SyncNode(nodeID, actor string) error {
	node, err := c.store.GetRouteNode(nodeID)
	if err != nil {
		return err
	}

	node.SyncStatus = SyncSynced
	node.LastCheck = time.Now()
	if err := c.store.SaveRouteNode(*node); err != nil {
		return err
	}

	_ = c.store.AppendAuditLog(AuditEntry{
		Actor:      actor,
		Action:     "node_synced",
		TargetType: "route",
		TargetID:   nodeID,
		Details:    fmt.Sprintf("Successfully synchronized configuration bundle to node %s", node.Name),
	})

	return nil
}

