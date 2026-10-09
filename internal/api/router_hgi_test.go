package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hyperdns/internal/game"
)

func setupTestHGIAPI(t *testing.T) (*API, http.Handler, func()) {
	t.Helper()
	apiInst, db, _, cleanup := setupTestAPI(t)

	eng, err := game.NewEngine(db, apiInst.matcher)
	if err != nil {
		t.Fatalf("failed to init game engine: %v", err)
	}
	apiInst.SetGameEngine(eng)

	mux := http.NewServeMux()
	apiInst.RegisterRoutes(mux)

	return apiInst, mux, cleanup
}

func hgiRequest(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-API-Key", v2TestKey)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestHGI_GamesListAndCreate(t *testing.T) {
	_, mux, cleanup := setupTestHGIAPI(t)
	defer cleanup()

	// 1. GET /api/v2/games returns both items and profiles
	w := hgiRequest(t, mux, http.MethodGet, "/api/v2/games", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/v2/games = %d: %s", w.Code, w.Body.String())
	}
	var listResp struct {
		Items    []any `json:"items"`
		Profiles []any `json:"profiles"`
		Total    int   `json:"total"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode GET /api/v2/games: %v", err)
	}
	if len(listResp.Items) == 0 || len(listResp.Profiles) == 0 {
		t.Fatalf("expected seeded games in items and profiles, got items=%d, profiles=%d", len(listResp.Items), len(listResp.Profiles))
	}

	// 2. POST /api/v2/games with frontend custom payload (default_policy + categorized domain lists)
	createPayload := map[string]any{
		"id":             "custom_game",
		"name":           "Custom Game",
		"publisher":      "Custom Studios",
		"enabled":        true,
		"default_policy": "proxy",
		"domains": map[string][]string{
			"matchmaking":   {"match.customgame.com", "lobby.customgame.com"},
			"game_services": {"api.customgame.com"},
		},
	}
	wCreate := hgiRequest(t, mux, http.MethodPost, "/api/v2/games", createPayload)
	if wCreate.Code != http.StatusCreated {
		t.Fatalf("POST /api/v2/games = %d: %s", wCreate.Code, wCreate.Body.String())
	}

	// 3. Verify newly created profile has mapped domains
	wGetItem := hgiRequest(t, mux, http.MethodGet, "/api/v2/games/custom_game", nil)
	if wGetItem.Code != http.StatusOK {
		t.Fatalf("GET /api/v2/games/custom_game = %d: %s", wGetItem.Code, wGetItem.Body.String())
	}
	var createdProfile game.GameProfile
	if err := json.Unmarshal(wGetItem.Body.Bytes(), &createdProfile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}
	if len(createdProfile.Domains) != 3 {
		t.Fatalf("expected 3 domains, got %d", len(createdProfile.Domains))
	}
	if createdProfile.Domains["match.customgame.com"].Policy != game.PolicyProxy {
		t.Fatalf("expected PROXY policy, got %s", createdProfile.Domains["match.customgame.com"].Policy)
	}

	// 4. DELETE /api/v2/games?id=custom_game
	wDel := hgiRequest(t, mux, http.MethodDelete, "/api/v2/games?id=custom_game", nil)
	if wDel.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v2/games?id=custom_game = %d: %s", wDel.Code, wDel.Body.String())
	}
}

func TestHGI_LearningSettings(t *testing.T) {
	_, mux, cleanup := setupTestHGIAPI(t)
	defer cleanup()

	// 1. POST /api/v2/learning/settings
	setPayload := map[string]any{
		"mode": "recommend",
	}
	wSet := hgiRequest(t, mux, http.MethodPost, "/api/v2/learning/settings", setPayload)
	if wSet.Code != http.StatusOK {
		t.Fatalf("POST /api/v2/learning/settings = %d: %s", wSet.Code, wSet.Body.String())
	}

	// 2. GET /api/v2/learning/settings
	wGet := hgiRequest(t, mux, http.MethodGet, "/api/v2/learning/settings", nil)
	if wGet.Code != http.StatusOK {
		t.Fatalf("GET /api/v2/learning/settings = %d: %s", wGet.Code, wGet.Body.String())
	}
	var getResp struct {
		Settings game.LearningSettings `json:"settings"`
	}
	if err := json.Unmarshal(wGet.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if getResp.Settings.Mode != game.ModeRecommend {
		t.Fatalf("expected mode recommend, got %s", getResp.Settings.Mode)
	}
}

func TestHGI_DiscoveryAndRouteActions(t *testing.T) {
	_, mux, cleanup := setupTestHGIAPI(t)
	defer cleanup()

	// 1. POST /api/v2/discovery/scan
	wScan := hgiRequest(t, mux, http.MethodPost, "/api/v2/discovery/scan", nil)
	if wScan.Code != http.StatusOK {
		t.Fatalf("POST /api/v2/discovery/scan = %d: %s", wScan.Code, wScan.Body.String())
	}

	// 2. POST /api/v2/discovery/approve
	approvePayload := map[string]any{
		"hostname": "newgame.valve.net",
		"policy":   "proxy",
	}
	wApprove := hgiRequest(t, mux, http.MethodPost, "/api/v2/discovery/approve", approvePayload)
	if wApprove.Code != http.StatusOK {
		t.Fatalf("POST /api/v2/discovery/approve = %d: %s", wApprove.Code, wApprove.Body.String())
	}

	// 3. POST /api/v2/routes/measure
	wMeasure := hgiRequest(t, mux, http.MethodPost, "/api/v2/routes/measure", nil)
	if wMeasure.Code != http.StatusOK {
		t.Fatalf("POST /api/v2/routes/measure = %d: %s", wMeasure.Code, wMeasure.Body.String())
	}

	// 4. PUT /api/v2/games/cs2/categories/matchmaking?propagate=true
	propPayload := map[string]any{
		"policy": "proxy",
	}
	wProp := hgiRequest(t, mux, http.MethodPut, "/api/v2/games/cs2/categories/matchmaking?propagate=true", propPayload)
	if wProp.Code != http.StatusOK {
		t.Fatalf("PUT /api/v2/games/cs2/categories/matchmaking?propagate=true = %d: %s", wProp.Code, wProp.Body.String())
	}
}

