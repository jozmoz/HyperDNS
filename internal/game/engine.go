package game

import (
	"encoding/json"
	"fmt"
	"time"

	"hyperdns/internal/core/matcher"
	"hyperdns/internal/database"
)

// Engine is the central coordinator for HyperDNS Game Intelligence (HGI).
type Engine struct {
	store      *Store
	agent      *DiscoveryAgent
	learning   *LearningManager
	routes     *RouteManager
	detector   *ActiveGameDetector
	simulator  *RuleSimulator
	timeline   *TimelineManager
	ai         *AIAssistant
	controller *MultiNodeController
	matcher    *matcher.Matcher
}

// NewEngine creates and initializes the complete HGI engine.
func NewEngine(db *database.DB, m *matcher.Matcher) (*Engine, error) {
	store := NewStore(db)
	if err := store.InitSeeds(); err != nil {
		return nil, fmt.Errorf("failed to initialize HGI seed data: %w", err)
	}

	agent := NewDiscoveryAgent(store)
	learning := NewLearningManager(store, agent)
	routes := NewRouteManager(store)
	detector := NewActiveGameDetector(store)
	simulator := NewRuleSimulator(store, m)
	timeline := NewTimelineManager(2000, false, 7)
	ai := NewAIAssistant(store)
	controller := NewMultiNodeController(store, routes)

	eng := &Engine{
		store:      store,
		agent:      agent,
		learning:   learning,
		routes:     routes,
		detector:   detector,
		simulator:  simulator,
		timeline:   timeline,
		ai:         ai,
		controller: controller,
		matcher:    m,
	}

	// Register seeds into Matcher
	eng.syncProfilesToMatcher()

	return eng, nil
}

// Close gracefully terminates all background workers and timers.
func (e *Engine) Close() {
	if e.agent != nil {
		e.agent.Stop()
	}
	if e.routes != nil {
		e.routes.Stop()
	}
	if e.timeline != nil {
		e.timeline.Stop()
	}
}

// ObserveDNSQuery is the fast, non-blocking telemetry hook called from dns.Handler.
func (e *Engine) ObserveDNSQuery(hostname, clientIP string, resolvedIPs []string, ttl uint32, action, ruleMatched string, latencyMs float64) {
	if hostname == "" {
		return
	}

	// 1. Feed query into sliding window for real-time game detection
	e.detector.RecordQuery(hostname)

	// 2. Identify active game
	activeGameID := ""
	if active := e.detector.DetectActiveGame(); active != nil {
		activeGameID = active.GameID
	}

	// 3. Hand off to async discovery worker
	e.agent.ObserveQuery(hostname, clientIP, resolvedIPs, ttl, activeGameID)

	// 4. Record to traffic timeline
	e.timeline.RecordEvent("query_observed", activeGameID, clientIP, hostname, action, ruleMatched, "", latencyMs)
}

func (e *Engine) syncProfilesToMatcher() {
	profiles, err := e.store.ListGameProfiles()
	if err != nil {
		return
	}

	// Rebuild custom rules in Matcher including game profile policies
	customProxied := make([]string, 0)
	customBlocked := make([]string, 0)
	customDirect := make([]string, 0)

	for _, p := range profiles {
		if !p.Enabled {
			continue
		}
		for host, d := range p.Domains {
			if !d.Enabled {
				continue
			}
			switch d.Policy {
			case PolicyProxy:
				customProxied = append(customProxied, host)
			case PolicyBlock:
				customBlocked = append(customBlocked, host)
			case PolicyDirect:
				customDirect = append(customDirect, host)
			}
		}
	}

	if e.matcher != nil {
		e.matcher.SetCustomRules(customProxied, customBlocked, customDirect, nil)
	}
}

// --- High Level API Methods ---

// Store returns the underlying persistent store.
func (e *Engine) Store() *Store {
	return e.store
}

// Routes returns the RouteManager.
func (e *Engine) Routes() *RouteManager {
	return e.routes
}

// Learning returns the LearningManager.
func (e *Engine) Learning() *LearningManager {
	return e.learning
}

// Detector returns the ActiveGameDetector.
func (e *Engine) Detector() *ActiveGameDetector {
	return e.detector
}

// Simulator returns the RuleSimulator.
func (e *Engine) Simulator() *RuleSimulator {
	return e.simulator
}

// Timeline returns the TimelineManager.
func (e *Engine) Timeline() *TimelineManager {
	return e.timeline
}

// AI returns the AIAssistant.
func (e *Engine) AI() *AIAssistant {
	return e.ai
}

// Controller returns the MultiNodeController.
func (e *Engine) Controller() *MultiNodeController {
	return e.controller
}

// ListGameProfiles returns all configured game profiles.
func (e *Engine) ListGameProfiles() ([]GameProfile, error) {
	return e.store.ListGameProfiles()
}

// GetGameProfile returns a game profile by ID.
func (e *Engine) GetGameProfile(id string) (*GameProfile, error) {
	return e.store.GetGameProfile(id)
}

// SaveGameProfile saves a game profile with rollback snapshot.
func (e *Engine) SaveGameProfile(p GameProfile, actor string) error {
	existing, _ := e.store.GetGameProfile(p.ID)
	now := time.Now()

	var snapshot string
	if existing != nil {
		if snapData, err := json.Marshal(existing); err == nil {
			snapshot = string(snapData)
		}
	}

	p.UpdateHistory = append(p.UpdateHistory, ProfileHistoryEntry{
		Version:   len(p.UpdateHistory) + 1,
		Timestamp: now,
		Actor:     actor,
		Summary:   fmt.Sprintf("Profile updated by %s", actor),
		Snapshot:  snapshot,
	})

	if err := e.store.SaveGameProfile(p); err != nil {
		return err
	}

	e.syncProfilesToMatcher()

	_ = e.store.AppendAuditLog(AuditEntry{
		Actor:      actor,
		Action:     "game_profile_saved",
		TargetType: "game",
		TargetID:   p.ID,
		Details:    fmt.Sprintf("Saved profile %s (%d domains)", p.Name, len(p.Domains)),
	})

	return nil
}

// DeleteGameProfile removes a profile and refreshes the matcher.
func (e *Engine) DeleteGameProfile(id, actor string) error {
	p, err := e.store.GetGameProfile(id)
	if err != nil {
		return err
	}

	if err := e.store.DeleteGameProfile(id); err != nil {
		return err
	}

	e.syncProfilesToMatcher()

	_ = e.store.AppendAuditLog(AuditEntry{
		Actor:      actor,
		Action:     "game_profile_deleted",
		TargetType: "game",
		TargetID:   id,
		Details:    fmt.Sprintf("Deleted game profile %s", p.Name),
	})

	return nil
}

// RollbackGameProfile restores a profile from an earlier snapshot.
func (e *Engine) RollbackGameProfile(id string, targetVersion int, actor string) error {
	p, err := e.store.GetGameProfile(id)
	if err != nil {
		return err
	}

	var targetSnapshot string
	for _, h := range p.UpdateHistory {
		if h.Version == targetVersion && h.Snapshot != "" {
			targetSnapshot = h.Snapshot
			break
		}
	}

	if targetSnapshot == "" {
		return fmt.Errorf("no rollback snapshot found for version %d", targetVersion)
	}

	var restored GameProfile
	if err := json.Unmarshal([]byte(targetSnapshot), &restored); err != nil {
		return fmt.Errorf("corrupt snapshot: %w", err)
	}

	now := time.Now()
	restored.UpdateHistory = append(p.UpdateHistory, ProfileHistoryEntry{
		Version:   len(p.UpdateHistory) + 1,
		Timestamp: now,
		Actor:     actor,
		Summary:   fmt.Sprintf("Rolled back to version %d", targetVersion),
	})

	if err := e.store.SaveGameProfile(restored); err != nil {
		return err
	}

	e.syncProfilesToMatcher()

	_ = e.store.AppendAuditLog(AuditEntry{
		Actor:      actor,
		Action:     "game_profile_rollback",
		TargetType: "game",
		TargetID:   id,
		Details:    fmt.Sprintf("Rolled back profile %s to version %d", p.Name, targetVersion),
	})

	return nil
}

// AddGameDomain adds a domain to an existing game profile.
func (e *Engine) AddGameDomain(gameID, hostname string, category DomainCategory, policy PolicyAction, actor string) error {
	p, err := e.store.GetGameProfile(gameID)
	if err != nil {
		return err
	}

	norm := normalizeDomain(hostname)
	now := time.Now()

	dom := DomainRecord{
		Hostname:         norm,
		GameID:           gameID,
		Publisher:        p.Publisher,
		Category:         category,
		Policy:           policy,
		ConfidenceScore:  100,
		Status:           StatusConfirmed,
		Evidence:         []string{fmt.Sprintf("Manually added by %s", actor)},
		FirstSeen:        now,
		LastSeen:         now,
		ObservationCount: 1,
		CreatedBy:        actor,
		UpdatedAt:        now,
		Enabled:          true,
	}

	if p.Domains == nil {
		p.Domains = make(map[string]DomainRecord)
	}
	p.Domains[norm] = dom

	if err := e.SaveGameProfile(*p, actor); err != nil {
		return err
	}
	return e.store.SaveDomainIntel(dom)
}

// RemoveGameDomain removes a domain from a game profile.
func (e *Engine) RemoveGameDomain(gameID, hostname, actor string) error {
	p, err := e.store.GetGameProfile(gameID)
	if err != nil {
		return err
	}

	norm := normalizeDomain(hostname)
	delete(p.Domains, norm)

	if err := e.SaveGameProfile(*p, actor); err != nil {
		return err
	}
	return e.store.DeleteDomainIntel(norm)
}

