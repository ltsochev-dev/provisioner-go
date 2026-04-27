package tenant

import "fmt"

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func ValidateCreateRequest(req CreateRequest) error {
	switch {
	case req.Slug == "":
		return ValidationError{Field: "slug", Message: "slug is required"}
	case req.Domain == "":
		return ValidationError{Field: "domain", Message: "domain is required"}
	case req.Plan == "":
		return ValidationError{Field: "plan", Message: "plan is required"}
	case !IsSafeSlug(req.Slug):
		return ValidationError{Field: "slug", Message: "slug may only contain lowercase letters, numbers, and hyphens"}
	default:
		return nil
	}
}

func IsSafeSlug(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			continue
		}

		if r >= '0' && r <= '9' {
			continue
		}

		if r == '-' {
			continue
		}

		return false
	}

	return true
}

func InvalidFieldError(field, message string) error {
	return ValidationError{Field: field, Message: fmt.Sprintf("%s: %s", field, message)}
}
