package main

// Weapon is a per-type attack profile: how much damage a landed swing deals,
// how likely it is to land, and how much HR stress the swing costs. Cost
// climbs with weapon class — bare hands < light (dagger) < medium (sword)
// < heavy (axe, hammer) — and a miss costs only half the stress of a hit.
type Weapon struct {
	Name   string
	Damage int
	ToHit  float64
	HRCost float64
}

// bareHands is the reference baseline that all weapons are tuned against, so
// keep it stable: punching costs 10 HR on a hit and 5 on a miss. At that cost,
// shadow-boxing steadily lands the player in the mid-150s and frenzied input
// can pass them out — exactly the intended pacing for the empty-hand default.
// Future weapons (dagger, sword, axe, hammer) should scale Damage and HRCost
// up from this reference so heavier hits are proportionally more taxing.
func bareHands() Weapon {
	return Weapon{
		Name:   "bare hands",
		Damage: 1,
		ToHit:  0.8,
		HRCost: 10,
	}
}
