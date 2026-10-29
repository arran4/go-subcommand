package go_subcommand

import (
	"fmt"
	"strings"

	"github.com/arran4/go-subcommand/model"
)

type plusMinusNames struct{ Short, Long []string }

func plusMinusParameterNames(p *model.FunctionParameter) plusMinusNames {
	var result plusMinusNames
	seen := make(map[string]bool)
	for _, name := range append([]string{p.Name}, p.FlagAliases...) {
		if seen[name] {
			continue
		}
		seen[name] = true
		if len(name) == 1 {
			result.Short = append(result.Short, name)
		} else {
			result.Long = append(result.Long, name)
		}
	}
	return result
}

func validatePlusMinusParameters(path string, params []*model.FunctionParameter) error {
	short, long := make(map[string]string), make(map[string]string)
	for _, p := range params {
		if p.IsPositional || p.Inherited || p.HasGenerator() || p.HasConstructor() {
			continue
		}
		if p.IsBool() && (p.IsSlice() || p.HasCustomParser()) {
			return fmt.Errorf("command %s: plus-minus requires scalar booleans without custom parsers (%s)", path, p.Name)
		}
		names := plusMinusParameterNames(p)
		for _, name := range names.Short {
			if !p.IsBool() || !asciiLetter(name[0]) || name == "h" {
				return fmt.Errorf("command %s: plus-minus short name %q requires a scalar boolean and an ASCII letter other than h", path, name)
			}
			if owner := short[name]; owner != "" {
				return fmt.Errorf("command %s: plus-minus short name %q collides between %s and %s", path, name, owner, p.Name)
			}
			short[name] = p.Name
		}
		for _, name := range names.Long {
			valid := len(name) >= 2 && (asciiLetter(name[0]) || name[0] == '_')
			for _, c := range []byte(name) {
				valid = valid && (asciiLetter(c) || c >= '0' && c <= '9' || c == '_' || c == '-')
			}
			if !valid || name == "help" {
				return fmt.Errorf("command %s: invalid or reserved plus-minus long name %q", path, name)
			}
			if owner := long[name]; owner != "" {
				return fmt.Errorf("command %s: plus-minus long name %q collides between %s and %s", path, name, owner, p.Name)
			}
			long[name] = p.Name
		}
	}
	return nil
}

func asciiLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func commandUsesPlusMinus(cmd *model.Command) bool {
	if cmd.ResolvedCLIParser == "plus-minus" {
		return true
	}
	var visit func([]*model.SubCommand) bool
	visit = func(children []*model.SubCommand) bool {
		for _, child := range children {
			if child.ResolvedCLIParser == "plus-minus" || visit(child.SubCommands) {
				return true
			}
		}
		return false
	}
	return visit(cmd.SubCommands)
}

func plusMinusConversionNeedsStrconv(params []*model.FunctionParameter) bool {
	for _, p := range params {
		if !p.HasGenerator() && !p.HasConstructor() && !p.Inherited && !p.HasCustomParser() && strings.Contains(p.ParserCall("value"), "strconv.") {
			return true
		}
	}
	return false
}
