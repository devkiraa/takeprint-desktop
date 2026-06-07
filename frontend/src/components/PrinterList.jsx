import { useState, useEffect, useCallback } from 'react';

/**
 * PrinterList — Shows locally installed printers with status badges,
 * plus remote printers from connected TakePrint devices.
 * Fetches from the Wails-bound Go method `GetPrinters()`.
 */
export default function PrinterList({ remotePrinters = [], collapsed, onToggleCollapse }) {
  const [printers, setPrinters] = useState([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(null);

  const fetchPrinters = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      // Call the Wails-bound Go method.
      if (window.go?.main?.App?.GetPrinters) {
        const result = await window.go.main.App.GetPrinters();
        setPrinters(result || []);
      } else {
        // Fallback mock data for development outside Wails.
        setPrinters([
          { name: 'HP LaserJet Pro', status: 'Normal', isDefault: true, shared: true },
          { name: 'Canon PIXMA', status: 'Idle', isDefault: false, shared: true },
          { name: 'Microsoft Print to PDF', status: 'Normal', isDefault: false, shared: false },
        ]);
      }
    } catch (err) {
      setError(err.message || 'Failed to fetch printers');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchPrinters();
    const interval = setInterval(fetchPrinters, 30000);
    return () => clearInterval(interval);
  }, [fetchPrinters]);

  const getStatusColor = (status) => {
    switch (status?.toLowerCase()) {
      case 'normal':
      case 'idle':
        return 'text-success-400 bg-success-400/10';
      case 'paused':
      case 'offline':
      case 'disabled':
        return 'text-warn-400 bg-warn-400/10';
      case 'error':
      case 'paper jam':
      case 'paper out':
        return 'text-error-400 bg-error-400/10';
      default:
        return 'text-slate-400 bg-slate-400/10';
    }
  };

  const getStatusDotColor = (status) => {
    switch (status?.toLowerCase()) {
      case 'normal':
      case 'idle':
        return 'bg-success-400 text-success-400';
      case 'paused':
      case 'offline':
      case 'disabled':
        return 'bg-warn-400 text-warn-400';
      case 'error':
      case 'paper jam':
      case 'paper out':
        return 'bg-error-400 text-error-400';
      default:
        return 'bg-slate-400 text-slate-400';
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* Section Header */}
      <div className={`flex items-center justify-between px-1 ${collapsed ? '' : 'mb-4'}`}>
        <div onClick={onToggleCollapse} className="flex items-center gap-2.5 cursor-pointer hover:opacity-80 select-none">
          <div className="w-8 h-8 rounded-lg bg-accent-500/15 flex items-center justify-center">
            <svg className="w-4 h-4 text-accent-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M6.72 13.829c-.24.03-.48.062-.72.096m.72-.096a42.415 42.415 0 0110.56 0m-10.56 0L6.34 18m10.94-4.171c.24.03.48.062.72.096m-.72-.096L17.66 18m0 0l.229 2.523a1.125 1.125 0 01-1.12 1.227H7.231c-.662 0-1.18-.568-1.12-1.227L6.34 18m11.318 0h1.091A2.25 2.25 0 0021 15.75V9.456c0-1.081-.768-2.015-1.837-2.175a48.055 48.055 0 00-1.913-.247M6.34 18H5.25A2.25 2.25 0 013 15.75V9.456c0-1.081.768-2.015 1.837-2.175a48.041 48.041 0 011.913-.247m10.5 0a48.536 48.536 0 00-10.5 0m10.5 0V3.375c0-.621-.504-1.125-1.125-1.125h-8.25c-.621 0-1.125.504-1.125 1.125v3.659M18.25 7.034V3.375" />
            </svg>
          </div>
          <h2 className="text-sm font-semibold text-slate-200 tracking-wide uppercase">
            Printers
          </h2>
          <span className="text-xs font-medium text-slate-500 bg-surface-700 px-2 py-0.5 rounded-full">
            {printers.length}
          </span>
          <svg className={`w-3.5 h-3.5 text-slate-500 transition-transform duration-200 ${collapsed ? '-rotate-90' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 8.25l-7.5 7.5-7.5-7.5" />
          </svg>
        </div>
        {!collapsed && (
          <button
            id="refresh-printers-btn"
            onClick={fetchPrinters}
            disabled={loading}
            className="p-1.5 rounded-lg text-slate-400 hover:text-accent-400 hover:bg-surface-700 transition-all duration-200 disabled:opacity-40"
            title="Refresh printers"
          >
            <svg className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182" />
            </svg>
          </button>
        )}
      </div>

      {/* Content */}
      {!collapsed && (
        <div className="flex-1 overflow-y-auto space-y-2 pr-1">
          {error && (
            <div className="p-3 rounded-xl bg-error-500/10 border border-error-500/20 text-error-400 text-xs animate-fade-in">
              ⚠️ {error}
            </div>
          )}

          {loading && printers.length === 0 && (
            <div className="space-y-2">
              {[1, 2, 3].map((i) => (
                <div key={i} className="h-16 rounded-xl bg-surface-700/50 animate-pulse" />
              ))}
            </div>
          )}

          {!loading && printers.length === 0 && !error && (
            <div className="flex flex-col items-center justify-center py-12 text-slate-500">
              <svg className="w-10 h-10 mb-3 opacity-40" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M6.72 13.829c-.24.03-.48.062-.72.096m.72-.096a42.415 42.415 0 0110.56 0m-10.56 0L6.34 18m10.94-4.171c.24.03.48.062.72.096m-.72-.096L17.66 18" />
              </svg>
              <p className="text-sm">No printers found</p>
            </div>
          )}

          {printers.map((printer, index) => (
            <div
              key={printer.name}
              id={`printer-card-${index}`}
              className="glass-card p-3.5 animate-fade-in flex flex-col gap-2.5 group cursor-default"
              style={{ animationDelay: `${index * 60}ms` }}
            >
              {/* Top row: Name & Share Switch */}
              <div className="flex items-center justify-between gap-3">
                {/* Printer info */}
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <h3 className="text-sm font-medium text-slate-200 truncate" title={printer.name}>
                      {printer.name}
                    </h3>
                    {printer.isDefault && (
                      <span className="text-[10px] font-semibold text-accent-400 bg-accent-500/15 px-1.5 py-0.5 rounded uppercase tracking-wider">
                        Default
                      </span>
                    )}
                  </div>
                </div>

                {/* Share Toggle Switch */}
                <div className="flex items-center gap-2 shrink-0">
                  <span className="text-[10px] font-semibold text-slate-500 uppercase">
                    {printer.shared !== false ? 'Shared' : 'Hidden'}
                  </span>
                  <button
                    onClick={async (e) => {
                      e.stopPropagation();
                      const newShared = printer.shared === false; // Toggle
                      setPrinters(prev => prev.map(p => p.name === printer.name ? { ...p, shared: newShared } : p));
                      
                      if (window.go?.main?.App?.TogglePrinterShare) {
                        try {
                          await window.go.main.App.TogglePrinterShare(printer.name, newShared);
                        } catch (err) {
                          console.error(err);
                          setPrinters(prev => prev.map(p => p.name === printer.name ? { ...p, shared: !newShared } : p));
                        }
                      }
                    }}
                    className={`w-9 h-5 rounded-full p-0.5 transition-all duration-200 cursor-pointer ${
                      printer.shared !== false ? 'bg-accent-500' : 'bg-surface-700 border border-surface-600'
                    }`}
                  >
                    <div className={`w-4 h-4 rounded-full bg-white transition-all duration-200 transform ${
                      printer.shared !== false ? 'translate-x-4' : 'translate-x-0'
                    }`} />
                  </button>
                </div>
              </div>

              {/* Bottom row: Supplies (Ink/Toner cartridge levels) */}
              {printer.supplies && printer.supplies.length > 0 && (
                <div className="pt-2 border-t border-surface-700/30 flex flex-col gap-1.5">
                  <div className="flex items-center justify-between text-[9px] font-semibold text-slate-500 uppercase tracking-wider">
                    <span>Supplies</span>
                    <span>{printer.supplies[0]?.type === 'toner' ? 'Toner' : 'Ink'}</span>
                  </div>
                  <div className="grid grid-cols-4 gap-2">
                    {printer.supplies.map((s) => {
                      let barBg = '#64748b'; // default slate
                      if (s.name.toLowerCase() === 'cyan') barBg = '#06b6d4'; // cyan-500
                      else if (s.name.toLowerCase() === 'magenta') barBg = '#ec4899'; // pink-500
                      else if (s.name.toLowerCase() === 'yellow') barBg = '#eab308'; // yellow-500
                      else if (s.name.toLowerCase() === 'black') barBg = '#cbd5e1'; // slate-300

                      return (
                        <div key={s.name} className="flex flex-col gap-1">
                          <div className="flex items-center justify-between text-[8px] text-slate-400">
                            <span>{s.name[0]}</span>
                            <span className="font-semibold">{Math.round(s.percent)}%</span>
                          </div>
                          <div className="w-full h-1 bg-surface-800 rounded-full overflow-hidden">
                            <div
                              className="h-full transition-all duration-500 rounded-full"
                              style={{ backgroundColor: barBg, width: `${s.percent}%` }}
                            />
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>
          ))}

          {/* ===== REMOTE / NETWORK PRINTERS ===== */}
          {remotePrinters.length > 0 && (
            <>
              <div className="flex items-center gap-2 pt-3 pb-1 px-1">
                <svg className="w-3.5 h-3.5 text-accent-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 21a9.004 9.004 0 008.354-5.646M12 21a9.004 9.004 0 01-8.354-5.646M12 21V3m0 0a9.004 9.004 0 018.354 5.646M12 3a9.004 9.004 0 00-8.354 5.646m16.708 0a18.45 18.45 0 01-2.57 6.708M3.646 8.646a18.45 18.45 0 002.57 6.708" />
                </svg>
                <span className="text-[10px] font-semibold text-slate-400 uppercase tracking-wider">Network Printers</span>
                <span className="text-[10px] font-medium text-slate-600 bg-surface-700 px-1.5 py-0.5 rounded-full">
                  {remotePrinters.length}
                </span>
              </div>
              {remotePrinters.map((rp, index) => (
                <div
                  key={`${rp.deviceName}-${rp.name}`}
                  className="glass-card p-3.5 animate-fade-in flex flex-col gap-2 cursor-default"
                  style={{ animationDelay: `${(printers.length + index) * 60}ms` }}
                >
                  <div className="flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <h3 className="text-sm font-medium text-slate-200 truncate" title={rp.name}>
                        {rp.name}
                      </h3>
                      <div className="flex items-center gap-1.5 mt-0.5">
                        <svg className="w-3 h-3 text-accent-400 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                          <path strokeLinecap="round" strokeLinejoin="round" d="M9 17.25v1.007a3 3 0 01-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0115 18.257V17.25m6-12V15a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 15V5.25A2.25 2.25 0 015.25 3h13.5A2.25 2.25 0 0121 5.25z" />
                        </svg>
                        <span className="text-[10px] text-accent-400 font-semibold truncate">{rp.deviceName}</span>
                      </div>
                    </div>
                    <span className={`text-[10px] font-semibold px-2 py-0.5 rounded uppercase tracking-wider ${
                      rp.status?.toLowerCase() === 'normal' || rp.status?.toLowerCase() === 'idle'
                        ? 'text-success-400 bg-success-400/10'
                        : 'text-slate-400 bg-surface-700'
                    }`}>
                      {rp.status}
                    </span>
                  </div>
                </div>
              ))}
            </>
          )}
        </div>
      )}
    </div>
  );
}
