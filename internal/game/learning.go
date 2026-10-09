package game

import (
	"encoding/json"
	"fmt"
	"time"
)

// LearningReport summarizes insights collected during learning mode.
type LearningReport struct {
	Mode               LearningMode          `json:"mode"`
	GeneratedAt        time.Time             `json:"generated_at"`
	TotalObserved      int                   `json:"total_observed"`
	PendingReviewCount int                   `json:"pending_review_count"`
	Candidates         []DiscoveryCandidate  `json:"candidates"`
	RecentAuditLogs    []AuditEntry          `json:"recent_audit_logs"`
	ActiveProfiles     []string              `json:"active_profiles"`
}

// LearningManager coordinates Learning Mode operation, proposals, approvals and report exports.
type LearningManager struct {
	store *Store
	agent *DiscoveryAgent
}

// NewLearningManager creates a new LearningManager.
func NewLearningManager(store *Store, agent *DiscoveryAgent) *LearningManager {
	return &LearningManager{
		store: store,
		agent: agent,
	}
}

// GetSettings returns current learning settings.
func (lm *LearningManager) GetSettings() (*LearningSettings, error) {
	return lm.store.GetLearningSettings()
}

// SetSettings updates the learning mode and safety thresholds.
func (lm *LearningManager) SetSettings(cfg LearningSettings, actor string) error {
	// Validate mode
	switch cfg.Mode {
	case ModeObserve, ModeRecommend, ModeAutoApply:
	default:
		return fmt.Errorf("invalid learning mode: %q (must be observe, recommend, or auto-apply)", cfg.Mode)
	}

	if cfg.AutoApplyMinScore < 50 {
		cfg.AutoApplyMinScore = 50
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 7
	}

	if err := lm.store.SaveLearningSettings(cfg); err != nil {
		return err
	}

	_ = lm.store.AppendAuditLog(AuditEntry{
		Actor:      actor,
		Action:     "learning_mode_changed",
		TargetType: "learning",
		TargetID:   string(cfg.Mode),
		Details:    fmt.Sprintf("Learning mode set to %s (AutoApplyMinScore: %d%%)", cfg.Mode, cfg.AutoApplyMinScore),
	})

	return nil
}

// ApproveCandidate approves a discovered candidate with an optional policy override.
func (lm *LearningManager) ApproveCandidate(candidateID string, overridePolicy PolicyAction, actor string) error {
	cand, err := lm.store.GetCandidate(candidateID)
	if err != nil {
		return err
	}

	if overridePolicy != "" {
		cand.ProposedPolicy = overridePolicy
		_ = lm.store.SaveCandidate(*cand)
	}

	return lm.agent.AutoPromoteCandidate(candidateID, actor)
}

// RejectCandidate marks a candidate as rejected with reason.
func (lm *LearningManager) RejectCandidate(candidateID, reason, actor string) error {
	cand, err := lm.store.GetCandidate(candidateID)
	if err != nil {
		return err
	}

	cand.Status = StatusRejected
	cand.Reason = reason
	cand.UpdatedAt = time.Now()
	if err := lm.store.SaveCandidate(*cand); err != nil {
		return err
	}

	_ = lm.store.AppendAuditLog(AuditEntry{
		Actor:      actor,
		Action:     "candidate_rejected",
		TargetType: "domain",
		TargetID:   cand.Hostname,
		Details:    fmt.Sprintf("Rejected candidate %s. Reason: %s", cand.Hostname, reason),
	})

	return nil
}

// IgnoreCandidate drops a candidate from the review queue.
func (lm *LearningManager) IgnoreCandidate(candidateID, actor string) error {
	_ = lm.store.AppendAuditLog(AuditEntry{
		Actor:      actor,
		Action:     "candidate_ignored",
		TargetType: "domain",
		TargetID:   candidateID,
		Details:    "Candidate dismissed from pending review",
	})
	return lm.store.DeleteCandidate(candidateID)
}

// ClearLearningData purges discovery candidates and resets learning data while preserving active rules.
func (lm *LearningManager) ClearLearningData(actor string) error {
	if err := lm.store.ClearCandidates(); err != nil {
		return err
	}

	_ = lm.store.AppendAuditLog(AuditEntry{
		Actor:      actor,
		Action:     "learning_data_cleared",
		TargetType: "learning",
		TargetID:   "candidates",
		Details:    "Purged all discovery candidates and temporary learning data",
	})

	return nil
}

// GenerateReport compiles a comprehensive JSON-exportable Learning Report.
func (lm *LearningManager) GenerateReport() (*LearningReport, error) {
	settings, err := lm.store.GetLearningSettings()
	if err != nil {
		return nil, err
	}

	candidates, err := lm.store.ListCandidates("")
	if err != nil {
		return nil, err
	}

	auditLogs, _ := lm.store.ListAuditLogs(50)
	profiles, _ := lm.store.ListGameProfiles()

	profileIDs := make([]string, 0, len(profiles))
	for _, p := range profiles {
		if p.Enabled {
			profileIDs = append(profileIDs, p.ID)
		}
	}

	pendingCount := 0
	for _, c := range candidates {
		if c.Status == StatusCandidate || c.Status == StatusNeedsReview {
			pendingCount++
		}
	}

	report := &LearningReport{
		Mode:               settings.Mode,
		GeneratedAt:        time.Now(),
		TotalObserved:      len(candidates),
		PendingReviewCount: pendingCount,
		Candidates:         candidates,
		RecentAuditLogs:    auditLogs,
		ActiveProfiles:     profileIDs,
	}

	return report, nil
}

// ExportReportJSON returns the learning report as indented JSON bytes.
func (lm *LearningManager) ExportReportJSON() ([]byte, error) {
	report, err := lm.GenerateReport()
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(report, "", "  ")
}

