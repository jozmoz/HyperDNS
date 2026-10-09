package game

import (
	"fmt"
	"strings"

	"hyperdns/internal/core/matcher"
)

// RuleSimulator provides What-If simulation and blast-radius conflict analysis.
type RuleSimulator struct {
	store   *Store
	matcher *matcher.Matcher
}

// NewRuleSimulator creates a new RuleSimulator.
func NewRuleSimulator(store *Store, m *matcher.Matcher) *RuleSimulator {
	return &RuleSimulator{
		store:   store,
		matcher: m,
	}
}

// Simulate evaluates the resolution chain for a domain and detects conflicts with proposed policy.
func (rs *RuleSimulator) Simulate(req RuleSimulationRequest) (*RuleSimulationResult, error) {
	domain := normalizeDomain(req.Domain)
	if domain == "" {
		return nil, fmt.Errorf("domain cannot be empty")
	}

	// 1. Resolve current effective policy using real Matcher engine
	var currentAction matcher.Action
	var matchedRule string

	if rs.matcher != nil {
		currentAction, matchedRule = rs.matcher.Match(domain)
	} else {
		currentAction = matcher.ActionDirect
		matchedRule = "Direct (Default)"
	}

	currentPolicy := PolicyDirect
	switch currentAction {
	case matcher.ActionProxy:
		currentPolicy = PolicyProxy
	case matcher.ActionBlock:
		currentPolicy = PolicyBlock
	default:
		currentPolicy = PolicyDirect
	}

	proposedPolicy := req.ProposedPolicy
	if proposedPolicy == "" {
		proposedPolicy = currentPolicy
	}

	conflict := false
	reasons := make([]string, 0)
	warnings := make([]string, 0)

	// 2. Conflict Detection
	if matchedRule == matcher.RuleCustomBlock && proposedPolicy != PolicyBlock {
		conflict = true
		reasons = append(reasons, "Domain is explicitly blocked by Global Custom Block (Highest administrative priority)")
	} else if matchedRule == matcher.RuleCustomDirect && proposedPolicy != PolicyDirect {
		conflict = true
		reasons = append(reasons, "Domain is explicitly whitelisted by Global Custom Direct rule")
	} else if matchedRule == matcher.RuleRealtimeDirect && proposedPolicy == PolicyProxy {
		conflict = true
		reasons = append(reasons, "Domain belongs to Realtime Media forced-direct plane; proxying would break UDP voice/video transport")
	}

	// 3. Broad Rule & Blast Radius Analysis
	cleanDom := strings.TrimPrefix(domain, "*.")
	sharedRoots := map[string]string{
		"ea.com":            "Electronic Arts (FC 26, Apex Legends, Battlefield)",
		"steampowered.com":  "Valve Steam (Store, Community, CS2, Dota 2)",
		"valvesoftware.com": "Valve Corporation infrastructure",
		"gaijin.net":        "Gaijin Entertainment (War Thunder, Enlisted)",
		"riotgames.com":     "Riot Games (Valorant, League of Legends)",
		"epicgames.com":     "Epic Games (Fortnite, Unreal Engine)",
		"blizzard.com":      "Blizzard Entertainment (Battle.net, WoW, Overwatch)",
	}

	for root, desc := range sharedRoots {
		if cleanDom == root {
			if proposedPolicy == PolicyBlock {
				warnings = append(warnings, fmt.Sprintf("CRITICAL: Blocking root domain %q affects all %s services and games!", root, desc))
			} else {
				warnings = append(warnings, fmt.Sprintf("Caution: %q is a shared publisher domain for %s", root, desc))
			}
			break
		}
	}

	// Find affected domains in Domain Intelligence DB
	allDomains, _ := rs.store.ListDomainIntel("", "", "")
	affected := make([]string, 0)
	for _, d := range allDomains {
		if d.Hostname == cleanDom || strings.HasSuffix(d.Hostname, "."+cleanDom) {
			affected = append(affected, d.Hostname)
		}
	}

	if len(affected) > 5 && proposedPolicy == PolicyBlock {
		warnings = append(warnings, fmt.Sprintf("High Blast Radius: Policy change will affect %d indexed subdomains", len(affected)))
	}

	effectivePolicy := proposedPolicy
	if conflict {
		effectivePolicy = currentPolicy
		if len(reasons) == 0 {
			reasons = append(reasons, "Rule conflict prevented application of proposed policy")
		}
	} else {
		reasons = append(reasons, fmt.Sprintf("Rule applies cleanly; current rule: %s", matchedRule))
	}

	return &RuleSimulationResult{
		Domain:           domain,
		CurrentPolicy:    currentPolicy,
		ProposedPolicy:   proposedPolicy,
		EffectivePolicy:  effectivePolicy,
		MatchedRule:      matchedRule,
		Conflict:         conflict,
		Reason:           strings.Join(reasons, "; "),
		Warnings:         warnings,
		BlastRadiusCount: len(affected),
		AffectedDomains:  affected,
	}, nil
}

