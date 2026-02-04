package main

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// CreateDTMSagaSpan cria um span específico para operações SAGA do DTM
func CreateDTMSagaSpan(ctx context.Context, operationName string, gid string) (context.Context, trace.Span) {
	tracer := otel.Tracer("dtm-saga")
	ctx, span := tracer.Start(ctx, "dtm."+operationName)

	span.SetAttributes(
		attribute.String("dtm.gid", gid),
		attribute.String("dtm.operation", operationName),
		attribute.String("component", "dtm-coordinator"),
	)

	return ctx, span
}

// CreateDTMActionSpan cria um span para uma ação específica da SAGA
func CreateDTMActionSpan(ctx context.Context, actionName string, gid string, actionURL string) (context.Context, trace.Span) {
	tracer := otel.Tracer("dtm-saga")
	ctx, span := tracer.Start(ctx, "dtm.action."+actionName)

	span.SetAttributes(
		attribute.String("dtm.gid", gid),
		attribute.String("dtm.action.name", actionName),
		attribute.String("dtm.action.url", actionURL),
		attribute.String("component", "dtm-coordinator"),
	)

	return ctx, span
}
