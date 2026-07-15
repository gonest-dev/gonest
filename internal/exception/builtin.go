package exception

import "net/http"

// NotFoundException is the framework's built-in exception for a missing
// resource. It embeds HttpException by value, which promotes
// Status/Name/Message/Details and satisfies Exception for free -- the same
// pattern INSIGHT.md's dev-defined-exception example follows.
type NotFoundException struct {
	HttpException
}

// NewNotFoundException builds a *NotFoundException fixed at
// http.StatusNotFound with name "NotFoundException". It returns a POINTER
// (unlike NewHttpException's value return) so that panic/recover round-trips
// via a type assertion to *NotFoundException, matching INSIGHT.md's own test
// pattern (`exc, ok := recover().(*gonest.NotFoundException)`). No message
// parameter is exposed -- INSIGHT.md never passes one to a built-in
// constructor, so message always defaults to "".
func NewNotFoundException(details any) *NotFoundException {
	return &NotFoundException{
		HttpException: NewHttpException().SetStatus(http.StatusNotFound).SetName("NotFoundException").SetDetails(details),
	}
}

// BadRequestException is the framework's built-in exception for a malformed
// or invalid request.
type BadRequestException struct {
	HttpException
}

// NewBadRequestException builds a *BadRequestException fixed at
// http.StatusBadRequest with name "BadRequestException". See
// NewNotFoundException's doc comment for the pointer-return and
// empty-message rationale, which applies identically here.
func NewBadRequestException(details any) *BadRequestException {
	return &BadRequestException{
		HttpException: NewHttpException().SetStatus(http.StatusBadRequest).SetName("BadRequestException").SetDetails(details),
	}
}

// ConflictException is the framework's built-in exception for a request
// that conflicts with the current state of a resource.
type ConflictException struct {
	HttpException
}

// NewConflictException builds a *ConflictException fixed at
// http.StatusConflict with name "ConflictException". See
// NewNotFoundException's doc comment for the pointer-return and
// empty-message rationale, which applies identically here.
func NewConflictException(details any) *ConflictException {
	return &ConflictException{
		HttpException: NewHttpException().SetStatus(http.StatusConflict).SetName("ConflictException").SetDetails(details),
	}
}

// UnauthorizedException is the framework's built-in exception for a missing
// or invalid authentication credential.
type UnauthorizedException struct {
	HttpException
}

// NewUnauthorizedException builds a *UnauthorizedException fixed at
// http.StatusUnauthorized with name "UnauthorizedException". See
// NewNotFoundException's doc comment for the pointer-return and
// empty-message rationale, which applies identically here.
func NewUnauthorizedException(details any) *UnauthorizedException {
	return &UnauthorizedException{
		HttpException: NewHttpException().SetStatus(http.StatusUnauthorized).SetName("UnauthorizedException").SetDetails(details),
	}
}

// ForbiddenException is the framework's built-in exception for a request
// that is authenticated but not permitted.
type ForbiddenException struct {
	HttpException
}

// NewForbiddenException builds a *ForbiddenException fixed at
// http.StatusForbidden with name "ForbiddenException". See
// NewNotFoundException's doc comment for the pointer-return and
// empty-message rationale, which applies identically here.
func NewForbiddenException(details any) *ForbiddenException {
	return &ForbiddenException{
		HttpException: NewHttpException().SetStatus(http.StatusForbidden).SetName("ForbiddenException").SetDetails(details),
	}
}
