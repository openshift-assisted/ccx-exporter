package host

import "github.com/openshift-assisted/ccx-exporter/internal/domain/entity"

type State struct {
	Metadata map[string]interface{}
	Payload  map[string]interface{}
}

func mapToModels(event entity.HostState) State {
	return State{
		Metadata: event.Metadata,
		Payload:  event.Payload,
	}
}

func mapToEntity(state State) entity.HostState {
	return entity.HostState{
		Metadata: state.Metadata,
		Payload:  state.Payload,
	}
}
