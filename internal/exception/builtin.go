package exception

import "net/http"

func newBuiltin[T any](
	wrap func(HttpException) T,
	status int, name, defaultMessage string,
	message string, details []any,
) *T {
	msg := message
	if msg == "" {
		msg = defaultMessage
	}
	var det any
	if len(details) > 0 {
		det = details[0]
	}
	exc := wrap(NewHttpException().
		SetStatus(status).
		SetName(name).
		SetMessage(msg).
		SetDetails(det))
	return &exc
}

// ---------------------------------------------------------------------------
// 4xx Client Errors
// ---------------------------------------------------------------------------

type BadRequestException struct{ HttpException }

func NewBadRequestException(message string, details ...any) *BadRequestException {
	return newBuiltin(func(h HttpException) BadRequestException { return BadRequestException{h} },
		http.StatusBadRequest, "BadRequestException", "Bad request", message, details)
}

type UnauthorizedException struct{ HttpException }

func NewUnauthorizedException(message string, details ...any) *UnauthorizedException {
	return newBuiltin(func(h HttpException) UnauthorizedException { return UnauthorizedException{h} },
		http.StatusUnauthorized, "UnauthorizedException", "Unauthorized", message, details)
}

type PaymentRequiredException struct{ HttpException }

func NewPaymentRequiredException(message string, details ...any) *PaymentRequiredException {
	return newBuiltin(func(h HttpException) PaymentRequiredException { return PaymentRequiredException{h} },
		http.StatusPaymentRequired, "PaymentRequiredException", "Payment required", message, details)
}

type ForbiddenException struct{ HttpException }

func NewForbiddenException(message string, details ...any) *ForbiddenException {
	return newBuiltin(func(h HttpException) ForbiddenException { return ForbiddenException{h} },
		http.StatusForbidden, "ForbiddenException", "Forbidden", message, details)
}

type NotFoundException struct{ HttpException }

func NewNotFoundException(message string, details ...any) *NotFoundException {
	return newBuiltin(func(h HttpException) NotFoundException { return NotFoundException{h} },
		http.StatusNotFound, "NotFoundException", "Resource not found", message, details)
}

type MethodNotAllowedException struct{ HttpException }

func NewMethodNotAllowedException(message string, details ...any) *MethodNotAllowedException {
	return newBuiltin(func(h HttpException) MethodNotAllowedException { return MethodNotAllowedException{h} },
		http.StatusMethodNotAllowed, "MethodNotAllowedException", "Method not allowed", message, details)
}

type NotAcceptableException struct{ HttpException }

func NewNotAcceptableException(message string, details ...any) *NotAcceptableException {
	return newBuiltin(func(h HttpException) NotAcceptableException { return NotAcceptableException{h} },
		http.StatusNotAcceptable, "NotAcceptableException", "Not acceptable", message, details)
}

type ProxyAuthRequiredException struct{ HttpException }

func NewProxyAuthRequiredException(message string, details ...any) *ProxyAuthRequiredException {
	return newBuiltin(func(h HttpException) ProxyAuthRequiredException { return ProxyAuthRequiredException{h} },
		http.StatusProxyAuthRequired, "ProxyAuthRequiredException", "Proxy authentication required", message, details)
}

type RequestTimeoutException struct{ HttpException }

func NewRequestTimeoutException(message string, details ...any) *RequestTimeoutException {
	return newBuiltin(func(h HttpException) RequestTimeoutException { return RequestTimeoutException{h} },
		http.StatusRequestTimeout, "RequestTimeoutException", "Request timeout", message, details)
}

type ConflictException struct{ HttpException }

func NewConflictException(message string, details ...any) *ConflictException {
	return newBuiltin(func(h HttpException) ConflictException { return ConflictException{h} },
		http.StatusConflict, "ConflictException", "Conflict", message, details)
}

type GoneException struct{ HttpException }

func NewGoneException(message string, details ...any) *GoneException {
	return newBuiltin(func(h HttpException) GoneException { return GoneException{h} },
		http.StatusGone, "GoneException", "Gone", message, details)
}

type LengthRequiredException struct{ HttpException }

func NewLengthRequiredException(message string, details ...any) *LengthRequiredException {
	return newBuiltin(func(h HttpException) LengthRequiredException { return LengthRequiredException{h} },
		http.StatusLengthRequired, "LengthRequiredException", "Length required", message, details)
}

type PreconditionFailedException struct{ HttpException }

func NewPreconditionFailedException(message string, details ...any) *PreconditionFailedException {
	return newBuiltin(func(h HttpException) PreconditionFailedException { return PreconditionFailedException{h} },
		http.StatusPreconditionFailed, "PreconditionFailedException", "Precondition failed", message, details)
}

type RequestEntityTooLargeException struct{ HttpException }

func NewRequestEntityTooLargeException(message string, details ...any) *RequestEntityTooLargeException {
	return newBuiltin(func(h HttpException) RequestEntityTooLargeException { return RequestEntityTooLargeException{h} },
		http.StatusRequestEntityTooLarge, "RequestEntityTooLargeException", "Request entity too large", message, details)
}

type RequestURITooLongException struct{ HttpException }

func NewRequestURITooLongException(message string, details ...any) *RequestURITooLongException {
	return newBuiltin(func(h HttpException) RequestURITooLongException { return RequestURITooLongException{h} },
		http.StatusRequestURITooLong, "RequestURITooLongException", "Request URI too long", message, details)
}

type UnsupportedMediaTypeException struct{ HttpException }

func NewUnsupportedMediaTypeException(message string, details ...any) *UnsupportedMediaTypeException {
	return newBuiltin(func(h HttpException) UnsupportedMediaTypeException { return UnsupportedMediaTypeException{h} },
		http.StatusUnsupportedMediaType, "UnsupportedMediaTypeException", "Unsupported media type", message, details)
}

type RequestedRangeNotSatisfiableException struct{ HttpException }

func NewRequestedRangeNotSatisfiableException(message string, details ...any) *RequestedRangeNotSatisfiableException {
	return newBuiltin(func(h HttpException) RequestedRangeNotSatisfiableException {
		return RequestedRangeNotSatisfiableException{h}
	}, http.StatusRequestedRangeNotSatisfiable, "RequestedRangeNotSatisfiableException", "Requested range not satisfiable", message, details)
}

type ExpectationFailedException struct{ HttpException }

func NewExpectationFailedException(message string, details ...any) *ExpectationFailedException {
	return newBuiltin(func(h HttpException) ExpectationFailedException { return ExpectationFailedException{h} },
		http.StatusExpectationFailed, "ExpectationFailedException", "Expectation failed", message, details)
}

type TeapotException struct{ HttpException }

func NewTeapotException(message string, details ...any) *TeapotException {
	return newBuiltin(func(h HttpException) TeapotException { return TeapotException{h} },
		http.StatusTeapot, "TeapotException", "I'm a teapot", message, details)
}

type MisdirectedRequestException struct{ HttpException }

func NewMisdirectedRequestException(message string, details ...any) *MisdirectedRequestException {
	return newBuiltin(func(h HttpException) MisdirectedRequestException { return MisdirectedRequestException{h} },
		http.StatusMisdirectedRequest, "MisdirectedRequestException", "Misdirected request", message, details)
}

type UnprocessableEntityException struct{ HttpException }

func NewUnprocessableEntityException(message string, details ...any) *UnprocessableEntityException {
	return newBuiltin(func(h HttpException) UnprocessableEntityException { return UnprocessableEntityException{h} },
		http.StatusUnprocessableEntity, "UnprocessableEntityException", "Unprocessable entity", message, details)
}

type LockedException struct{ HttpException }

func NewLockedException(message string, details ...any) *LockedException {
	return newBuiltin(func(h HttpException) LockedException { return LockedException{h} },
		http.StatusLocked, "LockedException", "Locked", message, details)
}

type FailedDependencyException struct{ HttpException }

func NewFailedDependencyException(message string, details ...any) *FailedDependencyException {
	return newBuiltin(func(h HttpException) FailedDependencyException { return FailedDependencyException{h} },
		http.StatusFailedDependency, "FailedDependencyException", "Failed dependency", message, details)
}

type TooEarlyException struct{ HttpException }

func NewTooEarlyException(message string, details ...any) *TooEarlyException {
	return newBuiltin(func(h HttpException) TooEarlyException { return TooEarlyException{h} },
		http.StatusTooEarly, "TooEarlyException", "Too early", message, details)
}

type UpgradeRequiredException struct{ HttpException }

func NewUpgradeRequiredException(message string, details ...any) *UpgradeRequiredException {
	return newBuiltin(func(h HttpException) UpgradeRequiredException { return UpgradeRequiredException{h} },
		http.StatusUpgradeRequired, "UpgradeRequiredException", "Upgrade required", message, details)
}

type PreconditionRequiredException struct{ HttpException }

func NewPreconditionRequiredException(message string, details ...any) *PreconditionRequiredException {
	return newBuiltin(func(h HttpException) PreconditionRequiredException { return PreconditionRequiredException{h} },
		http.StatusPreconditionRequired, "PreconditionRequiredException", "Precondition required", message, details)
}

type TooManyRequestsException struct{ HttpException }

func NewTooManyRequestsException(message string, details ...any) *TooManyRequestsException {
	return newBuiltin(func(h HttpException) TooManyRequestsException { return TooManyRequestsException{h} },
		http.StatusTooManyRequests, "TooManyRequestsException", "Too many requests", message, details)
}

type RequestHeaderFieldsTooLargeException struct{ HttpException }

func NewRequestHeaderFieldsTooLargeException(message string, details ...any) *RequestHeaderFieldsTooLargeException {
	return newBuiltin(func(h HttpException) RequestHeaderFieldsTooLargeException {
		return RequestHeaderFieldsTooLargeException{h}
	}, http.StatusRequestHeaderFieldsTooLarge, "RequestHeaderFieldsTooLargeException", "Request header fields too large", message, details)
}

type UnavailableForLegalReasonsException struct{ HttpException }

func NewUnavailableForLegalReasonsException(message string, details ...any) *UnavailableForLegalReasonsException {
	return newBuiltin(func(h HttpException) UnavailableForLegalReasonsException {
		return UnavailableForLegalReasonsException{h}
	}, http.StatusUnavailableForLegalReasons, "UnavailableForLegalReasonsException", "Unavailable for legal reasons", message, details)
}

// ---------------------------------------------------------------------------
// 5xx Server Errors
// ---------------------------------------------------------------------------

type InternalServerErrorException struct{ HttpException }

func NewInternalServerErrorException(message string, details ...any) *InternalServerErrorException {
	return newBuiltin(func(h HttpException) InternalServerErrorException { return InternalServerErrorException{h} },
		http.StatusInternalServerError, "InternalServerErrorException", "Internal server error", message, details)
}

type NotImplementedException struct{ HttpException }

func NewNotImplementedException(message string, details ...any) *NotImplementedException {
	return newBuiltin(func(h HttpException) NotImplementedException { return NotImplementedException{h} },
		http.StatusNotImplemented, "NotImplementedException", "Not implemented", message, details)
}

type BadGatewayException struct{ HttpException }

func NewBadGatewayException(message string, details ...any) *BadGatewayException {
	return newBuiltin(func(h HttpException) BadGatewayException { return BadGatewayException{h} },
		http.StatusBadGateway, "BadGatewayException", "Bad gateway", message, details)
}

type ServiceUnavailableException struct{ HttpException }

func NewServiceUnavailableException(message string, details ...any) *ServiceUnavailableException {
	return newBuiltin(func(h HttpException) ServiceUnavailableException { return ServiceUnavailableException{h} },
		http.StatusServiceUnavailable, "ServiceUnavailableException", "Service unavailable", message, details)
}

type GatewayTimeoutException struct{ HttpException }

func NewGatewayTimeoutException(message string, details ...any) *GatewayTimeoutException {
	return newBuiltin(func(h HttpException) GatewayTimeoutException { return GatewayTimeoutException{h} },
		http.StatusGatewayTimeout, "GatewayTimeoutException", "Gateway timeout", message, details)
}

type HTTPVersionNotSupportedException struct{ HttpException }

func NewHTTPVersionNotSupportedException(message string, details ...any) *HTTPVersionNotSupportedException {
	return newBuiltin(func(h HttpException) HTTPVersionNotSupportedException { return HTTPVersionNotSupportedException{h} },
		http.StatusHTTPVersionNotSupported, "HTTPVersionNotSupportedException", "HTTP version not supported", message, details)
}

type VariantAlsoNegotiatesException struct{ HttpException }

func NewVariantAlsoNegotiatesException(message string, details ...any) *VariantAlsoNegotiatesException {
	return newBuiltin(func(h HttpException) VariantAlsoNegotiatesException { return VariantAlsoNegotiatesException{h} },
		http.StatusVariantAlsoNegotiates, "VariantAlsoNegotiatesException", "Variant also negotiates", message, details)
}

type InsufficientStorageException struct{ HttpException }

func NewInsufficientStorageException(message string, details ...any) *InsufficientStorageException {
	return newBuiltin(func(h HttpException) InsufficientStorageException { return InsufficientStorageException{h} },
		http.StatusInsufficientStorage, "InsufficientStorageException", "Insufficient storage", message, details)
}

type LoopDetectedException struct{ HttpException }

func NewLoopDetectedException(message string, details ...any) *LoopDetectedException {
	return newBuiltin(func(h HttpException) LoopDetectedException { return LoopDetectedException{h} },
		http.StatusLoopDetected, "LoopDetectedException", "Loop detected", message, details)
}

type NotExtendedException struct{ HttpException }

func NewNotExtendedException(message string, details ...any) *NotExtendedException {
	return newBuiltin(func(h HttpException) NotExtendedException { return NotExtendedException{h} },
		http.StatusNotExtended, "NotExtendedException", "Not extended", message, details)
}

type NetworkAuthenticationRequiredException struct{ HttpException }

func NewNetworkAuthenticationRequiredException(message string, details ...any) *NetworkAuthenticationRequiredException {
	return newBuiltin(func(h HttpException) NetworkAuthenticationRequiredException {
		return NetworkAuthenticationRequiredException{h}
	}, http.StatusNetworkAuthenticationRequired, "NetworkAuthenticationRequiredException", "Network authentication required", message, details)
}
