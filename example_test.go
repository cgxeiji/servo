//+build !live

package servo_test

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

	// Output:
	// servo "My Servo" connected to gpio(14) [flags: ( NONE )]
}
