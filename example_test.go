//+build !live

package servo_test

import (
	"log"

	"github.com/cgxeiji/servo"
)

func Example() {
	// Use servo.Close() to close the connection of all servos and pi-blaster.
	defer servo.Close()
	// For example, start pi-blaster as:
	// $ sudo pi-blaster --gpio 14 --pcm
	myServo := servo.New(14)
	err := myServo.Connect()
	if err != nil {
		log.Fatal(err)
	}
	// Use myServo.Close() to close the connection to a specific servo. You
	// still need to close the connection to pi-blaster with `servo.Close()`.
	defer myServo.Close() // Make sure to Close() the servo.

	myServo.Name = "My Servo" // Set a verbose name (optional).
	myServo.Speed(0.5)        // Set the speed to half (default: 1.0).

	myServo.MoveTo(90) // This is a non-blocking call.

	/* do some work */

	myServo.Wait() // Call Wait() to sync with the servo.

	// MoveTo() returns a Waiter interface that can be used to move and wait on
	// the same line.
	myServo.MoveTo(0).Wait() // This is a blocking call.

	// Output:
}
