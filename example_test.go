//+build !live

package servo_test

import (
	"log"

	"github.com/cgxeiji/servo"
)

func Example() {
	defer servo.Close()
	// For example, start pi-blaster as:
	// $ sudo pi-blaster --gpio 14 --pcm
	servo1, err := servo.Connect(14)
	if err != nil {
		log.Fatal(err)
	}
	defer servo1.Close() // Make sure to Close() the servo.

	servo1.Name = "Servo 1" // Set a verbose name (optional).
	servo1.Speed(0.5)       // Set the speed to half (default: 1.0).

	servo1.MoveTo(90) // This is a non-blocking call.

	/* do some work */

	servo1.Wait() // Call Wait() to sync with the servo.
	// Output:
}
