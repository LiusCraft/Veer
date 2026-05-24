package models

import "time"

type RedirectRule struct {
	ID          uint   `json:"id" gorm:"primarykey"`
	Name        string `json:"name" gorm:"size:128"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled" gorm:"default:true"`
	Priority    int    `json:"priority" gorm:"default:0"`

	RuleType string `json:"rule_type" gorm:"default:'domain_routing';size:32"`

	Domain        string `json:"domain" gorm:"size:253"`
	Strategy      string `json:"strategy" gorm:"default:'round-robin'"`
	NodeIDs       string `json:"node_ids"` // Deprecated: use RuleCluster.ClusterID instead
	OriginBaseURL string `json:"origin_base_url" gorm:"size:512;default:''"`
	HitCount      int64  `json:"hit_count"`

	MatchType    string `json:"match_type" gorm:"default:'prefix';size:16"`
	SourcePath   string `json:"source_path" gorm:"default:'/';size:512"`
	TargetHost   string `json:"target_host" gorm:"size:253"`
	TargetPath   string `json:"target_path" gorm:"size:512"`
	RedirectCode int    `json:"redirect_code" gorm:"default:302"`

	// Per-domain cache configuration (edge nodes consume these)
	CacheTTLSeconds      *int   `json:"cache_ttl_seconds" gorm:"default:null"`
	CacheControlOverride string `json:"cache_control_override" gorm:"size:128;default:''"`
	BypassCache          bool   `json:"bypass_cache" gorm:"default:false"`

	// ResponseHeaderRules is a JSON array of response header rewrite actions, e.g.:
	// [{"action":"set","name":"X-Frame-Options","value":"DENY"},{"action":"remove","name":"X-Powered-By"}]
	ResponseHeaderRules string `json:"response_header_rules" gorm:"type:text;default:''"`

	// RequestHeaderRules is a JSON array of request header rewrite actions (applied to origin fetch).
	// Same structure as ResponseHeaderRules.
	RequestHeaderRules string `json:"request_header_rules" gorm:"type:text;default:''"`

	// RewriteFrom / RewriteTo: prefix-based path rewrite applied before origin fetch.
	// If the request path starts with RewriteFrom, it is replaced with RewriteTo.
	RewriteFrom string `json:"rewrite_from" gorm:"size:512;default:''"`
	RewriteTo   string `json:"rewrite_to" gorm:"size:512;default:''"`

	LuaScript       string `json:"lua_script" gorm:"type:text;default:''"`
	ScriptTimeoutMs *int   `json:"script_timeout_ms" gorm:"default:null"`

	CreatedAt time.Time `json:"created_at"`
}
