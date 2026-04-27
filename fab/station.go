package fab

// Station is a node in the Fab production line graph.
type Station struct {
	Name        string
	Instruments []string
	Intake      bool
	Terminus    bool
}
