package security

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"math"
	"time"
)

type GeneratorType int

const (
	GeneratorDigit6 GeneratorType = iota
	GeneratorDigit8
	GeneratorHex12
)

const opt_secret string = "a1bbb_dhg_tmsl__aoo_clnsae"

type Generator interface {
	Start() error
	Stop()
	Generate() (string, error)
	Validate(pass string) (bool, error)
}

type Builder struct {
	Secret        []byte
	PeriodSeconds int
	Skew          []int
	OffsetFromEnd int
	GeneratorType GeneratorType
}

func DefaultBuilder() *Builder {
	return &Builder{
		PeriodSeconds: 60,
		Skew:          []int{-1, 1},
		OffsetFromEnd: 2,
		GeneratorType: GeneratorHex12,
	}
}

func (builder *Builder) Build() (Generator, error) {
	gen := &generator{
		secret:        builder.Secret,
		periodSeconds: builder.PeriodSeconds,
		skew:          builder.Skew,
		hashFunc:      sha512.New,
		offsetFromEnd: builder.OffsetFromEnd,
		generatorType: builder.GeneratorType,
	}

	return gen, nil
}

type generator struct {
	Generator
	secret        []byte
	periodSeconds int
	skew          []int
	hashFunc      func() hash.Hash
	offsetFromEnd int
	generatorType GeneratorType
}

func NewGenerator(secret []byte, period int, skew []int, genType GeneratorType) (Generator, error) {
	gen := &generator{
		secret:        secret,
		periodSeconds: period,
		skew:          skew,
		hashFunc:      sha512.New,
		offsetFromEnd: 2,
		generatorType: genType,
	}

	return gen, nil
}

func (gen *generator) Start() error {
	// boot implement
	return nil
}

func (gen *generator) Stop() {
	// boot implement
}

func (gen *generator) Generate() (string, error) {
	tick := time.Now()

	counter := uint64(math.Floor(float64(tick.Unix()) / float64(gen.periodSeconds)))

	code, err := _generateCode(gen.secret, counter, gen.hashFunc, gen.generatorType,
		gen.offsetFromEnd, false)
	return code, err
}

func (gen *generator) Validate(pass string) (bool, error) {
	var counters []uint64

	tick := time.Now()
	counter := int(math.Floor(float64(tick.Unix()) / float64(gen.periodSeconds)))
	counters = append(counters, uint64(counter))

	for _, s := range gen.skew {
		counters = append(counters, uint64(counter+s))
	}

	for _, counter := range counters {
		code, err := _generateCode(gen.secret, counter, gen.hashFunc, gen.generatorType, gen.offsetFromEnd, false)
		if err != nil {
			return false, err
		}
		if code == pass {
			return true, nil
		}
	}

	return false, nil
}

func _generateCode(secret []byte, counter uint64, hashFunc func() hash.Hash, genType GeneratorType,
	offsetFromEnd int, debug bool) (string, error) {
	if len(secret) == 0 {
		secret = []byte(opt_secret) // secret 할당 없이 / 길이가 0으로 호출되는 경우
	}
	buf := make([]byte, 12)
	mac := hmac.New(hashFunc, secret)

	binary.BigEndian.PutUint64(buf, counter)
	if debug {
		fmt.Printf("counter=%v\n", counter)
		fmt.Printf("buf=%v\n", buf)
	}

	mac.Write(buf)
	hmacResult := mac.Sum(nil)

	if debug {
		fmt.Println(hex.Dump(hmacResult))
	}

	// "Dynamic truncation" in RFC 4226
	// http://tools.ietf.org/html/rfc4226#section-5.4
	offset := hmacResult[len(hmacResult)-offsetFromEnd] & 0xf

	if genType == GeneratorHex12 {
		value := int64(((int(hmacResult[offset]) & 0xff) << 40) |
			((int(hmacResult[offset+1]) & 0xff) << 32) |
			((int(hmacResult[offset+2]) & 0xff) << 24) |
			((int(hmacResult[offset+3]) & 0xff) << 16) |
			((int(hmacResult[offset+4]) & 0xff) << 8) |
			(int(hmacResult[offset+5]) & 0xff))
		return fmt.Sprintf("%012x", value), nil
	} else {
		length := 6
		if genType == GeneratorDigit8 {
			length = 8
		}

		value := int64(((int(hmacResult[offset]) & 0x7f) << 24) |
			((int(hmacResult[offset+1] & 0xff)) << 16) |
			((int(hmacResult[offset+2] & 0xff)) << 8) |
			(int(hmacResult[offset+3]) & 0xff))
		mod := int32(value % int64(math.Pow10(length)))

		if debug {
			fmt.Printf("offset = %v\n", offset)
			fmt.Printf("value  = %v  %x\n", value, value)
			fmt.Printf("mod'ed = %v\n", mod)
		}
		f := fmt.Sprintf("%%0%dd", length)
		return fmt.Sprintf(f, mod), nil
	}

}
