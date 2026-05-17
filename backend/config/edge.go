package config

// EdgeConfig 边缘节点服务配置
type EdgeConfig struct {
	Host          string            `mapstructure:"host" default:"0.0.0.0"`
	Port          int               `mapstructure:"port" default:"8082"`
	Name          string            `mapstructure:"name" default:"edge-1"`
	Region        string            `mapstructure:"region" default:"default"`
	PublicURL     string            `mapstructure:"public_url" default:"http://localhost:8082"`
	Manager       EdgeManagerConfig `mapstructure:"manager"`
	OriginBaseURL string            `mapstructure:"origin_base_url" default:"http://origin:80"`
	Cache         EdgeCacheConfig   `mapstructure:"cache"`
	NodeID        uint
}

// EdgeManagerConfig 边缘节点连接 Manager 的配置
type EdgeManagerConfig struct {
	URL    string `mapstructure:"url" default:"http://localhost:8080"`
	Secret string `mapstructure:"secret" default:"veer-edge-secret"`
}

// EdgeCacheConfig 边缘节点缓存配置
type EdgeCacheConfig struct {
	TTLSeconds int                 `mapstructure:"ttl_seconds" default:"300"`
	MaxSizeMB  int                 `mapstructure:"max_size_mb" default:"512"`
	MaxL1MB    int                 `mapstructure:"max_l1_mb" default:"4096"`
	Disk       EdgeDiskCacheConfig `mapstructure:"disk"`
}

// EdgeDiskCacheConfig 磁盘缓存层配置
type EdgeDiskCacheConfig struct {
	Enabled         bool                     `mapstructure:"enabled" default:"false"`
	Path            string                   `mapstructure:"path" default:"./cache"`
	MaxSizeGB       int                      `mapstructure:"max_size_gb" default:"500"`
	SegmentSizeMB   int                      `mapstructure:"segment_size_mb" default:"512"`
	WriteBufferKB   int                      `mapstructure:"write_buffer_kb" default:"4096"`
	FlushIntervalMS int                      `mapstructure:"flush_interval_ms" default:"100"`
	Debug           bool                     `mapstructure:"debug" default:"false"`
	Compaction      EdgeDiskCompactionConfig `mapstructure:"compaction"`
	Index           EdgeDiskIndexConfig      `mapstructure:"index"`
}

// EdgeDiskCompactionConfig Compaction 配置
type EdgeDiskCompactionConfig struct {
	Enabled         bool    `mapstructure:"enabled" default:"true"`
	Watermark       float64 `mapstructure:"watermark" default:"0.85"`
	IntervalMinutes int     `mapstructure:"interval_minutes" default:"30"`
	MaxSegments     int     `mapstructure:"max_segments" default:"200"`
}

// EdgeDiskIndexConfig 索引配置
type EdgeDiskIndexConfig struct {
	BloomBitsPerEntry int `mapstructure:"bloom_bits_per_entry" default:"16"`
	SparseMaxEntries  int `mapstructure:"sparse_max_entries" default:"10000000"`
}
