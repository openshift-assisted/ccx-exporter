package entity

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

type (
	ProjectedClusterEvent map[string]interface{}
	ProjectedClusterState map[string]interface{}
	ProjectedInfraEnv     map[string]interface{}
)
