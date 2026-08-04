package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"pickems-bot/app"
	"pickems-bot/sources"
	"pickems-bot/store"
)

// vrsStandingsDateLayout matches the yyyy_mm_dd label in a VRS snapshot header.
const vrsStandingsDateLayout = "2006_01_02"

// VRSSync periodically refreshes the VRS world rankings from the public GitHub
// snapshot. It hits GitHub, not PandaScore, so it does not use the PandaScore
// rate limiter. Standings change slowly and the store no-ops when the snapshot
// date is unchanged, so a modest interval is cheap.
type VRSSync struct {
	app      *app.App
	interval time.Duration
	log      *slog.Logger
}

// NewVRSSync constructs a VRSSync. interval <= 0 defaults to one hour; log may
// be nil (falls back to the slog default).
func NewVRSSync(a *app.App, interval time.Duration, log *slog.Logger) *VRSSync {
	if interval <= 0 {
		interval = time.Hour
	}
	var jobLog *slog.Logger
	if log != nil {
		jobLog = log.With("component", "ingest.vrs")
	}
	return &VRSSync{app: a, interval: interval, log: jobLog}
}

func (s *VRSSync) logger() *slog.Logger {
	if s.log == nil {
		return slog.Default()
	}
	return s.log
}

// Start runs an immediate sync, then repeats every interval until ctx is cancelled.
func (s *VRSSync) Start(ctx context.Context) {
	s.runOnce(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.logger().Info("VRS sync stopping", "reason", ctx.Err())
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

// runOnce fetches, parses, and syncs the latest VRS snapshot. Errors are logged
// and swallowed so the job retries on the next tick.
func (s *VRSSync) runOnce(ctx context.Context) {
	raw, err := sources.FetchLatestVRSStandings()
	if err != nil {
		s.logger().Warn("VRS fetch failed, will retry next tick", "error", err)
		return
	}

	parsed := sources.ParseVRSStandings(raw)
	if len(parsed) == 0 {
		s.logger().Warn("VRS parse produced no standings, skipping sync")
		return
	}

	entries, err := toStoreEntries(parsed)
	if err != nil {
		s.logger().Warn("VRS date parse failed, skipping sync", "error", err)
		return
	}

	synced, err := s.app.Store.SyncStandings(ctx, entries)
	if err != nil {
		s.logger().Warn("VRS sync failed", "error", err)
		return
	}
	if synced {
		s.logger().Info("VRS standings synced", "count", len(entries))
	} else {
		s.logger().Info("VRS standings already up to date, skipping", "date", parsed[0].StandingsDate)
	}
}

// toStoreEntries converts parsed standings into store entries, parsing the raw
// yyyy_mm_dd label into a real date once (all rows share it).
func toStoreEntries(parsed []sources.VRSStanding) ([]store.VRSEntry, error) {
	date, err := time.Parse(vrsStandingsDateLayout, parsed[0].StandingsDate)
	if err != nil {
		return nil, fmt.Errorf("parse standings date %q: %w", parsed[0].StandingsDate, err)
	}

	entries := make([]store.VRSEntry, 0, len(parsed))
	for _, p := range parsed {
		entries = append(entries, store.VRSEntry{
			Standing:      p.Standing,
			Points:        p.Points,
			TeamName:      p.TeamName,
			Roster:        p.Roster,
			StandingsDate: date,
		})
	}
	return entries, nil
}
