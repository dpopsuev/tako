package engine

import "github.com/dpopsuev/tangle/visual"

var approachToElement = map[string]visual.Element{
	"rapid":      visual.ElementFire,
	"aggressive": visual.ElementLightning,
	"methodical": visual.ElementEarth,
	"rigorous":   visual.ElementDiamond,
	"analytical": visual.ElementWater,
	"holistic":   visual.ElementAir,
}

func resolveApproach(name string) (visual.Element, bool) {
	e, ok := approachToElement[name]
	return e, ok
}
