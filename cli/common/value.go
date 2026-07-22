package common

import (
	"time"

	"github.com/gclaussn/go-bpmn/engine"
)

// ISO8601Duration is a flag value for a ISO 8601 duration.
type ISO8601Duration engine.ISO8601Duration

func (v *ISO8601Duration) Set(s string) error {
	d, err := engine.NewISO8601Duration(s)
	if err != nil {
		return err
	}

	*v = ISO8601Duration(d)
	return nil
}

func (v ISO8601Duration) String() string {
	return engine.ISO8601Duration(v).String()
}

func (v ISO8601Duration) Type() string {
	return "iso8601Duration"
}

// Partition is a flag value for an engine partition.
type Partition engine.Partition

func (v *Partition) Set(s string) error {
	p, err := engine.NewPartition(s)
	if err != nil {
		return err
	}

	*v = Partition(p)
	return nil
}

func (v Partition) String() string {
	return engine.Partition(v).String()
}

func (v Partition) Type() string {
	return "partition"
}

// Time is a flag value for a RFC 3339 formatted timestamp.
type Time time.Time

func (v *Time) Set(s string) error {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}

	*v = Time(t)
	return nil
}

func (v Time) String() string {
	return time.Time(v).Format(time.RFC3339)
}

func (v Time) Time() *time.Time {
	t := time.Time(v)
	if t.IsZero() {
		return nil
	}
	return &t
}

func (v Time) Type() string {
	return "time"
}
