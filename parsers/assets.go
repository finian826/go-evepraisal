package parsers

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type AssetList struct {
	Items []AssetItem
	lines []int
}

func (r *AssetList) Name() string {
	return "assets"
}

func (r *AssetList) Lines() []int {
	return r.lines
}

type AssetItem struct {
	Name          string
	Quantity      int64
	Volume        float64
	Group         string
	Category      string
	Size          string
	Slot          string
	MetaLevel     string
	TechLevel     string
	PriceEstimate float64
}

var reAssetList = regexp.MustCompile(strings.Join([]string{
	`^([\S\ ]*)`,                           // Name
	`\t([\d,'\.\ ]*)`,                      // Quantity
	`(?:\t([\S ]*))?`,                      // Group
	`(?:\t([\S ]*))?`,                      // Category
	`(?:\t(XLarge|Large|Medium|Small|))?`,  // Size
	`(?:\t(High|Medium|Low|Rigs|[\d ]*))?`, // Slot
	`(?:\t([\d ,\.]*) m3)?`,                // Volume
	`(?:\t([\d]+|))?`,                      // meta level
	`(?:\t([\d]+|))?`,                      // tech level
	`(?:\t([\d,'\.\ ]+) ISK)?$`,            // price estimate
}, ""))

func ParseAssets(input Input) (ParserResult, Input) {
	assetList := &AssetList{}
	matches, rest := regexParseLines(reAssetList, input)
	assetList.lines = regexMatchedLines(matches)
	for _, match := range matches {
		qty := ToInt(match[2])
		if qty == 0 {
			qty = 1
		}

		assetList.Items = append(assetList.Items,
			AssetItem{
				Name:          CleanTypeName(match[1]),
				Quantity:      qty,
				Volume:        ToFloat64(match[7]),
				Group:         match[3],
				Category:      match[4],
				Size:          match[5],
				Slot:          match[6],
				MetaLevel:     match[8],
				TechLevel:     match[9],
				PriceEstimate: ToFloat64(match[10]),
			})
	}
	sort.Slice(assetList.Items, func(i, j int) bool {
		return fmt.Sprintf("%v", assetList.Items[i]) < fmt.Sprintf("%v", assetList.Items[j])
	})
	return assetList, rest
}
