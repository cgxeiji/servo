// +build !live

package servo

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func init() {
	if hasBlaster() {
		fmt.Println("ignoring pi-blaster")
		noPiBlaster()
	}
	fmt.Printf("\n^ Ignore the previous warning during tests ^\n\n")
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
	s, cl, err := Connect(gpio)
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	if s.pin != gpio {
		t.Errorf("GPIO does not match, got: %d, want: %d", s.pin, gpio)
	}
	name := fmt.Sprintf("Servo%d", gpio)
	if s.Name != name {
		t.Errorf("Name does not match, got: %q, want: %q", s.Name, name)
	}
}

func TestServo_Position(t *testing.T) {
	s, cl, err := Connect(99)
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	const want = 0.0 //59.6
	s.position <- want
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

	s, cl, err := Connect(99)
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	for input, want := range tests {
		s.moveTo(input)
		got := <-s.target
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
	s, cl, err := Connect(99)
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	done := make(chan struct{})

	// Move to 180 degrees, but override concurrently to 0 when it reaches 110
	// degrees.
	s.moveTo(180)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		s.Wait()
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
	n := 100
	degrees := 2.0
	servos := make([]*Servo, 0, n)

	for i := 0; i < n; i++ {
		s, cl, err := Connect(i)
		if err != nil {
			b.Fatalf("servos[%d] -> %v", i, err)
		}
		defer cl()
		servos = append(servos, s)
	}

	var wg sync.WaitGroup
	wg.Add(n)

	b.Logf("This benchmark should read aprox %.0f ns/op", 0.19/60.0*degrees*float64(time.Second))

	b.ResetTimer()
	for j := 0; j < n; j++ {
		go func(j int) {
			defer wg.Done()

			for i := 0; i < b.N; i++ {
				servos[j].position <- 0
				servos[j].moveTo(degrees)
				servos[j].Wait()
			}
		}(j)
	}
	wg.Wait()
}

func BenchmarkServo_PWM(b *testing.B) {
	servo, cl, err := Connect(1)
	if err != nil {
		b.Fatalf("%v -> %v", servo, err)
	}
	defer cl()

	servo.position <- 0
	servo.moveTo(180)

	var wg sync.WaitGroup
	wg.Add(100)

	b.ResetTimer()
	for j := 0; j < 100; j++ {
		go func(j int) {
			defer wg.Done()

			for i := 0; i < b.N; i++ {
				servo.pwm()
			}
		}(j)
	}
	wg.Wait()

}

func TestServo_Stop(t *testing.T) {
	s, cl, err := Connect(99)
	if err != nil {
		t.Fatal(err)
	}
	defer cl()
	done := make(chan struct{})

	// Move to 180 degrees, but override concurrently to 0 when it reaches 110
	// degrees.
	s.moveTo(180)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		s.Wait()
	}()

	wg.Add(1)
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
	s, cl, err := Connect(99)
	if err != nil {
		t.Fatal(err)
	}
	defer cl()

	// Move to 180 degrees and wait until finished.
	degrees := 180.0
	s.moveTo(degrees)

	var wg sync.WaitGroup

	wg.Add(1)
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
