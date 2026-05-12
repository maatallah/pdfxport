package utils

import "time"

func Retry(attempts int, fn func() error) error {
	var err error

	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		time.Sleep(time.Duration(i*i) * time.Second)
	}

	return err
}
