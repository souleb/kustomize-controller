package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/ext"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	k8s "k8s.io/apiserver/pkg/cel/library"
)

type EvalResponse struct {
	// Result is the result of the evaluation.
	// It is always a boolean value.
	Result bool `json:"result"`
	// Cost is the tracked cost through the course of execution of the expression.
	Cost *uint64 `json:"cost,omitempty"`
}

var envOptions = []cel.EnvOption{
	cel.HomogeneousAggregateLiterals(),
	cel.EagerlyValidateDeclarations(true),
	cel.DefaultUTCTimeZone(true),
	k8s.URLs(),
	k8s.Regex(),
	k8s.Lists(),

	// 1.27
	k8s.Authz(),

	// 1.28
	cel.CrossTypeNumericComparisons(true),
	cel.OptionalTypes(),
	k8s.Quantity(),

	// 1.29 (see also validator.ExtendedValidations())
	cel.ASTValidators(
		cel.ValidateDurationLiterals(),
		cel.ValidateTimestampLiterals(),
		cel.ValidateRegexLiterals(),
		cel.ValidateHomogeneousAggregateLiterals(),
	),

	// Strings (from 1.29 onwards)
	ext.Strings(ext.StringsVersion(2)),
	// Set library (1.29 onwards)
	ext.Sets(),
}

var programOptions = []cel.ProgramOption{
	cel.EvalOptions(cel.OptOptimize, cel.OptTrackCost),
	cel.CostLimit(celconfig.PerCallLimit),
}

// Eval evaluates the given expression with the given input.
// https://github.com/undistro/cel-playground/eval.go
func Eval(expr string, input map[string]any) (*EvalResponse, error) {
	vars := make([]cel.EnvOption, 0, len(input))
	for k := range input {
		vars = append(vars, cel.Variable(k, cel.DynType))
	}
	env, err := cel.NewEnv(append(envOptions, vars...)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL env: %w", err)
	}
	ast, issues := env.Compile(expr)
	if issues != nil {
		return nil, fmt.Errorf("failed to compile the CEL expression: %s", issues.String())
	}

	prog, err := env.Program(ast, programOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate CEL program: %w", err)
	}

	val, costTracker, err := prog.Eval(input)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate: %w", err)
	}

	response, err := generateResponse(val, costTracker)
	if err != nil {
		return nil, fmt.Errorf("failed to generate the response: %w", err)
	}

	return response, nil
}

func generateResponse(val ref.Val, costTracker *cel.EvalDetails) (*EvalResponse, error) {
	switch val.(type) {
	case types.Bool:
		cost := costTracker.ActualCost()
		return &EvalResponse{
			Result: val.Value().(bool),
			Cost:   cost,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported type: %s", val.Type().TypeName())
	}
}
