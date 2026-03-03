package validate

import (
	"testing"
)

func TestWithinLenValid(t *testing.T) {
	v := WithinLen(3, 10, "username")
	if err := v("hello"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestWithinLenAtBoundaries(t *testing.T) {
	v := WithinLen(3, 5, "field")

	// Exactly min length
	if err := v("abc"); err != nil {
		t.Errorf("min boundary: expected nil, got %v", err)
	}

	// Exactly max length
	if err := v("abcde"); err != nil {
		t.Errorf("max boundary: expected nil, got %v", err)
	}
}

func TestWithinLenTooShort(t *testing.T) {
	v := WithinLen(3, 10, "username")
	if err := v("ab"); err == nil {
		t.Error("expected error for string shorter than min")
	}
}

func TestWithinLenTooLong(t *testing.T) {
	v := WithinLen(3, 5, "username")
	if err := v("abcdef"); err == nil {
		t.Error("expected error for string longer than max")
	}
}

func TestCcnValidatorValid(t *testing.T) {
	valid := []string{
		"4111111111111111", // Visa test
		"5500000000000004", // Mastercard test
		"378282246310005",  // Amex test
	}
	for _, cc := range valid {
		if err := CcnValidator(cc); err != nil {
			t.Errorf("expected %s to be valid, got %v", cc, err)
		}
	}
}

func TestCcnValidatorInvalid(t *testing.T) {
	if err := CcnValidator("4111111111111112"); err == nil {
		t.Error("expected error for invalid Luhn number")
	}
}

func TestCcnValidatorTooShort(t *testing.T) {
	if err := CcnValidator("411111"); err == nil {
		t.Error("expected error for card number shorter than 13 digits")
	}
}

func TestCcnValidatorWithSpaces(t *testing.T) {
	if err := CcnValidator("4111 1111 1111 1111"); err != nil {
		t.Errorf("expected valid with spaces, got %v", err)
	}
}

func TestIsDigitsValid(t *testing.T) {
	v := IsDigits("zip")
	if err := v("12345"); err != nil {
		t.Errorf("expected nil for all digits, got %v", err)
	}
}

func TestIsDigitsEmpty(t *testing.T) {
	v := IsDigits("zip")
	if err := v(""); err != nil {
		t.Errorf("expected nil for empty string, got %v", err)
	}
}

func TestIsDigitsWithLetters(t *testing.T) {
	v := IsDigits("zip")
	if err := v("123abc"); err == nil {
		t.Error("expected error for string with letters")
	}
}

func TestMustBeLenCorrect(t *testing.T) {
	v := MustBeLen(5, "zip")
	if err := v("12345"); err != nil {
		t.Errorf("expected nil for correct length, got %v", err)
	}
}

func TestMustBeLenWrong(t *testing.T) {
	v := MustBeLen(5, "zip")
	if err := v("1234"); err == nil {
		t.Error("expected error for wrong length")
	}
}

func TestMustBeLenEmpty(t *testing.T) {
	v := MustBeLen(5, "zip")
	if err := v(""); err != nil {
		t.Errorf("expected nil for empty string (special case), got %v", err)
	}
}

func TestNotEmptyValid(t *testing.T) {
	v := NotEmpty("name")
	if err := v("hello"); err != nil {
		t.Errorf("expected nil for non-empty string, got %v", err)
	}
}

func TestNotEmptyInvalid(t *testing.T) {
	v := NotEmpty("name")
	if err := v(""); err == nil {
		t.Error("expected error for empty string")
	}
}

func TestEmailValidatorValid(t *testing.T) {
	valid := []string{
		"user@example.com",
		"test.user@domain.org",
		"name+tag@gmail.com",
	}
	for _, email := range valid {
		if err := EmailValidator(email); err != nil {
			t.Errorf("expected %s to be valid, got %v", email, err)
		}
	}
}

func TestEmailValidatorInvalid(t *testing.T) {
	invalid := []string{
		"notanemail",
		"@missing.user",
		"",
	}
	for _, email := range invalid {
		if err := EmailValidator(email); err == nil {
			t.Errorf("expected error for %q", email)
		}
	}
}

func TestComposeAllPass(t *testing.T) {
	v := Compose(
		NotEmpty("field"),
		WithinLen(1, 10, "field"),
	)
	if err := v("hello"); err != nil {
		t.Errorf("expected nil when all validators pass, got %v", err)
	}
}

func TestComposeFirstFails(t *testing.T) {
	v := Compose(
		NotEmpty("field"),
		WithinLen(1, 10, "field"),
	)
	err := v("")
	if err == nil {
		t.Fatal("expected error when first validator fails")
	}
	if err.Error() != "field cannot be empty" {
		t.Errorf("expected NotEmpty error message, got %q", err.Error())
	}
}

func TestComposeSecondFails(t *testing.T) {
	v := Compose(
		NotEmpty("field"),
		WithinLen(1, 3, "field"),
	)
	err := v("toolong")
	if err == nil {
		t.Fatal("expected error when second validator fails")
	}
}
