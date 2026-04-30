package cerebrum

type Option func(*Cerebrum)

func WithSensory(s SensoryBus) Option {
	return func(cb *Cerebrum) { cb.sensory = s }
}

func WithMotor(m MotorBus) Option {
	return func(cb *Cerebrum) { cb.motor = m }
}

func WithMaxTurns(n int) Option {
	return func(cb *Cerebrum) { cb.maxTurns = n }
}

func WithClassifier(c Classifier) Option {
	return func(cb *Cerebrum) { cb.classifier = c }
}

func WithPromptBuilder(p PromptBuilder) Option {
	return func(cb *Cerebrum) { cb.promptBuilder = p }
}

func WithParser(p ResponseParser) Option {
	return func(cb *Cerebrum) { cb.parser = p }
}
