package parsers

import (
	"fmt"
	"log"
	"sort"
)

type Fitting struct {
	Items []ListingItem
	lines []int
}

func (r *Fitting) Name() string {
	return "fitting"
}

func (r *Fitting) Lines() []int {
	return r.lines
}

var fittingBlacklist = map[string]bool{
	"High power":   true,
	"Medium power": true,
	"Low power":    true,
	"Rig Slot":     true,
	"Sub System":   true,
	"Charges":      true,
	"Drones":       true,
	"Fuel":         true,
}

func ParseFitting(input Input) (ParserResult, Input) {
	fitting := &Fitting{}

	// remove blacklisted lines
	isFitting := false
	for i, line := range input {
		_, blacklisted := fittingBlacklist[line]
		if blacklisted {
			isFitting = true
			fitting.lines = append(fitting.lines, i)
			delete(input, i)
		}
	}
	if !isFitting {
		return nil, input
	}

	result, rest := ParseListing(input)
	listingResult, ok := result.(*Listing)
	if !ok {
		log.Fatal("ParseListing returned something other than parsers.Listing")
	}
	fitting.Items = listingResult.Items
	fitting.lines = append(fitting.lines, listingResult.Lines()...)

	sort.Slice(fitting.Items, func(i, j int) bool {
		return fmt.Sprintf("%v", fitting.Items[i]) < fmt.Sprintf("%v", fitting.Items[j])
	})
	sort.Ints(fitting.lines)
	return fitting, rest
}
