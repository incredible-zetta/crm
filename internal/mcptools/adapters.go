package mcptools

import (
	"context"

	"github.com/cipta/crm-for-aiagents/internal/db"
	"github.com/cipta/crm-for-aiagents/internal/email"
)

type RepoTemplateStore struct {
	Repo *db.Repo
}

func (r RepoTemplateStore) GetTemplate(ctx context.Context, id int64) (email.TemplateData, error) {
	t, err := r.Repo.GetTemplate(ctx, id)
	if err != nil {
		return email.TemplateData{}, err
	}
	return email.TemplateData{
		ID:       t.ID,
		Subject:  t.Subject,
		BodyHTML: t.BodyHTML,
		BodyText: t.BodyText,
	}, nil
}

type RepoLinkMaker struct {
	Repo *db.Repo
}

func (r RepoLinkMaker) CreateLink(ctx context.Context, targetURL string, campaignID, contactID *int64) (string, error) {
	return r.Repo.CreateLink(ctx, targetURL, campaignID, contactID)
}

type RepoEventLogger struct {
	Repo *db.Repo
}

func (r RepoEventLogger) LogEvent(ctx context.Context, contactID int64, campaignID *int64, eventType, linkCode string, meta map[string]any) error {
	return r.Repo.InsertEvent(ctx, db.EmailEvent{
		ContactID:  contactID,
		CampaignID: campaignID,
		Type:       eventType,
		LinkCode:   linkCode,
		Meta:       meta,
	})
}
