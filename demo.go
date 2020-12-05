package main

import (
	"fmt"
	kite "github.com/get-code-ch/kite-common"
	"time"
)

func xxx() {
	// Initialize a new Notifier.
	n := kite.EventNotifier{
		Observers: map[kite.Observer]struct{}{},
	}

	// Register a couple of observers.
	n.Register(&kite.GenericEventObserver{Id: 1})
	n.Register(&kite.GenericEventObserver{Id: 2})

	// A simple loop publishing the current Unix timestamp to observers.
	stop := time.NewTimer(10 * time.Second).C
	tick1 := time.NewTicker(time.Second).C
	tick2 := time.NewTicker(time.Second).C
	for {
		select {
		case <-stop:
			n.Broadcast(kite.Event{Data: "Time elapsed"})
			return
		case t1 := <-tick1:
			n.Broadcast(kite.Event{Data: fmt.Sprintf("Timestamp %d", t1.UnixNano())})
			for o := range n.Observers {
				if o.Key() == 1 {
					n.Deregister(o)
				}
			}
			break
		case <-tick2:
			n.Broadcast(kite.Event{Data: "Tick 2 Event occurred"})
		}
	}
}
