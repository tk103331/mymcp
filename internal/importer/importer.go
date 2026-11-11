// Package importer brings servers from existing client configurations into
// the gateway registry, leaving MyMCP-managed entries behind.
package importer

import (
	"context"
	"fmt"

	"github.com/tk103331/mymcp/internal/configsync"
	"github.com/tk103331/mymcp/internal/store"
)

// Result is one server seen during import.
type Result struct {
	Name   string `json:"name"`
	Client string `json:"client"`
	Status string `json:"status"` // imported, existing, duplicate, invalid
	Error  string `json:"error,omitempty"`
}

// Stats summarizes one import.
type Stats struct {
	Imported  int      `json:"imported"`
	Existing  int      `json:"existing"`
	Duplicate int      `json:"duplicate"`
	Invalid   int      `json:"invalid"`
	Results   []Result `json:"results"`
}

// Import reads one client or every client and adds new servers to profile.
// dry stops before writing.
func Import(ctx context.Context, db *store.DB, sync *configsync.Manager, profile, client string, dry bool) (*Stats, error) {
	if profile == "" {
		profile = "default"
	}
	if _, err := db.Profile(ctx, profile); err != nil {
		if _, err := db.UpsertProfile(ctx, &store.Profile{Name: profile}); err != nil {
			return nil, err
		}
	}
	var servers []configsync.ImportedServer
	var err error
	if client != "" {
		servers, err = sync.Import(client)
	} else {
		servers, err = sync.ImportAll()
	}
	if err != nil {
		return nil, err
	}
	stats := &Stats{}
	seen := map[string]bool{}
	for _, incoming := range servers {
		result := Result{Name: incoming.Name, Client: incoming.Client}
		if seen[incoming.Name] {
			result.Status = "duplicate"
			stats.Duplicate++
			stats.Results = append(stats.Results, result)
			continue
		}
		seen[incoming.Name] = true
		if existing, err := db.GetServer(ctx, incoming.Name); err == nil {
			if !dry {
				if err := db.AttachServer(ctx, profile, existing.Name, true); err != nil {
					return nil, fmt.Errorf("%s: %w", incoming.Name, err)
				}
			}
			result.Status = "existing"
			stats.Existing++
			stats.Results = append(stats.Results, result)
			continue
		}
		if dry {
			result.Status = "imported"
			stats.Imported++
			stats.Results = append(stats.Results, result)
			continue
		}
		server := &store.Server{
			Name: incoming.Name, Transport: incoming.Transport,
			Command: incoming.Command, Args: incoming.Args, URL: incoming.URL,
			Env: incoming.Env, Headers: incoming.Headers,
			Enabled: true, IdleSeconds: 600,
		}
		if _, err := db.UpsertServer(ctx, server); err != nil {
			result.Status = "invalid"
			result.Error = err.Error()
			stats.Invalid++
			stats.Results = append(stats.Results, result)
			continue
		}
		if err := db.AttachServer(ctx, profile, incoming.Name, true); err != nil {
			return nil, fmt.Errorf("%s: %w", incoming.Name, err)
		}
		result.Status = "imported"
		stats.Imported++
		stats.Results = append(stats.Results, result)
	}
	return stats, nil
}
