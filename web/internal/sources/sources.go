// Package sources fetches water temperatures from the upstream providers and
// maps them to station.Station records.
package sources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

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
		return nil, &statusError{url: url, status: res.Status, code: res.StatusCode}
	}
	return io.ReadAll(io.LimitReader(res.Body, maxBody))
}

// partialError reports the failures of a source made of several independent
// stations or datasets. Single ones are regularly out of service (maintenance,
// winter), which must not mark the whole source as unreachable, so they are only
// logged; the source fails when none of its total parts could be read.
func partialError(source string, errs []error, total int) error {
	if len(errs) < total {
		for _, err := range errs {
			slog.Warn("fetching part of a source failed", "source", source, "error", err)
		}
		return nil
	}
	return errors.Join(errs...)
}

// statusError is a non-200 response, so callers can tell e.g. a 404 apart.
type statusError struct {
	url, status string
	code        int
}

func (e *statusError) Error() string { return fmt.Sprintf("GET %s: %s", e.url, e.status) }

func isNotFound(err error) bool {
	var se *statusError
	return errors.As(err, &se) && se.code == http.StatusNotFound
}

var numberPattern = regexp.MustCompile(`-?\d+(?:[.,]\d+)?`)

// parseTemperature extracts the first number from strings like "18.4°", "18,4 °C" or "18.4".
func parseTemperature(s string) (float64, error) {
	m := numberPattern.FindString(s)
	if m == "" {
		return 0, fmt.Errorf("no number in %q", s)
	}
	return strconv.ParseFloat(strings.Replace(m, ",", ".", 1), 64)
}

var slugStrip = regexp.MustCompile(`[^a-z0-9]+`)

// slug turns "Zürichsee (Obersee)" into "zurichsee-obersee".
func slug(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	folded, _, _ := transform.String(t, strings.ToLower(s))
	return strings.Trim(slugStrip.ReplaceAllString(folded, "-"), "-")
}

func ptr[T any](v T) *T { return &v }
