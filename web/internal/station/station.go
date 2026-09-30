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
	// ID is unique across all sources, e.g. "bafu-2135" or "boot24-zurichsee".
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
	// Change24h is the temperature change compared to about 24 hours earlier.
	Change24h *float64 `json:"change24h,omitempty"`
	// Hydro is flow and level at the BAFU gauge that measures the same water, if any.
	Hydro *Hydro `json:"hydro,omitempty"`

	// HydroKey is that gauge's BAFU station key, set by the source; the store
	// attaches Hydro by it.
	HydroKey string `json:"-"`

	// Recent holds past values a source already knows (none of the current ones do),
	// so history can be filled in without waiting for our own polling.
	Recent []Point `json:"-"`
}

// Hydro is the latest reading of a BAFU discharge / water level station.
type Hydro struct {
	// Discharge in m³/s (small streams reported in l/s are converted).
	Discharge *float64 `json:"discharge,omitempty"`
	// WaterLevel in metres above sea level.
	WaterLevel *float64 `json:"waterLevel,omitempty"`
	// DangerLevel is BAFU's flood danger level, 1 (none or low) to 5 (very high).
	DangerLevel *int       `json:"dangerLevel,omitempty"`
	MeasuredAt  *time.Time `json:"measuredAt,omitempty"`
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
