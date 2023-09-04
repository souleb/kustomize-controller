package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	celext "github.com/google/cel-go/ext"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
)

// https://github.com/tektoncd/triggers/blob/main/pkg/interceptors/cel/cel.go

func evaluate(expr string, env *cel.Env, data map[string]interface{}) (ref.Val, error) {
	ast, issues := env.Parse(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to parse expression %v: %w", expr, issues.Err())
	}

	checked, issues := env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("expression %v check failed: %w", expr, issues.Err())
	}

	prg, err := env.Program(checked, cel.EvalOptions(cel.OptOptimize, cel.OptTrackCost), cel.CostLimit(celconfig.PerCallLimit))
	if err != nil {
		return nil, fmt.Errorf("expression %v failed to create a Program: %w", expr, err)
	}

	out, _, err := prg.Eval(data)
	if err != nil {
		return nil, fmt.Errorf("expression %#v failed to evaluate: %w", expr, err)
	}
	return out, nil
}

func makeCelEnv() (*cel.Env, error) {
	return cel.NewEnv(
		celext.Strings(),
		celext.Encoders(),
		celext.Lists(),
		celext.Math(),
		celext.Sets())
}

func ProcessExpr(expr string, obj map[string]any) (bool, error) {
	env, err := makeCelEnv()
	if err != nil {
		return false, err
	}
	out, err := evaluate(expr, env, obj)
	if err != nil {
		return false, err
	}
	if out == nil {
		return false, fmt.Errorf("expression %v returned nil", expr)
	}

	switch out.(type) {
	case types.Bool:
		return out.Value().(bool), nil
	default:
		return false, fmt.Errorf("expression %v returned non-boolean type %v", expr, out.Type())
	}
}
