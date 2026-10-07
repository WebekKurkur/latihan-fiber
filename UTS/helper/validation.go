package helper

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

var yearPattern = regexp.MustCompile(`^([0-9]{4})/([0-9]{4})-(Ganjil|Genap)$`)
var validate = newValidator()

func newValidator() *validator.Validate {
	v := validator.New()
	v.RegisterTagNameFunc(func(field reflect.StructField) string {
		name := strings.SplitN(field.Tag.Get("json"), ",", 2)[0]
		if name == "" || name == "-" {
			return field.Name
		}
		return name
	})
	_ = v.RegisterValidation("nim", func(fl validator.FieldLevel) bool {
		value := fl.Field().String()
		if len(value) != 12 {
			return false
		}
		_, err := strconv.ParseUint(value, 10, 64)
		return err == nil
	})
	_ = v.RegisterValidation("angkatan", func(fl validator.FieldLevel) bool {
		value := int(fl.Field().Int())
		return value >= 1000 && value <= time.Now().Year()
	})
	_ = v.RegisterValidation("tahunakademik", func(fl validator.FieldLevel) bool {
		match := yearPattern.FindStringSubmatch(fl.Field().String())
		if match == nil {
			return false
		}
		start, _ := strconv.Atoi(match[1])
		end, _ := strconv.Atoi(match[2])
		return end == start+1
	})
	return v
}

func ValidateStruct(value any) error {
	err := validate.Struct(value)
	if err == nil {
		return nil
	}
	var invalid *validator.InvalidValidationError
	if errors.As(err, &invalid) {
		return Internal(err)
	}
	var fieldErrors validator.ValidationErrors
	if !errors.As(err, &fieldErrors) {
		return Internal(err)
	}
	fields := make(map[string][]string, len(fieldErrors))
	for _, fieldError := range fieldErrors {
		if _, exists := fields[fieldError.Field()]; !exists {
			fields[fieldError.Field()] = []string{validationMessage(fieldError)}
		}
	}
	return Validation(fields)
}

func validationMessage(field validator.FieldError) string {
	switch field.Tag() {
	case "required":
		return "Wajib diisi"
	case "email":
		return "Format email tidak valid"
	case "min":
		return "Minimal " + field.Param() + " karakter"
	case "gt":
		return "Harus lebih besar dari " + field.Param()
	case "gte", "lte":
		return "Nilai harus antara 0 dan 4"
	case "nim":
		return "NIM harus 12 digit"
	case "angkatan":
		return "Angkatan harus 4 digit dan tidak melebihi tahun berjalan"
	case "tahunakademik":
		return "Format harus 2026/2027-Ganjil atau Genap dan tahun harus berurutan"
	default:
		return "Tidak memenuhi aturan " + field.Tag()
	}
}

func ParamID(c *fiber.Ctx) (int, error) {
	n, err := strconv.Atoi(c.Params("id"))
	if err != nil || n <= 0 {
		return 0, Validation(map[string][]string{"id": {"ID harus angka positif"}})
	}
	return n, nil
}

func PositiveQuery(c *fiber.Ctx, key string, fallback, max int) (int, error) {
	value := c.Query(key)
	if value == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 1 || n > max {
		return 0, Validation(map[string][]string{key: {fmt.Sprintf("%s harus 1 sampai %d", key, max)}})
	}
	return n, nil
}
