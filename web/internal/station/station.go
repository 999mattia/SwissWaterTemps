// Package station defines the normalized water temperature record that every
// data source is mapped to.
package station

import "time"

type Kind string

const (
	Lake  Kind = "lake"
	River Kind = "river"
)

type Station struct {
	// ID is unique across all sources, e.g. "bafu-2135" or "alplakes-zug".
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	WaterBody   string  `json:"waterBody,omitempty"`
	Kind        Kind    `json:"kind"`
	Temperature float64 `json:"temperature"`
	// MeasuredAt is when the source took the measurement, if it tells us.
	MeasuredAt *time.Time `json:"measuredAt,omitempty"`
	Min24h     *float64   `json:"min24h,omitempty"`
	Max24h     *float64   `json:"max24h,omitempty"`
	Lat        *float64   `json:"lat,omitempty"`
	Lon        *float64   `json:"lon,omitempty"`
	Source     string     `json:"source"`
	// Modelled is true when the value comes from a lake model rather than a sensor.
	Modelled bool `json:"modelled,omitempty"`
	// Change24h is the temperature change compared to about 24 hours earlier.
	Change24h *float64 `json:"change24h,omitempty"`

	// Recent holds past values a source already knows (e.g. a model hindcast),
	// so history can be filled in without waiting for our own polling.
	Recent []Point `json:"-"`
	// Forecast holds future values, if the source provides them.
	Forecast []Point `json:"-"`
}

type Point struct {
	Time  time.Time `json:"t"`
	Value float64   `json:"v"`
}

// Source describes where a group of stations comes from and how fresh it is.
type Source struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	URL         string     `json:"url"`
	OK          bool       `json:"ok"`
	LastSuccess *time.Time `json:"lastSuccess,omitempty"`
	LastError   string     `json:"lastError,omitempty"`
}

type Snapshot struct {
	UpdatedAt time.Time `json:"updatedAt"`
	Sources   []Source  `json:"sources"`
	Stations  []Station `json:"stations"`
}
