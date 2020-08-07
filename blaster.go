package servo

import (
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"
)

type blaster struct {
	disabled bool
	buffer   chan string
	done     chan struct{}
	servos   chan servoPkg
	_servos  map[gpio]*Servo

	rate time.Duration
}

var _blaster *blaster

type gpio int
type pwm float64

type servoPkg struct {
	servo *Servo
	add   bool
}

func init() {
	_blaster = &blaster{
		buffer:  make(chan string),
		done:    make(chan struct{}),
		servos:  make(chan servoPkg),
		rate:    40 * time.Millisecond,
		_servos: make(map[gpio]*Servo),
	}

	if err := _blaster.start(); err != nil {
		if err == errPiBlasterNotFound {
			log.Println("WARNING:", err, "\n\t(servo will continue with pi-blaster disabled)")
			noPiBlaster()
			if err := _blaster.start(); err != nil {
				panic(err)
			}
		} else {
			panic(err)
		}
	}
}

// noPiBlaster stops this package from sending text to /dev/pi-blaster. Useful
// for debugging in devices without pi-blaster installed.
func noPiBlaster() {
	_blaster.disabled = true
}

// hasBlaster checks if pi-blaster is running in the system. It depends on
// /bin/sh and pgrep.
func hasBlaster() bool {
	cmd := exec.Command("/bin/sh", "-c", "pgrep pi-blaster")
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

var (
	// errPiBlasterNotFound is thrown when an instance of pi-blaster could not
	// be found on the system.
	errPiBlasterNotFound = fmt.Errorf("pi-blaster was not found running: start pi-blaster to avoid this error")
)

// start runs a goroutine to send data to pi-blaster. If NoPiBlaster was
// called, the data is sent to ioutil.Discard.
func (b *blaster) start() error {
	if !b.disabled && !hasBlaster() {
		return errPiBlasterNotFound
	}

	b.manager(b.done, time.NewTicker(b.rate).C)

	return nil
}

// manager keeps track of changes to servos and flushes the data to pi-blaster
// given the flushCh signal. The flush will happen only if there was a change
// in the servos data. Everytime the data is flushed, the variable is emptied.
func (b *blaster) manager(done <-chan struct{}, flushCh <-chan time.Time) {
	data := make(map[gpio]pwm)

	updateCh := time.NewTicker(3 * time.Millisecond)

	go func() {
		for {
			select {
			case <-done:
				return
			case pkg := <-b.servos:
				servo := pkg.servo
				if pkg.add {
					b._servos[servo.pin] = servo
				} else {
					delete(b._servos, servo.pin)
				}
				updateCh.Stop()
				factor := math.Log10(float64(len(b._servos)+1))*3 + 1
				updateCh = time.NewTicker(time.Duration(factor) * 3 * time.Millisecond)
			case <-updateCh.C:
				for _, servo := range b._servos {
					if !servo.isIdle() {
						pin, pwm := servo.pwm()
						data[pin] = pwm
					}
				}
			case <-flushCh:
				if len(data) != 0 {
					b.flush(data)
					data = make(map[gpio]pwm)
				}
			}
		}
	}()
}

// subscribe adds a Servo reference to the manager.
func (b *blaster) subscribe(servo *Servo) {
	b.servos <- servoPkg{servo, true}
}

// unsubscribe removes a Servo reference from the manager.
func (b *blaster) unsubscribe(servo *Servo) {
	b.servos <- servoPkg{servo, false}
}

// Close cleans up the servo package. Make sure to call this in your main
// goroutine.
func Close() {
	if _blaster == nil {
		return
	}
	_blaster.close()
}

// close stops blaster if it was started.
func (b *blaster) close() {
	b.write("*=0.0")
	close(b.done)
}

// flush parses the data into "PIN=PWM PIN=PWM" format.
func (b *blaster) flush(data map[gpio]pwm) {
	s := new(strings.Builder)

	for pin, pwm := range data {
		fmt.Fprintf(s, " %d=%.6f", pin, pwm)
	}

	if s.Len() == 0 {
		return
	}

	b.write(s.String())
}

// write sends a string s to the designated io.Writer.
func (b *blaster) write(s string) {
	w := ioutil.Discard

	if !b.disabled {
		const pipepath = "/dev/pi-blaster"
		f, err := os.OpenFile(pipepath,
			os.O_WRONLY, os.ModeNamedPipe)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		w = f
	}

	fmt.Fprintf(w, "%s\n", s)
	//fmt.Fprintf(os.Stdout, "%s\n", s)
}
