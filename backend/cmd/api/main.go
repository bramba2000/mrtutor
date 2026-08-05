package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/bramba2000/mrtutor/backend/errs"
)

var isShuttingDown atomic.Bool

func main() {
	err := run(context.Background(), os.Stderr, os.LookupEnv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal %v\n", err)
		if errors.Is(err, errs.Invalid) {
			os.Exit(2) // misconfiguration - don't restart, fix the config
		}
		os.Exit(1)
	}
}
