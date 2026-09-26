// Package sources fetches water temperatures from the upstream providers and
// maps them to station.Station records.
package sources

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/999mattia/SwissWaterTemps/internal/station"
)

// Fetcher is implemented by every data source.
type Fetcher interface {
	ID() string
	Name() string
	URL() string
	// Fetch returns the source's stations. It may return stations together
	// with an error, e.g. cached data when a refresh failed: the stations are
	// used and the source is still reported as failing.
	Fetch(ctx context.Context) ([]station.Station, error)
}

const userAgent = "SwissWaterTemps/2.0 (+https://github.com/999mattia/SwissWaterTemps)"

// maxBody caps upstream responses so a misbehaving source can't exhaust memory.
const maxBody = 20 << 20

// NewHTTPClient returns a client with a hard timeout for upstream requests.
func NewHTTPClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func get(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, maxBody))
}

func ptr[T any](v T) *T { return &v }
