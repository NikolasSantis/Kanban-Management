package events

type Handler func(Event) error

type Dispatcher struct {
	handlers map[string][]Handler
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string][]Handler),
	}
}

func (d *Dispatcher) Register(eventName string, handler Handler) {
	d.handlers[eventName] = append(
		d.handlers[eventName],
		handler,
	)
}

func (d *Dispatcher) Dispatch(event Event) error {
	handlers := d.handlers[event.Name()]

	for _, handler := range handlers {
		if err := handler(event); err != nil {
			return err
		}
	}

	return nil
}
