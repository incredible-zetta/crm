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

	"github.com/cipta/crm-for-aiagents/internal/domain"
	"github.com/cipta/crm-for-aiagents/internal/port"
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
	return s.repo.Update(ctx, id, patch)
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
