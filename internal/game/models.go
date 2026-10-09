package game

import (
	"time"
)

// DomainCategory categorizes domains according to their role in gaming traffic.
type DomainCategory string

const (
	CategoryAuth         DomainCategory = "Authentication"
	CategoryMatchmaking  DomainCategory = "Matchmaking"
	CategoryGameServices DomainCategory = "Game Services"
	CategoryCDN          DomainCategory = "CDN / Downloads"
	CategoryTelemetry    DomainCategory = "Telemetry"
	CategoryCandidate    DomainCategory = "Unknown / Candidate"
)

// PolicyAction defines the DNS action applied to domains.
type PolicyAction string

const (
	PolicyDirect PolicyAction = "DIRECT"
	PolicyProxy  PolicyAction = "PROXY"
	PolicyBlock  PolicyAction = "BLOCK"
)

// CandidateStatus defines the evaluation status of a domain or candidate.
type CandidateStatus string

const (
	StatusCandidate   CandidateStatus = "candidate"
	StatusConfirmed   CandidateStatus = "confirmed"
	StatusRejected    CandidateStatus = "rejected"
	StatusExpired     CandidateStatus = "expired"
	StatusNeedsReview CandidateStatus = "needs_review"
)

// LearningMode represents the operating mode of the learning engine.
type LearningMode string

const (
	ModeObserve    LearningMode = "observe"
	ModeRecommend  LearningMode = "recommend"
	ModeAutoApply  LearningMode = "auto-apply"
)

// NodeHealthStatus represents the connectivity state of a VPS node.
type NodeHealthStatus string

const (
	NodeHealthy  NodeHealthStatus = "Healthy"
	NodeDegraded NodeHealthStatus = "Degraded"
	NodeDown     NodeHealthStatus = "Down"
)

// NodeSyncStatus represents the synchronization state between master and node.
type NodeSyncStatus string

const (
	SyncSynced   NodeSyncStatus = "Synced"
	SyncPending  NodeSyncStatus = "Pending"
	SyncDiverged NodeSyncStatus = "Diverged"
	SyncOffline  NodeSyncStatus = "Offline"
)

// GameProfile represents a dedicated game profile with domain categories, policies, and route settings.
type GameProfile struct {
	ID                string                    `json:"id"`
	Name              string                    `json:"name"`
	Publisher         string                    `json:"publisher"`
	Enabled           bool                      `json:"enabled"`
	Domains           map[string]DomainRecord   `json:"domains"`
	DNSPolicies       map[DomainCategory]string `json:"dns_policies"`
	RoutePolicy       GameRoutePolicy           `json:"route_policy"`
	DiscoverySettings GameDiscoveryConfig       `json:"discovery_settings"`
	ConfidenceScore   int                       `json:"confidence_score"`
	UpdateHistory     []ProfileHistoryEntry     `json:"update_history"`
	CreatedAt         time.Time                 `json:"created_at"`
	UpdatedAt         time.Time                 `json:"updated_at"`
}

// DomainRecord holds details about a domain inside a Game Profile.
type DomainRecord struct {
	Hostname         string         `json:"hostname"`
	GameID           string         `json:"game_id"`
	Publisher        string         `json:"publisher"`
	Category         DomainCategory `json:"category"`
	Policy           PolicyAction   `json:"policy"`
	ConfidenceScore  int            `json:"confidence_score"`
	Status           CandidateStatus`json:"status"`
	Evidence         []string       `json:"evidence"`
	FirstSeen        time.Time      `json:"first_seen"`
	LastSeen         time.Time      `json:"last_seen"`
	ObservationCount uint64         `json:"observation_count"`
	LastResolvedIPs  []string       `json:"last_resolved_ips"`
	LastTTL          uint32         `json:"last_ttl"`
	CreatedBy        string         `json:"created_by"`
	UpdatedAt        time.Time      `json:"updated_at"`
	Enabled          bool           `json:"enabled"`
}

// GameRoutePolicy specifies routing preferences for a game.
type GameRoutePolicy struct {
	AutoSelectEnabled    bool     `json:"auto_select_enabled"`
	PreferredNodeID      string   `json:"preferred_node_id"`
	FallbackNodeIDs      []string `json:"fallback_node_ids"`
	MaxRTTThresholdMs    float64  `json:"max_rtt_threshold_ms"`
	MaxLossThresholdPct  float64  `json:"max_loss_threshold_pct"`
	MaxJitterThresholdMs float64  `json:"max_jitter_threshold_ms"`
}

// GameDiscoveryConfig holds discovery rules for a game.
type GameDiscoveryConfig struct {
	Enabled               bool     `json:"enabled"`
	AutoCandidateMinScore int      `json:"auto_candidate_min_score"`
	KeywordPatterns       []string `json:"keyword_patterns"`
	KnownRootDomains      []string `json:"known_root_domains"`
}

// ProfileHistoryEntry logs changes to a game profile for audit and rollback.
type ProfileHistoryEntry struct {
	Version   int       `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Actor     string    `json:"actor"`
	Summary   string    `json:"summary"`
	Snapshot  string    `json:"snapshot,omitempty"` // JSON-encoded snapshot for rollback
}

// RouteNode represents a VPS node or relay endpoint with quality metrics.
type RouteNode struct {
	ID             string           `json:"id"`
	Name           string           `json:"name"`
	Region         string           `json:"region"`
	Endpoint       string           `json:"endpoint"`
	Port           int              `json:"port"`
	IsMaster       bool             `json:"is_master"`
	Active         bool             `json:"active"`
	RTTMs          float64          `json:"rtt_ms"`
	PacketLossPct  float64          `json:"packet_loss_pct"`
	JitterMs       float64          `json:"jitter_ms"`
	DNSTimeMs      float64          `json:"dns_time_ms"`
	TCPTimeMs      float64          `json:"tcp_time_ms"`
	TLSTimeMs      float64          `json:"tls_time_ms"`
	StabilityScore float64          `json:"stability_score"`
	Status         NodeHealthStatus `json:"status"`
	LastCheck      time.Time        `json:"last_check"`
	SyncStatus     NodeSyncStatus   `json:"sync_status"`
	Version        string           `json:"version"`
	AuthToken      string           `json:"auth_token,omitempty"`
}

// RouteMeasurement records a single route test event.
type RouteMeasurement struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	NodeID    string    `json:"node_id"`
	Target    string    `json:"target"`
	RTTMs     float64   `json:"rtt_ms"`
	LossPct   float64   `json:"loss_pct"`
	JitterMs  float64   `json:"jitter_ms"`
	DNSTimeMs float64   `json:"dns_time_ms"`
	TCPTimeMs float64   `json:"tcp_time_ms"`
	TLSTimeMs float64   `json:"tls_time_ms"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
}

// DiscoveryCandidate represents an unconfirmed domain discovered by the Agent.
type DiscoveryCandidate struct {
	ID              string          `json:"id"`
	Hostname        string          `json:"hostname"`
	GameID          string          `json:"game_id"`
	Publisher       string          `json:"publisher"`
	Category        DomainCategory  `json:"category"`
	ConfidenceScore int             `json:"confidence_score"`
	Status          CandidateStatus `json:"status"`
	Evidence        []string        `json:"evidence"`
	FirstSeen       time.Time       `json:"first_seen"`
	LastSeen        time.Time       `json:"last_seen"`
	Count           uint64          `json:"count"`
	SampleIPs       []string        `json:"sample_ips"`
	SampleTTL       uint32          `json:"sample_ttl"`
	ProposedPolicy  PolicyAction    `json:"proposed_policy"`
	Reason          string          `json:"reason"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// LearningSettings configures the Learning Mode engine.
type LearningSettings struct {
	Mode                  LearningMode `json:"mode"`
	AutoApplyMinScore     int          `json:"auto_apply_min_score"`
	AllowAutoProxy        bool         `json:"allow_auto_proxy"`
	AllowAutoBlock        bool         `json:"allow_auto_block"`
	RetentionDays         int          `json:"retention_days"`
	AnonymizeQueryData    bool         `json:"anonymize_query_data"`
	EnableActiveDetection bool         `json:"enable_active_detection"`
}

// DetectedGame reports the real-time active game detected by the engine.
type DetectedGame struct {
	GameID         string    `json:"game_id"`
	GameName       string    `json:"game_name"`
	Publisher      string    `json:"publisher"`
	Confidence     int       `json:"confidence"`
	Evidence       string    `json:"evidence"`
	DetectedAt     time.Time `json:"detected_at"`
	SuggestedAction string   `json:"suggested_action"`
}

// RuleSimulationRequest is the input to the Rule Simulator.
type RuleSimulationRequest struct {
	Domain         string       `json:"domain"`
	ClientID       string       `json:"client_id,omitempty"`
	ProposedPolicy PolicyAction `json:"proposed_policy"`
	GameID         string       `json:"game_id,omitempty"`
}

// RuleSimulationResult is the output from the Rule Simulator.
type RuleSimulationResult struct {
	Domain          string       `json:"domain"`
	CurrentPolicy   PolicyAction `json:"current_policy"`
	ProposedPolicy  PolicyAction `json:"proposed_policy"`
	EffectivePolicy PolicyAction `json:"effective_policy"`
	MatchedRule     string       `json:"matched_rule"`
	Conflict        bool         `json:"conflict"`
	Reason          string       `json:"reason"`
	Warnings        []string     `json:"warnings"`
	BlastRadiusCount int         `json:"blast_radius_count"`
	AffectedDomains []string     `json:"affected_domains"`
}

// GameTimelineEvent logs chronological traffic and decision events.
type GameTimelineEvent struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	EventType string    `json:"event_type"` // query_observed, rule_matched, policy_selected, route_checked, candidate_added, failover
	GameID    string    `json:"game_id,omitempty"`
	ClientIP  string    `json:"client_ip"`
	Hostname  string    `json:"hostname"`
	Policy    string    `json:"policy"`
	RuleName  string    `json:"rule_name"`
	Details   string    `json:"details"`
	LatencyMs float64   `json:"latency_ms"`
}

// AuditEntry logs administrative and automated modifications.
type AuditEntry struct {
	ID         string    `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Actor      string    `json:"actor"` // "admin", "agent", "system", "node"
	Action     string    `json:"action"` // "approve_candidate", "update_policy", "route_failover", "rollback", etc.
	TargetType string    `json:"target_type"` // "game", "domain", "route", "learning"
	TargetID   string    `json:"target_id"`
	Details    string    `json:"details"`
	Changes    string    `json:"changes,omitempty"`
}

// AIAssistantRequest represents an analysis prompt or task sent to the AI Assistant.
type AIAssistantRequest struct {
	Query       string `json:"query"`
	GameID      string `json:"game_id,omitempty"`
	ContextType string `json:"context_type,omitempty"` // "candidates", "routes", "conflict", "general"
}

// AIAssistantResponse represents structured findings from the AI Assistant.
type AIAssistantResponse struct {
	Summary          string   `json:"summary"`
	Analysis         string   `json:"analysis"`
	Recommendations  []string `json:"recommendations"`
	ConfidenceScore  int      `json:"confidence_score"`
	SuggestedActions []string `json:"suggested_actions"`
	ConflictingRules []string `json:"conflicting_rules,omitempty"`
	GeneratedAt      time.Time `json:"generated_at"`
}

