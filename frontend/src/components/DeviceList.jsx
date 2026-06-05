import { useState, useEffect, useCallback } from 'react';

/**
 * DeviceList — Displays connected remote TakePrint servers and allows
 * scanning for / adding new devices on the local network.
 */
export default function DeviceList({ onRemotePrintersUpdate }) {
  const [devices, setDevices] = useState([]);
  const [loading, setLoading] = useState(true);
  const [showScanModal, setShowScanModal] = useState(false);

  const fetchDevices = useCallback(async () => {
    try {
      if (window.go?.main?.App?.GetConnectedDevices) {
        const result = await window.go.main.App.GetConnectedDevices();
        setDevices(result || []);
      } else {
        // Mock for dev
        setDevices([
          { name: 'Office-PC', ips: ['192.168.1.105'], port: 8080, status: 'online', activeIP: '192.168.1.105' },
          { name: 'Living Room', ips: ['192.168.1.110'], port: 8080, status: 'offline', activeIP: '' },
        ]);
      }
    } catch (err) {
      console.error('Failed to fetch devices:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  // Fetch remote printers whenever devices change.
  const fetchRemotePrinters = useCallback(async () => {
    try {
      if (window.go?.main?.App?.GetAllRemotePrinters) {
        const printers = await window.go.main.App.GetAllRemotePrinters();
        onRemotePrintersUpdate?.(printers || []);
      }
    } catch (err) {
      console.error('Failed to fetch remote printers:', err);
    }
  }, [onRemotePrintersUpdate]);

  useEffect(() => {
    fetchDevices();
    const interval = setInterval(fetchDevices, 15000);
    return () => clearInterval(interval);
  }, [fetchDevices]);

  useEffect(() => {
    if (devices.length > 0) {
      fetchRemotePrinters();
      const interval = setInterval(fetchRemotePrinters, 20000);
      return () => clearInterval(interval);
    } else {
      onRemotePrintersUpdate?.([]);
    }
  }, [devices, fetchRemotePrinters, onRemotePrintersUpdate]);

  const handleRemoveDevice = async (name) => {
    if (window.go?.main?.App?.RemoveRemoteDevice) {
      await window.go.main.App.RemoveRemoteDevice(name);
    }
    setDevices(prev => prev.filter(d => d.name !== name));
  };

  const handleDeviceAdded = () => {
    fetchDevices();
    fetchRemotePrinters();
  };

  return (
    <div className="flex flex-col h-full">
      {/* Section Header */}
      <div className="flex items-center justify-between mb-4 px-1">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-lg bg-accent-500/15 flex items-center justify-center">
            <svg className="w-4 h-4 text-accent-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 17.25v1.007a3 3 0 01-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0115 18.257V17.25m6-12V15a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 15V5.25A2.25 2.25 0 015.25 3h13.5A2.25 2.25 0 0121 5.25z" />
            </svg>
          </div>
          <h2 className="text-sm font-semibold text-slate-200 tracking-wide uppercase">
            Devices
          </h2>
          <span className="text-xs font-medium text-slate-500 bg-surface-700 px-2 py-0.5 rounded-full">
            {devices.length}
          </span>
        </div>
        <button
          onClick={() => setShowScanModal(true)}
          className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-xl border border-accent-500/30 bg-accent-500/10 text-accent-400 hover:bg-accent-500/20 transition-all duration-200 cursor-pointer"
          title="Add a new TakePrint device"
        >
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
          </svg>
          <span className="text-[10px] font-semibold uppercase tracking-wider">Add</span>
        </button>
      </div>

      {/* Device Cards */}
      <div className="flex-1 overflow-y-auto space-y-2 pr-1">
        {loading && devices.length === 0 && (
          <div className="space-y-2">
            {[1, 2].map((i) => (
              <div key={i} className="h-14 rounded-xl bg-surface-700/50 animate-pulse" />
            ))}
          </div>
        )}

        {!loading && devices.length === 0 && (
          <div className="flex flex-col items-center justify-center py-10 text-slate-500">
            <svg className="w-10 h-10 mb-3 opacity-40" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 17.25v1.007a3 3 0 01-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0115 18.257V17.25m6-12V15a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 15V5.25A2.25 2.25 0 015.25 3h13.5A2.25 2.25 0 0121 5.25z" />
            </svg>
            <p className="text-sm mb-1">No devices connected</p>
            <p className="text-[11px] text-slate-600">Tap "Add" to scan for devices</p>
          </div>
        )}

        {devices.map((device, index) => (
          <div
            key={device.name}
            className="glass-card p-3.5 animate-fade-in flex items-center justify-between gap-3 group cursor-default"
            style={{ animationDelay: `${index * 60}ms` }}
          >
            <div className="flex items-center gap-3 min-w-0">
              {/* Status dot */}
              <div className={`w-2.5 h-2.5 rounded-full shrink-0 ${
                device.status === 'online'
                  ? 'bg-success-400 animate-pulse'
                  : device.status === 'checking'
                    ? 'bg-warn-400 animate-pulse'
                    : 'bg-error-400'
              }`} />
              <div className="min-w-0">
                <h3 className="text-sm font-medium text-slate-200 truncate">{device.name}</h3>
                <p className="text-[10px] text-slate-500 truncate">
                  {device.activeIP || device.ips?.[0] || 'Unknown IP'}:{device.port}
                  <span className="ml-1.5">•</span>
                  <span className={`ml-1.5 font-semibold uppercase ${
                    device.status === 'online' ? 'text-success-400' :
                    device.status === 'checking' ? 'text-warn-400' : 'text-error-400'
                  }`}>
                    {device.status}
                  </span>
                </p>
              </div>
            </div>
            <button
              onClick={() => handleRemoveDevice(device.name)}
              className="opacity-0 group-hover:opacity-100 p-1.5 rounded-lg text-slate-500 hover:text-error-400 hover:bg-error-400/10 transition-all duration-200 cursor-pointer shrink-0"
              title="Remove device"
            >
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        ))}
      </div>

      {/* Scan Modal */}
      {showScanModal && (
        <ScanModal
          onClose={() => setShowScanModal(false)}
          onDeviceAdded={handleDeviceAdded}
          connectedDeviceNames={devices.map(d => d.name)}
        />
      )}
    </div>
  );
}

/**
 * ScanModal — Scans the network for TakePrint servers via mDNS
 * and allows the user to connect to discovered devices.
 */
function ScanModal({ onClose, onDeviceAdded, connectedDeviceNames }) {
  const [scanning, setScanning] = useState(false);
  const [discovered, setDiscovered] = useState([]);
  const [manualIP, setManualIP] = useState('');
  const [manualName, setManualName] = useState('');
  const [showManual, setShowManual] = useState(false);
  const [connecting, setConnecting] = useState(null);

  const handleScan = async () => {
    setScanning(true);
    setDiscovered([]);
    try {
      if (window.go?.main?.App?.ScanForDevices) {
        const devices = await window.go.main.App.ScanForDevices();
        setDiscovered(devices || []);
      } else {
        // Mock for dev
        await new Promise(r => setTimeout(r, 2000));
        setDiscovered([
          { name: 'Office-PC', ips: ['192.168.1.105'], port: 8080 },
          { name: 'Kitchen-Desktop', ips: ['192.168.1.112'], port: 8080 },
        ]);
      }
    } catch (err) {
      console.error('Scan failed:', err);
    } finally {
      setScanning(false);
    }
  };

  const handleConnect = async (device) => {
    setConnecting(device.name);
    try {
      if (window.go?.main?.App?.AddRemoteDevice) {
        await window.go.main.App.AddRemoteDevice(device.name, device.ips, device.port);
      }
      onDeviceAdded();
      setDiscovered(prev => prev.filter(d => d.name !== device.name));
    } catch (err) {
      console.error('Failed to add device:', err);
      alert('Failed to connect: ' + err);
    } finally {
      setConnecting(null);
    }
  };

  const handleManualAdd = async () => {
    if (!manualIP.trim() || !manualName.trim()) return;
    setConnecting(manualName);
    try {
      const port = 8080;
      if (window.go?.main?.App?.AddRemoteDevice) {
        await window.go.main.App.AddRemoteDevice(manualName.trim(), [manualIP.trim()], port);
      }
      onDeviceAdded();
      setManualIP('');
      setManualName('');
      setShowManual(false);
    } catch (err) {
      console.error('Manual add failed:', err);
      alert('Failed to connect: ' + err);
    } finally {
      setConnecting(null);
    }
  };

  // Auto-scan on mount.
  useEffect(() => {
    handleScan();
  }, []);

  // Filter out already connected devices.
  const filteredDevices = discovered.filter(d => !connectedDeviceNames.includes(d.name));

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm animate-fade-in">
      <div className="bg-surface-900 border border-surface-700/50 rounded-2xl p-6 w-[400px] shadow-2xl flex flex-col max-h-[80vh]">
        {/* Header */}
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-2.5">
            <div className="w-8 h-8 rounded-lg bg-accent-500/15 flex items-center justify-center">
              <svg className="w-4 h-4 text-accent-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" />
              </svg>
            </div>
            <h3 className="text-sm font-bold text-slate-200">Add Device</h3>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-slate-200 cursor-pointer p-1">
            <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Scan Button */}
        <button
          onClick={handleScan}
          disabled={scanning}
          className="w-full flex items-center justify-center gap-2 px-4 py-3 rounded-xl border border-accent-500/30 bg-accent-500/10 text-accent-400 hover:bg-accent-500/20 disabled:opacity-50 transition-all duration-200 cursor-pointer mb-4"
        >
          <svg className={`w-4 h-4 ${scanning ? 'animate-spin' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182" />
          </svg>
          <span className="text-xs font-semibold">
            {scanning ? 'Scanning network...' : 'Scan for Devices'}
          </span>
        </button>

        {/* Scanning animation */}
        {scanning && (
          <div className="flex flex-col items-center py-6 animate-fade-in">
            <div className="relative w-16 h-16 mb-3">
              <div className="absolute inset-0 rounded-full border-2 border-accent-500/20 animate-ping" />
              <div className="absolute inset-2 rounded-full border-2 border-accent-500/30 animate-ping" style={{ animationDelay: '300ms' }} />
              <div className="absolute inset-4 rounded-full border-2 border-accent-500/40 animate-ping" style={{ animationDelay: '600ms' }} />
              <div className="absolute inset-0 flex items-center justify-center">
                <svg className="w-6 h-6 text-accent-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M8.288 15.038a5.25 5.25 0 017.424 0M5.106 11.856c3.807-3.808 9.98-3.808 13.788 0M1.924 8.674c5.565-5.565 14.587-5.565 20.152 0" />
                </svg>
              </div>
            </div>
            <p className="text-[11px] text-slate-400">Looking for TakePrint servers...</p>
          </div>
        )}

        {/* Discovered Devices List */}
        {!scanning && (
          <div className="flex-1 overflow-y-auto space-y-2 mb-4">
            {filteredDevices.length === 0 && discovered.length > 0 && (
              <p className="text-center text-[11px] text-slate-500 py-4">All discovered devices are already connected.</p>
            )}
            {filteredDevices.length === 0 && discovered.length === 0 && !scanning && (
              <p className="text-center text-[11px] text-slate-500 py-4">No new devices found. Try scanning again or add manually.</p>
            )}
            {filteredDevices.map((device) => (
              <div
                key={device.name}
                className="flex items-center justify-between gap-3 p-3 rounded-xl bg-surface-800/60 border border-surface-700/50 animate-fade-in"
              >
                <div className="min-w-0">
                  <h4 className="text-sm font-medium text-slate-200 truncate">{device.name}</h4>
                  <p className="text-[10px] text-slate-500 truncate">{device.ips?.join(', ')}:{device.port}</p>
                </div>
                <button
                  onClick={() => handleConnect(device)}
                  disabled={connecting === device.name}
                  className="shrink-0 flex items-center gap-1.5 px-3 py-1.5 rounded-xl bg-success-500/10 border border-success-500/20 text-success-400 hover:bg-success-500/20 disabled:opacity-50 transition-all duration-200 cursor-pointer"
                >
                  {connecting === device.name ? (
                    <svg className="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182" />
                    </svg>
                  ) : (
                    <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                      <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                    </svg>
                  )}
                  <span className="text-[10px] font-semibold">Connect</span>
                </button>
              </div>
            ))}
          </div>
        )}

        {/* Manual Add Section */}
        <div className="border-t border-surface-700/30 pt-3">
          {!showManual ? (
            <button
              onClick={() => setShowManual(true)}
              className="w-full text-center text-[11px] text-slate-500 hover:text-slate-300 cursor-pointer py-1 transition-colors duration-200"
            >
              Can't find your device? Add manually →
            </button>
          ) : (
            <div className="flex flex-col gap-2 animate-fade-in">
              <p className="text-[11px] text-slate-400 font-semibold uppercase tracking-wider">Manual Connection</p>
              <input
                type="text"
                placeholder="Device name (e.g. Office PC)"
                value={manualName}
                onChange={(e) => setManualName(e.target.value)}
                className="bg-surface-800 border border-surface-600 rounded-xl px-3 py-2 text-xs text-slate-200 focus:outline-none focus:border-accent-500 placeholder:text-slate-600"
              />
              <input
                type="text"
                placeholder="IP Address (e.g. 192.168.1.105)"
                value={manualIP}
                onChange={(e) => setManualIP(e.target.value)}
                className="bg-surface-800 border border-surface-600 rounded-xl px-3 py-2 text-xs text-slate-200 focus:outline-none focus:border-accent-500 placeholder:text-slate-600"
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleManualAdd();
                }}
              />
              <div className="flex gap-2">
                <button
                  onClick={handleManualAdd}
                  disabled={!manualIP.trim() || !manualName.trim() || connecting}
                  className="flex-1 px-3 py-2 rounded-xl bg-success-500/10 border border-success-500/20 text-success-400 hover:bg-success-500/20 disabled:opacity-50 text-xs font-medium cursor-pointer"
                >
                  Add Device
                </button>
                <button
                  onClick={() => setShowManual(false)}
                  className="px-3 py-2 rounded-xl bg-surface-800 border border-surface-700/50 text-slate-400 hover:text-slate-200 text-xs font-medium cursor-pointer"
                >
                  Cancel
                </button>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
