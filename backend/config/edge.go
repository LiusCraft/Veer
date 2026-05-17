package config

import (
	"github.com/spf13/viper"
)

// EdgeManagerConfig 边缘节点连接 Manager 的配置
type EdgeManagerConfig struct {
	URL    string `mapstructure:"url" default:"http://localhost:8080"`
	Secret string `mapstructure:"secret" default:"veer-edge-secret"`
}

// EdgeDiskIndexConfig 索引配置
type EdgeDiskIndexConfig struct {
	BloomBitsPerEntry int `mapstructure:"bloom_bits_per_entry" default:"16"`
	SparseMaxEntries  int `mapstructure:"sparse_max_entries" default:"10000000"`
}

// EdgeDiskCompactionConfig Compaction 配置
type EdgeDiskCompactionConfig struct {
	Enabled         bool    `mapstructure:"enabled" default:"true"`
	Watermark       float64 `mapstructure:"watermark" default:"0.85"`
	IntervalMinutes int     `mapstructure:"interval_minutes" default:"30"`
	MaxSegments     int     `mapstructure:"max_segments" default:"200"`
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

// EdgeCacheConfig 边缘节点缓存配置
type EdgeCacheConfig struct {
	TTLSeconds int                 `mapstructure:"ttl_seconds" default:"300"`
	MaxSizeMB  int                 `mapstructure:"max_size_mb" default:"512"`
	MaxL1MB    int                 `mapstructure:"max_l1_mb" default:"4096"`
	Disk       EdgeDiskCacheConfig `mapstructure:"disk"`
}

// EdgeConfig 边缘节点服务专有配置
type EdgeConfig struct {
	Service       ServiceConfig     `mapstructure:"service"`
	Name          string            `mapstructure:"name" default:"edge-1"`
	Region        string            `mapstructure:"region" default:"default"`
	PublicURL     string            `mapstructure:"public_url" default:"http://localhost:8082"`
	Manager       EdgeManagerConfig `mapstructure:"manager"`
	OriginBaseURL string            `mapstructure:"origin_base_url" default:"http://origin:80"`
	Cache         EdgeCacheConfig   `mapstructure:"cache"`
	NodeID        uint
}

// LoadEdgeConfig 加载 Edge 配置（config-edge.yaml）
func LoadEdgeConfig() (*EdgeConfig, error) {
	viper.SetConfigName("config-edge")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.SetEnvPrefix("CDNC")
	viper.AutomaticEnv()

	bindEnv(
		"service.host", "CDNC_SERVICE_HOST",
		"service.port", "CDNC_SERVICE_PORT",
		"name", "CDNC_EDGE_NAME",
		"region", "CDNC_EDGE_REGION",
		"public_url", "CDNC_EDGE_PUBLIC_URL",
		"manager.url", "CDNC_EDGE_MANAGER_URL",
		"manager.secret", "CDNC_EDGE_MANAGER_SECRET",
		"origin_base_url", "CDNC_EDGE_ORIGIN_BASE_URL",
		"cache.ttl_seconds", "CDNC_EDGE_CACHE_TTL_SECONDS",
		"cache.max_size_mb", "CDNC_EDGE_CACHE_MAX_SIZE_MB",
		"cache.max_l1_mb", "CDNC_EDGE_CACHE_MAX_L1_MB",
		"cache.disk.enabled", "CDNC_EDGE_CACHE_DISK_ENABLED",
		"cache.disk.path", "CDNC_EDGE_CACHE_DISK_PATH",
		"cache.disk.max_size_gb", "CDNC_EDGE_CACHE_DISK_MAX_SIZE_GB",
		"cache.disk.segment_size_mb", "CDNC_EDGE_CACHE_DISK_SEGMENT_SIZE_MB",
		"cache.disk.write_buffer_kb", "CDNC_EDGE_CACHE_DISK_WRITE_BUFFER_KB",
		"cache.disk.flush_interval_ms", "CDNC_EDGE_CACHE_DISK_FLUSH_INTERVAL_MS",
		"cache.disk.debug", "CDNC_EDGE_CACHE_DISK_DEBUG",
		"cache.disk.compaction.enabled", "CDNC_EDGE_CACHE_DISK_COMPACTION_ENABLED",
		"cache.disk.compaction.watermark", "CDNC_EDGE_CACHE_DISK_COMPACTION_WATERMARK",
		"cache.disk.compaction.interval_minutes", "CDNC_EDGE_CACHE_DISK_COMPACTION_INTERVAL_MINUTES",
		"cache.disk.compaction.max_segments", "CDNC_EDGE_CACHE_DISK_COMPACTION_MAX_SEGMENTS",
		"cache.disk.index.bloom_bits_per_entry", "CDNC_EDGE_CACHE_DISK_INDEX_BLOOM_BITS_PER_ENTRY",
		"cache.disk.index.sparse_max_entries", "CDNC_EDGE_CACHE_DISK_INDEX_SPARSE_MAX_ENTRIES",
	)

	setEdgeDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg EdgeConfig
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setEdgeDefaults() {
	viper.SetDefault("service.host", "0.0.0.0")
	viper.SetDefault("service.port", 8082)
	viper.SetDefault("name", "edge-1")
	viper.SetDefault("region", "default")
	viper.SetDefault("public_url", "http://localhost:8082")
	viper.SetDefault("manager.url", "")
	viper.SetDefault("manager.secret", "veer-edge-secret")
	viper.SetDefault("origin_base_url", "")
	viper.SetDefault("cache.ttl_seconds", 300)
	viper.SetDefault("cache.max_size_mb", 512)
	viper.SetDefault("cache.max_l1_mb", 4096)
	viper.SetDefault("cache.disk.enabled", false)
	viper.SetDefault("cache.disk.path", "./cache")
	viper.SetDefault("cache.disk.max_size_gb", 500)
	viper.SetDefault("cache.disk.segment_size_mb", 512)
	viper.SetDefault("cache.disk.write_buffer_kb", 4096)
	viper.SetDefault("cache.disk.flush_interval_ms", 100)
	viper.SetDefault("cache.disk.debug", false)
	viper.SetDefault("cache.disk.compaction.enabled", true)
	viper.SetDefault("cache.disk.compaction.watermark", 0.85)
	viper.SetDefault("cache.disk.compaction.interval_minutes", 30)
	viper.SetDefault("cache.disk.compaction.max_segments", 200)
	viper.SetDefault("cache.disk.index.bloom_bits_per_entry", 16)
	viper.SetDefault("cache.disk.index.sparse_max_entries", 10000000)
}
