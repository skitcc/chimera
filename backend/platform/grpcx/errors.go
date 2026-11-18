// Package grpcx wires gRPC servers and clients: error mapping, run id propagation, health.
package grpcx

import (
	"context"
	"errors"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"chimera/platform/apperr"
)

const errorDomain = "chimera"

var toGRPC = map[apperr.Code]codes.Code{
	apperr.CodeNotFound:        codes.NotFound,
	apperr.CodeInvalid:         codes.InvalidArgument,
	apperr.CodeUnauthorized:    codes.Unauthenticated,
	apperr.CodeForbidden:       codes.PermissionDenied,
	apperr.CodeConflict:        codes.FailedPrecondition,
	apperr.CodeTooManyRequests: codes.ResourceExhausted,
	apperr.CodeUnavailable:     codes.Unavailable,
	apperr.CodeInternal:        codes.Internal,
}

var fromGRPC = map[codes.Code]apperr.Code{
	codes.NotFound:           apperr.CodeNotFound,
	codes.InvalidArgument:    apperr.CodeInvalid,
	codes.Unauthenticated:    apperr.CodeUnauthorized,
	codes.PermissionDenied:   apperr.CodeForbidden,
	codes.FailedPrecondition: apperr.CodeConflict,
	codes.AlreadyExists:      apperr.CodeConflict,
	codes.ResourceExhausted:  apperr.CodeTooManyRequests,
	codes.Unavailable:        apperr.CodeUnavailable,
	codes.DeadlineExceeded:   apperr.CodeUnavailable,
}

// ToStatus turns an application error into a gRPC status that carries the code
// in ErrorInfo.reason. Internal errors lose their details on the wire.
func ToStatus(err error) error {
	app, ok := apperr.As(err)
	if !ok || app.Code == apperr.CodeInternal {
		return status.Error(codes.Internal, "internal error")
	}
	st := status.New(toGRPC[app.Code], app.Message)
	withInfo, detailErr := st.WithDetails(&errdetails.ErrorInfo{Reason: string(app.Code), Domain: errorDomain})
	if detailErr != nil {
		return st.Err()
	}
	return withInfo.Err()
}

// FromError turns a gRPC client error back into an application error.
// Without ErrorInfo the gRPC code decides; an unreachable peer becomes unavailable.
func FromError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return apperr.Wrap(apperr.CodeUnavailable, "service unavailable", err)
	}
	st, ok := status.FromError(err)
	if !ok {
		return apperr.Wrap(apperr.CodeInternal, "grpc call", err)
	}
	for _, d := range st.Details() {
		info, ok := d.(*errdetails.ErrorInfo)
		if ok && info.GetDomain() == errorDomain {
			return apperr.NewError(apperr.Code(info.GetReason()), st.Message())
		}
	}
	code, ok := fromGRPC[st.Code()]
	if !ok {
		return apperr.Wrap(apperr.CodeInternal, "grpc call", err)
	}
	if code == apperr.CodeUnavailable {
		return apperr.Wrap(code, "service unavailable", err)
	}
	return apperr.NewError(code, st.Message())
}
