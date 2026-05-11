import React from "react";
import { createRoot } from "react-dom/client";
import { Activity, AlertTriangle, Clock3, FolderOpen, RefreshCcw, WifiOff } from "lucide-react";
import "./styles.css";

const pollInterval = 900;

function App() {
  const [status, setStatus] = React.useState(null);
  const [events, setEvents] = React.useState([]);
  const [connected, setConnected] = React.useState(false);
  const [lastError, setLastError] = React.useState("");

  const refresh = React.useCallback(async () => {
    try {
      const [statusResponse, eventsResponse] = await Promise.all([
        fetch("/api/status", { cache: "no-store" }),
        fetch("/api/events", { cache: "no-store" }),
      ]);

      if (!statusResponse.ok || !eventsResponse.ok) {
        throw new Error("api request failed");
      }

      setStatus(await statusResponse.json());
      const eventData = await eventsResponse.json();
      setEvents(Array.isArray(eventData.events) ? eventData.events : []);
      setConnected(true);
      setLastError("");
    } catch (error) {
      setConnected(false);
      setLastError(error instanceof Error ? error.message : "connection failed");
    }
  }, []);

  React.useEffect(() => {
    refresh();
    const id = window.setInterval(refresh, pollInterval);
    return () => window.clearInterval(id);
  }, [refresh]);

  const newestFirst = React.useMemo(() => [...events].reverse(), [events]);

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="brand">
          <span className="brand-mark" aria-hidden="true">
            <Activity size={22} strokeWidth={2.2} />
          </span>
          <div>
            <h1>dfw Example Watch</h1>
            <p>{status?.root ?? "Waiting for watcher status"}</p>
          </div>
        </div>
        <div className={connected ? "connection is-live" : "connection is-offline"}>
          {connected ? <Activity size={16} /> : <WifiOff size={16} />}
          <span>{connected ? "Live" : "Disconnected"}</span>
        </div>
      </header>

      {!connected && (
        <div className="disconnect-banner" role="status">
          <AlertTriangle size={18} />
          <span>Disconnected from watcher{lastError ? `: ${lastError}` : ""}</span>
        </div>
      )}

      <main className="workspace">
        <section className="status-grid" aria-label="Watcher status">
          <Metric
            icon={<FolderOpen size={18} />}
            label="Path"
            value={status?.root ?? "unknown"}
            wide
          />
          <Metric
            icon={<Activity size={18} />}
            label="Events"
            value={String(status?.event_count ?? events.length)}
          />
          <Metric
            icon={<Clock3 size={18} />}
            label="Started"
            value={formatDate(status?.started_at)}
          />
        </section>

        <section className="timeline-panel">
          <div className="panel-heading">
            <div>
              <h2>Events</h2>
              <p>{events.length} retained in the local ring buffer</p>
            </div>
            <button type="button" className="icon-button" onClick={refresh} title="Refresh events" aria-label="Refresh events">
              <RefreshCcw size={18} />
            </button>
          </div>

          {newestFirst.length === 0 ? (
            <div className="empty-state">
              <Activity size={28} />
              <span>No events recorded yet</span>
            </div>
          ) : (
            <ol className="timeline">
              {newestFirst.map((event) => (
                <li key={event.id} className="event-row">
                  <span className={`event-op op-${opClass(event.op)}`}>{event.op || "event"}</span>
                  <div className="event-body">
                    <div className="event-path">{event.message || event.path || event.full_path}</div>
                    <div className="event-meta">
                      <span>{formatDate(event.observed_at)}</span>
                      {event.directory && <span>directory</span>}
                    </div>
                  </div>
                </li>
              ))}
            </ol>
          )}
        </section>
      </main>
    </div>
  );
}

function Metric({ icon, label, value, wide = false }) {
  return (
    <div className={wide ? "metric is-wide" : "metric"}>
      <span className="metric-icon" aria-hidden="true">{icon}</span>
      <span className="metric-label">{label}</span>
      <strong title={value}>{value}</strong>
    </div>
  );
}

function formatDate(value) {
  if (!value) {
    return "unknown";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "unknown";
  }
  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  }).format(date);
}

function opClass(op = "") {
  if (op.includes("create")) return "create";
  if (op.includes("write")) return "write";
  if (op.includes("remove")) return "remove";
  if (op.includes("rename")) return "rename";
  if (op.includes("error")) return "error";
  return "other";
}

createRoot(document.getElementById("root")).render(<App />);
