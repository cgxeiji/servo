package servo

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type blaster struct {
	disabled bool
	buffer   chan string
	data     chan map[int]float64
	done     chan struct{}
}

var _blaster *blaster

func init() {
	_blaster = &blaster{
		buffer: make(chan string),
		data:   make(chan map[int]float64, 1),
		done:   make(chan struct{}),
	}
	_blaster.data <- make(map[int]float64)

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

	var started sync.WaitGroup
	started.Add(1)
	go func() {
		started.Done()
		for {
			select {
			case data := <-b.buffer:
				b.send(data)
			case <-b.done:
				return
			}
		}
	}()

	started.Wait()
	return nil
}

// Close cleans up the servo package. Make sure to call this in your main
// goroutine.
func Close() {
	if _blaster == nil {
		return
	}
	close(_blaster.done)
}

// close stops blaster if it was started.
func (b *blaster) close() {
	close(b.done)
}

// send writes data to /dev/pi-blaster. If NoPiBlaster was called, the output
// is written to ioutil.Discard.
func (b *blaster) send(data string) {
	const blasterFile = "/dev/pi-blaster"

	w := ioutil.Discard

	if !b.disabled {
		f, err := os.OpenFile(blasterFile,
			os.O_WRONLY, os.ModeNamedPipe)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		w = f
	}

	fmt.Fprintf(w, "%s", data)
}

// flush parses the data into "PIN=PWM PIN=PWM" format and sends it to
// pi-blaster. It is safe to use concurrently.
func (b *blaster) flush() {
	s := new(strings.Builder)

	data := <-b.data
	for pin, pwm := range data {
		fmt.Fprintf(s, " %d=%.2f ", pin, pwm)
	}
	b.data <- data

	if s.Len() == 0 {
		return
	}

	b.buffer <- s.String()
}

// set sets the data of blaster to a map[gpio] = pwm. It is safe to use
// concurrently.
func (b *blaster) set(gpio int, pwm float64) {
	data := <-b.data
	data[gpio] = pwm
	b.data <- data
}
