package entity

import "time"

type Event struct {
	Name     string                 `json:"name"`
	Payload  map[string]interface{} `json:"payload"`
	Metadata map[string]interface{} `json:"metadata"`
}

type HostState struct {
	ClusterID string
	HostID    string
	Payload   map[string]interface{}
	Metadata  map[string]interface{}
}

type Projection struct {
	ID        string
	Timestamp time.Time
	Payload   map[string]interface{}
}

type (
	ProjectedClusterEvent Projection
	ProjectedClusterState Projection
	ProjectedInfraEnv     Projection
)
