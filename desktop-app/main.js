const { app, BrowserWindow, Tray, Menu, ipcMain, nativeImage, shell } = require('electron');
const path = require('path');
const { exec } = require('child_process');
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
async function resolveDNSHost(host) {
  if (!host) return '';
  if (net.isIP(host)) return host;
  return new Promise((resolve) => {
    dns.lookup(host, { family: 4 }, (err, address) => {
      if (err || !address) resolve(host);
      else resolve(address);
    });
  });
}

// ─── DNS Management (requires admin/elevated) ─────────────────────────
async function setDNS(primaryDNS, secondaryDNS = '1.1.1.1') {
  // Resolve domain name to IPv4 if needed
  const resolvedPrimary = await resolveDNSHost(primaryDNS);
  const resolvedSecondary = await resolveDNSHost(secondaryDNS);

  const cmd = `
    $ErrorActionPreference = 'Stop'
    try {
      Get-NetAdapter | Where-Object {$_.Status -eq 'Up'} | ForEach-Object {
        Set-DnsClientServerAddress -InterfaceIndex $_.ifIndex -ServerAddresses @('${resolvedPrimary}','${resolvedSecondary}')
      }
      Clear-DnsClientCache
      Write-Output "SUCCESS"
    } catch {
      Write-Error $_.Exception.Message
    }
  `;

  exec(`powershell -NoProfile -Command "${cmd.replace(/\r?\n/g, ' ')}"`, { shell: 'powershell.exe' }, (error, stdout, stderr) => {
    if (mainWindow) {
      let isSuccess = !error && stdout.includes('SUCCESS');
      let msg = '';
      if (isSuccess) {
        msg = `DNS ضدتحریم (${resolvedPrimary}) فعال شد ✓`;
      } else {
        const errText = (stderr || error?.message || '').toLowerCase();
        if (errText.includes('permission') || errText.includes('access is denied') || errText.includes('administrator')) {
          msg = 'نیاز به دسترسی Admin: لطفاً برنامه را با Run as administrator باز کنید.';
        } else {
          msg = `خطا در تنظیم DNS: ${stderr || error?.message || 'ناشناخته'}`;
        }
      }
      mainWindow.webContents.send('dns-result', {
        success: isSuccess,
        message: msg,
        primary: resolvedPrimary,
        secondary: resolvedSecondary
      });
    }
  });
}

function resetDNS() {
  const cmd = `
    $ErrorActionPreference = 'Stop'
    try {
      Get-NetAdapter | Where-Object {$_.Status -eq 'Up'} | ForEach-Object {
        Set-DnsClientServerAddress -InterfaceIndex $_.ifIndex -ResetServerAddresses
      }
      Clear-DnsClientCache
      Write-Output "SUCCESS"
    } catch {
      Write-Error $_.Exception.Message
    }
  `;

  exec(`powershell -NoProfile -Command "${cmd.replace(/\r?\n/g, ' ')}"`, { shell: 'powershell.exe' }, (error, stdout, stderr) => {
    if (mainWindow) {
      let isSuccess = !error && stdout.includes('SUCCESS');
      let msg = '';
      if (isSuccess) {
        msg = 'DNS به حالت پیش‌فرض ویندوز (خودکار) برگشت ✓';
      } else {
        const errText = (stderr || error?.message || '').toLowerCase();
        if (errText.includes('permission') || errText.includes('access is denied') || errText.includes('administrator')) {
          msg = 'نیاز به دسترسی Admin: لطفاً برنامه را با Run as administrator باز کنید.';
        } else {
          msg = `خطا: ${stderr || error?.message || 'ناشناخته'}`;
        }
      }
      mainWindow.webContents.send('dns-result', {
        success: isSuccess,
        message: msg,
        primary: 'Auto',
        secondary: 'Auto'
      });
    }
  });
}

function getCurrentDNS() {
  const cmd = `Get-DnsClientServerAddress -AddressFamily IPv4 | Where-Object {$_.ServerAddresses.Count -gt 0} | Select-Object -First 1 -ExpandProperty ServerAddresses`;
  exec(`powershell -NoProfile -Command "${cmd}"`, { shell: 'powershell.exe' }, (error, stdout) => {
    if (mainWindow) {
      const addresses = stdout ? stdout.trim().split(/\r?\n/).map(s => s.trim()).filter(Boolean) : [];
      mainWindow.webContents.send('current-dns', { addresses });
    }
  });
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
