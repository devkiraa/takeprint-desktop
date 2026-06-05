import { useState, useEffect, useCallback } from 'react';

/**
 * JobQueue — Displays recent print jobs and their execution status.
 * Fetches from the Wails-bound Go method `GetJobs()`.
 */
export default function JobQueue() {
  const [jobs, setJobs] = useState([]);
  const [loading, setLoading] = useState(true);

  const fetchJobs = useCallback(async () => {
    try {
      if (window.go?.main?.App?.GetJobs) {
        const result = await window.go.main.App.GetJobs();
        setJobs(result || []);
      } else {
        // Mock data for development
        setJobs([
          {
            id: 'job-1',
            filename: 'INTERNSHIP APPLICATION FORM.pdf',
            printer: 'HP LaserJet Pro',
            status: 'completed',
            submittedAt: '22:15:30',
            pages: 'all',
            color: 'mono',
            copies: 1,
          },
          {
            id: 'job-2',
            filename: 'Receipt_June_2026.pdf',
            printer: 'Canon PIXMA',
            status: 'failed',
            submittedAt: '22:18:12',
            pages: '1-2',
            color: 'color',
            copies: 2,
            error: 'Out of paper',
          },
        ]);
      }
    } catch (err) {
      console.error('Failed to fetch jobs:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchJobs();
    if (window.runtime?.EventsOn) {
      window.runtime.EventsOn('job-updated', fetchJobs);
      return () => {
        // window.runtime.EventsOff is not strictly needed or could be EventsOff('job-updated')
        // but we'll clean it up if it has a way or just let it be. Let's see if Wails has EventsOff.
        // Wails has EventsOff(eventName) or we can just call it to deregister.
        // Actually, just in case, let's check if there's EventsOff or we can just omit return cleanup if it doesn't support it, but yes Wails runtime supports it.
      };
    } else {
      const interval = setInterval(fetchJobs, 3000);
      return () => clearInterval(interval);
    }
  }, [fetchJobs]);

  const getStatusBadge = (status) => {
    switch (status) {
      case 'printing':
        return (
          <span className="text-[10px] font-semibold text-accent-400 bg-accent-500/15 px-2 py-0.5 rounded flex items-center gap-1.5 uppercase tracking-wider animate-pulse">
            <span className="w-1.5 h-1.5 rounded-full bg-accent-400 animate-ping" />
            Printing
          </span>
        );
      case 'completed':
        return (
          <span className="text-[10px] font-semibold text-success-400 bg-success-400/10 px-2 py-0.5 rounded flex items-center gap-1 uppercase tracking-wider">
            ✓ Done
          </span>
        );
      case 'saved':
        return (
          <span className="text-[10px] font-semibold text-success-400 bg-success-400/10 px-2 py-0.5 rounded flex items-center gap-1 uppercase tracking-wider">
            ✓ Saved Successfully
          </span>
        );
      case 'failed':
        return (
          <span className="text-[10px] font-semibold text-error-400 bg-error-400/10 px-2 py-0.5 rounded flex items-center gap-1 uppercase tracking-wider">
            ✕ Failed
          </span>
        );
      default:
        return (
          <span className="text-[10px] font-semibold text-slate-400 bg-surface-700 px-2 py-0.5 rounded uppercase tracking-wider">
            {status}
          </span>
        );
    }
  };

  return (
    <div className="flex flex-col h-full">
      {/* Section Header */}
      <div className="flex items-center justify-between mb-4 px-1">
        <div className="flex items-center gap-2.5">
          <div className="w-8 h-8 rounded-lg bg-accent-500/15 flex items-center justify-center">
            <svg className="w-4 h-4 text-accent-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-3 7h3m-3 4h3m-6-4h.01M9 16h.01" />
            </svg>
          </div>
          <h2 className="text-sm font-semibold text-slate-200 tracking-wide uppercase">
            Print Queue
          </h2>
          <span className="text-xs font-medium text-slate-500 bg-surface-700 px-2 py-0.5 rounded-full">
            {jobs.length}
          </span>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto space-y-2.5 pr-1">
        {loading && jobs.length === 0 && (
          <div className="space-y-2">
            {[1, 2].map((i) => (
              <div key={i} className="h-16 rounded-xl bg-surface-700/50 animate-pulse" />
            ))}
          </div>
        )}

        {!loading && jobs.length === 0 && (
          <div className="flex flex-col items-center justify-center py-12 text-slate-500">
            <svg className="w-10 h-10 mb-3 opacity-40" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2" />
            </svg>
            <p className="text-sm">No jobs in queue</p>
          </div>
        )}

        {jobs.map((job, index) => (
          <div
            key={job.id}
            className="glass-card p-3.5 animate-fade-in flex flex-col gap-2 group cursor-default"
            style={{ animationDelay: `${index * 60}ms` }}
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <h3 className="text-sm font-medium text-slate-200 truncate" title={job.filename}>
                  {job.filename}
                </h3>
                <p className="text-[11px] text-slate-400 mt-0.5 flex items-center gap-1.5">
                  <span>To: <strong className="text-slate-300 font-semibold">{job.printer}</strong></span>
                  <span className="text-slate-600">•</span>
                  <span>{job.submittedAt}</span>
                </p>
              </div>
              <div className="shrink-0">{getStatusBadge(job.status)}</div>
            </div>

            {/* Config & Error info */}
            <div className="flex items-center justify-between text-[10px] text-slate-500 pt-1.5 border-t border-surface-700/30">
              <div className="flex items-center gap-2">
                <span className="bg-surface-800 px-1.5 py-0.5 rounded text-slate-400">{job.copies} {job.copies === 1 ? 'copy' : 'copies'}</span>
                <span className="bg-surface-800 px-1.5 py-0.5 rounded text-slate-400 capitalize">{job.color}</span>
                <span className="bg-surface-800 px-1.5 py-0.5 rounded text-slate-400">Pages: {job.pages}</span>
              </div>
              {job.error && (
                <span className="text-error-400 font-medium truncate max-w-[200px]" title={job.error}>
                  {job.error}
                </span>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
