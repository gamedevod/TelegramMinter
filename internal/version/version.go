// Package version provides a location to set the release versions for all
// packages to consume, without creating import cycles.
//
// This package should not import any other packages.
package version

// Version is the main version number that is being run at the moment.
var Version = "1.0.0"

// Prerelease is a pre-release marker for the version. If this is "" (empty string)
// then it means that it is a final release. Otherwise, this is a pre-release
// such as "dev" (in development), "beta", "rc1", etc.
var Prerelease = "beta"

var Production = true
