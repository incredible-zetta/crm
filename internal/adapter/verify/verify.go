// Package verify provides a self-hosted email verifier: RFC syntax checking,
// DNS MX/A lookups, and disposable/role-address heuristics. It deliberately
// avoids SMTP RCPT probing, which is unreliable and harms sender reputation.
package verify

import (
	"context"
	"net"
	"net/mail"
	"strings"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
	"github.com/incredible-zetta/crm/internal/port"
)

// Resolver is the subset of net.Resolver used by the verifier, extracted for
// testability.
type Resolver interface {
	LookupMX(ctx context.Context, name string) ([]*net.MX, error)
	LookupHost(ctx context.Context, host string) ([]string, error)
}

// Verifier implements port.EmailVerifier.
type Verifier struct {
	resolver   Resolver
	now        func() time.Time
	disposable map[string]bool
	roleLocal  map[string]bool
}

var _ port.EmailVerifier = (*Verifier)(nil)

// New returns a Verifier using the system DNS resolver.
func New() *Verifier {
	return &Verifier{
		resolver:   net.DefaultResolver,
		now:        time.Now,
		disposable: defaultDisposableDomains(),
		roleLocal:  defaultRoleLocals(),
	}
}

// Verify classifies an email address. It never returns an error; an
// undeterminable result is reported as EmailUnknown so callers can decide.
func (v *Verifier) Verify(ctx context.Context, email string) domain.EmailVerification {
	now := v.now().UTC()
	res := domain.EmailVerification{Email: email, CheckedAt: now}

	addr := strings.TrimSpace(email)
	parsed, err := mail.ParseAddress(addr)
	if err != nil || parsed.Address != addr || !strings.Contains(addr, "@") {
		res.Status = domain.EmailInvalid
		res.Reason = "invalid syntax"
		return res
	}

	at := strings.LastIndex(addr, "@")
	local := strings.ToLower(addr[:at])
	domainPart := strings.ToLower(addr[at+1:])
	if domainPart == "" || strings.Contains(domainPart, " ") {
		res.Status = domain.EmailInvalid
		res.Reason = "invalid domain"
		return res
	}

	// DNS: prefer MX; fall back to A/AAAA (RFC 5321 implicit MX).
	mxs, mxErr := v.resolver.LookupMX(ctx, domainPart)
	hasMail := mxErr == nil && len(mxs) > 0
	if !hasMail {
		hosts, hostErr := v.resolver.LookupHost(ctx, domainPart)
		if hostErr != nil {
			if isNoSuchHost(mxErr) || isNoSuchHost(hostErr) {
				res.Status = domain.EmailInvalid
				res.Reason = "domain has no mail server"
				return res
			}
			// Temporary DNS failure: do not condemn the address.
			res.Status = domain.EmailUnknown
			res.Reason = "dns lookup failed"
			return res
		}
		hasMail = len(hosts) > 0
	}
	if !hasMail {
		res.Status = domain.EmailInvalid
		res.Reason = "domain has no mail server"
		return res
	}

	if v.disposable[domainPart] {
		res.Status = domain.EmailRisky
		res.Reason = "disposable domain"
		return res
	}
	if v.roleLocal[local] {
		res.Status = domain.EmailRisky
		res.Reason = "role address"
		return res
	}

	res.Status = domain.EmailValid
	res.Reason = "deliverable"
	return res
}

func isNoSuchHost(err error) bool {
	var dnsErr *net.DNSError
	if as, ok := err.(*net.DNSError); ok {
		dnsErr = as
	}
	return dnsErr != nil && dnsErr.IsNotFound
}

func defaultDisposableDomains() map[string]bool {
	domains := []string{
		"mailinator.com", "guerrillamail.com", "10minutemail.com",
		"tempmail.com", "temp-mail.org", "throwawaymail.com",
		"yopmail.com", "trashmail.com", "getnada.com", "dispostable.com",
		"fakeinbox.com", "maildrop.cc", "sharklasers.com", "guerrillamail.info",
	}
	m := make(map[string]bool, len(domains))
	for _, d := range domains {
		m[d] = true
	}
	return m
}

func defaultRoleLocals() map[string]bool {
	roles := []string{
		"admin", "administrator", "postmaster", "hostmaster", "webmaster",
		"info", "support", "sales", "contact", "help", "noreply", "no-reply",
		"abuse", "billing", "marketing", "office", "team", "hello",
	}
	m := make(map[string]bool, len(roles))
	for _, r := range roles {
		m[r] = true
	}
	return m
}
