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

	position, target chan float64
	pulse            chan pwm

	step, maxStep float64
	speed         chan float64

	idle     chan bool
	unlock   chan bool
	finished *sync.Cond

	wait func()
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
func Connect(GPIO int) (*Servo, func(), error) {
	const maxS = 315.7

	s := &Servo{
		pin:      gpio(GPIO),
		Name:     fmt.Sprintf("Servo%d", GPIO),
		maxStep:  maxS,
		step:     maxS,
		minPulse: 0.05,
		maxPulse: 0.25,

		idle:     make(chan bool),
		unlock:   make(chan bool),
		finished: sync.NewCond(&sync.Mutex{}),

		position: make(chan float64),
		target:   make(chan float64),

		pulse: make(chan pwm),

		speed: make(chan float64),
	}

	done := make(chan struct{})
	go func() {
		var (
			position, target float64
			pulse            pwm
			startT           time.Time
			idle             bool
		)

		for {
			select {
			case <-done:
				return
			case s.idle <- idle:
			case s.position <- position:
			case position = <-s.position:
			case s.target <- target:
			case s.step = <-s.speed:
			case target = <-s.target:
				idle = false
				startT = time.Now()
			case s.pulse <- pulse:
				t := startT
				startT = time.Now()
				pulse, position, idle = s.getPulse(position, target, t)
			}
		}
	}()

	_blaster.subscribe(s)

	closeFunc := func() {
		_blaster.unsubscribe(s)
		close(done)
		_blaster.write(fmt.Sprintf("%d=%.2f", s.pin, 0.0))
	}

	return s, closeFunc, nil
}

func (s *Servo) getPulse(p, t float64, sT time.Time) (pwm, float64, bool) {
	idle := false
	if p != t {
		delta := time.Since(sT).Seconds() * s.step
		if t < p {
			p -= delta
			if p < t {
				p = t
				idle = true
			}
		} else {
			p += delta
			if p > t {
				p = t
				idle = true
			}
		}
	} else {
		idle = true
	}

	if idle {
		s.finished.L.Lock()
		s.finished.Broadcast()
		s.finished.L.Unlock()
	}

	pulse := pwm(remap(p, 0, 180, s.minPulse, s.maxPulse))

	return pulse, p, idle
}

func (s *Servo) pwm() (gpio, pwm) {
	pulse := <-s.pulse
	/*
		if s.pin == 99 {
			fmt.Println(s, pulse, <-s.position, <-s.target)
		}
	*/
	return s.pin, pulse
}

// Position returns the current angle of the servo, adjusted for its Flags.
func (s *Servo) Position() float64 {
	p := <-s.position

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

	if s.step == 0.0 {
		s.target <- <-s.position
	} else {
		select {
		case <-s.unlock:
			s.unlock = make(chan bool)
		default:
		}
		s.target <- clamp(target, 0, 180)
	}

	s.wait = func() {
		<-s.unlock
	}
}

// Speed changes the speed of the servo from (still) 0.0 to 1.0 (max speed).
// Setting a speed of 0.0 effectively sets the target position to the current
// position and the servo will not move.
func (s *Servo) Speed(percentage float64) {
	percentage = clamp(percentage, 0.0, 1.0)
	s.speed <- s.maxStep * percentage
}

// Stop stops moving the servo. This effectively sets the target position to
// the stopped position of the servo.
func (s *Servo) Stop() {
	s.target <- <-s.position
}

// isIdle checks if the servo is not moving.
func (s *Servo) isIdle() bool {
	return <-s.idle
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
