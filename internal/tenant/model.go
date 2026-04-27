package tenant

type Tenant struct {
	ID     int64  `json:"id"`
	Email  string `json:"email,omitempty"`
	Name   string `json:"name,omitempty"`
	Slug   string `json:"slug"`
	Domain string `json:"domain"`
	Plan   string `json:"plan"`
	Status string `json:"status"`
}

type CreateRequest struct {
	Slug   string `json:"slug"`
	Domain string `json:"domain"`
	Plan   string `json:"plan"`
}

type CreateResponse struct {
	Status string `json:"status"`
	Tenant string `json:"tenant"`
}
