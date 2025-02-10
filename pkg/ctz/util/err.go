package util

import "fmt"

func Wrap(newErr string, cause error) error {
	return fmt.Errorf(newErr+": %w", cause)
}

func WrapFmt(cause error, newErr string, args ...any) error {
	return fmt.Errorf(newErr+": %w", append(args, cause)...)
}
