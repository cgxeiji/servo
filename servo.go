package servo

import (
	"fmt"
	"strings"
	"sync"
	"time"
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
// initialization. Servo is designed to be concurrent-safe.
type Servo struct {
	// pin is the GPIO pin number of the Raspberry Pi. Check that the pin is
	// controllable with pi-blaster.
	//
	// CAUTION: Incorrect pin assignment might cause damage to your Raspberry
	// Pi.
	pin gpio
	// Name is an optional value to assign a meaningful name to the servo.
	Name string
	// Flags is a bit flag that sets various configuration parameters.
	//
	// servo.Centered sets the range of the servo from -90 to 90 degrees.
	//
	// servo.Normalized sets the range of the servo from 0 to 2.
	// Together with servo.Centered, the range of the servo is set to -1 to 1.
	Flags flag

	// These calibration variables should be immutables once initialized.
	minPulse, maxPulse float64
	lastPWM            pwm

	start, target, position float64
	done                    chan struct{}
	startT                  time.Time

	step, maxStep float64

	idle     bool
	finished *sync.Cond
	lock     *sync.RWMutex
}

// updateRate is set to 3ms/degree, an approximate on 0.19s/60degrees.

// String implements the Stringer interface.
// It returns a string in the following format:
//
// servo "NAME" connected to gpio(GPIO_PIN) [flags: ( FLAGS_SET )]
//
// where NAME is the verbose name (default: fmt.Sprintf("Servo%d", GPIO)),
// GPIO_PIN is the connection pin of the servo, and FLAGS_SET is the list of
// flags set (default: NONE).
func (s *Servo) String() string {
	return fmt.Sprintf("servo %q connected to gpio(%d) [flags: %v]", s.Name, s.pin, s.Flags)
}

// Connect defines a new servo connected at a GPIO pin of the Raspberry Pi. Check that the pin is
// controllable with pi-blaster.
//
// CAUTION: Incorrect pin assignment might cause damage to your Raspberry
// Pi.
func Connect(GPIO int) (*Servo, error) {
	const maxS = 315.7

	s := &Servo{
		pin:      gpio(GPIO),
		Name:     fmt.Sprintf("Servo%d", GPIO),
		maxStep:  maxS,
		step:     maxS,
		minPulse: 0.05,
		maxPulse: 0.25,

		idle:     true,
		finished: sync.NewCond(&sync.Mutex{}),
		lock:     new(sync.RWMutex),

		done: make(chan struct{}),
	}

	_blaster.subscribe(s)

	return s, nil
}

// Close cleans up the state of the servo and deactivates the corresponding
// GPIO pin.
func (s *Servo) Close() {
	_blaster.unsubscribe(s)
	close(s.done)
	_blaster.write(fmt.Sprintf("%d=%.2f", s.pin, 0.0))
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
// range. If called concurrently, the target position is overridden by the last
// goroutine (usually non-deterministic).
func (s *Servo) MoveTo(target float64) {
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

	if s.step == 0.0 {
		s.target = s.position
	} else {
		s.target = clamp(target, 0, 180)
	}
	s.start = s.position
	s.startT = time.Now()
	s.idle = false
}

// Speed changes the speed of the servo from (still) 0.0 to 1.0 (max speed).
// Setting a speed of 0.0 effectively sets the target position to the current
// position and the servo will not move.
func (s *Servo) Speed(percentage float64) {
	s.lock.Lock()
	defer s.lock.Unlock()

	percentage = clamp(percentage, 0.0, 1.0)
	s.step = s.maxStep * percentage
}

// Stop stops moving the servo. This effectively sets the target position to
// the stopped position of the servo.
func (s *Servo) Stop() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.target = s.position
	s.idle = true
	s.finished.L.Lock()
	s.finished.Broadcast()
	s.finished.L.Unlock()
}

// pwm linearly interpolates an angle based on the start, finish, and
// duration of the movement, and returns the gpio pin and adjusted pwm for the
// current time.
func (s *Servo) pwm() (gpio, pwm) {
	ok := false
	s.lock.RLock()
	p := s.position
	_pwm := s.lastPWM

	defer func() {
		if !ok {
			s.lock.Lock()
			s.position = p
			s.lastPWM = _pwm

			if p == s.target {
				s.idle = true
				s.finished.L.Lock()
				s.finished.Broadcast()
				s.finished.L.Unlock()
			}
			s.lock.Unlock()
		}
	}()
	defer s.lock.RUnlock()

	if s.position == s.target {
		ok = true
		return s.pin, _pwm
	}

	delta := time.Since(s.startT).Seconds() * s.step
	if s.target < s.start {
		p = s.start - delta
		if p <= s.target {
			p = s.target
		}
	} else {
		p = s.start + delta
		if p >= s.target {
			p = s.target
		}
	}

	_pwm = pwm(remap(p, 0, 180, s.minPulse, s.maxPulse))

	return s.pin, _pwm
}

// isIdle checks if the servo is not moving.
func (s *Servo) isIdle() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.idle
}

// Wait waits for the servo to stop moving. It is concurrent-safe.
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

func remap(value, min, max, toMin, toMax float64) float64 {
	return (value-min)/(max-min)*(toMax-toMin) + toMin
}
