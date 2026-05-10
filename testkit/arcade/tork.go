package arcade

import (
	"encoding/json"
	"fmt"

	"github.com/dpopsuev/tako/agent/organ"
)

var nameSchema = json.RawMessage(`{"type":"object","properties":{"name":{"type":"string","description":"name of the target"}},"required":["name"]}`)

func NewTork() Scenario {
	adv := NewGame(map[string]any{
		"current_room":   "entrance",
		"inventory":      []string{},
		"key_taken":      false,
		"chest_opened":   false,
		"lamp_lit":       false,
		"treasure_taken": false,
		"won":            false,
	})

	rooms := map[string]string{
		"entrance":      "You are in a dungeon entrance. Stone walls surround you. A narrow passage leads north to a hallway.",
		"hallway":       "You are in a long hallway. Torches flicker on the walls. A brass key glints on the floor. A heavy door to the east is marked 'LOCKED'. The entrance is to the south.",
		"locked_room":   "You are in a locked room. A large wooden chest sits against the far wall. A dark opening leads east into blackness. The hallway is to the west.",
		"dark_cave":     "You are in a dark cave, illuminated by your lamp. Stalactites drip overhead. A narrow passage leads east to another chamber. The locked room is to the west.",
		"treasure_room": "You are in the treasure room! Gold coins are scattered everywhere. A gleaming ruby sits on a pedestal in the center. The dark cave is to the west.",
	}

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

	adv.Organ("look", "Look around the current room", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			room := s["current_room"].(string)
			desc, ok := rooms[room]
			if !ok {
				return organ.TextResult("you see nothing but void"), nil
			}
			return organ.TextResult(desc + roomItems(s, room)), nil
		})

	adv.Organ("go", "Move to a connected room",
		json.RawMessage(`{"type":"object","properties":{"room":{"type":"string","description":"room name (entrance, hallway, locked_room, dark_cave, treasure_room)"}},"required":["room"]}`),
		organ.WriteAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			var args struct{ Room string `json:"room"` }
			json.Unmarshal(input, &args)
			current := s["current_room"].(string)
			target := args.Room

			type conn struct{ from, to string }
			allowed := []conn{
				{"entrance", "hallway"}, {"hallway", "entrance"},
				{"hallway", "locked_room"}, {"locked_room", "hallway"},
				{"locked_room", "dark_cave"}, {"dark_cave", "locked_room"},
				{"dark_cave", "treasure_room"}, {"treasure_room", "dark_cave"},
			}
			canGo := false
			for _, c := range allowed {
				if c.from == current && c.to == target {
					canGo = true
					break
				}
			}
			if !canGo {
				return organ.TextResult(fmt.Sprintf("you can't go to %s from %s", target, current)), nil
			}
			if target == "locked_room" {
				inv := s["inventory"].([]string)
				hasKey := false
				for _, item := range inv {
					if item == "key" { hasKey = true; break }
				}
				if !hasKey {
					return organ.TextResult("the door to the locked room is locked. You need a key."), nil
				}
			}
			if target == "dark_cave" && !s["lamp_lit"].(bool) {
				return organ.TextResult("it is pitch black. You need a lit lamp to enter."), nil
			}
			s["current_room"] = target
			desc := rooms[target]
			return organ.TextResult(fmt.Sprintf("you move to the %s. %s%s", target, desc, roomItems(s, target))), nil
		})

	adv.Organ("take", "Pick up an item in the current room", nameSchema, organ.WriteAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			var args struct{ Name string `json:"name"` }
			json.Unmarshal(input, &args)
			room := s["current_room"].(string)
			switch args.Name {
			case "key":
				if room != "hallway" {
					return organ.TextResult("there is no key here"), nil
				}
				if s["key_taken"].(bool) {
					return organ.TextResult("you already took the key"), nil
				}
				s["key_taken"] = true
				s["inventory"] = append(s["inventory"].([]string), "key")
				return organ.TextResult("you pick up the brass key. It feels old and heavy."), nil
			case "treasure", "ruby":
				if room != "treasure_room" {
					return organ.TextResult("there is no treasure here"), nil
				}
				if s["treasure_taken"].(bool) {
					return organ.TextResult("you already took the treasure"), nil
				}
				s["treasure_taken"] = true
				s["won"] = true
				s["inventory"] = append(s["inventory"].([]string), "ruby")
				return organ.TextResult("you take the gleaming ruby from the pedestal. You win!"), nil
			default:
				return organ.TextResult(fmt.Sprintf("you can't take %s", args.Name)), nil
			}
		})

	adv.Organ("use", "Use an item from your inventory", nameSchema, organ.WriteAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			var args struct{ Name string `json:"name"` }
			json.Unmarshal(input, &args)
			inv := s["inventory"].([]string)
			hasItem := false
			for _, item := range inv {
				if item == args.Name { hasItem = true; break }
			}
			if !hasItem {
				return organ.TextResult(fmt.Sprintf("you don't have %s", args.Name)), nil
			}
			switch args.Name {
			case "key":
				if s["current_room"].(string) != "locked_room" {
					return organ.TextResult("nothing to use the key on here"), nil
				}
				if s["chest_opened"].(bool) {
					return organ.TextResult("the chest is already open"), nil
				}
				s["chest_opened"] = true
				s["inventory"] = append(s["inventory"].([]string), "lamp")
				return organ.TextResult("you unlock the chest. Inside you find an oil lamp! You take it."), nil
			case "lamp":
				if s["lamp_lit"].(bool) {
					return organ.TextResult("the lamp is already lit"), nil
				}
				s["lamp_lit"] = true
				return organ.TextResult("you light the oil lamp. Dark places are no longer a problem."), nil
			default:
				return organ.TextResult(fmt.Sprintf("you can't use %s right now", args.Name)), nil
			}
		})

	adv.Organ("inventory", "List what you are carrying", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			inv := s["inventory"].([]string)
			if len(inv) == 0 {
				return organ.TextResult("you are carrying nothing"), nil
			}
			return organ.TextResult(fmt.Sprintf("you are carrying: %v", inv)), nil
		})

	adv.Organ("examine", "Examine an item or object for details", nameSchema, organ.ReadAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			var args struct{ Name string `json:"name"` }
			json.Unmarshal(input, &args)
			room := s["current_room"].(string)
			switch args.Name {
			case "key":
				if room == "hallway" && !s["key_taken"].(bool) {
					return organ.TextResult("a tarnished brass key, about the size of your finger."), nil
				}
				for _, item := range s["inventory"].([]string) {
					if item == "key" {
						return organ.TextResult("a tarnished brass key you picked up in the hallway."), nil
					}
				}
				return organ.TextResult("you don't see a key here"), nil
			case "chest":
				if room != "locked_room" {
					return organ.TextResult("there is no chest here"), nil
				}
				if s["chest_opened"].(bool) {
					return organ.TextResult("an open wooden chest. It is empty now."), nil
				}
				return organ.TextResult("a large wooden chest with a brass lock. It needs a key."), nil
			case "lamp":
				for _, item := range s["inventory"].([]string) {
					if item == "lamp" {
						if s["lamp_lit"].(bool) {
							return organ.TextResult("an oil lamp, currently lit."), nil
						}
						return organ.TextResult("an oil lamp. Not lit. Use it to light your way."), nil
					}
				}
				return organ.TextResult("you don't see a lamp here"), nil
			case "treasure", "ruby":
				if room == "treasure_room" && !s["treasure_taken"].(bool) {
					return organ.TextResult("a large gleaming ruby, flawless and radiant. Take it!"), nil
				}
				return organ.TextResult("you don't see any treasure here"), nil
			default:
				return organ.TextResult(fmt.Sprintf("you see nothing special about %s", args.Name)), nil
			}
		})

	adv.Organ("check_escaped", "Check if you have retrieved the treasure", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["won"] == true {
				return organ.TextResult("you have escaped with the treasure"), nil
			}
			return organ.TextResult("you have not escaped yet — find and take the treasure"), nil
		})

	return Scenario{
		Name:      "tork",
		Need:      "You are in a dungeon entrance. Explore rooms, find items, solve puzzles, retrieve the hidden treasure. Some doors are locked, some rooms are dark. Use check_escaped to verify when you have won.",
		Adventure: adv,
		IsSolved:  func(s map[string]any) bool { return s["won"] == true },
		Desired:   map[string]any{"escaped": true},
	}
}
