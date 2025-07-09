package stats

import (
	"encoding/json"
	"expvar"
	"net/http"
	"time"
)

type StatsProvider interface {
	Incr(name string)
	Decr(name string)
	RegisterMetric(name string)
	Run()
}

type StatsUpdater struct {
	vars       *expvar.Map
	updateChan chan *metricsUpdateReq
}

type metricsUpdateReq struct {
	name  string
	value int
}

func (su *StatsUpdater) expvarHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	expvarData := make(map[string]any)
	su.vars.Do(func(kv expvar.KeyValue) {
		var value any
		json.Unmarshal([]byte(kv.Value.String()), &value)
		expvarData[kv.Key] = value
	})

	json.NewEncoder(w).Encode(expvarData)
}

// NewStatsUpdater creates a new stats updater instance.
func NewStatsUpdater(mux *http.ServeMux) *StatsUpdater {
	su := &StatsUpdater{
		updateChan: make(chan *metricsUpdateReq, 512),
	}
	mux.Handle("GET /debug/vars", http.HandlerFunc(su.expvarHandler))
	su.vars = expvar.NewMap("gochat-stats")
	su.initializeMetrics()

	return su
}

func (su *StatsUpdater) initializeMetrics() {
	startTime := time.Now()
	su.vars.Set("Uptime", expvar.Func(func() any {
		return time.Since(startTime).Milliseconds()
	}))
}

func (su *StatsUpdater) updateMetrics() {
	for req := range su.updateChan {
		metric := su.vars.Get(req.name)
		if metric == nil {
			panic("metric not found: " + req.name)
		}

		metric.(*expvar.Int).Add(int64(req.value))
	}
}

func (su *StatsUpdater) Incr(name string) {
	su.updateChan <- &metricsUpdateReq{name: name, value: 1}
}

func (su *StatsUpdater) Decr(name string) {
	su.updateChan <- &metricsUpdateReq{name: name, value: -1}
}

func (su *StatsUpdater) RegisterMetric(name string) {
	su.vars.Set(name, expvar.NewInt(name))
}

func (su *StatsUpdater) Run() {
	go su.updateMetrics()
}

func (su *StatsUpdater) Stop() {
	close(su.updateChan)
}
