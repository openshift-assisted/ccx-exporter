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

type ProjectionMeta struct {
	ID        string
	Timestamp time.Time
}

type Projection struct {
	Meta    ProjectionMeta
	Payload []byte
}

type (
	ProjectedClusterEvent Projection
	ProjectedClusterState Projection
	ProjectedInfraEnv     Projection
)

type Type string

const (
	ClusterEventType Type = "event"
	ClusterStateType Type = "cluster"
	InfraEnvType     Type = "infraenv"
)
