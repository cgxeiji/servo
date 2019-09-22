// +build live

package servo_test

import (
	"testing"

	"github.com/cgxeiji/servo"
)

func init() {
	if !hasBlaster() {
		panic("start pi-blaster before running the live test!")
	}
}

func TestLive(t *testing.T) {
	test, err := servo.Connect(14)
	if err != nil {
		t.Fatalf("Could not connect servo to pin 14, got:\n%v", err)
	}
	servo.MoveTo(180)
	servo.Wait()
	servo.MoveTo(0)
	servo.MoveTo(90)
	servo.MoveTo(0)
	servo.Wait()
}
