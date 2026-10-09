package game

import (
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"sort"
	"sync"
	"time"
)

// RouteManager handles route quality testing, scoring, anti-flapping hysteresis, and failover.
type RouteManager struct {
	store        *Store
	mu           sync.RWMutex
	measurements []RouteMeasurement
	measMu       sync.RWMutex
	stopCh       chan struct{}

	// Anti-flapping state: nodeID -> consecutive cycles candidate was better
	candidateStreak map[string]int
}

// NewRouteManager creates a new RouteManager and starts the background scheduler.
func NewRouteManager(store *Store) *RouteManager {
	rm := &RouteManager{
		store:           store,
		measurements:    make([]RouteMeasurement, 0, 1000),
		stopCh:          make(chan struct{}),
		candidateStreak: make(map[string]int),
	}
	go rm.schedulerLoop()
	return rm
}

// Stop stops the background scheduler.
func (rm *RouteManager) Stop() {
	select {
	case <-rm.stopCh:
	default:
		close(rm.stopCh)
	}
}

func (rm *RouteManager) schedulerLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-rm.stopCh:
			return
		case <-ticker.C:
			rm.ProbeAllNodes()
		}
	}
}

// ProbeAllNodes triggers quality diagnostics on all registered route nodes.
func (rm *RouteManager) ProbeAllNodes() []RouteNode {
	nodes, err := rm.store.ListRouteNodes()
	if err != nil || len(nodes) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	results := make([]RouteNode, len(nodes))

	for i, n := range nodes {
		wg.Add(1)
		go func(idx int, node RouteNode) {
			defer wg.Done()
			updated := rm.ProbeNode(node)
			_ = rm.store.SaveRouteNode(updated)
			results[idx] = updated
		}(i, n)
	}
	wg.Wait()

	// Evaluate failover for game profiles
	rm.EvaluateFailover(results)

	return results
}

// ProbeNode measures network quality metrics (DNS, TCP, TLS, RTT, Loss, Jitter) for a single node.
func (rm *RouteManager) ProbeNode(node RouteNode) RouteNode {
	start := time.Now()
	targetHost := node.Endpoint
	port := node.Port
	if port <= 0 {
		port = 443
	}

	// 1. DNS Resolution Time
	dnsStart := time.Now()
	ips, err := net.LookupHost(targetHost)
	dnsTime := float64(time.Since(dnsStart).Microseconds()) / 1000.0

	var tcpTime, tlsTime float64
	var rtts []float64
	lossCount := 0
	probes := 3

	if err != nil || len(ips) == 0 {
		node.Status = NodeDown
		node.RTTMs = 0
		node.PacketLossPct = 100.0
		node.JitterMs = 0
		node.StabilityScore = 0
		node.LastCheck = start

		rm.recordMeasurement(RouteMeasurement{
			ID:        fmt.Sprintf("meas_%d_%s", time.Now().UnixNano(), node.ID),
			Timestamp: start,
			NodeID:    node.ID,
			Target:    targetHost,
			LossPct:   100.0,
			Success:   false,
			Error:     fmt.Sprintf("DNS lookup failed: %v", err),
		})
		return node
	}

	// 2. TCP Connect Time (run probes to compute RTT, jitter, loss)
	targetAddr := fmt.Sprintf("%s:%d", ips[0], port)
	for i := 0; i < probes; i++ {
		tcpStart := time.Now()
		conn, dialErr := net.DialTimeout("tcp", targetAddr, 1500*time.Millisecond)
		elapsed := float64(time.Since(tcpStart).Microseconds()) / 1000.0
		if dialErr != nil {
			lossCount++
		} else {
			rtts = append(rtts, elapsed)
			if i == 0 {
				tcpTime = elapsed
			}
			_ = conn.Close()
		}
		time.Sleep(30 * time.Millisecond)
	}

	// 3. TLS Handshake Time (if port 443)
	if port == 443 && len(rtts) > 0 {
		tlsStart := time.Now()
		tlsConn, tlsErr := tls.DialWithDialer(
			&net.Dialer{Timeout: 2 * time.Second},
			"tcp",
			targetAddr,
			&tls.Config{InsecureSkipVerify: true, ServerName: targetHost},
		)
		if tlsErr == nil {
			tlsTime = float64(time.Since(tlsStart).Microseconds()) / 1000.0
			_ = tlsConn.Close()
		}
	}

	// Compute average RTT, Jitter, Packet Loss
	var avgRTT, jitter float64
	lossPct := (float64(lossCount) / float64(probes)) * 100.0

	if len(rtts) > 0 {
		var sum float64
		for _, r := range rtts {
			sum += r
		}
		avgRTT = sum / float64(len(rtts))

		// Jitter = mean absolute difference of successive RTTs
		if len(rtts) > 1 {
			var diffSum float64
			for j := 1; j < len(rtts); j++ {
				diffSum += math.Abs(rtts[j] - rtts[j-1])
			}
			jitter = diffSum / float64(len(rtts)-1)
		}
	}

	// Calculate Stability Score: 100 - (Loss * 10) - (RTT * 0.2) - (Jitter * 0.5)
	stability := 100.0 - (lossPct * 10.0) - (avgRTT * 0.2) - (jitter * 0.5)
	if stability < 0 {
		stability = 0
	}
	if stability > 100 {
		stability = 100
	}

	// Classify Status
	status := NodeHealthy
	if lossPct >= 10.0 || avgRTT >= 200.0 || len(rtts) == 0 {
		status = NodeDown
	} else if lossPct > 2.0 || avgRTT > 100.0 || jitter > 15.0 || stability < 70.0 {
		status = NodeDegraded
	}

	node.RTTMs = math.Round(avgRTT*10) / 10
	node.PacketLossPct = math.Round(lossPct*10) / 10
	node.JitterMs = math.Round(jitter*10) / 10
	node.DNSTimeMs = math.Round(dnsTime*10) / 10
	node.TCPTimeMs = math.Round(tcpTime*10) / 10
	node.TLSTimeMs = math.Round(tlsTime*10) / 10
	node.StabilityScore = math.Round(stability*10) / 10
	node.Status = status
	node.LastCheck = start

	rm.recordMeasurement(RouteMeasurement{
		ID:        fmt.Sprintf("meas_%d_%s", time.Now().UnixNano(), node.ID),
		Timestamp: start,
		NodeID:    node.ID,
		Target:    targetHost,
		RTTMs:     node.RTTMs,
		LossPct:   node.PacketLossPct,
		JitterMs:  node.JitterMs,
		DNSTimeMs: node.DNSTimeMs,
		TCPTimeMs: node.TCPTimeMs,
		TLSTimeMs: node.TLSTimeMs,
		Success:   status != NodeDown,
	})

	return node
}

func (rm *RouteManager) recordMeasurement(m RouteMeasurement) {
	rm.measMu.Lock()
	defer rm.measMu.Unlock()

	rm.measurements = append(rm.measurements, m)
	if len(rm.measurements) > 1000 {
		rm.measurements = rm.measurements[len(rm.measurements)-1000:]
	}
}

// GetMeasurements returns recorded measurements up to limit.
func (rm *RouteManager) GetMeasurements(limit int) []RouteMeasurement {
	rm.measMu.RLock()
	defer rm.measMu.RUnlock()

	if limit <= 0 || limit > len(rm.measurements) {
		limit = len(rm.measurements)
	}
	res := make([]RouteMeasurement, limit)
	copy(res, rm.measurements[len(rm.measurements)-limit:])
	sort.Slice(res, func(i, j int) bool {
		return res[i].Timestamp.After(res[j].Timestamp)
	})
	return res
}

// EvaluateFailover inspects game profiles and performs auto-failover if current route degrades.
func (rm *RouteManager) EvaluateFailover(nodes []RouteNode) {
	nodeMap := make(map[string]RouteNode, len(nodes))
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	profiles, err := rm.store.ListGameProfiles()
	if err != nil {
		return
	}

	for _, p := range profiles {
		if !p.RoutePolicy.AutoSelectEnabled || p.RoutePolicy.PreferredNodeID == "" {
			continue
		}

		current, ok := nodeMap[p.RoutePolicy.PreferredNodeID]
		if !ok {
			continue
		}

		// Check if current node violated threshold
		needsFailover := false
		reason := ""

		if current.Status == NodeDown {
			needsFailover = true
			reason = fmt.Sprintf("Primary route %s is Down (Loss: %.1f%%)", current.Name, current.PacketLossPct)
		} else if current.Status == NodeDegraded {
			if p.RoutePolicy.MaxLossThresholdPct > 0 && current.PacketLossPct > p.RoutePolicy.MaxLossThresholdPct {
				needsFailover = true
				reason = fmt.Sprintf("Packet loss %.1f%% exceeded threshold %.1f%%", current.PacketLossPct, p.RoutePolicy.MaxLossThresholdPct)
			} else if p.RoutePolicy.MaxRTTThresholdMs > 0 && current.RTTMs > p.RoutePolicy.MaxRTTThresholdMs {
				needsFailover = true
				reason = fmt.Sprintf("Latency %.1fms exceeded threshold %.1fms", current.RTTMs, p.RoutePolicy.MaxRTTThresholdMs)
			}
		}

		if needsFailover {
			// Find best fallback node with anti-flapping hysteresis
			var bestCandidate *RouteNode
			for _, fbID := range p.RoutePolicy.FallbackNodeIDs {
				fb, fbOk := nodeMap[fbID]
				if fbOk && fb.Status == NodeHealthy && fb.Active {
					if bestCandidate == nil || fb.StabilityScore > bestCandidate.StabilityScore {
						nodeCopy := fb
						bestCandidate = &nodeCopy
					}
				}
			}

			if bestCandidate != nil {
				// Anti-flapping check: candidate must be ahead for at least 1 cycle
				rm.mu.Lock()
				streak := rm.candidateStreak[bestCandidate.ID] + 1
				rm.candidateStreak[bestCandidate.ID] = streak
				rm.mu.Unlock()

				if streak >= 1 {
					// Verify pre-activation health probe before switching
					verified := rm.ProbeNode(*bestCandidate)
					if verified.Status == NodeHealthy {
						oldNodeID := p.RoutePolicy.PreferredNodeID
						p.RoutePolicy.PreferredNodeID = verified.ID
						_ = rm.store.SaveGameProfile(p)

						_ = rm.store.AppendAuditLog(AuditEntry{
							Actor:      "smart_route_selector",
							Action:     "route_failover_triggered",
							TargetType: "game",
							TargetID:   p.ID,
							Details:    fmt.Sprintf("Failed over %s from %s to %s. Reason: %s", p.Name, oldNodeID, verified.ID, reason),
						})
					}
				}
			}
		} else {
			// Current node is healthy; reset streaks
			rm.mu.Lock()
			rm.candidateStreak = make(map[string]int)
			rm.mu.Unlock()
		}
	}
}

// SelectRoute manually overrides preferred route for a game profile.
func (rm *RouteManager) SelectRoute(gameID, nodeID, actor string) error {
	p, err := rm.store.GetGameProfile(gameID)
	if err != nil {
		return err
	}

	node, err := rm.store.GetRouteNode(nodeID)
	if err != nil {
		return err
	}

	oldNode := p.RoutePolicy.PreferredNodeID
	p.RoutePolicy.PreferredNodeID = node.ID
	p.UpdatedAt = time.Now()

	if err := rm.store.SaveGameProfile(*p); err != nil {
		return err
	}

	_ = rm.store.AppendAuditLog(AuditEntry{
		Actor:      actor,
		Action:     "manual_route_selection",
		TargetType: "game",
		TargetID:   gameID,
		Details:    fmt.Sprintf("Manually changed route for %s from %s to %s", p.Name, oldNode, node.Name),
	})

	return nil
}

