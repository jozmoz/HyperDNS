package game

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type querySample struct {
	hostname  string
	timestamp time.Time
}

// ActiveGameDetector analyzes recent DNS query bursts to infer the currently active game.
type ActiveGameDetector struct {
	store   *Store
	mu      sync.RWMutex
	samples []querySample
	maxAge  time.Duration
}

// NewActiveGameDetector creates an ActiveGameDetector with a 60-second analysis window.
func NewActiveGameDetector(store *Store) *ActiveGameDetector {
	return &ActiveGameDetector{
		store:   store,
		samples: make([]querySample, 0, 500),
		maxAge:  60 * time.Second,
	}
}

// RecordQuery adds a queried hostname to the rolling sample window.
func (d *ActiveGameDetector) RecordQuery(hostname string) {
	if hostname == "" {
		return
	}
	norm := normalizeDomain(hostname)
	now := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	d.samples = append(d.samples, querySample{hostname: norm, timestamp: now})

	// Trim expired samples
	cutoff := now.Add(-d.maxAge)
	validStart := 0
	for i, s := range d.samples {
		if s.timestamp.After(cutoff) {
			validStart = i
			break
		}
	}
	if validStart > 0 {
		d.samples = d.samples[validStart:]
	}
}

// DetectActiveGame evaluates the sliding query window and returns the most probable active game.
func (d *ActiveGameDetector) DetectActiveGame() *DetectedGame {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.samples) == 0 {
		return nil
	}

	profiles, err := d.store.ListGameProfiles()
	if err != nil || len(profiles) == 0 {
		return nil
	}

	type gameScore struct {
		profile      GameProfile
		specificHits int
		generalHits  int
		matchedHosts []string
	}

	scores := make(map[string]*gameScore)
	for _, p := range profiles {
		if p.Enabled {
			scores[p.ID] = &gameScore{profile: p}
		}
	}

	for _, s := range d.samples {
		for _, p := range profiles {
			gs := scores[p.ID]
			if gs == nil {
				continue
			}

			// Check domain match in profile
			if dom, ok := p.Domains[s.hostname]; ok {
				if dom.Category == CategoryMatchmaking {
					gs.specificHits += 2
					gs.matchedHosts = append(gs.matchedHosts, s.hostname)
				} else if dom.Category == CategoryAuth || dom.Category == CategoryGameServices {
					gs.generalHits++
				}
				continue
			}

			// Check keyword pattern
			for _, kw := range p.DiscoverySettings.KeywordPatterns {
				if strings.Contains(s.hostname, kw) {
					gs.specificHits++
					gs.matchedHosts = append(gs.matchedHosts, s.hostname)
					break
				}
			}
		}
	}

	var bestGame *gameScore
	bestTotal := 0

	for _, gs := range scores {
		total := (gs.specificHits * 25) + (gs.generalHits * 8)
		if total > bestTotal {
			bestTotal = total
			bestGame = gs
		}
	}

	// Threshold check: need at least some specific evidence
	if bestGame == nil || bestTotal < 30 {
		return nil
	}

	confidence := 45 + (bestGame.specificHits * 15) + (bestGame.generalHits * 3)
	if confidence > 95 {
		confidence = 95
	}

	// Remove duplicate matched hostnames for clean evidence presentation
	uniqueHosts := make([]string, 0, len(bestGame.matchedHosts))
	seen := make(map[string]bool)
	for _, h := range bestGame.matchedHosts {
		if !seen[h] && len(uniqueHosts) < 3 {
			seen[h] = true
			uniqueHosts = append(uniqueHosts, h)
		}
	}

	evidence := fmt.Sprintf("%s services + %d queries to endpoints (%s)",
		bestGame.profile.Publisher,
		bestGame.specificHits+bestGame.generalHits,
		strings.Join(uniqueHosts, ", "),
	)

	return &DetectedGame{
		GameID:          bestGame.profile.ID,
		GameName:        bestGame.profile.Name,
		Publisher:       bestGame.profile.Publisher,
		Confidence:      confidence,
		Evidence:        evidence,
		DetectedAt:      time.Now(),
		SuggestedAction: fmt.Sprintf("Activate %s profile route policies", bestGame.profile.Name),
	}
}
