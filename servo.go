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

	// MinPulse is the minimum pwm pulse of the servo. (default 0.05 s)
	// MaxPulse is the maximum pwm pulse of the servo. (default 0.25 s)
	// These calibration variables should be immutables once the servo is
	// connected..
	MinPulse, MaxPulse float64

	target, position float64
	done             chan struct{}
	deltaT           time.Time
	lastPWM          pwm

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

// New creates a new Servo struct with default values, connected at a GPIO pin
// of the Raspberry Pi. You should check that the pin is controllable with pi-blaster.
//
// CAUTION: Incorrect pin assignment might cause damage to your Raspberry
// Pi.
func New(GPIO int) (s *Servo) {
	// maxS is the maximun degrees/s for a tipical servo of speed
	// 0.19s/60degrees.
	const maxS = 315.7

	s = &Servo{
		pin:      gpio(GPIO),
		Name:     fmt.Sprintf("Servo%d", GPIO),
		maxStep:  maxS,
		step:     maxS,
		MinPulse: 0.05,
		MaxPulse: 0.25,

		idle:     true,
		finished: sync.NewCond(&sync.Mutex{}),
		lock:     new(sync.RWMutex),

		done: make(chan struct{}),
	}

	return s
}

// Connect connects the servo to the pi-blaster daemon.
func (s *Servo) Connect() error {
	_blaster.subscribe(s)

	return nil
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

// Waiter implements the Wait function.
type Waiter interface {
	// Wait waits for the servo to finish moving.
	Wait()
}

// MoveTo sets a target angle for the servo to move. The magnitude of the target
// depends on the servo's Flags. The target is automatically clamped to the set
// range. If called concurrently, the target position is overridden by the last
// goroutine (usually non-deterministic).
func (s *Servo) MoveTo(target float64) (wait Waiter) {
	s.moveTo(target)
	return s
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
	s.deltaT = time.Now()
	s.idle = false
}

// SetSpeed changes the speed of the servo from (still) 0.0 to 1.0 (max speed).
// Setting a speed of 0.0 effectively sets the target position to the current
// position and the servo will not move.
func (s *Servo) SetSpeed(percentage float64) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.step = s.maxStep * clamp(percentage, 0.0, 1.0)
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

// SetPosition immediately sets the angle the servo.
func (s *Servo) SetPosition(position float64) {
	if s.Flags.is(Normalized) {
		position *= 90
	}
	if s.Flags.is(Centered) {
		position += 90
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	s.position = clamp(position, 0, 180)
	s.target = s.position
	s.idle = false
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
			s.deltaT = time.Now()

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

	if s.position == s.target && s.idle {
		ok = true
		return s.pin, _pwm
	}

	delta := time.Since(s.deltaT).Seconds() * s.step
	if s.target < s.position {
		p = s.position - delta
		if p <= s.target {
			p = s.target
		}
	} else {
		p = s.position + delta
		if p >= s.target {
			p = s.target
		}
	}

	_pwm = pwm(remap(p, 0, 180, s.MinPulse, s.MaxPulse))

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
