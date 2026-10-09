// HyperDNS Game Intelligence Engine (HGI) UI Controller (v2.2)
// ES Module loaded after app.js. Bridges to window.__hdns for auth & API communication.

(() => {
  'use strict';

  const S = () => window.__hdns;
  const authToken = () => (S() ? S().getToken() : '');
  const api = (p) => (S() ? S().api(p) : p);
  const showToast = (m, t) => (S() ? S().showToast(m, t) : console.log(m));
  const errorMessage = (r, f) => (S() ? S().errorMessage(r, f) : Promise.resolve(f));

  function escapeHTML(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  }

  function safeFeatherReplace() {
    try {
      if (typeof feather !== 'undefined' && feather.replace) {
        feather.replace();
      }
    } catch (e) {}
  }

  // State
  let cachedProfiles = [];
  let cachedCandidates = [];
  let cachedRoutes = [];
  let cachedLearningSettings = { mode: 'observe', auto_apply_safe: true, min_confidence: 85 };
  let currentActiveGame = null;
  let activeSubView = 'profiles';

  // Sub-view switcher
  function switchHGIView(viewName) {
    activeSubView = viewName;
    document.querySelectorAll('[data-hgi-view]').forEach(btn => {
      const isTarget = btn.getAttribute('data-hgi-view') === viewName;
      if (isTarget) {
        btn.classList.add('bg-cyan-500/20', 'text-cyan-300', 'border-cyan-500/50');
        btn.classList.remove('bg-slate-900', 'text-slate-400', 'border-slate-800');
      } else {
        btn.classList.remove('bg-cyan-500/20', 'text-cyan-300', 'border-cyan-500/50');
        btn.classList.add('bg-slate-900', 'text-slate-400', 'border-slate-800');
      }
    });

    ['profiles', 'discovery', 'routes', 'simulator', 'timeline', 'ai'].forEach(v => {
      const panel = document.getElementById(`hgi-view-${v}`);
      if (panel) {
        panel.classList.toggle('hidden', v !== viewName);
      }
    });

    safeFeatherReplace();
  }

  // API Client Helper
  async function fetchHGI(endpoint, options = {}) {
    const token = authToken();
    const headers = {
      'Content-Type': 'application/json',
      ...(token ? { 'Authorization': `Bearer ${token}` } : {}),
      ...(options.headers || {})
    };
    return fetch(api(endpoint), { ...options, headers });
  }

  // Load All HGI Data
  async function loadGamingData() {
    try {
      await Promise.allSettled([
        loadProfiles(),
        loadCandidates(),
        loadRoutes(),
        loadLearningSettings(),
        loadActiveGame(),
        loadTimeline(),
        loadAuditLogs()
      ]);
      updateKPIs();
      safeFeatherReplace();
    } catch (err) {
      console.error('Failed to load HGI data:', err);
    }
  }

  // Update Top KPIs
  function updateKPIs() {
    const profCountEl = document.getElementById('hgi-stat-profiles');
    if (profCountEl) profCountEl.innerText = cachedProfiles.length || '3';

    const candCountEl = document.getElementById('hgi-stat-candidates');
    if (candCountEl) candCountEl.innerText = cachedCandidates.length || '0';

    const modeEl = document.getElementById('hgi-stat-learning-mode');
    if (modeEl) {
      const mode = (cachedLearningSettings.mode || 'observe').toUpperCase();
      modeEl.innerText = mode;
      modeEl.className = mode === 'AUTO-APPLY' 
        ? 'px-2 py-0.5 rounded text-[10px] font-bold bg-amber-500/20 text-amber-300 border border-amber-500/30 font-mono'
        : (mode === 'RECOMMEND' 
            ? 'px-2 py-0.5 rounded text-[10px] font-bold bg-purple-500/20 text-purple-300 border border-purple-500/30 font-mono'
            : 'px-2 py-0.5 rounded text-[10px] font-bold bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 font-mono');
    }

    const gameEl = document.getElementById('hgi-stat-active-game');
    const dotEl = document.getElementById('hgi-game-detect-dot');
    if (gameEl) {
      if (currentActiveGame && currentActiveGame.game_name) {
        gameEl.innerText = `${currentActiveGame.game_name} (${Math.round(currentActiveGame.confidence || 95)}%)`;
        gameEl.className = 'text-sm font-bold text-emerald-400 font-heading truncate';
        if (dotEl) dotEl.className = 'w-2 h-2 rounded-full bg-emerald-400 animate-pulse';
      } else {
        gameEl.innerText = 'No active game detected';
        gameEl.className = 'text-xs text-slate-400 font-heading truncate';
        if (dotEl) dotEl.className = 'w-2 h-2 rounded-full bg-slate-600';
      }
    }

    const bestNodeEl = document.getElementById('hgi-stat-best-node');
    if (bestNodeEl) {
      const healthy = cachedRoutes.filter(r => r.healthy);
      if (healthy.length > 0) {
        // pick lowest latency
        healthy.sort((a, b) => a.latency_ms - b.latency_ms);
        const best = healthy[0];
        bestNodeEl.innerText = `${best.name} (${best.latency_ms.toFixed(0)} ms)`;
        bestNodeEl.className = 'text-sm font-bold text-emerald-400 font-mono';
      } else if (cachedRoutes.length > 0) {
        bestNodeEl.innerText = 'Measuring routes...';
        bestNodeEl.className = 'text-xs text-amber-400 font-mono';
      } else {
        bestNodeEl.innerText = 'No route nodes';
        bestNodeEl.className = 'text-xs text-slate-500 font-mono';
      }
    }
  }

  // 1. Profiles & Domains View
  async function loadProfiles() {
    try {
      const res = await fetchHGI('/api/v2/games');
      if (!res.ok) return;
      const data = await res.json();
      cachedProfiles = data.profiles || [];
      renderProfiles();
    } catch (e) {
      console.warn('loadProfiles error', e);
    }
  }

  function renderProfiles() {
    const listEl = document.getElementById('hgi-profiles-grid');
    if (!listEl) return;

    if (!cachedProfiles.length) {
      listEl.innerHTML = '<div class="col-span-full p-8 text-center text-slate-500 text-xs">No game profiles registered. Click "+ Add Custom Game Profile" above.</div>';
      return;
    }

    listEl.innerHTML = cachedProfiles.map(p => {
      const authCount = (p.domains && p.domains.authentication) ? p.domains.authentication.length : 0;
      const matchCount = (p.domains && p.domains.matchmaking) ? p.domains.matchmaking.length : 0;
      const svcCount = (p.domains && p.domains.game_services) ? p.domains.game_services.length : 0;
      const cdnCount = (p.domains && p.domains.cdn_downloads) ? p.domains.cdn_downloads.length : 0;
      const totalDomains = authCount + matchCount + svcCount + cdnCount;

      const sampleDomains = [];
      if (p.domains) {
        if (p.domains.matchmaking && p.domains.matchmaking.length > 0) {
          sampleDomains.push(...p.domains.matchmaking.slice(0, 3));
        } else if (p.domains.game_services && p.domains.game_services.length > 0) {
          sampleDomains.push(...p.domains.game_services.slice(0, 3));
        }
      }

      return `
        <div class="glass-panel p-4 sm:p-5 border border-slate-800 hover:border-cyan-500/40 transition flex flex-col justify-between space-y-4">
          <div>
            <div class="flex items-start justify-between gap-2">
              <div class="flex items-center gap-3">
                <div class="w-10 h-10 rounded-xl bg-gradient-to-tr from-cyan-500/20 to-blue-500/20 border border-cyan-500/30 flex items-center justify-center text-cyan-400 font-bold font-mono text-sm">
                  ${escapeHTML((p.name || 'G').slice(0, 2).toUpperCase())}
                </div>
                <div>
                  <h4 class="text-sm font-bold text-white font-heading">${escapeHTML(p.name)}</h4>
                  <p class="text-[11px] text-cyan-400 font-mono">${escapeHTML(p.publisher || 'Unknown Publisher')}</p>
                </div>
              </div>
              <span class="px-2 py-0.5 rounded text-[10px] font-bold font-mono ${p.enabled ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' : 'bg-red-500/20 text-red-400 border border-red-500/30'}">
                ${p.enabled ? 'ACTIVE' : 'DISABLED'}
              </span>
            </div>

            <!-- Categories pills -->
            <div class="grid grid-cols-2 gap-2 mt-4 text-[10px] font-mono">
              <div class="p-2 rounded-lg bg-slate-950/80 border border-slate-800">
                <span class="text-slate-400">Matchmaking:</span>
                <span class="text-cyan-300 font-bold ms-1">${matchCount}</span>
              </div>
              <div class="p-2 rounded-lg bg-slate-950/80 border border-slate-800">
                <span class="text-slate-400">Services:</span>
                <span class="text-purple-300 font-bold ms-1">${svcCount}</span>
              </div>
              <div class="p-2 rounded-lg bg-slate-950/80 border border-slate-800">
                <span class="text-slate-400">Authentication:</span>
                <span class="text-emerald-300 font-bold ms-1">${authCount}</span>
              </div>
              <div class="p-2 rounded-lg bg-slate-950/80 border border-slate-800">
                <span class="text-slate-400">CDN/Direct:</span>
                <span class="text-amber-300 font-bold ms-1">${cdnCount}</span>
              </div>
            </div>

            <!-- Sample Domains list -->
            <div class="mt-3">
              <div class="text-[10px] uppercase font-mono text-slate-500 mb-1">Mapped Domains (${totalDomains} total)</div>
              <div class="space-y-1">
                ${sampleDomains.map(d => `<div class="text-[11px] font-mono text-slate-300 bg-slate-950/50 px-2 py-0.5 rounded border border-slate-800/80 truncate">${escapeHTML(d)}</div>`).join('')}
                ${totalDomains > sampleDomains.length ? `<div class="text-[10px] text-cyan-400 font-mono ps-1">+ ${totalDomains - sampleDomains.length} more domains</div>` : ''}
              </div>
            </div>
          </div>

          <!-- Action Buttons -->
          <div class="pt-3 border-t border-slate-800/80 flex items-center justify-between gap-2">
            <button class="btn-hgi-manage-domains px-2.5 py-1 rounded bg-cyan-500/15 hover:bg-cyan-500/25 text-cyan-300 text-[11px] font-semibold border border-cyan-500/30 transition flex items-center gap-1.5" data-game-id="${escapeHTML(p.id)}" title="Manage & Edit Domain Policies">
              <i data-feather="sliders" class="w-3 h-3 text-cyan-400"></i>
              <span>ویرایش سیاست‌ها</span>
            </button>
            <div class="flex items-center gap-1.5">
              <button class="btn-hgi-purge-game p-1.5 rounded bg-slate-900 hover:bg-slate-800 text-slate-400 hover:text-white border border-slate-800 transition" data-game-id="${escapeHTML(p.id)}" title="Purge DNS Cache for this game">
                <i data-feather="refresh-cw" class="w-3.5 h-3.5 text-cyan-400"></i>
              </button>
              <button class="btn-hgi-export-game p-1.5 rounded bg-slate-900 hover:bg-slate-800 text-slate-400 hover:text-white border border-slate-800 transition" data-game-id="${escapeHTML(p.id)}" title="Export Profile JSON">
                <i data-feather="download" class="w-3.5 h-3.5"></i>
              </button>
              <button class="btn-hgi-delete-game p-1.5 rounded bg-slate-900 hover:bg-red-500/20 text-slate-400 hover:text-red-400 border border-slate-800 transition" data-game-id="${escapeHTML(p.id)}" title="Delete Profile">
                <i data-feather="trash-2" class="w-3.5 h-3.5"></i>
              </button>
            </div>
          </div>
        </div>
      `;
    }).join('');

    safeFeatherReplace();
  }

  // --- Domain Policies Management Editor ---
  let currentEditProfile = null;

  async function openDomainPolicyEditor(gid) {
    const modal = document.getElementById('hgi-domain-policy-modal');
    if (!modal) return;

    try {
      const res = await fetchHGI(`/api/v2/games/${encodeURIComponent(gid)}`);
      if (!res.ok) {
        showToast('Failed to load game profile', 'error');
        return;
      }
      currentEditProfile = await res.json();
    } catch (e) {
      showToast('Error loading game profile', 'error');
      return;
    }

    const nameEl = document.getElementById('hgi-edit-game-name');
    const pubEl = document.getElementById('hgi-edit-game-publisher');
    if (nameEl) nameEl.textContent = currentEditProfile.name || gid;
    if (pubEl) pubEl.textContent = currentEditProfile.publisher || 'Unknown Publisher';

    // Set category default selects
    const pols = currentEditProfile.dns_policies || {};
    const catKeys = ['auth', 'matchmaking', 'game_services', 'cdn', 'telemetry'];
    catKeys.forEach(cat => {
      const el = document.getElementById(`hgi-cat-policy-${cat}`);
      if (el) {
        el.value = pols[cat] || (cat === 'cdn' || cat === 'telemetry' ? 'direct' : 'proxy');
      }
    });

    renderDomainRows();
    modal.classList.remove('hidden');
    safeFeatherReplace();
  }

  function renderDomainRows() {
    const tbody = document.getElementById('hgi-domains-tbody');
    const countEl = document.getElementById('hgi-edit-game-count');
    if (!tbody || !currentEditProfile) return;

    const searchTerm = (document.getElementById('hgi-domain-search')?.value || '').toLowerCase().trim();
    const filterCat = document.getElementById('hgi-filter-category')?.value || '';
    const filterPol = document.getElementById('hgi-filter-policy')?.value || '';

    const rawDomains = currentEditProfile.domains || {};
    let domainList = [];
    if (Array.isArray(rawDomains)) {
      domainList = rawDomains;
    } else {
      domainList = Object.values(rawDomains);
    }

    if (countEl) {
      countEl.textContent = `${domainList.length} دامنه‌ ثبت‌شده`;
    }

    const filtered = domainList.filter(d => {
      const host = (d.hostname || '').toLowerCase();
      if (searchTerm && !host.includes(searchTerm)) return false;
      if (filterCat && d.category !== filterCat) return false;
      if (filterPol && d.policy !== filterPol) return false;
      return true;
    });

    if (!filtered.length) {
      tbody.innerHTML = `<tr><td colspan="5" class="py-6 text-center text-slate-500 text-xs">هیچ دامنه‌ای با این فیلتر یافت نشد.</td></tr>`;
      return;
    }

    tbody.innerHTML = filtered.map(d => {
      const host = escapeHTML(d.hostname);
      const isProxy = d.policy === 'proxy';
      const isDirect = d.policy === 'direct';
      const isBlock = d.policy === 'block';

      return `
        <tr class="hover:bg-slate-900/50 transition">
          <td class="py-2.5 px-3 font-mono text-cyan-300 font-bold">${host}</td>
          <td class="py-2.5 px-3">
            <select class="sel-domain-cat bg-slate-900 border border-slate-700 text-slate-200 rounded px-2 py-1 text-xs font-mono" data-host="${host}">
              <option value="auth" ${d.category === 'auth' ? 'selected' : ''}>Authentication</option>
              <option value="matchmaking" ${d.category === 'matchmaking' ? 'selected' : ''}>Matchmaking</option>
              <option value="game_services" ${d.category === 'game_services' ? 'selected' : ''}>Game Services</option>
              <option value="cdn" ${d.category === 'cdn' ? 'selected' : ''}>CDN / Downloads</option>
              <option value="telemetry" ${d.category === 'telemetry' ? 'selected' : ''}>Telemetry</option>
            </select>
          </td>
          <td class="py-2.5 px-3">
            <select class="sel-domain-pol bg-slate-900 border ${isProxy ? 'border-cyan-500/50 text-cyan-400' : isDirect ? 'border-emerald-500/50 text-emerald-400' : 'border-red-500/50 text-red-400'} rounded px-2 py-1 text-xs font-mono font-bold" data-host="${host}">
              <option value="proxy" ${isProxy ? 'selected' : ''}>Proxy (هدایت به سرور)</option>
              <option value="direct" ${isDirect ? 'selected' : ''}>Direct (مستقیم اینترنت)</option>
              <option value="block" ${isBlock ? 'selected' : ''}>Block (مسدودسازی)</option>
            </select>
          </td>
          <td class="py-2.5 px-3 text-center">
            <span class="inline-block px-2 py-0.5 rounded text-[10px] font-bold ${d.enabled !== false ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' : 'bg-slate-800 text-slate-500 border border-slate-700'}">
              ${d.enabled !== false ? 'Active' : 'Disabled'}
            </span>
          </td>
          <td class="py-2.5 px-3 text-end whitespace-nowrap">
            <button class="btn-hgi-save-domain-row px-2.5 py-1 rounded bg-cyan-500/20 hover:bg-cyan-500/30 text-cyan-300 border border-cyan-500/30 text-xs font-semibold transition" data-host="${host}">
              ذخیره
            </button>
            <button class="btn-hgi-delete-domain-row p-1 rounded hover:bg-red-500/20 text-slate-400 hover:text-red-400 border border-slate-800 transition ms-1" data-host="${host}" title="Delete Domain">
              <i data-feather="trash-2" class="w-3.5 h-3.5"></i>
            </button>
          </td>
        </tr>
      `;
    }).join('');

    safeFeatherReplace();
  }

  // 2. Candidates & Discovery View
  async function loadCandidates() {
    try {
      const res = await fetchHGI('/api/v2/discovery/candidates');
      if (!res.ok) return;
      const data = await res.json();
      cachedCandidates = data.candidates || [];
      renderCandidates();
    } catch (e) {
      console.warn('loadCandidates error', e);
    }
  }

  function renderCandidates() {
    const tbody = document.getElementById('hgi-candidates-tbody');
    if (!tbody) return;

    if (!cachedCandidates.length) {
      tbody.innerHTML = `
        <tr>
          <td colspan="6" class="text-center py-8 text-slate-500 text-xs">
            No candidates pending review. Learning agent is monitoring incoming traffic.
          </td>
        </tr>
      `;
      return;
    }

    tbody.innerHTML = cachedCandidates.map(c => {
      const score = Math.round(c.confidence_score || 0);
      const scoreClass = score >= 80 ? 'text-emerald-400' : (score >= 50 ? 'text-amber-400' : 'text-slate-400');
      const timeStr = c.last_seen ? new Date(c.last_seen).toLocaleTimeString() : 'Recently';

      return `
        <tr class="hover:bg-slate-900/50 transition border-b border-slate-800/60">
          <td class="py-3 px-3 font-mono text-xs text-white">
            <span class="font-bold">${escapeHTML(c.hostname)}</span>
            <div class="text-[10px] text-slate-500">${escapeHTML(c.evidence || 'Observed DNS activity')}</div>
          </td>
          <td class="py-3 px-3 text-xs text-slate-300 font-mono">
            <span class="px-1.5 py-0.5 rounded bg-cyan-500/10 text-cyan-300 border border-cyan-500/20 text-[10px]">
              ${escapeHTML(c.game_id || 'Unknown')}
            </span>
          </td>
          <td class="py-3 px-3 text-xs font-mono text-purple-300">
            ${escapeHTML(c.category || 'Matchmaking')}
          </td>
          <td class="py-3 px-3 text-xs font-mono font-bold ${scoreClass}">
            ${score}%
          </td>
          <td class="py-3 px-3 text-xs font-mono text-slate-400">
            ${timeStr} (${c.observation_count || 1}x)
          </td>
          <td class="py-3 px-3 text-end">
            <div class="inline-flex items-center gap-1.5">
              <button class="btn-hgi-approve-candidate px-2 py-1 rounded bg-emerald-500/20 hover:bg-emerald-500/30 text-emerald-300 border border-emerald-500/40 text-[10px] font-bold font-mono transition" data-host="${escapeHTML(c.hostname)}" data-policy="proxy" title="Approve with Proxy Policy">
                Proxy
              </button>
              <button class="btn-hgi-approve-candidate px-2 py-1 rounded bg-cyan-500/20 hover:bg-cyan-500/30 text-cyan-300 border border-cyan-500/40 text-[10px] font-bold font-mono transition" data-host="${escapeHTML(c.hostname)}" data-policy="direct" title="Approve with Direct Policy">
                Direct
              </button>
              <button class="btn-hgi-reject-candidate p-1 rounded bg-slate-800 hover:bg-red-500/20 text-slate-400 hover:text-red-400 border border-slate-700 text-[10px] transition" data-host="${escapeHTML(c.hostname)}" title="Reject Candidate">
                <i data-feather="x" class="w-3.5 h-3.5"></i>
              </button>
            </div>
          </td>
        </tr>
      `;
    }).join('');

    safeFeatherReplace();
  }

  // 3. Routes & Quality Monitor View
  async function loadRoutes() {
    try {
      const res = await fetchHGI('/api/v2/routes');
      if (!res.ok) return;
      const data = await res.json();
      cachedRoutes = data.routes || [];
      renderRoutes();
    } catch (e) {
      console.warn('loadRoutes error', e);
    }
  }

  function renderRoutes() {
    const tbody = document.getElementById('hgi-routes-tbody');
    if (!tbody) return;

    if (!cachedRoutes.length) {
      tbody.innerHTML = `
        <tr>
          <td colspan="7" class="text-center py-8 text-slate-500 text-xs">
            No route measurements available. Click "Test Routes Now" to run active probes.
          </td>
        </tr>
      `;
      return;
    }

    tbody.innerHTML = cachedRoutes.map(r => {
      const rtt = typeof r.latency_ms === 'number' ? `${r.latency_ms.toFixed(1)} ms` : '—';
      const loss = typeof r.packet_loss === 'number' ? `${r.packet_loss.toFixed(1)}%` : '0%';
      const jitter = typeof r.jitter_ms === 'number' ? `${r.jitter_ms.toFixed(1)} ms` : '—';
      const stab = typeof r.stability_score === 'number' ? `${Math.round(r.stability_score)}%` : '—';

      const statusBadge = r.healthy 
        ? '<span class="px-2 py-0.5 rounded-full text-[10px] font-bold font-mono bg-emerald-500/20 text-emerald-400 border border-emerald-500/30">Healthy</span>'
        : '<span class="px-2 py-0.5 rounded-full text-[10px] font-bold font-mono bg-amber-500/20 text-amber-400 border border-amber-500/30">Degraded</span>';

      return `
        <tr class="hover:bg-slate-900/50 transition border-b border-slate-800/60">
          <td class="py-3 px-3 font-mono text-xs text-white">
            <div class="font-bold flex items-center gap-1.5">
              <span>${escapeHTML(r.name || r.node_id)}</span>
              ${r.is_active ? '<span class="text-[9px] px-1 rounded bg-cyan-500/20 text-cyan-300 border border-cyan-500/30">CURRENT</span>' : ''}
            </div>
            <div class="text-[10px] text-slate-500">${escapeHTML(r.node_id)} · ${escapeHTML(r.endpoint || '—')}</div>
          </td>
          <td class="py-3 px-3 font-mono text-xs text-cyan-300 font-bold">
            ${rtt}
          </td>
          <td class="py-3 px-3 font-mono text-xs ${r.packet_loss > 0 ? 'text-amber-400' : 'text-slate-300'}">
            ${loss}
          </td>
          <td class="py-3 px-3 font-mono text-xs text-slate-300">
            ${jitter}
          </td>
          <td class="py-3 px-3 font-mono text-xs text-emerald-400 font-bold">
            ${stab}
          </td>
          <td class="py-3 px-3">
            ${statusBadge}
          </td>
          <td class="py-3 px-3 text-end">
            <button class="btn-hgi-test-single-node px-2.5 py-1 rounded bg-slate-900 hover:bg-slate-800 text-slate-300 border border-slate-700 text-[10px] font-mono transition" data-node-id="${escapeHTML(r.node_id)}">
              Probe
            </button>
          </td>
        </tr>
      `;
    }).join('');

    safeFeatherReplace();
  }

  // 4. Learning Settings
  async function loadLearningSettings() {
    try {
      const res = await fetchHGI('/api/v2/learning/settings');
      if (!res.ok) return;
      const data = await res.json();
      cachedLearningSettings = data.settings || cachedLearningSettings;
      const modeSelect = document.getElementById('hgi-learning-mode-select');
      if (modeSelect) modeSelect.value = cachedLearningSettings.mode || 'observe';
    } catch (e) {
      console.warn('loadLearningSettings error', e);
    }
  }

  // 5. Active Game Detection
  async function loadActiveGame() {
    try {
      const res = await fetchHGI('/api/v2/game-detect');
      if (!res.ok) return;
      const data = await res.json();
      currentActiveGame = data.detected;
    } catch (e) {
      console.warn('loadActiveGame error', e);
    }
  }

  // 6. Timeline
  async function loadTimeline() {
    try {
      const res = await fetchHGI('/api/v2/timeline?limit=15');
      if (!res.ok) return;
      const data = await res.json();
      renderTimeline(data.events || []);
    } catch (e) {
      console.warn('loadTimeline error', e);
    }
  }

  function renderTimeline(events) {
    const listEl = document.getElementById('hgi-timeline-list');
    if (!listEl) return;

    if (!events.length) {
      listEl.innerHTML = '<div class="text-center py-6 text-slate-500 text-xs">No gaming traffic events recorded in current window.</div>';
      return;
    }

    listEl.innerHTML = events.map(ev => {
      const t = ev.timestamp ? new Date(ev.timestamp).toLocaleTimeString() : '—';
      const polBadge = ev.policy === 'PROXY' 
        ? '<span class="text-[9px] px-1.5 py-0.5 rounded bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 font-mono">PROXY</span>'
        : (ev.policy === 'BLOCK' 
            ? '<span class="text-[9px] px-1.5 py-0.5 rounded bg-red-500/20 text-red-300 border border-red-500/30 font-mono">BLOCK</span>'
            : '<span class="text-[9px] px-1.5 py-0.5 rounded bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 font-mono">DIRECT</span>');

      return `
        <div class="flex items-center justify-between p-2.5 rounded-lg bg-slate-950/60 border border-slate-800/80 text-xs font-mono">
          <div class="flex items-center gap-2 min-w-0">
            <span class="text-slate-500 text-[10px]">${t}</span>
            <span class="text-slate-300 font-bold truncate">${escapeHTML(ev.domain)}</span>
            <span class="text-[10px] text-purple-400">(${escapeHTML(ev.game_id || 'Game')})</span>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            ${polBadge}
            <span class="text-[10px] text-slate-400">${escapeHTML(ev.route_node || 'Default')}</span>
          </div>
        </div>
      `;
    }).join('');
  }

  // 7. Audit Logs
  async function loadAuditLogs() {
    try {
      const res = await fetchHGI('/api/v2/audit?limit=15');
      if (!res.ok) return;
      const data = await res.json();
      renderAuditLogs(data.audits || []);
    } catch (e) {
      console.warn('loadAuditLogs error', e);
    }
  }

  function renderAuditLogs(audits) {
    const listEl = document.getElementById('hgi-audit-list');
    if (!listEl) return;

    if (!audits.length) {
      listEl.innerHTML = '<div class="text-center py-6 text-slate-500 text-xs">No audit entries yet. Changes will be securely recorded here.</div>';
      return;
    }

    listEl.innerHTML = audits.map(a => {
      const t = a.timestamp ? new Date(a.timestamp).toLocaleString() : '—';
      return `
        <div class="p-2.5 rounded-lg bg-slate-950/60 border border-slate-800/80 text-xs font-mono space-y-1">
          <div class="flex items-center justify-between text-[10px] text-slate-400">
            <span class="text-cyan-400 font-bold">${escapeHTML(a.action || 'MODIFY')}</span>
            <span>${t}</span>
          </div>
          <div class="text-slate-300 text-[11px]">${escapeHTML(a.details || a.target)}</div>
          <div class="text-[9px] text-slate-500">Actor: ${escapeHTML(a.actor || 'system')}</div>
        </div>
      `;
    }).join('');
  }

  // Init Event Listeners
  let hgiEventsInited = false;
  function initHGIEvents() {
    if (hgiEventsInited) return;
    hgiEventsInited = true;

    // Sub-view buttons (direct and delegated)
    document.querySelectorAll('[data-hgi-view]').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.preventDefault();
        const v = btn.getAttribute('data-hgi-view');
        if (v) switchHGIView(v);
      });
    });
    document.addEventListener('click', (e) => {
      const vBtn = e.target.closest('[data-hgi-view]');
      if (vBtn) {
        e.preventDefault();
        const v = vBtn.getAttribute('data-hgi-view');
        if (v) switchHGIView(v);
      }
    });

    // Test Routes All
    document.getElementById('btn-hgi-test-routes')?.addEventListener('click', async () => {
      const btn = document.getElementById('btn-hgi-test-routes');
      if (btn) btn.disabled = true;
      showToast('Initiating precision route tests across nodes…', 'info');
      try {
        const res = await fetchHGI('/api/v2/routes/measure', { method: 'POST' });
        if (res.ok) {
          showToast('Route measurements updated successfully!', 'success');
          await loadRoutes();
          updateKPIs();
        } else {
          showToast('Failed to measure routes', 'error');
        }
      } catch (e) {
        showToast('Error communicating with server', 'error');
      } finally {
        if (btn) btn.disabled = false;
      }
    });

    // Scan Candidates
    document.getElementById('btn-hgi-scan-discovery')?.addEventListener('click', async () => {
      const btn = document.getElementById('btn-hgi-scan-discovery');
      if (btn) btn.disabled = true;
      showToast('Scanning DNS traffic logs for game candidates…', 'info');
      try {
        const res = await fetchHGI('/api/v2/discovery/scan', { method: 'POST' });
        if (res.ok) {
          const data = await res.json();
          showToast(`Discovery scan complete. Found ${data.found || 0} candidates.`, 'success');
          await loadCandidates();
          updateKPIs();
        } else {
          showToast('Failed to trigger scan', 'error');
        }
      } catch (e) {
        showToast('Error communicating with server', 'error');
      } finally {
        if (btn) btn.disabled = false;
      }
    });

    // Detect Active Game
    document.getElementById('btn-hgi-detect')?.addEventListener('click', async () => {
      showToast('Analyzing active gaming telemetry…', 'info');
      await loadActiveGame();
      updateKPIs();
      if (currentActiveGame && currentActiveGame.game_name) {
        showToast(`Detected Active Game: ${currentActiveGame.game_name}`, 'success');
      } else {
        showToast('No active game patterns matched in rolling window', 'info');
      }
    });

    // Export Intelligence Report
    document.getElementById('btn-hgi-export')?.addEventListener('click', async () => {
      try {
        const res = await fetchHGI('/api/v2/learning/export');
        if (!res.ok) {
          showToast('Failed to export report', 'error');
          return;
        }
        const blob = await res.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `hyperdns-hgi-report-${Date.now()}.json`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(url);
        showToast('HGI Intelligence report downloaded', 'success');
      } catch (e) {
        showToast('Error downloading export', 'error');
      }
    });

    // Save Learning Mode
    document.getElementById('btn-hgi-save-learning')?.addEventListener('click', async () => {
      const modeSelect = document.getElementById('hgi-learning-mode-select');
      const mode = modeSelect ? modeSelect.value : 'observe';
      try {
        const res = await fetchHGI('/api/v2/learning/settings', {
          method: 'POST',
          body: JSON.stringify({ mode })
        });
        if (res.ok) {
          cachedLearningSettings.mode = mode;
          updateKPIs();
          showToast(`Learning Mode updated to ${mode.toUpperCase()}`, 'success');
        } else {
          showToast('Failed to update learning settings', 'error');
        }
      } catch (e) {
        showToast('Error communicating with server', 'error');
      }
    });

    // Clear Learning Data
    document.getElementById('btn-hgi-clear-learning')?.addEventListener('click', async () => {
      if (!confirm('Clear all unapproved candidate data? Active profiles and DNS rules will NOT be modified.')) return;
      try {
        const res = await fetchHGI('/api/v2/learning/clear', { method: 'POST' });
        if (res.ok) {
          showToast('Discovery candidates cleared', 'success');
          await loadCandidates();
          updateKPIs();
        } else {
          showToast('Failed to clear learning data', 'error');
        }
      } catch (e) {
        showToast('Error communicating with server', 'error');
      }
    });

    // Delegate table clicks (Candidates approve/reject, game purge/delete)
    document.addEventListener('click', async (e) => {
      const approveBtn = e.target.closest('.btn-hgi-approve-candidate');
      if (approveBtn) {
        const host = approveBtn.getAttribute('data-host');
        const policy = approveBtn.getAttribute('data-policy') || 'proxy';
        if (!host) return;
        try {
          const res = await fetchHGI('/api/v2/discovery/approve', {
            method: 'POST',
            body: JSON.stringify({ hostname: host, policy: policy })
          });
          if (res.ok) {
            showToast(`Approved "${host}" as ${policy.toUpperCase()}`, 'success');
            await loadCandidates();
            await loadProfiles();
            updateKPIs();
          } else {
            showToast('Failed to approve candidate', 'error');
          }
        } catch (err) {
          showToast('Error approving candidate', 'error');
        }
        return;
      }

      const rejectBtn = e.target.closest('.btn-hgi-reject-candidate');
      if (rejectBtn) {
        const host = rejectBtn.getAttribute('data-host');
        if (!host) return;
        try {
          const res = await fetchHGI('/api/v2/discovery/reject', {
            method: 'POST',
            body: JSON.stringify({ hostname: host })
          });
          if (res.ok) {
            showToast(`Rejected candidate "${host}"`, 'info');
            await loadCandidates();
            updateKPIs();
          } else {
            showToast('Failed to reject candidate', 'error');
          }
        } catch (err) {
          showToast('Error rejecting candidate', 'error');
        }
        return;
      }

      const purgeGameBtn = e.target.closest('.btn-hgi-purge-game');
      if (purgeGameBtn) {
        const gid = purgeGameBtn.getAttribute('data-game-id');
        if (!gid) return;
        try {
          const res = await fetchHGI('/api/v2/cache/purge', {
            method: 'POST',
            body: JSON.stringify({ game_id: gid })
          });
          if (res.ok) {
            showToast(`DNS cache purged for ${gid}!`, 'success');
          } else {
            showToast('Failed to purge cache', 'error');
          }
        } catch (err) {
          showToast('Error purging cache', 'error');
        }
        return;
      }

      const delGameBtn = e.target.closest('.btn-hgi-delete-game');
      if (delGameBtn) {
        const gid = delGameBtn.getAttribute('data-game-id');
        if (!gid) return;
        if (!confirm(`Delete profile for ${gid}? Associated DNS mappings will revert to default.`)) return;
        try {
          const res = await fetchHGI(`/api/v2/games?id=${encodeURIComponent(gid)}`, {
            method: 'DELETE'
          });
          if (res.ok) {
            showToast(`Profile ${gid} deleted`, 'success');
            await loadProfiles();
            updateKPIs();
          } else {
            showToast('Failed to delete game profile', 'error');
          }
        } catch (err) {
          showToast('Error deleting profile', 'error');
        }
        return;
      }

      const exportGameBtn = e.target.closest('.btn-hgi-export-game');
      if (exportGameBtn) {
        const gid = exportGameBtn.getAttribute('data-game-id');
        const prof = cachedProfiles.find(p => p.id === gid);
        if (!prof) return;
        const blob = new Blob([JSON.stringify(prof, null, 2)], { type: 'application/json' });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `game-profile-${gid}.json`;
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(url);
        return;
      }

      const manageDomainsBtn = e.target.closest('.btn-hgi-manage-domains');
      if (manageDomainsBtn) {
        const gid = manageDomainsBtn.getAttribute('data-game-id');
        if (gid) openDomainPolicyEditor(gid);
        return;
      }

      const saveDomainBtn = e.target.closest('.btn-hgi-save-domain-row');
      if (saveDomainBtn) {
        const host = saveDomainBtn.getAttribute('data-host');
        if (!host || !currentEditProfile) return;
        const tr = saveDomainBtn.closest('tr');
        const catSel = tr ? tr.querySelector('.sel-domain-cat') : null;
        const polSel = tr ? tr.querySelector('.sel-domain-pol') : null;
        const cat = catSel ? catSel.value : 'game_services';
        const pol = polSel ? polSel.value : 'proxy';

        try {
          saveDomainBtn.disabled = true;
          const res = await fetchHGI(`/api/v2/games/${encodeURIComponent(currentEditProfile.id)}/domains/${encodeURIComponent(host)}`, {
            method: 'PUT',
            body: JSON.stringify({ policy: pol, category: cat, enabled: true })
          });
          if (res.ok) {
            showToast(`Policy for "${host}" updated to ${pol.toUpperCase()}!`, 'success');
            const rawDomains = currentEditProfile.domains || {};
            let domainList = Array.isArray(rawDomains) ? rawDomains : Object.values(rawDomains);
            const target = domainList.find(d => (d.hostname || '').toLowerCase() === host.toLowerCase());
            if (target) {
              target.category = cat;
              target.policy = pol;
            }
            renderDomainRows();
            await loadProfiles();
            updateKPIs();
          } else {
            showToast('Failed to update domain policy', 'error');
          }
        } catch (err) {
          showToast('Error saving domain policy', 'error');
        } finally {
          saveDomainBtn.disabled = false;
        }
        return;
      }

      const delDomainBtn = e.target.closest('.btn-hgi-delete-domain-row');
      if (delDomainBtn) {
        const host = delDomainBtn.getAttribute('data-host');
        if (!host || !currentEditProfile) return;
        if (!confirm(`Delete domain "${host}" from ${currentEditProfile.name}?`)) return;

        try {
          const res = await fetchHGI(`/api/v2/games/${encodeURIComponent(currentEditProfile.id)}/domains/${encodeURIComponent(host)}`, {
            method: 'DELETE'
          });
          if (res.ok) {
            showToast(`Domain "${host}" removed`, 'success');
            if (Array.isArray(currentEditProfile.domains)) {
              currentEditProfile.domains = currentEditProfile.domains.filter(d => (d.hostname || '').toLowerCase() !== host.toLowerCase());
            } else if (currentEditProfile.domains) {
              delete currentEditProfile.domains[host];
            }
            renderDomainRows();
            await loadProfiles();
            updateKPIs();
          } else {
            showToast('Failed to remove domain', 'error');
          }
        } catch (err) {
          showToast('Error removing domain', 'error');
        }
        return;
      }

      const testSingleNodeBtn = e.target.closest('.btn-hgi-test-single-node');
      if (testSingleNodeBtn) {
        const nid = testSingleNodeBtn.getAttribute('data-node-id');
        if (!nid) return;
        showToast(`Probing node ${nid}…`, 'info');
        try {
          const res = await fetchHGI(`/api/v2/routes/measure?node_id=${encodeURIComponent(nid)}`, { method: 'POST' });
          if (res.ok) {
            showToast(`Probe completed for ${nid}`, 'success');
            await loadRoutes();
            updateKPIs();
          } else {
            showToast('Failed to probe node', 'error');
          }
        } catch (err) {
          showToast('Error probing node', 'error');
        }
        return;
      }
    });

    // Rule Simulator Form
    const simForm = document.getElementById('hgi-simulator-form');
    if (simForm) {
      simForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const hostInput = document.getElementById('hgi-sim-hostname');
        const clientInput = document.getElementById('hgi-sim-client-ip');
        const host = hostInput ? hostInput.value.trim() : '';
        const clientIP = clientInput ? clientInput.value.trim() : '';

        if (!host) {
          showToast('Please enter a hostname to simulate', 'error');
          return;
        }

        const outBox = document.getElementById('hgi-sim-output');
        if (outBox) {
          outBox.innerHTML = '<div class="text-cyan-400 font-mono text-xs text-center py-6">Running rule resolution engine simulation…</div>';
        }

        try {
          const res = await fetchHGI('/api/v2/rules/simulate', {
            method: 'POST',
            body: JSON.stringify({ hostname: host, client_ip: clientIP })
          });

          if (!res.ok) {
            if (outBox) outBox.innerHTML = '<div class="text-red-400 font-mono text-xs text-center py-6">Simulation request was refused</div>';
            return;
          }

          const sim = await res.json();
          renderSimulationResult(sim);
        } catch (err) {
          if (outBox) outBox.innerHTML = '<div class="text-red-400 font-mono text-xs text-center py-6">Could not contact simulator service</div>';
        }
      });
    }

    // AI Gaming Assistant
    const aiForm = document.getElementById('hgi-ai-form');
    if (aiForm) {
      aiForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const input = document.getElementById('hgi-ai-input');
        const prompt = input ? input.value.trim() : '';
        if (!prompt) return;

        appendAIMessage('user', prompt);
        if (input) input.value = '';

        const thinkingId = appendAIThinking();

        try {
          const res = await fetchHGI('/api/v2/ai/assistant', {
            method: 'POST',
            body: JSON.stringify({ prompt: prompt })
          });

          removeAIThinking(thinkingId);

          if (!res.ok) {
            appendAIMessage('assistant', 'Sorry, I encountered an error processing your query. Please try again.');
            return;
          }

          const data = await res.json();
          appendAIMessage('assistant', data.response || 'No analysis available.', data.recommendations);
        } catch (err) {
          removeAIThinking(thinkingId);
          appendAIMessage('assistant', 'Network error connecting to AI Assistant engine.');
        }
      });
    }

    // Quick AI prompt chips
    document.querySelectorAll('.btn-ai-quick-prompt').forEach(chip => {
      chip.addEventListener('click', () => {
        const p = chip.getAttribute('data-prompt');
        const input = document.getElementById('hgi-ai-input');
        if (input && p) {
          input.value = p;
          document.getElementById('hgi-ai-form')?.dispatchEvent(new Event('submit'));
        }
      });
    });

    // Add Custom Game Profile Modal
    document.getElementById('btn-hgi-open-add-game')?.addEventListener('click', () => {
      document.getElementById('hgi-add-game-modal')?.classList.remove('hidden');
    });
    document.getElementById('btn-hgi-close-add-game')?.addEventListener('click', () => {
      document.getElementById('hgi-add-game-modal')?.classList.add('hidden');
    });
    document.getElementById('btn-hgi-cancel-add-game')?.addEventListener('click', () => {
      document.getElementById('hgi-add-game-modal')?.classList.add('hidden');
    });
    document.getElementById('btn-hgi-refresh-timeline')?.addEventListener('click', () => {
      loadTimeline();
    });
    document.getElementById('btn-hgi-refresh-audit')?.addEventListener('click', () => {
      loadAuditLogs();
    });
    document.getElementById('btn-hgi-reprobe-routes')?.addEventListener('click', () => {
      document.getElementById('btn-hgi-test-routes')?.click();
    });

    document.getElementById('hgi-add-game-form')?.addEventListener('submit', async (e) => {
      e.preventDefault();
      const id = document.getElementById('hgi-add-game-id')?.value.trim();
      const name = document.getElementById('hgi-add-game-name')?.value.trim();
      const pub = document.getElementById('hgi-add-game-publisher')?.value.trim();
      const pol = document.getElementById('hgi-add-game-policy')?.value || 'proxy';
      const domainsRaw = document.getElementById('hgi-add-game-domains')?.value || '';

      if (!id || !name) {
        showToast('Game ID and Name are required', 'error');
        return;
      }

      const domainList = domainsRaw
        .split(/[\n,]+/)
        .map(d => d.trim().toLowerCase())
        .filter(d => d.length > 0);

      const profilePayload = {
        id: id,
        name: name,
        publisher: pub,
        enabled: true,
        default_policy: pol,
        domains: {
          matchmaking: domainList,
          game_services: [],
          authentication: [],
          cdn_downloads: []
        }
      };

      try {
        const res = await fetchHGI('/api/v2/games', {
          method: 'POST',
          body: JSON.stringify(profilePayload)
        });

        if (res.ok) {
          showToast(`Game Profile "${name}" created successfully!`, 'success');
          document.getElementById('hgi-add-game-modal')?.classList.add('hidden');
          document.getElementById('hgi-add-game-form')?.reset();
          await loadProfiles();
          updateKPIs();
        } else {
          showToast('Failed to create game profile', 'error');
        }
      } catch (err) {
        showToast('Error communicating with server', 'error');
      }
    });

    // Domain Policies Modal Handlers
    const closeDomainModal = () => {
      document.getElementById('hgi-domain-policy-modal')?.classList.add('hidden');
      currentEditProfile = null;
    };
    document.getElementById('btn-hgi-close-domain-policy')?.addEventListener('click', closeDomainModal);
    document.getElementById('btn-hgi-done-domain-policy')?.addEventListener('click', closeDomainModal);

    document.getElementById('hgi-domain-search')?.addEventListener('input', () => {
      renderDomainRows();
    });
    document.getElementById('hgi-filter-category')?.addEventListener('change', () => {
      renderDomainRows();
    });
    document.getElementById('hgi-filter-policy')?.addEventListener('change', () => {
      renderDomainRows();
    });

    const catSelectIds = [
      { id: 'hgi-cat-policy-auth', cat: 'auth' },
      { id: 'hgi-cat-policy-matchmaking', cat: 'matchmaking' },
      { id: 'hgi-cat-policy-game_services', cat: 'game_services' },
      { id: 'hgi-cat-policy-cdn', cat: 'cdn' },
      { id: 'hgi-cat-policy-telemetry', cat: 'telemetry' }
    ];
    catSelectIds.forEach(item => {
      const catEl = document.getElementById(item.id);
      if (catEl) {
        catEl.addEventListener('change', async () => {
          if (!currentEditProfile) return;
          const newPolicy = catEl.value;
          try {
            const res = await fetchHGI(`/api/v2/games/${encodeURIComponent(currentEditProfile.id)}/categories/${encodeURIComponent(item.cat)}?propagate=true`, {
              method: 'PUT',
              body: JSON.stringify({ policy: newPolicy })
            });
            if (res.ok) {
              showToast(`Updated category "${item.cat}" policy to ${newPolicy.toUpperCase()}`, 'success');
              const rawDomains = currentEditProfile.domains || {};
              let domainList = Array.isArray(rawDomains) ? rawDomains : Object.values(rawDomains);
              domainList.forEach(d => {
                if (d.category === item.cat) {
                  d.policy = newPolicy;
                }
              });
              if (!currentEditProfile.dns_policies) currentEditProfile.dns_policies = {};
              currentEditProfile.dns_policies[item.cat] = newPolicy;
              renderDomainRows();
              await loadProfiles();
              updateKPIs();
            } else {
              showToast('Failed to update category policy', 'error');
            }
          } catch (err) {
            showToast('Error updating category policy', 'error');
          }
        });
      }
    });

    const addDomainForm = document.getElementById('hgi-add-domain-inline-form');
    if (addDomainForm) {
      addDomainForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        if (!currentEditProfile) return;
        const hostEl = document.getElementById('hgi-new-domain-host');
        const catEl = document.getElementById('hgi-new-domain-category');
        const polEl = document.getElementById('hgi-new-domain-policy');
        const submitBtn = document.getElementById('btn-hgi-add-domain-submit');

        const host = hostEl ? hostEl.value.trim().toLowerCase() : '';
        const cat = catEl ? catEl.value : 'matchmaking';
        const pol = polEl ? polEl.value : 'proxy';

        if (!host) {
          showToast('Please enter a hostname', 'error');
          return;
        }

        try {
          if (submitBtn) submitBtn.disabled = true;
          const res = await fetchHGI(`/api/v2/games/${encodeURIComponent(currentEditProfile.id)}/domains/${encodeURIComponent(host)}`, {
            method: 'PUT',
            body: JSON.stringify({ policy: pol, category: cat, enabled: true })
          });
          if (res.ok) {
            showToast(`Added "${host}" to ${currentEditProfile.name}`, 'success');
            if (hostEl) hostEl.value = '';
            const profileRes = await fetchHGI(`/api/v2/games/${encodeURIComponent(currentEditProfile.id)}`);
            if (profileRes.ok) {
              currentEditProfile = await profileRes.json();
            }
            renderDomainRows();
            await loadProfiles();
            updateKPIs();
          } else {
            showToast('Failed to add domain', 'error');
          }
        } catch (err) {
          showToast('Error adding domain', 'error');
        } finally {
          if (submitBtn) submitBtn.disabled = false;
        }
      });
    }
  }

  function renderSimulationResult(sim) {
    const outBox = document.getElementById('hgi-sim-output');
    if (!outBox) return;

    const matchedPol = sim.matched_policy || 'DIRECT';
    const polClass = matchedPol === 'PROXY' 
      ? 'text-cyan-400 bg-cyan-500/10 border-cyan-500/30'
      : (matchedPol === 'BLOCK' ? 'text-red-400 bg-red-500/10 border-red-500/30' : 'text-emerald-400 bg-emerald-500/10 border-emerald-500/30');

    outBox.innerHTML = `
      <div class="space-y-4 font-mono text-xs animate-fade-in">
        <div class="p-3.5 rounded-xl bg-slate-950/80 border border-slate-800 flex items-center justify-between">
          <div>
            <div class="text-[10px] text-slate-500 uppercase">Policy Verdict</div>
            <div class="text-base font-bold font-heading ${polClass.split(' ')[0]} mt-0.5">${escapeHTML(matchedPol)}</div>
          </div>
          <span class="px-2.5 py-1 rounded text-[11px] font-bold border ${polClass}">
            Priority: ${sim.priority || 100}
          </span>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div class="p-3 rounded-xl bg-slate-950/60 border border-slate-800">
            <span class="text-slate-500 text-[10px] block">ASSOCIATED GAME</span>
            <span class="text-white font-bold mt-1 block">${escapeHTML(sim.game_name || 'Generic / Non-Game')}</span>
            <span class="text-[10px] text-cyan-400">${escapeHTML(sim.publisher || 'Direct Resolution')}</span>
          </div>
          <div class="p-3 rounded-xl bg-slate-950/60 border border-slate-800">
            <span class="text-slate-500 text-[10px] block">ASSIGNED ROUTE NODE</span>
            <span class="text-white font-bold mt-1 block">${escapeHTML(sim.route_node || 'Default Upstream')}</span>
            <span class="text-[10px] text-emerald-400">Target IP: ${escapeHTML(sim.target_ip || 'Resolved Dynamic')}</span>
          </div>
        </div>

        <!-- Precedence Trace -->
        <div class="p-3 rounded-xl bg-slate-950/60 border border-slate-800 space-y-1.5">
          <span class="text-slate-500 text-[10px] block">EVALUATION TRACE</span>
          <div class="space-y-1 text-[11px]">
            ${(sim.trace || []).map(t => `<div class="text-slate-300 flex items-center gap-1.5"><i data-feather="check" class="w-3 h-3 text-cyan-400"></i><span>${escapeHTML(t)}</span></div>`).join('')}
          </div>
        </div>

        <!-- Blast Radius & Conflicts -->
        <div class="p-3 rounded-xl ${sim.has_conflicts ? 'bg-amber-950/30 border-amber-500/40 text-amber-200' : 'bg-emerald-950/20 border-emerald-500/30 text-emerald-200'} border">
          <div class="flex items-center gap-1.5 font-bold mb-1">
            <i data-feather="${sim.has_conflicts ? 'alert-triangle' : 'shield'}" class="w-3.5 h-3.5"></i>
            <span>${sim.has_conflicts ? 'Conflict Warning' : 'Safe Execution Assessment'}</span>
          </div>
          <p class="text-[11px] leading-relaxed opacity-90">${escapeHTML(sim.blast_radius || 'Zero blast radius: Resolution will only impact the tested domain without shadowing critical system records.')}</p>
        </div>
      </div>
    `;

    safeFeatherReplace();
  }

  // AI Chat Helpers
  let messageCounter = 0;
  function appendAIMessage(role, text, recommendations = []) {
    const box = document.getElementById('hgi-ai-chat-box');
    if (!box) return;

    messageCounter++;
    const isUser = role === 'user';
    const msgDiv = document.createElement('div');
    msgDiv.className = `flex gap-2.5 ${isUser ? 'justify-end' : ''} text-xs leading-relaxed`;

    const recsHtml = recommendations && recommendations.length > 0 ? `
      <div class="mt-2.5 pt-2 border-t border-cyan-500/20 space-y-1.5">
        <div class="text-[10px] font-mono text-cyan-300 font-bold uppercase">Recommendations:</div>
        ${recommendations.map(r => `
          <div class="p-2 rounded bg-slate-900/80 border border-slate-700/80 flex items-center justify-between gap-2">
            <span class="font-mono text-slate-200 text-[11px]">${escapeHTML(r.text || r)}</span>
            ${r.action ? `<button class="px-2 py-0.5 rounded bg-cyan-500/20 text-cyan-300 border border-cyan-500/30 text-[10px] font-mono hover:bg-cyan-500/30 transition">Apply</button>` : ''}
          </div>
        `).join('')}
      </div>
    ` : '';

    msgDiv.innerHTML = `
      <div class="max-w-md p-3.5 rounded-2xl ${isUser ? 'bg-cyan-500/20 border border-cyan-500/40 text-cyan-100' : 'bg-slate-950/80 border border-slate-800 text-slate-200'} shadow-lg space-y-1">
        <div class="flex items-center gap-1.5 text-[10px] font-mono ${isUser ? 'text-cyan-400' : 'text-purple-400'}">
          <i data-feather="${isUser ? 'user' : 'cpu'}" class="w-3 h-3"></i>
          <span>${isUser ? 'Operator' : 'HGI Intelligence Core'}</span>
        </div>
        <div class="text-xs break-words whitespace-pre-wrap">${escapeHTML(text)}</div>
        ${recsHtml}
      </div>
    `;

    box.appendChild(msgDiv);
    box.scrollTop = box.scrollHeight;
    safeFeatherReplace();
  }

  function appendAIThinking() {
    const box = document.getElementById('hgi-ai-chat-box');
    if (!box) return null;
    const id = `thinking-${Date.now()}`;
    const tDiv = document.createElement('div');
    tDiv.id = id;
    tDiv.className = 'flex gap-2.5 text-xs';
    tDiv.innerHTML = `
      <div class="p-3 rounded-xl bg-slate-950/60 border border-slate-800 text-cyan-400 flex items-center gap-2 font-mono text-xs">
        <span class="w-2 h-2 rounded-full bg-cyan-400 animate-ping"></span>
        <span>Analyzing gaming intelligence patterns...</span>
      </div>
    `;
    box.appendChild(tDiv);
    box.scrollTop = box.scrollHeight;
    return id;
  }

  function removeAIThinking(id) {
    if (!id) return;
    const el = document.getElementById(id);
    if (el) el.remove();
  }

  // Public Export
  window.renderGamingTab = () => {
    initHGIEvents();
    loadGamingData();
  };
  window.initHGIEvents = initHGIEvents;

  // Bootstrap when DOM ready (or immediately if already parsed)
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', () => {
      initHGIEvents();
      const gamingTab = document.getElementById('tab-gaming');
      if (gamingTab && !gamingTab.classList.contains('hidden')) {
        loadGamingData();
      }
    });
  } else {
    initHGIEvents();
    const gamingTab = document.getElementById('tab-gaming');
    if (gamingTab && !gamingTab.classList.contains('hidden')) {
      loadGamingData();
    }
  }

})();
