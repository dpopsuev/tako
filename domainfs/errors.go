package domainfs

import "errors"

var (
	// ErrNilFilesystem is returned for: domainfs: nil filesystem
	ErrNilFilesystem = errors.New("domainfs: nil filesystem")

	// ErrSection is returned for: domainfs: section
	ErrSection = errors.New("domainfs: section")

	// ErrAsset is returned for: domainfs: asset
	ErrAsset = errors.New("domainfs: asset")

	// ErrRemoteCall is returned when a remote MCP tool call fails.
	ErrRemoteCall = errors.New("remote call failed")

	// ErrIsADirectory is returned for: is a directory
	ErrIsADirectory = errors.New("is a directory")
)
