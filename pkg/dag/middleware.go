package dag

import "context"

// MiddlewareFunc is the function signature for a handler in the middleware chain.
// It matches the Node.Run signature.
type MiddlewareFunc func(ctx context.Context, inputs map[string]any) (map[string]any, error)

// Middleware wraps a MiddlewareFunc, allowing pre/post inspection and modification
// of inputs and outputs. Call next to continue the chain; return an error to
// short-circuit (the handler is NOT called).
//
// Note: the inputs map is shared across the middleware chain and across retries.
// Mutations to inputs (e.g., adding keys) persist between retry attempts.
// Middlewares that need clean inputs per attempt should copy the map first.
type Middleware func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error)

// ChainMiddlewares composes multiple middlewares into a single middleware.
// Execution order follows the onion model: middlewares[0] is outermost
// (first to see inputs, last to see outputs).
func ChainMiddlewares(middlewares ...Middleware) Middleware {
	if len(middlewares) == 0 {
		return func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
			return next(ctx, inputs)
		}
	}
	if len(middlewares) == 1 {
		return middlewares[0]
	}
	return func(ctx context.Context, inputs map[string]any, next MiddlewareFunc) (map[string]any, error) {
		// Build the inner chain: middlewares[1:] wrapping next
		inner := buildChain(next, middlewares[1:])
		return middlewares[0](ctx, inputs, inner)
	}
}

// buildChain wraps a handler with the middleware chain, returning a single
// callable MiddlewareFunc. Returns handler unchanged if middlewares is empty
// (zero-overhead path).
func buildChain(handler MiddlewareFunc, middlewares []Middleware) MiddlewareFunc {
	if len(middlewares) == 0 {
		return handler
	}
	// Build from innermost to outermost: last middleware wraps handler first.
	chain := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		mw := middlewares[i]
		next := chain
		chain = func(ctx context.Context, inputs map[string]any) (map[string]any, error) {
			return mw(ctx, inputs, next)
		}
	}
	return chain
}
