package service

import (
	"github.com/incredible-zetta/crm/internal/port"
)

// Config holds the runtime values the service layer needs that are not ports.
type Config struct {
	BaseURL   string // public base URL, used to build tracking/export/unsubscribe links
	ExportDir string // directory where contact CSV exports are written
}

// Repos bundles the segregated repository ports. Adapters (e.g. the mysql
// Store) provide concrete implementations.
type Repos struct {
	Contacts  port.ContactRepo
	Campaigns port.CampaignRepo
	Templates port.TemplateRepo
	Tasks     port.TaskRepo
	Events    port.EventRepo
	Tracking  port.TrackingRepo
	Exports   port.ExportRepo
	Inbox     port.InboxRepo
	Threads   port.ThreadsRepo
}

// Services is the assembled use-case layer. It is the single entry point the
// transports (MCP, HTTP) depend on, so they never touch repositories or
// adapters directly.
type Services struct {
	Contact       *ContactService
	Template      *TemplateService
	Campaign      *CampaignService
	Email         *EmailService
	Task          *TaskService
	CampaignQueue *CampaignQueue
	Analytics     *AnalyticsService
	Tracking      *TrackingService
	Inbox         *InboxService
	WhatsApp      *WhatsAppService
	Threads       *ThreadsService
	X             *XService
	XWatch        *XWatchService
}

// New wires every service from the provided ports and config. The wiring order
// matters: EmailService satisfies CampaignMailer (so it is built before the
// campaign service), and TaskService dispatches to both Email and Campaign.
func New(repos Repos, sender port.EmailSender, clock port.Clock, idgen port.IDGenerator, cfg Config) *Services {
	contact := NewContactService(repos.Contacts, repos.Events, repos.Exports, idgen, clock, cfg.ExportDir, cfg.BaseURL)
	template := NewTemplateService(repos.Templates)

	email := NewEmailService(sender, repos.Contacts, repos.Templates, repos.Tracking, repos.Events, clock, idgen, cfg.BaseURL)

	campaign := NewCampaignService(repos.Campaigns, repos.Contacts, repos.Events, email)

	task := NewTaskService(repos.Tasks, clock, email, campaign)
	campaignQueue := NewCampaignQueue(repos.Campaigns, repos.Tasks)
	analytics := NewAnalyticsService(repos.Contacts, repos.Events, repos.Tasks)
	tracking := NewTrackingService(repos.Tracking, repos.Events, clock, cfg.BaseURL)
	task.SetContact(contact)
	var inbox *InboxService
	if repos.Inbox != nil {
		inbox = NewInboxService(repos.Inbox, repos.Contacts, nil, nil, sender, clock, "INBOX")
	}

	return &Services{
		Contact:       contact,
		Template:      template,
		Campaign:      campaign,
		Email:         email,
		Task:          task,
		CampaignQueue: campaignQueue,
		Analytics:     analytics,
		Tracking:      tracking,
		Inbox:         inbox,
	}
}
