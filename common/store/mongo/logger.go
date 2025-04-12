package mongo

import "github.com/rs/zerolog"

type logger struct {
	log zerolog.Logger
}

func (l *logger) Info(level int, message string, keysAndValues ...interface{}) {
	var event *zerolog.Event
	switch level {
	case 1:
		event = l.log.Info()
	case 2:
		event = l.log.Debug()
	default:
		return
	}
	event = l.buildEventArgs(event, keysAndValues...)
	event.Msg(message)
}

func (l *logger) Error(err error, message string, keysAndValues ...interface{}) {
	event := l.log.Error().Err(err)
	event = l.buildEventArgs(event, keysAndValues...)
	event.Msg(message)
}

func (l *logger) buildEventArgs(event *zerolog.Event, keysAndValues ...interface{}) *zerolog.Event {
	if len(keysAndValues)%2 != 0 {
		keysAndValues = append(keysAndValues, nil)
	}
	for i := 0; i < len(keysAndValues); i += 2 {
		if key, ok := keysAndValues[i].(string); ok {
			event = event.Interface(key, keysAndValues[i+1])
		}
	}
	return event
}
