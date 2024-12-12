package processingerror

import "time"

type ProcessingError struct {
	ProcessingContext ProcessingContext
	Sources           Sources
	Reason            Reason
}

type ProcessingContext struct {
	Component Component
	Time      time.Time
	Host      string
}

type Component struct {
	Branch   string
	Revision string
}

type Sources struct {
	Main       Source
	Additional []KeyValue
}

type Source struct {
	Topic     string
	Partition int32
	Offset    int64
	Payload   []byte
}

type KeyValue struct {
	Key   string
	Value []byte
}

type Reason struct {
	Category string
	Error    string
}
