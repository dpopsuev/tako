package scenarios

import "fmt"

type Scenario struct {
	Name      string
	Need      string
	Adventure *TextAdventure
	IsSolved  func(state map[string]any) bool
}

func NewFridge() Scenario {
	adv := NewTextAdventure(map[string]any{
		"hungry": true,
		"fridge": []string{"eggs", "milk", "cheese"},
		"stove":  "off",
		"hand":   "",
		"plate":  "",
	})

	adv.AddInstrument("look_fridge", "See what food is in the fridge", func(s map[string]any, _ string) string {
		items, _ := s["fridge"].([]string)
		if len(items) == 0 {
			return "the fridge is empty"
		}
		return fmt.Sprintf("fridge contains: %v", items)
	})

	adv.AddInstrument("take", "Take an item from the fridge. Input: item name", func(s map[string]any, input string) string {
		items, _ := s["fridge"].([]string)
		for i, item := range items {
			if item == input {
				s["fridge"] = append(items[:i], items[i+1:]...)
				s["hand"] = input
				return fmt.Sprintf("you took %s from the fridge", input)
			}
		}
		return fmt.Sprintf("%s is not in the fridge", input)
	})

	adv.AddInstrument("cook", "Cook what you're holding. Requires stove to be on.", func(s map[string]any, _ string) string {
		if s["stove"] != "on" {
			return "the stove is off, turn it on first"
		}
		hand, _ := s["hand"].(string)
		if hand == "" {
			return "you're not holding anything to cook"
		}
		s["plate"] = fmt.Sprintf("cooked %s", hand)
		s["hand"] = ""
		return fmt.Sprintf("you cooked %s, it's on the plate now", hand)
	})

	adv.AddInstrument("turn_on_stove", "Turn on the stove", func(s map[string]any, _ string) string {
		s["stove"] = "on"
		return "stove is now on"
	})

	adv.AddInstrument("eat", "Eat what's on the plate", func(s map[string]any, _ string) string {
		plate, _ := s["plate"].(string)
		if plate == "" {
			return "there's nothing on the plate to eat"
		}
		s["plate"] = ""
		s["hungry"] = false
		return fmt.Sprintf("you ate %s, you're no longer hungry", plate)
	})

	return Scenario{
		Name:      "fridge",
		Need:      "You are hungry. Find food in the fridge, cook it, and eat. You must turn on the stove before cooking.",
		Adventure: adv,
		IsSolved:  func(s map[string]any) bool { return s["hungry"] == false },
	}
}

func NewPastaBolognese() Scenario {
	adv := NewTextAdventure(map[string]any{
		"pantry":    []string{"pasta", "canned tomatoes", "garlic", "onion", "olive oil", "salt", "pepper"},
		"fridge":    []string{"ground beef", "parmesan", "butter"},
		"pot":       "",
		"pan":       "",
		"stove":     "off",
		"water":     "not boiling",
		"hand":      "",
		"cutting_board": "",
		"sauce":     "",
		"pasta_cooked": false,
		"plated":    false,
		"eaten":     false,
	})

	adv.AddInstrument("look_pantry", "Check what's in the pantry", func(s map[string]any, _ string) string {
		items, _ := s["pantry"].([]string)
		if len(items) == 0 {
			return "pantry is empty"
		}
		return fmt.Sprintf("pantry has: %v", items)
	})

	adv.AddInstrument("look_fridge", "Check what's in the fridge", func(s map[string]any, _ string) string {
		items, _ := s["fridge"].([]string)
		if len(items) == 0 {
			return "fridge is empty"
		}
		return fmt.Sprintf("fridge has: %v", items)
	})

	adv.AddInstrument("take", "Take an item from pantry or fridge. Input: item name", func(s map[string]any, input string) string {
		for _, source := range []string{"pantry", "fridge"} {
			items, _ := s[source].([]string)
			for i, item := range items {
				if item == input {
					s[source] = append(items[:i], items[i+1:]...)
					s["hand"] = input
					return fmt.Sprintf("you took %s", input)
				}
			}
		}
		return fmt.Sprintf("%s not found in pantry or fridge", input)
	})

	adv.AddInstrument("chop", "Chop what you're holding on the cutting board", func(s map[string]any, _ string) string {
		hand, _ := s["hand"].(string)
		if hand == "" {
			return "you're not holding anything"
		}
		s["cutting_board"] = fmt.Sprintf("chopped %s", hand)
		s["hand"] = ""
		return fmt.Sprintf("chopped %s on the cutting board", hand)
	})

	adv.AddInstrument("turn_on_stove", "Turn on the stove", func(s map[string]any, _ string) string {
		s["stove"] = "on"
		return "stove is on"
	})

	adv.AddInstrument("boil_water", "Fill pot with water and put on stove to boil", func(s map[string]any, _ string) string {
		if s["stove"] != "on" {
			return "stove is off, turn it on first"
		}
		s["pot"] = "water"
		s["water"] = "boiling"
		return "pot of water is now boiling"
	})

	adv.AddInstrument("cook_pasta", "Put pasta in boiling water", func(s map[string]any, _ string) string {
		if s["water"] != "boiling" {
			return "water isn't boiling yet, boil water first"
		}
		s["pasta_cooked"] = true
		s["pot"] = "cooked pasta"
		return "pasta is cooked al dente"
	})

	adv.AddInstrument("brown_meat", "Brown ground beef in the pan", func(s map[string]any, _ string) string {
		if s["stove"] != "on" {
			return "stove is off, turn it on first"
		}
		s["pan"] = "browned meat"
		return "ground beef is browned"
	})

	adv.AddInstrument("make_sauce", "Add tomatoes and chopped vegetables to the pan with meat", func(s map[string]any, _ string) string {
		pan, _ := s["pan"].(string)
		if pan != "browned meat" {
			return "brown the meat first"
		}
		cutting, _ := s["cutting_board"].(string)
		if cutting == "" {
			return "nothing chopped on the cutting board, chop some vegetables first"
		}
		s["sauce"] = "bolognese"
		s["pan"] = "bolognese sauce"
		s["cutting_board"] = ""
		return "bolognese sauce is simmering with meat, tomatoes, and vegetables"
	})

	adv.AddInstrument("plate", "Combine pasta and sauce on a plate", func(s map[string]any, _ string) string {
		if s["pasta_cooked"] != true {
			return "pasta isn't cooked yet"
		}
		sauce, _ := s["sauce"].(string)
		if sauce != "bolognese" {
			return "sauce isn't ready yet"
		}
		s["plated"] = true
		return "pasta bolognese plated beautifully"
	})

	adv.AddInstrument("eat", "Eat the plated dish", func(s map[string]any, _ string) string {
		if s["plated"] != true {
			return "nothing plated yet"
		}
		s["eaten"] = true
		return "delicious pasta bolognese, buon appetito"
	})

	return Scenario{
		Name: "pasta_bolognese",
		Need: "Cook pasta bolognese from scratch. Check the pantry and fridge for ingredients. You need to: chop vegetables, brown the meat, make the sauce, boil water, cook pasta, combine and eat. The stove must be on before cooking. Vegetables must be chopped before adding to sauce.",
		Adventure: adv,
		IsSolved:  func(s map[string]any) bool { return s["eaten"] == true },
	}
}

func NewDirtyRoom() Scenario {
	adv := NewTextAdventure(map[string]any{
		"floor":   []string{"dust", "crumbs"},
		"table":   []string{"dirty dishes"},
		"trash":   "full",
		"broom":   "closet",
		"cleaned": false,
	})

	adv.AddInstrument("look", "Look around the room to see what needs cleaning", func(s map[string]any, _ string) string {
		floor, _ := s["floor"].([]string)
		table, _ := s["table"].([]string)
		trash, _ := s["trash"].(string)
		parts := []string{}
		if len(floor) > 0 {
			parts = append(parts, fmt.Sprintf("floor has %v", floor))
		}
		if len(table) > 0 {
			parts = append(parts, fmt.Sprintf("table has %v", table))
		}
		if trash == "full" {
			parts = append(parts, "trash can is full")
		}
		if len(parts) == 0 {
			return "the room is clean"
		}
		return fmt.Sprintf("you see: %s", fmt.Sprint(parts))
	})

	adv.AddInstrument("get_broom", "Get the broom from the closet", func(s map[string]any, _ string) string {
		if s["broom"] == "hand" {
			return "you already have the broom"
		}
		s["broom"] = "hand"
		return "you got the broom from the closet"
	})

	adv.AddInstrument("sweep", "Sweep the floor. Requires broom in hand.", func(s map[string]any, _ string) string {
		if s["broom"] != "hand" {
			return "you need the broom first, get it from the closet"
		}
		s["floor"] = []string{}
		return "floor swept clean"
	})

	adv.AddInstrument("wash_dishes", "Wash the dishes on the table", func(s map[string]any, _ string) string {
		s["table"] = []string{}
		return "dishes washed and put away"
	})

	adv.AddInstrument("take_out_trash", "Take the trash out", func(s map[string]any, _ string) string {
		s["trash"] = "empty"
		return "trash taken out"
	})

	adv.AddInstrument("check_done", "Check if the room is fully clean", func(s map[string]any, _ string) string {
		floor, _ := s["floor"].([]string)
		table, _ := s["table"].([]string)
		trash, _ := s["trash"].(string)
		if len(floor) == 0 && len(table) == 0 && trash == "empty" {
			s["cleaned"] = true
			return "the room is completely clean"
		}
		remaining := []string{}
		if len(floor) > 0 {
			remaining = append(remaining, "floor still dirty")
		}
		if len(table) > 0 {
			remaining = append(remaining, "dishes still on table")
		}
		if trash == "full" {
			remaining = append(remaining, "trash still full")
		}
		return fmt.Sprintf("not done yet: %v", remaining)
	})

	return Scenario{
		Name:      "dirty_room",
		Need:      "The room is dirty. Look around, then clean everything: sweep the floor (need broom from closet first), wash dishes, take out trash.",
		Adventure: adv,
		IsSolved:  func(s map[string]any) bool { return s["cleaned"] == true },
	}
}
