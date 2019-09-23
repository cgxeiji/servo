// +build live

package servo_test

import (
	"testing"

	"github.com/cgxeiji/servo"
)

func init() {
	if !HasBlaster() {
		panic("start pi-blaster before running the live test!")
	}
}

func TestLive(t *testing.T) {
	test, err := servo.Connect(14)
	if err != nil {
		t.Fatalf("Could not connect servo to pin 14, got:\n%v", err)
	}
	test.MoveTo(180)
	test.Wait()
	test.MoveTo(0)
	test.MoveTo(90)
	test.MoveTo(0)
	test.Wait()
}
