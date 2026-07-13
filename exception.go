package gonest

import "github.com/gonest-dev/gonest/internal/exception"

// Exception is the single assertion point for "is this value a structured
// HTTP exception". It is satisfied purely structurally -- any type that
// embeds HttpException gets Status/Name/Message/Details promoted
// automatically, with no explicit "implements Exception" needed. See
// internal/exception.Exception's doc comment for the full contract.
type Exception = exception.Exception

// HttpException is the concrete carrier of an exception's four pieces of
// data (status, name, message, details). It is meant to be embedded BY
// VALUE by both the framework's built-in exceptions (below) and any
// dev-defined exception type, e.g. INSIGHT.md's
// `type FooExampleError struct { gonest.HttpException }`. See
// internal/exception.HttpException's doc comment for the full contract.
type HttpException = exception.HttpException

// NewHttpException builds an HttpException from its four parts, returning a
// VALUE (not a pointer) for embedding into a struct literal, e.g.
// `HttpException: gonest.NewHttpException(status, name, message, details)`.
// Unlike NewApp/MustParam elsewhere in this package, NewHttpException is not
// generic, so Go allows aliasing the plain func directly via var -- no
// wrapper function is needed. See internal/exception.NewHttpException's doc
// comment for the full contract.
var NewHttpException = exception.NewHttpException

// NotFoundException is the framework's built-in exception for a missing
// resource.
type NotFoundException = exception.NotFoundException

// NewNotFoundException builds a *NotFoundException fixed at
// http.StatusNotFound with name "NotFoundException". See
// internal/exception.NewNotFoundException's doc comment for the
// pointer-return and empty-message rationale.
var NewNotFoundException = exception.NewNotFoundException

// BadRequestException is the framework's built-in exception for a malformed
// or invalid request.
type BadRequestException = exception.BadRequestException

// NewBadRequestException builds a *BadRequestException fixed at
// http.StatusBadRequest with name "BadRequestException". See
// internal/exception.NewBadRequestException's doc comment for the
// pointer-return and empty-message rationale.
var NewBadRequestException = exception.NewBadRequestException

// ConflictException is the framework's built-in exception for a request
// that conflicts with the current state of a resource.
type ConflictException = exception.ConflictException

// NewConflictException builds a *ConflictException fixed at
// http.StatusConflict with name "ConflictException". See
// internal/exception.NewConflictException's doc comment for the
// pointer-return and empty-message rationale.
var NewConflictException = exception.NewConflictException

// UnauthorizedException is the framework's built-in exception for a missing
// or invalid authentication credential.
type UnauthorizedException = exception.UnauthorizedException

// NewUnauthorizedException builds a *UnauthorizedException fixed at
// http.StatusUnauthorized with name "UnauthorizedException". See
// internal/exception.NewUnauthorizedException's doc comment for the
// pointer-return and empty-message rationale.
var NewUnauthorizedException = exception.NewUnauthorizedException

// ForbiddenException is the framework's built-in exception for a request
// that is authenticated but not permitted.
type ForbiddenException = exception.ForbiddenException

// NewForbiddenException builds a *ForbiddenException fixed at
// http.StatusForbidden with name "ForbiddenException". See
// internal/exception.NewForbiddenException's doc comment for the
// pointer-return and empty-message rationale.
var NewForbiddenException = exception.NewForbiddenException
