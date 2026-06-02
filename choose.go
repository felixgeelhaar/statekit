package statekit

// ChooseBranch is one arm of a Choose action: when When passes (or is nil,
// acting as an "else"), Then runs and evaluation stops (v1.x).
type ChooseBranch[C any] struct {
	// When gates this branch. A nil When always matches — use it as the
	// final else branch.
	When Guard[C]
	// Then is the action executed when this branch is selected. May be nil.
	Then Action[C]
}

// Choose builds an Action that runs the Then of the first branch whose When
// guard passes, then stops — the action equivalent of XState's choose(). A
// branch with a nil When acts as an else. If no branch matches, it is a no-op.
//
// Register it like any named action and reference it from transitions or
// entry/exit hooks:
//
//	WithAction("classify", statekit.Choose(
//	    statekit.ChooseBranch[Ctx]{When: isGold, Then: tagGold},
//	    statekit.ChooseBranch[Ctx]{When: isSilver, Then: tagSilver},
//	    statekit.ChooseBranch[Ctx]{Then: tagBronze}, // else
//	))
func Choose[C any](branches ...ChooseBranch[C]) Action[C] {
	return func(ctx *C, event Event) {
		for _, b := range branches {
			if b.When == nil || b.When(*ctx, event) {
				if b.Then != nil {
					b.Then(ctx, event)
				}
				return
			}
		}
	}
}
