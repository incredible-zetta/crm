package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestTemplatesRepo(t *testing.T) {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		t.Skip("no DB_DSN set; skipping integration test")
	}

	repo := getTestDB(t)
	ctx := context.Background()

	// Cleanup templates
	t.Cleanup(func() {
		_, _ = repo.db.ExecContext(ctx, "DELETE FROM email_templates WHERE name LIKE 't4tpl_%'")
	})

	uniqueName1 := fmt.Sprintf("t4tpl_%d_1", time.Now().UnixNano())
	uniqueName2 := fmt.Sprintf("t4tpl_%d_2", time.Now().UnixNano())

	tpl1 := EmailTemplate{
		Name:      uniqueName1,
		Subject:   "Welcome!",
		BodyHTML:  "<p>Hello {{name}}</p>",
		BodyText:  "Hello {{name}}",
		Variables: []string{"name"},
	}

	// 1. CreateTemplate
	created, err := repo.CreateTemplate(ctx, tpl1)
	if err != nil {
		t.Fatalf("CreateTemplate failed: %v", err)
	}

	if created.ID == 0 {
		t.Errorf("expected non-zero ID")
	}
	if created.Name != uniqueName1 {
		t.Errorf("expected Name %q, got %q", uniqueName1, created.Name)
	}
	if created.Subject != "Welcome!" {
		t.Errorf("expected Subject 'Welcome!', got %q", created.Subject)
	}
	if len(created.Variables) != 1 || created.Variables[0] != "name" {
		t.Errorf("expected Variables ['name'], got %v", created.Variables)
	}

	// 2. GetTemplate
	fetched, err := repo.GetTemplate(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if fetched.ID != created.ID || fetched.Name != uniqueName1 {
		t.Errorf("GetTemplate returned incorrect record")
	}

	// 3. GetTemplateByName
	fetchedByName, err := repo.GetTemplateByName(ctx, uniqueName1)
	if err != nil {
		t.Fatalf("GetTemplateByName failed: %v", err)
	}
	if fetchedByName.ID != created.ID {
		t.Errorf("GetTemplateByName returned incorrect record")
	}

	// 4. Duplicate name error
	_, err = repo.CreateTemplate(ctx, EmailTemplate{
		Name:    uniqueName1,
		Subject: "Another",
	})
	if err == nil {
		t.Errorf("expected duplicate name error, got nil")
	}

	// 5. Get non-existent
	_, err = repo.GetTemplate(ctx, 999999999)
	if err == nil {
		t.Errorf("expected GetTemplate non-existent to fail")
	} else if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing template, got %v", err)
	}

	_, err = repo.GetTemplateByName(ctx, "t4tpl_nonexistent")
	if err == nil {
		t.Errorf("expected GetTemplateByName non-existent to fail")
	} else if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for missing template by name, got %v", err)
	}

	// 6. ListTemplates (ordered by name)
	tpl2 := EmailTemplate{
		Name:      uniqueName2,
		Subject:   "Newsletter",
		BodyHTML:  "<p>Update</p>",
		Variables: []string{},
	}
	_, err = repo.CreateTemplate(ctx, tpl2)
	if err != nil {
		t.Fatalf("CreateTemplate 2 failed: %v", err)
	}

	list, err := repo.ListTemplates(ctx)
	if err != nil {
		t.Fatalf("ListTemplates failed: %v", err)
	}

	var found1, found2 bool
	var lastIdx1, lastIdx2 int = -1, -1
	for idx, item := range list {
		if item.Name == uniqueName1 {
			found1 = true
			lastIdx1 = idx
		}
		if item.Name == uniqueName2 {
			found2 = true
			lastIdx2 = idx
		}
	}

	if !found1 || !found2 {
		t.Errorf("expected both created templates in list, got found1=%t, found2=%t", found1, found2)
	}

	// name1 and name2 should be alphabetically ordered
	if uniqueName1 < uniqueName2 {
		if lastIdx1 > lastIdx2 {
			t.Errorf("expected %q before %q, but index is reverse: %d > %d", uniqueName1, uniqueName2, lastIdx1, lastIdx2)
		}
	} else {
		if lastIdx2 > lastIdx1 {
			t.Errorf("expected %q before %q, but index is reverse: %d > %d", uniqueName2, uniqueName1, lastIdx2, lastIdx1)
		}
	}
}
