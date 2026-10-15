package game

import (
	"time"
)

// DefaultSeedProfiles returns initial pre-configured profiles for FC 26, CS2, and War Thunder.
func DefaultSeedProfiles() []GameProfile {
	now := time.Now()

	fc26Domains := map[string]DomainRecord{}
	addDomain := func(m map[string]DomainRecord, host string, cat DomainCategory, policy PolicyAction, gameID, pub string) {
		m[host] = DomainRecord{
			Hostname:         host,
			GameID:           gameID,
			Publisher:        pub,
			Category:         cat,
			Policy:           policy,
			ConfidenceScore:  98,
			Status:           StatusConfirmed,
			Evidence:         []string{"Official publisher endpoint", "Verified game telemetry"},
			FirstSeen:        now,
			LastSeen:         now,
			ObservationCount: 1,
			CreatedBy:        "seed",
			UpdatedAt:        now,
			Enabled:          true,
		}
	}

	// 1. EA SPORTS FC 26 (EA)
	fc26 := GameProfile{
		ID:        "fc26",
		Name:      "EA SPORTS FC 26",
		Publisher: "Electronic Arts",
		Enabled:   true,
		Domains:   fc26Domains,
		DNSPolicies: map[DomainCategory]string{
			CategoryAuth:         string(PolicyProxy),
			CategoryMatchmaking:  string(PolicyProxy),
			CategoryGameServices: string(PolicyProxy),
			CategoryCDN:          string(PolicyDirect),
			CategoryTelemetry:    string(PolicyDirect),
			CategoryCandidate:    string(PolicyDirect),
		},
		RoutePolicy: GameRoutePolicy{
			AutoSelectEnabled:    true,
			PreferredNodeID:      "node_tr",
			FallbackNodeIDs:      []string{"node_de", "node_fi"},
			MaxRTTThresholdMs:    80.0,
			MaxLossThresholdPct:  1.5,
			MaxJitterThresholdMs: 8.0,
		},
		DiscoverySettings: GameDiscoveryConfig{
			Enabled:               true,
			AutoCandidateMinScore: 70,
			KeywordPatterns:       []string{"fc26", "fifa", "fut", "easports", "river.data.ea"},
			KnownRootDomains:      []string{"ea.com", "origin.com", "eaassets-a.akamaihd.net"},
		},
		ConfidenceScore: 95,
		UpdateHistory: []ProfileHistoryEntry{
			{
				Version:   1,
				Timestamp: now,
				Actor:     "system_seed",
				Summary:   "Initialized EA SPORTS FC 26 profile with official EA/FUT endpoints",
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// FC 26 Domains
	addDomain(fc26Domains, "accounts.ea.com", CategoryAuth, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "signin.ea.com", CategoryAuth, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "login.ea.com", CategoryAuth, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "identity.ea.com", CategoryAuth, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "nucleus.ea.com", CategoryAuth, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "connect.ea.com", CategoryAuth, PolicyProxy, "fc26", "Electronic Arts")

	addDomain(fc26Domains, "utas.fut.ea.com", CategoryMatchmaking, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "utas.mob.fut.ea.com", CategoryMatchmaking, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "fut.ea.com", CategoryMatchmaking, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "pin-river.data.ea.com", CategoryMatchmaking, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "fc26-matchmaking.ea.com", CategoryMatchmaking, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "fc25-matchmaking.ea.com", CategoryMatchmaking, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "fc24-matchmaking.ea.com", CategoryMatchmaking, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "fifa-server.ea.com", CategoryMatchmaking, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "gosredirector.ea.com", CategoryMatchmaking, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "eadp-sdk.ea.com", CategoryMatchmaking, PolicyProxy, "fc26", "Electronic Arts")

	addDomain(fc26Domains, "ea.com", CategoryGameServices, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "origin.com", CategoryGameServices, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "api1.origin.com", CategoryGameServices, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "api2.origin.com", CategoryGameServices, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "api3.origin.com", CategoryGameServices, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "service-ea.net", CategoryGameServices, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "river.data.ea.com", CategoryGameServices, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "eafc.com", CategoryGameServices, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "electronicarts.com", CategoryGameServices, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "help.ea.com", CategoryGameServices, PolicyProxy, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "answers.ea.com", CategoryGameServices, PolicyProxy, "fc26", "Electronic Arts")

	addDomain(fc26Domains, "origin-a.akamaihd.net", CategoryCDN, PolicyDirect, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "eaassets-a.akamaihd.net", CategoryCDN, PolicyDirect, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "download.origin.com", CategoryCDN, PolicyDirect, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "eacdn.com", CategoryCDN, PolicyDirect, "fc26", "Electronic Arts")

	addDomain(fc26Domains, "telemetry.ea.com", CategoryTelemetry, PolicyDirect, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "pin-emitter.ea.com", CategoryTelemetry, PolicyDirect, "fc26", "Electronic Arts")
	addDomain(fc26Domains, "pin-dragon.ea.com", CategoryTelemetry, PolicyDirect, "fc26", "Electronic Arts")

	// 2. Counter-Strike 2 (CS2) (Valve)
	cs2Domains := map[string]DomainRecord{}
	cs2 := GameProfile{
		ID:        "cs2",
		Name:      "Counter-Strike 2",
		Publisher: "Valve Corporation",
		Enabled:   true,
		Domains:   cs2Domains,
		DNSPolicies: map[DomainCategory]string{
			CategoryAuth:         string(PolicyProxy),
			CategoryMatchmaking:  string(PolicyProxy),
			CategoryGameServices: string(PolicyProxy),
			CategoryCDN:          string(PolicyDirect),
			CategoryTelemetry:    string(PolicyDirect),
			CategoryCandidate:    string(PolicyDirect),
		},
		RoutePolicy: GameRoutePolicy{
			AutoSelectEnabled:    true,
			PreferredNodeID:      "node_tr",
			FallbackNodeIDs:      []string{"node_de", "node_fi"},
			MaxRTTThresholdMs:    70.0,
			MaxLossThresholdPct:  1.0,
			MaxJitterThresholdMs: 5.0,
		},
		DiscoverySettings: GameDiscoveryConfig{
			Enabled:               true,
			AutoCandidateMinScore: 75,
			KeywordPatterns:       []string{"cs2", "counterstrike", "valvesoftware", "steampowered", "steamserver"},
			KnownRootDomains:      []string{"counterstrike.net", "valvesoftware.com", "steampowered.com", "steamcommunity.com"},
		},
		ConfidenceScore: 98,
		UpdateHistory: []ProfileHistoryEntry{
			{
				Version:   1,
				Timestamp: now,
				Actor:     "system_seed",
				Summary:   "Initialized Counter-Strike 2 profile with official Steam/Valve network endpoints",
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// CS2 Domains
	addDomain(cs2Domains, "login.steampowered.com", CategoryAuth, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "steamcommunity.com", CategoryAuth, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "api.steampowered.com", CategoryAuth, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "help.steampowered.com", CategoryAuth, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "store.steampowered.com", CategoryAuth, PolicyProxy, "cs2", "Valve Corporation")

	addDomain(cs2Domains, "counterstrike.net", CategoryMatchmaking, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "valve.net", CategoryMatchmaking, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "steamserver.net", CategoryMatchmaking, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "relay.valvesoftware.com", CategoryMatchmaking, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "csgo-servers.valvesoftware.com", CategoryMatchmaking, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "coordinator.counter-strike.net", CategoryMatchmaking, PolicyProxy, "cs2", "Valve Corporation")

	addDomain(cs2Domains, "valvesoftware.com", CategoryGameServices, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "steampowered.com", CategoryGameServices, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "steamstatic.com", CategoryGameServices, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "steamgames.com", CategoryGameServices, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "steam-chat.com", CategoryGameServices, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "steamdeck.com", CategoryGameServices, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "faceit.com", CategoryGameServices, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "faceitcdn.net", CategoryGameServices, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "faceit-cdn.net", CategoryGameServices, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "faceit.net", CategoryGameServices, PolicyProxy, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "faceitstatic.com", CategoryGameServices, PolicyProxy, "cs2", "Valve Corporation")

	addDomain(cs2Domains, "steamcontent.com", CategoryCDN, PolicyDirect, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "content1.steampowered.com", CategoryCDN, PolicyDirect, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "content2.steampowered.com", CategoryCDN, PolicyDirect, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "cdn.steampowered.com", CategoryCDN, PolicyDirect, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "steamcommunity-a.akamaihd.net", CategoryCDN, PolicyDirect, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "steamusercontent-a.akamaihd.net", CategoryCDN, PolicyDirect, "cs2", "Valve Corporation")

	addDomain(cs2Domains, "crash.steampowered.com", CategoryTelemetry, PolicyDirect, "cs2", "Valve Corporation")
	addDomain(cs2Domains, "stats.steampowered.com", CategoryTelemetry, PolicyDirect, "cs2", "Valve Corporation")

	// 3. War Thunder (Gaijin Entertainment)
	wtDomains := map[string]DomainRecord{}
	wt := GameProfile{
		ID:        "warthunder",
		Name:      "War Thunder",
		Publisher: "Gaijin Entertainment",
		Enabled:   true,
		Domains:   wtDomains,
		DNSPolicies: map[DomainCategory]string{
			CategoryAuth:         string(PolicyProxy),
			CategoryMatchmaking:  string(PolicyProxy),
			CategoryGameServices: string(PolicyProxy),
			CategoryCDN:          string(PolicyDirect),
			CategoryTelemetry:    string(PolicyDirect),
			CategoryCandidate:    string(PolicyDirect),
		},
		RoutePolicy: GameRoutePolicy{
			AutoSelectEnabled:    true,
			PreferredNodeID:      "node_tr",
			FallbackNodeIDs:      []string{"node_de", "node_fi"},
			MaxRTTThresholdMs:    85.0,
			MaxLossThresholdPct:  2.0,
			MaxJitterThresholdMs: 8.0,
		},
		DiscoverySettings: GameDiscoveryConfig{
			Enabled:               true,
			AutoCandidateMinScore: 70,
			KeywordPatterns:       []string{"warthunder", "gaijin", "wt-matchmaking", "yupmaster"},
			KnownRootDomains:      []string{"warthunder.com", "gaijin.net", "gaijinent.com"},
		},
		ConfidenceScore: 94,
		UpdateHistory: []ProfileHistoryEntry{
			{
				Version:   1,
				Timestamp: now,
				Actor:     "system_seed",
				Summary:   "Initialized War Thunder profile with Gaijin Entertainment network endpoints",
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// War Thunder Domains
	addDomain(wtDomains, "login.gaijin.net", CategoryAuth, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "auth.gaijin.net", CategoryAuth, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "gaijin.net", CategoryAuth, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "store.gaijin.net", CategoryAuth, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "sso.gaijin.net", CategoryAuth, PolicyProxy, "warthunder", "Gaijin Entertainment")

	addDomain(wtDomains, "warthunder.com", CategoryMatchmaking, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "matchmaking.warthunder.com", CategoryMatchmaking, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "game.warthunder.com", CategoryMatchmaking, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "wt-matchmaking.gaijin.net", CategoryMatchmaking, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "matchmaker.warthunder.com", CategoryMatchmaking, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "cluster.warthunder.com", CategoryMatchmaking, PolicyProxy, "warthunder", "Gaijin Entertainment")

	addDomain(wtDomains, "gaijinent.com", CategoryGameServices, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "gaijin.ru", CategoryGameServices, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "cdn.gaijin.net", CategoryGameServices, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "live.warthunder.com", CategoryGameServices, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "api.warthunder.com", CategoryGameServices, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "gaijin.systems", CategoryGameServices, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "enlisted.net", CategoryGameServices, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "crossout.net", CategoryGameServices, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "wiki.warthunder.com", CategoryGameServices, PolicyProxy, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "forum.warthunder.com", CategoryGameServices, PolicyProxy, "warthunder", "Gaijin Entertainment")

	// Anti-Cheat (EAC / BattlEye for War Thunder)
	addDomain(wtDomains, "easyanticheat.net", CategoryAuth, PolicyProxy, "warthunder", "EasyAntiCheat")
	addDomain(wtDomains, "eac-cdn.com", CategoryAuth, PolicyProxy, "warthunder", "EasyAntiCheat")
	addDomain(wtDomains, "modules-cdn.eac-prod.on.epicgames.com", CategoryAuth, PolicyProxy, "warthunder", "EasyAntiCheat")
	addDomain(wtDomains, "on.epicgames.com", CategoryAuth, PolicyProxy, "warthunder", "Epic Games")
	addDomain(wtDomains, "epicgames.dev", CategoryAuth, PolicyProxy, "warthunder", "Epic Games")
	addDomain(wtDomains, "battleye.com", CategoryAuth, PolicyProxy, "warthunder", "BattlEye")

	addDomain(wtDomains, "yupmaster.gaijinent.com", CategoryCDN, PolicyDirect, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "content.warthunder.com", CategoryCDN, PolicyDirect, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "distr.gaijinent.com", CategoryCDN, PolicyDirect, "warthunder", "Gaijin Entertainment")

	addDomain(wtDomains, "stat.gaijinent.com", CategoryTelemetry, PolicyDirect, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "telemetry.warthunder.com", CategoryTelemetry, PolicyDirect, "warthunder", "Gaijin Entertainment")
	addDomain(wtDomains, "log.gaijinent.com", CategoryTelemetry, PolicyDirect, "warthunder", "Gaijin Entertainment")

	// 4. Nintendo Switch (Nintendo)
	nintendoDomains := map[string]DomainRecord{}
	nintendoProfile := GameProfile{
		ID:        "nintendo",
		Name:      "Nintendo Switch Online",
		Publisher: "Nintendo",
		Enabled:   true,
		Domains:   nintendoDomains,
		DNSPolicies: map[DomainCategory]string{
			CategoryAuth:         string(PolicyProxy),
			CategoryMatchmaking:  string(PolicyProxy),
			CategoryGameServices: string(PolicyProxy),
			CategoryCDN:          string(PolicyDirect),
			CategoryTelemetry:    string(PolicyDirect),
			CategoryCandidate:    string(PolicyDirect),
		},
		RoutePolicy: GameRoutePolicy{
			AutoSelectEnabled:    true,
			PreferredNodeID:      "node_tr",
			FallbackNodeIDs:      []string{"node_de", "node_fi"},
			MaxRTTThresholdMs:    80.0,
			MaxLossThresholdPct:  1.5,
			MaxJitterThresholdMs: 6.0,
		},
		DiscoverySettings: GameDiscoveryConfig{
			Enabled:               true,
			AutoCandidateMinScore: 70,
			KeywordPatterns:       []string{"nintendo", "nintendowifi", "nintendoswitch"},
			KnownRootDomains:      []string{"nintendo.com", "nintendo.net", "nintendowifi.net", "nintendo-europe.com", "nintendo.co.jp"},
		},
		ConfidenceScore: 98,
		UpdateHistory: []ProfileHistoryEntry{
			{
				Version:   1,
				Timestamp: now,
				Actor:     "system_seed",
				Summary:   "Initialized Nintendo Switch Online profile with official Nintendo NPLN endpoints",
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Nintendo Switch Domains
	addDomain(nintendoDomains, "accounts.nintendo.com", CategoryAuth, PolicyProxy, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "api.accounts.nintendo.com", CategoryAuth, PolicyProxy, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "dauth-lp1.ndas.srv.nintendo.net", CategoryAuth, PolicyProxy, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "aauth-lp1.ndas.srv.nintendo.net", CategoryAuth, PolicyProxy, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "e-eshop.nintendo.net", CategoryAuth, PolicyProxy, "nintendo", "Nintendo")

	addDomain(nintendoDomains, "beach.srv.nintendo.net", CategoryMatchmaking, PolicyProxy, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "dragons.srv.nintendo.net", CategoryMatchmaking, PolicyProxy, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "sun.hac.lp1.d4c.nintendo.net", CategoryMatchmaking, PolicyProxy, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "ecs.nintendo.net", CategoryMatchmaking, PolicyProxy, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "receive.nintendo.net", CategoryMatchmaking, PolicyProxy, "nintendo", "Nintendo")

	addDomain(nintendoDomains, "nintendo.com", CategoryGameServices, PolicyProxy, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "nintendo.net", CategoryGameServices, PolicyProxy, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "nintendowifi.net", CategoryGameServices, PolicyProxy, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "nintendo-europe.com", CategoryGameServices, PolicyProxy, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "nintendo.co.jp", CategoryGameServices, PolicyProxy, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "nintendoswitch.com", CategoryGameServices, PolicyProxy, "nintendo", "Nintendo")

	addDomain(nintendoDomains, "ctest.cdn.nintendo.net", CategoryCDN, PolicyDirect, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "conntest.nintendowifi.net", CategoryCDN, PolicyDirect, "nintendo", "Nintendo")
	addDomain(nintendoDomains, "atumn.hac.lp1.d4c.nintendo.net", CategoryCDN, PolicyDirect, "nintendo", "Nintendo")

	addDomain(nintendoDomains, "receive-lp1.er.srv.nintendo.net", CategoryTelemetry, PolicyDirect, "nintendo", "Nintendo")

	return []GameProfile{fc26, cs2, wt, nintendoProfile}
}

// DefaultSeedRouteNodes returns the default VPS nodes representing edge routes.
func DefaultSeedRouteNodes() []RouteNode {
	now := time.Now()
	return []RouteNode{
		{
			ID:             "node_tr",
			Name:           "Turkey (Istanbul)",
			Region:         "tr",
			Endpoint:       "tr.JozmozDNS.network",
			Port:           443,
			IsMaster:       false,
			Active:         true,
			RTTMs:          0.0,
			PacketLossPct:  0.0,
			JitterMs:       0.0,
			StabilityScore: 95.0,
			Status:         NodeHealthy,
			LastCheck:      now,
			SyncStatus:     SyncSynced,
			Version:        "2.3.0",
		},
		{
			ID:             "node_de",
			Name:           "Germany (Frankfurt)",
			Region:         "de",
			Endpoint:       "de.JozmozDNS.network",
			Port:           443,
			IsMaster:       false,
			Active:         true,
			RTTMs:          0.0,
			PacketLossPct:  0.0,
			JitterMs:       0.0,
			StabilityScore: 92.0,
			Status:         NodeHealthy,
			LastCheck:      now,
			SyncStatus:     SyncSynced,
			Version:        "2.3.0",
		},
		{
			ID:             "node_fi",
			Name:           "Finland (Helsinki)",
			Region:         "fi",
			Endpoint:       "fi.JozmozDNS.network",
			Port:           443,
			IsMaster:       false,
			Active:         true,
			RTTMs:          0.0,
			PacketLossPct:  0.0,
			JitterMs:       0.0,
			StabilityScore: 88.0,
			Status:         NodeHealthy,
			LastCheck:      now,
			SyncStatus:     SyncSynced,
			Version:        "2.3.0",
		},
	}
}


