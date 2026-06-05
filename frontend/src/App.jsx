import { useState, useEffect, useRef, useCallback } from 'react';
import QRCode from 'qrcode';
import PrinterList from './components/PrinterList';
import LogConsole from './components/LogConsole';
import JobQueue from './components/JobQueue';
import DeviceList from './components/DeviceList';

export default function App() {
  const [status, setStatus] = useState({
    mdnsActive: false,
    httpActive: false,
    httpAddress: ':8080',
    printerCount: 0,
  });
  const [showConsole, setShowConsole] = useState(false);
  const [showQRModal, setShowQRModal] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [theme, setTheme] = useState(() => {
    const saved = localStorage.getItem('theme');
    return saved === 'light' ? 'light' : 'black';
  });
  const [serverName, setServerName] = useState('TakePrint');
  const [isEditingName, setIsEditingName] = useState(false);
  const [tempName, setTempName] = useState('');
  const [autoLaunch, setAutoLaunch] = useState(false);
  const [remotePrinters, setRemotePrinters] = useState([]);

  const handleRemotePrintersUpdate = useCallback((printers) => {
    setRemotePrinters(printers || []);
  }, []);

  useEffect(() => {
    if (theme === 'light') {
      document.documentElement.classList.remove('dark');
      document.documentElement.classList.add('light');
    } else {
      document.documentElement.classList.remove('light');
      document.documentElement.classList.add('dark');
    }
    localStorage.setItem('theme', theme);
  }, [theme]);

  useEffect(() => {
    const fetchStatus = async () => {
      if (window.go?.main?.App?.GetServerStatus) {
        try {
          const s = await window.go.main.App.GetServerStatus();
          setStatus(s);
        } catch {
          // Wails not ready.
        }
      } else {
        // Mock for dev.
        setStatus({ mdnsActive: true, httpActive: true, httpAddress: ':8080', printerCount: 3 });
      }
    };

    const fetchServerName = async () => {
      if (window.go?.main?.App?.GetServerName) {
        try {
          const name = await window.go.main.App.GetServerName();
          setServerName(name);
        } catch {
          // Wails not ready.
        }
      }
    };

    const fetchAutoLaunch = async () => {
      if (window.go?.main?.App?.IsAutoLaunchEnabled) {
        try {
          const enabled = await window.go.main.App.IsAutoLaunchEnabled();
          setAutoLaunch(enabled);
        } catch {
          // Wails not ready.
        }
      }
    };

    fetchStatus();
    fetchServerName();
    fetchAutoLaunch();
    const interval = setInterval(fetchStatus, 10000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.key === 'Escape') {
        setShowSettings(false);
        setShowQRModal(false);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  const handleSaveName = async () => {
    if (!tempName.trim()) return;
    if (window.go?.main?.App?.UpdateServerName) {
      try {
        await window.go.main.App.UpdateServerName(tempName.trim());
        setServerName(tempName.trim());
        setIsEditingName(false);
      } catch (err) {
        alert("Failed to update name: " + err);
      }
    } else {
      setServerName(tempName.trim());
      setIsEditingName(false);
    }
  };

  const handleToggleAutoLaunch = async () => {
    const nextVal = !autoLaunch;
    if (window.go?.main?.App?.SetAutoLaunch) {
      try {
        await window.go.main.App.SetAutoLaunch(nextVal);
        setAutoLaunch(nextVal);
      } catch (err) {
        alert("Failed to toggle auto-launch: " + err);
      }
    } else {
      setAutoLaunch(nextVal);
    }
  };

  return (
    <div className="w-full h-full flex flex-col bg-surface-950">
      {/* ===== HEADER ===== */}
      <header className="flex items-center justify-between px-6 py-4 border-b border-surface-700/50">
        {/* Left: Title */}
        <div className="flex items-center gap-3">
          <div>
            <h1 className="text-base font-bold text-slate-200 tracking-tight flex items-center gap-1.5">
              TakePrint
            </h1>
            {isEditingName ? (
              <div className="flex items-center gap-1 mt-0.5">
                <input
                  type="text"
                  value={tempName}
                  onChange={(e) => setTempName(e.target.value)}
                  className="bg-surface-800 border border-surface-600 rounded px-1.5 py-0.5 text-[11px] text-slate-200 focus:outline-none focus:border-accent-500 w-32"
                  autoFocus
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') handleSaveName();
                    if (e.key === 'Escape') {
                      e.stopPropagation();
                      setIsEditingName(false);
                    }
                  }}
                />
                <button
                  onClick={handleSaveName}
                  className="p-0.5 rounded hover:bg-surface-700 text-success-400 cursor-pointer"
                >
                  <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                  </svg>
                </button>
                <button
                  onClick={() => setIsEditingName(false)}
                  className="p-0.5 rounded hover:bg-surface-700 text-error-400 cursor-pointer"
                >
                  <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
            ) : (
              <p className="text-[11px] text-slate-500 -mt-0.5 flex items-center gap-1 group">
                <span>Computer: <strong className="text-slate-300 font-semibold">{serverName}</strong></span>
                <button
                  onClick={() => {
                    setTempName(serverName);
                    setIsEditingName(true);
                  }}
                  className="opacity-0 group-hover:opacity-100 transition-opacity p-0.5 hover:bg-surface-700/50 rounded text-accent-400 cursor-pointer"
                  title="Rename Computer Name"
                >
                  <svg className="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                  </svg>
                </button>
              </p>
            )}
          </div>
        </div>

        {/* Right: Status indicators */}
        <div className="flex items-center gap-4">
          {/* Settings Button */}
          <button
            onClick={() => setShowSettings(!showSettings)}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-xl border transition-all duration-200 cursor-pointer ${
              showSettings
                ? 'bg-accent-500/10 border-accent-500/30 text-accent-400'
                : 'bg-surface-800/80 border-surface-700/50 text-slate-400 hover:text-slate-200'
            }`}
            title="Open Server Settings"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
              <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
            <span className="text-[11px] font-medium">Settings</span>
          </button>

          {/* QR Code Connect Button */}
          <button
            onClick={() => setShowQRModal(true)}
            className="flex items-center gap-2 px-3 py-1.5 rounded-xl border border-surface-700/50 bg-surface-800/80 text-slate-400 hover:text-slate-200 transition-all duration-200 cursor-pointer"
            title="Show QR Code for mobile connection"
          >
            <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <rect x="3" y="3" width="7" height="7" />
              <rect x="14" y="3" width="7" height="7" />
              <rect x="14" y="14" width="7" height="7" />
              <rect x="3" y="14" width="7" height="7" />
            </svg>
            <span className="text-[11px] font-medium">QR Connect</span>
          </button>

          {/* Console Toggle Button */}
          <button
            onClick={() => setShowConsole(!showConsole)}
            className={`flex items-center gap-2 px-3 py-1.5 rounded-xl border transition-all duration-200 cursor-pointer ${
              showConsole
                ? 'bg-accent-500/10 border-accent-500/30 text-accent-400'
                : 'bg-surface-800/80 border-surface-700/50 text-slate-400 hover:text-slate-200'
            }`}
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
            </svg>
            <span className="text-[11px] font-medium">Console</span>
          </button>

          {/* Server Status */}
          <div id="server-status" className="flex items-center gap-2 px-3 py-1.5 rounded-xl bg-surface-800/80 border border-surface-700/50">
            <div className={`w-2 h-2 rounded-full ${status.mdnsActive && status.httpActive ? 'bg-success-400 animate-pulse' : 'bg-error-400'}`} />
            <span className="text-[11px] font-medium text-slate-300">Status</span>
            <span className={`text-[10px] font-semibold px-1.5 py-0.5 rounded ${
              status.mdnsActive && status.httpActive
                ? 'text-success-400 bg-success-400/10'
                : 'text-error-400 bg-error-400/10'
            }`}>
              {status.mdnsActive && status.httpActive ? 'Online' : 'Offline'}
            </span>
          </div>
        </div>
      </header>

      {/* ===== MAIN CONTENT ===== */}
      <main className="flex-1 flex min-h-0 p-5 gap-5">
        {showSettings ? (
          <section className="flex-1 glass-card p-6 flex flex-col animate-fade-in">
            <SettingsView
              serverName={serverName}
              onClose={() => setShowSettings(false)}
              autoLaunch={autoLaunch}
              onToggleAutoLaunch={handleToggleAutoLaunch}
              onSaveServerName={handleSaveName}
              tempName={tempName}
              setTempName={setTempName}
              isEditingName={isEditingName}
              setIsEditingName={setIsEditingName}
              theme={theme}
              setTheme={setTheme}
            />
          </section>
        ) : (
          <>
            {/* Left Column: Printers + Devices */}
            <section className="w-[340px] shrink-0 flex flex-col gap-5 animate-fade-in">
              <div className="glass-card p-5 flex flex-col flex-1 min-h-0">
                <PrinterList remotePrinters={remotePrinters} />
              </div>
              <div className="glass-card p-5 flex flex-col h-[260px] shrink-0">
                <DeviceList onRemotePrintersUpdate={handleRemotePrintersUpdate} />
              </div>
            </section>

            {/* Middle Column: Print Queue */}
            <section className="flex-1 glass-card p-5 flex flex-col animate-fade-in" style={{ animationDelay: '60ms' }}>
              <JobQueue />
            </section>
          </>
        )}

        {/* Right Column: Console */}
        {showConsole && (
          <section className="w-[340px] shrink-0 glass-card p-5 flex flex-col animate-fade-in" style={{ animationDelay: '120ms' }}>
            <LogConsole />
          </section>
        )}
      </main>


      {/* ===== FOOTER ===== */}
      <footer className="flex items-center justify-between px-6 py-2.5 border-t border-surface-700/30 text-[10px] text-slate-600">
        <span>TakePrint v1.0.0</span>
        <span>
          {status.printerCount} printer{status.printerCount !== 1 ? 's' : ''} available
        </span>
      </footer>

      {showQRModal && (
        <QRModal
          onClose={() => setShowQRModal(false)}
          serverName={serverName}
        />
      )}
    </div>
  );
}

function QRModal({ onClose, serverName }) {
  const [ips, setIps] = useState([]);
  const canvasRef = useRef(null);

  useEffect(() => {
    const fetchIPs = async () => {
      if (window.go?.main?.App?.GetLocalIPs) {
        try {
          const list = await window.go.main.App.GetLocalIPs();
          setIps(list);
        } catch (err) {
          console.error("Failed to get local IPs", err);
        }
      } else {
        setIps(["192.168.1.100"]); // Fallback for dev mode
      }
    };
    fetchIPs();
  }, []);

  useEffect(() => {
    if (ips.length > 0 && canvasRef.current) {
      const qrData = JSON.stringify({
        name: serverName,
        ips: ips,
        port: 8080
      });
      QRCode.toCanvas(canvasRef.current, qrData, {
        width: 180,
        margin: 2,
        color: {
          dark: '#0057ff', // TakePrint Blue
          light: '#ffffff'
        }
      }, (error) => {
        if (error) console.error(error);
      });
    }
  }, [ips, serverName]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm animate-fade-in">
      <div className="bg-surface-900 border border-surface-700/50 rounded-2xl p-6 w-[320px] shadow-2xl flex flex-col items-center">
        <div className="w-full flex justify-between items-center mb-4">
          <h3 className="text-sm font-bold text-slate-200">Connect Mobile App</h3>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200 cursor-pointer">
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        <div className="bg-white p-3 rounded-2xl border border-surface-700/50 flex items-center justify-center">
          <canvas ref={canvasRef} />
        </div>
        <p className="text-[11px] text-slate-400 text-center mt-4 leading-relaxed">
          Scan this QR code from the TakePrint Mobile app to connect instantly.
        </p>
        <div className="w-full mt-4 border-t border-surface-700/30 pt-3 text-[10px] text-slate-500 flex flex-col gap-1">
          <div><span className="font-semibold text-slate-400">Server Name:</span> {serverName}</div>
          <div><span className="font-semibold text-slate-400">IP Addresses:</span> {ips.join(', ')}</div>
          <div><span className="font-semibold text-slate-400">Port:</span> 8080</div>
        </div>
      </div>
    </div>
  );
}

function SettingsView({
  serverName,
  onClose,
  autoLaunch,
  onToggleAutoLaunch,
  onSaveServerName,
  tempName,
  setTempName,
  isEditingName,
  setIsEditingName,
  theme,
  setTheme
}) {
  const [savePath, setSavePath] = useState('');
  const [updating, setUpdating] = useState(false);

  useEffect(() => {
    const fetchSavePath = async () => {
      if (window.go?.main?.App?.GetVirtualPrinterDir) {
        try {
          const path = await window.go.main.App.GetVirtualPrinterDir();
          setSavePath(path);
        } catch (err) {
          console.error("Failed to load save path", err);
        }
      } else {
        setSavePath("C:\\Users\\kiran\\Downloads"); // Mock
      }
    };
    fetchSavePath();
  }, []);

  const handleSelectDir = async () => {
    if (window.go?.main?.App?.SelectVirtualPrinterDir) {
      try {
        setUpdating(true);
        const selected = await window.go.main.App.SelectVirtualPrinterDir();
        setSavePath(selected);
      } catch (err) {
        alert("Failed to pick folder: " + err);
      } finally {
        setUpdating(false);
      }
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between pb-4 border-b border-surface-700/50 mb-6">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-lg bg-accent-500/15 flex items-center justify-center">
            <svg className="w-4 h-4 text-accent-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
              <circle cx="12" cy="12" r="3" />
              <path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 008 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 11-2.83-2.83l.06-.06a1.65 1.65 0 00.33-1.82 1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 8a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06a1.65 1.65 0 001.82.33H9a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 112.83 2.83l-.06.06a1.65 1.65 0 00-.33 1.82V9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z" />
            </svg>
          </div>
          <h2 className="text-sm font-semibold text-slate-200 tracking-wide uppercase">
            Settings
          </h2>
        </div>
        <button
          onClick={onClose}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-xl border border-surface-700/50 bg-surface-800/80 text-slate-400 hover:text-slate-200 transition-all duration-200 cursor-pointer"
        >
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M10 19l-7-7m0 0l7-7m-7 7h18" />
          </svg>
          <span className="text-[11px] font-medium">Back</span>
        </button>
      </div>

      {/* Settings Options List */}
      <div className="flex-1 overflow-y-auto space-y-6 max-w-2xl">
        {/* Device Name */}
        <div className="flex flex-col gap-2">
          <label className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Device / Server Name</label>
          <p className="text-[11px] text-slate-500 mb-1">How this printer server appears to other devices on the local Wi-Fi network.</p>
          {isEditingName ? (
            <div className="flex items-center gap-2 max-w-sm">
              <input
                type="text"
                value={tempName}
                onChange={(e) => setTempName(e.target.value)}
                className="flex-1 bg-surface-800 border border-surface-600 rounded-xl px-3 py-2 text-xs text-slate-200 focus:outline-none focus:border-accent-500"
                autoFocus
                onKeyDown={(e) => {
                  if (e.key === 'Enter') onSaveServerName();
                  if (e.key === 'Escape') {
                    e.stopPropagation();
                    setIsEditingName(false);
                  }
                }}
              />
              <button
                onClick={onSaveServerName}
                className="px-3 py-2 rounded-xl bg-success-500/10 border border-success-500/20 text-success-400 hover:bg-success-500/20 text-xs font-medium cursor-pointer"
              >
                Save
              </button>
              <button
                onClick={() => setIsEditingName(false)}
                className="px-3 py-2 rounded-xl bg-surface-800 border border-surface-700/50 text-slate-400 hover:text-slate-200 text-xs font-medium cursor-pointer"
              >
                Cancel
              </button>
            </div>
          ) : (
            <div className="flex items-center gap-3">
              <span className="text-sm text-slate-200 font-semibold">{serverName}</span>
              <button
                onClick={() => {
                  setTempName(serverName);
                  setIsEditingName(true);
                }}
                className="px-2.5 py-1.5 rounded-xl border border-surface-700/50 bg-surface-800/80 text-slate-400 hover:text-slate-200 transition-all duration-200 cursor-pointer text-[10px] font-medium"
              >
                Rename
              </button>
            </div>
          )}
        </div>

        {/* Theme Settings (Appearance) */}
        <div className="flex flex-col gap-2 pt-4 border-t border-surface-700/30">
          <label className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Appearance</label>
          <p className="text-[11px] text-slate-500 mb-2">Choose your preferred visual theme for TakePrint.</p>
          <div className="flex items-center gap-3 max-w-sm">
            <button
              onClick={() => setTheme('light')}
              className={`flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl border transition-all duration-200 cursor-pointer ${
                theme === 'light'
                  ? 'bg-accent-500/10 border-accent-500/30 text-accent-400 font-semibold'
                  : 'bg-surface-800/80 border-surface-700/50 text-slate-400 hover:text-slate-200'
              }`}
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364-6.364l-.707.707M6.343 17.657l-.707.707m0-12.728l.707.707m12.728 12.728l.707-.707M12 8a4 4 0 100 8 4 4 0 000-8z" />
              </svg>
              <span className="text-xs">Light Mode</span>
            </button>
            <button
              onClick={() => setTheme('black')}
              className={`flex-1 flex items-center justify-center gap-2 px-4 py-3 rounded-xl border transition-all duration-200 cursor-pointer ${
                theme === 'black'
                  ? 'bg-accent-500/10 border-accent-500/30 text-accent-400 font-semibold'
                  : 'bg-surface-800/80 border-surface-700/50 text-slate-400 hover:text-slate-200'
              }`}
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
              </svg>
              <span className="text-xs">AMOLED Black</span>
            </button>
          </div>
        </div>

        {/* Save Location for Virtual Printers */}
        <div className="flex flex-col gap-2 pt-4 border-t border-surface-700/30">
          <label className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Virtual Printer Save Location</label>
          <p className="text-[11px] text-slate-500 mb-1">Virtual printers (like PDF and OneNote) will save print jobs directly to this directory instead of prompting dialogs.</p>
          <div className="flex items-center gap-3 max-w-xl">
            <div className="flex-1 bg-surface-800/60 border border-surface-700/50 rounded-xl px-3.5 py-2 text-xs text-slate-300 font-mono truncate">
              {savePath || 'Loading...'}
            </div>
            <button
              onClick={handleSelectDir}
              disabled={updating}
              className="shrink-0 px-3 py-2 rounded-xl bg-accent-500/10 border border-accent-500/20 text-accent-400 hover:bg-accent-500/20 disabled:opacity-50 text-xs font-medium cursor-pointer transition-all duration-200"
            >
              Change Folder
            </button>
          </div>
        </div>

        {/* Launch on Boot */}
        <div className="flex flex-col gap-2 pt-4 border-t border-surface-700/30">
          <label className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Startup Configuration</label>
          <p className="text-[11px] text-slate-500 mb-3">Enable this option to start TakePrint silently in the background when your computer boots up.</p>
          <button
            onClick={onToggleAutoLaunch}
            className={`flex items-center justify-between max-w-sm px-4 py-3 rounded-xl border transition-all duration-200 cursor-pointer ${
              autoLaunch
                ? 'bg-accent-500/10 border-accent-500/30 text-accent-400'
                : 'bg-surface-800/80 border-surface-700/50 text-slate-400 hover:text-slate-200'
            }`}
          >
            <div className="flex items-center gap-3">
              <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
              </svg>
              <div className="text-left">
                <div className="text-xs font-semibold text-slate-200">Launch on Boot</div>
                <div className="text-[10px] text-slate-500 mt-0.5">Auto-start on Windows boot</div>
              </div>
            </div>
            <div className={`w-8 h-4 rounded-full p-0.5 transition-colors duration-200 ${autoLaunch ? 'bg-accent-500' : 'bg-surface-700'}`}>
              <div className={`w-3 h-3 rounded-full bg-white transition-transform duration-200 transform ${autoLaunch ? 'translate-x-4' : 'translate-x-0'}`} />
            </div>
          </button>
        </div>
      </div>
    </div>
  );
}

