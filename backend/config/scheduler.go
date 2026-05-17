package config

// SchedulerConfig 调度服务配置
type SchedulerConfig struct {
	Host            string `mapstructure:"host" default:"0.0.0.0"`
	Port            int    `mapstructure:"port" default:"8081"`
	RefreshInterval int    `mapstructure:"refresh_interval" default:"10"`
}
