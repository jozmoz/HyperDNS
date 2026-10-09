package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"hyperdns/internal/game"
)

// RegisterHGIRoutes attaches the Game Intelligence Engine (HGI) API endpoints to mux.
func (a *API) RegisterHGIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v2/games", a.SecurityMiddleware(a.handleV2Games))
	mux.HandleFunc("/api/v2/games/", a.SecurityMiddleware(a.handleV2GameItem))
	mux.HandleFunc("/api/v2/discovery/candidates", a.SecurityMiddleware(a.handleV2Candidates))
	mux.HandleFunc("/api/v2/discovery/candidates/", a.SecurityMiddleware(a.handleV2CandidateAction))
	mux.HandleFunc("/api/v2/discovery/events", a.SecurityMiddleware(a.handleV2DiscoveryEvents))
	mux.HandleFunc("/api/v2/learning", a.SecurityMiddleware(a.handleV2Learning))
	mux.HandleFunc("/api/v2/learning/export", a.SecurityMiddleware(a.handleV2LearningExport))
	mux.HandleFunc("/api/v2/learning/clear", a.SecurityMiddleware(a.handleV2LearningClear))
	mux.HandleFunc("/api/v2/routes", a.SecurityMiddleware(a.handleV2Routes))
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
			"items": profiles,
			"total": len(profiles),
		})

	case http.MethodPost:
		var profile game.GameProfile
		if !decodeStrict(w, r, &profile) {
			return
		}
		if profile.ID == "" || profile.Name == "" {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "Game profile 'id' and 'name' are required")
			return
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

	default:
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET or POST")
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
			Hostname string              `json:"hostname"`
			Category game.DomainCategory `json:"category"`
			Policy   game.PolicyAction   `json:"policy"`
		}
		if !decodeStrict(w, r, &body) {
			return
		}
		if body.Hostname == "" {
			writeProblem(w, http.StatusBadRequest, "invalid_request", "Hostname is required")
			return
		}
		if body.Policy == "" {
			body.Policy = game.PolicyProxy
		}
		if body.Category == "" {
			body.Category = game.CategoryGameServices
		}

		if err := a.gameEngine.AddGameDomain(gameID, body.Hostname, body.Category, body.Policy, "admin_api"); err != nil {
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
		targetDomain := subParts[0]
		if err := a.gameEngine.RemoveGameDomain(gameID, targetDomain, "admin_api"); err != nil {
			writeProblem(w, http.StatusInternalServerError, "database_error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true})

	default:
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET, POST, or DELETE")
	}
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
		"items": candidates,
		"total": len(candidates),
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

// GET /api/v2/discovery/events
func (a *API) handleV2DiscoveryEvents(w http.ResponseWriter, r *http.Request) {
	if !a.checkEngine(w) {
		return
	}
	events := a.gameEngine.Timeline().GetEvents("", "", "", 50)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"items": events,
		"total": len(events),
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

	case http.MethodPost:
		var cfg game.LearningSettings
		if !decodeStrict(w, r, &cfg) {
			return
		}
		if err := a.gameEngine.Learning().SetSettings(cfg, "admin_api"); err != nil {
			writeProblem(w, http.StatusBadRequest, "invalid_settings", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":  true,
			"settings": cfg,
		})

	default:
		writeProblem(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use GET or POST")
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
			"items": nodes,
			"total": len(nodes),
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
		"items": events,
		"total": len(events),
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
		"items": logs,
		"total": len(logs),
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
