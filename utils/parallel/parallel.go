package parallel

import (
	"fmt"
)

type Errors []error

func (e Errors) IsError() bool {
	for _, err := range e {
		if err != nil {
			return true
		}
	}
	return false
}

func (e Errors) Combine() (err error) {
	for _, cErr := range e {
		if cErr != nil {
			if err == nil {
				err = cErr
			} else {
				err = fmt.Errorf("%s\n%w", cErr, err)
			}
		}
	}
	return
}

type indexedError struct {
	err error
	idx int
}

func ForAll[V any](m []V, f func(V) error) Errors {
	count := len(m)
	ch := make(chan indexedError, count)

	for i, v := range m {
		go func(val V, i int) { ch <- indexedError{f(val), i} }(v, i)
	}

	return combineErrors(ch, count)
}

func combineErrors(ch chan indexedError, count int) Errors {
	errors := make(Errors, count)
	for i := 0; i < count; i++ {
		tErr := <-ch
		errors[tErr.idx] = tErr.err
	}
	return errors
}
