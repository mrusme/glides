package validation

import (
	"regexp"

	"github.com/go-playground/validator/v10"
)

const SlugPattern string = `^[a-z0-9\-]+$`

var slug = regexp.MustCompile(SlugPattern)

func IsValidSlug(fl validator.FieldLevel) bool {
	return slug.MatchString(fl.Field().String())
}

func New() *validator.Validate {
	validate := validator.New()
	validate.RegisterValidation("slug", IsValidSlug)

	return validate
}

type StructValidator struct {
	validate *validator.Validate
}

func NewStructValidator(validate *validator.Validate) *StructValidator {
	return &StructValidator{validate: validate}
}

func (v *StructValidator) Validate(out any) error {
	return v.validate.Struct(out)
}
