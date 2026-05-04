package arcade

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/dpopsuev/tako/agent/cerebrum"
	"github.com/dpopsuev/tako/agent/organ"
)

func NewTakoTrail(ctx context.Context, sensory cerebrum.Bus) Scenario {
	adv := NewGame("tako_trail", map[string]any{
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

	adv.AddInstrument("status", "Show all current stats: money, food, medicine, health, distance traveled, distance remaining, days", organ.ReadAction, func(s map[string]any, _ string) string {
		return fmt.Sprintf("Day %d | Money: %d | Food: %d | Medicine: %d | Health: %d | Traveled: %d miles | Remaining: %d miles",
			intVal(s, "days"), intVal(s, "money"), intVal(s, "food"), intVal(s, "medicine"),
			intVal(s, "health"), intVal(s, "distance_traveled"), intVal(s, "distance_remaining"))
	})

	adv.AddInstrument("buy_food", "Spend money to buy food. Input: amount of money to spend. 1 money = 2 food.", organ.WriteAction, func(s map[string]any, input string) string {
		amount, err := strconv.Atoi(input)
		if err != nil || amount <= 0 {
			return "specify a positive number of money to spend"
		}
		money := intVal(s, "money")
		if amount > money {
			return fmt.Sprintf("not enough money, you only have %d", money)
		}
		s["money"] = money - amount
		food := intVal(s, "food") + amount*2
		s["food"] = food
		return fmt.Sprintf("bought %d food for %d money. Food: %d, Money: %d", amount*2, amount, food, money-amount)
	})

	adv.AddInstrument("buy_medicine", "Spend money to buy medicine. Input: amount of money to spend. 5 money = 1 medicine.", organ.WriteAction, func(s map[string]any, input string) string {
		amount, err := strconv.Atoi(input)
		if err != nil || amount <= 0 {
			return "specify a positive number of money to spend"
		}
		money := intVal(s, "money")
		if amount > money {
			return fmt.Sprintf("not enough money, you only have %d", money)
		}
		if amount < 5 {
			return "you need at least 5 money to buy 1 medicine"
		}
		units := amount / 5
		cost := units * 5
		s["money"] = money - cost
		med := intVal(s, "medicine") + units
		s["medicine"] = med
		return fmt.Sprintf("bought %d medicine for %d money. Medicine: %d, Money: %d", units, cost, med, money-cost)
	})

	adv.AddInstrument("travel", "Travel forward on the trail. Consumes 1 food per 10 miles. Travels 50-100 miles per leg. Increments days.", organ.WriteAction, func(s map[string]any, _ string) string {
		if s["alive"] != true {
			return "you are dead, the journey is over"
		}
		if s["arrived"] == true {
			return "you have already arrived"
		}

		days := intVal(s, "days")
		food := intVal(s, "food")
		health := intVal(s, "health")
		remaining := intVal(s, "distance_remaining")
		traveled := intVal(s, "distance_traveled")

		// Distance varies 50-100 based on day parity for determinism.
		miles := 50 + (days%3)*25
		if miles > remaining {
			miles = remaining
		}

		s["days"] = days + 1

		// Consume food: 1 per 10 miles.
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
				return fmt.Sprintf("traveled %d miles but ran out of food. Health dropped to 0. You have died on the trail.", miles)
			}
			s["health"] = health
		}

		remaining -= miles
		traveled += miles
		s["distance_remaining"] = remaining
		s["distance_traveled"] = traveled

		if remaining <= 0 {
			s["arrived"] = true
			return fmt.Sprintf("traveled %d miles and arrived at your destination! Total distance: %d miles in %d days.", miles, traveled, days+1)
		}

		result := fmt.Sprintf("traveled %d miles today (day %d). Remaining: %d miles. Food: %d, Health: %d",
			miles, days+1, remaining, intVal(s, "food"), health)

		// River crossing milestone every 150 miles.
		if (traveled/150) > ((traveled-miles)/150) && remaining > 0 {
			result += ". You have reached a river crossing! Use ford_river to cross."
		}

		// Random events via timers on sensory bus.
		// 30% storm chance (deterministic via day modulo).
		if (days+1)%10 < 3 {
			adv.StartTimer(ctx, TimerConfig{
				After: 2 * time.Second,
				Event: "A violent storm hits your camp! You lost 5 food supplies to water damage.",
				Mutate: func(st map[string]any) {
					f := intVal(st, "food")
					f -= 5
					if f < 0 {
						f = 0
					}
					st["food"] = f
				},
			})
		}

		// 20% disease chance.
		if (days+1)%5 == 0 {
			adv.StartTimer(ctx, TimerConfig{
				After: 3 * time.Second,
				Event: func() string {
					med := intVal(s, "medicine")
					if med > 0 {
						return "Disease strikes the party! You used 1 medicine to treat it."
					}
					return "Disease strikes the party! No medicine available, health dropped by 15."
				}(),
				Mutate: func(st map[string]any) {
					med := intVal(st, "medicine")
					if med > 0 {
						st["medicine"] = med - 1
					} else {
						h := intVal(st, "health")
						h -= 15
						if h <= 0 {
							h = 0
							st["alive"] = false
						}
						st["health"] = h
					}
				},
			})
		}

		return result
	})

	adv.AddInstrument("rest", "Rest for a day. Restores 10 health (max 100). Consumes 1 food.", organ.WriteAction, func(s map[string]any, _ string) string {
		if s["alive"] != true {
			return "you are dead, the journey is over"
		}

		food := intVal(s, "food")
		if food > 0 {
			s["food"] = food - 1
		}

		health := intVal(s, "health")
		health += 10
		if health > 100 {
			health = 100
		}
		s["health"] = health
		s["days"] = intVal(s, "days") + 1

		return fmt.Sprintf("rested for a day. Health: %d, Food: %d", health, intVal(s, "food"))
	})

	adv.AddInstrument("hunt", "Spend a day hunting for food. Gain 10 food at no money cost.", organ.WriteAction, func(s map[string]any, _ string) string {
		if s["alive"] != true {
			return "you are dead, the journey is over"
		}

		s["days"] = intVal(s, "days") + 1
		food := intVal(s, "food") + 10
		s["food"] = food

		return fmt.Sprintf("hunted for a day and gathered 10 food. Food: %d", food)
	})

	adv.AddInstrument("ford_river", "Attempt to ford a river crossing. 50% chance of losing supplies. Available at river crossings (every 150 miles).", organ.WriteAction, func(s map[string]any, _ string) string {
		if s["alive"] != true {
			return "you are dead, the journey is over"
		}

		traveled := intVal(s, "distance_traveled")
		if traveled == 0 || traveled%150 != 0 {
			return "there is no river crossing here"
		}

		days := intVal(s, "days")
		// Deterministic 50% based on crossing number.
		crossing := traveled / 150
		if crossing%2 == 0 {
			// Safe crossing.
			return "you forded the river safely! The party continues on."
		}

		// Lose supplies — fire timer on sensory bus.
		adv.StartTimer(ctx, TimerConfig{
			After: 2 * time.Second,
			Event: "The river current was strong! Some supplies were swept away. Lost 10 food and 1 medicine.",
			Mutate: func(st map[string]any) {
				f := intVal(st, "food")
				f -= 10
				if f < 0 {
					f = 0
				}
				st["food"] = f

				m := intVal(st, "medicine")
				if m > 0 {
					st["medicine"] = m - 1
				}
			},
		})

		_ = days
		return "fording the river... the current is strong!"
	})

	return Scenario{
		Name: "tako_trail",
		Need: "You must travel 500 miles to reach safety. You have 100 money, 50 food, and 3 medicine. " +
			"Buy supplies wisely, travel in legs, hunt when food is low, and rest when health drops. " +
			"Random storms and disease will test your planning. You will receive notifications about events.",
		Adventure: adv,
		IsSolved: func(s map[string]any) bool {
			return s["arrived"] == true && s["alive"] == true
		},
	}
}
