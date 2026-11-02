package go_subcommand

import (
	"fmt"
	"strings"

	"github.com/arran4/go-subcommand/model"
)

type plusMinusNames struct{ Short, Long []string }

func plusMinusParameterNames(p *model.FunctionParameter) plusMinusNames {
	var result plusMinusNames
	seenShort := make(map[string]bool)
	seenLong := make(map[string]bool)

	for _, s := range p.ShortAliases {
		if !seenShort[s] {
			result.Short = append(result.Short, s)
			seenShort[s] = true
		}
	}
	for _, l := range p.LongAliases {
		if !seenLong[l] {
			result.Long = append(result.Long, l)
			seenLong[l] = true
		}
	}

	if p.Inherited {
		return result
	}

	for _, name := range append([]string{p.Name}, p.FlagAliases...) {
		if len(name) == 1 {
			if !seenShort[name] {
				result.Short = append(result.Short, name)
				seenShort[name] = true
			}
		} else {
			if !seenLong[name] {
				result.Long = append(result.Long, name)
				seenLong[name] = true
			}
		}
	}
	return result
}

func plusMinusAllParameters(data any) []*model.FunctionParameter {
	var params []*model.FunctionParameter
	seenParamName := make(map[string]bool)
	seenShort := make(map[string]bool)
	seenLong := make(map[string]bool)

	addParam := func(p *model.FunctionParameter, isInherited bool) {
		if p.IsPositional || p.HasGenerator() || p.HasConstructor() {
			return
		}
		if seenParamName[p.Name] {
			return
		}

		names := plusMinusParameterNames(p)
		var uniqueShort []string
		var uniqueLong []string

		for _, s := range names.Short {
			if !seenShort[s] {
				uniqueShort = append(uniqueShort, s)
			}
		}
		for _, l := range names.Long {
			if !seenLong[l] {
				uniqueLong = append(uniqueLong, l)
			}
		}

		// If all names would be shadowed by child or nearer ancestor, skip this ancestor param
		if isInherited && len(uniqueShort) == 0 && len(uniqueLong) == 0 {
			return
		}

		for _, s := range uniqueShort {
			seenShort[s] = true
		}
		for _, l := range uniqueLong {
			seenLong[l] = true
		}
		seenParamName[p.Name] = true

		if isInherited {
			inheritedParam := *p
			inheritedParam.Inherited = true
			if inheritedParam.InheritedFrom == "" {
				inheritedParam.InheritedFrom = p.Name
			}
			inheritedParam.ShortAliases = uniqueShort
			inheritedParam.LongAliases = uniqueLong
			params = append(params, &inheritedParam)
		} else {
			params = append(params, p)
		}
	}

	switch cmd := data.(type) {
	case *model.Command:
		for _, p := range cmd.Parameters {
			addParam(p, false)
		}
	case *model.SubCommand:
		for _, p := range cmd.Parameters {
			addParam(p, p.Inherited)
		}
		curr := cmd.Parent
		for curr != nil {
			for _, p := range curr.Parameters {
				addParam(p, true)
			}
			curr = curr.Parent
		}
		if cmd.Command != nil {
			for _, p := range cmd.Command.Parameters {
				addParam(p, true)
			}
		}
	}
	return params
}

func validatePlusMinusParameters(path string, params []*model.FunctionParameter, ancestors ...[]*model.FunctionParameter) error {
	short, long := make(map[string]string), make(map[string]string)

	ancestorShort, ancestorLong := make(map[string]string), make(map[string]string)
	for _, aList := range ancestors {
		for _, p := range aList {
			if p.IsPositional || p.HasGenerator() || p.HasConstructor() {
				continue
			}
			names := plusMinusParameterNames(p)
			for _, s := range names.Short {
				ancestorShort[s] = p.Name
			}
			for _, l := range names.Long {
				ancestorLong[l] = p.Name
			}
		}
	}

	for _, p := range params {
		if p.IsPositional || p.Inherited || p.HasGenerator() || p.HasConstructor() {
			continue
		}
		if p.IsBool() && (p.IsSlice() || p.HasCustomParser()) {
			return fmt.Errorf("command %s: plus-minus requires scalar booleans without custom parsers (%s)", path, p.Name)
		}
		if p.PlusBare.Mode != "" && p.PlusBare.Mode != "reset" && p.PlusBare.Mode != "zero" && p.PlusBare.Mode != "func" {
			return fmt.Errorf("command %s: invalid plus-bare mode %q on parameter %s", path, p.PlusBare.Mode, p.Name)
		}
		if p.PlusValue.Mode != "" && p.PlusValue.Mode != "set" && p.PlusValue.Mode != "func" {
			return fmt.Errorf("command %s: invalid plus-value mode %q on parameter %s", path, p.PlusValue.Mode, p.Name)
		}
		names := plusMinusParameterNames(p)
		for _, name := range names.Short {
			if len(name) != 1 || !asciiLetter(name[0]) || name == "h" {
				return fmt.Errorf("command %s: plus-minus short name %q requires an ASCII letter other than h", path, name)
			}
			if owner := short[name]; owner != "" {
				return fmt.Errorf("command %s: plus-minus short name %q collides between %s and %s", path, name, owner, p.Name)
			}
			short[name] = p.Name
		}
		for _, name := range names.Long {
			valid := len(name) >= 1 && (asciiLetter(name[0]) || name[0] == '_')
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
		if !p.HasGenerator() && !p.HasConstructor() && !p.HasCustomParser() && strings.Contains(p.ParserCall("value"), "strconv.") {
			return true
		}
	}
	return false
}
