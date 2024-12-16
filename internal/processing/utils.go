package processing

import (
	"errors"
	"time"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
)

func ExtractEventTime(event entity.Event) (time.Time, error) {
	return time.Time{}, errors.New("not implemented")
}
