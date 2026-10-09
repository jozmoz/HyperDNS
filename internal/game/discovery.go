package game

import (
	"fmt"
	"net"
	"regexp"
	"strings"
	"sync"
	"time"
)

var validHostnameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

// ObservationEvent carries telemetry for an observed DNS query.
type ObservationEvent struct {
	Hostname     string
	ClientIP     string
	ResolvedIPs  []string
	TTL          uint32
	ActiveGameID string
	Timestamp    time.Time
}

// DiscoveryAgent observes DNS requests, classifies unknown hostnames, and surfaces candidates.
type DiscoveryAgent struct {
	store   *Store
	eventCh chan ObservationEvent
	stopCh  chan struct{}
	mu      sync.RWMutex

	// In-memory frequency counters to avoid hitting DB on every packet
	freqMap map[string]*domainFreq
	freqMu  sync.Mutex
}

type domainFreq struct {
	count     uint64
	firstSeen time.Time
	lastSeen  time.Time
	lastIPs   []string
	lastTTL   uint32
}

// NewDiscoveryAgent creates a new DiscoveryAgent.
func NewDiscoveryAgent(store *Store) *DiscoveryAgent {
	agent := &DiscoveryAgent{
		store:   store,
		eventCh: make(chan ObservationEvent, 2048),
		stopCh:  make(chan struct{}),
		freqMap: make(map[string]*domainFreq),
	}
	go agent.workerLoop()
	return agent
}

// Stop terminates the background analysis worker.
func (a *DiscoveryAgent) Stop() {
	select {
	case <-a.stopCh:
		return
	default:
		close(a.stopCh)
	}
}

// ObserveQuery pushes an observed query into the async analysis queue without blocking DNS.
func (a *DiscoveryAgent) ObserveQuery(hostname, clientIP string, resolvedIPs []string, ttl uint32, activeGameID string) {
	if hostname == "" {
		return
	}
	event := ObservationEvent{
		Hostname:     normalizeDomain(hostname),
		ClientIP:     clientIP,
		ResolvedIPs:  resolvedIPs,
		TTL:          ttl,
		ActiveGameID: activeGameID,
		Timestamp:    time.Now(),
	}

	select {
	case a.eventCh <- event:
	default:
		// Queue full under extreme flood; drop safely without blocking DNS
	}
}

func (a *DiscoveryAgent) workerLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopCh:
			return
		case event := <-a.eventCh:
			a.processEvent(event)
		case <-ticker.C:
			// periodic flush / cleanup if needed
		}
	}
}

func (a *DiscoveryAgent) processEvent(e ObservationEvent) {
	host := e.Hostname
	if !isValidHostname(host) || isSystemOrInternal(host) {
		return
	}

	// Update in-memory frequency
	a.freqMu.Lock()
	f, exists := a.freqMap[host]
	if !exists {
		f = &domainFreq{
			count:     0,
			firstSeen: e.Timestamp,
			lastSeen:  e.Timestamp,
			lastIPs:   e.ResolvedIPs,
			lastTTL:   e.TTL,
		}
		a.freqMap[host] = f
	}
	f.count++
	f.lastSeen = e.Timestamp
	if len(e.ResolvedIPs) > 0 {
		f.lastIPs = e.ResolvedIPs
	}
	if e.TTL > 0 {
		f.lastTTL = e.TTL
	}
	currCount := f.count
	a.freqMu.Unlock()

	// Check if already in domain intelligence database
	existing, err := a.store.GetDomainIntel(host)
	if err == nil && existing != nil {
		// Existing confirmed/tracked domain: update observation stats
		existing.ObservationCount = currCount
		existing.LastSeen = e.Timestamp
		if len(e.ResolvedIPs) > 0 {
			// Detect unexpected IP changes
			if len(existing.LastResolvedIPs) > 0 && !equalIPs(existing.LastResolvedIPs, e.ResolvedIPs) {
				_ = a.store.AppendAuditLog(AuditEntry{
					Actor:      "agent",
					Action:     "ip_change_detected",
					TargetType: "domain",
					TargetID:   host,
					Details:    fmt.Sprintf("DNS IP response changed from %v to %v", existing.LastResolvedIPs, e.ResolvedIPs),
				})
			}
			existing.LastResolvedIPs = e.ResolvedIPs
		}
		if e.TTL > 0 {
			existing.LastTTL = e.TTL
		}
		_ = a.store.SaveDomainIntel(*existing)
		return
	}

	// Unknown domain: analyze heuristics
	profiles, _ := a.store.ListGameProfiles()
	gameID, publisher, cat, confidence, evidence := a.analyzeAssociation(host, e.ActiveGameID, currCount, profiles)
	if gameID == "" && confidence < 40 {
		// Not associated with any known gaming infrastructure
		return
	}

	// Look up candidate record
	candID := fmt.Sprintf("cand_%x", host)
	cand, err := a.store.GetCandidate(candID)
	if err == nil && cand != nil {
		// Update existing candidate
		cand.Count = currCount
		cand.LastSeen = e.Timestamp
		cand.ConfidenceScore = confidence
		cand.Evidence = evidence
		if len(e.ResolvedIPs) > 0 {
			cand.SampleIPs = e.ResolvedIPs
		}
		if e.TTL > 0 {
			cand.SampleTTL = e.TTL
		}
		_ = a.store.SaveCandidate(*cand)
	} else {
		// Create new candidate
		newCand := DiscoveryCandidate{
			ID:              candID,
			Hostname:        host,
			GameID:          gameID,
			Publisher:       publisher,
			Category:        cat,
			ConfidenceScore: confidence,
			Status:          StatusCandidate,
			Evidence:        evidence,
			FirstSeen:       e.Timestamp,
			LastSeen:        e.Timestamp,
			Count:           currCount,
			SampleIPs:       e.ResolvedIPs,
			SampleTTL:       e.TTL,
			ProposedPolicy:  PolicyDirect, // Safe default
			Reason:          strings.Join(evidence, "; "),
			UpdatedAt:       e.Timestamp,
		}
		_ = a.store.SaveCandidate(newCand)

		_ = a.store.AppendAuditLog(AuditEntry{
			Actor:      "agent",
			Action:     "candidate_discovered",
			TargetType: "domain",
			TargetID:   host,
			Details:    fmt.Sprintf("Discovered candidate for %s (Confidence: %d%%)", gameID, confidence),
		})
	}

	// Auto-Apply evaluation if learning settings allow
	settings, _ := a.store.GetLearningSettings()
	if settings != nil && settings.Mode == ModeAutoApply && confidence >= settings.AutoApplyMinScore {
		a.AutoPromoteCandidate(candID, "auto_agent")
	}
}

// AutoPromoteCandidate promotes a candidate into the confirmed Game Profile and Domain DB.
func (a *DiscoveryAgent) AutoPromoteCandidate(candidateID, actor string) error {
	cand, err := a.store.GetCandidate(candidateID)
	if err != nil {
		return err
	}

	if cand.GameID == "" {
		return fmt.Errorf("candidate has no associated game")
	}

	profile, err := a.store.GetGameProfile(cand.GameID)
	if err != nil {
		return err
	}

	now := time.Now()
	// Create domain record
	dom := DomainRecord{
		Hostname:         cand.Hostname,
		GameID:           cand.GameID,
		Publisher:        cand.Publisher,
		Category:         cand.Category,
		Policy:           cand.ProposedPolicy,
		ConfidenceScore:  cand.ConfidenceScore,
		Status:           StatusConfirmed,
		Evidence:         cand.Evidence,
		FirstSeen:        cand.FirstSeen,
		LastSeen:         now,
		ObservationCount: cand.Count,
		LastResolvedIPs:  cand.SampleIPs,
		LastTTL:          cand.SampleTTL,
		CreatedBy:        actor,
		UpdatedAt:        now,
		Enabled:          true,
	}

	if profile.Domains == nil {
		profile.Domains = make(map[string]DomainRecord)
	}
	profile.Domains[cand.Hostname] = dom
	profile.UpdateHistory = append(profile.UpdateHistory, ProfileHistoryEntry{
		Version:   len(profile.UpdateHistory) + 1,
		Timestamp: now,
		Actor:     actor,
		Summary:   fmt.Sprintf("Promoted candidate %s to %s (%s)", cand.Hostname, cand.Category, cand.ProposedPolicy),
	})

	if err := a.store.SaveGameProfile(*profile); err != nil {
		return err
	}
	if err := a.store.SaveDomainIntel(dom); err != nil {
		return err
	}
	_ = a.store.DeleteCandidate(candidateID)

	_ = a.store.AppendAuditLog(AuditEntry{
		Actor:      actor,
		Action:     "candidate_promoted",
		TargetType: "game",
		TargetID:   cand.GameID,
		Details:    fmt.Sprintf("Promoted %s to profile %s", cand.Hostname, cand.GameID),
	})

	return nil
}

// analyzeAssociation uses multi-signal heuristics to associate a hostname with a game and category.
func (a *DiscoveryAgent) analyzeAssociation(
	host, activeGameID string,
	count uint64,
	profiles []GameProfile,
) (gameID, publisher string, cat DomainCategory, score int, evidence []string) {
	score = 10
	evidence = make([]string, 0)
	cat = CategoryCandidate

	for _, p := range profiles {
		// 1. Root domain matching
		for _, root := range p.DiscoverySettings.KnownRootDomains {
			if host == root || strings.HasSuffix(host, "."+root) {
				gameID = p.ID
				publisher = p.Publisher
				score += 40
				evidence = append(evidence, fmt.Sprintf("Matches known %s root domain: %s", p.Name, root))
				break
			}
		}

		// 2. Keyword matching
		for _, kw := range p.DiscoverySettings.KeywordPatterns {
			if strings.Contains(host, kw) {
				if gameID == "" {
					gameID = p.ID
					publisher = p.Publisher
				}
				score += 25
				evidence = append(evidence, fmt.Sprintf("Contains game keyword pattern: %q", kw))
				break
			}
		}

		if gameID != "" {
			break
		}
	}

	// 3. Co-occurrence with active game
	if activeGameID != "" {
		if gameID == activeGameID {
			score += 20
			evidence = append(evidence, fmt.Sprintf("Co-occurred during active %s gaming session", activeGameID))
		} else if gameID == "" {
			// Weak association as candidate
			score += 15
			evidence = append(evidence, fmt.Sprintf("Observed during %s traffic burst", activeGameID))
		}
	}

	// 4. Persistence / frequency
	if count > 5 {
		score += 10
		evidence = append(evidence, fmt.Sprintf("Persistent traffic pattern (%d observations)", count))
	}

	// 5. Categorization heuristics
	cat = inferCategory(host)
	evidence = append(evidence, fmt.Sprintf("Inferred category: %s", cat))

	if score > 100 {
		score = 100
	}
	return gameID, publisher, cat, score, evidence
}

func inferCategory(host string) DomainCategory {
	h := strings.ToLower(host)
	switch {
	case strings.Contains(h, "auth") || strings.Contains(h, "login") || strings.Contains(h, "signin") ||
		strings.Contains(h, "identity") || strings.Contains(h, "account") || strings.Contains(h, "token"):
		return CategoryAuth

	case strings.Contains(h, "matchmaking") || strings.Contains(h, "mm") || strings.Contains(h, "lobby") ||
		strings.Contains(h, "server") || strings.Contains(h, "relay") || strings.Contains(h, "game") ||
		strings.Contains(h, "session") || strings.Contains(h, "utas") || strings.Contains(h, "fut"):
		return CategoryMatchmaking

	case strings.Contains(h, "download") || strings.Contains(h, "cdn") || strings.Contains(h, "content") ||
		strings.Contains(h, "patch") || strings.Contains(h, "update") || strings.Contains(h, "akamai") ||
		strings.Contains(h, "yupmaster"):
		return CategoryCDN

	case strings.Contains(h, "telemetry") || strings.Contains(h, "stat") || strings.Contains(h, "metric") ||
		strings.Contains(h, "crash") || strings.Contains(h, "report") || strings.Contains(h, "analytics") ||
		strings.Contains(h, "pin"):
		return CategoryTelemetry

	default:
		return CategoryGameServices
	}
}

func isValidHostname(h string) bool {
	if len(h) == 0 || len(h) > 253 {
		return false
	}
	return validHostnameRegex.MatchString(h)
}

func isSystemOrInternal(h string) bool {
	if h == "localhost" || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".internal") ||
		strings.HasSuffix(h, ".in-addr.arpa") || strings.HasSuffix(h, ".ip6.arpa") {
		return true
	}
	if net.ParseIP(h) != nil {
		return true
	}
	return false
}

func equalIPs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]bool, len(a))
	for _, ip := range a {
		m[ip] = true
	}
	for _, ip := range b {
		if !m[ip] {
			return false
		}
	}
	return true
}

