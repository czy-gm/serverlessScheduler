package env

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// HandleSignal ...
func HandleSignal(cancel context.CancelFunc, done chan struct{}) {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGTERM)
	select {
	case <-sc:
		fmt.Printf("Server is killed by SIGTERM")
		cancel()
		close(done)
	}
}
