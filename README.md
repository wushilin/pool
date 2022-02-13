# pool
Reusable Object Pool for golang

```go
package main

import (
	"fmt"
	"math/rand"
	"github.com/wushilin/pool"
	"time"
)
// Demonstrate how to use
func main() {
    // Create a pool can park 200 objects, with a maker, tester, destroyer, and with idle timeout of 1 second
	p := pool.NewFixedPool(200, maker).WithTester(tester).WithDestroyer(destroyer).WithIdleTimeout(1)
	for {
	    // keep borrowing from the pool
		k, err:= p.Borrow()
                if err != nil {
			// do something about it
		} else {
			// print what is borrowed
			fmt.Printf("Borrwed %d\n", k)
			p.Return(k)
		}
		time.Sleep(3*time.Second)
	}
}

// Maker will create a random int between 0 and 9
func maker() (int, error) {
	result := rand.Intn(10)
	fmt.Printf("Made: %d\n", result)
	return result, nil
}

// Tester is random, for test to pass, it must be greater than 5, and a random number generated must be even
func tester(i int) bool {
	return i > 5 && rand.Int() %2 == 0
}

// Just show case objects are how they are destroyed
func destroyer(i int) {
	fmt.Printf("Destroyed: %d\n", i)
}

```
