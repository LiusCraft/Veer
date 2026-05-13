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
	NodeIDs       string `json:"node_ids"`
	OriginBaseURL string `json:"origin_base_url" gorm:"size:512;default:''"`
	HitCount      int64  `json:"hit_count"`

	MatchType    string `json:"match_type" gorm:"default:'prefix';size:16"`
	SourcePath   string `json:"source_path" gorm:"default:'/';size:512"`
	TargetHost   string `json:"target_host" gorm:"size:253"`
	TargetPath   string `json:"target_path" gorm:"size:512"`
	RedirectCode int    `json:"redirect_code" gorm:"default:302"`

	CreatedAt time.Time `json:"created_at"`
}
