package v8go

import (
	"fmt"
	"testing"
)

func TestFunctionPoolOwnership(t *testing.T) {
	for _, count := range []int{0, 1, 8, 9, 31, 32, 33, 256} {
		t.Run(fmt.Sprintf("args_%d", count), func(t *testing.T) {
			t.Parallel()
			iso := NewIsolate()
			defer iso.Dispose()

			value := Undefined(iso)
			input := make([]Valuer, count)
			for i := range input {
				input[i] = value
			}

			for repeat := 0; repeat < 16; repeat++ {
				first := marshalFunctionArgs(input)
				second := marshalFunctionArgs(input)
				if count > 0 && first.ptr == second.ptr {
					t.Fatal("simultaneous calls share argument storage")
				}

				storage := first.large
				if first.pooled != nil {
					storage = first.pooled[:]
				}

				for _, ptr := range storage[:count] {
					if ptr != value.ptr {
						t.Fatal("argument pointer changed")
					}
				}

				if count > 0 && count <= pooledFunctionArgs && first.pooled == nil {
					t.Fatal("bounded arguments were not pooled")
				}

				if count > pooledFunctionArgs && first.pooled != nil {
					t.Fatal("oversized arguments were pooled")
				}

				first.release()
				second.release()
			}
		})
	}
}

func TestFunctionPoolClearsHandles(t *testing.T) {
	iso := NewIsolate()
	defer iso.Dispose()

	input := make([]Valuer, pooledFunctionArgs)
	for i := range input {
		input[i] = Undefined(iso)
	}

	args := marshalFunctionArgs(input)
	storage := args.pooled
	args.release()
	for _, ptr := range storage {
		if ptr != nil {
			t.Fatal("released buffer retains a native handle")
		}
	}
}
