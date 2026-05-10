package arcade

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/organ"
)

var moneySchema = json.RawMessage(`{"type":"object","properties":{"amount":{"type":"integer","description":"money to spend"}},"required":["amount"]}`)

func NewTakoTrail(ctx context.Context, sensory cerebrum.Bus) Scenario {
	adv := NewGame(map[string]any{
		"money":              100,
		"food":               50,
		"medicine":           3,
		"health":             100,
		"distance_remaining": 500,
		"distance_traveled":  0,
		"days":               0,
		"alive":              true,
		"arrived":            false,
	}).WithSensory(sensory)

	intVal := func(s map[string]any, key string) int {
		v, _ := s[key].(int)
		return v
	}

	adv.Organ("status", "Show all current stats", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			return organ.TextResult(fmt.Sprintf("Day %d | Money: %d | Food: %d | Medicine: %d | Health: %d | Traveled: %d miles | Remaining: %d miles",
				intVal(s, "days"), intVal(s, "money"), intVal(s, "food"), intVal(s, "medicine"),
				intVal(s, "health"), intVal(s, "distance_traveled"), intVal(s, "distance_remaining"))), nil
		})

	adv.Organ("buy_food", "Spend money to buy food (1 money = 2 food)",
		moneySchema,
		organ.WriteAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			var args struct{ Amount int `json:"amount"` }
			if err := json.Unmarshal(input, &args); err != nil { return organ.ErrorResult("invalid input: " + err.Error()), nil }
			if args.Amount <= 0 {
				return organ.TextResult("specify a positive number"), nil
			}
			money := intVal(s, "money")
			if args.Amount > money {
				return organ.TextResult(fmt.Sprintf("not enough money, you only have %d", money)), nil
			}
			s["money"] = money - args.Amount
			food := intVal(s, "food") + args.Amount*2
			s["food"] = food
			return organ.TextResult(fmt.Sprintf("bought %d food for %d money. Food: %d, Money: %d", args.Amount*2, args.Amount, food, money-args.Amount)), nil
		})

	adv.Organ("buy_medicine", "Spend money to buy medicine (5 money = 1 medicine)",
		moneySchema,
		organ.WriteAction,
		func(s map[string]any, input json.RawMessage) (organ.Result, error) {
			var args struct{ Amount int `json:"amount"` }
			if err := json.Unmarshal(input, &args); err != nil { return organ.ErrorResult("invalid input: " + err.Error()), nil }
			if args.Amount <= 0 {
				return organ.TextResult("specify a positive number"), nil
			}
			money := intVal(s, "money")
			if args.Amount > money {
				return organ.TextResult(fmt.Sprintf("not enough money, you only have %d", money)), nil
			}
			if args.Amount < 5 {
				return organ.TextResult("you need at least 5 money to buy 1 medicine"), nil
			}
			units := args.Amount / 5
			cost := units * 5
			s["money"] = money - cost
			med := intVal(s, "medicine") + units
			s["medicine"] = med
			return organ.TextResult(fmt.Sprintf("bought %d medicine for %d money. Medicine: %d, Money: %d", units, cost, med, money-cost)), nil
		})

	adv.Organ("travel", "Travel forward on the trail (consumes food, advances days)", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["alive"] != true {
				return organ.TextResult("you are dead, the journey is over"), nil
			}
			if s["arrived"] == true {
				return organ.TextResult("you have already arrived"), nil
			}
			days := intVal(s, "days")
			food := intVal(s, "food")
			health := intVal(s, "health")
			remaining := intVal(s, "distance_remaining")
			traveled := intVal(s, "distance_traveled")

			miles := 50 + (days%3)*25
			if miles > remaining {
				miles = remaining
			}
			s["days"] = days + 1
			foodCost := miles / 10
			if food >= foodCost {
				s["food"] = food - foodCost
			} else {
				s["food"] = 0
				health -= 20
				if health <= 0 {
					health = 0
					s["health"] = health
					s["alive"] = false
					return organ.TextResult(fmt.Sprintf("traveled %d miles but ran out of food. Health dropped to 0. You have died.", miles)), nil
				}
				s["health"] = health
			}
			remaining -= miles
			traveled += miles
			s["distance_remaining"] = remaining
			s["distance_traveled"] = traveled
			if remaining <= 0 {
				s["arrived"] = true
				return organ.TextResult(fmt.Sprintf("traveled %d miles and arrived! Total: %d miles in %d days.", miles, traveled, days+1)), nil
			}
			result := fmt.Sprintf("traveled %d miles (day %d). Remaining: %d miles. Food: %d, Health: %d", miles, days+1, remaining, intVal(s, "food"), health)
			if (traveled/150) > ((traveled-miles)/150) && remaining > 0 {
				result += ". River crossing ahead! Use ford_river to cross."
			}
			if (days+1)%10 < 3 {
				adv.StartTimer(ctx, TimerConfig{
					After: 2 * time.Second,
					Event: "A violent storm hits! You lost 5 food supplies.",
					Mutate: func(st map[string]any) {
						f := intVal(st, "food")
						f -= 5
						if f < 0 { f = 0 }
						st["food"] = f
					},
				})
			}
			if (days+1)%5 == 0 {
				adv.StartTimer(ctx, TimerConfig{
					After: 3 * time.Second,
					Event: "Disease strikes the party!",
					Mutate: func(st map[string]any) {
						med := intVal(st, "medicine")
						if med > 0 {
							st["medicine"] = med - 1
						} else {
							h := intVal(st, "health")
							h -= 15
							if h <= 0 { h = 0; st["alive"] = false }
							st["health"] = h
						}
					},
				})
			}
			return organ.TextResult(result), nil
		})

	adv.Organ("rest", "Rest for a day. Restores 10 health, consumes 1 food.", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["alive"] != true {
				return organ.TextResult("you are dead"), nil
			}
			food := intVal(s, "food")
			if food > 0 { s["food"] = food - 1 }
			health := intVal(s, "health") + 10
			if health > 100 { health = 100 }
			s["health"] = health
			s["days"] = intVal(s, "days") + 1
			return organ.TextResult(fmt.Sprintf("rested. Health: %d, Food: %d", health, intVal(s, "food"))), nil
		})

	adv.Organ("hunt", "Hunt for food. Gain 10 food, spend a day.", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["alive"] != true {
				return organ.TextResult("you are dead"), nil
			}
			s["days"] = intVal(s, "days") + 1
			food := intVal(s, "food") + 10
			s["food"] = food
			return organ.TextResult(fmt.Sprintf("hunted and gathered 10 food. Food: %d", food)), nil
		})

	adv.Organ("ford_river", "Ford a river crossing (available every 150 miles)", emptySchema, organ.WriteAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["alive"] != true {
				return organ.TextResult("you are dead"), nil
			}
			traveled := intVal(s, "distance_traveled")
			if traveled == 0 || traveled%150 != 0 {
				return organ.TextResult("no river crossing here"), nil
			}
			crossing := traveled / 150
			if crossing%2 == 0 {
				return organ.TextResult("you forded the river safely!"), nil
			}
			adv.StartTimer(ctx, TimerConfig{
				After: 2 * time.Second,
				Event: "The river current swept away supplies! Lost 10 food and 1 medicine.",
				Mutate: func(st map[string]any) {
					f := intVal(st, "food")
					f -= 10
					if f < 0 { f = 0 }
					st["food"] = f
					m := intVal(st, "medicine")
					if m > 0 { st["medicine"] = m - 1 }
				},
			})
			return organ.TextResult("fording the river... the current is strong!"), nil
		})

	adv.Organ("check_arrival", "Check if you have arrived", emptySchema, organ.ReadAction,
		func(s map[string]any, _ json.RawMessage) (organ.Result, error) {
			if s["arrived"] == true && s["alive"] == true {
				return organ.TextResult("you have arrived safely"), nil
			}
			if s["alive"] != true {
				return organ.TextResult("you are dead — arrival not possible"), nil
			}
			return organ.TextResult(fmt.Sprintf("not arrived — %d miles remaining", intVal(s, "distance_remaining"))), nil
		})

	return Scenario{
		Name: "tako_trail",
		Need: "Travel 500 miles to reach safety. You have 100 money, 50 food, 3 medicine. " +
			"Buy supplies, travel in legs, hunt when food is low, rest when health drops. " +
			"Random storms and disease will test your planning. Use check_arrival to verify.",
		Adventure: adv,
		IsSolved: func(s map[string]any) bool {
			return s["arrived"] == true && s["alive"] == true
		},
		Desired: map[string]any{"arrived": true},
	}
}
