package firewall

import (
	"fmt"
	"github.com/TIR3D4/GreFlow/core/config"
	"strings"
)

// Update changes only the difference in owned rules, preserving hooks, GRE links
// and unrelated mappings. Caller holds the host lock and journals both configs.
func (b IPTables) Update(old, next config.Config) error {
	for _, x := range b.chains() {
		if e := b.owned(x[0], x[2]); e != nil {
			return e
		}
	}
	key := func(r Rule) string { return r.Table + "\x00" + r.Chain + "\x00" + strings.Join(r.Args, "\x00") }
	before, after := map[string]bool{}, map[string]bool{}
	for _, r := range b.Rules(old) {
		before[key(r)] = true
	}
	for _, r := range b.Rules(next) {
		after[key(r)] = true
	}
	for _, r := range b.Rules(old) {
		if !after[key(r)] {
			if _, e := b.run(r.Table, append([]string{"-C", r.Chain}, b.ruleArgs(r.Args)...)...); e == nil {
				if _, e = b.run(r.Table, append([]string{"-D", r.Chain}, b.ruleArgs(r.Args)...)...); e != nil {
					return e
				}
			}
		}
	}
	for _, r := range b.Rules(next) {
		if !before[key(r)] {
			if _, e := b.run(r.Table, append([]string{"-C", r.Chain}, b.ruleArgs(r.Args)...)...); e != nil {
				if _, e = b.run(r.Table, append([]string{"-A", r.Chain}, b.ruleArgs(r.Args)...)...); e != nil {
					return e
				}
			}
		}
	}
	if e := b.Check(next); e != nil {
		return fmt.Errorf("rule readback: %w", e)
	}
	return nil
}
