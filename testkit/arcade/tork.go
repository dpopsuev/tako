package arcade

import "fmt"

// NewTork returns a Zork-inspired inventory puzzle scenario.
// The agent must explore rooms, collect items, solve lock/light
// dependencies, and retrieve the hidden treasure.
func NewTork() Scenario {
	adv := NewTextAdventure(map[string]any{
		"current_room":  "entrance",
		"inventory":     []string{},
		"key_taken":     false,
		"chest_opened":  false,
		"lamp_lit":      false,
		"treasure_taken": false,
		"won":           false,
	})

	// Room descriptions.
	rooms := map[string]string{
		"entrance":      "You are in a dungeon entrance. Stone walls surround you. A narrow passage leads north to a hallway.",
		"hallway":       "You are in a long hallway. Torches flicker on the walls. A brass key glints on the floor. A heavy door to the east is marked 'LOCKED'. The entrance is to the south.",
		"locked_room":   "You are in a locked room. A large wooden chest sits against the far wall. A dark opening leads east into blackness. The hallway is to the west.",
		"dark_cave":     "You are in a dark cave, illuminated by your lamp. Stalactites drip overhead. A narrow passage leads east to another chamber. The locked room is to the west.",
		"treasure_room": "You are in the treasure room! Gold coins are scattered everywhere. A gleaming ruby sits on a pedestal in the center. The dark cave is to the west.",
	}

	// Items visible per room (only when not yet taken).
	roomItems := func(s map[string]any, room string) string {
		switch room {
		case "hallway":
			if !s["key_taken"].(bool) {
				return " You see a brass key on the floor."
			}
		case "locked_room":
			if !s["chest_opened"].(bool) {
				return " There is a large wooden chest here. It is locked."
			}
			return " There is an open chest here. It is empty."
		case "treasure_room":
			if !s["treasure_taken"].(bool) {
				return " A gleaming ruby sits on the pedestal."
			}
		}
		return ""
	}

	adv.AddInstrument("look", "Look around the current room to see its description and visible items", func(s map[string]any, _ string) string {
		room := s["current_room"].(string)
		desc, ok := rooms[room]
		if !ok {
			return "you see nothing but void"
		}
		return desc + roomItems(s, room)
	})

	adv.AddInstrument("go", "Move to a connected room. Input: room name (entrance, hallway, locked_room, dark_cave, treasure_room)", func(s map[string]any, input string) string {
		current := s["current_room"].(string)

		// Define allowed connections.
		type conn struct {
			from, to string
		}
		allowed := []conn{
			{"entrance", "hallway"},
			{"hallway", "entrance"},
			{"hallway", "locked_room"},
			{"locked_room", "hallway"},
			{"locked_room", "dark_cave"},
			{"dark_cave", "locked_room"},
			{"dark_cave", "treasure_room"},
			{"treasure_room", "dark_cave"},
		}

		canGo := false
		for _, c := range allowed {
			if c.from == current && c.to == input {
				canGo = true
				break
			}
		}
		if !canGo {
			return fmt.Sprintf("you can't go to %s from %s", input, current)
		}

		// Check locked_room requires key in inventory.
		if input == "locked_room" {
			inv := s["inventory"].([]string)
			hasKey := false
			for _, item := range inv {
				if item == "key" {
					hasKey = true
					break
				}
			}
			if !hasKey {
				return "the door to the locked room is locked. You need a key."
			}
		}

		// Check dark_cave requires lamp lit.
		if input == "dark_cave" {
			if !s["lamp_lit"].(bool) {
				return "it is pitch black in the cave. You need a lit lamp to enter."
			}
		}

		s["current_room"] = input
		desc := rooms[input]
		return fmt.Sprintf("you move to the %s. %s%s", input, desc, roomItems(s, input))
	})

	adv.AddInstrument("take", "Pick up an item in the current room. Input: item name (key, treasure)", func(s map[string]any, input string) string {
		room := s["current_room"].(string)

		switch input {
		case "key":
			if room != "hallway" {
				return "there is no key here"
			}
			if s["key_taken"].(bool) {
				return "you already took the key"
			}
			s["key_taken"] = true
			s["inventory"] = append(s["inventory"].([]string), "key")
			return "you pick up the brass key. It feels old and heavy."
		case "treasure", "ruby":
			if room != "treasure_room" {
				return "there is no treasure here"
			}
			if s["treasure_taken"].(bool) {
				return "you already took the treasure"
			}
			s["treasure_taken"] = true
			s["won"] = true
			s["inventory"] = append(s["inventory"].([]string), "ruby")
			return "you take the gleaming ruby from the pedestal. You win! The treasure is yours."
		default:
			return fmt.Sprintf("you can't take %s", input)
		}
	})

	adv.AddInstrument("use", "Use an item from your inventory. Input: item name (key, lamp)", func(s map[string]any, input string) string {
		inv := s["inventory"].([]string)
		hasItem := false
		for _, item := range inv {
			if item == input {
				hasItem = true
				break
			}
		}
		if !hasItem {
			return fmt.Sprintf("you don't have %s in your inventory", input)
		}

		switch input {
		case "key":
			room := s["current_room"].(string)
			if room != "locked_room" {
				return "there is nothing to use the key on here"
			}
			if s["chest_opened"].(bool) {
				return "the chest is already open"
			}
			s["chest_opened"] = true
			s["inventory"] = append(s["inventory"].([]string), "lamp")
			return "you unlock the chest with the brass key. Inside you find an oil lamp! You take the lamp."
		case "lamp":
			if s["lamp_lit"].(bool) {
				return "the lamp is already lit"
			}
			s["lamp_lit"] = true
			return "you light the oil lamp. It casts a warm glow around you. Dark places are no longer a problem."
		default:
			return fmt.Sprintf("you can't use %s right now", input)
		}
	})

	adv.AddInstrument("inventory", "List what you are currently carrying", func(s map[string]any, _ string) string {
		inv := s["inventory"].([]string)
		if len(inv) == 0 {
			return "you are carrying nothing"
		}
		return fmt.Sprintf("you are carrying: %v", inv)
	})

	adv.AddInstrument("examine", "Examine an item or object in the current room for more details. Input: item or object name", func(s map[string]any, input string) string {
		room := s["current_room"].(string)

		switch input {
		case "key":
			if room == "hallway" && !s["key_taken"].(bool) {
				return "a tarnished brass key, about the size of your finger. It looks like it could open a lock."
			}
			// Check inventory.
			for _, item := range s["inventory"].([]string) {
				if item == "key" {
					return "a tarnished brass key you picked up in the hallway. It might open something."
				}
			}
			return "you don't see a key here"
		case "chest":
			if room != "locked_room" {
				return "there is no chest here"
			}
			if s["chest_opened"].(bool) {
				return "an open wooden chest. It is empty now."
			}
			return "a large wooden chest with a brass lock. It looks like it needs a key to open."
		case "lamp":
			for _, item := range s["inventory"].([]string) {
				if item == "lamp" {
					if s["lamp_lit"].(bool) {
						return "an oil lamp, currently lit and glowing warmly."
					}
					return "an oil lamp. It is not lit. You could use it to light your way."
				}
			}
			return "you don't see a lamp here"
		case "treasure", "ruby":
			if room == "treasure_room" && !s["treasure_taken"].(bool) {
				return "a large gleaming ruby, flawless and radiant. It must be worth a fortune. Take it!"
			}
			return "you don't see any treasure here"
		case "door":
			if room == "hallway" {
				return "a heavy oak door to the east. The word 'LOCKED' is carved into its frame."
			}
			return "you don't see a notable door here"
		case "walls":
			return fmt.Sprintf("the %s has rough stone walls covered in moss and age.", room)
		default:
			return fmt.Sprintf("you see nothing special about %s", input)
		}
	})

	return Scenario{
		Name:      "tork",
		Need:      "You are in a dungeon entrance. Explore the rooms, find items, solve puzzles, and retrieve the hidden treasure. Some doors are locked and some rooms are dark.",
		Adventure: adv,
		IsSolved:  func(s map[string]any) bool { return s["won"] == true },
	}
}
