package metrics

// CounterOpts configures a Counter or CounterVec metric.
type CounterOpts struct {
	Subsystem string
	Name      string
	Help      string
}

// HistogramOpts configures a Histogram or HistogramVec metric.
type HistogramOpts struct {
	Subsystem string
	Name      string
	Help      string
	Buckets   []float64
}

// GaugeOpts configures a Gauge or GaugeVec metric.
type GaugeOpts struct {
	Subsystem string
	Name      string
	Help      string
}
