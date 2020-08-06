// +build !race,!live

package servo

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStress(t *testing.T) {
	degrees := 180.0
	_t := time.Duration(degrees/315.7*1000) * time.Millisecond
	const tolerance = 50 * time.Millisecond
	min := _t - tolerance
	max := _t + tolerance

	for n := 100; n <= 10000; n *= 10 {
		t.Run(fmt.Sprintf("%dServos", n), func(t *testing.T) {
			servos := make([]*Servo, 0, n)
			times := make([]time.Duration, 0, n)

			for i := 0; i < n; i++ {
				s, cl, err := Connect(i)
				if err != nil {
					t.Fatalf("servos[%d] -> %v", i, err)
				}
				defer cl()
				servos = append(servos, s)
			}

			var wg sync.WaitGroup
			timeout := make(chan time.Duration)

			wg.Add(n)

			for i := 0; i < n; i++ {
				go func(i int) {
					defer wg.Done()
					times := make([]time.Duration, 0, 3)
					tests := []float64{180, 0, 180}

					for _, d := range tests {
						start := time.Now()
						servos[i].moveTo(d)
						servos[i].Wait()
						elapsed := time.Since(start)
						if elapsed < min || elapsed > max {
							times = append(times, elapsed)
						}
					}
					var sum time.Duration
					for _, t := range times {
						sum += t
					}
					if sum > 0 {
						timeout <- sum / time.Duration(len(times))
					}
				}(i)
			}

			go func() {
				wg.Wait()
				close(timeout)
			}()

			for t := range timeout {
				times = append(times, t)
			}

			fn := len(times)

			if fn != 0 {
				sum := time.Duration(0)
				for _, t := range times {
					sum += t
				}
				mean := sum / time.Duration(fn)

				ratio := float64(fn) / float64(n)
				if n > 100 {
					t.Logf("%d out of %d (%.2f%%) servos failed with a mean time of %v (it should take between %v and %v to move %.2f degrees)",
						fn, n, ratio*100.0, mean, min, max, degrees)
				} else {
					t.Errorf("%d out of %d (%.2f%%) servos failed with a mean time of %v (it should take between %v and %v to move %.2f degrees)",
						fn, n, ratio*100.0, mean, min, max, degrees)
				}
			}
		})
	}
}
