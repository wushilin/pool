package pool

import (
	"time"
	"sync/atomic"
	"sync"
)

// Stores a single pooled object. It also stores the time it was last validated
type element[T any] struct {
	data          T
	lastValidated time.Time
}

type Pool[T any] struct {
	queue       chan element[T]
	created     int64
	borrowed    int64
	destroyed   int64
	tested      int64
	returned    int64
	idleTimeout time.Duration
	maker       func() (T,error)
	tester      func(T) bool
	destroyer   func(T)
}

// Set the tester function of the object. When set, objects exceeded idleTimeout will be revalidated before returning
// Without tester, objects exceeded idleTimeout will be discarded (via destroyer)
func (v *Pool[T]) WithTester(tester func(T) bool) *Pool[T] {
	v.tester = tester
	return v
}

// Function to be called when object is about to be discarded
// Either triggered by invalidator (tester) or timeout but no validator
func (v *Pool[T]) WithDestroyer(destroyer func(T)) *Pool[T] {
	v.destroyer = destroyer
	return v
}

// Objects idled for more than this time (seconds) will be revalidated via validator upon Borrow().
// If no validator provided, object will be discarded and Borrow() will
// return freshly made one via maker function
func (v *Pool[T]) WithIdleTimeout(seconds int) *Pool[T] {
	v.idleTimeout = time.Second * time.Duration(seconds)
	return v
}

// Create a new fixed pool. size is the max number of object to pool
// maker is the function that generates new object for the pool (when pool is empty)
func NewFixedPool[T any](size int, maker func() (T, error)) *Pool[T] {
	duration, _ := time.ParseDuration("30s")
	if maker == nil {
		panic("Need maker function")
	}
	result := &Pool[T]{
		queue:       make(chan element[T], size),
		created:     0,
		destroyed:   0,
		tested:      0,
		returned:    0,
		borrowed:    0,
		idleTimeout: duration,
		maker:       maker,
		tester:      nil,
		destroyer:   nil,
	}
	result.PreFill()
	return result
}

func (v *Pool[T]) wrapMaker() (T, error) {
	made, err := v.maker()
	if err != nil {
		return made, err
	}
	atomic.AddInt64(&v.created, 1)
	return made, err
}

func (v *Pool[T]) wrapDestroyer(what T) {
	if v.destroyer == nil {
		return
	}
	v.destroyer(what)
	atomic.AddInt64(&v.destroyed, 1)
}

func (v *Pool[T]) wrapTester(what T) bool {
	if v.tester == nil {
		return true
	}
	atomic.AddInt64(&v.tested, 1)
	return v.tester(what)
}
// Prepopulate the pool with full elements. This will call the maker repeately until it is full
// Failed maker will be discarded. If maker never return successful result, this may be in dead loop
func (v *Pool[T]) PreFill() int {
	var madeCount int32 = 0
	var wg sync.WaitGroup

	for i:=0; i < cap(v.queue); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			made, err := v.wrapMaker()
			if err != nil {
				return
			}
			elem := element[T]{made, time.Now()}
			select {
			case v.queue <- elem:
				atomic.AddInt32(&madeCount, 1)
				return
			default:
			    v.wrapDestroyer(elem.data)
			}
		}()
	}
	wg.Wait()
	return int(madeCount)
}

// Borrow a object from the pool, returns immediately if one is available
// If an object failed test upon checkout because of tester func fails, a new object will be made and returned
// Pool will not retry making. If you want to retry, retry in your maker function
func (v *Pool[T]) Borrow() (T, error) {
	result, err := v.borrowInternal()
	if err == nil {
		atomic.AddInt64(&v.borrowed, 1)
	}
	return result, err
}

func (v *Pool[T]) borrowInternal() (T, error) {
	select {
	case c := <-v.queue:
		data := c.data
		now := time.Now()
		elapsed := now.Sub(c.lastValidated)
		if elapsed >= v.idleTimeout {
			if v.tester != nil {
				// the thing may need to be validated again
				if v.wrapTester(data) {
					// the object is still good
					return data, nil
				} else {
					v.wrapDestroyer(data)
					return v.wrapMaker()
				}
			} else {
				// objects are discarded directly
				v.wrapDestroyer(data)
				for j := 0; j < 2; j++ {
					r, e := v.wrapMaker()
					if e != nil {
						time.Sleep(time.Second)
					} else {
						return r, e
					}
				}
				return v.wrapMaker()
			}
		} else {
			// no need to revalidate again yet
			return data, nil
		}
	default:
		return v.wrapMaker()
	}
}

// Return an object to the pool, the object doesn't has to be borrowed
// Returns true if returned successfully
// Returns false if pool is full and object had been discarded
// If a destroyer is defined, the object will be destroyed
// (which is unlikely unless you returned something extra to the pool)
func (v *Pool[T]) Return(c T) bool {
	elem := element[T]{c, time.Now()}
	atomic.AddInt64(&v.returned, 1)
	select {
	case v.queue <- elem:
		return true
	default:
		v.wrapDestroyer(c)
		return false
	}
}


func (v *Pool[T]) CreatedCount() int64 {
	return v.created
}

func (v *Pool[T]) TestedCount() int64 {
	return v.tested
}

func (v *Pool[T]) DestroyedCount() int64 {
	return v.destroyed
}

func (v *Pool[T]) ReturnedCount() int64 {
	return v.returned
}

func (v *Pool[T]) BorrowedCount() int64 {
	return v.borrowed
}
