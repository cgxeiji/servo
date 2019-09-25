// +build live

package servo_test

import (
	"testing"
	"time"

	"github.com/cgxeiji/servo"
)

func init() {
	if !servo.HasBlaster() {
		panic("start pi-blaster before running the live test!")
	}
}

func TestLive(t *testing.T) {
	test, err := servo.Connect(14)
	if err != nil {
		t.Fatalf("Could not connect servo to pin 14, got:\n%v", err)
	}
	defer func() {
		test.MoveTo(90)
		test.Wait()
		test.Close()
	}()

	test.MoveTo(180)
	start := time.Now()
	test.Wait()
	elapsed := time.Since(start)

	_t := time.Duration(570) * time.Millisecond
	const tolerance = 50 * time.Millisecond
	min := _t - tolerance
	max := _t + tolerance

	if elapsed < min || elapsed > max {
		t.Errorf("it should take between %v and %v to move %.2f degrees, got: %v", min, max, 180.0, elapsed)
	}
	if test.Position() != 180 {
		t.Errorf("servo position got: %.2f, want: %.2f", test.Position(), 180.0)
	}

	time.Sleep(500 * time.Millisecond)
	test.Speed(0.1)

	test.MoveTo(0)
	test.MoveTo(90)
	test.MoveTo(0)
	test.Wait()
	if test.Position() != 0 {
		t.Errorf("servo position got: %.2f, want: %.2f", test.Position(), 0.0)
	}
	time.Sleep(500 * time.Millisecond)
}
