package seed

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestLocalSecretsGenerateCodes(t *testing.T) {
	for _, secret := range []string{EditorTOTP, ReviewerTOTP, ManagerTOTP} {
		code, err := totp.GenerateCode(secret, time.Now())
		if err != nil || !totp.Validate(code, secret) {
			t.Fatalf("%s: %v", secret, err)
		}
	}
}
