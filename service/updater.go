// File: service/updater.go
// Provides core self-update logic using go-selfupdate.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/creativeprojects/go-selfupdate"
)

const (
	gllmOwner = "activebook"
	gllmRepo  = "gllm"

	// UpdateCheckTimeout is the maximum time budget for a remote version check.
	UpdateCheckTimeout = 10 * time.Second
)

// ReleaseInfo carries the result of a version check.
type ReleaseInfo struct {
	Version   string
	AssetURL  string
	AssetName string
	Newer     bool
}

// CheckLatest queries GitHub Releases for the latest version.
// Returns (release, error). isNewer is true when remote > currentVersion.
func CheckLatest(currentVersion string) (*ReleaseInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), UpdateCheckTimeout)
	defer cancel()

	latest, found, err := selfupdate.DetectLatest(ctx, selfupdate.ParseSlug(gllmOwner+"/"+gllmRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to detect latest version: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("no release found on GitHub for %s/%s", gllmOwner, gllmRepo)
	}

	if currentVersion == "dev" || currentVersion == "" {
		// Local development build, always assume remote is newer to allow testing updates
		return &ReleaseInfo{
			Version:   latest.Version(),
			AssetURL:  latest.AssetURL,
			AssetName: latest.AssetName,
			Newer:     true,
		}, nil
	}

	_, parseErr := semver.NewVersion(currentVersion)
	if parseErr != nil {
		// Not a valid semantic version, avoid the panic in LessOrEqual
		return &ReleaseInfo{
			Version:   latest.Version(),
			AssetURL:  latest.AssetURL,
			AssetName: latest.AssetName,
			Newer:     true,
		}, nil
	}

	if latest.LessOrEqual(currentVersion) {
		return &ReleaseInfo{Version: latest.Version(), Newer: false}, nil
	}

	return &ReleaseInfo{
		Version:   latest.Version(),
		AssetURL:  latest.AssetURL,
		AssetName: latest.AssetName,
		Newer:     true,
	}, nil
}

// ApplyUpdate downloads and replaces the running binary in place.
// The process must be restarted for the new version to take effect.
func ApplyUpdate(release *ReleaseInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	exe, err := selfupdate.ExecutablePath()
	if err != nil {
		return fmt.Errorf("could not locate current executable: %w", err)
	}

	if err := selfupdate.UpdateTo(ctx, release.AssetURL, release.AssetName, exe); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	return nil
}
