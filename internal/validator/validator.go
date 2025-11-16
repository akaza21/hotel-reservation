package validator

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	registerCustomValidations()
}

func registerCustomValidations() {
	validate.RegisterValidation("password", validatePassword)
	validate.RegisterValidation("rating", validateRating)
}

func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	return len(password) >= 6
}

func validateRating(fl validator.FieldLevel) bool {
	rating := fl.Field().Int()
	return rating >= 1 && rating <= 5
}

type ValidationError struct {
	Message string
	Details string
}

func (v ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", v.Message, v.Details)
}

func ValidateStruct(s interface{}) error {
	if err := validate.Struct(s); err != nil {
		return formatValidationError(err)
	}
	return nil
}

func formatValidationError(err error) error {
	var errors []string
	
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, validationError := range validationErrors {
			field := validationError.Field()
			tag := validationError.Tag()
			
			var message string
			switch tag {
			case "required":
				message = fmt.Sprintf("%s is required", field)
			case "email":
				message = fmt.Sprintf("%s must be a valid email", field)
			case "min":
				message = fmt.Sprintf("%s must be at least %s characters", field, validationError.Param())
			case "max":
				message = fmt.Sprintf("%s must be at most %s characters", field, validationError.Param())
			case "password":
				message = fmt.Sprintf("%s must be at least 6 characters", field)
			case "rating":
				message = fmt.Sprintf("%s must be between 1 and 5", field)
			default:
				message = fmt.Sprintf("%s is invalid", field)
			}
			
			errors = append(errors, message)
		}
	}
	
	return ValidationError{
		Message: "Validation failed",
		Details: strings.Join(errors, "; "),
	}
}

func ValidateObjectID(id string) error {
	if len(id) != 24 {
		return ValidationError{
			Message: "Invalid ID format",
			Details: "The provided ID is not a valid ObjectID",
		}
	}
	
	for _, char := range id {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return ValidationError{
				Message: "Invalid ID format",
				Details: "The provided ID is not a valid ObjectID",
			}
		}
	}
	
	return nil
}