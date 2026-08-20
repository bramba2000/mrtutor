package validation_test

import (
	"testing"
	"time"

	"github.com/bramba2000/mrtutor/backend/validation"
)

func TestMin(t *testing.T) {
	v := validation.Min(4)
	if err := v(5); err != nil {
		t.Errorf("Min(4)(5) = %v, want nil", err)
	}
	if err := v(4); err != nil {
		t.Errorf("Min(4)(4) = %v, want nil", err)
	}
	if err := v(3); err == nil {
		t.Errorf("Min(4)(3) = nil, want error")
	}
}

func TestMinDuration(t *testing.T) {
	v := validation.Min(time.Duration(0))
	if err := v(0); err != nil {
		t.Errorf("Min(0)(0) = %v, want nil", err)
	}
	if err := v(-time.Second); err == nil {
		t.Errorf("Min(0)(-1s) = nil, want error")
	}
}

func TestMax(t *testing.T) {
	v := validation.Max(10)
	if err := v(9); err != nil {
		t.Errorf("Max(10)(9) = %v, want nil", err)
	}
	if err := v(10); err != nil {
		t.Errorf("Max(10)(10) = %v, want nil", err)
	}
	if err := v(11); err == nil {
		t.Errorf("Max(10)(11) = nil, want error")
	}
}

func TestMinMax(t *testing.T) {
	v := validation.MinMax(0, 65535)
	if err := v(0); err != nil {
		t.Errorf("MinMax(0,65535)(0) = %v, want nil", err)
	}
	if err := v(65535); err != nil {
		t.Errorf("MinMax(0,65535)(65535) = %v, want nil", err)
	}
	if err := v(-1); err == nil {
		t.Errorf("MinMax(0,65535)(-1) = nil, want error")
	}
	if err := v(65536); err == nil {
		t.Errorf("MinMax(0,65535)(65536) = nil, want error")
	}
}

type testMode string

const (
	testModeDev  testMode = "dev"
	testModeProd testMode = "prod"
)

func TestOneOf(t *testing.T) {
	v := validation.OneOf(testModeDev, testModeProd)
	if err := v(testModeDev); err != nil {
		t.Errorf("OneOf(dev,prod)(dev) = %v, want nil", err)
	}
	if err := v(testModeProd); err != nil {
		t.Errorf("OneOf(dev,prod)(prod) = %v, want nil", err)
	}
	if err := v(testMode("staging")); err == nil {
		t.Errorf("OneOf(dev,prod)(staging) = nil, want error")
	}
}

func TestOneOfEmpty(t *testing.T) {
	v := validation.OneOf[int]()
	if err := v(1); err == nil {
		t.Errorf("OneOf()(1) = nil, want error")
	}
}
