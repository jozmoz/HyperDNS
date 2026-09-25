const { app, BrowserWindow, Tray, Menu, ipcMain, nativeImage, shell } = require('electron');
const path = require('path');
const { exec, execFile } = require('child_process');
const https = require('https');
const http = require('http');
const dns = require('dns');
const net = require('net');
const Store = require('electron-store');

const store = new Store({
  defaults: {
    serverUrl: '',
    token: '',
    secret: '',
    autoStart: false,
    minimizeToTray: true
  }
});

let mainWindow = null;
let tray = null;
let isQuitting = false;

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 480,
    height: 740,
    minWidth: 420,
    minHeight: 640,
    resizable: true,
    frame: false,
    transparent: false,
    backgroundColor: '#0a0e1a',
    icon: path.join(__dirname, 'assets', 'icon.png'),
    webPreferences: {
      nodeIntegration: true,
      contextIsolation: false
    }
  });

  mainWindow.loadFile('src/index.html');

  mainWindow.on('close', (event) => {
    if (!isQuitting && store.get('minimizeToTray')) {
      event.preventDefault();
      mainWindow.hide();
    }
  });

  mainWindow.on('closed', () => {
    mainWindow = null;
  });
}

function createTray() {
  const iconPath = path.join(__dirname, 'assets', 'tray-icon.png');
  let trayIcon;
  try {
    trayIcon = nativeImage.createFromPath(iconPath);
    if (trayIcon.isEmpty()) {
      trayIcon = nativeImage.createEmpty();
    }
  } catch {
    trayIcon = nativeImage.createEmpty();
  }

  tray = new Tray(trayIcon);
  tray.setToolTip('HyperDNS Client');

  const contextMenu = Menu.buildFromTemplate([
    {
      label: '🏠 باز کردن برنامه',
      click: () => {
        if (mainWindow) {
          mainWindow.show();
          mainWindow.focus();
        }
      }
    },
    { type: 'separator' },
    {
      label: '⚡ فعال‌سازی DNS',
      click: () => {
        const serverUrl = store.get('serverUrl');
        if (serverUrl) {
          try {
            const url = new URL(serverUrl);
            setDNS(url.hostname);
          } catch {}
        }
      }
    },
    {
      label: '🔄 بازگردانی DNS خودکار',
      click: () => resetDNS()
    },
    { type: 'separator' },
    {
      label: '❌ خروج کامل',
      click: () => {
        isQuitting = true;
        app.quit();
      }
    }
  ]);

  tray.setContextMenu(contextMenu);

  tray.on('double-click', () => {
    if (mainWindow) {
      mainWindow.show();
      mainWindow.focus();
    }
  });
}

// ─── DNS Host Resolution ──────────────────────────────────────────────
function cleanHost(input) {
  if (!input) return '';
  let str = input.trim();
  // Strip protocol (https://, http://)
  str = str.replace(/^[a-zA-Z]+:\/\//, '');
  // Strip URL path (/sub/..., etc.)
  str = str.split('/')[0];
  // Strip port (:56104, :8080, etc.)
  str = str.split(':')[0];
  return str.trim();
}

async function resolveDNSHost(raw) {
  const host = cleanHost(raw);
  if (!host) return '';
  if (net.isIP(host)) return host;
  return new Promise((resolve) => {
    dns.lookup(host, { family: 4 }, (err, address) => {
      if (err || !address) resolve(host);
      else resolve(address);
    });
  });
}

function formatPowerShellError(raw) {
  if (!raw) return '';
  if (raw.includes('#< CLIXML')) {
    const matches = raw.match(/<S S="Error">([\s\S]*?)<\/S>/g);
    if (matches) {
      return matches
        .map(m => m.replace(/<\/?S[^>]*>/g, '').replace(/_x000D__x000A_/g, '\n').trim())
        .filter(Boolean)
        .join(' ');
    }
  }
  return raw;
}

function runPowerShell(script) {
  return new Promise((resolve, reject) => {
    const buffer = Buffer.from(script, 'utf16le');
    const encoded = buffer.toString('base64');
    execFile(
      'powershell.exe',
      ['-NoProfile', '-NonInteractive', '-ExecutionPolicy', 'Bypass', '-EncodedCommand', encoded],
      (error, stdout, stderr) => {
        if (error) {
          const errText = formatPowerShellError(stderr) || stdout || error.message;
          reject(new Error(errText));
        } else {
          resolve(stdout.trim());
        }
      }
    );
  });
}

// ─── DNS Management (requires admin/elevated) ─────────────────────────
async function setDNS(primaryDNS, secondaryDNS = '1.1.1.1') {
  try {
    const isAdmin = await isRunningAsAdmin();
    if (!isAdmin) {
      if (mainWindow) {
        mainWindow.webContents.send('dns-result', {
          success: false,
          message: 'نیاز به دسترسی Administrator: لطفاً دکمه «اجرا با دسترسی ادمین» بالای صفحه را کلیک کنید.',
          primary: '',
          secondary: ''
        });
      }
      return;
    }

    const resolvedPrimary = await resolveDNSHost(primaryDNS);
    const resolvedSecondary = await resolveDNSHost(secondaryDNS);

    if (!net.isIPv4(resolvedPrimary)) {
      throw new Error(`آدرس سرور (${primaryDNS}) به یک آی‌پی معتبر تبدیل نشد.`);
    }

    const script = `
      $ErrorActionPreference = 'Stop'
      $adapters = Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceDescription -notlike '*Tunnel*' -and $_.InterfaceDescription -notlike '*Loopback*' }
      if (-not $adapters) {
        $adapters = Get-NetAdapter | Where-Object { $_.Status -eq 'Up' }
      }
      foreach ($ad in $adapters) {
        Set-DnsClientServerAddress -InterfaceIndex $ad.ifIndex -ServerAddresses @('${resolvedPrimary}', '${resolvedSecondary}')
      }
      Clear-DnsClientCache
      Write-Output "SUCCESS"
    `;

    await runPowerShell(script);

    if (mainWindow) {
      mainWindow.webContents.send('dns-result', {
        success: true,
        message: `DNS ضدتحریم (${resolvedPrimary}) با موفقیت فعال شد ✓`,
        primary: resolvedPrimary,
        secondary: resolvedSecondary
      });
    }
  } catch (err) {
    let msg = err.message || 'خطای ناشناخته';
    const lower = msg.toLowerCase();
    if (
      lower.includes('permission') ||
      lower.includes('access is denied') ||
      lower.includes('administrator') ||
      lower.includes('cim resource') ||
      lower.includes('denied')
    ) {
      msg = 'نیاز به دسترسی Administrator: لطفاً با کلیک روی دکمه بالای صفحه، برنامه را با دسترسی ادمین باز کنید.';
    }
    if (mainWindow) {
      mainWindow.webContents.send('dns-result', {
        success: false,
        message: `خطا در تنظیم DNS: ${msg}`,
        primary: '',
        secondary: ''
      });
    }
  }
}

async function resetDNS() {
  try {
    const isAdmin = await isRunningAsAdmin();
    if (!isAdmin) {
      if (mainWindow) {
        mainWindow.webContents.send('dns-result', {
          success: false,
          message: 'نیاز به دسترسی Administrator: لطفاً دکمه «اجرا با دسترسی ادمین» بالای صفحه را کلیک کنید.',
          primary: 'Auto',
          secondary: 'Auto'
        });
      }
      return;
    }

    const script = `
      $ErrorActionPreference = 'Stop'
      $adapters = Get-NetAdapter | Where-Object { $_.Status -eq 'Up' }
      foreach ($ad in $adapters) {
        Set-DnsClientServerAddress -InterfaceIndex $ad.ifIndex -ResetServerAddresses
      }
      Clear-DnsClientCache
      Write-Output "SUCCESS"
    `;

    await runPowerShell(script);

    if (mainWindow) {
      mainWindow.webContents.send('dns-result', {
        success: true,
        message: 'DNS به حالت پیش‌فرض ویندوز (خودکار) بازگردانده شد ✓',
        primary: 'Auto',
        secondary: 'Auto'
      });
    }
  } catch (err) {
    let msg = err.message || 'خطای ناشناخته';
    const lower = msg.toLowerCase();
    if (
      lower.includes('permission') ||
      lower.includes('access is denied') ||
      lower.includes('administrator') ||
      lower.includes('cim resource') ||
      lower.includes('denied')
    ) {
      msg = 'نیاز به دسترسی Administrator: لطفاً با کلیک روی دکمه بالای صفحه، برنامه را با دسترسی ادمین باز کنید.';
    }
    if (mainWindow) {
      mainWindow.webContents.send('dns-result', {
        success: false,
        message: `خطا در بازگردانی DNS: ${msg}`,
        primary: 'Auto',
        secondary: 'Auto'
      });
    }
  }
}

async function getCurrentDNS() {
  try {
    const script = `
      $active = Get-NetAdapter | Where-Object { $_.Status -eq 'Up' -and $_.InterfaceDescription -notlike '*Tunnel*' }
      if (-not $active) { $active = Get-NetAdapter | Where-Object { $_.Status -eq 'Up' } }
      $ips = @()
      foreach ($ad in $active) {
        $dns = (Get-DnsClientServerAddress -InterfaceIndex $ad.ifIndex -AddressFamily IPv4).ServerAddresses
        if ($dns) { $ips += $dns }
      }
      $ips | Select-Object -Unique
    `;
    const out = await runPowerShell(script);
    const addresses = out ? out.split(/\r?\n/).map(s => s.trim()).filter(Boolean) : [];
    if (mainWindow) {
      mainWindow.webContents.send('current-dns', { addresses });
    }
  } catch (e) {
    console.error('getCurrentDNS error:', e);
  }
}

// ─── API Proxy (bypass CORS) ──────────────────────────────────────────
function apiRequest(url, method, body) {
  return new Promise((resolve, reject) => {
    const parsed = new URL(url);
    const isHttps = parsed.protocol === 'https:';
    const lib = isHttps ? https : http;

    const options = {
      hostname: parsed.hostname,
      port: parsed.port || (isHttps ? 443 : 80),
      path: parsed.pathname + parsed.search,
      method: method || 'GET',
      headers: { 'Content-Type': 'application/json' },
      rejectUnauthorized: false,
      timeout: 10000
    };

    const req = lib.request(options, (res) => {
      let data = '';
      res.on('data', (chunk) => data += chunk);
      res.on('end', () => {
        try {
          resolve({ status: res.statusCode, data: JSON.parse(data) });
        } catch {
          resolve({ status: res.statusCode, data: { raw: data } });
        }
      });
    });
    req.on('error', (e) => reject(e));
    req.on('timeout', () => { req.destroy(); reject(new Error('Connection timed out')); });
    if (body) req.write(JSON.stringify(body));
    req.end();
  });
}

// ─── IPC Handlers ─────────────────────────────────────────────────────
ipcMain.handle('api-request', async (event, { url, method, body }) => {
  try {
    return await apiRequest(url, method, body);
  } catch (err) {
    return { status: 0, data: { error: err.message } };
  }
});

ipcMain.handle('set-dns', async (event, { primary, secondary }) => {
  setDNS(primary, secondary || '1.1.1.1');
});

ipcMain.handle('reset-dns', async () => {
  resetDNS();
});

ipcMain.handle('get-current-dns', async () => {
  getCurrentDNS();
});

ipcMain.handle('save-credentials', async (event, { serverUrl, token, secret }) => {
  store.set('serverUrl', serverUrl);
  store.set('token', token);
  store.set('secret', secret);
});

ipcMain.handle('load-credentials', async () => {
  return {
    serverUrl: store.get('serverUrl'),
    token: store.get('token'),
    secret: store.get('secret')
  };
});

ipcMain.handle('clear-credentials', async () => {
  store.delete('serverUrl');
  store.delete('token');
  store.delete('secret');
});

ipcMain.handle('minimize-window', () => mainWindow?.minimize());
ipcMain.handle('close-window', () => mainWindow?.close());
ipcMain.handle('open-external', (event, url) => shell.openExternal(url));

// ─── Admin Check & Elevation ──────────────────────────────────────────
async function isRunningAsAdmin() {
  try {
    const script = `
      ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
    `;
    const res = await runPowerShell(script);
    return res.trim().toLowerCase() === 'true';
  } catch (e) {
    return false;
  }
}

ipcMain.handle('check-admin', async () => {
  return await isRunningAsAdmin();
});

ipcMain.handle('relaunch-as-admin', async () => {
  const exePath = process.execPath;
  let script = '';
  if (exePath.toLowerCase().endsWith('electron.exe')) {
    const appDir = path.resolve(__dirname);
    script = `Start-Process -FilePath "${exePath}" -ArgumentList '"${appDir}"' -Verb RunAs`;
  } else {
    script = `Start-Process -FilePath "${exePath}" -Verb RunAs`;
  }
  try {
    await runPowerShell(script);
    isQuitting = true;
    setTimeout(() => {
      app.quit();
    }, 600);
    return { success: true };
  } catch (err) {
    console.error('Failed to elevate:', err);
    return { success: false, error: err.message };
  }
});

ipcMain.handle('test-dns', async (event, dnsServer) => {
  const start = Date.now();
  try {
    const targetServer = await resolveDNSHost(dnsServer || '1.1.1.1');
    return await new Promise((resolve) => {
      const resolver = new dns.Resolver();
      resolver.setServers([targetServer]);
      const timer = setTimeout(() => {
        resolve({ success: false, error: 'مهلت تست به پایان رسید (Timeout)', latency: 5000 });
      }, 5000);

      resolver.resolve4('epicgames.com', (err, addresses) => {
        clearTimeout(timer);
        const latency = Date.now() - start;
        if (err) {
          resolve({ success: false, error: err.message, latency });
        } else {
          resolve({ success: true, addresses, latency });
        }
      });
    });
  } catch (err) {
    return { success: false, error: err.message, latency: Date.now() - start };
  }
});

// ─── App Lifecycle ────────────────────────────────────────────────────
const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
  app.quit();
} else {
  app.on('second-instance', () => {
    if (mainWindow) {
      if (mainWindow.isMinimized()) mainWindow.restore();
      mainWindow.show();
      mainWindow.focus();
    }
  });

  app.on('ready', () => {
    createWindow();
    createTray();
  });

  app.on('before-quit', () => {
    isQuitting = true;
  });

  app.on('window-all-closed', () => {
    if (process.platform !== 'darwin') {
      // Don't quit — tray keeps running
    }
  });

  app.on('activate', () => {
    if (mainWindow === null) createWindow();
  });
}
