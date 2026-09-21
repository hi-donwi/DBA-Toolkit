package report

import "github.com/hi-donwi/DBA-Toolkit/internal/model"

// Command data wrappers give each command's report a stable `data` JSON shape
// shared by the terminal renderers and machine consumers.

type HealthData struct {
	Connectivity model.Connectivity `json:"connectivity"`
	Health       model.Health       `json:"health,omitempty"`
}

func (d HealthData) connectivity() model.Connectivity { return d.Connectivity }
func (d HealthData) health() model.Health             { return d.Health }

type DiagnoseData struct {
	Connectivity model.Connectivity   `json:"connectivity"`
	Health       model.Health         `json:"health,omitempty"`
	LongQueries  []model.Session      `json:"long_queries,omitempty"`
	Locks        []model.BlockingPair `json:"blocking_pairs,omitempty"`
	Replication  *model.Replication   `json:"replication,omitempty"`
}

func (d DiagnoseData) connectivity() model.Connectivity { return d.Connectivity }
func (d DiagnoseData) health() model.Health             { return d.Health }

type SessionsData struct {
	Sessions []model.Session `json:"sessions"`
}

type LocksData struct {
	BlockingPairs []model.BlockingPair `json:"blocking_pairs"`
}

type ReplicationData struct {
	Replication model.Replication `json:"replication"`
}

type DatabasesData struct {
	Databases []model.DatabaseInfo `json:"databases"`
}

type ConfigData struct {
	Settings []model.ConfigSetting `json:"settings"`
}
