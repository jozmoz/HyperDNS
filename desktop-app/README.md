# HyperDNS Desktop Client 🖥️

A premium Windows desktop application for HyperDNS subscribers to manage their SmartDNS subscription.

## Features

- ⚡ **One-Click Login** — Authenticate with your subscription Token + Registration Secret
- 📊 **Live Dashboard** — Real-time traffic usage, subscription status, and device info
- 🌐 **IP Registration** — Register your current IP address with a single click
- 🔧 **Auto DNS Setup** — Automatically configure Windows DNS settings (no manual config needed)
- 🔔 **System Tray** — Runs in the background with quick access from the taskbar
- 🔒 **Secure** — Credentials saved locally with encryption, API communication via HTTPS
- 🔄 **Auto-Refresh** — Subscription data refreshes every 30 seconds

## Requirements

- Windows 10/11
- Node.js 18+ (for development only)

## Development

```bash
# Install dependencies
npm install

# Run in development mode
npm start
```

## Build

```bash
# Build Windows installer (.exe) + portable
npm run build
```

Output will be in the `dist/` folder:
- `HyperDNS Setup x.x.x.exe` — NSIS installer
- `HyperDNS x.x.x.exe` — Portable executable

## Usage

1. Open the app
2. Enter your **Server URL** (e.g., `https://dns.example.com` or `http://YOUR_SERVER_IP:8080`)
3. Enter your **Subscription Token** (from your provider)
4. Enter your **Registration Secret** (from your provider)
5. Click **"ورود به حساب"** to login

### DNS Auto-Configuration

The app can automatically set your Windows DNS to point to the HyperDNS server. Click **"🚀 فعال‌سازی DNS"** to enable or **"🔄 بازگردانی پیش‌فرض"** to restore default DNS settings.

> ⚠️ DNS configuration requires running the app as Administrator.

## License

MIT — Part of the [HyperDNS](https://github.com/jozmoz/HyperDNS) project.
