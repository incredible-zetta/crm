package mcptransport

import (
	"context"
	"encoding/json"

	"github.com/incredible-zetta/crm/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// LinkedIn tools proxy to the lingin binary via the LinkedIn service. lingin
// returns raw JSON strings (evolving Voyager schemas), so handlers pass the
// payload through as a `raw` string field rather than re-modeling it here.
// `account` selects the stored LinkedIn identity (per tenant); empty uses the
// tenant's default account.

type liRawOut struct {
	Raw json.RawMessage `json:"raw"`
}

// rawOut wraps a lingin JSON string as structured output, tolerating non-JSON
// by falling back to a quoted string.
func rawOut(s string) liRawOut {
	if json.Valid([]byte(s)) {
		return liRawOut{Raw: json.RawMessage(s)}
	}
	b, _ := json.Marshal(s)
	return liRawOut{Raw: b}
}

func (d *Deps) liEnabled() bool {
	return d.Svc.LinkedIn != nil && d.Svc.LinkedIn.Enabled()
}

type LinkedInMeIn struct {
	Account string `json:"account,omitempty" jsonschema:"Stored LinkedIn account label; empty = tenant default"`
}

func (d *Deps) LinkedInMe(ctx context.Context, req *mcp.CallToolRequest, in LinkedInMeIn) (*mcp.CallToolResult, liRawOut, error) {
	if !d.liEnabled() {
		return mcpserver.Err("disabled", "linkedin channel not configured"), liRawOut{}, nil
	}
	s, err := d.Svc.LinkedIn.Me(ctx, in.Account)
	if err != nil {
		return mcpserver.Err("linkedin_error", err.Error()), liRawOut{}, nil
	}
	return nil, rawOut(s), nil
}

type LinkedInProfileIn struct {
	Account string `json:"account,omitempty" jsonschema:"Stored LinkedIn account label; empty = tenant default"`
	ID      string `json:"id" jsonschema:"Public id (e.g. indra-gunanda) or member urn fragment"`
}

func (d *Deps) LinkedInProfile(ctx context.Context, req *mcp.CallToolRequest, in LinkedInProfileIn) (*mcp.CallToolResult, liRawOut, error) {
	if !d.liEnabled() {
		return mcpserver.Err("disabled", "linkedin channel not configured"), liRawOut{}, nil
	}
	s, err := d.Svc.LinkedIn.Profile(ctx, in.Account, in.ID)
	if err != nil {
		return mcpserver.Err("linkedin_error", err.Error()), liRawOut{}, nil
	}
	return nil, rawOut(s), nil
}

type LinkedInCompanyIn struct {
	Account string `json:"account,omitempty" jsonschema:"Stored LinkedIn account label; empty = tenant default"`
	Name    string `json:"name" jsonschema:"Company universal name (URL slug)"`
}

func (d *Deps) LinkedInCompany(ctx context.Context, req *mcp.CallToolRequest, in LinkedInCompanyIn) (*mcp.CallToolResult, liRawOut, error) {
	if !d.liEnabled() {
		return mcpserver.Err("disabled", "linkedin channel not configured"), liRawOut{}, nil
	}
	s, err := d.Svc.LinkedIn.Company(ctx, in.Account, in.Name)
	if err != nil {
		return mcpserver.Err("linkedin_error", err.Error()), liRawOut{}, nil
	}
	return nil, rawOut(s), nil
}

type LinkedInSearchPeopleIn struct {
	Account  string `json:"account,omitempty" jsonschema:"Stored LinkedIn account label; empty = tenant default"`
	Keywords string `json:"keywords" jsonschema:"Search keywords"`
}

func (d *Deps) LinkedInSearchPeople(ctx context.Context, req *mcp.CallToolRequest, in LinkedInSearchPeopleIn) (*mcp.CallToolResult, liRawOut, error) {
	if !d.liEnabled() {
		return mcpserver.Err("disabled", "linkedin channel not configured"), liRawOut{}, nil
	}
	s, err := d.Svc.LinkedIn.SearchPeople(ctx, in.Account, in.Keywords)
	if err != nil {
		return mcpserver.Err("linkedin_error", err.Error()), liRawOut{}, nil
	}
	return nil, rawOut(s), nil
}
