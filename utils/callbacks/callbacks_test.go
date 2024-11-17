package callbacks

import (
	"testing"
)

func TestSchedulingCallbacks(t *testing.T) {
	defer Purge()

	var a, b int

	persistentCallback := func() {
		a += 1
	}
	oneOffCallback := func() {
		b += 1
	}

	ScheduleCallback(ConfigLoaded, false, Callback{Fn: persistentCallback, Name: "inc a", Registerer: "test"})
	ScheduleCallback(ConfigLoaded, true, Callback{Fn: oneOffCallback, Name: "inc b", Registerer: "test"})

	if a != 0 || b != 0 {
		t.Fatal("registering callbacks should not fire them\n")
	}

	RegisterEvent(ConfigLoaded)
	RegisterEvent(ConfigLoaded)
	if a != 2 {
		t.Fatal("persistent callback should run 2 times\n")
	}
	if b != 1 {
		t.Fatal("oneOff callback should only run once\n")
	}

	ScheduleCallback(ConfigLoaded, true, Callback{Fn: oneOffCallback, Name: "inc b", Registerer: "test"})
	RegisterEvent(ConfigLoaded)
	if a != 3 {
		t.Fatal("persistent callback should run 3 times\n")
	}
	if b != 2 {
		t.Fatal("oneOff callback should be able to be re-added\n")
	}

	b = 0
	ScheduleCallback(ConfigLoaded, true, Callback{Fn: oneOffCallback, Name: "inc b", Registerer: "test"})
	ScheduleCallback(ConfigLoaded, true, Callback{Fn: oneOffCallback, Name: "inc b", Registerer: "test"})
	ScheduleCallback(ConfigLoaded, true, Callback{Fn: oneOffCallback, Name: "inc b", Registerer: "test"})
	ScheduleCallback(ConfigLoaded, true, Callback{Fn: oneOffCallback, Name: "inc b", Registerer: "test"})
	ScheduleCallback(ConfigLoaded, true, Callback{Fn: oneOffCallback, Name: "inc b", Registerer: "test"})
	RegisterEvent(ConfigLoaded)
	if b != 5 {
		t.Fatal("oneOff callback should be able to be registered many times\n")
	}
}

func TestPersistentEvents(t *testing.T) {
	defer Purge()

	var a, b int

	aCallback := func() {
		a += 1
	}
	bCallback := func() {
		b += 1
	}

	ScheduleCallback(ConfigLoaded, true, Callback{Fn: aCallback, Name: "inc a", Registerer: "test"})
	ScheduleCallback(ConfigLoaded, false, Callback{Fn: bCallback, Name: "inc b", Registerer: "test"})
	RegisterPersistentEvent(ConfigLoaded)
	if a != 1 || b != 1 {
		t.Fatal("both callback should be called\n")
	}

	// This should immediately run this one-off callback and not add it to the collection.
	ScheduleCallback(ConfigLoaded, true, Callback{Fn: aCallback, Name: "inc a", Registerer: "test"})
	if a != 2 {
		t.Fatal("callback should be called immediately, without waiting for an event\n")
	}

	// There should now be one persistent bCallback in the collection.
	a, b = 0, 0
	// This should not change the fact that ConfigLoaded is a persistent event, but should run all callbacks.
	RegisterEvent(ConfigLoaded)
	if a != 0 || b != 1 {
		t.Fatal("only bCallback should be called\n")
	}

	b = 0
	// This should add a new persistent aCallback to the collection and run it, but not run other persistent callbacks.
	ScheduleCallback(ConfigLoaded, false, Callback{Fn: aCallback, Name: "inc a", Registerer: "test"})
	if a != 1 || b != 0 {
		t.Fatal("only aCallback should be called\n")
	}

	a, b = 0, 0
	// This should run both persistent callbacks.
	RegisterEvent(ConfigLoaded)
	if a != 1 || b != 1 {
		t.Fatal("both persistent callback should be called\n")
	}
}
