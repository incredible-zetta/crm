package service_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/service"
)

// --- Fake Template Repo ---

type fakeTemplateRepo struct {
	mu        sync.Mutex
	templates map[int64]domain.Template
	nextID    int64
}

func newFakeTemplateRepo() *fakeTemplateRepo {
	return &fakeTemplateRepo{
		templates: make(map[int64]domain.Template),
		nextID:    1,
	}
}

func (r *fakeTemplateRepo) Create(ctx context.Context, t domain.Template) (domain.Template, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.templates {
		if existing.DeletedAt == nil && existing.Name == t.Name {
			return domain.Template{}, domain.ErrConflict
		}
	}

	if t.ID == 0 {
		t.ID = r.nextID
		r.nextID++
	}
	t.CreatedAt = time.Now()
	t.UpdatedAt = time.Now()
	r.templates[t.ID] = t
	return t, nil
}

func (r *fakeTemplateRepo) Get(ctx context.Context, id int64) (domain.Template, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	t, ok := r.templates[id]
	if !ok || t.DeletedAt != nil {
		return domain.Template{}, domain.ErrNotFound
	}
	return t, nil
}

func (r *fakeTemplateRepo) GetByName(ctx context.Context, name string) (domain.Template, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, t := range r.templates {
		if t.DeletedAt == nil && t.Name == name {
			return t, nil
		}
	}
	return domain.Template{}, domain.ErrNotFound
}

func (r *fakeTemplateRepo) List(ctx context.Context) ([]domain.Template, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var list []domain.Template
	for _, t := range r.templates {
		if t.DeletedAt == nil {
			list = append(list, t)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list, nil
}

func (r *fakeTemplateRepo) Update(ctx context.Context, id int64, t domain.Template) (domain.Template, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.templates[id]
	if !ok || existing.DeletedAt != nil {
		return domain.Template{}, domain.ErrNotFound
	}

	for _, other := range r.templates {
		if other.ID != id && other.DeletedAt == nil && other.Name == t.Name {
			return domain.Template{}, domain.ErrConflict
		}
	}

	existing.Name = t.Name
	existing.Subject = t.Subject
	existing.BodyHTML = t.BodyHTML
	existing.BodyText = t.BodyText
	existing.Variables = t.Variables
	existing.UpdatedAt = time.Now()

	r.templates[id] = existing
	return existing, nil
}

func (r *fakeTemplateRepo) SoftDelete(ctx context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.templates[id]
	if !ok || existing.DeletedAt != nil {
		return domain.ErrNotFound
	}

	now := time.Now()
	existing.DeletedAt = &now
	r.templates[id] = existing
	return nil
}

// --- Test Cases ---

func TestTemplateCreateValidatesName(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := service.NewTemplateService(repo)

	// Case 1: empty name
	_, err := svc.Create(context.Background(), domain.Template{Name: "", Subject: "Some Subject"})
	if err == nil {
		t.Fatal("expected validation error for empty name, got nil")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got: %v", err)
	}
	if !strings.Contains(err.Error(), "name required") {
		t.Fatalf("expected message to contain 'name required', got: %v", err)
	}

	// Case 2: empty subject
	_, err = svc.Create(context.Background(), domain.Template{Name: "welcome", Subject: ""})
	if err == nil {
		t.Fatal("expected validation error for empty subject, got nil")
	}
	if !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected ErrValidation, got: %v", err)
	}
	if !strings.Contains(err.Error(), "subject required") {
		t.Fatalf("expected message to contain 'subject required', got: %v", err)
	}
}

func TestTemplateCreateGetList(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := service.NewTemplateService(repo)

	// Create two templates
	t1, err := svc.Create(context.Background(), domain.Template{Name: "welcome", Subject: "Welcome!"})
	if err != nil {
		t.Fatalf("failed to create t1: %v", err)
	}
	t2, err := svc.Create(context.Background(), domain.Template{Name: "bye", Subject: "Goodbye!"})
	if err != nil {
		t.Fatalf("failed to create t2: %v", err)
	}

	// List templates
	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("failed to list templates: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(list))
	}

	// Get by ID
	got1, err := svc.Get(context.Background(), t1.ID)
	if err != nil {
		t.Fatalf("failed to get t1: %v", err)
	}
	if got1.Name != "welcome" {
		t.Errorf("expected welcome, got %q", got1.Name)
	}

	// Get by Name
	got2, err := svc.GetByName(context.Background(), "bye")
	if err != nil {
		t.Fatalf("failed to get t2 by name: %v", err)
	}
	if got2.ID != t2.ID {
		t.Errorf("expected template ID %d, got %d", t2.ID, got2.ID)
	}
}

func TestTemplateUpdate(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := service.NewTemplateService(repo)

	t1, err := svc.Create(context.Background(), domain.Template{Name: "welcome", Subject: "Welcome!"})
	if err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	updatedTmpl := t1
	updatedTmpl.Subject = "New Subject"
	updatedTmpl.BodyText = "New Text"

	updated, err := svc.Update(context.Background(), t1.ID, updatedTmpl)
	if err != nil {
		t.Fatalf("failed to update: %v", err)
	}
	if updated.Subject != "New Subject" || updated.BodyText != "New Text" {
		t.Errorf("update fields not set in return value: %+v", updated)
	}

	got, err := svc.Get(context.Background(), t1.ID)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	if got.Subject != "New Subject" || got.BodyText != "New Text" {
		t.Errorf("update fields not persisted: %+v", got)
	}
}

func TestTemplateDeleteHides(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := service.NewTemplateService(repo)

	t1, err := svc.Create(context.Background(), domain.Template{Name: "welcome", Subject: "Welcome!"})
	if err != nil {
		t.Fatalf("failed to create: %v", err)
	}

	err = svc.Delete(context.Background(), t1.ID)
	if err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	_, err = svc.Get(context.Background(), t1.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after deletion, got: %v", err)
	}

	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("failed to list: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("expected list to be empty, got %d", len(list))
	}
}

func TestRenderFromRawTextDefault(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := service.NewTemplateService(repo)

	in := service.RenderInput{
		Subject:  "Hi {{.name}}",
		BodyText: "Hello {{.name}}",
		BodyHTML: "<b>{{.name}}</b>",
		Vars:     map[string]any{"name": "Indra"},
		WantHTML: false,
	}

	res, err := svc.Render(context.Background(), in)
	if err != nil {
		t.Fatalf("failed to render: %v", err)
	}

	if res.Subject != "Hi Indra" {
		t.Errorf("expected Subject 'Hi Indra', got %q", res.Subject)
	}
	if res.Text != "Hello Indra" {
		t.Errorf("expected Text 'Hello Indra', got %q", res.Text)
	}
	if res.HTML != "" {
		t.Errorf("expected HTML empty when WantHTML is false, got %q", res.HTML)
	}
}

func TestRenderFromRawWantHTML(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := service.NewTemplateService(repo)

	in := service.RenderInput{
		Subject:  "Hi {{.name}}",
		BodyText: "Hello {{.name}}",
		BodyHTML: "<b>{{.name}}</b>",
		Vars:     map[string]any{"name": "Indra"},
		WantHTML: true,
	}

	res, err := svc.Render(context.Background(), in)
	if err != nil {
		t.Fatalf("failed to render: %v", err)
	}

	if res.Subject != "Hi Indra" {
		t.Errorf("expected Subject 'Hi Indra', got %q", res.Subject)
	}
	if res.Text != "Hello Indra" {
		t.Errorf("expected Text 'Hello Indra', got %q", res.Text)
	}
	if res.HTML != "<b>Indra</b>" {
		t.Errorf("expected HTML '<b>Indra</b>', got %q", res.HTML)
	}
}

func TestRenderFromTemplateID(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := service.NewTemplateService(repo)

	t1, err := repo.Create(context.Background(), domain.Template{
		Name:     "tpl",
		Subject:  "Hi {{.name}}",
		BodyText: "Hello {{.name}}",
		BodyHTML: "<b>{{.name}}</b>",
	})
	if err != nil {
		t.Fatalf("failed to seed: %v", err)
	}

	// Case 1: Render existing TemplateID with WantHTML = true
	in := service.RenderInput{
		TemplateID: t1.ID,
		Vars:       map[string]any{"name": "Indra"},
		WantHTML:   true,
	}

	res, err := svc.Render(context.Background(), in)
	if err != nil {
		t.Fatalf("failed to render: %v", err)
	}

	if res.Subject != "Hi Indra" {
		t.Errorf("expected Subject 'Hi Indra', got %q", res.Subject)
	}
	if res.Text != "Hello Indra" {
		t.Errorf("expected Text 'Hello Indra', got %q", res.Text)
	}
	if res.HTML != "<b>Indra</b>" {
		t.Errorf("expected HTML '<b>Indra</b>', got %q", res.HTML)
	}

	// Case 2: Render missing template ID -> ErrNotFound
	inMissing := service.RenderInput{
		TemplateID: 999,
		Vars:       map[string]any{"name": "Indra"},
		WantHTML:   true,
	}
	_, err = svc.Render(context.Background(), inMissing)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got: %v", err)
	}
}

func TestRenderMissingVarBlank(t *testing.T) {
	repo := newFakeTemplateRepo()
	svc := service.NewTemplateService(repo)

	in := service.RenderInput{
		Subject:  "Hi {{.name}}!",
		BodyText: "Hello {{.name}}!",
		BodyHTML: "<b>{{.name}}</b>!",
		Vars:     nil, // missing
		WantHTML: true,
	}

	res, err := svc.Render(context.Background(), in)
	if err != nil {
		t.Fatalf("failed to render: %v", err)
	}

	if res.Subject != "Hi !" {
		t.Errorf("expected Subject 'Hi !', got %q", res.Subject)
	}
	if res.Text != "Hello !" {
		t.Errorf("expected Text 'Hello !', got %q", res.Text)
	}
	if res.HTML != "<b></b>!" {
		t.Errorf("expected HTML '<b></b>!', got %q", res.HTML)
	}
}
