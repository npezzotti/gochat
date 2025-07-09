package stats

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStatsUpdater(t *testing.T) {
	mux := http.NewServeMux()
	su := NewStatsUpdater(mux)
	assert.NotNil(t, su, "expected StatsUpdater to be non-nil")
	assert.NotNil(t, su.updateChan, "expected updateChan to be initialized")
	handler, pattern := mux.Handler(&http.Request{URL: &url.URL{Path: "/debug/vars"}, Method: http.MethodGet})
	assert.NotNil(t, handler, "expected handler for /debug/vars to be set")
	assert.Equal(t, "GET /debug/vars", pattern, "expected handler to be registered for GET method on /debug/vars")
}
