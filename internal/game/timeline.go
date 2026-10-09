package game

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// TimelineManager manages real-time and persistent game traffic events with privacy controls.
type TimelineManager struct {
	events      []GameTimelineEvent
	mu          sync.RWMutex
	maxCapacity int
	anonymize   bool
	retention   time.Duration
	stopCh      chan struct{}
}

// NewTimelineManager creates a new TimelineManager.
func NewTimelineManager(maxCapacity int, anonymize bool, retentionDays int) *TimelineManager {
	if maxCapacity <= 0 {
		maxCapacity = 2000
	}
	if retentionDays <= 0 {
		retentionDays = 7
	}

	tm := &TimelineManager{
		events:      make([]GameTimelineEvent, 0, maxCapacity),
		maxCapacity: maxCapacity,
		anonymize:   anonymize,
		retention:   time.Duration(retentionDays) * 24 * time.Hour,
		stopCh:      make(chan struct{}),
	}
	go tm.cleanupLoop()
	return tm
}

// Stop ends the cleanup sweeper.
func (tm *TimelineManager) Stop() {
	select {
	case <-tm.stopCh:
	default:
		close(tm.stopCh)
	}
}

func (tm *TimelineManager) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-tm.stopCh:
			return
		case <-ticker.C:
			tm.purgeExpired()
		}
	}
}

func (tm *TimelineManager) purgeExpired() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	cutoff := time.Now().Add(-tm.retention)
	validStart := 0
	for i, ev := range tm.events {
		if ev.Timestamp.After(cutoff) {
			validStart = i
			break
		}
	}
	if validStart > 0 {
		tm.events = tm.events[validStart:]
	}
}

// RecordEvent appends an event to the timeline.
func (tm *TimelineManager) RecordEvent(eventType, gameID, clientIP, hostname, policy, ruleName, details string, latencyMs float64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	effectiveIP := clientIP
	if tm.anonymize && effectiveIP != "" {
		effectiveIP = anonymizeIP(effectiveIP)
	}

	event := GameTimelineEvent{
		ID:        fmt.Sprintf("ev_%d_%d", time.Now().UnixNano(), len(tm.events)),
		Timestamp: time.Now(),
		EventType: eventType,
		GameID:    gameID,
		ClientIP:  effectiveIP,
		Hostname:  hostname,
		Policy:    policy,
		RuleName:  ruleName,
		Details:   details,
		LatencyMs: latencyMs,
	}

	tm.events = append(tm.events, event)
	if len(tm.events) > tm.maxCapacity {
		tm.events = tm.events[len(tm.events)-tm.maxCapacity:]
	}
}

// GetEvents returns filtered events matching criteria.
func (tm *TimelineManager) GetEvents(gameID, clientIP, hostname string, limit int) []GameTimelineEvent {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	results := make([]GameTimelineEvent, 0, limit)
	// Iterate in reverse (newest first)
	for i := len(tm.events) - 1; i >= 0; i-- {
		ev := tm.events[i]
		if gameID != "" && ev.GameID != gameID {
			continue
		}
		if clientIP != "" && !strings.Contains(ev.ClientIP, clientIP) {
			continue
		}
		if hostname != "" && !strings.Contains(ev.Hostname, hostname) {
			continue
		}
		results = append(results, ev)
		if len(results) >= limit {
			break
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})
	return results
}

// ExportJSON exports events as formatted JSON.
func (tm *TimelineManager) ExportJSON() ([]byte, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return json.MarshalIndent(tm.events, "", "  ")
}

func anonymizeIP(ipStr string) string {
	parts := strings.Split(ipStr, ".")
	if len(parts) == 4 {
		return fmt.Sprintf("%s.%s.xxx.xxx", parts[0], parts[1])
	}
	return "xxxx:xxxx:xxxx:xxxx"
}

