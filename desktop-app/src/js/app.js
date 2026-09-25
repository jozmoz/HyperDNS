const { ipcRenderer } = require('electron');

// ─── State ────────────────────────────────────────────────────────────
let state = {
  serverUrl: '',
  token: '',
  secret: '',
  clientData: null,
  serverDNS: '',
  detectedIP: '',
  currentDNSList: [],
  refreshInterval: null,
  dnsCheckInterval: null
};

// ─── Init ─────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', async () => {
  try {
    const creds = await ipcRenderer.invoke('load-credentials');
    if (creds && creds.serverUrl && creds.token && creds.secret) {
      document.getElementById('input-server').value = creds.serverUrl;
      document.getElementById('input-token').value = creds.token;
      document.getElementById('input-secret').value = creds.secret;
      // Auto-login with saved credentials
      await doLogin(true);
    }
  } catch (e) {
    console.log('No saved credentials');
  }

  // Listen for DNS results from main process
  ipcRenderer.on('dns-result', (event, result) => {
    showToast(result.message, result.success ? 'success' : 'error');
    setTimeout(() => ipcRenderer.invoke('get-current-dns'), 1000);
  });

  // Listen for current DNS state
  ipcRenderer.on('current-dns', (event, result) => {
    state.currentDNSList = result.addresses || [];
    updateDNSStatusUI();
  });
});

// ─── Window Controls ──────────────────────────────────────────────────
function minimizeWindow() { ipcRenderer.invoke('minimize-window'); }
function closeWindow() { ipcRenderer.invoke('close-window'); }
function openExternal(url) { ipcRenderer.invoke('open-external', url); }

// ─── Toast ────────────────────────────────────────────────────────────
function showToast(message, type = 'info') {
  const toast = document.getElementById('toast');
  if (!toast) return;
  toast.textContent = message;
  toast.className = `toast ${type} show`;
  setTimeout(() => toast.classList.remove('show'), 3500);
}

// ─── View Switching ───────────────────────────────────────────────────
function showView(viewId) {
  document.querySelectorAll('.view').forEach(v => v.classList.remove('active'));
  const target = document.getElementById(viewId);
  if (target) target.classList.add('active');
}

// ─── Password Visibility Toggle ───────────────────────────────────────
function togglePasswordVisibility() {
  const input = document.getElementById('input-secret');
  const icon = document.getElementById('eye-icon');
  if (input.type === 'password') {
    input.type = 'text';
    icon.innerHTML = `
      <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"></path>
      <line x1="1" y1="1" x2="23" y2="23"></line>
    `;
  } else {
    input.type = 'password';
    icon.innerHTML = `
      <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"></path>
      <circle cx="12" cy="12" r="3"></circle>
    `;
  }
}

// ─── Smart Link Input Detection ───────────────────────────────────────
function handleServerInput(val) {
  if (!val) return;
  val = val.trim();
  // Check if user pasted a complete subscription or IP link
  const match = val.match(/^(https?:\/\/[^\/]+)\/(?:sub|ip|api\/sub)\/([a-zA-Z0-9_\-]+)/i);
  if (match) {
    document.getElementById('input-server').value = match[1];
    document.getElementById('input-token').value = match[2];
    document.getElementById('input-secret').focus();
    showToast('لینک اشتراک شناسایی شد؛ لطفاً رمز عبور را وارد کنید ✓', 'info');
  }
}

// ─── Login ────────────────────────────────────────────────────────────
async function doLogin(silent = false) {
  const serverInput = document.getElementById('input-server');
  const tokenInput = document.getElementById('input-token');
  const secretInput = document.getElementById('input-secret');
  const errorEl = document.getElementById('login-error');
  const btn = document.getElementById('btn-login');

  const serverUrl = serverInput.value.trim();
  const token = tokenInput.value.trim();
  const secret = secretInput.value.trim();

  if (!serverUrl || !token || !secret) {
    if (!silent) {
      errorEl.textContent = 'لطفاً آدرس سرور، توکن و رمز عبور اشتراک را وارد کنید.';
      errorEl.classList.add('show');
    }
    return;
  }

  // Clean server URL
  let cleanUrl = serverUrl.replace(/\/+$/, '');
  if (!cleanUrl.startsWith('http://') && !cleanUrl.startsWith('https://')) {
    cleanUrl = 'https://' + cleanUrl;
  }

  // Visual loading state
  btn.disabled = true;
  btn.innerHTML = '<div class="spinner"></div> در حال تایید رمز و اتصال...';
  errorEl.classList.remove('show');

  try {
    // 1. Try dedicated auth endpoint (validates secret + auto-registers IP)
    const authUrl = `${cleanUrl}/api/sub/${token}/auth`;
    let authResult = await ipcRenderer.invoke('api-request', {
      url: authUrl,
      method: 'POST',
      body: { secret, register_ip: true }
    });

    let clientData = null;

    if (authResult.status === 200 && authResult.data && authResult.data.success) {
      clientData = authResult.data;
    } else if (authResult.status === 401) {
      // Secret is WRONG! Strictly reject login
      throw new Error('رمز عبور اشتراک اشتباه است. لطفاً رمز صحیح را بررسی کنید.');
    } else if (authResult.status === 404) {
      // Fallback for older HyperDNS server releases: verify via POST /ip/<token>
      const ipUrl = `${cleanUrl}/ip/${token}`;
      const ipResult = await ipcRenderer.invoke('api-request', {
        url: ipUrl,
        method: 'POST',
        body: { secret }
      });

      if (ipResult.status === 401) {
        throw new Error('رمز عبور اشتراک اشتباه است.');
      } else if (ipResult.status === 404) {
        throw new Error('توکن اشتراک یا آدرس سرور یافت نشد.');
      }

      // If secret matched, load subscriber data
      const subUrl = `${cleanUrl}/api/sub/${token}`;
      const subResult = await ipcRenderer.invoke('api-request', { url: subUrl, method: 'GET' });
      if (subResult.status === 200 && subResult.data && subResult.data.success) {
        clientData = subResult.data;
      } else {
        throw new Error('امکان دریافت اطلاعات اشتراک وجود ندارد.');
      }
    } else if (authResult.status === 0) {
      throw new Error(`اتصال به سرور برقرار نشد: ${authResult.data?.error || 'بررسی اتصال اینترنت یا فایروال'}`);
    } else {
      throw new Error(authResult.data?.error || 'خطا در احراز هویت');
    }

    // Login Succeeded!
    state.serverUrl = cleanUrl;
    state.token = token;
    state.secret = secret;
    state.clientData = clientData;
    state.serverDNS = clientData.server_dns || '';
    state.detectedIP = clientData.detected_ip || '';

    // Save credentials if remember is checked
    const remember = document.getElementById('input-remember').checked;
    if (remember) {
      await ipcRenderer.invoke('save-credentials', { serverUrl: cleanUrl, token, secret });
    }

    // Switch to dashboard view
    renderDashboard(clientData);
    showView('view-dashboard');
    showToast('ورود با موفقیت انجام شد ✓', 'success');

    // Auto-refresh interval (every 30 seconds)
    if (state.refreshInterval) clearInterval(state.refreshInterval);
    state.refreshInterval = setInterval(refreshData, 30000);

    // DNS check interval (every 10 seconds)
    if (state.dnsCheckInterval) clearInterval(state.dnsCheckInterval);
    state.dnsCheckInterval = setInterval(() => ipcRenderer.invoke('get-current-dns'), 10000);

    // Initial check of current DNS
    ipcRenderer.invoke('get-current-dns');

  } catch (e) {
    if (!silent) {
      errorEl.textContent = e.message || 'خطا در ورود به حساب';
      errorEl.classList.add('show');
    }
  } finally {
    btn.disabled = false;
    btn.innerHTML = `
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M15 3h4a2 2 0 012 2v14a2 2 0 01-2 2h-4M10 17l5-5-5-5M15 12H3"/></svg>
      ورود به حساب و فعال‌سازی
    `;
  }
}

// ─── Logout ───────────────────────────────────────────────────────────
async function doLogout() {
  if (state.refreshInterval) clearInterval(state.refreshInterval);
  if (state.dnsCheckInterval) clearInterval(state.dnsCheckInterval);
  state.clientData = null;
  state.token = '';
  state.secret = '';
  await ipcRenderer.invoke('clear-credentials');
  document.getElementById('input-secret').value = '';
  showView('view-login');
  showToast('از حساب خارج شدید', 'info');
}

// ─── Refresh Data ─────────────────────────────────────────────────────
async function refreshData() {
  if (!state.token || !state.serverUrl) return;
  try {
    const apiUrl = `${state.serverUrl}/api/sub/${state.token}`;
    const result = await ipcRenderer.invoke('api-request', { url: apiUrl, method: 'GET' });
    if (result.status === 200 && result.data && result.data.success) {
      state.clientData = result.data;
      state.serverDNS = result.data.server_dns || state.serverDNS;
      state.detectedIP = result.data.detected_ip || state.detectedIP;
      renderDashboard(result.data);
    }
  } catch (e) {
    console.error('Refresh error:', e);
  }
}

async function manualRefresh() {
  const btn = document.getElementById('btn-refresh');
  btn.classList.add('loading');
  await refreshData();
  await ipcRenderer.invoke('get-current-dns');
  setTimeout(() => btn.classList.remove('loading'), 600);
  showToast('داده‌ها بروزرسانی شدند ✓', 'info');
}

// ─── Render Dashboard ─────────────────────────────────────────────────
function renderDashboard(data) {
  const client = data.client || {};
  const name = client.display_name || client.name || 'مشترک ویژه';

  // User Header
  document.getElementById('dash-avatar').textContent = name.charAt(0).toUpperCase();
  document.getElementById('dash-name').textContent = name;

  // Status
  const dot = document.getElementById('dash-dot');
  const statusText = document.getElementById('dash-status-text');
  const now = new Date();
  const expiresAt = client.expires_at ? new Date(client.expires_at) : null;
  const isExpired = expiresAt && expiresAt < now;
  const isEnabled = client.enabled !== false;

  if (!isEnabled) {
    dot.className = 'dot offline';
    statusText.textContent = 'غیرفعال / مسدود';
    statusText.style.color = 'var(--red)';
  } else if (isExpired) {
    dot.className = 'dot expired';
    statusText.textContent = 'منقضی شده';
    statusText.style.color = 'var(--yellow)';
  } else {
    dot.className = 'dot online';
    statusText.textContent = 'اشتراک فعال';
    statusText.style.color = 'var(--green)';
  }

  // Traffic
  const usedBytes = data.traffic_used_bytes || client.traffic_used_bytes || 0;
  const limitGB = data.traffic_limit_gb || client.traffic_limit_gb || 0;
  const usedMB = usedBytes / (1024 * 1024);
  const usedGB = usedMB / 1024;

  let usedLabel = usedGB >= 1 ? `${usedGB.toFixed(2)} GB` : `${usedMB.toFixed(1)} MB`;
  let limitLabel = limitGB > 0 ? `${limitGB.toFixed(1)} GB` : 'نامحدود';
  let percent = limitGB > 0 ? Math.min(100, (usedGB / limitGB) * 100) : 0;

  document.getElementById('traffic-used-label').textContent = usedLabel;
  document.getElementById('traffic-limit-label').textContent = limitLabel;
  document.getElementById('traffic-percent').textContent = percent.toFixed(0);

  const bar = document.getElementById('traffic-bar');
  bar.style.width = `${percent}%`;
  bar.className = 'progress-fill';
  if (percent > 90) bar.classList.add('danger');
  else if (percent > 70) bar.classList.add('warning');

  // Traffic Reset Hint
  const resetHint = document.getElementById('traffic-reset-hint');
  if (data.next_traffic_reset) {
    const resetDate = new Date(data.next_traffic_reset);
    resetHint.textContent = `تمدید ترافیک: ${resetDate.toLocaleDateString('fa-IR')}`;
  } else {
    resetHint.textContent = 'حجم کل اشتراک';
  }

  // Info Grid: Expiration
  const expiresEl = document.getElementById('info-expires');
  if (expiresAt && !isNaN(expiresAt.getTime())) {
    const diff = expiresAt - now;
    const days = Math.ceil(diff / (1000 * 60 * 60 * 24));
    if (days > 0) {
      expiresEl.textContent = `${days} روز باقی‌مانده`;
      expiresEl.className = 'info-value green';
    } else {
      expiresEl.textContent = 'منقضی شده';
      expiresEl.className = 'info-value red';
    }
  } else {
    expiresEl.textContent = 'مادام‌العمر (نامحدود)';
    expiresEl.className = 'info-value green';
  }

  // Devices
  const allowedIPs = data.allowed_ips || client.allowed_ips || [];
  const maxDevices = data.max_devices || client.max_devices || 1;
  document.getElementById('info-devices').textContent = `${allowedIPs.length} از ${maxDevices} مجاز`;

  // IPs
  document.getElementById('info-my-ip').textContent = state.detectedIP || '—';
  const regIPEl = document.getElementById('info-registered-ip');
  if (allowedIPs.length > 0) {
    regIPEl.textContent = allowedIPs.join(', ');
    if (allowedIPs.includes(state.detectedIP)) {
      regIPEl.className = 'info-value green';
    } else {
      regIPEl.className = 'info-value yellow';
    }
  } else {
    regIPEl.textContent = 'ثبت نشده';
    regIPEl.className = 'info-value red';
  }

  // DNS Hero Badge
  state.serverDNS = data.server_dns || '';
  document.getElementById('dns-server-badge').textContent = state.serverDNS || '—';

  updateDNSStatusUI();
}

// ─── Update DNS Status UI ─────────────────────────────────────────────
function updateDNSStatusUI() {
  const dot = document.getElementById('dns-state-dot');
  const label = document.getElementById('dns-state-label');
  const activateBtn = document.getElementById('btn-activate-dns');

  if (!state.serverDNS) {
    dot.className = 'dns-state-dot';
    label.textContent = 'آدرس DNS سرور دریافت نشده';
    return;
  }

  const isDnsActive = state.currentDNSList.some(ip => ip.trim() === state.serverDNS.trim());
  if (isDnsActive) {
    dot.className = 'dns-state-dot active';
    label.textContent = 'DNS ضدتحریم ویندوز: فعال و متصل ✓';
    label.style.color = 'var(--green)';
    activateBtn.textContent = '✅ DNS فعال است (اعمال مجدد)';
  } else {
    dot.className = 'dns-state-dot inactive';
    label.textContent = 'DNS ویندوز: روی حالت عادی / غیرفعال';
    label.style.color = 'var(--yellow)';
    activateBtn.textContent = '🚀 فعال‌سازی DNS ضدتحریم';
  }
}

// ─── DNS Control ──────────────────────────────────────────────────────
function activateDNS() {
  if (!state.serverDNS) {
    showToast('آدرس DNS سرور در دسترس نیست', 'error');
    return;
  }
  showToast('در حال تنظیم DNS سیستم... (نیازمند Admin)', 'info');
  ipcRenderer.invoke('set-dns', { primary: state.serverDNS, secondary: '1.1.1.1' });
}

function deactivateDNS() {
  showToast('در حال بازگردانی DNS به حالت خودکار...', 'info');
  ipcRenderer.invoke('reset-dns');
}

// ─── DNS Latency & Quality Test ───────────────────────────────────────
async function testDNSConnection() {
  const btn = document.getElementById('btn-test-dns');
  const resultEl = document.getElementById('dns-test-result');

  if (!state.serverDNS) {
    showToast('آدرس سرور DNS موجود نیست', 'error');
    return;
  }

  btn.disabled = true;
  resultEl.textContent = 'در حال تست پینگ...';
  resultEl.className = 'dns-test-result';

  try {
    const res = await ipcRenderer.invoke('test-dns', state.serverDNS);
    if (res.success) {
      resultEl.textContent = `⚡ تاخیر: ${res.latency}ms (بدون تحریم ✓)`;
      resultEl.className = 'dns-test-result success';
      showToast(`تست موفق: تاخیر ${res.latency} میلی‌ثانیه`, 'success');
    } else {
      resultEl.textContent = `خطا: ${res.error || 'عدم پاسخگویی'}`;
      resultEl.className = 'dns-test-result error';
      showToast('خطا در تست DNS سرور', 'error');
    }
  } catch (e) {
    resultEl.textContent = `خطا: ${e.message}`;
    resultEl.className = 'dns-test-result error';
  } finally {
    btn.disabled = false;
  }
}

// ─── Register IP ──────────────────────────────────────────────────────
async function registerIP() {
  const btn = document.getElementById('btn-register-ip');
  btn.disabled = true;
  btn.innerHTML = '<div class="spinner"></div> در حال ثبت آی‌پی...';

  try {
    const ipUrl = `${state.serverUrl}/ip/${state.token}`;
    const result = await ipcRenderer.invoke('api-request', {
      url: ipUrl,
      method: 'POST',
      body: { secret: state.secret }
    });

    if (result.status === 200 && result.data && result.data.success) {
      showToast('آی‌پی سیستم شما با موفقیت در سرور ثبت شد ✓', 'success');
      await refreshData();
    } else {
      const reason = result.data?.reason || '';
      let msg = result.data?.error || 'خطا در ثبت آی‌پی';
      if (reason === 'secret') msg = 'رمز ثبت اشتراک اشتباه است';
      else if (reason === 'suspended') msg = 'این اشتراک مسدود شده است';
      else if (reason === 'expired') msg = 'اشتراک شما منقضی شده است';
      else if (reason === 'quota') msg = 'سقف ترافیک اشتراک تمام شده است';
      else if (reason === 'conflict') msg = 'این آی‌پی قبلاً در اشتراک دیگری فعال شده است';
      showToast(msg, 'error');
    }
  } catch (e) {
    showToast(`خطا: ${e.message}`, 'error');
  } finally {
    btn.disabled = false;
    btn.innerHTML = `
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
      ثبت / بروزرسانی آی‌پی فعلی در سرور
    `;
  }
}

// ─── Open Web Portal in Browser ───────────────────────────────────────
function openPortalInBrowser() {
  if (state.serverUrl && state.token) {
    openExternal(`${state.serverUrl}/sub/${state.token}`);
  }
}

// ─── Keyboard Shortcuts ───────────────────────────────────────────────
document.addEventListener('keydown', (e) => {
  if (e.key === 'Enter') {
    const loginView = document.getElementById('view-login');
    if (loginView && loginView.classList.contains('active')) {
      doLogin();
    }
  }
  if (e.key === 'F5') {
    manualRefresh();
  }
});
