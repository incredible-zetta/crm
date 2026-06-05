package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// ContactService implements business logic for managing contacts.
type ContactService struct {
	repo      port.ContactRepo
	events    port.EventRepo
	exports   port.ExportRepo
	idgen     port.IDGenerator
	clock     port.Clock
	exportDir string
	baseURL   string
	verifier  port.EmailVerifier // optional; nil disables verification
}

// SetVerifier attaches an email verifier. When set, Create and Update verify
// the address and persist the verdict; AuditEmails and VerifyContact use it too.
func (s *ContactService) SetVerifier(v port.EmailVerifier) {
	s.verifier = v
}

// NewContactService creates a new ContactService.
func NewContactService(
	repo port.ContactRepo,
	events port.EventRepo,
	exports port.ExportRepo,
	idgen port.IDGenerator,
	clock port.Clock,
	exportDir, baseURL string,
) *ContactService {
	return &ContactService{
		repo:      repo,
		events:    events,
		exports:   exports,
		idgen:     idgen,
		clock:     clock,
		exportDir: exportDir,
		baseURL:   baseURL,
	}
}

// Create creates a new contact.
func (s *ContactService) Create(ctx context.Context, c domain.Contact) (domain.Contact, error) {
	if c.Email == "" {
		return domain.Contact{}, fmt.Errorf("%w: email required", domain.ErrValidation)
	}
	if c.Stage == "" {
		c.Stage = domain.StageNew
	} else if !c.Stage.Valid() {
		return domain.Contact{}, fmt.Errorf("%w: invalid stage %q", domain.ErrValidation, c.Stage)
	}

	created, err := s.repo.Upsert(ctx, c)
	if err != nil {
		return domain.Contact{}, err
	}
	if s.verifier != nil {
		v := s.verifier.Verify(ctx, created.Email)
		if setErr := s.repo.SetEmailStatus(ctx, created.ID, v); setErr == nil {
			created.EmailStatus = v.Status
			created.EmailReason = v.Reason
			created.EmailCheckedAt = &v.CheckedAt
		}
	}
	return created, nil
}

// Get retrieves a contact by ID.
func (s *ContactService) Get(ctx context.Context, id int64) (domain.Contact, error) {
	return s.repo.Get(ctx, id)
}

// GetByEmail retrieves a contact by email.
func (s *ContactService) GetByEmail(ctx context.Context, email string) (domain.Contact, error) {
	return s.repo.GetByEmail(ctx, email)
}

// Update updates a contact's fields.
func (s *ContactService) Update(ctx context.Context, id int64, patch domain.ContactPatch) (domain.Contact, error) {
	if patch.Stage != nil {
		if !domain.Stage(*patch.Stage).Valid() {
			return domain.Contact{}, fmt.Errorf("%w: invalid stage %q", domain.ErrValidation, *patch.Stage)
		}
	}
	if patch.Email != nil && *patch.Email == "" {
		return domain.Contact{}, fmt.Errorf("%w: email required", domain.ErrValidation)
	}
	updated, err := s.repo.Update(ctx, id, patch)
	if err != nil {
		return domain.Contact{}, err
	}
	// Re-verify only when the email address itself changed.
	if patch.Email != nil && s.verifier != nil {
		v := s.verifier.Verify(ctx, updated.Email)
		if setErr := s.repo.SetEmailStatus(ctx, updated.ID, v); setErr == nil {
			updated.EmailStatus = v.Status
			updated.EmailReason = v.Reason
			updated.EmailCheckedAt = &v.CheckedAt
		}
	}
	return updated, nil
}

// VerifyContact verifies one contact's email and persists the verdict.
func (s *ContactService) VerifyContact(ctx context.Context, id int64) (domain.Contact, domain.EmailVerification, error) {
	if s.verifier == nil {
		return domain.Contact{}, domain.EmailVerification{}, fmt.Errorf("%w: email verification disabled", domain.ErrValidation)
	}
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return domain.Contact{}, domain.EmailVerification{}, err
	}
	v := s.verifier.Verify(ctx, c.Email)
	if err := s.repo.SetEmailStatus(ctx, c.ID, v); err != nil {
		return domain.Contact{}, domain.EmailVerification{}, err
	}
	c.EmailStatus = v.Status
	c.EmailReason = v.Reason
	c.EmailCheckedAt = &v.CheckedAt
	return c, v, nil
}

// EmailAuditResult summarizes a batch verification run.
type EmailAuditResult struct {
	Checked    int
	Valid      int
	Invalid    int
	Risky      int
	Unknown    int
	NextCursor int64
}

// AuditEmails verifies a page of contacts matching the filter and persists each
// verdict. It processes up to limit contacts (default 100, cap 500) starting at
// cursor, returning NextCursor for the caller to continue. onlyUnchecked skips
// contacts already verified.
func (s *ContactService) AuditEmails(ctx context.Context, f domain.ContactFilter, onlyUnchecked bool, limit int, cursor int64) (EmailAuditResult, error) {
	if s.verifier == nil {
		return EmailAuditResult{}, fmt.Errorf("%w: email verification disabled", domain.ErrValidation)
	}
	if limit <= 0 {
		limit = 100
	} else if limit > 500 {
		limit = 500
	}
	var res EmailAuditResult
	page, err := s.repo.List(ctx, f, port.Paging{Limit: limit, Cursor: cursor})
	if err != nil {
		return res, fmt.Errorf("failed to list contacts: %w", err)
	}
	for _, c := range page.Items {
		if onlyUnchecked && c.EmailCheckedAt != nil {
			continue
		}
		v := s.verifier.Verify(ctx, c.Email)
		if err := s.repo.SetEmailStatus(ctx, c.ID, v); err != nil {
			continue
		}
		res.Checked++
		switch v.Status {
		case domain.EmailValid:
			res.Valid++
		case domain.EmailInvalid:
			res.Invalid++
		case domain.EmailRisky:
			res.Risky++
		default:
			res.Unknown++
		}
	}
	res.NextCursor = page.NextCursor
	return res, nil
}

// BulkPatch describes a partial update applied to many contacts at once. Unlike
// ContactPatch, tags are expressed as additive/subtractive sets so an agent can
// tag a segment without clobbering existing tags. SetTags (when non-nil)
// overwrites the whole tag list and takes precedence over Add/Remove.
type BulkPatch struct {
	Company    *string
	Stage      *string
	Notes      *string
	Source     *string
	SetTags    *[]string
	AddTags    []string
	RemoveTags []string
}

func (p BulkPatch) isEmpty() bool {
	return p.Company == nil && p.Stage == nil && p.Notes == nil && p.Source == nil &&
		p.SetTags == nil && len(p.AddTags) == 0 && len(p.RemoveTags) == 0
}

// BulkUpdateResult summarizes a bulk update run.
type BulkUpdateResult struct {
	Matched int
	Updated int
	Skipped int
	Errors  []string
}

const bulkUpdateMaxIDs = 500

// BulkUpdateByIDs applies the same partial patch to each listed contact.
func (s *ContactService) BulkUpdateByIDs(ctx context.Context, ids []int64, patch BulkPatch) (BulkUpdateResult, error) {
	if len(ids) == 0 {
		return BulkUpdateResult{}, fmt.Errorf("%w: ids required", domain.ErrValidation)
	}
	if len(ids) > bulkUpdateMaxIDs {
		return BulkUpdateResult{}, fmt.Errorf("%w: too many ids (max %d)", domain.ErrValidation, bulkUpdateMaxIDs)
	}
	if patch.isEmpty() {
		return BulkUpdateResult{}, fmt.Errorf("%w: empty patch", domain.ErrValidation)
	}
	if patch.Stage != nil && !domain.Stage(*patch.Stage).Valid() {
		return BulkUpdateResult{}, fmt.Errorf("%w: invalid stage %q", domain.ErrValidation, *patch.Stage)
	}

	var res BulkUpdateResult
	for _, id := range ids {
		res.Matched++
		if err := s.applyBulkPatch(ctx, id, patch); err != nil {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("id %d: %v", id, err))
			continue
		}
		res.Updated++
	}
	return res, nil
}

// BulkUpdateByFilter applies the same partial patch to every contact matching
// the filter. It pages internally so large segments do not require the caller
// to loop.
func (s *ContactService) BulkUpdateByFilter(ctx context.Context, f domain.ContactFilter, patch BulkPatch) (BulkUpdateResult, error) {
	if patch.isEmpty() {
		return BulkUpdateResult{}, fmt.Errorf("%w: empty patch", domain.ErrValidation)
	}
	if patch.Stage != nil && !domain.Stage(*patch.Stage).Valid() {
		return BulkUpdateResult{}, fmt.Errorf("%w: invalid stage %q", domain.ErrValidation, *patch.Stage)
	}

	var res BulkUpdateResult
	var cursor int64
	for {
		page, err := s.repo.List(ctx, f, port.Paging{Limit: 100, Cursor: cursor})
		if err != nil {
			return res, fmt.Errorf("failed to page contacts: %w", err)
		}
		if len(page.Items) == 0 {
			break
		}
		for _, c := range page.Items {
			res.Matched++
			if err := s.applyBulkPatchToContact(ctx, c, patch); err != nil {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("id %d: %v", c.ID, err))
				continue
			}
			res.Updated++
		}
		if page.NextCursor == 0 {
			break
		}
		cursor = page.NextCursor
	}
	return res, nil
}

func (s *ContactService) applyBulkPatch(ctx context.Context, id int64, patch BulkPatch) error {
	c, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.applyBulkPatchToContact(ctx, c, patch)
}

func (s *ContactService) applyBulkPatchToContact(ctx context.Context, c domain.Contact, patch BulkPatch) error {
	cp := domain.ContactPatch{
		Company: patch.Company,
		Stage:   patch.Stage,
		Notes:   patch.Notes,
		Source:  patch.Source,
	}
	if tags, changed := resolveBulkTags(c.Tags, patch); changed {
		cp.Tags = &tags
	}
	_, err := s.repo.Update(ctx, c.ID, cp)
	return err
}

// resolveBulkTags computes the new tag set for a contact given a BulkPatch.
// Returns the resolved tags and whether they differ from the patch intent.
func resolveBulkTags(current []string, patch BulkPatch) ([]string, bool) {
	if patch.SetTags != nil {
		return append([]string{}, (*patch.SetTags)...), true
	}
	if len(patch.AddTags) == 0 && len(patch.RemoveTags) == 0 {
		return nil, false
	}
	seen := map[string]bool{}
	var out []string
	for _, t := range current {
		seen[t] = true
	}
	remove := map[string]bool{}
	for _, t := range patch.RemoveTags {
		remove[t] = true
	}
	for _, t := range current {
		if !remove[t] {
			out = append(out, t)
		}
	}
	for _, t := range patch.AddTags {
		if !seen[t] && !remove[t] {
			out = append(out, t)
			seen[t] = true
		}
	}
	return out, true
}

// List returns a page of contacts.
func (s *ContactService) List(ctx context.Context, f domain.ContactFilter, limit int, cursor int64) (port.ContactPage, error) {
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	return s.repo.List(ctx, f, port.Paging{Limit: limit, Cursor: cursor})
}

// Import processes a list of contacts and CSV data.
func (s *ContactService) Import(ctx context.Context, arr []domain.Contact, csvData string) (inserted, updated, skipped int, errs []string, err error) {
	if len(arr) == 0 && csvData == "" {
		return 0, 0, 0, nil, fmt.Errorf("%w: empty import", domain.ErrValidation)
	}

	var errorsList []string

	splitTags := func(st string) []string {
		if st == "" {
			return nil
		}
		st = strings.ReplaceAll(st, ",", ";")
		parts := strings.Split(st, ";")
		var res []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				res = append(res, p)
			}
		}
		return res
	}

	// 1. Process array contacts
	for idx, c := range arr {
		if c.Email == "" {
			skipped++
			errorsList = append(errorsList, fmt.Sprintf("contact %d: missing email", idx))
			continue
		}
		if c.Stage != "" && !c.Stage.Valid() {
			skipped++
			errorsList = append(errorsList, fmt.Sprintf("contact %d: invalid stage %q", idx, c.Stage))
			continue
		}

		_, getErr := s.repo.GetByEmail(ctx, c.Email)
		isUpdate := getErr == nil

		_, upsertErr := s.repo.Upsert(ctx, c)
		if upsertErr != nil {
			skipped++
			errorsList = append(errorsList, fmt.Sprintf("contact %d: failed to import %s: %v", idx, c.Email, upsertErr))
		} else {
			if isUpdate {
				updated++
			} else {
				inserted++
			}
		}
	}

	// 2. Process streaming CSV
	if csvData != "" {
		r := csv.NewReader(strings.NewReader(csvData))
		header, err := r.Read()
		if err != nil {
			return 0, 0, 0, nil, fmt.Errorf("%w: could not parse csv header: %v", domain.ErrValidation, err)
		}

		colIdx := make(map[string]int)
		for i, h := range header {
			colIdx[strings.ToLower(strings.TrimSpace(h))] = i
		}

		rowNum := 1
		for {
			rowNum++
			row, err := r.Read()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				skipped++
				errorsList = append(errorsList, fmt.Sprintf("row %d: parse error: %v", rowNum, err))
				continue
			}

			getVal := func(key string) string {
				idx, ok := colIdx[key]
				if !ok || idx >= len(row) {
					return ""
				}
				return strings.TrimSpace(row[idx])
			}

			emailVal := getVal("email")
			if emailVal == "" {
				skipped++
				errorsList = append(errorsList, fmt.Sprintf("row %d: missing email", rowNum))
				continue
			}

			stageVal := getVal("stage")
			if stageVal != "" && !domain.Stage(stageVal).Valid() {
				skipped++
				errorsList = append(errorsList, fmt.Sprintf("row %d: bad stage %q", rowNum, stageVal))
				continue
			}

			tagsStr := getVal("tags")
			tags := splitTags(tagsStr)

			_, getErr := s.repo.GetByEmail(ctx, emailVal)
			isUpdate := getErr == nil

			_, upsertErr := s.repo.Upsert(ctx, domain.Contact{
				Email:     emailVal,
				FirstName: getVal("first_name"),
				LastName:  getVal("last_name"),
				Company:   getVal("company"),
				Phone:     getVal("phone"),
				Stage:     domain.Stage(stageVal),
				Tags:      tags,
				Source:    getVal("source"),
			})
			if upsertErr != nil {
				skipped++
				errorsList = append(errorsList, fmt.Sprintf("row %d: failed to import %s: %v", rowNum, emailVal, upsertErr))
			} else {
				if isUpdate {
					updated++
				} else {
					inserted++
				}
			}
		}
	}

	return inserted, updated, skipped, errorsList, nil
}

// Export dumps contacts matching a filter to a CSV file.
func (s *ContactService) Export(ctx context.Context, f domain.ContactFilter) (exportID, url string, rows int, err error) {
	if err := os.MkdirAll(s.exportDir, 0755); err != nil {
		return "", "", 0, fmt.Errorf("create export directory: %w", err)
	}

	id, err := s.idgen.ExportID()
	if err != nil {
		return "", "", 0, fmt.Errorf("generate export ID: %w", err)
	}
	// Defence-in-depth: the id comes from idgen (hex), but never allow it to
	// escape exportDir even if a future generator misbehaves.
	if id != filepath.Base(id) || strings.ContainsAny(id, `/\`) {
		return "", "", 0, fmt.Errorf("%w: invalid export id", domain.ErrValidation)
	}

	filename := id + ".csv"
	filePath := filepath.Join(s.exportDir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		return "", "", 0, fmt.Errorf("create export file: %w", err)
	}
	defer file.Close()

	w := csv.NewWriter(file)
	header := []string{"id", "email", "first_name", "last_name", "company", "phone", "stage", "tags", "notes", "source"}
	if err := w.Write(header); err != nil {
		return "", "", 0, fmt.Errorf("write CSV header: %w", err)
	}

	cursor := int64(0)
	totalRows := 0

	for {
		page, err := s.repo.List(ctx, f, port.Paging{Limit: 100, Cursor: cursor})
		if err != nil {
			return "", "", 0, fmt.Errorf("list contacts for export: %w", err)
		}

		for _, c := range page.Items {
			tagsStr := strings.Join(c.Tags, ";")
			row := []string{
				strconv.FormatInt(c.ID, 10),
				c.Email,
				c.FirstName,
				c.LastName,
				c.Company,
				c.Phone,
				string(c.Stage),
				tagsStr,
				c.Notes,
				c.Source,
			}
			if err := w.Write(row); err != nil {
				return "", "", 0, fmt.Errorf("write CSV row: %w", err)
			}
			totalRows++
		}

		if page.NextCursor == 0 || len(page.Items) == 0 {
			break
		}
		cursor = page.NextCursor
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return "", "", 0, fmt.Errorf("flush CSV writer: %w", err)
	}

	expires := s.clock.Now().Add(24 * time.Hour)
	exp := domain.Export{
		ID:        id,
		Path:      filePath,
		Rows:      totalRows,
		ExpiresAt: &expires,
	}

	if err := s.exports.Create(ctx, exp); err != nil {
		return "", "", 0, fmt.Errorf("create export record: %w", err)
	}

	urlVal := strings.TrimSuffix(s.baseURL, "/") + "/export/" + id + ".csv"
	return id, urlVal, totalRows, nil
}

// Delete removes a contact (soft delete or purge).
func (s *ContactService) Delete(ctx context.Context, id int64, purge bool) error {
	if purge {
		return s.repo.Purge(ctx, id)
	}
	return s.repo.SoftDelete(ctx, id)
}

// Unsubscribe marks a contact as unsubscribed.
func (s *ContactService) Unsubscribe(ctx context.Context, id int64) (domain.Contact, error) {
	now := s.clock.Now()
	err := s.repo.SetUnsubscribed(ctx, id, now)
	if err != nil {
		return domain.Contact{}, err
	}

	// best-effort event logging
	_ = s.events.Insert(ctx, domain.EmailEvent{
		ContactID: id,
		Type:      domain.EventUnsubscribe,
		TS:        now,
	})

	c, err := s.repo.Get(ctx, id)
	if err != nil {
		// The unsubscribe itself already succeeded (SetUnsubscribed returns
		// ErrNotFound on 0 rows). Get filters soft-deleted rows, so a
		// soft-deleted-but-now-unsubscribed contact yields ErrNotFound here;
		// that is not a failure of the operation, so return an empty contact.
		if errors.Is(err, domain.ErrNotFound) {
			return domain.Contact{}, nil
		}
		return domain.Contact{}, err
	}
	return c, nil
}

// UnsubscribeByCode unsubscribes a contact using their unique code.
func (s *ContactService) UnsubscribeByCode(ctx context.Context, code string) (domain.Contact, error) {
	c, err := s.repo.GetByUnsubCode(ctx, code)
	if err != nil {
		return domain.Contact{}, err
	}

	now := s.clock.Now()
	err = s.repo.SetUnsubscribed(ctx, c.ID, now)
	if err != nil {
		return domain.Contact{}, err
	}

	// best-effort event logging
	_ = s.events.Insert(ctx, domain.EmailEvent{
		ContactID: c.ID,
		Type:      domain.EventUnsubscribe,
		TS:        now,
	})

	updated, err := s.repo.Get(ctx, c.ID)
	if err != nil {
		return domain.Contact{}, err
	}
	return updated, nil
}

// EnsureUnsubCode ensures that a contact has an unsubscribe code, generating one if not.
func (s *ContactService) EnsureUnsubCode(ctx context.Context, c domain.Contact) (string, error) {
	if c.UnsubCode != "" {
		return c.UnsubCode, nil
	}
	code, err := s.idgen.UnsubCode()
	if err != nil {
		return "", fmt.Errorf("generate unsub code: %w", err)
	}
	err = s.repo.SetUnsubCode(ctx, c.ID, code)
	if err != nil {
		return "", fmt.Errorf("set unsub code: %w", err)
	}
	return code, nil
}

// GetExportFile resolves a previously-generated export by id, returning its
// on-disk path and expiry. Used by the HTTP download handler. Returns
// domain.ErrNotFound if the export does not exist.
func (s *ContactService) GetExportFile(ctx context.Context, id string) (path string, expiresAt *time.Time, err error) {
	exp, err := s.exports.Get(ctx, id)
	if err != nil {
		return "", nil, err
	}
	return exp.Path, exp.ExpiresAt, nil
}
