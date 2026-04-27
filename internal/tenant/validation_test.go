package tenant

import "testing"

func TestValidateCreateRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     CreateRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: CreateRequest{
				Slug:   "acme-main",
				Domain: "acme.example.com",
				Plan:   "starter",
			},
		},
		{
			name: "missing slug",
			req: CreateRequest{
				Domain: "acme.example.com",
				Plan:   "starter",
			},
			wantErr: true,
		},
		{
			name: "unsafe slug",
			req: CreateRequest{
				Slug:   "Acme_Main",
				Domain: "acme.example.com",
				Plan:   "starter",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateCreateRequest(tt.req)
			if tt.wantErr && err == nil {
				t.Fatal("expected validation error")
			}

			if !tt.wantErr && err != nil {
				t.Fatalf("expected valid request, got %v", err)
			}
		})
	}
}

func TestIsSafeSlug(t *testing.T) {
	t.Parallel()

	tests := map[string]bool{
		"acme":       true,
		"acme-1":     true,
		"acme_main":  false,
		"Acme":       false,
		"acme.local": false,
	}

	for slug, want := range tests {
		if got := IsSafeSlug(slug); got != want {
			t.Fatalf("IsSafeSlug(%q) = %v, want %v", slug, got, want)
		}
	}
}
