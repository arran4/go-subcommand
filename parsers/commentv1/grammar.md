# Comment V1 Parser Grammar

This document describes the grammar for the comment-based command definition syntax supported by the `commentv1` parser.

## Grammar

The syntax is defined using ABNF (Augmented Backus-Naur Form) notation.

```abnf
; Top-level command definition in comments
CommandComment = [Description] [FlagsBlock]

; Description of the command
Description    = *TEXT

; Flags block definition
FlagsBlock     = "Flags:" CRLF *FlagDefinition

; Flag definition line
FlagDefinition = Indent (ExplicitParam / ImplicitParam) CRLF

; Indentation (space or tab)
Indent         = 1*(SP / HTAB)

; Explicit parameter definition
; e.g. "param Name: (required) User name"
ExplicitParam  = ("param" / "flag") SP ParamName ":" SP ParamDetails

; Implicit parameter definition
; e.g. "Name: @1 User name"
ImplicitParam  = ParamName ":" SP ParamDetails

; Parameter name
ParamName      = 1*(ALPHA / DIGIT / "_")

; Parameter details including attributes and description
ParamDetails   = [Block] *SP DescriptionText *SP [Block]

; Parenthesized block of attributes
; e.g. "(required; global; parser: MyParser)"
Block          = "(" Attribute *( ";" Attribute ) ")"

; Individual attribute
Attribute      = RequiredAttr
               / GlobalAttr
               / InheritedAttr
               / ParserAttr
               / GeneratorAttr
               / AliasAttr
               / DefaultAttr
               / PositionalAttr
               / VarArgAttr
               / FlagAttr

RequiredAttr   = "required"
GlobalAttr     = "global"
InheritedAttr  = "from parent"

; Parser function reference
; e.g. "parser: MyFunc" or "parser: pkg.Func"
ParserAttr     = "parser" [":"] SP FuncRef

; Generator function reference
; e.g. "generator: MyGen"
GeneratorAttr  = "generator" [":"] SP FuncRef

; Function reference
FuncRef        = [PackageName "."] FunctionName
PackageName    = 1*(ALPHA / DIGIT / "_")
FunctionName   = 1*(ALPHA / DIGIT / "_")

; Aliases/Flags
; e.g. "aka: n, name" or "alias: f"
AliasAttr      = ("alias" / "aliases" / "aka") [":"] SP AliasList
AliasList      = AliasName *( ("," / SP) AliasName )
AliasName      = 1*(ALPHA / DIGIT / "-" / "_")

; Default value
; e.g. "default: 'value'"
DefaultAttr    = "default" [":"] SP Value
Value          = QuotedString / UnquotedString
QuotedString   = DQUOTE *CHAR DQUOTE
UnquotedString = 1*(ALPHA / DIGIT / SYMBOL)

; Positional argument index
; e.g. "@1"
PositionalAttr = "@" 1*DIGIT

; Variadic argument constraint
; e.g. "1...3" or "..."
VarArgAttr     = [Min] "..." [Max]
Min            = 1*DIGIT
Max            = 1*DIGIT

; Flag declaration
; e.g. "-f" or "-flag"
FlagAttr       = "-" FlagName
FlagName       = 1*(ALPHA / DIGIT / "-" / "_")

; Remaining description text
DescriptionText = *TEXT
```

## Examples

### Basic Command

```go
// MyCmd is a subcommand `my-cmd`
// param Name: (required; aka n) The name of the user
// param Verbose: (global; default: false) Enable verbose output
func MyCmd(Name string, Verbose bool) { ... }
```

### Complex Attributes

```go
// param Input: (parser: ParseInput; required) Input file
// param Output: (generator: GenerateOutput) Output file
```

### Positional Arguments

```go
// param Source: @1 Source file
// param Dest: @2 Destination file
```

### Variadic Arguments

```go
// param Files: 1... (required) List of files
```
