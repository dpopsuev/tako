package arcade

import (
	"fmt"
	"strconv"
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

	adv.AddInstrument("status", "Show current kingdom status: grain, population, land, season, planted, fed", organ.ReadAction, func(s map[string]any, _ string) string {
		return fmt.Sprintf(
			"Season %d/%d | Grain: %d | Population: %d | Land: %d acres | Planted: %d | Fed: %d grain to people",
			s["season"], s["max_seasons"], s["grain"], s["population"], s["land"], s["planted"], s["fed"],
		)
	})

	adv.AddInstrument("plant", "Plant grain on your land. Input: number of grain to plant (costs 1 grain per acre, max = min(grain, land))", organ.WriteAction, func(s map[string]any, input string) string {
		if s["harvested"] == true {
			return "you already harvested this season, advance to next season first"
		}
		amount, err := strconv.Atoi(strings.TrimSpace(input))
		if err != nil || amount < 0 {
			return "provide a valid non-negative number of grain to plant"
		}
		grain := s["grain"].(int)
		land := s["land"].(int)
		maxPlant := grain
		if land < maxPlant {
			maxPlant = land
		}
		if amount > maxPlant {
			return fmt.Sprintf("cannot plant %d; you have %d grain and %d acres of land (max plantable: %d)", amount, grain, land, maxPlant)
		}
		s["grain"] = grain - amount
		s["planted"] = amount
		return fmt.Sprintf("planted %d grain across %d acres; %d grain remaining", amount, amount, s["grain"])
	})

	adv.AddInstrument("feed", "Feed your population. Input: total grain to distribute (each person needs 20 grain per season)", organ.WriteAction, func(s map[string]any, input string) string {
		if s["harvested"] == true {
			return "you already harvested this season, advance to next season first"
		}
		amount, err := strconv.Atoi(strings.TrimSpace(input))
		if err != nil || amount < 0 {
			return "provide a valid non-negative number of grain to feed"
		}
		grain := s["grain"].(int)
		if amount > grain {
			return fmt.Sprintf("not enough grain; you have %d but tried to feed %d", grain, amount)
		}
		population := s["population"].(int)
		needed := population * 20
		s["grain"] = grain - amount
		s["fed"] = amount
		if amount >= needed {
			return fmt.Sprintf("fed %d grain to %d people (needed %d); everyone is well-fed", amount, population, needed)
		}
		return fmt.Sprintf("fed %d grain to %d people (needed %d); some people will go hungry", amount, population, needed)
	})

	adv.AddInstrument("trade", "Buy or sell land. Input: 'buy N' or 'sell N'. Buying costs 20 grain/acre, selling yields 15 grain/acre", organ.WriteAction, func(s map[string]any, input string) string {
		parts := strings.Fields(strings.TrimSpace(input))
		if len(parts) != 2 {
			return "input must be 'buy N' or 'sell N'"
		}
		action := strings.ToLower(parts[0])
		amount, err := strconv.Atoi(parts[1])
		if err != nil || amount < 0 {
			return "provide a valid non-negative number of acres"
		}
		grain := s["grain"].(int)
		land := s["land"].(int)

		switch action {
		case "buy":
			cost := amount * 20
			if cost > grain {
				return fmt.Sprintf("cannot buy %d acres; costs %d grain but you only have %d", amount, cost, grain)
			}
			s["grain"] = grain - cost
			s["land"] = land + amount
			return fmt.Sprintf("bought %d acres for %d grain; now have %d acres and %d grain", amount, cost, s["land"], s["grain"])
		case "sell":
			if amount > land {
				return fmt.Sprintf("cannot sell %d acres; you only have %d", amount, land)
			}
			revenue := amount * 15
			s["grain"] = grain + revenue
			s["land"] = land - amount
			return fmt.Sprintf("sold %d acres for %d grain; now have %d acres and %d grain", amount, revenue, s["land"], s["grain"])
		default:
			return "input must start with 'buy' or 'sell'"
		}
	})

	adv.AddInstrument("harvest", "End the current season. Calculates harvest yield (planted * 3), applies starvation, advances season", organ.WriteAction, func(s map[string]any, _ string) string {
		if s["harvested"] == true {
			return "you already harvested this season"
		}

		planted := s["planted"].(int)
		fed := s["fed"].(int)
		population := s["population"].(int)
		season := s["season"].(int)
		maxSeasons := s["max_seasons"].(int)

		// Harvest yield
		yield := planted * 3
		grain := s["grain"].(int) + yield

		// Starvation check
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
			return report + fmt.Sprintf("%d people starved. Your kingdom has perished — all people are dead. Game over.", starved)
		}

		if starved > 0 {
			report += fmt.Sprintf("%d people starved (fed %d of %d needed). ", starved, fed, needed)
		} else {
			report += "No one starved. "
		}

		// Advance season
		season++
		s["grain"] = grain
		s["population"] = population
		s["land"] = s["land"]
		s["season"] = season
		s["planted"] = 0
		s["fed"] = 0
		s["starved"] = starved
		s["harvested"] = false

		if season > maxSeasons && population > 0 && grain > 0 {
			s["won"] = true
			return report + fmt.Sprintf("Population: %d, Grain: %d. You survived all %d seasons! Your kingdom endures!", population, grain, maxSeasons)
		}

		report += fmt.Sprintf("Population: %d, Grain: %d. Season %d begins.", population, grain, season)
		return report
	})

	adv.AddInstrument("look", "Describe the current state of your kingdom narratively", organ.ReadAction, func(s map[string]any, _ string) string {
		season := s["season"].(int)
		maxSeasons := s["max_seasons"].(int)
		grain := s["grain"].(int)
		population := s["population"].(int)
		land := s["land"].(int)
		planted := s["planted"].(int)
		fed := s["fed"].(int)

		if population <= 0 {
			return "Your kingdom lies in ruins. The people have all perished from starvation. There is no one left to rule."
		}

		needed := population * 20
		desc := fmt.Sprintf("It is season %d of %d. ", season, maxSeasons)
		desc += fmt.Sprintf("Your kingdom has %d people tending %d acres of land, with %d grain in the granary. ", population, land, grain)

		if planted > 0 {
			desc += fmt.Sprintf("%d grain has been planted in the fields. ", planted)
		} else {
			desc += "No grain has been planted yet this season. "
		}

		if fed > 0 {
			if fed >= needed {
				desc += "The people are well-fed. "
			} else {
				desc += fmt.Sprintf("Only %d of the %d grain needed has been distributed — some will starve. ", fed, needed)
			}
		} else {
			desc += fmt.Sprintf("The people have not been fed yet (they need %d grain). ", needed)
		}

		return desc
	})

	adv.AddInstrument("check_prosperity", "Check if the kingdom has survived all seasons", organ.ReadAction, func(s map[string]any, _ string) string {
		if s["won"] == true {
			return "the kingdom is prosperous — all seasons survived"
		}
		pop := s["population"].(int)
		if pop <= 0 {
			return "the kingdom has fallen — prosperity is not achieved"
		}
		return fmt.Sprintf("prosperity is not achieved yet — season %d of %d", s["season"], s["max_seasons"])
	})

	return Scenario{
		Name: "takonomics",
		Need: "You rule a small kingdom for 5 seasons. Start with 1000 grain, 100 people, and 200 acres of land. " +
			"Each season: plant grain on your land, feed your people (20 grain per person per season), optionally trade land, then harvest. " +
			"Planted grain yields 3x at harvest. If people starve, population drops. " +
			"Survive all 5 seasons with people alive and grain remaining. Use check_prosperity to verify.",
		Adventure: adv,
		IsSolved:  func(s map[string]any) bool { return s["won"] == true },
		Desired:  map[string]any{"prosperous": true},
	}
}
