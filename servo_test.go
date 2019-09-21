package servo

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func init() {
	if hasBlaster() {
		panic("stop pi-blaster when running tests!")
	}
}

func TestServo(t *testing.T) {
	s := &Servo{
		Flags: Centered | Normalized,
	}

	if !s.Flags.is(Centered) {
		t.Error("Flags was not set to Centered")
	}
	if !s.Flags.is(Normalized) {
		t.Error("Flags was not set to Normalized")
	}
}

func TestConnect(t *testing.T) {
	const gpio = 99
	s, err := Connect(gpio)
	if err != nil {
		t.Fatal(err)
	}

	if s.GPIO != gpio {
		t.Errorf("GPIO does not match, got: %d, want: %d", s.GPIO, gpio)
	}
	name := fmt.Sprintf("Servo%d", gpio)
	if s.Name != name {
		t.Errorf("Name does not match, got: %q, want: %q", s.Name, name)
	}
}

func TestServo_Position(t *testing.T) {
	s, err := Connect(99)
	if err != nil {
		t.Fatal(err)
	}

	const want = 59.6
	s.position = want
	got := s.Position()
	if got != want {
		t.Errorf("positions do not match, got: %.2f, want: %.2f", got, want)
	}

	t.Run("Centered", func(t *testing.T) {
		s.Flags = Centered
		got := s.Position()
		if got != want-90 {
			t.Errorf("positions do not match, got: %.2f, want: %.2f", got, want-90)
		}
	})

	t.Run("Normalized", func(t *testing.T) {
		s.Flags = Normalized
		got := s.Position()
		if got != want/90 {
			t.Errorf("positions do not match, got: %.2f, want: %.2f", got, want/90)
		}
	})

	t.Run("Centered | Normalized", func(t *testing.T) {
		s.Flags = Centered | Normalized
		got := s.Position()
		if got != (want-90)/90 {
			t.Errorf("positions do not match, got: %.2f, want: %.2f", got, (want-90)/90)
		}
	})
}

func TestServo_MoveTo(t *testing.T) {
	// map[input]want
	tests := map[float64]float64{
		0:    0,
		10:   10,
		200:  180,
		-200: 0,
	}

	s, err := Connect(99)
	if err != nil {
		t.Fatal(err)
	}

	for input, want := range tests {
		s.moveTo(input)
		got := s.target
		if got != want {
			t.Errorf("Servo.moveTo(%.2f) -> got: %.2f, want: %.2f", input, got, want)
		}
	}

	t.Run("Concurrent", func(t *testing.T) {
		var wg sync.WaitGroup

		wg.Add(5)
		for i := 0; i < 5; i++ {
			go func(i int) {
				defer wg.Done()
				for j := 0; j < 30; j++ {
					s.moveTo(float64(i + j))
				}
			}(i)

		}
		wg.Wait()
	})
}

func TestServo_Reach(t *testing.T) {
	s, err := Connect(99)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})

	// Move to 180 degrees, but override concurrently to 0 when it reaches 110
	// degrees.
	s.moveTo(180)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		s.reach()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		b := true
		for {
			select {
			case <-done:
				want := 0.0
				got := s.Position()
				if got != want {
					t.Errorf("Servo.moveTo(%.2f) -> got: %.2f, want: %.2f", 0.0, got, want)
				}
				return
			default:
				if b && s.Position() >= 110 {
					s.moveTo(0)
					b = false
				}
			}
		}
	}()

	<-done
	wg.Wait()
}

func BenchmarkServo_Reach(b *testing.B) {
	s, err := Connect(99)
	if err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		s.position = 0
		s.moveTo(1) // should take 3ms at max speed
		s.reach()
	}
}

func TestServo_Stop(t *testing.T) {
	s, err := Connect(99)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})

	// Move to 180 degrees, but override concurrently to 0 when it reaches 110
	// degrees.
	s.moveTo(180)

	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		defer close(done)
		s.reach()
	}()

	go func() {
		defer wg.Done()
		b := true
		for {
			select {
			case <-done:
				got := s.Position()
				if got == 180 {
					t.Errorf("Servo.Stop() failed to stop -> got: %.2f", got)
				}
				t.Logf("Servo.Stop() stopped at: %.2f (requested: %.2f)", got, 110.0)
				return
			default:
				if b && s.Position() >= 110 {
					s.Stop()
					b = false
				}
			}
		}
	}()

	<-done
	wg.Wait()
}

func TestServo_Wait(t *testing.T) {
	s, err := Connect(99)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})

	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		<-done
		s.reach()
	}()

	// Move to 180 degrees and wait until finished.
	degrees := 180.0
	s.moveTo(degrees)
	done <- struct{}{}

	// Test a concurrent waiter.
	go func() {
		defer wg.Done()
		s.Wait()
	}()

	start := time.Now()
	s.Wait()
	elapsed := time.Since(start)

	_t := time.Duration(degrees/s.step*1000) * time.Millisecond
	const tolerance = 50 * time.Millisecond
	min := _t - tolerance
	max := _t + tolerance
	if elapsed < min || elapsed > max {
		t.Errorf("it should take between %v and %v to move %.2f degrees, got: %v", min, max, degrees, elapsed)
	}

	wg.Wait()
}

func TestStress(t *testing.T) {
	degrees := 180.0
	_t := time.Duration(degrees/315.7*1000) * time.Millisecond
	const tolerance = 50 * time.Millisecond
	min := _t - tolerance
	max := _t + tolerance

	for n := 100; n <= 10000; n *= 10 {
		t.Run(fmt.Sprintf("%dServos", n), func(t *testing.T) {
			servos := make([]*Servo, 0, n)
			times := make([]time.Duration, 0, n)

			for i := 0; i < n; i++ {
				s, err := Connect(i)
				if err != nil {
					t.Fatalf("servos[%d] -> %v", i, err)
				}
				servos = append(servos, s)
			}

			var wg sync.WaitGroup
			timeout := make(chan time.Duration)

			wg.Add(n)

			for i := 0; i < n; i++ {
				go func(i int) {
					defer wg.Done()
					servos[i].moveTo(180)
					start := time.Now()
					servos[i].reach()
					elapsed := time.Since(start)
					if elapsed < min || elapsed > max {
						timeout <- elapsed
					}
				}(i)
			}

			go func() {
				wg.Wait()
				close(timeout)
			}()

			for t := range timeout {
				times = append(times, t)
			}

			fn := len(times)

			if fn != 0 {
				sum := time.Duration(0)
				for _, t := range times {
					sum += t
				}
				mean := sum / time.Duration(fn)

				ratio := float64(fn) / float64(n)
				if ratio > 0.2 {
					t.Errorf("%d out of %d (%.2f%%) servos failed with a mean time of %v (it should take between %v and %v to move %.2f degrees)",
						fn, n, ratio*100.0, mean, min, max, degrees)
				} else {
					t.Logf("%d out of %d (%.2f%%) servos failed with a mean time of %v (it should take between %v and %v to move %.2f degrees)",
						fn, n, ratio*100.0, mean, min, max, degrees)
				}
			}
		})
	}
}

func TestClamp(t *testing.T) {
	// map[input]want
	tests := map[float64]float64{
		0:   0,
		10:  1,
		-10: -1,
		0.5: 0.5,
	}

	for input, want := range tests {
		got := clamp(input, -1, 1)
		if got != want {
			t.Errorf("clam(%.2f, -1, 1) -> got: %.2f, want: %.2f", input, got, want)
		}
	}
}
