package arcade

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dpopsuev/tako/agent/organ"
)

func NewTakonomics() Scenario {
	adv := NewGame(map[string]any{
		"grain":       1000,
		"population":  100,
		"land":        200,
		"season":      1,
		"max_seasons": 5,
		"planted":     0,
		"fed":         0,
		"starved":     0,
		"harvested":   false,
		"won":         false,
	})

	adv.Organ("status", "Show current kingdom status", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			return organ.TextResult(fmt.Sprintf(
				"Season %d/%d | Grain: %d | Population: %d | Land: %d acres | Planted: %d | Fed: %d grain to people",
				s["season"], s["max_seasons"], s["grain"], s["population"], s["land"], s["planted"], s["fed"],
			)), nil
		})

	adv.Organ("plant", "Plant grain on your land (costs 1 grain per acre)",
		json.RawMessage(`{"type":"object","properties":{"amount":{"type":"integer","description":"number of grain to plant"}},"required":["amount"]}`),
		organ.WriteAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			if s["harvested"] == true {
				return organ.TextResult("you already harvested this season, advance to next season first"), nil
			}
			var args struct{ Amount int `json:"amount"` }
			if err := json.Unmarshal(input, &args); err != nil { return organ.ErrorResult("invalid input: " + err.Error()), nil }
			amount := args.Amount
			if amount < 0 {
				return organ.TextResult("provide a valid non-negative number"), nil
			}
			grain := s["grain"].(int)
			land := s["land"].(int)
			maxPlant := grain
			if land < maxPlant {
				maxPlant = land
			}
			if amount > maxPlant {
				return organ.TextResult(fmt.Sprintf("cannot plant %d; you have %d grain and %d acres (max: %d)", amount, grain, land, maxPlant)), nil
			}
			s["grain"] = grain - amount
			s["planted"] = amount
			return organ.TextResult(fmt.Sprintf("planted %d grain across %d acres; %d grain remaining", amount, amount, s["grain"])), nil
		})

	adv.Organ("feed", "Feed your population (each person needs 20 grain per season)",
		json.RawMessage(`{"type":"object","properties":{"amount":{"type":"integer","description":"total grain to distribute"}},"required":["amount"]}`),
		organ.WriteAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			if s["harvested"] == true {
				return organ.TextResult("you already harvested this season, advance to next season first"), nil
			}
			var args struct{ Amount int `json:"amount"` }
			if err := json.Unmarshal(input, &args); err != nil { return organ.ErrorResult("invalid input: " + err.Error()), nil }
			amount := args.Amount
			if amount < 0 {
				return organ.TextResult("provide a valid non-negative number"), nil
			}
			grain := s["grain"].(int)
			if amount > grain {
				return organ.TextResult(fmt.Sprintf("not enough grain; you have %d but tried to feed %d", grain, amount)), nil
			}
			population := s["population"].(int)
			needed := population * 20
			s["grain"] = grain - amount
			s["fed"] = amount
			if amount >= needed {
				return organ.TextResult(fmt.Sprintf("fed %d grain to %d people (needed %d); everyone is well-fed", amount, population, needed)), nil
			}
			return organ.TextResult(fmt.Sprintf("fed %d grain to %d people (needed %d); some people will go hungry", amount, population, needed)), nil
		})

	adv.Organ("trade", "Buy or sell land (buy: 20 grain/acre, sell: 15 grain/acre)",
		json.RawMessage(`{"type":"object","properties":{"action":{"type":"string","enum":["buy","sell"],"description":"buy or sell"},"acres":{"type":"integer","description":"number of acres"}},"required":["action","acres"]}`),
		organ.WriteAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			var args struct {
				Action string `json:"action"`
				Acres  int    `json:"acres"`
			}
			if err := json.Unmarshal(input, &args); err != nil { return organ.ErrorResult("invalid input: " + err.Error()), nil }
			action := strings.ToLower(args.Action)
			amount := args.Acres
			if amount < 0 {
				return organ.TextResult("provide a valid non-negative number of acres"), nil
			}
			grain := s["grain"].(int)
			land := s["land"].(int)

			switch action {
			case "buy":
				cost := amount * 20
				if cost > grain {
					return organ.TextResult(fmt.Sprintf("cannot buy %d acres; costs %d grain but you only have %d", amount, cost, grain)), nil
				}
				s["grain"] = grain - cost
				s["land"] = land + amount
				return organ.TextResult(fmt.Sprintf("bought %d acres for %d grain; now have %d acres and %d grain", amount, cost, s["land"], s["grain"])), nil
			case "sell":
				if amount > land {
					return organ.TextResult(fmt.Sprintf("cannot sell %d acres; you only have %d", amount, land)), nil
				}
				revenue := amount * 15
				s["grain"] = grain + revenue
				s["land"] = land - amount
				return organ.TextResult(fmt.Sprintf("sold %d acres for %d grain; now have %d acres and %d grain", amount, revenue, s["land"], s["grain"])), nil
			default:
				return organ.TextResult("action must be 'buy' or 'sell'"), nil
			}
		})

	adv.Organ("harvest", "End the current season — calculates yield, starvation, advances season", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["harvested"] == true {
				return organ.TextResult("you already harvested this season"), nil
			}
			planted := s["planted"].(int)
			fed := s["fed"].(int)
			population := s["population"].(int)
			season := s["season"].(int)
			maxSeasons := s["max_seasons"].(int)

			yield := planted * 3
			grain := s["grain"].(int) + yield

			needed := population * 20
			starved := 0
			if fed < needed {
				starved = (needed - fed) / 20
				population -= starved
			}

			report := fmt.Sprintf("Season %d harvest: gained %d grain from %d planted. ", season, yield, planted)
			if population <= 0 {
				s["grain"] = grain
				s["population"] = 0
				s["starved"] = starved
				s["harvested"] = true
				return organ.TextResult(report + fmt.Sprintf("%d people starved. Your kingdom has perished. Game over.", starved)), nil
			}
			if starved > 0 {
				report += fmt.Sprintf("%d people starved (fed %d of %d needed). ", starved, fed, needed)
			} else {
				report += "No one starved. "
			}
			season++
			s["grain"] = grain
			s["population"] = population
			s["season"] = season
			s["planted"] = 0
			s["fed"] = 0
			s["starved"] = starved
			s["harvested"] = false
			if season > maxSeasons && population > 0 && grain > 0 {
				s["won"] = true
				return organ.TextResult(report + fmt.Sprintf("Population: %d, Grain: %d. You survived all %d seasons!", population, grain, maxSeasons)), nil
			}
			return organ.TextResult(report + fmt.Sprintf("Population: %d, Grain: %d. Season %d begins.", population, grain, season)), nil
		})

	adv.Organ("look", "Describe the current state of your kingdom", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			season := s["season"].(int)
			maxSeasons := s["max_seasons"].(int)
			grain := s["grain"].(int)
			population := s["population"].(int)
			land := s["land"].(int)
			planted := s["planted"].(int)
			fed := s["fed"].(int)
			if population <= 0 {
				return organ.TextResult("Your kingdom lies in ruins. The people have all perished from starvation."), nil
			}
			needed := population * 20
			desc := fmt.Sprintf("It is season %d of %d. ", season, maxSeasons)
			desc += fmt.Sprintf("Your kingdom has %d people tending %d acres of land, with %d grain in the granary. ", population, land, grain)
			if planted > 0 {
				desc += fmt.Sprintf("%d grain has been planted. ", planted)
			} else {
				desc += "No grain planted yet. "
			}
			if fed > 0 {
				if fed >= needed {
					desc += "The people are well-fed. "
				} else {
					desc += fmt.Sprintf("Only %d of %d grain needed distributed — some will starve. ", fed, needed)
				}
			} else {
				desc += fmt.Sprintf("People not fed yet (need %d grain). ", needed)
			}
			return organ.TextResult(desc), nil
		})

	adv.Organ("check_prosperity", "Check if the kingdom has survived all seasons", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["won"] == true {
				return organ.TextResult("the kingdom is prosperous — all seasons survived"), nil
			}
			pop := s["population"].(int)
			if pop <= 0 {
				return organ.TextResult("the kingdom has fallen — prosperity is not achieved"), nil
			}
			return organ.TextResult(fmt.Sprintf("prosperity not achieved yet — season %d of %d", s["season"], s["max_seasons"])), nil
		})

	return Scenario{
		Name: "takonomics",
		Need: "You rule a small kingdom for 5 seasons. Start with 1000 grain, 100 people, and 200 acres. " +
			"Each season: plant grain, feed people (20 grain/person/season), optionally trade land, then harvest. " +
			"Planted grain yields 3x at harvest. If people starve, population drops. " +
			"Survive all 5 seasons with people alive and grain remaining. Use check_prosperity to verify.",
		Adventure: adv,
		IsSolved:  func(s map[string]any) bool { return s["won"] == true },
		Desired:   map[string]any{"prosperous": true},
	}
}
