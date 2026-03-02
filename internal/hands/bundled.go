package hands

import (
	"time"
)

var bundledHands = []*HandDefinition{
	// Researcher Hand
	{
		ID:          "researcher",
		Name:        "Researcher",
		Description: "Deep autonomous researcher. Cross-references multiple sources, evaluates credibility using CRAAP criteria, generates cited reports.",
		Category:    HandCategoryContent,
		Icon:        "🔍",
		Tools: []string{
			"web_search",
			"web_scrape",
			"pdf_extract",
			"citation_generator",
		},
		Requires: []HandRequirement{
			{
				Key:             "search_provider",
				Label:           "Search Provider API Key",
				RequirementType: RequirementTypeAPIKey,
				Description:     "API key for web search (Tavily, Serper, or Google)",
			},
		},
		Settings: []HandSetting{
			{
				Key:         "research_depth",
				Label:       "Research Depth",
				Description: "How deep to research topics",
				SettingType: HandSettingTypeSelect,
				Default:     "medium",
				Options: []HandSettingOption{
					{Value: "shallow", Label: "Shallow (1-2 sources)"},
					{Value: "medium", Label: "Medium (3-5 sources)"},
					{Value: "deep", Label: "Deep (6+ sources)"},
				},
			},
			{
				Key:         "citation_style",
				Label:       "Citation Style",
				Description: "Format for citations in reports",
				SettingType: HandSettingTypeSelect,
				Default:     "apa",
				Options: []HandSettingOption{
					{Value: "apa", Label: "APA 7th Edition"},
					{Value: "mla", Label: "MLA 9th Edition"},
					{Value: "chicago", Label: "Chicago Notes & Bibliography"},
				},
			},
		},
		Agent: HandAgentConfig{
			Name:         "Research Assistant",
			Description:  "Deep research specialist with CRAAP evaluation",
			Module:       "researcher",
			Provider:     "openai",
			Model:        "gpt-4o",
			MaxTokens:    4096,
			Temperature:  0.3,
			SystemPrompt: ResearcherSystemPrompt,
		},
		Dashboard: HandDashboard{
			Metrics: []HandMetric{
				{Label: "Reports Generated", MemoryKey: "reports_count", Format: "number"},
				{Label: "Sources Checked", MemoryKey: "sources_checked", Format: "number"},
				{Label: "Avg. Research Time", MemoryKey: "avg_research_time", Format: "duration"},
			},
		},
	},

	// Lead Hand
	{
		ID:          "lead",
		Name:        "Lead",
		Description: "Daily lead generation. Discovers prospects, enriches with research, scores 0-100.",
		Category:    HandCategoryProductivity,
		Icon:        "🎯",
		Tools: []string{
			"linkedin_search",
			"company_enrich",
			"email_finder",
			"csv_export",
		},
		Requires: []HandRequirement{
			{
				Key:             "linkedin_access",
				Label:           "LinkedIn API Access",
				RequirementType: RequirementTypeAPIKey,
				Description:     "LinkedIn API key or proxy service",
			},
		},
		Settings: []HandSetting{
			{
				Key:         "icp_industry",
				Label:       "Target Industry",
				Description: "Industry to target for leads",
				SettingType: HandSettingTypeText,
				Default:     "technology",
			},
			{
				Key:         "lead_score_threshold",
				Label:       "Lead Score Threshold",
				Description: "Minimum score to include a lead (0-100)",
				SettingType: HandSettingTypeSelect,
				Default:     "70",
				Options: []HandSettingOption{
					{Value: "50", Label: "50+"},
					{Value: "70", Label: "70+"},
					{Value: "85", Label: "85+"},
					{Value: "95", Label: "95+"},
				},
			},
		},
		Agent: HandAgentConfig{
			Name:         "Lead Generator",
			Description:  "Smart prospect discovery and scoring",
			Module:       "lead",
			Provider:     "openai",
			Model:        "gpt-4o",
			MaxTokens:    2048,
			Temperature:  0.2,
			SystemPrompt: LeadSystemPrompt,
		},
		Dashboard: HandDashboard{
			Metrics: []HandMetric{
				{Label: "Leads Found", MemoryKey: "leads_found", Format: "number"},
				{Label: "Qualified Leads", MemoryKey: "qualified_leads", Format: "number"},
				{Label: "Avg. Lead Score", MemoryKey: "avg_lead_score", Format: "number"},
			},
		},
	},

	// Collector Hand
	{
		ID:          "collector",
		Name:        "Collector",
		Description: "OSINT intelligence. Monitors targets continuously with change detection and knowledge graphs.",
		Category:    HandCategoryData,
		Icon:        "📊",
		Tools: []string{
			"web_monitor",
			"rss_feed",
			"sentiment_analyzer",
			"knowledge_graph",
		},
		Requires: []HandRequirement{},
		Settings: []HandSetting{
			{
				Key:         "targets",
				Label:       "Monitoring Targets",
				Description: "Comma-separated list of targets to monitor",
				SettingType: HandSettingTypeText,
				Default:     "",
			},
			{
				Key:         "check_frequency",
				Label:       "Check Frequency",
				Description: "How often to check for updates",
				SettingType: HandSettingTypeSelect,
				Default:     "hourly",
				Options: []HandSettingOption{
					{Value: "15min", Label: "Every 15 minutes"},
					{Value: "hourly", Label: "Hourly"},
					{Value: "daily", Label: "Daily"},
				},
			},
		},
		Agent: HandAgentConfig{
			Name:         "Intelligence Collector",
			Description:  "Continuous monitoring and intelligence gathering",
			Module:       "collector",
			Provider:     "openai",
			Model:        "gpt-4o",
			MaxTokens:    3072,
			Temperature:  0.4,
			SystemPrompt: CollectorSystemPrompt,
		},
		Dashboard: HandDashboard{
			Metrics: []HandMetric{
				{Label: "Changes Detected", MemoryKey: "changes_detected", Format: "number"},
				{Label: "Alerts Sent", MemoryKey: "alerts_sent", Format: "number"},
				{Label: "Knowledge Graph Size", MemoryKey: "kg_size", Format: "number"},
			},
		},
	},

	// Predictor Hand
	{
		ID:          "predictor",
		Name:        "Predictor",
		Description: "Superforecasting engine. Collects signals, builds calibrated reasoning chains, makes predictions.",
		Category:    HandCategoryData,
		Icon:        "🔮",
		Tools: []string{
			"signal_aggregator",
			"reasoning_chain",
			"brier_scorer",
			"prediction_tracker",
		},
		Requires: []HandRequirement{},
		Settings: []HandSetting{
			{
				Key:         "prediction_horizon",
				Label:       "Prediction Horizon",
				Description: "How far ahead to predict",
				SettingType: HandSettingTypeSelect,
				Default:     "30d",
				Options: []HandSettingOption{
					{Value: "7d", Label: "7 days"},
					{Value: "30d", Label: "30 days"},
					{Value: "90d", Label: "90 days"},
					{Value: "1y", Label: "1 year"},
				},
			},
			{
				Key:         "contrarian_mode",
				Label:       "Contrarian Mode",
				Description: "Deliberately argue against consensus",
				SettingType: HandSettingTypeToggle,
				Default:     "false",
			},
		},
		Agent: HandAgentConfig{
			Name:         "Superforecaster",
			Description:  "Calibrated prediction engine",
			Module:       "predictor",
			Provider:     "openai",
			Model:        "gpt-4o",
			MaxTokens:    4096,
			Temperature:  0.5,
			SystemPrompt: PredictorSystemPrompt,
		},
		Dashboard: HandDashboard{
			Metrics: []HandMetric{
				{Label: "Predictions Made", MemoryKey: "predictions_made", Format: "number"},
				{Label: "Brier Score", MemoryKey: "brier_score", Format: "number"},
				{Label: "Accuracy", MemoryKey: "accuracy", Format: "percent"},
			},
		},
	},

	// Clip Hand
	{
		ID:          "clip",
		Name:        "Clip",
		Description: "YouTube video processing. Downloads, identifies best moments, cuts into vertical shorts.",
		Category:    HandCategoryContent,
		Icon:        "🎬",
		Tools: []string{
			"youtube_download",
			"transcribe",
			"highlight_detection",
			"video_editor",
			"caption_generator",
		},
		Requires: []HandRequirement{
			{
				Key:             "ffmpeg",
				Label:           "FFmpeg",
				RequirementType: RequirementTypeBinary,
				Description:     "Video processing tool required",
				Install: &HandInstallInfo{
					MacOS:    "brew install ffmpeg",
					LinuxApt: "sudo apt install ffmpeg",
				},
			},
			{
				Key:             "yt_dlp",
				Label:           "yt-dlp",
				RequirementType: RequirementTypeBinary,
				Description:     "YouTube downloader",
				Install: &HandInstallInfo{
					Pip: "pip install yt-dlp",
				},
			},
		},
		Settings: []HandSetting{
			{
				Key:         "aspect_ratio",
				Label:       "Aspect Ratio",
				Description: "Output video aspect ratio",
				SettingType: HandSettingTypeSelect,
				Default:     "9:16",
				Options: []HandSettingOption{
					{Value: "9:16", Label: "Vertical (9:16)"},
					{Value: "1:1", Label: "Square (1:1)"},
					{Value: "16:9", Label: "Horizontal (16:9)"},
				},
			},
			{
				Key:         "clip_duration",
				Label:       "Clip Duration",
				Description: "Target duration for each clip",
				SettingType: HandSettingTypeSelect,
				Default:     "60",
				Options: []HandSettingOption{
					{Value: "30", Label: "30 seconds"},
					{Value: "60", Label: "60 seconds"},
					{Value: "90", Label: "90 seconds"},
				},
			},
		},
		Agent: HandAgentConfig{
			Name:         "Content Creator",
			Description:  "YouTube clip and short-form video specialist",
			Module:       "clip",
			Provider:     "openai",
			Model:        "gpt-4o",
			MaxTokens:    2048,
			Temperature:  0.6,
			SystemPrompt: ClipSystemPrompt,
		},
		Dashboard: HandDashboard{
			Metrics: []HandMetric{
				{Label: "Videos Processed", MemoryKey: "videos_processed", Format: "number"},
				{Label: "Clips Generated", MemoryKey: "clips_generated", Format: "number"},
				{Label: "Total Runtime", MemoryKey: "total_runtime", Format: "duration"},
			},
		},
	},

	// Twitter Hand
	{
		ID:          "twitter",
		Name:        "Twitter",
		Description: "Autonomous Twitter/X account management. Creates content, schedules posts, responds to mentions.",
		Category:    HandCategoryCommunication,
		Icon:        "🐦",
		Tools: []string{
			"twitter_api",
			"content_generator",
			"scheduler",
			"engagement_tracker",
		},
		Requires: []HandRequirement{
			{
				Key:             "twitter_api_key",
				Label:           "Twitter/X API Key",
				RequirementType: RequirementTypeAPIKey,
				Description:     "Twitter Developer API credentials",
			},
		},
		Settings: []HandSetting{
			{
				Key:         "posting_frequency",
				Label:       "Posting Frequency",
				Description: "How often to post",
				SettingType: HandSettingTypeSelect,
				Default:     "3x_daily",
				Options: []HandSettingOption{
					{Value: "1x_daily", Label: "Once daily"},
					{Value: "3x_daily", Label: "3x daily"},
					{Value: "5x_daily", Label: "5x daily"},
				},
			},
			{
				Key:         "tone",
				Label:       "Content Tone",
				Description: "Tone of voice for posts",
				SettingType: HandSettingTypeSelect,
				Default:     "professional",
				Options: []HandSettingOption{
					{Value: "professional", Label: "Professional"},
					{Value: "casual", Label: "Casual"},
					{Value: "humorous", Label: "Humorous"},
					{Value: "technical", Label: "Technical"},
				},
			},
			{
				Key:         "require_approval",
				Label:       "Require Approval",
				Description: "Approve all posts before publishing",
				SettingType: HandSettingTypeToggle,
				Default:     "true",
			},
		},
		Agent: HandAgentConfig{
			Name:         "Social Media Manager",
			Description:  "Autonomous Twitter/X content and engagement",
			Module:       "twitter",
			Provider:     "openai",
			Model:        "gpt-4o",
			MaxTokens:    1024,
			Temperature:  0.7,
			SystemPrompt: TwitterSystemPrompt,
		},
		Dashboard: HandDashboard{
			Metrics: []HandMetric{
				{Label: "Posts Published", MemoryKey: "posts_published", Format: "number"},
				{Label: "Engagement Rate", MemoryKey: "engagement_rate", Format: "percent"},
				{Label: "Followers Gained", MemoryKey: "followers_gained", Format: "number"},
			},
		},
	},

	// Browser Hand
	{
		ID:          "browser",
		Name:        "Browser",
		Description: "Web automation agent. Navigates sites, fills forms, handles multi-step workflows.",
		Category:    HandCategoryProductivity,
		Icon:        "🌐",
		Tools: []string{
			"playwright",
			"form_filler",
			"click_element",
			"page_navigator",
		},
		Requires: []HandRequirement{
			{
				Key:             "playwright",
				Label:           "Playwright",
				RequirementType: RequirementTypeBinary,
				Description:     "Browser automation framework",
				Install: &HandInstallInfo{
					Pip: "pip install playwright && playwright install",
				},
			},
		},
		Settings: []HandSetting{
			{
				Key:         "headless",
				Label:       "Headless Mode",
				Description: "Run browser without UI",
				SettingType: HandSettingTypeToggle,
				Default:     "true",
			},
			{
				Key:         "purchase_approval",
				Label:       "Purchase Approval",
				Description: "Require approval for any purchases",
				SettingType: HandSettingTypeToggle,
				Default:     "true",
			},
		},
		Agent: HandAgentConfig{
			Name:         "Web Automation Agent",
			Description:  "Browser automation and web workflow executor",
			Module:       "browser",
			Provider:     "openai",
			Model:        "gpt-4o",
			MaxTokens:    4096,
			Temperature:  0.2,
			SystemPrompt: BrowserSystemPrompt,
		},
		Dashboard: HandDashboard{
			Metrics: []HandMetric{
				{Label: "Workflows Executed", MemoryKey: "workflows_executed", Format: "number"},
				{Label: "Pages Navigated", MemoryKey: "pages_navigated", Format: "number"},
				{Label: "Forms Submitted", MemoryKey: "forms_submitted", Format: "number"},
			},
		},
	},
}

// GetBundledHands returns all bundled Hand definitions
func GetBundledHands() []*HandDefinition {
	return bundledHands
}

// GetBundledHand returns a specific bundled Hand by ID
func GetBundledHand(id string) (*HandDefinition, bool) {
	for _, hand := range bundledHands {
		if hand.ID == id {
			return hand, true
		}
	}
	return nil, false
}

// CreateHandInstance creates a new Hand instance from a definition
func CreateHandInstance(def *HandDefinition, config map[string]interface{}) *HandInstance {
	now := time.Now()
	return &HandInstance{
		InstanceID:  "inst_" + def.ID + "_" + now.Format("20060102150405"),
		HandID:      def.ID,
		Status:      HandStatusInactive,
		AgentID:     "agent_" + def.ID,
		AgentName:   def.Agent.Name,
		Config:      config,
		ActivatedAt: now,
		UpdatedAt:   now,
	}
}
