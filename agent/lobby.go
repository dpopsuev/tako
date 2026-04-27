package agent

// Lobby is the PDP-PEP meeting point for agent admission.
// PDP evaluates, Lobby enforces. Issues Capability tokens.
type Lobby interface {
	Admit(identity string, persona Uniform) (Capability, error)
}

// StubLobby auto-admits all agents with default Capability for their Persona.
type StubLobby struct{}

var _ Lobby = StubLobby{}

func (StubLobby) Admit(identity string, persona Uniform) (Capability, error) {
	switch persona {
	case Worker:
		return WorkerCapability(identity), nil
	case Foreman:
		return ForemanCapability(identity), nil
	case Director:
		return DirectorCapability(identity), nil
	case Avatar:
		return AvatarCapability(identity), nil
	default:
		return WorkerCapability(identity), nil
	}
}
