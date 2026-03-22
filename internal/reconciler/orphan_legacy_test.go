package reconciler

import (
	"context"
	"log/slog"

	"gitlab.bluewillows.net/root/dnsweaver/pkg/provider"
)

// =============================================================================
// Legacy delete helpers — retained for test coverage of cache-based and
// ownership-based deletion logic. Not called from production code paths;
// the active orphan cleanup uses strategy-specific methods instead.
// =============================================================================

// deleteFromCache removes DNS records using the cache to determine actual record types.
// This is used during orphan cleanup when ownership tracking is disabled.
// Renamed from deleteRecordFromCache for clarity.
func (r *Reconciler) deleteFromCache(ctx context.Context, hostname string, cache *recordCache) []Action {
	var actions []Action

	matchingProviders := r.providers.MatchingProviders(hostname)

	for _, inst := range matchingProviders {
		if r.isDryRun() {
			action := Action{
				Type:       ActionDelete,
				Provider:   inst.Name(),
				Hostname:   hostname,
				RecordType: string(inst.RecordType),
				Target:     inst.Target,
				Status:     StatusSuccess,
			}
			r.logger.Info("would delete record (dry-run)",
				slog.String("hostname", hostname),
				slog.String("provider", inst.Name()),
			)
			actions = append(actions, action)
			continue
		}

		// Get actual records from cache to know what types to delete
		var recordsToDelete []provider.Record
		if cache != nil {
			cachedRecords, ok := cache.getAllRecordsForHostname(inst.Name(), hostname)
			if ok && len(cachedRecords) > 0 {
				recordsToDelete = cachedRecords
			}
		}

		// If no cached records found, fall back to querying the provider
		if len(recordsToDelete) == 0 {
			allRecords, err := inst.Provider.List(ctx)
			if err != nil {
				r.logger.Warn("failed to list records for deletion",
					slog.String("hostname", hostname),
					slog.String("provider", inst.Name()),
					slog.String("error", err.Error()),
				)
				action := Action{
					Type:       ActionDelete,
					Provider:   inst.Name(),
					Hostname:   hostname,
					RecordType: string(inst.RecordType),
					Target:     inst.Target,
					Status:     StatusFailed,
					Error:      "failed to list records: " + err.Error(),
				}
				actions = append(actions, action)
				continue
			}
			for _, rec := range allRecords {
				if rec.Hostname == hostname {
					switch rec.Type {
					case provider.RecordTypeA, provider.RecordTypeAAAA, provider.RecordTypeCNAME, provider.RecordTypeSRV:
						recordsToDelete = append(recordsToDelete, rec)
					case provider.RecordTypeTXT:
						// Skip TXT records (ownership markers)
					}
				}
			}
		}

		// Delete each record found
		for _, record := range recordsToDelete {
			action := Action{
				Type:       ActionDelete,
				Provider:   inst.Name(),
				Hostname:   hostname,
				RecordType: string(record.Type),
				Target:     record.Target,
			}

			var err error
			if record.Type == provider.RecordTypeSRV {
				err = inst.DeleteSRVRecord(ctx, hostname, record.Target, record.SRV)
			} else {
				err = inst.DeleteRecordByTarget(ctx, hostname, record.Type, record.Target)
			}

			if err != nil {
				action.Status = StatusFailed
				action.Error = err.Error()
				r.logger.Error("failed to delete record",
					slog.String("hostname", hostname),
					slog.String("provider", inst.Name()),
					slog.String("type", string(record.Type)),
					slog.String("error", err.Error()),
				)
			} else {
				action.Status = StatusSuccess
				r.logger.Info("deleted record",
					slog.String("hostname", hostname),
					slog.String("provider", inst.Name()),
					slog.String("type", string(record.Type)),
					slog.String("target", record.Target),
				)
			}
			actions = append(actions, action)
		}
	}

	return actions
}

// deleteWithOwnership removes DNS records only if we own them (have ownership TXT record).
// This prevents deletion of manually-created DNS records during orphan cleanup.
// It uses the cache to determine actual record types (A, AAAA, SRV, etc.) to delete.
// Renamed from deleteRecordWithOwnershipCheck for clarity.
func (r *Reconciler) deleteWithOwnership(ctx context.Context, hostname string, cache *recordCache) []Action {
	var actions []Action

	matchingProviders := r.providers.MatchingProviders(hostname)

	for _, inst := range matchingProviders {
		if r.isDryRun() {
			action := Action{
				Type:       ActionDelete,
				Provider:   inst.Name(),
				Hostname:   hostname,
				RecordType: string(inst.RecordType),
				Target:     inst.Target,
				Status:     StatusSuccess,
			}
			r.logger.Info("would delete record if owned (dry-run)",
				slog.String("hostname", hostname),
				slog.String("provider", inst.Name()),
			)
			actions = append(actions, action)
			continue
		}

		// Check if we own this record (using cache if available)
		var hasOwnership bool
		if cache != nil {
			hasOwnership = cache.hasOwnershipRecord(inst.Name(), hostname, r.config.InstanceID)
		} else {
			var err error
			hasOwnership, err = inst.HasOwnershipRecord(ctx, hostname)
			if err != nil {
				r.logger.Warn("failed to check ownership record, skipping deletion",
					slog.String("hostname", hostname),
					slog.String("provider", inst.Name()),
					slog.String("error", err.Error()),
				)
				action := Action{
					Type:       ActionSkip,
					Provider:   inst.Name(),
					Hostname:   hostname,
					RecordType: string(inst.RecordType),
					Target:     inst.Target,
					Status:     StatusSkipped,
					Error:      "failed to check ownership: " + err.Error(),
				}
				actions = append(actions, action)
				continue
			}
		}

		if !hasOwnership {
			r.logger.Info("skipping orphan deletion - no ownership record (manually created?)",
				slog.String("hostname", hostname),
				slog.String("provider", inst.Name()),
			)
			action := Action{
				Type:       ActionSkip,
				Provider:   inst.Name(),
				Hostname:   hostname,
				RecordType: string(inst.RecordType),
				Target:     inst.Target,
				Status:     StatusSkipped,
				Error:      "no ownership record - may be manually created",
			}
			actions = append(actions, action)
			continue
		}

		// We own this record - get actual records from cache to know what types to delete
		var recordsToDelete []provider.Record
		if cache != nil {
			cachedRecords, ok := cache.getAllRecordsForHostname(inst.Name(), hostname)
			if ok && len(cachedRecords) > 0 {
				recordsToDelete = cachedRecords
			}
		}

		// If no cached records found, fall back to querying the provider
		if len(recordsToDelete) == 0 {
			allRecords, err := inst.Provider.List(ctx)
			if err != nil {
				r.logger.Warn("failed to list records for deletion",
					slog.String("hostname", hostname),
					slog.String("provider", inst.Name()),
					slog.String("error", err.Error()),
				)
				action := Action{
					Type:       ActionDelete,
					Provider:   inst.Name(),
					Hostname:   hostname,
					RecordType: string(inst.RecordType),
					Target:     inst.Target,
					Status:     StatusFailed,
					Error:      "failed to list records: " + err.Error(),
				}
				actions = append(actions, action)
				continue
			}
			for _, rec := range allRecords {
				if rec.Hostname == hostname {
					switch rec.Type {
					case provider.RecordTypeA, provider.RecordTypeAAAA, provider.RecordTypeCNAME, provider.RecordTypeSRV:
						recordsToDelete = append(recordsToDelete, rec)
					case provider.RecordTypeTXT:
						// Skip TXT records (ownership markers)
					}
				}
			}
		}

		// Delete each record found
		for _, record := range recordsToDelete {
			action := Action{
				Type:       ActionDelete,
				Provider:   inst.Name(),
				Hostname:   hostname,
				RecordType: string(record.Type),
				Target:     record.Target,
			}

			var err error
			if record.Type == provider.RecordTypeSRV {
				err = inst.DeleteSRVRecord(ctx, hostname, record.Target, record.SRV)
			} else {
				err = inst.DeleteRecordByTarget(ctx, hostname, record.Type, record.Target)
			}

			if err != nil {
				action.Status = StatusFailed
				action.Error = err.Error()
				r.logger.Error("failed to delete owned record",
					slog.String("hostname", hostname),
					slog.String("provider", inst.Name()),
					slog.String("type", string(record.Type)),
					slog.String("error", err.Error()),
				)
			} else {
				action.Status = StatusSuccess
				r.logger.Info("deleted owned record",
					slog.String("hostname", hostname),
					slog.String("provider", inst.Name()),
					slog.String("type", string(record.Type)),
					slog.String("target", record.Target),
				)
			}
			actions = append(actions, action)
		}

		// Delete ownership TXT record
		if ownerErr := inst.DeleteOwnershipRecord(ctx, hostname); ownerErr != nil {
			r.logger.Warn("failed to delete ownership record",
				slog.String("hostname", hostname),
				slog.String("provider", inst.Name()),
				slog.String("error", ownerErr.Error()),
			)
		} else {
			r.logger.Debug("deleted ownership record",
				slog.String("hostname", hostname),
				slog.String("provider", inst.Name()),
			)
		}
	}

	return actions
}
