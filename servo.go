package servo

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type flag uint8

// is check if the given bits are set in the flag.
func (f flag) is(bits flag) bool {
	return f&bits != 0
}

// String implements the Stringer interface.
func (f flag) String() string {
	if f == 0 {
		return "( NONE )"
	}

	s := new(strings.Builder)

	fmt.Fprintf(s, "(")

	if f.is(Centered) {
		fmt.Fprintf(s, " Centered")
	}
	if f.is(Normalized) {
		fmt.Fprintf(s, " Normalized")
	}

	fmt.Fprintf(s, " )")

	return s.String()
}

const (
	// Centered sets the range of the servo from -90 to 90 degrees.
	// Together with Normalized, the range of the servo is set to -1 to 1.
	Centered flag = (1 << iota)
	// Normalized sets the range of the servo from 0 to 2.
	// Together with Centered, the range of the servo is set to -1 to 1.
	Normalized
)

// Servo is a struct that holds all the information necessary to control a
// servo motor. Use the function servo.Connect(gpio) for correct
// initialization.
type Servo struct {
	// GPIO is the GPIO pin number of the Raspberry Pi. Check that the pin is
	// controllable with pi-blaster.
	//
	// CAUTION: Incorrect pin assignment might cause damage to your Raspberry
	// Pi.
	GPIO int
	// Name is an optional value to assign a meaningful name to the servo.
	Name string
	// Flags is a bit flag that sets various configuration parameters.
	//
	// servo.Centered sets the range of the servo from -90 to 90 degrees.
	//
	// servo.Normalized sets the range of the servo from 0 to 2.
	// Together with servo.Centered, the range of the servo is set to -1 to 1.
	Flags flag

	target   float64
	position float64

	step, maxStep float64

	idle     bool
	finished *sync.Cond
	lock     *sync.RWMutex

	rate *rate.Limiter
}

// updateRate is set to 3ms/degree, an approximate on 0.19s/60degrees.
const updateRate = 3 * time.Millisecond

// String implements the Stringer interface.
func (s *Servo) String() string {
	return fmt.Sprintf("servo %q connected to gpio(%d) [flags: %v]", s.Name, s.GPIO, s.Flags)
}

// Connect defines a new servo connected at a GPIO pin of the Raspberry Pi. Check that the pin is
// controllable with pi-blaster.
//
// CAUTION: Incorrect pin assignment might cause damage to your Raspberry
// Pi.
func Connect(gpio int) (*Servo, error) {
	const maxS = 315.7

	s := &Servo{
		GPIO:    gpio,
		Name:    fmt.Sprintf("Servo%d", gpio),
		maxStep: maxS,
		step:    maxS,

		idle:     true,
		finished: sync.NewCond(&sync.Mutex{}),
		lock:     new(sync.RWMutex),

		rate: rate.NewLimiter(rate.Every(updateRate), 1),
	}

	return s, nil
}

// Position returns the current angle of the servo, adjusted for its Flags.
func (s *Servo) Position() float64 {
	s.lock.RLock()
	defer s.lock.RUnlock()

	p := s.position
	if s.Flags.is(Centered) {
		p -= 90
	}
	if s.Flags.is(Normalized) {
		p /= 90
	}

	return p
}

// MoveTo sets a target angle for the servo to move. The magnitude of the target
// depends on the servo's Flags. The target is automatically clamped to the set
// range.
func (s *Servo) MoveTo(target float64) {
	if s.isIdle() {
		// activate reach() only if the servo is idle and after setting the
		// target.
		defer func() {
			var wg sync.WaitGroup

			// wait until reach() has been called.
			wg.Add(1)
			go func() {
				wg.Done()
				s.reach()
			}()
			wg.Wait()
		}()
	}

	s.moveTo(target)
}

func (s *Servo) moveTo(target float64) {
	if s.Flags.is(Normalized) {
		target *= 90
	}
	if s.Flags.is(Centered) {
		target += 90
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	s.target = clamp(target, 0, 180)
}

// Stop stops moving the servo. This effectively sets the target position to
// the stopped position of the servo.
func (s *Servo) Stop() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.target = s.position
}

// reach tries to reach the assigned target.
func (s *Servo) reach() {
	if !s.isIdle() {
		panic(fmt.Errorf("%v called reach() while busy", s))
	}
	s.lock.Lock()
	s.idle = false
	s.lock.Unlock()

	for d, t := s.delta(updateRate); d != 0; d, t = s.delta(time.Since(t)) {
		s.lock.Lock()
		s.position += d
		s.lock.Unlock()
		s.rate.Wait(context.Background())
	}
	s.lock.Lock()
	s.position = s.target
	s.idle = true
	s.lock.Unlock()

	s.finished.L.Lock()
	s.finished.Broadcast()
	s.finished.L.Unlock()
}

// delta returns the difference between the target and position.
func (s *Servo) delta(deltaT time.Duration) (float64, time.Time) {
	t := time.Now()

	s.lock.RLock()
	step := s.step * deltaT.Seconds()
	d := s.target - s.position
	s.lock.RUnlock()

	if d <= step {
		if -d <= step {
			return 0, t
		}
		return -step, t
	}

	return step, t
}

// isIdle checks if the servo is not moving.
func (s *Servo) isIdle() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.idle
}

// Wait waits for the servo to stop moving.
func (s *Servo) Wait() {
	s.finished.L.Lock()
	defer s.finished.L.Unlock()

	for !s.isIdle() {
		s.finished.Wait()
	}
}

func clamp(value, min, max float64) float64 {
	if value < min {
		value = min
	}
	if value > max {
		value = max
	}
	return value
}
