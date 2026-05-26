package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	DiscordCommandsTotal = newCounterVec("discord_commands_total", "Total number of discord commands recieved, labelled by command name", "command")
	PollerTicksTotal     = newCounter("poller_ticks_total", "Total number of PandaScore poller ticks completed.")
	PollerErrorsTotal    = newCounter("poller_errors_total", "Total number of errors from the PandaScore poller")
	MatchUpdatesTotal    = newCounter("match_updates_total", "Total number of match updates recieved")
	MongoOpsTotal        = newCounterVec("mongodb_operations_total", "Total number of calls made to mongodb", "operation")

	LeaderboardDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "leaderboard_generation_duration_seconds",
			Help:    "Time taken to regenerate the leaderboard, in seconds.",
			Buckets: prometheus.DefBuckets,
		},
	)
	ImageRenderDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "image_render_duration_seconds",
			Help:    "Time taken for Chromium to render the results bracket image, in seconds",
			Buckets: []float64{0.5, 1, 2, 5, 10, 20, 30},
		},
	)
)

// init registers the prometheus methods
func init() {
	prometheus.MustRegister(
		DiscordCommandsTotal,
		PollerTicksTotal,
		PollerErrorsTotal,
		MatchUpdatesTotal,
		LeaderboardDuration,
		ImageRenderDuration,
		MongoOpsTotal,
	)
}

// newCounter is a wrapper for prometheus.NewCounter that reduces the inline boilerplate
func newCounter(name, help string) prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{
		Name: name,
		Help: help,
	})
}

// newCounterVec is a wrapper for prometheus.CounterVec that reduces the inline boilerplate
func newCounterVec(name, help string, labels ...string) *prometheus.CounterVec {
	return prometheus.NewCounterVec(prometheus.CounterOpts{Name: name, Help: help}, labels)
}
