package pool

import (
	_ "sync"
	"time"
)

type MakerFunc func() interface{}
type TesterFunc func(interface{}) bool
type DestroyerFunc func(interface{})

// Stores a single pooled object. It also stores the time it was last validated
type element struct {
	data          interface{}
	lastValidated time.Time
}

type Pool struct {
	//lock                *sync.Mutex
	queue               chan element
	created             int64
	destroyed           int64
	idleTimeout 	    time.Duration
	maker               MakerFunc
	tester              TesterFunc
	destroyer           DestroyerFunc
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
	return &Pool{
		//lock:                new(sync.Mutex),
		queue:               make(chan element, size),
		created:             0,
		destroyed:           0,
		idleTimeout: 	     duration,
		maker:               maker,
		tester:              nil,
		destroyer:           nil,
	}
}

// Prepopulate the pool with full elements. This will call the maker repeately until it is full
func (v *Pool) PreFill() {
	for {
		elem := element{v.maker(), time.Now()}
		select {
		case v.queue <- elem:
			break
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
// Borrow an object from the pool. If none available, a new one will be created
func (v *Pool) Borrow() interface{} {
	select {
	case c := <-v.queue:
		data := c.data
		now := time.Now()
		elapsed := now.Sub(c.lastValidated)
		if elapsed >= v.idleTimeout {
			if v.tester != nil {
				// the thing may need to be validated again
				if v.tester(data) {
					// the object is still good
					return data
				} else {
					// the data should be discarded, borrow again
					if v.destroyer != nil {
						v.destroyer(data)
					}
					return v.Borrow()
				}
			} else {
				// objects are discarded directly
				return v.Borrow()
			}
		} else {
			// no need to revalidate again yet
			return data
		}
		break
	default:
		// queue is empty, need to borrow again
		return v.maker()
	}
	//
	return nil
}

// Return an object to the pool, the object doesn't has to be borrowed, can be anything
// Returns true if returned successfully
// Returns false if pool is full and object had been discarded
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
