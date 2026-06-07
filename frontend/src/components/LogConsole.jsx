import { useState, useEffect, useRef, useCallback } from 'react';

/**
 * LogConsole — Real-time log viewer with color-coded entries.
 * Subscribes to Wails "log" events for live updates.
 */
export default function LogConsole() {
  const [logs, setLogs] = useState([]);
  const [autoScroll, setAutoScroll] = useState(true);
  const scrollRef = useRef(null);
  const maxLogs = 200;

  // Subscribe to Wails real-time log events.
  useEffect(() => {
    // Load existing logs on mount.
    const loadInitialLogs = async () => {
      if (window.go?.main?.App?.GetLogs) {
        try {
          const existing = await window.go.main.App.GetLogs();
          if (existing?.length) {
            setLogs(existing);
          }
        } catch {
          // Wails not ready yet, ignore.
        }
      } else {
        // Mock logs for development outside Wails.
        setLogs([
          { timestamp: '20:30:01', message: 'TakePrint starting up...', level: 'info' },
          { timestamp: '20:30:02', message: 'mDNS broadcasting \'_localshareprint._tcp\' on 192.168.1.42:8080', level: 'info' },
          { timestamp: '20:30:02', message: 'mDNS service started successfully', level: 'success' },
          { timestamp: '20:30:03', message: 'HTTP server listening on :8080', level: 'info' },
          { timestamp: '20:30:03', message: 'HTTP server started on :8080', level: 'success' },
          { timestamp: '20:30:15', message: 'Fetched 3 printer(s)', level: 'info' },
        ]);
      }
    };

    loadInitialLogs();

    // Subscribe to live events if Wails runtime is available.
    if (window.runtime?.EventsOn) {
      window.runtime.EventsOn('log', (entry) => {
        setLogs((prev) => {
          const updated = [...prev, entry];
          return updated.length > maxLogs ? updated.slice(-maxLogs) : updated;
        });
      });
    }

    return () => {
      if (window.runtime?.EventsOff) {
        window.runtime.EventsOff('log');
      }
    };
  }, []);

  // Auto-scroll to bottom.
  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs, autoScroll]);

  const handleScroll = useCallback(() => {
    if (!scrollRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = scrollRef.current;
    setAutoScroll(scrollHeight - scrollTop - clientHeight < 40);
  }, []);

  const clearLogs = () => setLogs([]);

  const getLevelStyle = (level) => {
    switch (level) {
      case 'success':
        return 'text-success-400';
      case 'warn':
        return 'text-warn-400';
      case 'error':
        return 'text-error-400';
      default:
        return 'text-slate-300';
    }
  };

  const getLevelIcon = (level) => {
    switch (level) {
      case 'success': return '●';
      case 'warn': return '▲';
      case 'error': return '✖';
      default: return '›';
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* Section Header */}
      <div className="flex items-center justify-between mb-4 px-1">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-lg bg-accent-500/15 flex items-center justify-center">
            <svg className="w-4 h-4 text-accent-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 7.5l3 2.25-3 2.25m4.5 0h3m-9 8.25h13.5A2.25 2.25 0 0021 18V6a2.25 2.25 0 00-2.25-2.25H5.25A2.25 2.25 0 003 6v12a2.25 2.25 0 002.25 2.25z" />
            </svg>
          </div>
          <h2 className="text-sm font-semibold text-slate-200 tracking-wide uppercase">
            Console
          </h2>
          <span className="text-xs font-medium text-slate-500 bg-surface-700 px-2 py-0.5 rounded-full">
            {logs.length}
          </span>
        </div>
        <div className="flex items-center gap-1.5">
          {/* Auto-scroll toggle */}
          <button
            id="auto-scroll-btn"
            onClick={() => setAutoScroll(!autoScroll)}
            className={`p-1.5 rounded-lg text-xs transition-all duration-200 ${
              autoScroll
                ? 'text-accent-400 bg-accent-500/15'
                : 'text-slate-500 hover:text-slate-300 hover:bg-surface-700'
            }`}
            title={autoScroll ? 'Auto-scroll ON' : 'Auto-scroll OFF'}
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 13.5L12 21m0 0l-7.5-7.5M12 21V3" />
            </svg>
          </button>
          {/* Clear button */}
          <button
            id="clear-logs-btn"
            onClick={clearLogs}
            className="p-1.5 rounded-lg text-slate-400 hover:text-error-400 hover:bg-surface-700 transition-all duration-200"
            title="Clear logs"
          >
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
            </svg>
          </button>
        </div>
      </div>

      {/* Log entries */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto rounded-xl bg-surface-950/60 border border-surface-700/50 font-mono text-xs"
      >
        {logs.length === 0 ? (
          <div className="flex items-center justify-center h-full text-slate-600">
            <p>Waiting for events...</p>
          </div>
        ) : (
          <div className="p-3 space-y-0.5">
            {logs.map((log, index) => (
              <div
                key={index}
                className="flex items-start gap-2 py-1 px-2 rounded-lg hover:bg-surface-800/50 transition-colors duration-150 animate-slide-in"
                style={{ animationDelay: `${Math.min(index * 15, 300)}ms` }}
              >
                <span className="text-surface-500 select-none shrink-0 w-[60px]">
                  {log.timestamp}
                </span>
                <span className={`select-none shrink-0 w-3 text-center ${getLevelStyle(log.level)}`}>
                  {getLevelIcon(log.level)}
                </span>
                <span className={`${getLevelStyle(log.level)} break-all`}>
                  {log.message}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Footer */}
      {!autoScroll && logs.length > 0 && (
        <button
          onClick={() => {
            setAutoScroll(true);
            if (scrollRef.current) {
              scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
            }
          }}
          className="mt-2 w-full py-1.5 text-xs text-accent-400 bg-accent-500/10 rounded-lg hover:bg-accent-500/20 transition-colors duration-200"
        >
          ↓ Scroll to latest
        </button>
      )}
    </div>
  );
}
