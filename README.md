# Servo

[![Version](https://img.shields.io/github/v/tag/cgxeiji/servo?sort=semver)](https://github.com/cgxeiji/servo/releases)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/cgxeiji/servo)](https://pkg.go.dev/github.com/cgxeiji/servo)
[![License](https://img.shields.io/github/license/cgxeiji/servo)](https://github.com/cgxeiji/servo/blob/master/LICENSE)
![Go version](https://img.shields.io/github/go-mod/go-version/cgxeiji/servo)

Servo is a Go library to control servo motors on a Raspberry Pi using pi-blaster.

## DISCLAIMER

> **This library controls physical pins of a Raspberry Pi using pi-blaster.
> Incorrect pin assignment or pi-blaster configuration may DAMAGE the Raspberry
> Pi or connected devices.  Please, make sure to carefully check your connections
> and code before running your program.**
>
> **You have been warned.**
>
> **Good luck and Godspeed!**

## About the library

This library uses [pi-blaster](https://github.com/sarfata/pi-blaster) to
control servo motors on a Raspberry Pi. Under the hood, it opens a pipeline to
`/dev/pi-blaster` and sends commands in the format `GPIO=PWM`. The library
calculates the appropriate PWM based on the speed and position of the servo and
groups the writes to `/dev/pi-blaster` at a rate of 40 ms, if multiple servos
are connected. You can check the
[documentation](https://godoc.org/github.com/cgxeiji/servo) for more detailed
information.

Each connected servo is managed independently from one another and is designed
to be concurrent-safe.

If the package `servo` detects that `pi-blaster` is not running on the system when
executed, it will throw a warning:
```
YYYY/MM/DD HH:mm:ss WARNING: pi-blaster was not found running: start pi-blaster to avoid this error
        (servo will continue with pi-blaster disabled)
```
and redirect all writes to `/dev/null`. This way, you can build and test your code
on machines other than a Raspberry Pi or do a cold run before committing.

## Testing your System

To check if your system can handle real-time control of servos (i.e. move the
servos at the expected speed), a system check and a stress test are provided.

You can run them with:
```
$ cd $(go env GOPATH)/src/github.com/cgxeiji/servo
$ go test -v
```

This test makes sure that the system is capable of running **100 servos**
concurrently within the expected time frame. It simulates connecting 100 servos
and moving them from `0 to 180` degrees, from `180 to 0` degrees, and from `0
to 180` degrees for a total of 3 passes. At the nominal speed of a common
TowerPro servo of `0.19s/60degrees`, it should take approximately `570ms +/-
50ms` for the servo to move 180 degrees.

For benchmarking, the test also checks 1,000 and 10,000 concurrent servos
connected, but these tests won't throw critical failures.

In simple terms, if your Raspberry Pi is capable of running 100 servos at the
same time (which is a number way above the number of pins available), you can
be confident that the servos will be controlled as expected.

## Example code

```go
import (
	"fmt"
	"log"

	"github.com/cgxeiji/servo"
)

func Example() {
	// Use servo.Close() to close the connection of all servos and pi-blaster.
	defer servo.Close()

	// If you want to move the servos, make sure that pi-blaster is running.
	// For example, start pi-blaster as:
	// $ sudo pi-blaster --gpio 14 --pcm

	// Create a new servo connected to gpio 14.
	myServo := servo.New(14)
	// (optional) Initialize the servo with your preferred values.
	// myServo.Flags = servo.Normalized | servo.Centered
	myServo.MinPulse = 0.05 // Set the minimum pwm pulse width (default: 0.05).
	myServo.MaxPulse = 0.25 // Set the maximum pwm pulse width (default: 0.25).
	myServo.SetPosition(90) // Set the initial position to 90 degrees.
	myServo.SetSpeed(0.2)   // Set the speed to 20% (default: 1.0).
	// NOTE: The maximum speed of the servo is 0.19s/60degrees.
	// (optional) Set a verbose name.
	myServo.Name = "My Servo"

	// Print the information of the servo.
	fmt.Println(myServo)

	// Connect the servo to the daemon.
	err := myServo.Connect()
	if err != nil {
		log.Fatal(err)
	}

	// (optional) Use myServo.Close() to close the connection to a specific
	// servo. You still need to close the connection to pi-blaster with
	// `servo.Close()`.
	defer myServo.Close()

	myServo.SetSpeed(0.5) // Set the speed to half. This is concurrent-safe.
	myServo.MoveTo(180)   // This is a non-blocking call.

	/* do some work */

	myServo.Wait() // Call Wait() to sync with the servo.

	// MoveTo() returns a Waiter interface that can be used to move and wait on
	// the same line.
	myServo.MoveTo(0).Wait() // This is a blocking call.
}
```
