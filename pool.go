package pool

import (
	"sync"
	"time"
)

type MakerFunc func() (interface{}, error)
type TesterFunc func(interface{}) bool
type DestroyerFunc func(interface{})

// Stores a single pooled object. It also stores the time it was last validated
type element struct {
	data          interface{}
	lastValidated time.Time
}

type Pool struct {
	lock        *sync.Mutex
	queue       chan element
	created     int64
	destroyed   int64
	idleTimeout time.Duration
	maker       MakerFunc
	tester      TesterFunc
	destroyer   DestroyerFunc
}

// Set the tester function of the object. When set, objects exceeded idleTimeout will be revalidated before returning
// Without tester, objects exceeded idleTimeout will be discarded (via destroyer)
func (v *Pool) WithTester(tester TesterFunc) *Pool {
	v.tester = tester
	return v
}

// Function to be called when object is about to be discarded
// Either triggered by invalidator (tester) or timeout but no validator
func (v *Pool) WithDestroyer(destroyer DestroyerFunc) *Pool {
	v.destroyer = destroyer
	return v
}

// Objects idled for more than this time (seconds) will be revalidated via validator upon Borrow().
// If no validator provided, object will be discarded and Borrow() will
// return freshly made one via maker function
func (v *Pool) WithIdleTimeout(seconds int) *Pool {
	v.idleTimeout = time.Second * time.Duration(seconds)
	return v
}

// Create a new fixed pool. size is the max number of object to pool
// maker is the function that generates new object for the pool (when pool is empty)
func NewFixedPool(size int, maker MakerFunc) *Pool {
	duration, _ := time.ParseDuration("30s")
	if maker == nil {
		panic("Need maker function")
	}
	result := &Pool{
		lock:        new(sync.Mutex),
		queue:       make(chan element, size),
		created:     0,
		destroyed:   0,
		idleTimeout: duration,
		maker:       maker,
		tester:      nil,
		destroyer:   nil,
	}
	result.PreFill()
	return result
}

// Prepopulate the pool with full elements. This will call the maker repeately until it is full
// Failed maker will be discarded. If maker never return successful result, this may be in dead loop
func (v *Pool) PreFill() {
	for cap(v.queue) > 0 {
		made, err := v.maker()
		if err != nil {
			continue
		}
		elem := element{made, time.Now()}
		select {
		case v.queue <- elem:
			continue
		default:
			if v.destroyer != nil {
				v.destroyer(elem.data)
			}
			goto END
		}
	}
END:
	return
}

// Borrow a object from the pool, block until one is available.
// If an object failed test upon checkout because of tester func fails, a new object will be made and returned
// Maker will be tried 3 times, with 1 seconds delay in between
func (v *Pool) Borrow() (interface{}, error) {
	c := <-v.queue
	data := c.data
	now := time.Now()
	elapsed := now.Sub(c.lastValidated)
	if elapsed >= v.idleTimeout {
		if v.tester != nil {
			// the thing may need to be validated again
			if v.tester(data) {
				// the object is still good
				return data, nil
			} else {
				// the data should be discarded, borrow again
				if v.destroyer != nil {
					v.destroyer(data)
				}
				for j := 0; j < 2; j++ {
					r, e := v.maker()
					if e != nil {
						time.Sleep(time.Second)
					} else {
						return r, e
					}
				}
				return v.maker()
			}
		} else {
			// objects are discarded directly
			if v.destroyer != nil {
				v.destroyer(data)
			}
			for j := 0; j < 2; j++ {
				r, e := v.maker()
				if e != nil {
					time.Sleep(time.Second)
				} else {
					return r, e
				}
			}
			return v.maker()
		}
	} else {
		// no need to revalidate again yet
		return data, nil
	}

}

// Return an object to the pool, the object doesn't has to be borrowed
// Returns true if returned successfully
// Returns false if pool is full and object had been discarded
// (which is unlikely unless you returned something extra to the pool)
func (v *Pool) Return(c interface{}) bool {
	elem := element{c, time.Now()}

	select {
	case v.queue <- elem:
		return true
	default:
		if v.destroyer != nil {
			v.destroyer(c)
		}
		return false
	}
}
