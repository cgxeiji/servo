// +build !live

package servo

import (
	"testing"
)

func TestInit(t *testing.T) {
	if _blaster == nil {
		t.Fatal("_blaster was not initialized")
	}
}

func TestHasBlaster(t *testing.T) {
	if hasBlaster() {
		t.Error("pi-blaster was found running during test")
	}
}

func TestNoPiBlaster(t *testing.T) {
	noPiBlaster()
	if !_blaster.disabled {
		t.Error("NoPiBlaster() could not disable _blaster")
	}
}
