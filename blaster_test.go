package servo

import (
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestInit(t *testing.T) {
	if _blaster == nil {
		t.Fatal("_blaster was not initialized")
	}
}

func TestHasBlaster(t *testing.T) {
	if hasBlaster() {
		t.Error("pi-blaster was found running during test")
	}
}

func TestNoPiBlaster(t *testing.T) {
	noPiBlaster()
	if !_blaster.disabled {
		t.Error("NoPiBlaster() could not disable _blaster")
	}
}

func TestStart(t *testing.T) {
	for i := 0; i < 5; i++ {
		_blaster.buffer <- "testing"
	}
}

func TestSend(t *testing.T) {
	_blaster.send("testing")
}

func TestFlush(t *testing.T) {
	_blaster.set(10, rand.Float64())
	_blaster.flush()

	t.Run("Concurrency", func(t *testing.T) {
		var wg sync.WaitGroup

		wg.Add(5)
		for i := 0; i < 5; i++ {
			go func() {
				defer wg.Done()
				for i := 0; i < 5; i++ {
					_blaster.flush()
				}
			}()
		}

		wg.Wait()
	})
}

func TestSet(t *testing.T) {
	_blaster.set(10, rand.Float64())

	t.Run("Concurrency", func(t *testing.T) {
		var wg sync.WaitGroup
		done := make(chan struct{})

		wg.Add(5)
		for i := 0; i < 5; i++ {
			go func(i int) {
				defer wg.Done()
				for {
					select {
					case <-done:
						return
					default:
						_blaster.set(i, rand.Float64())
					}
				}
			}(i)
		}

		var fwg sync.WaitGroup

		fwg.Add(5)
		for i := 0; i < 5; i++ {
			go func() {
				defer fwg.Done()
				time.Sleep(10 * time.Microsecond)
				_blaster.flush()
			}()
		}

		fwg.Wait()
		close(done)
		wg.Wait()
	})
}
