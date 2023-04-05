package security_test

import (
	"testing"
	"time"

	"github.com/machbase/neo-server/mods/service/security"
)

func TestOtpForMqtt(t *testing.T) {
	time.Now().Format("2006-01-02 15:04:05")
	secret := []byte("SECRET")
	builder := security.DefaultBuilder()
	builder.Secret = secret
	builder.PeriodSeconds = 60
	builder.GeneratorType = security.GeneratorHex12

	// generate
	gen, err := builder.Build()
	if err != nil {
		t.Fatalf("NewGenerator failed, %s", err)
	}

	code, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate failed, %s", err)
	}
	t.Logf("OTP: %s", code)
}

func TestTOTP(t *testing.T) {
	secret := []byte("SECRET")
	period := 2
	skew := []int{-1, -2, 1}

	genTypes := []security.GeneratorType{security.GeneratorDigit6, security.GeneratorDigit8, security.GeneratorHex12}

	for _, gt := range genTypes {
		// builder
		builder := security.DefaultBuilder()
		builder.Secret = secret
		builder.PeriodSeconds = period
		builder.Skew = skew
		builder.GeneratorType = gt

		// generate
		gen, err := builder.Build()
		if err != nil {
			t.Fatalf("NewGenerator failed, %s", err)
		}
		code, err := gen.Generate()
		if err != nil {
			t.Fatalf("Generate failed, %s", err)
		}
		t.Logf("OTP: %s", code)

		// validate - expect ok
		chk, _ := builder.Build()
		expectOk, err := chk.Validate(code)
		if err != nil {
			t.Fatalf("Validate failed, %s", err)
		}
		if !expectOk {
			t.Fatalf("Validate falsed")
		}

		// validate - expect !ok by expired
		time.Sleep(time.Duration(period*3) * time.Second)

		fck, _ := builder.Build()
		expectKo, err := fck.Validate(code)
		if err != nil {
			t.Fatalf("Validate failed, %s", err)
		}
		if expectKo {
			t.Fatalf("Validate true, expected false")
		}
	}

}
