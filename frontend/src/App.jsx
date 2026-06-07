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
  const [showQRModal, setShowQRModal] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [theme, setTheme] = useState(() => {
    const saved = localStorage.getItem('theme');
    return saved === 'light' ? 'light' : 'black';
  });
  const [serverName, setServerName] = useState('TakePrint');
  const [appVersion, setAppVersion] = useState('1.0.2');
  const [isEditingName, setIsEditingName] = useState(false);
  const [tempName, setTempName] = useState('');
  const [autoLaunch, setAutoLaunch] = useState(false);
  const [remotePrinters, setRemotePrinters] = useState([]);
  const [printersCollapsed, setPrintersCollapsed] = useState(false);
  const [devicesCollapsed, setDevicesCollapsed] = useState(true);
  const [ips, setIps] = useState([]);
  const [showStatusPopover, setShowStatusPopover] = useState(false);
  const [activeJobProgress, setActiveJobProgress] = useState(null);

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

    const fetchIPs = async () => {
      if (window.go?.main?.App?.GetLocalIPs) {
        try {
          const list = await window.go.main.App.GetLocalIPs();
          setIps(list || []);
        } catch {
          // Wails not ready.
        }
      } else {
        setIps(["192.168.1.100"]); // Fallback
      }
    };

    fetchStatus();
    fetchServerName();
    fetchAutoLaunch();
    fetchIPs();

    const fetchVersion = async () => {
      if (window.go?.main?.App?.GetVersion) {
        try {
          const v = await window.go.main.App.GetVersion();
          setAppVersion(v);
        } catch {
          // Wails not ready.
        }
      }
    };
    fetchVersion();

    const interval = setInterval(fetchStatus, 10000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    const handleKeyDown = (e) => {
      if (e.key === 'Escape') {
        setShowSettings(false);
        setShowQRModal(false);
        setShowStatusPopover(false);
      }
    };
    const handleOutsideClick = () => {
      setShowStatusPopover(false);
    };
    const preventContextMenu = (e) => e.preventDefault();
    window.addEventListener('keydown', handleKeyDown);
    window.addEventListener('click', handleOutsideClick);
    window.addEventListener('contextmenu', preventContextMenu);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
      window.removeEventListener('click', handleOutsideClick);
      window.removeEventListener('contextmenu', preventContextMenu);
    };
  }, []);

  useEffect(() => {
    if (window.runtime?.EventsOn) {
      window.runtime.EventsOn('print-progress', (progress) => {
        if (progress.pagesPrinted === -1) {
          setTimeout(() => {
            setActiveJobProgress(prev => {
              if (prev?.jobId === progress.jobId) return null;
              return prev;
            });
          }, 1500);
        } else {
          setActiveJobProgress(progress);
        }
      });
      window.runtime.EventsOn('open-settings-update', () => {
        setShowSettings(true);
      });
      return () => {
        if (window.runtime?.EventsOff) {
          window.runtime.EventsOff('print-progress');
          window.runtime.EventsOff('open-settings-update');
        }
      };
    }
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


          {/* Server Status */}
          <div
            id="server-status"
            onClick={(e) => {
              e.stopPropagation();
              setShowStatusPopover(!showStatusPopover);
            }}
            className="flex items-center gap-2 px-3 py-1.5 rounded-xl bg-surface-800/80 border border-surface-700/50 cursor-pointer hover:bg-surface-700/50 transition-colors relative select-none"
            title="Click to view server diagnostics"
          >
            <div className={`w-2 h-2 rounded-full ${status.mdnsActive && status.httpActive ? 'bg-success-400 animate-pulse' : 'bg-error-400'}`} />
            <span className="text-[11px] font-medium text-slate-300">Status</span>
            <span className={`text-[10px] font-semibold px-1.5 py-0.5 rounded ${
              status.mdnsActive && status.httpActive
                ? 'text-success-400 bg-success-400/10'
                : 'text-error-400 bg-error-400/10'
            }`}>
              {status.mdnsActive && status.httpActive ? 'Online' : 'Offline'}
            </span>

            {showStatusPopover && (
              <div
                onClick={(e) => e.stopPropagation()}
                className="absolute right-0 top-full mt-2 w-72 bg-surface-900 border border-surface-700/50 rounded-2xl p-4 shadow-2xl z-50 flex flex-col gap-3.5 animate-fade-in"
              >
                <div className="flex items-center justify-between pb-2 border-b border-surface-800">
                  <span className="text-xs font-bold text-slate-200">Server Diagnostics</span>
                  <span className={`text-[10px] font-semibold px-2 py-0.5 rounded uppercase ${
                    status.mdnsActive && status.httpActive
                      ? 'text-success-400 bg-success-400/10'
                      : 'text-error-400 bg-error-400/10'
                  }`}>
                    {status.mdnsActive && status.httpActive ? 'Online' : 'Offline'}
                  </span>
                </div>

                <div className="flex flex-col gap-2 text-xs">
                  <div className="flex justify-between items-center">
                    <span className="text-slate-500">Computer Name</span>
                    <span className="text-slate-300 font-semibold">{serverName}</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-slate-500">mDNS Service</span>
                    <span className={`font-semibold ${status.mdnsActive ? 'text-success-400' : 'text-error-400'}`}>
                      {status.mdnsActive ? 'Active' : 'Inactive'}
                    </span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-slate-500">HTTP Port</span>
                    <span className="text-slate-300 font-mono">8080</span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-slate-500">HTTP Service</span>
                    <span className={`font-semibold ${status.httpActive ? 'text-success-400' : 'text-error-400'}`}>
                      {status.httpActive ? 'Active' : 'Inactive'}
                    </span>
                  </div>
                  <div className="flex justify-between items-center">
                    <span className="text-slate-500">Available Printers</span>
                    <span className="text-slate-300 font-semibold">{status.printerCount}</span>
                  </div>
                  <div className="flex justify-between items-start">
                    <span className="text-slate-500 shrink-0">IP Addresses</span>
                    <span className="text-slate-300 text-right font-mono truncate max-w-[140px]" title={ips.join(', ')}>
                      {ips.length > 0 ? ips.join(', ') : 'Loading...'}
                    </span>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      </header>

      {/* ===== MAIN CONTENT ===== */}
      <main className="flex-1 flex min-h-0 p-5 gap-5">
        {/* Settings View */}
        <section
          className="flex-1 glass-card p-6 flex flex-col animate-fade-in"
          style={{ display: showSettings ? 'flex' : 'none' }}
        >
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
            appVersion={appVersion}
          />
        </section>

        {/* Dashboard View */}
        <div
          className="flex-1 flex min-h-0 gap-5"
          style={{ display: showSettings ? 'none' : 'flex' }}
        >
          {/* Left Column: Printers + Devices */}
          <section className="w-[340px] shrink-0 flex flex-col gap-5 animate-fade-in">
            <div className={`glass-card p-5 flex flex-col transition-all duration-300 min-h-0 ${
              printersCollapsed ? 'h-[72px] shrink-0 overflow-hidden' : 'flex-1'
            }`}>
              <PrinterList
                remotePrinters={remotePrinters}
                collapsed={printersCollapsed}
                onToggleCollapse={() => setPrintersCollapsed(!printersCollapsed)}
              />
            </div>
            <div className={`glass-card p-5 flex flex-col transition-all duration-300 min-h-0 ${
              devicesCollapsed ? 'h-[72px] shrink-0 overflow-hidden' : printersCollapsed ? 'flex-1' : 'h-[340px] shrink-0'
            }`}>
              <DeviceList
                remotePrinters={remotePrinters}
                onRemotePrintersUpdate={handleRemotePrintersUpdate}
                collapsed={devicesCollapsed}
                onToggleCollapse={() => setDevicesCollapsed(!devicesCollapsed)}
              />
            </div>
          </section>

          {/* Middle Column: Print Queue */}
          <section className="flex-1 glass-card p-5 flex flex-col animate-fade-in" style={{ animationDelay: '60ms' }}>
            <JobQueue />
          </section>
        </div>

      </main>


      {/* ===== FOOTER ===== */}
      <footer className="flex items-center justify-between px-6 py-2.5 border-t border-surface-700/30 text-[10px] text-slate-600">
        <span>TakePrint v{appVersion}</span>
        <span>
          {status.printerCount} printer{status.printerCount !== 1 ? 's' : ''} available
        </span>
      </footer>

      {showQRModal && (
        <QRModal
          onClose={() => setShowQRModal(false)}
          serverName={serverName}
          ips={ips}
        />
      )}

      {/* Floating Print Progress Toast (Bottom Right) */}
      {activeJobProgress && (
        <div className="fixed bottom-6 right-6 w-80 bg-surface-900 border border-surface-700/50 rounded-2xl p-4 shadow-2xl z-50 flex flex-col gap-3 animate-slide-in">
          {/* Header */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-accent-400 opacity-75"></span>
                <span className="relative inline-flex rounded-full h-2 w-2 bg-accent-500"></span>
              </span>
              <span className="text-xs font-bold text-slate-200">Printing Spooler Job</span>
            </div>
            <button
              onClick={() => setActiveJobProgress(null)}
              className="text-slate-500 hover:text-slate-300 p-0.5 rounded transition-colors cursor-pointer"
              title="Close notification"
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>

          {/* Details */}
          <div className="min-w-0">
            <h4 className="text-xs font-semibold text-slate-200 truncate" title={activeJobProgress.filename}>
              {activeJobProgress.filename}
            </h4>
            <p className="text-[10px] text-slate-400 mt-0.5 truncate">
              Printer: <span className="text-slate-300 font-medium">{activeJobProgress.printerName}</span>
            </p>
          </div>

          {/* Progress Bar / Counter */}
          <div className="space-y-1.5">
            <div className="flex items-center justify-between text-[10px] font-semibold text-slate-500 uppercase tracking-wider">
              <span>Pages Printed</span>
              <span className="text-accent-400">
                {activeJobProgress.pagesPrinted >= 0 ? `${activeJobProgress.pagesPrinted} / ${activeJobProgress.totalPages}` : 'Finished'}
              </span>
            </div>
            <div className="w-full h-1.5 bg-surface-800 rounded-full overflow-hidden">
              <div
                className="h-full bg-accent-500 transition-all duration-300 rounded-full"
                style={{
                  width: `${activeJobProgress.pagesPrinted >= 0 ? (activeJobProgress.pagesPrinted / activeJobProgress.totalPages) * 100 : 100}%`
                }}
              />
            </div>
          </div>

          {/* Cancel Button */}
          {activeJobProgress.pagesPrinted >= 0 && (
            <button
              onClick={async () => {
                if (window.go?.main?.App?.CancelPrintJob) {
                  try {
                    await window.go.main.App.CancelPrintJob(activeJobProgress.jobId);
                    setActiveJobProgress(null);
                  } catch (err) {
                    console.error(err);
                  }
                }
              }}
              className="w-full py-1.5 bg-error-500/10 hover:bg-error-500/20 text-error-400 hover:text-error-300 border border-error-500/20 rounded-xl text-[10px] font-semibold tracking-wide uppercase transition-all duration-200 cursor-pointer"
            >
              Cancel Print Job
            </button>
          )}
        </div>
      )}
    </div>
  );
}

function QRModal({ onClose, serverName, ips }) {
  const [token, setToken] = useState('');
  const canvasRef = useRef(null);

  useEffect(() => {
    const fetchToken = async () => {
      if (window.go?.main?.App?.GetAuthToken) {
        try {
          const t = await window.go.main.App.GetAuthToken();
          setToken(t);
        } catch (err) {
          console.error("Failed to get auth token", err);
        }
      } else {
        setToken("mock_token_12345");
      }
    };
    fetchToken();
  }, []);

  useEffect(() => {
    if (ips.length > 0 && canvasRef.current) {
      const qrData = JSON.stringify({
        name: serverName,
        ips: ips,
        port: 8080,
        token: token
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
  }, [ips, serverName, token]);

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
  setTheme,
  appVersion
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

        {/* System Console / Logs */}
        <div className="flex flex-col gap-2 pt-4 border-t border-surface-700/30 h-[320px] min-h-[300px]">
          <LogConsole />
        </div>

        {/* Software Updates */}
        <div className="flex flex-col gap-2 pt-4 border-t border-surface-700/30">
          <label className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Software Updates</label>
          <p className="text-[11px] text-slate-500 mb-3">Check if you are running the latest version of TakePrint and download updates.</p>
          <UpdateChecker currentVersion={appVersion} />
        </div>
      </div>
    </div>
  );
}


function UpdateChecker({ currentVersion }) {
  const [status, setStatus] = useState('idle'); // 'idle' | 'checking' | 'latest' | 'available' | 'downloading' | 'ready' | 'installing' | 'error'
  const [latestInfo, setLatestInfo] = useState(null);
  const [progress, setProgress] = useState(0);
  const [errorMsg, setErrorMsg] = useState('');

  useEffect(() => {
    if (window.runtime?.EventsOn) {
      window.runtime.EventsOn('update_progress', (p) => {
        setStatus('downloading');
        setProgress(p);
      });
      window.runtime.EventsOn('update_status', (s) => {
        if (s === 'launching') {
          setStatus('installing');
        } else if (s === 'ready') {
          setStatus('ready');
        }
      });
      window.runtime.EventsOn('update_error', (err) => {
        setStatus('error');
        setErrorMsg(err);
      });
    }
  }, []);

  const handleCheck = async () => {
    setStatus('checking');
    setErrorMsg('');
    if (window.go?.main?.App?.CheckForUpdate) {
      try {
        const res = await window.go.main.App.CheckForUpdate();
        if (res.updateAvailable) {
          setLatestInfo(res);
          setStatus('available');
        } else {
          setStatus('latest');
        }
      } catch (err) {
        setStatus('error');
        setErrorMsg(err.toString());
      }
    } else {
      // Mock for development
      setTimeout(() => {
        setStatus('available');
        setLatestInfo({
          latestVersion: 'v1.0.3',
          downloadUrl: 'https://github.com/devkiraa/takeprint-desktop/releases/download/v1.0.3/TakePrint-Desktop-Installer.exe',
          releaseNotes: 'Performance improvements, custom installer logos, and an integrated auto-updater.'
        });
      }, 1200);
    }
  };

  const handleInstall = async () => {
    if (!latestInfo) return;
    setStatus('downloading');
    setProgress(0);
    if (window.go?.main?.App?.StartUpdate) {
      try {
        await window.go.main.App.StartUpdate(latestInfo.downloadUrl);
      } catch (err) {
        setStatus('error');
        setErrorMsg(err.toString());
      }
    } else {
      // Mock for development
      let p = 0;
      const interval = setInterval(() => {
        p += 5;
        setProgress(p);
        if (p >= 100) {
          clearInterval(interval);
          setStatus('ready');
        }
      }, 100);
    }
  };

  const handleLaunch = async () => {
    setStatus('installing');
    if (window.go?.main?.App?.LaunchInstaller) {
      try {
        await window.go.main.App.LaunchInstaller();
      } catch (err) {
        setStatus('error');
        setErrorMsg(err.toString());
      }
    } else {
      // Mock for development
      alert("Launching installer mock...");
      setStatus('idle');
    }
  };

  return (
    <div className="bg-surface-800/40 border border-surface-700/30 rounded-xl p-4 flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <div className="flex flex-col">
          <span className="text-xs font-semibold text-slate-300">
            Current Version: <span className="font-mono text-accent-400">v{currentVersion}</span>
          </span>
          {status === 'checking' && <span className="text-[10px] text-slate-500 mt-0.5 animate-pulse">Checking GitHub Releases...</span>}
          {status === 'latest' && <span className="text-[10px] text-success-400 mt-0.5 font-medium">You are running the latest version!</span>}
          {status === 'available' && (
            <span className="text-[10px] text-accent-400 mt-0.5 font-semibold animate-pulse">
              New version available: {latestInfo?.latestVersion}
            </span>
          )}
          {status === 'downloading' && (
            <span className="text-[10px] text-slate-400 mt-0.5">
              Downloading installer: {Math.round(progress)}%
            </span>
          )}
          {status === 'ready' && (
            <span className="text-[10px] text-success-400 mt-0.5 font-semibold animate-pulse">
              Update downloaded! Ready to install.
            </span>
          )}
          {status === 'installing' && (
            <span className="text-[10px] text-success-400 mt-0.5 font-semibold animate-pulse">
              Launching setup wizard... App will exit.
            </span>
          )}
          {status === 'error' && (
            <span className="text-[10px] text-error-400 mt-0.5 font-medium truncate max-w-sm">
              Error: {errorMsg}
            </span>
          )}
        </div>

        {(status === 'idle' || status === 'latest' || status === 'error') && (
          <button
            onClick={handleCheck}
            className="px-3.5 py-1.5 rounded-lg bg-surface-800 border border-surface-700/50 text-slate-300 hover:text-slate-100 hover:bg-surface-700/50 text-[11px] font-semibold transition-all cursor-pointer"
          >
            Check for Updates
          </button>
        )}

        {status === 'available' && (
          <button
            onClick={handleInstall}
            className="px-3.5 py-1.5 rounded-lg bg-accent-500 hover:bg-accent-600 text-white text-[11px] font-semibold transition-all cursor-pointer shadow-lg shadow-accent-500/10"
          >
            Update Now
          </button>
        )}

        {status === 'downloading' && (
          <div className="relative overflow-hidden px-3.5 py-1.5 rounded-lg bg-surface-800 border border-surface-700/50 text-slate-300 text-[11px] font-semibold min-w-[120px] text-center select-none">
            <div
              className="absolute left-0 top-0 bottom-0 bg-accent-500/20 transition-all duration-300"
              style={{ width: `${progress}%` }}
            />
            <span className="relative z-10">Downloading {Math.round(progress)}%</span>
          </div>
        )}

        {status === 'ready' && (
          <button
            onClick={handleLaunch}
            className="px-3.5 py-1.5 rounded-lg bg-success-500 hover:bg-success-600 text-white text-[11px] font-semibold transition-all cursor-pointer shadow-lg shadow-success-500/10"
          >
            Restart & Install
          </button>
        )}
      </div>

      {status === 'downloading' && (
        <div className="w-full bg-surface-900 rounded-full h-1.5 overflow-hidden">
          <div className="bg-accent-500 h-1.5 rounded-full transition-all duration-300" style={{ width: `${progress}%` }} />
        </div>
      )}

      {status === 'available' && latestInfo?.releaseNotes && (
        <div className="bg-surface-900/50 border border-surface-700/20 rounded-lg p-2.5 mt-1">
          <span className="text-[10px] font-bold text-slate-400 uppercase tracking-wider block mb-1">Release Notes:</span>
          <p className="text-[10px] text-slate-400 leading-relaxed font-mono whitespace-pre-line max-h-32 overflow-y-auto">
            {latestInfo.releaseNotes}
          </p>
        </div>
      )}
    </div>
  );
}



