package tenant

type Tenant struct {
	ID        string `json:"id"`
	Email     string `json:"email,omitempty"`
	Name      string `json:"name,omitempty"`
	Slug      string `json:"slug"`
	Domain    string `json:"domain"`
	Plan      string `json:"plan"`
	Status    string `json:"status"`
	SecretKey string `json:"-"`
}

type CreateRequest struct {
	Email  string `json:"email,omitempty"`
	Name   string `json:"name,omitempty"`
	Slug   string `json:"slug"`
	Domain string `json:"domain"`
	Plan   string `json:"plan"`
}

type CreateResponse struct {
	Status string `json:"status"`
	Tenant string `json:"tenant"`
	APIKey string `json:"api_key,omitempty"`
}

type TenantInsert struct {
	ID        string
	Email     string
	Name      string
	Slug      string
	Domain    string
	Plan      string
	SecretKey string
}
