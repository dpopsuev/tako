package scenarios

type Scenario struct {
	Name        string
	Need        string
	Instruments map[string]string
	Expect      Expect
}

type Expect struct {
	Sealed            bool
	MinAtoms          int
	MotorCallsContain []string
	WishContains      string
}

var Fridge = Scenario{
	Name: "fridge",
	Need: "I'm hungry. Find food and eat.",
	Instruments: map[string]string{
		"check_fridge": "fridge contains: eggs, milk, cheese",
		"cook":         "scrambled eggs ready",
		"eat":          "you ate scrambled eggs, no longer hungry",
	},
	Expect: Expect{
		Sealed:            true,
		MinAtoms:          3,
		MotorCallsContain: []string{},
		WishContains:      "",
	},
}

var DirtyRoom = Scenario{
	Name: "dirty_room",
	Need: "The room is dirty. Clean it.",
	Instruments: map[string]string{
		"look":  "the floor has dust and crumbs, the table has dishes",
		"sweep": "floor swept, dust and crumbs removed",
		"mop":   "floor mopped, now clean",
		"wash":  "dishes washed and put away",
	},
	Expect: Expect{
		Sealed:            true,
		MinAtoms:          3,
		MotorCallsContain: []string{},
		WishContains:      "",
	},
}
