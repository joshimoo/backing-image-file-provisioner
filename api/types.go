package api

type FileInfo struct {
	DiskUUID   string            `json:"diskUUID"`
	SourceType string            `json:"sourceType"`
	Parameters map[string]string `json:"parameters"`

	FileName      string `json:"fileName"`
	FilePath      string `json:"filePath"`
	State         string `json:"state"`
	Size          int64  `json:"size"`
	Progress      int    `json:"progress"`
	ProcessedSize int64  `json:"processedSize"`
	MD5Checksum   string `json:"md5Checksum"`
}
