package servo

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

type blaster struct {
	disabled bool
	buffer   chan string
	done     chan struct{}
	servos   chan gpioPWM

	file io.Writer
	rate time.Duration
}

var _blaster *blaster

type gpio int
type pwm float64

type gpioPWM struct {
	gpio gpio
	pwm  pwm
}

func init() {
	_blaster = &blaster{
		buffer: make(chan string),
		done:   make(chan struct{}),
		servos: make(chan gpioPWM),
		rate:   40 * time.Millisecond,
		file:   ioutil.Discard,
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

	if !b.disabled {
		const pipepath = "/dev/pi-blaster"
		f, err := os.OpenFile(pipepath,
			os.O_WRONLY, os.ModeNamedPipe)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		b.file = f
	}

	b.manager(b.done, time.NewTicker(b.rate).C)

	return nil
}

// manager keeps track of changes to servos and flushes the data to pi-blaster
// given the flushCh signal. The flush will happen only if there was a change
// in the servos data. Everytime the data is flushed, the variable is emptied.
func (b *blaster) manager(done <-chan struct{}, flushCh <-chan time.Time) {
	data := make(map[gpio]pwm)

	go func() {
		for {
			select {
			case <-done:
				return
			case servo := <-b.servos:
				data[servo.gpio] = servo.pwm
			case <-flushCh:
				if len(data) != 0 {
					flush(b.file, data)
					data = make(map[gpio]pwm)
				}
			}
		}
	}()
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
	close(b.done)
}

// set sets the data of blaster to a map[gpio] = pwm. It is safe to use
// concurrently.
func (b *blaster) set(gpio gpio, pwm pwm) {
	b.servos <- gpioPWM{gpio, pwm}
}

// flush parses the data into "PIN=PWM PIN=PWM" format and sends it to
// the designited io.Writer.
func flush(w io.Writer, data map[gpio]pwm) {
	s := new(strings.Builder)

	for pin, pwm := range data {
		fmt.Fprintf(s, " %d=%.2f", pin, pwm)
	}

	if s.Len() == 0 {
		return
	}

	fmt.Fprintf(w, "%s\n", s)
	//fmt.Fprintf(os.Stdout, "%s\n", s)
}
