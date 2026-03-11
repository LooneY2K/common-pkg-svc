package validate

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/LooneY2K/common-pkg-svc/errors"
	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
)

var (
	validate *validator.Validate
	trans    ut.Translator
)

func init() {
	InitValidator()
}

func InitValidator() *validator.Validate {
	validate = validator.New()

	enLocale := en.New()
	uni := ut.New(enLocale, enLocale)
	trans, _ = uni.GetTranslator("en")

	en_translations.RegisterDefaultTranslations(validate, trans)

	validate.RegisterTranslation("required", trans, func(ut ut.Translator) error {
		return ut.Add("required", "{0} is required", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("required", fe.Field())
		return t
	})

	validate.RegisterValidation("password", validatePassword)
	validate.RegisterValidation("expirydate", ValidateDate)

	validate.RegisterTranslation("password", trans, func(ut ut.Translator) error {
		return ut.Add("password", "{0} must be at least 8 characters long, contain at least one uppercase letter and one lowercase letter and one special character and one numerical", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("password", fe.Field())
		return t
	})

	validate.RegisterTranslation("expirydate", trans, func(ut ut.Translator) error {
		return ut.Add("expirydate", "{0} must be valid yyyy-mm-dd format and greater than today ", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("expirydate", fe.Field())
		return t
	})

	validate.RegisterTranslation("min", trans, func(ut ut.Translator) error {
		return ut.Add("min", "{0} must be at least {1} characters long", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("min", fe.Field(), fe.Param())
		return t
	})
	return validate
}

func FormatValidationErrors(err error) *errors.AppError {
	var validationErrors []string
	for _, err := range err.(validator.ValidationErrors) {
		validationErrors = append(validationErrors, err.Translate(trans))
	}
	errorMsg := strings.Join(validationErrors, ", ")
	return errors.BadRequest(errorMsg)
}

func validatePassword(fl validator.FieldLevel) bool {
	password := fl.Field().String()
	var hasMinLen bool = len(password) >= 8
	var hasUpper bool
	var hasLower bool
	var hasSpecial bool
	var hasDigit bool
	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		case unicode.IsDigit(char):
			hasDigit = true
		}
	}
	return hasMinLen && hasUpper && hasSpecial && hasDigit && hasLower
}

func ValidateRequestDto(ReqDto interface{}) *errors.AppError {
	err := validate.Struct(ReqDto)
	if err != nil {
		errMessage := FormatValidationErrors(err)
		return errMessage
	}
	return nil
}

func ValidateDate(fl validator.FieldLevel) bool {
	// Date format yyyy-mm-dd
	datePattern := `^(\d{4})-(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$`
	dateRegex := regexp.MustCompile(datePattern)

	dateStr := fl.Field().String()

	if !dateRegex.MatchString(dateStr) {
		return false
	}

	// Parse the year, month, and day from the string
	year, err := strconv.Atoi(dateStr[:4])
	if err != nil {
		return false
	}
	month, err := strconv.Atoi(dateStr[5:7])
	if err != nil {
		return false
	}
	day, err := strconv.Atoi(dateStr[8:])
	if err != nil {
		return false
	}

	// Check months with 30 days
	if (month == 4 || month == 6 || month == 9 || month == 11) && day > 30 {
		return false
	}

	// Check February (leap year calculation)
	if month == 2 {
		isLeapYear := (year%4 == 0 && year%100 != 0) || (year%400 == 0)
		if isLeapYear && day > 29 {
			return false
		}
		if !isLeapYear && day > 28 {
			return false
		}
	}

	inputDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}

	today := time.Now()

	return !inputDate.Before(today)
}
