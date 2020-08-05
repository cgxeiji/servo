// +build !live

package servo_test

import (
	"sync"
	"testing"
	"time"

	"github.com/cgxeiji/servo"
)

func initServo(t *testing.T) *servo.Servo {
	s, err := servo.Connect(99)
	if err != nil {
		t.Fatal(err)
	}

	s.Name = "Tester"
	return s
}

func TestExportConnect(t *testing.T) {
	s := initServo(t)
	defer s.Close()

	want := `servo "Tester" connected to gpio(99) [flags: ( NONE )]`
	got := s.String()

	if got != want {
		t.Errorf("error connecting servo\ngot:\n%v\nwant:\n%v", got, want)
	}
}

func TestExportServo_MoveTo(t *testing.T) {
	s := initServo(t)
	defer s.Close()

	var wg sync.WaitGroup

	// Move to 180 degrees and wait until finished.
	degrees := 180.0
	s.MoveTo(degrees)

	wg.Add(1)
	// Test a concurrent waiter.
	go func() {
		defer wg.Done()
		s.Wait()
	}()

	start := time.Now()
	s.Wait()
	elapsed := time.Since(start)

	_t := time.Duration(degrees/315.7*1000) * time.Millisecond
	const tolerance = 50 * time.Millisecond
	min := _t - tolerance
	max := _t + tolerance

	if elapsed < min || elapsed > max {
		t.Errorf("it should take between %v and %v to move %.2f degrees, got: %v", min, max, degrees, elapsed)
	}

	got := s.Position()
	if got != degrees {
		t.Errorf("did not move to %.2f, got: %.2f", degrees, got)
	}

	wg.Wait()
}
