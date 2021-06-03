package types

import (
	"time"
)

const (
	DefaultPort = 8001

	HTTPTimeout = 1 * time.Minute

	FileProvisionerDirectoryName = "tmp-files"
	DiskPathInContainer          = "/data/"

	DefaultSectorSize = 512
)

type FileState string

const (
	FileStatePending    = FileState("pending")
	FileStateStarting   = FileState("starting")
	FileStateInProgress = FileState("in_progress")
	FileStateReady      = FileState("ready")
	FileStateFailed     = FileState("failed")
)

type DataSourceType string

const (
	DataSourceTypeURL    = DataSourceType("url")
	DataSourceTypeUpload = DataSourceType("upload")
)

const (
	DataSourceTypeURLParameterURL = "url"
)
