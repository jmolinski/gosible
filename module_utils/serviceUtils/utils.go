package serviceUtils

import "fmt"

func ErrorIfMissing(found bool, service, msg string) error {
	if found {
		return nil
	}
	return fmt.Errorf("could not find the requested service %s: %s", service, msg)
}
