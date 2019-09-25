# Servo

Servo is a Go library to control servo motors on a Raspberry Pi using pi-blaster.

## NOT READY YET
---

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

This library facilitates the use of
[pi-blaster](https://github.com/sarfata/pi-blaster) to control servo motors on
a Raspberry Pi. Under the hood, it opens a pipeline to `/dev/pi-blaster` and
sends commands in the format `GPIO=PWM`. The library calculates the appropriate
PWM based on the speed and position of the servo and groups the writes to
`/dev/pi-blaster` if multiple servos are connected. You can check the
[documentation](https://godoc.org/github.com/cgxeiji/servo) for more detailed
information.

Each connected servo is managed independently from one another and is designed
to be concurrent-safe.

If `servo` detects that `pi-blaster` is not running on the system when
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
be more confident that the servos will be controlled as expected.
