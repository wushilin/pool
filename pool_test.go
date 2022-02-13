package pool


import (
	"testing"
	"math/rand"
	"time"
	"fmt"
)

var rands = rand.NewSource(time.Now().UnixNano())
var randSource = rand.New(rands)

// TestHelloName calls greetings.Hello with a name, checking
// for a valid return value.
func TestPool(t *testing.T) {
	p := NewFixedPool(20, maker).WithTester(tester).WithDestroyer(destroyer).WithIdleTimeout(900)
	count := 0
	for {
		count++
		if count > 50 {
			break
		}
		fmt.Printf("Iteration %d:\n", count)
		func() {
			v,err := p.Borrow()
			fmt.Println("Borrowed", v, "error", err)
			if err == nil {
				defer func() {
						fmt.Printf("Returned %d\n", v)
						p.Return(v)
				}()
			}
			time.Sleep(100 * time.Millisecond)
			fmt.Printf("Created %d tested %d destroyed %d borrowed %d returned %d\n", p.CreatedCount(), p.TestedCount(), 
				p.DestroyedCount(), p.BorrowedCount(), p.ReturnedCount())
		}()
	}
}

func maker() (int, error) {
	time.Sleep(5 * time.Second)
	result := randSource.Intn(100)
	fmt.Printf("Making number: %d\n", result)
	return result, nil
}

func tester(what int) bool {
	fmt.Printf("Testing %d => false\n", what)
	return false
}

func destroyer(what int) {
	fmt.Printf("Destroying %d\n", what)
}
