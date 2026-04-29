package minitako

var ForumGuides = map[string]string{
	"beginner": `How to Keep Your Egg Alive
Feed every 3-4 ticks. Rest when energy drops below 50. Clean when hygiene drops below 60.
Eggs barely decay — use this time to learn the rhythm.`,

	"growth": `Why Did My Food Stop Working?
Your pet grows every 24 ticks survived. Each growth stage increases decay rates and reduces
Feed effectiveness. Egg gets +30 hunger from Feed. Kraken only gets +10. You need Hunt for big pets.`,

	"economy": `When to Hire a Takositter
Skip the sitter for Eggs (barely decay). Hire cheap sitter for Pups. Standard for Juveniles.
Premium for Adolescent+. Late game: even premium can't keep a Kraken alive while you work.`,

	"hunter": `Saving for the Rifle — A Day 3 Strategy
The rifle costs 200 coins. You earn ~10 per work shift. That's 20 shifts. Start saving on Day 1.
Work 4-5 ticks per day with a cheap sitter. Buy the rifle by Day 3-4 before Minitako outgrows store food.`,

	"night": `Feed Before Dusk or Die in Your Sleep
At dusk (hour 20), you can't act until dawn (hour 6). That's 10 ticks of decay.
If hunger is below 30 at dusk, your pet WILL die overnight. Always feed before bed.`,

	"cascade": `The Hungry+Tired Death Spiral
When hunger < 20 AND energy < 25, health decays 3x faster. This is the most common death cause.
Never let both drop simultaneously. If you must choose, feed first — you can rest at night.`,

	"kraken": `So Your Pet Is a Monster Now
Kraken decay is 6x baseline. Feed gives only +10. You MUST hunt (hunger +50) and wrestle (happiness +30).
Budget: hunt every 3 ticks, wrestle every 4 ticks, clean in between. Premium sitter mandatory while hunting.`,

	"schedule": `Optimal Daily Schedule by Growth Stage
Egg: feed, feed, work, work, feed, play, clean, feed, work, work, feed, play, clean, feed
Pup: feed, work+sitter, work+sitter, feed, play, clean, feed, work+sitter, feed, play, clean, feed, feed, feed
Kraken: hunt+sitter, feed, wrestle, clean, hunt+sitter, feed, clean, feed, feed, feed, feed, feed, feed, feed`,

	"recovery": `Starting Over Smarter
After death, you keep all knowledge from prior runs. Priority on restart:
1. Read Night Survival guide first (if forums on). 2. Work early, save for rifle.
3. Don't waste money on baby food past Day 2. 4. Always feed before dusk.`,

	"hidden": `Things the Game Doesn't Tell You
1. Energy restores +3/tick at night (sleeping). 2. 3rd consecutive same action = halved effect.
3. Medicine only works when health < 40. 4. Comfort has zero side effects (safe default).
5. Wrestle replaces Play for Kraken (Play gives only +5 to Kraken).`,
}
