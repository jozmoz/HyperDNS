package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hyperdns/internal/game"
)

// normalizeDomain helper ensures domain strings are trimmed and lowercased.
func normalizeDomain(domain string) string {
	d := strings.TrimSpace(strings.ToLower(domain))
	d = strings.TrimSuffix(d, ".")
	return d
}

// normalizePolicy maps string to valid game.PolicyAction.
func normalizePolicy(p string) game.PolicyAction {
	switch strings.ToUpper(strings.TrimSpace(p)) {
	case "PROXY":
		return game.PolicyProxy
	case "BLOCK":
		return game.PolicyBlock
	case "DIRECT":
		return game.PolicyDirect
	default:
		return game.PolicyProxy
	}
}

// normalizeCategory maps flexible category string to canonical game.DomainCategory.
func normalizeCategory(c string) game.DomainCategory {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "auth", "authentication":
		return game.CategoryAuth
	case "matchmaking", "match":
		return game.CategoryMatchmaking
	case "game_services", "services", "game services", "service":
		return game.CategoryGameServices
	case "cdn", "cdn_downloads", "cdn / downloads", "downloads":
		return game.CategoryCDN
	case "telemetry":
		return game.CategoryTelemetry
	default:
		if c != "" {
			return game.DomainCategory(c)
		}
		return game.CategoryGameServices
	}
}

// RegisterHGIRoutes attaches the Game Intelligence Engine (HGI) API endpoints to mux.
func (a *API) RegisterHGIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v2/games", a.SecurityMiddleware(a.handleV2Games))
	mux.HandleFunc("/api/v2/games/", a.SecurityMiddleware(a.handleV2GameItem))
	mux.HandleFunc("/api/v2/discovery/candidates", a.SecurityMiddleware(a.handleV2Candidates))
	mux.HandleFunc("/api/v2/discovery/candidates/", a.SecurityMiddleware(a.handleV2CandidateAction))
	mux.HandleFunc("/api/v2/discovery/events", a.SecurityMiddleware(a.handleV2DiscoveryEvents))
	mux.HandleFunc("/api/v2/discovery/scan", a.SecurityMiddleware(a.handleV2DiscoveryScan))
	mux.HandleFunc("/api/v2/discovery/approve", a.SecurityMiddleware(a.handleV2DiscoveryApprove))
	mux.HandleFunc("/api/v2/discovery/reject", a.SecurityMiddleware(a.handleV2DiscoveryReject))
	mux.HandleFunc("/api/v2/learning", a.SecurityMiddleware(a.handleV2Learning))
	mux.HandleFunc("/api/v2/learning/settings", a.SecurityMiddleware(a.handleV2LearningSettings))
	mux.HandleFunc("/api/v2/learning/export", a.SecurityMiddleware(a.handleV2LearningExport))
	mux.HandleFunc("/api/v2/learning/clear", a.SecurityMiddleware(a.handleV2LearningClear))
	mux.HandleFunc("/api/v2/routes", a.SecurityMiddleware(a.handleV2Routes))
	mux.HandleFunc("/api/v2/routes/measure", a.SecurityMiddleware(a.handleV2RouteMeasure))
	mux.HandleFunc("/api/v2/routes/measurements", a.SecurityMiddleware(a.handleV2Measurements))
	mux.HandleFunc("/api/v2/routes/measurements/run", a.SecurityMiddleware(a.handleV2MeasurementsRun))
	mux.HandleFunc("/api/v2/routes/select", a.SecurityMiddleware(a.handleV2RouteSelect))
	mux.HandleFunc("/api/v2/nodes", a.SecurityMiddleware(a.handleV2Nodes))
	mux.HandleFunc("/api/v2/nodes/health", a.SecurityMiddleware(a.handleV2NodesHealth))
	mux.HandleFunc("/api/v2/nodes/", a.SecurityMiddleware(a.handleV2NodeAction))
	mux.HandleFunc("/api/v2/rules/simulate", a.SecurityMiddleware(a.handleV2RulesSimulate))
	mux.HandleFunc("/api/v2/game-detect", a.SecurityMiddleware(a.handleV2GameDetect))
	mux.HandleFunc("/api/v2/timeline", a.SecurityMiddleware(a.handleV2Timeline))
	mux.HandleFunc("/api/v2/ai/assistant", a.SecurityMiddleware(a.handleV2AIAssistant))
	mux.HandleFunc("/api/v2/audit", a.SecurityMiddleware(a.handleV2Audit))
	mux.HandleFunc("/api/v2/cache/purge", a.SecurityMiddleware(a.handleV2CachePurge))
}

func (a *API) checkEngine(w http.ResponseWriter) bool {
	if a.gameEngine == nil {
		writeProblem(w, http.StatusServiceUnavailable, "service_unavailable", "HGI Game Intelligence Engine is not initialized")
		return false
	}
	return true
}

// GET /api/v2/games - List games
// POST /api/v2/games - Create new game profile
// DELETE /api/v2/games?id={id} - Delete game profile
func (a *API) handleV2Games(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}

	switch r.Method {
	case http.MethodGet, http.MethodHead:
		profiles, err := a.gameEngine.ListGameProfiles()
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":    profiles,
			"profiles": profiles,
			"total":    len(profiles),
		})

	case http.MethodPost:
		bodyBytes, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "Failed to read request body")
			return
		}

		var flexReq struct {
			ID                string                         `json:"id"`
			Name              string                         `json:"name"`
			Publisher         string                         `json:"publisher"`
			Enabled           *bool                          `json:"enabled"`
			DefaultPolicy     string                         `json:"default_policy"`
			Domains           json.RawMessage                `json:"domains"`
			DNSPolicies       map[game.DomainCategory]string `json:"dns_policies"`
			RoutePolicy       game.GameRoutePolicy           `json:"route_policy"`
			DiscoverySettings game.GameDiscoveryConfig       `json:"discovery_settings"`
			ConfidenceScore   int                            `json:"confidence_score"`
		}

		if err := json.Unmarshal(bodyBytes, &flexReq); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "Malformed JSON body: "+err.Error())
			return
		}

		if flexReq.ID == "" || flexReq.Name == "" {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "Game profile 'id' and 'name' are required")
			return
		}

		defPol := normalizePolicy(flexReq.DefaultPolicy)
		enabled := true
		if flexReq.Enabled != nil {
			enabled = *flexReq.Enabled
		}

		now := time.Now()
		profile := game.GameProfile{
			ID:                flexReq.ID,
			Name:              flexReq.Name,
			Publisher:         flexReq.Publisher,
			Enabled:           enabled,
			Domains:           make(map[string]game.DomainRecord),
			DNSPolicies:       flexReq.DNSPolicies,
			RoutePolicy:       flexReq.RoutePolicy,
			DiscoverySettings: flexReq.DiscoverySettings,
			ConfidenceScore:   flexReq.ConfidenceScore,
			CreatedAt:         now,
			UpdatedAt:         now,
		}

		if profile.ConfidenceScore == 0 {
			profile.ConfidenceScore = 95
		}

		if profile.DNSPolicies == nil {
			profile.DNSPolicies = map[game.DomainCategory]string{
				game.CategoryAuth:         string(defPol),
				game.CategoryMatchmaking:  string(defPol),
				game.CategoryGameServices: string(defPol),
				game.CategoryCDN:          string(game.PolicyDirect),
				game.CategoryTelemetry:    string(game.PolicyDirect),
				game.CategoryCandidate:    string(game.PolicyDirect),
			}
		}

		// Parse flexible Domains format
		if len(flexReq.Domains) > 0 && string(flexReq.Domains) != "null" {
			// Option 1: map[string]game.DomainRecord
			var domRecords map[string]game.DomainRecord
			if err := json.Unmarshal(flexReq.Domains, &domRecords); err == nil && len(domRecords) > 0 {
				for host, rec := range domRecords {
					rec.Hostname = normalizeDomain(host)
					rec.GameID = profile.ID
					if rec.Publisher == "" {
						rec.Publisher = profile.Publisher
					}
					if rec.Policy == "" {
						rec.Policy = defPol
					} else {
						rec.Policy = normalizePolicy(string(rec.Policy))
					}
					if rec.Category == "" {
						rec.Category = game.CategoryGameServices
					} else {
						rec.Category = normalizeCategory(string(rec.Category))
					}
					rec.Enabled = true
					rec.Status = game.StatusConfirmed
					rec.FirstSeen = now
					rec.LastSeen = now
					profile.Domains[rec.Hostname] = rec
				}
			} else {
				// Option 2: map[string][]string (e.g. {"matchmaking": ["host1", "host2"]})
				var catMap map[string][]string
				if err := json.Unmarshal(flexReq.Domains, &catMap); err == nil && len(catMap) > 0 {
					for catKey, hosts := range catMap {
						cat := normalizeCategory(catKey)
						for _, h := range hosts {
							normH := normalizeDomain(h)
							if normH == "" {
								continue
							}
							profile.Domains[normH] = game.DomainRecord{
								Hostname:         normH,
								GameID:           profile.ID,
								Publisher:        profile.Publisher,
								Category:         cat,
								Policy:           defPol,
								ConfidenceScore:  100,
								Status:           game.StatusConfirmed,
								Evidence:         []string{"User configured game profile"},
								FirstSeen:        now,
								LastSeen:         now,
								ObservationCount: 1,
								CreatedBy:        "admin_api",
								UpdatedAt:        now,
								Enabled:          true,
							}
						}
					}
				} else {
					// Option 3: []string (domain list)
					var listHosts []string
					if err := json.Unmarshal(flexReq.Domains, &listHosts); err == nil {
						for _, h := range listHosts {
							normH := normalizeDomain(h)
							if normH == "" {
								continue
							}
							profile.Domains[normH] = game.DomainRecord{
								Hostname:         normH,
								GameID:           profile.ID,
								Publisher:        profile.Publisher,
								Category:         game.CategoryMatchmaking,
								Policy:           defPol,
								ConfidenceScore:  100,
								Status:           game.StatusConfirmed,
								Evidence:         []string{"User configured game profile"},
								FirstSeen:        now,
								LastSeen:         now,
								ObservationCount: 1,
								CreatedBy:        "admin_api",
								UpdatedAt:        now,
								Enabled:          true,
							}
						}
					}
				}
			}
		}

		if err := a.gameEngine.SaveGameProfile(profile, "admin_api"); err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"profile": profile,
		})

	case http.MethodDelete:
		gameID := r.URL.Query().Get("id")
		if gameID == "" {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "Game profile 'id' query parameter is required")
			return
		}
		if err := a.gameEngine.DeleteGameProfile(gameID, "admin_api"); err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})

	default:
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET, POST, or DELETE")
	}
}

// GET /api/v2/games/{id}
// PUT /api/v2/games/{id}
// DELETE /api/v2/games/{id}
// POST /api/v2/games/{id}/domains
func (a *API) handleV2GameItem(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}

	tail := strings.TrimPrefix(r.URL.Path, "/api/v2/games/")
	parts := strings.Split(tail, "/")
	gameID := parts[0]
	if gameID == "" {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "Game ID is required")
		return
	}

	// Sub-resource: /api/v2/games/{id}/domains
	if len(parts) >= 2 && parts[1] == "domains" {
		a.handleV2GameDomains(w, r, gameID, parts[2:])
		return
	}

	// Sub-resource: /api/v2/games/{id}/categories
	if len(parts) >= 2 && parts[1] == "categories" {
		a.handleV2GameCategories(w, r, gameID, parts[2:])
		return
	}

	// Sub-resource: /api/v2/games/{id}/rollback
	if len(parts) >= 2 && parts[1] == "rollback" {
		if r.Method != http.MethodPost {
			writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
			return
		}
		var body struct {
			Version int `json:"version"`
		}
		if !decodeStrict(w, r, &body) {
			return
		}
		if err := a.gameEngine.RollbackGameProfile(gameID, body.Version, "admin_api"); err != nil {
			writeProblem(w, http.StatusBadRequest, "rollback_failed", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		return
	}

	switch r.Method {
	case http.MethodGet:
		p, err := a.gameEngine.GetGameProfile(gameID)
		if err != nil {
			writeProblem(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(p)

	case http.MethodPut:
		var updated game.GameProfile
		if !decodeStrict(w, r, &updated) {
			return
		}
		updated.ID = gameID
		if err := a.gameEngine.SaveGameProfile(updated, "admin_api"); err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"profile": updated,
		})

	case http.MethodDelete:
		if err := a.gameEngine.DeleteGameProfile(gameID, "admin_api"); err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})

	default:
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET, PUT, or DELETE")
	}
}

func (a *API) handleV2GameDomains(w http.ResponseWriter, r *http.Request, gameID string, subParts []string) {
	switch r.Method {
	case http.MethodGet:
		p, err := a.gameEngine.GetGameProfile(gameID)
		if err != nil {
			writeProblem(w, http.StatusNotFound, "not_found", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"game_id": gameID,
			"domains": p.Domains,
		})

	case http.MethodPost:
		var body struct {
			Hostname string `json:"hostname"`
			Category string `json:"category"`
			Policy   string `json:"policy"`
		}
		if !decodeStrict(w, r, &body) {
			return
		}
		if body.Hostname == "" {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "Hostname is required")
			return
		}
		normHost := normalizeDomain(body.Hostname)
		pol := normalizePolicy(body.Policy)
		cat := normalizeCategory(body.Category)

		if err := a.gameEngine.AddGameDomain(gameID, normHost, cat, pol, "admin_api"); err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})

	case http.MethodPut:
		var body struct {
			Hostname string `json:"hostname"`
			Category string `json:"category"`
			Policy   string `json:"policy"`
			Enabled  *bool  `json:"enabled"`
		}
		if !decodeStrict(w, r, &body) {
			return
		}
		targetDomain := body.Hostname
		if targetDomain == "" && len(subParts) > 0 {
			targetDomain = subParts[0]
		}
		if targetDomain == "" {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "Hostname is required")
			return
		}
		normHost := normalizeDomain(targetDomain)
		pol := normalizePolicy(body.Policy)
		cat := normalizeCategory(body.Category)
		enabled := true
		if body.Enabled != nil {
			enabled = *body.Enabled
		}

		if err := a.gameEngine.UpdateGameDomainPolicy(gameID, normHost, pol, cat, enabled, "admin_api"); err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})

	case http.MethodDelete:
		if len(subParts) == 0 || subParts[0] == "" {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "Domain name is required in path")
			return
		}
		targetDomain := normalizeDomain(subParts[0])
		if err := a.gameEngine.RemoveGameDomain(gameID, targetDomain, "admin_api"); err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})

	default:
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET, POST, PUT, or DELETE")
	}
}

func (a *API) handleV2GameCategories(w http.ResponseWriter, r *http.Request, gameID string, subParts []string) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use PUT or POST")
		return
	}
	var body struct {
		Category  string `json:"category"`
		Policy    string `json:"policy"`
		Propagate *bool  `json:"propagate"`
	}
	if !decodeStrict(w, r, &body) {
		return
	}
	catStr := body.Category
	if catStr == "" && len(subParts) > 0 {
		catStr = subParts[0]
	}
	if catStr == "" {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "Category is required")
		return
	}
	if body.Policy == "" {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "Policy is required")
		return
	}

	category := normalizeCategory(catStr)
	policy := normalizePolicy(body.Policy)
	propagate := false
	if body.Propagate != nil {
		propagate = *body.Propagate
	}
	if r.URL.Query().Get("propagate") == "true" {
		propagate = true
	}

	if err := a.gameEngine.UpdateGameCategoryPolicy(gameID, category, policy, propagate, "admin_api"); err != nil {
		writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// GET /api/v2/discovery/candidates
func (a *API) handleV2Candidates(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	if r.Method != http.MethodGet {
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET")
		return
	}

	statusFilter := r.URL.Query().Get("status")
	candidates, err := a.gameEngine.Store().ListCandidates(statusFilter)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"items":      candidates,
		"candidates": candidates,
		"total":      len(candidates),
	})
}

// POST /api/v2/discovery/candidates/{id}/approve
// POST /api/v2/discovery/candidates/{id}/reject
// DELETE /api/v2/discovery/candidates/{id}
func (a *API) handleV2CandidateAction(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}

	tail := strings.TrimPrefix(r.URL.Path, "/api/v2/discovery/candidates/")
	parts := strings.Split(tail, "/")
	candID := parts[0]
	if candID == "" {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "Candidate ID is required")
		return
	}

	if len(parts) >= 2 {
		action := parts[1]
		switch action {
		case "approve":
			if r.Method != http.MethodPost {
				writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
				return
			}
			var body struct {
				Policy game.PolicyAction `json:"policy,omitempty"`
			}
			_ = decodeStrict(w, r, &body)
			if err := a.gameEngine.Learning().ApproveCandidate(candID, body.Policy, "admin_api"); err != nil {
				writeProblem(w, http.StatusBadRequest, "approval_failed", err.Error())
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
			return

		case "reject":
			if r.Method != http.MethodPost {
				writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
				return
			}
			var body struct {
				Reason string `json:"reason,omitempty"`
			}
			_ = decodeStrict(w, r, &body)
			if err := a.gameEngine.Learning().RejectCandidate(candID, body.Reason, "admin_api"); err != nil {
				writeProblem(w, http.StatusBadRequest, "reject_failed", err.Error())
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
			return
		}
	}

	if r.Method == http.MethodDelete {
		if err := a.gameEngine.Learning().IgnoreCandidate(candID, "admin_api"); err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		return
	}

	writeProblem(w, http.StatusBadRequest, "invalid_action", "Supported actions: approve, reject, or DELETE")
}

// POST /api/v2/discovery/scan
func (a *API) handleV2DiscoveryScan(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	if r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
		return
	}

	cands, err := a.gameEngine.Store().ListCandidates("")
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":    true,
		"found":      len(cands),
		"items":      cands,
		"candidates": cands,
	})
}

// POST /api/v2/discovery/approve
func (a *API) handleV2DiscoveryApprove(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	if r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
		return
	}

	var body struct {
		Hostname    string `json:"hostname"`
		CandidateID string `json:"candidate_id,omitempty"`
		Policy      string `json:"policy,omitempty"`
	}
	if !decodeStrict(w, r, &body) {
		return
	}

	pol := normalizePolicy(body.Policy)

	candID := body.CandidateID
	if candID == "" && body.Hostname != "" {
		norm := normalizeDomain(body.Hostname)
		candID = fmt.Sprintf("cand_%x", norm)
		if _, err := a.gameEngine.Store().GetCandidate(candID); err != nil {
			cands, _ := a.gameEngine.Store().ListCandidates("")
			for _, c := range cands {
				if normalizeDomain(c.Hostname) == norm {
					candID = c.ID
					break
				}
			}
		}
	}

	if candID != "" {
		if err := a.gameEngine.Learning().ApproveCandidate(candID, pol, "admin_api"); err == nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
			return
		}
	}

	if body.Hostname != "" {
		targetGame := "cs2"
		if active := a.gameEngine.Detector().DetectActiveGame(); active != nil && active.GameID != "" {
			targetGame = active.GameID
		}
		_ = a.gameEngine.AddGameDomain(targetGame, normalizeDomain(body.Hostname), game.CategoryMatchmaking, pol, "admin_api")
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// POST /api/v2/discovery/reject
func (a *API) handleV2DiscoveryReject(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	if r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
		return
	}

	var body struct {
		Hostname    string `json:"hostname"`
		CandidateID string `json:"candidate_id,omitempty"`
		Reason      string `json:"reason,omitempty"`
	}
	if !decodeStrict(w, r, &body) {
		return
	}

	candID := body.CandidateID
	if candID == "" && body.Hostname != "" {
		norm := normalizeDomain(body.Hostname)
		candID = fmt.Sprintf("cand_%x", norm)
		if _, err := a.gameEngine.Store().GetCandidate(candID); err != nil {
			cands, _ := a.gameEngine.Store().ListCandidates("")
			for _, c := range cands {
				if normalizeDomain(c.Hostname) == norm {
					candID = c.ID
					break
				}
			}
		}
	}

	if candID != "" {
		reason := body.Reason
		if reason == "" {
			reason = "Rejected via admin console"
		}
		_ = a.gameEngine.Learning().RejectCandidate(candID, reason, "admin_api")
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// GET /api/v2/discovery/events
func (a *API) handleV2DiscoveryEvents(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	events := a.gameEngine.Timeline().GetEvents("", "", "", 50)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"items":  events,
		"events": events,
		"total":  len(events),
	})
}

// GET /api/v2/learning
// POST /api/v2/learning
func (a *API) handleV2Learning(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		settings, err := a.gameEngine.Learning().GetSettings()
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		report, _ := a.gameEngine.Learning().GenerateReport()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"settings": settings,
			"report":   report,
		})

	case http.MethodPost, http.MethodPut:
		a.handleV2LearningSettings(w, r)

	default:
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET, POST, or PUT")
	}
}

// GET /api/v2/learning/settings
// POST /api/v2/learning/settings
func (a *API) handleV2LearningSettings(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		settings, err := a.gameEngine.Learning().GetSettings()
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"settings": settings,
		})

	case http.MethodPost, http.MethodPut:
		var req struct {
			Mode                  string `json:"mode,omitempty"`
			AutoApplyMinScore     *int   `json:"auto_apply_min_score,omitempty"`
			AllowAutoProxy        *bool  `json:"allow_auto_proxy,omitempty"`
			AllowAutoBlock        *bool  `json:"allow_auto_block,omitempty"`
			RetentionDays         *int   `json:"retention_days,omitempty"`
			AnonymizeQueryData    *bool  `json:"anonymize_query_data,omitempty"`
			EnableActiveDetection *bool  `json:"enable_active_detection,omitempty"`
		}
		if !decodeStrict(w, r, &req) {
			return
		}

		current, err := a.gameEngine.Learning().GetSettings()
		if err != nil || current == nil {
			current = &game.LearningSettings{
				Mode:                  game.ModeObserve,
				AutoApplyMinScore:     80,
				AllowAutoProxy:        true,
				RetentionDays:         7,
				EnableActiveDetection: true,
			}
		}

		if req.Mode != "" {
			switch strings.ToLower(req.Mode) {
			case "observe":
				current.Mode = game.ModeObserve
			case "recommend":
				current.Mode = game.ModeRecommend
			case "auto-apply", "auto_apply", "auto":
				current.Mode = game.ModeAutoApply
			}
		}
		if req.AutoApplyMinScore != nil {
			current.AutoApplyMinScore = *req.AutoApplyMinScore
		}
		if req.AllowAutoProxy != nil {
			current.AllowAutoProxy = *req.AllowAutoProxy
		}
		if req.AllowAutoBlock != nil {
			current.AllowAutoBlock = *req.AllowAutoBlock
		}
		if req.RetentionDays != nil {
			current.RetentionDays = *req.RetentionDays
		}
		if req.AnonymizeQueryData != nil {
			current.AnonymizeQueryData = *req.AnonymizeQueryData
		}
		if req.EnableActiveDetection != nil {
			current.EnableActiveDetection = *req.EnableActiveDetection
		}

		if err := a.gameEngine.Learning().SetSettings(*current, "admin_api"); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_settings", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":  true,
			"settings": current,
		})

	default:
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET, POST, or PUT")
	}
}

// GET /api/v2/learning/export
func (a *API) handleV2LearningExport(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	data, err := a.gameEngine.Learning().ExportReportJSON()
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "export_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=\"hyperdns-hgi-learning-report.json\"")
	_, _ = w.Write(data)
}

// POST /api/v2/learning/clear
func (a *API) handleV2LearningClear(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	if r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
		return
	}
	if err := a.gameEngine.Learning().ClearLearningData("admin_api"); err != nil {
		writeProblem(w, http.StatusInternalServerError, "clear_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// GET /api/v2/routes
// POST /api/v2/routes
func (a *API) handleV2Routes(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}

	switch r.Method {
	case http.MethodGet:
		nodes, err := a.gameEngine.Store().ListRouteNodes()
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items":  nodes,
			"routes": nodes,
			"total":  len(nodes),
		})

	case http.MethodPost:
		var node game.RouteNode
		if !decodeStrict(w, r, &node) {
			return
		}
		if node.ID == "" || node.Endpoint == "" {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "Node 'id' and 'endpoint' are required")
			return
		}
		if err := a.gameEngine.Store().SaveRouteNode(node); err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"node":    node,
		})

	default:
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET or POST")
	}
}

// POST /api/v2/routes/measure
func (a *API) handleV2RouteMeasure(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	if r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
		return
	}

	nodeID := r.URL.Query().Get("node_id")
	if nodeID != "" {
		node, err := a.gameEngine.Store().GetRouteNode(nodeID)
		if err != nil {
			writeProblem(w, http.StatusNotFound, "not_found", "Route node not found")
			return
		}
		probed := a.gameEngine.Routes().ProbeNode(*node)
		_ = a.gameEngine.Store().SaveRouteNode(probed)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"node":    probed,
			"nodes":   []game.RouteNode{probed},
			"routes":  []game.RouteNode{probed},
		})
		return
	}

	results := a.gameEngine.Routes().ProbeAllNodes()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"nodes":   results,
		"routes":  results,
	})
}

// GET /api/v2/routes/measurements
func (a *API) handleV2Measurements(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	meas := a.gameEngine.Routes().GetMeasurements(limit)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"items": meas,
		"total": len(meas),
	})
}

// POST /api/v2/routes/measurements/run
func (a *API) handleV2MeasurementsRun(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	if r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
		return
	}
	results := a.gameEngine.Routes().ProbeAllNodes()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"nodes":   results,
	})
}

// POST /api/v2/routes/select
func (a *API) handleV2RouteSelect(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	if r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
		return
	}
	var body struct {
		GameID string `json:"game_id"`
		NodeID string `json:"node_id"`
	}
	if !decodeStrict(w, r, &body) {
		return
	}
	if err := a.gameEngine.Routes().SelectRoute(body.GameID, body.NodeID, "admin_api"); err != nil {
		writeProblem(w, http.StatusBadRequest, "route_selection_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
}

// GET /api/v2/nodes
// POST /api/v2/nodes
func (a *API) handleV2Nodes(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		nodes, err := a.gameEngine.Store().ListRouteNodes()
		if err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": nodes,
			"total": len(nodes),
		})

	case http.MethodPost:
		var body struct {
			Node  game.RouteNode `json:"node"`
			Token string         `json:"token"`
		}
		if !decodeStrict(w, r, &body) {
			return
		}
		if err := a.gameEngine.Controller().RegisterNode(body.Node, body.Token, "admin_api"); err != nil {
			writeProblem(w, http.StatusBadRequest, "registration_failed", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})

	default:
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET or POST")
	}
}

// GET /api/v2/nodes/health
func (a *API) handleV2NodesHealth(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	nodes, _ := a.gameEngine.Store().ListRouteNodes()
	summary := make(map[string]string)
	for _, n := range nodes {
		summary[n.ID] = string(n.Status)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"nodes": summary,
	})
}

// POST /api/v2/nodes/{id}/sync
// DELETE /api/v2/nodes/{id}
func (a *API) handleV2NodeAction(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}

	tail := strings.TrimPrefix(r.URL.Path, "/api/v2/nodes/")
	parts := strings.Split(tail, "/")
	nodeID := parts[0]
	if nodeID == "" {
		writeProblem(w, http.StatusBadRequest, "invalid_request", "Node ID is required")
		return
	}

	if len(parts) >= 2 && parts[1] == "sync" && r.Method == http.MethodPost {
		if err := a.gameEngine.Controller().SyncNode(nodeID, "admin_api"); err != nil {
			writeProblem(w, http.StatusInternalServerError, "sync_failed", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		return
	}

	if r.Method == http.MethodDelete {
		if err := a.gameEngine.Controller().RevokeNode(nodeID, "admin_api"); err != nil {
			writeProblem(w, http.StatusInternalServerError, "revoke_failed", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		return
	}

	writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST /sync or DELETE")
}

// POST /api/v2/rules/simulate
func (a *API) handleV2RulesSimulate(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	if r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
		return
	}

	var req game.RuleSimulationRequest
	if !decodeStrict(w, r, &req) {
		return
	}

	res, err := a.gameEngine.Simulator().Simulate(req)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "simulation_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// GET /api/v2/game-detect
func (a *API) handleV2GameDetect(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	detected := a.gameEngine.Detector().DetectActiveGame()
	w.Header().Set("Content-Type", "application/json")
	if detected == nil {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"detected": false,
			"message":  "No active game traffic detected in current window",
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"detected": true,
		"game":     detected,
	})
}

// GET /api/v2/timeline
func (a *API) handleV2Timeline(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	gameID := r.URL.Query().Get("game_id")
	clientIP := r.URL.Query().Get("client_ip")
	hostname := r.URL.Query().Get("hostname")
	limit := 100
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	events := a.gameEngine.Timeline().GetEvents(gameID, clientIP, hostname, limit)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"items":  events,
		"events": events,
		"total":  len(events),
	})
}

// POST /api/v2/ai/assistant
func (a *API) handleV2AIAssistant(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	if r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
		return
	}

	var req game.AIAssistantRequest
	if !decodeStrict(w, r, &req) {
		return
	}

	res, err := a.gameEngine.AI().Analyze(req)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "analysis_failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

// GET /api/v2/audit
func (a *API) handleV2Audit(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	limit := 100
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}
	logs, err := a.gameEngine.Store().ListAuditLogs(limit)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"items":  logs,
		"audits": logs,
		"total":  len(logs),
	})
}

// POST /api/v2/cache/purge
func (a *API) handleV2CachePurge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST")
		return
	}

	var body struct {
		Domain  string   `json:"domain,omitempty"`
		Domains []string `json:"domains,omitempty"`
		GameID  string   `json:"game_id,omitempty"`
	}
	if !decodeStrict(w, r, &body) {
		return
	}

	if a.cache == nil {
		writeProblem(w, http.StatusServiceUnavailable, "cache_unavailable", "DNS Cache is not initialized")
		return
	}

	purgedCount := 0
	if body.GameID != "" && a.gameEngine != nil {
		p, err := a.gameEngine.GetGameProfile(body.GameID)
		if err == nil && p != nil {
			var doms []string
			for h := range p.Domains {
				doms = append(doms, h)
			}
			purgedCount += a.cache.PurgeDomains(doms)
		}
	}

	if body.Domain != "" {
		purgedCount += a.cache.PurgeDomain(body.Domain)
	}
	if len(body.Domains) > 0 {
		purgedCount += a.cache.PurgeDomains(body.Domains)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"success":      true,
		"purged_count": purgedCount,
	})
}
