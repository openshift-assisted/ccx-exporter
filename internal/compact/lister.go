package compact

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift-assisted/ccx-exporter/internal/domain/entity"
)

// Only yesterday

type OnlyYesterdayLister struct{}

func (lister OnlyYesterdayLister) ListTasks(ctx context.Context) ([]Task, error) {
	ret := make([]Task, 0)

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)

	ret = append(ret, Task{Date: yesterday, Type: entity.InfraEnvType})
	ret = append(ret, Task{Date: yesterday, Type: entity.ClusterStateType})
	ret = append(ret, Task{Date: yesterday, Type: entity.ClusterEventType})

	return ret, nil
}

// Since a given a date (except today)

type SinceDateLister struct {
	since time.Time
}

func NewSinceDateLister(since time.Time) SinceDateLister {
	return SinceDateLister{since: since}
}

func (lister SinceDateLister) ListTasks(ctx context.Context) ([]Task, error) {
	ret := make([]Task, 0)

	last := lister.lastDate()

	if last.Before(lister.since) {
		return nil, fmt.Errorf("invalid date, since (%v) is after the last date (%v)", lister.since, last)
	}

	toAppend := lister.since

	for last.After(toAppend) || last.Equal(toAppend) {
		ret = append(ret, Task{Date: toAppend, Type: entity.InfraEnvType})
		ret = append(ret, Task{Date: toAppend, Type: entity.ClusterStateType})
		ret = append(ret, Task{Date: toAppend, Type: entity.ClusterEventType})

		toAppend = toAppend.AddDate(0, 0, 1)
	}

	return ret, nil
}

func (lister SinceDateLister) lastDate() time.Time {
	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)

	return time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, time.UTC)
}
