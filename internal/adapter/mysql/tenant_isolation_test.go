package mysql

import (
	"testing"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
	"github.com/incredible-zetta/crm/internal/tenant"
)

// TestTenantIsolationContacts verifies that two tenants sharing the same email
// do not see each other's contacts and may reuse the same email address.
// Integration test: requires DB_DSN.
func TestTenantIsolationContacts(t *testing.T) {
	store := getTestStore(t)
	repo := store.Contacts()

	ctxA := tenant.With(t.Context(), "t_mysql_iso_a")
	ctxB := tenant.With(t.Context(), "t_mysql_iso_b")

	email := "t_mysql_iso_shared@example.com"

	if _, err := repo.Upsert(ctxA, domain.Contact{Email: email, Company: "Aco"}); err != nil {
		t.Fatalf("upsert A: %v", err)
	}
	if _, err := repo.Upsert(ctxB, domain.Contact{Email: email, Company: "Bco"}); err != nil {
		t.Fatalf("upsert B (same email, other tenant): %v", err)
	}

	gotA, err := repo.GetByEmail(ctxA, email)
	if err != nil {
		t.Fatalf("get A: %v", err)
	}
	gotB, err := repo.GetByEmail(ctxB, email)
	if err != nil {
		t.Fatalf("get B: %v", err)
	}
	if gotA.ID == gotB.ID {
		t.Fatalf("expected distinct rows per tenant, both id=%d", gotA.ID)
	}
	if gotA.Company != "Aco" || gotB.Company != "Bco" {
		t.Fatalf("cross-tenant data leak: A.company=%q B.company=%q", gotA.Company, gotB.Company)
	}

	// Tenant A's list must not contain tenant B's row.
	pageA, err := repo.List(ctxA, domain.ContactFilter{Q: "t_mysql_iso_shared"}, port.Paging{Limit: 100})
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	for _, c := range pageA.Items {
		if c.ID == gotB.ID {
			t.Fatalf("tenant A list leaked tenant B contact id=%d", gotB.ID)
		}
	}

	// A contact id from tenant B is invisible to tenant A by id.
	if _, err := repo.Get(ctxA, gotB.ID); err == nil {
		t.Fatalf("tenant A should not Get tenant B contact id=%d", gotB.ID)
	}

	t.Cleanup(func() {
		_ = repo.Purge(ctxA, gotA.ID)
		_ = repo.Purge(ctxB, gotB.ID)
	})
}
