package main

import (
	"context"
	"log/slog"
	"time"
)

func (s *service) applyHistoryRetention(ctx context.Context) {
	days := s.cfg.HistoryRetentionDays
	if days == 0 {
		return
	}
	now := s.now()
	last := s.state.HistoryPurgeAt
	if s.state.HistoryKeepDays == days && !last.IsZero() && !now.Before(last) && now.Sub(last) < 24*time.Hour {
		return
	}
	address, token := supervisorCoreAPI()
	count, err := purgeRegisteredEntityHistory(ctx, address, token, s.ownedEntityUniqueIDs(), days)
	if err == nil && count == 0 {
		// Discovery may still be registering the first entities. Retry next poll.
		return
	}
	if err == nil {
		previousDays := s.state.HistoryKeepDays
		s.state.HistoryPurgeAt, s.state.HistoryKeepDays = now, days
		err = s.state.save(s.dir)
		if err != nil {
			s.state.HistoryPurgeAt, s.state.HistoryKeepDays = last, previousDays
		}
	}
	if err != nil {
		if s.historyPurgeWarning != err.Error() {
			slog.Warn("Cannot apply Bridge history retention; retrying on the next poll", "reason", err)
			s.historyPurgeWarning = err.Error()
		}
		return
	}
	s.historyPurgeWarning = ""
	slog.Info("Requested Bridge entity history cleanup", "keep_days", days, "entities", count)
}
