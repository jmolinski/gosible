package callbacks

// TODO make it thread safe?

type eventType int

const (
	ConfigLoaded eventType = iota
)

type callbacksCollection struct {
	runOnce             []Callback
	persistentCallbacks []Callback
}

type Callback struct {
	Fn         func()
	Name       string
	Registerer string
}

var scheduledCallbacksForEvents = make(map[eventType]*callbacksCollection)
var persistentEventsStore = make(map[eventType]bool)

// RegisterEvent registers that an event of type eventType has been triggered and runs associated callbacks.
func RegisterEvent(eventName eventType) {
	collection, ok := scheduledCallbacksForEvents[eventName]
	if !ok {
		return
	}

	for _, callback := range collection.persistentCallbacks {
		callback.Fn()
	}
	for _, callback := range collection.runOnce {
		callback.Fn()
	}
	collection.runOnce = nil
}

// RegisterPersistentEvent registers that a state marked by an event of type eventType has changed.
// All new callbacks added for this eventType will be called immediately, without waiting for the next event.
// Persistent callbacks will be saved nevertheless.
// Triggers all callbacks associated with this eventType.
func RegisterPersistentEvent(eventName eventType) {
	persistentEventsStore[eventName] = true
	RegisterEvent(eventName)
}

// ScheduleCallback is used to register a callback that will be called when the event is triggered.
func ScheduleCallback(event eventType, runOnce bool, callback Callback) {
	collection, ok := scheduledCallbacksForEvents[event]
	if !ok {
		collection = &callbacksCollection{}
		scheduledCallbacksForEvents[event] = collection
	}

	// persistent && runImmediately -> run and add to list
	// persistent && !runImmediately -> add to list
	// oneOff && runImmediately -> run
	// oneOff && !runImmediately -> add to list

	if runImmediately, ok := persistentEventsStore[event]; ok && runImmediately {
		callback.Fn()
	} else if runOnce {
		collection.runOnce = append(collection.runOnce, callback)
	}

	if !runOnce {
		collection.persistentCallbacks = append(collection.persistentCallbacks, callback)
	}
}

// Purge removes all saved callbacks and registered events.
func Purge() {
	scheduledCallbacksForEvents = make(map[eventType]*callbacksCollection)
	persistentEventsStore = make(map[eventType]bool)
}
