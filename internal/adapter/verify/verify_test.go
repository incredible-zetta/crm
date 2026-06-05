package verify

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/incredible-zetta/crm/internal/domain"
)

type fakeResolver struct {
	mx      map[string][]*net.MX
	hosts   map[string][]string
	mxErr   map[string]error
	hostErr map[string]error
}

func (f *fakeResolver) LookupMX(ctx context.Context, name string) ([]*net.MX, error) {
	if err, ok := f.mxErr[name]; ok {
		return nil, err
	}
	return f.mx[name], nil
}

func (f *fakeResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	if err, ok := f.hostErr[host]; ok {
		return nil, err
	}
	return f.hosts[host], nil
}

func newVerifier(r Resolver) *Verifier {
	return &Verifier{
		resolver:   r,
		now:        func() time.Time { return time.Unix(0, 0) },
		disposable: defaultDisposableDomains(),
		roleLocal:  defaultRoleLocals(),
	}
}

func notFound() error { return &net.DNSError{Err: "no such host", IsNotFound: true} }

func TestVerifyInvalidSyntax(t *testing.T) {
	v := newVerifier(&fakeResolver{})
	for _, bad := range []string{"", "no-at-sign", "a@", "@b.com", "a b@c.com"} {
		got := v.Verify(context.Background(), bad)
		if got.Status != domain.EmailInvalid {
			t.Errorf("%q: expected invalid, got %s (%s)", bad, got.Status, got.Reason)
		}
	}
}

func TestVerifyValidWithMX(t *testing.T) {
	r := &fakeResolver{mx: map[string][]*net.MX{"example.com": {{Host: "mx.example.com", Pref: 10}}}}
	got := newVerifier(r).Verify(context.Background(), "jane@example.com")
	if got.Status != domain.EmailValid {
		t.Fatalf("expected valid, got %s (%s)", got.Status, got.Reason)
	}
}

func TestVerifyFallsBackToAHost(t *testing.T) {
	r := &fakeResolver{
		mxErr: map[string]error{"noa.com": notFound()},
		hosts: map[string][]string{"noa.com": {"1.2.3.4"}},
	}
	got := newVerifier(r).Verify(context.Background(), "x@noa.com")
	if got.Status != domain.EmailValid {
		t.Fatalf("expected valid via A record, got %s (%s)", got.Status, got.Reason)
	}
}

func TestVerifyNoMailServer(t *testing.T) {
	r := &fakeResolver{
		mxErr:   map[string]error{"nodomain.com": notFound()},
		hostErr: map[string]error{"nodomain.com": notFound()},
	}
	got := newVerifier(r).Verify(context.Background(), "x@nodomain.com")
	if got.Status != domain.EmailInvalid {
		t.Fatalf("expected invalid, got %s (%s)", got.Status, got.Reason)
	}
}

func TestVerifyTemporaryDNSFailureIsUnknown(t *testing.T) {
	tempErr := &net.DNSError{Err: "server misbehaving", IsTemporary: true}
	r := &fakeResolver{
		mxErr:   map[string]error{"slow.com": tempErr},
		hostErr: map[string]error{"slow.com": tempErr},
	}
	got := newVerifier(r).Verify(context.Background(), "x@slow.com")
	if got.Status != domain.EmailUnknown {
		t.Fatalf("expected unknown on temporary dns failure, got %s (%s)", got.Status, got.Reason)
	}
}

func TestVerifyDisposableIsRisky(t *testing.T) {
	r := &fakeResolver{mx: map[string][]*net.MX{"mailinator.com": {{Host: "mx", Pref: 1}}}}
	got := newVerifier(r).Verify(context.Background(), "x@mailinator.com")
	if got.Status != domain.EmailRisky || got.Reason != "disposable domain" {
		t.Fatalf("expected risky disposable, got %s (%s)", got.Status, got.Reason)
	}
}

func TestVerifyRoleAddressIsRisky(t *testing.T) {
	r := &fakeResolver{mx: map[string][]*net.MX{"example.com": {{Host: "mx", Pref: 1}}}}
	got := newVerifier(r).Verify(context.Background(), "support@example.com")
	if got.Status != domain.EmailRisky || got.Reason != "role address" {
		t.Fatalf("expected risky role, got %s (%s)", got.Status, got.Reason)
	}
}
