package models

type GeoJSONFile struct {
	Type     string    `json:"type"`
	Name     string    `json:"name"`
	Features []Feature `json:"features"`
	CRS      CRS       `json:"crs"`
	Meta     Meta      `json:"meta"`
}

type CRS struct {
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties"`
}

type Feature struct {
	Type       string     `json:"type"`
	ID         int        `json:"id"`
	Geometry   Geometry   `json:"geometry"`
	Properties Properties `json:"properties"`
}

type Geometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type Properties struct {
	Label            string  `json:"label"`
	Key              string  `json:"key"`
	Icon             string  `json:"icon"`
	IconPath         string  `json:"icon_path"`
	HydroBody        string  `json:"hydro_body"`
	HydroBodyName    string  `json:"hydro_body_name"`
	LastValue        string  `json:"last_value"`
	Metric           string  `json:"metric"`
	Unit             string  `json:"unit"`
	UnitShort        string  `json:"unit_short"`
	Plot             string  `json:"plot"`
	LastMeasuredAt   string  `json:"last_measured_at"`
	Min24h           string  `json:"min_24h"`
	Max24h           string  `json:"max_24h"`
	Mean24h          string  `json:"mean_24h"`
	FailureText      *string `json:"failure_text"`
	FailureValidFrom *string `json:"failure_valid_from"`
	HydroStationID   int     `json:"hydro_station_id"`
}

type Meta struct {
	DE Localization `json:"de"`
	FR Localization `json:"fr"`
	IT Localization `json:"it"`
	EN Localization `json:"en"`
}

type Localization struct {
	Title  string              `json:"title"`
	Legend map[string]string   `json:"legend"`
	Icons  map[string][]string `json:"icons"`
}
