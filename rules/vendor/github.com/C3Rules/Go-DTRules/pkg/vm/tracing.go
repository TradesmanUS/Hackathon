package vm

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type keyFor[V any] struct{}

func StartSpan(s State, name string) (trace.Span, context.CancelFunc) {
	ogctx := s.Context()
	ctx, span := Tracer(ogctx, "").Start(ogctx, name)
	s.SetContext(ctx)
	return span, func() {
		span.End()
		s.SetContext(ogctx)
	}
}

func WithTraceProvider(ctx context.Context, p trace.TracerProvider) context.Context {
	return context.WithValue(ctx, keyFor[trace.TracerProvider]{}, p)
}

func Tracer(ctx context.Context, name string, opts ...trace.TracerOption) trace.Tracer {
	tp, ok := ctx.Value(keyFor[trace.TracerProvider]{}).(trace.TracerProvider)
	if ok {
		return tp.Tracer(name, opts...)
	}
	return otel.Tracer(name, opts...)
}
