package service

import (
	"context"
	"fmt"

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/port"
	"github.com/cipta/crm-for-aiagents/internal/template"
)

// TemplateService implements business logic for managing email templates.
type TemplateService struct {
	repo port.TemplateRepo
}

// NewTemplateService creates a new TemplateService.
func NewTemplateService(repo port.TemplateRepo) *TemplateService {
	return &TemplateService{
		repo: repo,
	}
}

// Create validates and persists a new email template.
func (s *TemplateService) Create(ctx context.Context, t domain.Template) (domain.Template, error) {
	if t.Name == "" {
		return domain.Template{}, fmt.Errorf("%w: name required", domain.ErrValidation)
	}
	if t.Subject == "" {
		return domain.Template{}, fmt.Errorf("%w: subject required", domain.ErrValidation)
	}

	return s.repo.Create(ctx, t)
}

// Get retrieves a template by ID.
func (s *TemplateService) Get(ctx context.Context, id int64) (domain.Template, error) {
	return s.repo.Get(ctx, id)
}

// GetByName retrieves a template by name.
func (s *TemplateService) GetByName(ctx context.Context, name string) (domain.Template, error) {
	return s.repo.GetByName(ctx, name)
}

// List retrieves all templates.
func (s *TemplateService) List(ctx context.Context) ([]domain.Template, error) {
	return s.repo.List(ctx)
}

// Update validates and updates an existing template.
func (s *TemplateService) Update(ctx context.Context, id int64, t domain.Template) (domain.Template, error) {
	if t.Name == "" {
		return domain.Template{}, fmt.Errorf("%w: name required", domain.ErrValidation)
	}
	if t.Subject == "" {
		return domain.Template{}, fmt.Errorf("%w: subject required", domain.ErrValidation)
	}

	return s.repo.Update(ctx, id, t)
}

// Delete marks a template as deleted (soft-delete).
func (s *TemplateService) Delete(ctx context.Context, id int64) error {
	return s.repo.SoftDelete(ctx, id)
}

// RenderInput specifies inputs for rendering a template.
type RenderInput struct {
	TemplateID int64 // if >0, load template from repo; else use raw fields
	Subject    string
	BodyHTML   string
	BodyText   string
	Vars       map[string]any
	WantHTML   bool // include rendered HTML in result
}

// RenderResult contains the rendering output.
type RenderResult struct {
	Subject string
	Text    string
	HTML    string // populated only when WantHTML
}

// Render executes a template (loaded from repo or passed raw) with the provided variables.
func (s *TemplateService) Render(ctx context.Context, in RenderInput) (RenderResult, error) {
	var rawSubject, rawHTML, rawText string

	if in.TemplateID > 0 {
		t, err := s.repo.Get(ctx, in.TemplateID)
		if err != nil {
			return RenderResult{}, err
		}
		rawSubject = t.Subject
		rawHTML = t.BodyHTML
		rawText = t.BodyText
	} else {
		rawSubject = in.Subject
		rawHTML = in.BodyHTML
		rawText = in.BodyText
	}

	var res RenderResult
	var err error

	if rawSubject != "" {
		res.Subject, err = template.Render(rawSubject, in.Vars)
		if err != nil {
			return RenderResult{}, err
		}
	}

	if rawText != "" {
		res.Text, err = template.Render(rawText, in.Vars)
		if err != nil {
			return RenderResult{}, err
		}
	}

	if in.WantHTML && rawHTML != "" {
		res.HTML, err = template.Render(rawHTML, in.Vars)
		if err != nil {
			return RenderResult{}, err
		}
	}

	return res, nil
}
