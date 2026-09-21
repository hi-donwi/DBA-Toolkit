package model

// Data types here are the normalized view of PostgreSQL between collectors and
// evaluators: collectors produce them from read-only SQL, evaluators consume
// them without touching the database. Durations are seconds (float64) so they
// serialize to plain, stable numbers in JSON.

// Connectivity captures the result of establishing a connection.
type Connectivity struct {
	Connected      bool    `json:"connected"`
	Error          string  `json:"error,omitempty"` // redacted before it ever reaches a report
	Version        string  `json:"version"`         // short form, e.g. "16.4"
	VersionNum     int     `json:"version_num,omitempty"`
	Database       string  `json:"database"`
	LatencySeconds float64 `json:"latency_seconds"`
}

// Health carries basic server health facts.
type Health struct {
	Version           string  `json:"version"`
	VersionNum        int     `json:"version_num,omitempty"`
	CurrentDatabase   string  `json:"current_database"`
	UptimeSeconds     float64 `json:"uptime_seconds"`
	TotalConnections  int     `json:"total_connections"`
	ActiveConnections int     `json:"active_connections"`
	MaxConnections    int     `json:"max_connections"`
	ConnectionUsage   float64 `json:"connection_usage_percent"`
	DatabaseSizeBytes int64   `json:"database_size_bytes"`
}

// Session is one row of pg_stat_activity.
type Session struct {
	PID             int     `json:"pid"`
	User            string  `json:"user"`
	Database        string  `json:"database"`
	State           string  `json:"state"`
	DurationSeconds float64 `json:"duration_seconds"`
	WaitType        string  `json:"wait_type,omitempty"`
	WaitEvent       string  `json:"wait_event,omitempty"`
	Query           string  `json:"query,omitempty"` // already truncated to QueryLimit
}

// BlockingPair records one blocked session and the session blocking it.
type BlockingPair struct {
	BlockedPID    int     `json:"blocked_pid"`
	BlockedUser   string  `json:"blocked_user"`
	BlockingPID   int     `json:"blocking_pid"`
	BlockingUser  string  `json:"blocking_user"`
	Database      string  `json:"database"`
	WaitSeconds   float64 `json:"wait_seconds"`
	BlockedState  string  `json:"blocked_state"`
	BlockingState string  `json:"blocking_state"`
	BlockedQuery  string  `json:"blocked_query,omitempty"`
	BlockingQuery string  `json:"blocking_query,omitempty"`
}

// Replica describes one connected standby, as seen from a primary.
type Replica struct {
	ApplicationName  string  `json:"application_name"`
	ClientAddr       string  `json:"client_addr,omitempty"`
	State            string  `json:"state"`
	SyncState        string  `json:"sync_state,omitempty"`
	ReplayLagSeconds float64 `json:"replay_lag_seconds"`
}

// Replication describes the server's replication role and connected standbys.
type Replication struct {
	Role              string    `json:"role"` // "primary" or "standby"
	IsInRecovery      bool      `json:"is_in_recovery"`
	ConnectedStandbys int       `json:"connected_standbys"`
	Replicas          []Replica `json:"replicas,omitempty"`
	// LocalReplayLagSeconds is only meaningful on a standby and measures how far
	// behind this server is from receiving WAL.
	LocalReplayLagSeconds float64 `json:"local_replay_lag_seconds,omitempty"`
}

// DatabaseInfo is one row of the database inventory.
type DatabaseInfo struct {
	Name        string `json:"name"`
	Owner       string `json:"owner"`
	SizeBytes   int64  `json:"size_bytes"`
	Connections int    `json:"connections"`
	SizeUnknown bool   `json:"size_unknown,omitempty"`
}

// ConfigSetting is one PostgreSQL server configuration setting.
type ConfigSetting struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Unit    string `json:"unit,omitempty"`
	Source  string `json:"source,omitempty"`
	Context string `json:"context,omitempty"`
}
