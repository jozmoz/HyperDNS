package game

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
	"hyperdns/internal/database"
)

var (
	bucketGames       = []byte("game_profiles")
	bucketDomains     = []byte("domain_intelligence")
	bucketRoutes      = []byte("route_nodes")
	bucketDiscoveries = []byte("discoveries")
	bucketAuditLogs   = []byte("audit_logs")
	bucketSettings    = []byte("settings")
)

// Store handles bbolt persistence for all HGI components.
type Store struct {
	db *database.DB
	mu sync.RWMutex
}

// NewStore initializes a new Store instance.
func NewStore(db *database.DB) *Store {
	return &Store{db: db}
}

// InitSeeds initializes default game profiles and route nodes if the store is empty.
func (s *Store) InitSeeds() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	boltDB := s.db.Bolt()
	if boltDB == nil {
		return fmt.Errorf("database not initialized")
	}

	return boltDB.Update(func(tx *bolt.Tx) error {
		// Ensure buckets exist
		bGames, err := tx.CreateBucketIfNotExists(bucketGames)
		if err != nil {
			return err
		}
		bDomains, err := tx.CreateBucketIfNotExists(bucketDomains)
		if err != nil {
			return err
		}
		bRoutes, err := tx.CreateBucketIfNotExists(bucketRoutes)
		if err != nil {
			return err
		}
		_, _ = tx.CreateBucketIfNotExists(bucketDiscoveries)
		_, _ = tx.CreateBucketIfNotExists(bucketAuditLogs)

		// Seed games if bucket is empty
		if bGames.Stats().KeyN == 0 {
			seeds := DefaultSeedProfiles()
			for _, profile := range seeds {
				data, err := json.Marshal(profile)
				if err != nil {
					return err
				}
				if err := bGames.Put([]byte(profile.ID), data); err != nil {
					return err
				}

				// Also populate domain intelligence table
				for _, dom := range profile.Domains {
					domData, err := json.Marshal(dom)
					if err == nil {
						_ = bDomains.Put([]byte(normalizeDomain(dom.Hostname)), domData)
					}
				}
			}
		}

		// Seed routes if bucket is empty
		if bRoutes.Stats().KeyN == 0 {
			nodes := DefaultSeedRouteNodes()
			for _, node := range nodes {
				data, err := json.Marshal(node)
				if err != nil {
					return err
				}
				if err := bRoutes.Put([]byte(node.ID), data); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// normalizeDomain helper
func normalizeDomain(domain string) string {
	d := strings.TrimSuffix(strings.TrimSpace(domain), ".")
	return strings.ToLower(d)
}

// --- Game Profiles ---

// SaveGameProfile saves or updates a game profile.
func (s *Store) SaveGameProfile(p GameProfile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p.UpdatedAt = time.Now()
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}

	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketGames)
		if b == nil {
			return fmt.Errorf("game_profiles bucket missing")
		}
		return b.Put([]byte(p.ID), data)
	})
}

// GetGameProfile retrieves a game profile by its ID.
func (s *Store) GetGameProfile(id string) (*GameProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var p GameProfile
	err := s.db.Bolt().View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketGames)
		if b == nil {
			return fmt.Errorf("game_profiles bucket missing")
		}
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("game profile %q not found", id)
		}
		return json.Unmarshal(data, &p)
	})
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListGameProfiles returns all configured game profiles.
func (s *Store) ListGameProfiles() ([]GameProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	profiles := make([]GameProfile, 0)
	err := s.db.Bolt().View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketGames)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var p GameProfile
			if err := json.Unmarshal(v, &p); err == nil {
				profiles = append(profiles, p)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})
	return profiles, nil
}

// DeleteGameProfile deletes a game profile.
func (s *Store) DeleteGameProfile(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketGames)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(id))
	})
}

// --- Domain Intelligence ---

// SaveDomainIntel stores a domain intelligence record.
func (s *Store) SaveDomainIntel(d DomainRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	d.Hostname = normalizeDomain(d.Hostname)
	d.UpdatedAt = time.Now()
	data, err := json.Marshal(d)
	if err != nil {
		return err
	}

	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDomains)
		if b == nil {
			return fmt.Errorf("domain_intelligence bucket missing")
		}
		return b.Put([]byte(d.Hostname), data)
	})
}

// GetDomainIntel retrieves domain intelligence by hostname.
func (s *Store) GetDomainIntel(hostname string) (*DomainRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	norm := normalizeDomain(hostname)
	var d DomainRecord
	err := s.db.Bolt().View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDomains)
		if b == nil {
			return fmt.Errorf("domain_intelligence bucket missing")
		}
		data := b.Get([]byte(norm))
		if data == nil {
			return fmt.Errorf("domain %q not found", norm)
		}
		return json.Unmarshal(data, &d)
	})
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// ListDomainIntel lists domain records filtered by optional gameID, category, status.
func (s *Store) ListDomainIntel(gameID, category, status string) ([]DomainRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]DomainRecord, 0)
	err := s.db.Bolt().View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDomains)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var d DomainRecord
			if err := json.Unmarshal(v, &d); err == nil {
				if gameID != "" && d.GameID != gameID {
					return nil
				}
				if category != "" && string(d.Category) != category {
					return nil
				}
				if status != "" && string(d.Status) != status {
					return nil
				}
				results = append(results, d)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Hostname < results[j].Hostname
	})
	return results, nil
}

// DeleteDomainIntel removes a domain intelligence record.
func (s *Store) DeleteDomainIntel(hostname string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	norm := normalizeDomain(hostname)
	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDomains)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(norm))
	})
}

// --- Route Nodes ---

// SaveRouteNode stores or updates a route node.
func (s *Store) SaveRouteNode(node RouteNode) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(node)
	if err != nil {
		return err
	}

	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRoutes)
		if b == nil {
			return fmt.Errorf("route_nodes bucket missing")
		}
		return b.Put([]byte(node.ID), data)
	})
}

// GetRouteNode retrieves a route node by its ID.
func (s *Store) GetRouteNode(id string) (*RouteNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var node RouteNode
	err := s.db.Bolt().View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRoutes)
		if b == nil {
			return fmt.Errorf("route_nodes bucket missing")
		}
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("route node %q not found", id)
		}
		return json.Unmarshal(data, &node)
	})
	if err != nil {
		return nil, err
	}
	return &node, nil
}

// ListRouteNodes returns all configured route nodes.
func (s *Store) ListRouteNodes() ([]RouteNode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]RouteNode, 0)
	err := s.db.Bolt().View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRoutes)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var node RouteNode
			if err := json.Unmarshal(v, &node); err == nil {
				nodes = append(nodes, node)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})
	return nodes, nil
}

// DeleteRouteNode removes a route node.
func (s *Store) DeleteRouteNode(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketRoutes)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(id))
	})
}

// --- Discovery Candidates ---

// SaveCandidate persists an unconfirmed candidate.
func (s *Store) SaveCandidate(c DiscoveryCandidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c.Hostname = normalizeDomain(c.Hostname)
	if c.ID == "" {
		c.ID = fmt.Sprintf("cand_%x", c.Hostname)
	}
	c.UpdatedAt = time.Now()
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}

	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDiscoveries)
		if b == nil {
			return fmt.Errorf("discoveries bucket missing")
		}
		return b.Put([]byte(c.ID), data)
	})
}

// GetCandidate retrieves a candidate by ID.
func (s *Store) GetCandidate(id string) (*DiscoveryCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var c DiscoveryCandidate
	err := s.db.Bolt().View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDiscoveries)
		if b == nil {
			return fmt.Errorf("discoveries bucket missing")
		}
		data := b.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("candidate %q not found", id)
		}
		return json.Unmarshal(data, &c)
	})
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListCandidates returns all candidates, optionally filtered by status.
func (s *Store) ListCandidates(status string) ([]DiscoveryCandidate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	candidates := make([]DiscoveryCandidate, 0)
	err := s.db.Bolt().View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDiscoveries)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var c DiscoveryCandidate
			if err := json.Unmarshal(v, &c); err == nil {
				if status != "" && string(c.Status) != status {
					return nil
				}
				candidates = append(candidates, c)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].LastSeen.After(candidates[j].LastSeen)
	})
	return candidates, nil
}

// DeleteCandidate removes a candidate from the bucket.
func (s *Store) DeleteCandidate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDiscoveries)
		if b == nil {
			return nil
		}
		return b.Delete([]byte(id))
	})
}

// ClearCandidates drops all candidates (for reset / cleanup).
func (s *Store) ClearCandidates() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(bucketDiscoveries); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		_, err := tx.CreateBucket(bucketDiscoveries)
		return err
	})
}

// --- Audit Logs ---

// AppendAuditLog appends an audit entry.
func (s *Store) AppendAuditLog(entry AuditEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry.ID == "" {
		entry.ID = fmt.Sprintf("audit_%d", time.Now().UnixNano())
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAuditLogs)
		if b == nil {
			return fmt.Errorf("audit_logs bucket missing")
		}
		return b.Put([]byte(entry.ID), data)
	})
}

// ListAuditLogs returns recent audit logs up to limit.
func (s *Store) ListAuditLogs(limit int) ([]AuditEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}
	logs := make([]AuditEntry, 0)
	err := s.db.Bolt().View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketAuditLogs)
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var e AuditEntry
			if err := json.Unmarshal(v, &e); err == nil {
				logs = append(logs, e)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(logs, func(i, j int) bool {
		return logs[i].Timestamp.After(logs[j].Timestamp)
	})
	if len(logs) > limit {
		logs = logs[:limit]
	}
	return logs, nil
}

// --- Learning Mode Settings ---

// GetLearningSettings retrieves stored learning settings or returns defaults.
func (s *Store) GetLearningSettings() (*LearningSettings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var cfg LearningSettings
	err := s.db.Bolt().View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		if b == nil {
			return nil
		}
		data := b.Get([]byte("hgi_learning"))
		if data == nil {
			return nil
		}
		return json.Unmarshal(data, &cfg)
	})
	if err != nil {
		return nil, err
	}
	if cfg.Mode == "" {
		cfg = LearningSettings{
			Mode:                  ModeObserve,
			AutoApplyMinScore:     85,
			AllowAutoProxy:        false,
			AllowAutoBlock:        false,
			RetentionDays:         7,
			AnonymizeQueryData:    false,
			EnableActiveDetection: true,
		}
	}
	return &cfg, nil
}

// SaveLearningSettings stores learning settings.
func (s *Store) SaveLearningSettings(cfg LearningSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	return s.db.Bolt().Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketSettings)
		if b == nil {
			return fmt.Errorf("settings bucket missing")
		}
		return b.Put([]byte("hgi_learning"), data)
	})
}

